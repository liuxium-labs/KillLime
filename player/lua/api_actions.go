package lua

import (
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/killlime/killlime/player/command"
)

const permAdmin = 8 // player.PermissionAdmin: 1 << 3

func (e *Engine) RegisterCommandAPI() {
	e.L.SetGlobal("register_command", e.L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		desc := L.OptString(2, "")
		usage := L.OptString(3, "")
		perm := uint64(L.OptNumber(4, 0))

		cmd := &command.Command{
			Name:        name,
			Description: desc,
			Permission:  perm,
			Usage:       usage,
			Run: func(ctx *command.Context) {
				e.execLua(func(L *lua.LState) {
					fn := L.GetGlobal(name)
					if fn == lua.LNil {
						ctx.Sender.Message("<red>Command function not loaded</red>")
						return
					}
					argsTbl := L.NewTable()
					for i, a := range ctx.Args {
						argsTbl.RawSetInt(i+1, lua.LString(a))
					}
					senderTbl := L.NewTable()
					senderTbl.RawSetString("name", lua.LString(ctx.Sender.Name()))
					senderTbl.RawSetString("is_op", lua.LBool(ctx.Sender.HasPerm(permAdmin)))
					senderTbl.RawSetString("message", L.NewFunction(func(L *lua.LState) int {
						ctx.Sender.Message(L.CheckString(2))
						return 0
					}))
					if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, senderTbl, argsTbl); err != nil {
						e.log.Warn("lua: command error", "cmd", name, "err", err)
					}
				})
			},
		}
		command.Register(cmd)
		e.log.Info("lua: registered command", "name", name)
		return 0
	}))
}

func (e *Engine) RegisterPlayerActionsAPI() {
	e.L.SetGlobal("kick_player", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		reason := L.OptString(2, "kicked by admin")
		if e.playerGetter == nil {
			return 0
		}
		p := e.playerGetter(playerName)
		if p == nil {
			return 0
		}
		type kickable interface {
			Message(string, ...any)
			Disconnect(string)
		}
		if d, ok := p.(kickable); ok {
			d.Message("<red>You were kicked: <yellow>%s</yellow></red>", reason)
			d.Disconnect(reason)
		}
		return 0
	}))

	e.L.SetGlobal("ban_player", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		reason := L.OptString(2, "banned by admin")
		type bannable interface {
			Ban(name, xuid, reason, by string)
		}
		type disconnectable interface {
			Message(string, ...any)
			Disconnect(string)
		}
		type xuidable interface{ XUID() string }

		var xuid string
		if e.playerGetter != nil {
			if p := e.playerGetter(playerName); p != nil {
				if xg, ok := p.(xuidable); ok {
					xuid = xg.XUID()
				}
			}
		}
		if b, ok := e.adminStore.(bannable); ok {
			b.Ban(playerName, xuid, reason, "lua")
		}
		if e.playerGetter != nil {
			if p := e.playerGetter(playerName); p != nil {
				if d, ok := p.(disconnectable); ok {
					d.Message("<red>You were banned: <yellow>%s</yellow></red>", reason)
					d.Disconnect(reason)
				}
			}
		}
		return 0
	}))

	e.L.SetGlobal("tempban_player", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		seconds := float64(L.CheckNumber(2))
		reason := L.OptString(3, "temp banned by admin")
		type tempBannable interface {
			BanTemp(name, xuid, reason, by string, expiresAt time.Time)
		}
		type xuidable interface{ XUID() string }

		var xuid string
		if e.playerGetter != nil {
			if p := e.playerGetter(playerName); p != nil {
				if xg, ok := p.(xuidable); ok {
					xuid = xg.XUID()
				}
			}
		}
		if b, ok := e.adminStore.(tempBannable); ok {
			b.BanTemp(playerName, xuid, reason, "lua", time.Now().Add(time.Duration(seconds*float64(time.Second))))
		}
		return 0
	}))

	e.L.SetGlobal("unban_player", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		type unbannable interface {
			Unban(name, xuid string) bool
		}
		type xuidable interface{ XUID() string }

		var xuid string
		if e.playerGetter != nil {
			if p := e.playerGetter(playerName); p != nil {
				if xg, ok := p.(xuidable); ok {
					xuid = xg.XUID()
				}
			}
		}
		if b, ok := e.adminStore.(unbannable); ok {
			b.Unban(playerName, xuid)
		}
		return 0
	}))

	e.L.SetGlobal("send_message", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		msg := L.CheckString(2)
		if e.playerGetter == nil {
			return 0
		}
		p := e.playerGetter(playerName)
		if p == nil {
			return 0
		}
		type messenger interface{ Message(string, ...any) }
		if m, ok := p.(messenger); ok {
			m.Message(msg)
		}
		return 0
	}))

	e.L.SetGlobal("send_popup", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		msg := L.CheckString(2)
		if e.playerGetter == nil {
			return 0
		}
		p := e.playerGetter(playerName)
		if p == nil {
			return 0
		}
		type popuper interface{ Popup(string, ...any) }
		if pup, ok := p.(popuper); ok {
			pup.Popup(msg)
		}
		return 0
	}))

	e.L.SetGlobal("broadcast", e.L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		if e.broadcastPlayers == nil {
			return 0
		}
		type messenger interface{ Message(string, ...any) }
		players := e.broadcastPlayers()
		for _, p := range players {
			if m, ok := p.(messenger); ok {
				m.Message(msg)
			}
		}
		return 0
	}))

	e.L.SetGlobal("get_online_players", e.L.NewFunction(func(L *lua.LState) int {
		tbl := L.NewTable()
		if e.broadcastPlayers != nil {
			players := e.broadcastPlayers()
			i := 1
			for name := range players {
				tbl.RawSetInt(i, lua.LString(name))
				i++
			}
		}
		L.Push(tbl)
		return 1
	}))

	e.L.SetGlobal("is_banned", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		type banChecker interface {
			IsBanned(name, xuid string) (any, bool)
		}
		if b, ok := e.adminStore.(banChecker); ok {
			_, banned := b.IsBanned(playerName, "")
			L.Push(lua.LBool(banned))
		} else {
			L.Push(lua.LBool(false))
		}
		return 1
	}))

	e.L.SetGlobal("is_op", e.L.NewFunction(func(L *lua.LState) int {
		playerName := L.CheckString(1)
		type opChecker interface {
			IsOperator(name, xuid string) bool
		}
		if b, ok := e.adminStore.(opChecker); ok {
			L.Push(lua.LBool(b.IsOperator(playerName, "")))
		} else {
			L.Push(lua.LBool(false))
		}
		return 1
	}))
}
