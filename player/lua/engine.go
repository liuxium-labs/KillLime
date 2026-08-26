package lua

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type Engine struct {
	mu      sync.RWMutex
	L       *lua.LState
	log     *slog.Logger
	scripts map[string]string
	stop    chan struct{}
	closed  bool
	timers  *TimerRegistry
	luaCh   chan func(*lua.LState)

	playerGetter     func(name string) any
	adminStore       any
	broadcastPlayers func() map[string]any
}

func NewEngine(log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	stop := make(chan struct{})
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	e := &Engine{
		L:       L,
		log:     log,
		scripts: make(map[string]string),
		stop:    stop,
		timers:  NewTimerRegistry(log, stop),
		luaCh:   make(chan func(*lua.LState), 256),
	}
	e.openSafeLibs()
	e.registerBuiltins()
	e.RegisterCommandAPI()
	e.RegisterPlayerActionsAPI()
	go e.luaWorker()
	return e
}

func (e *Engine) SetPlayerGetter(fn func(name string) any) {
	e.playerGetter = fn
}

func (e *Engine) SetAdminStore(store any) {
	e.adminStore = store
}

func (e *Engine) SetBroadcastPlayers(fn func() map[string]any) {
	e.broadcastPlayers = fn
}

func (e *Engine) luaWorker() {
	for {
		select {
		case <-e.stop:
			return
		case fn := <-e.luaCh:
			e.mu.RLock()
			if e.closed {
				e.mu.RUnlock()
				return
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						e.log.Warn("lua: panic", "panic", r)
					}
				}()
				fn(e.L)
			}()
			e.mu.RUnlock()
		}
	}
}

func (e *Engine) execLua(fn func(L *lua.LState)) {
	select {
	case e.luaCh <- fn:
	default:
		e.log.Warn("lua: call queue full, dropping call")
	}
}

func (e *Engine) openSafeLibs() {
	L := e.L

	L.OpenLibs()

	L.SetGlobal("os", lua.LNil)
	L.SetGlobal("io", lua.LNil)
	L.SetGlobal("debug", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("module", lua.LNil)
	L.SetGlobal("package", lua.LNil)
	L.SetGlobal("require", lua.LNil)
	L.SetGlobal("collectgarbage", lua.LNil)
	L.SetGlobal("newproxy", lua.LNil)

	stringMod := L.RegisterModule(lua.StringLibName, map[string]lua.LGFunction{
		"byte":   luaStringByte,
		"char":   luaStringChar,
		"find":   luaStringFind,
		"format": luaStringFormat,
		"gsub":   luaStringGsub,
		"len":    luaStringLen,
		"lower":  luaStringLower,
		"upper":  luaStringUpper,
		"trim":   luaStringTrim,
		"match":  luaStringMatch,
		"rep":    luaStringRep,
		"sub":    luaStringSub,
	})
	L.SetGlobal("string", stringMod)

	mathMod := L.RegisterModule(lua.MathLibName, map[string]lua.LGFunction{
		"abs":   luaMathAbs,
		"ceil":  luaMathCeil,
		"floor": luaMathFloor,
		"max":   luaMathMax,
		"min":   luaMathMin,
		"sqrt":  luaMathSqrt,
		"random": func(L *lua.LState) int {
			L.Push(lua.LNumber(float64(time.Now().UnixNano()%10000) / 10000.0))
			return 1
		},
	})
	L.SetGlobal("math", mathMod)

	tableMod := L.RegisterModule("table", map[string]lua.LGFunction{
		"insert": luaTableInsert,
		"remove": luaTableRemove,
		"sort":   luaTableSort,
		"concat": luaTableConcat,
		"keys":   luaTableKeys,
	})
	L.SetGlobal("table", tableMod)

	L.SetGlobal("print", L.NewFunction(luaPrint))
}

func (e *Engine) registerBuiltins() {
	L := e.L
	L.SetGlobal("type", L.NewFunction(luaType))
	L.SetGlobal("tostring", L.NewFunction(luaTostring))
	L.SetGlobal("tonumber", L.NewFunction(luaTonumber))
	L.SetGlobal("error", L.NewFunction(func(L *lua.LState) int {
		msg := L.OptString(1, "error")
		L.ArgError(1, msg)
		return 0
	}))

	timerMod := L.RegisterModule("timer", map[string]lua.LGFunction{
		"after": func(L *lua.LState) int {
			sec := float64(L.CheckNumber(1))
			fn := L.CheckFunction(2)
			e.timers.After(sec, func() {
				e.execLua(func(L *lua.LState) {
					L.CallByParam(lua.P{Fn: fn, Protect: true})
				})
			})
			return 0
		},
		"every": func(L *lua.LState) int {
			sec := float64(L.CheckNumber(1))
			fn := L.CheckFunction(2)
			id := e.timers.Every(sec, func() {
				e.execLua(func(L *lua.LState) {
					L.CallByParam(lua.P{Fn: fn, Protect: true})
				})
			})
			L.Push(lua.LNumber(id))
			return 1
		},
		"cancel": func(L *lua.LState) int {
			id := int64(L.CheckNumber(1))
			e.timers.Cancel(id)
			return 0
		},
	})
	L.SetGlobal("timer", timerMod)

	logMod := L.RegisterModule("log", map[string]lua.LGFunction{
		"info":  func(L *lua.LState) int { e.log.Info(L.OptString(1, "")); return 0 },
		"warn":  func(L *lua.LState) int { e.log.Warn(L.OptString(1, "")); return 0 },
		"error": func(L *lua.LState) int { e.log.Error(L.OptString(1, "")); return 0 },
		"debug": func(L *lua.LState) int { e.log.Debug(L.OptString(1, "")); return 0 },
	})
	L.SetGlobal("log", logMod)
}

func (e *Engine) LoadDir(dir string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if dir == "" {
		return nil
	}
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".lua") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			e.log.Warn("lua: read script", "path", path, "err", err)
			return nil
		}
		name := filepath.Base(path)
		e.scripts[name] = string(data)
		return nil
	})
}

func (e *Engine) LoadScript(name, src string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scripts[name] = src
}

func (e *Engine) RunAll() {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for name, src := range e.scripts {
		e.execSource(name, src)
	}
}

func (e *Engine) execSource(name, src string) {
	e.execLua(func(L *lua.LState) {
		fn, err := L.LoadString(src)
		if err != nil {
			e.log.Warn("lua: load script error", "script", name, "err", err)
			return
		}
		if err := L.CallByParam(lua.P{Fn: fn, Protect: true}); err != nil {
			e.log.Warn("lua: script error", "script", name, "err", err)
		}
	})
}

func (e *Engine) CallHandler(name string, ev *PlayerEvent) {
	e.execLua(func(L *lua.LState) {
		fn := L.GetGlobal(name)
		if fn == lua.LNil {
			return
		}
		tbl := e.eventToTable(ev)
		if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, tbl); err != nil {
			e.log.Warn("lua: handler error", "fn", name, "err", err)
		}
	})
}

func (e *Engine) eventToTable(ev *PlayerEvent) *lua.LTable {
	tbl := e.L.NewTable()
	tbl.RawSetString("name", lua.LString(ev.Name))
	tbl.RawSetString("xuid", lua.LString(ev.XUID))
	tbl.RawSetString("display_name", lua.LString(ev.DisplayName))
	tbl.RawSetString("version", lua.LNumber(ev.Version))
	tbl.RawSetString("gamemode", lua.LNumber(ev.GameMode))
	tbl.RawSetString("input_mode", lua.LNumber(ev.InputMode))
	tbl.RawSetString("ticks_alive", lua.LNumber(ev.TicksAlive))
	tbl.RawSetString("is_op", lua.LBool(ev.IsOp))
	tbl.RawSetString("perms", lua.LNumber(ev.Perms))

	if ev.Detection != nil {
		d := ev.Detection
		dt := e.L.NewTable()
		dt.RawSetString("x", lua.LNumber(d.Position[0]))
		dt.RawSetString("y", lua.LNumber(d.Position[1]))
		dt.RawSetString("z", lua.LNumber(d.Position[2]))
		dt.RawSetString("last_x", lua.LNumber(d.LastPos[0]))
		dt.RawSetString("last_y", lua.LNumber(d.LastPos[1]))
		dt.RawSetString("last_z", lua.LNumber(d.LastPos[2]))
		dt.RawSetString("vel_x", lua.LNumber(d.VelX))
		dt.RawSetString("vel_y", lua.LNumber(d.VelY))
		dt.RawSetString("vel_z", lua.LNumber(d.VelZ))
		dt.RawSetString("yaw", lua.LNumber(d.Yaw))
		dt.RawSetString("pitch", lua.LNumber(d.Pitch))
		dt.RawSetString("head_yaw", lua.LNumber(d.HeadYaw))
		dt.RawSetString("on_ground", lua.LBool(d.OnGround))
		dt.RawSetString("air_ticks", lua.LNumber(d.AirTicks))
		dt.RawSetString("fly_ticks", lua.LNumber(d.FlyTicks))
		dt.RawSetString("sprint_ticks", lua.LNumber(d.SprintTicks))
		dt.RawSetString("swim_ticks", lua.LNumber(d.SwimTicks))
		dt.RawSetString("climb_ticks", lua.LNumber(d.ClimbTicks))
		dt.RawSetString("action", lua.LNumber(d.Action))
		dt.RawSetString("hyper_speed", lua.LNumber(d.HyperSpeed))
		dt.RawSetString("hover_time", lua.LNumber(d.HoverTime))
		dt.RawSetString("step_height", lua.LNumber(d.StepHeight))
		dt.RawSetString("allow_flying", lua.LBool(d.AllowFlying))
		dt.RawSetString("collision", lua.LBool(d.Collision))
		dt.RawSetString("fall_distance", lua.LNumber(d.FallDistance))
		dt.RawSetString("speed", lua.LNumber(d.Speed))
		dt.RawSetString("vspeed", lua.LNumber(d.VSpeed))
		dt.RawSetString("yaw_diff", lua.LNumber(d.YawDiff))
		dt.RawSetString("pitch_diff", lua.LNumber(d.PitchDiff))
		dt.RawSetString("head_rot_diff", lua.LNumber(d.HeadRotDiff))
		dt.RawSetString("ticks", lua.LNumber(d.Ticks))
		tbl.RawSetString("detection", dt)
	}
	return tbl
}

func (e *Engine) Reload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("engine closed")
	}

	close(e.stop)
	e.stop = make(chan struct{})

	e.L.Close()

drained:
	for {
		select {
		case <-e.luaCh:
		default:
			break drained
		}
	}

	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	e.L = L
	e.timers = NewTimerRegistry(e.log, e.stop)

	e.openSafeLibs()
	e.registerBuiltins()
	e.RegisterCommandAPI()
	e.RegisterPlayerActionsAPI()

	for name, src := range e.scripts {
		fn, err := e.L.LoadString(src)
		if err != nil {
			e.log.Warn("lua: reload script error", "script", name, "err", err)
			continue
		}
		if err := e.L.CallByParam(lua.P{Fn: fn, Protect: true}); err != nil {
			e.log.Warn("lua: reload script error", "script", name, "err", err)
		}
	}
	return nil
}

func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	close(e.stop)
	e.L.Close()
}

type TimerRegistry struct {
	mu     sync.Mutex
	nextID int64
	timers map[int64]func() bool
	stop   chan struct{}
	log    *slog.Logger
}

func NewTimerRegistry(log *slog.Logger, stop chan struct{}) *TimerRegistry {
	return &TimerRegistry{
		timers: make(map[int64]func() bool),
		stop:   stop,
		log:    log,
	}
}

func (t *TimerRegistry) After(seconds float64, fn func()) {
	time.AfterFunc(time.Duration(seconds*float64(time.Second)), fn)
}

func (t *TimerRegistry) Every(seconds float64, fn func()) int64 {
	d := time.Duration(seconds * float64(time.Second))
	ticker := time.NewTicker(d)
	t.mu.Lock()
	id := t.nextID
	t.nextID++
	t.timers[id] = func() bool { ticker.Stop(); return true }
	t.mu.Unlock()
	go func() {
		for {
			select {
			case <-t.stop:
				ticker.Stop()
				return
			case <-ticker.C:
				fn()
			}
		}
	}()
	return id
}

func (t *TimerRegistry) Cancel(id int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if stopFn, ok := t.timers[id]; ok && stopFn != nil {
		stopFn()
	}
	delete(t.timers, id)
}
