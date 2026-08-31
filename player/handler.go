package player

import (
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/event"
	"github.com/killlime/killlime/oconfig"
	"github.com/killlime/killlime/player/command"
	"github.com/killlime/killlime/player/lua"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/exp/maps"
)

type EventHandler interface {
	HandleJoin(ctx *event.Context[*Player])
	HandleQuit(ctx *event.Context[*Player])
	HandleCommand(ctx *event.Context[*Player], command string, args []string)
	HandlePunishment(ctx *event.Context[*Player], detection Detection, message *string)
	HandleFlag(ctx *event.Context[*Player], detection Detection, extraData []any)
}

type NopEventHandler struct{}

func (NopEventHandler) HandleJoin(*event.Context[*Player])                           {}
func (NopEventHandler) HandleQuit(*event.Context[*Player])                           {}
func (NopEventHandler) HandleCommand(*event.Context[*Player], string, []string)      {}
func (NopEventHandler) HandlePunishment(*event.Context[*Player], Detection, *string) {}
func (NopEventHandler) HandleFlag(*event.Context[*Player], Detection, []any)         {}

type ExampleEventHandler struct {
	connected     map[string]*Player
	allowedAlerts map[string]*Player
	pMu           sync.RWMutex
	admin         AdminStore
	lua           *lua.Engine
}

func NewExampleEventHandler() *ExampleEventHandler {
	return NewExampleEventHandlerWithAdmin(NewAdminStore(DefaultAdminStorePath, nil))
}

func NewExampleEventHandlerWithAdmin(store AdminStore) *ExampleEventHandler {
	if store == nil {
		store = NewAdminStore(DefaultAdminStorePath, nil)
	}
	h := &ExampleEventHandler{
		connected:     make(map[string]*Player),
		allowedAlerts: make(map[string]*Player),
		admin:         store,
	}

	// Initialize Lua engine if configured.
	if oconfig.Global.Lua.Enabled {
		e := lua.NewEngine(nil)
		e.SetPlayerGetter(func(name string) any {
			h.pMu.RLock()
			defer h.pMu.RUnlock()
			for n, p := range h.connected {
				if strings.EqualFold(n, name) {
					return p
				}
			}
			return nil
		})
		e.SetAdminStore(store)
		e.SetBroadcastPlayers(func() map[string]any {
			players := make(map[string]any)
			h.pMu.RLock()
			for name, p := range h.connected {
				players[name] = p
			}
			h.pMu.RUnlock()
			return players
		})
		dir := oconfig.Global.Lua.Dir
		if dir == "" {
			dir = "scripts"
		}
		if err := e.LoadDir(dir); err != nil {
			slog.Warn("lua: failed to load scripts", "dir", dir, "err", err)
		}
		e.RunAll()
		h.lua = e
	}

	command.TargetResolver = h.resolveTarget

	command.Register(&command.Command{
		Name:        "help",
		Description: "Show all available KillLime commands",
		Permission:  0,
		Usage:       "help",
		Run: func(ctx *command.Context) {
			cmds := command.Commands()
			sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
			ctx.Sender.Message("<green>--- KillLime Commands ---</green>")
			for _, c := range cmds {
				if c.Permission != 0 && !ctx.Sender.HasPerm(c.Permission) {
					continue
				}
				if c.Description == "" {
					continue
				}
				ctx.Sender.Message("<yellow>%s</yellow> <grey>%s</grey>", c.Name, c.Description)
			}
		},
	})

	command.Register(&command.Command{
		Name:        "list",
		Description: "List online players",
		Permission:  PermissionLogs,
		Usage:       "list",
		Run: func(ctx *command.Context) {
			h.pMu.RLock()
			defer h.pMu.RUnlock()
			names := make([]string, 0, len(h.connected))
			for name := range h.connected {
				names = append(names, name)
			}
			sort.Strings(names)
			if len(names) == 0 {
				ctx.Sender.Message("<grey>No players online.</grey>")
				return
			}
			ctx.Sender.Message("<green>Online players (%d):</green> <yellow>%s</yellow>", len(names), strings.Join(names, ", "))
		},
	})

	command.Register(&command.Command{
		Name:        "version",
		Description: "Show KillLime version info",
		Permission:  0,
		Usage:       "version",
		Run: func(ctx *command.Context) {
			ctx.Sender.Message("<green>KillLime</green> <yellow>v2.3.0</yellow>")
			ctx.Sender.Message("<grey>Protocol: %d | Go: %s | OS: %s/%s</grey>",
				GameVersion1_26_40, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			ctx.Sender.Message("<grey>Detections: %d | Config version: %d</grey>",
				len(detectionRegistry()), oconfig.ConfigVersion)
		},
	})

	command.Register(&command.Command{
		Name:        "alerts",
		Description: "Toggle or configure alert notifications",
		Permission:  PermissionAlerts,
		Usage:       "alerts <enable|disable|delayMs>",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAlerts) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:alerts", []string{"alerts"})
			valIdx := command.FindOrCreateEnum(pk, "KillLime:alerts_val", []string{"true", "false", "enable", "disable"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("alerts", enumIdx, false, false),
					{
						Name:     "value",
						Type:     protocol.CommandArgValid | protocol.CommandArgEnum | valIdx,
						Optional: false,
					},
				},
			}
		},
		Run: func(ctx *command.Context) {
			val := ctx.Arg(0)
			switch strings.ToLower(val) {
			case "true", "enable":
				ctx.Sender.(*Player).ReceiveAlerts = true
				ctx.Sender.Message("<green>Alerts enabled</green>")
			case "false", "disable":
				ctx.Sender.(*Player).ReceiveAlerts = false
				ctx.Sender.Message("<red>Alerts disabled</red>")
			default:
				if ms, err := strconv.Atoi(val); err == nil {
					ctx.Sender.(*Player).AlertDelay = time.Duration(ms) * time.Millisecond
					ctx.Sender.Message("<green>Alert delay set to <yellow>%s</yellow></green>", ctx.Sender.(*Player).AlertDelay)
				} else {
					ctx.Sender.Message("<red>Usage: /ac alerts <true|false|enable|disable|delayMs></red>")
				}
			}
		},
	})

	command.Register(&command.Command{
		Name:        "logs",
		Description: "View detection logs for a player",
		Permission:  PermissionLogs,
		Usage:       "logs <player>",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionLogs) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:logs", []string{"logs"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("logs", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
				},
			}
		},
		Run: func(ctx *command.Context) {
			targets := ctx.ResolveTargets(ctx.Arg(0))
			if len(targets) == 0 {
				ctx.Sender.Message("<red>Player not found.</red>")
				return
			}
			t := targets[0]
			tp, ok := t.(*Player)
			if !ok {
				ctx.Sender.Message("<red>Player not found.</red>")
				return
			}
			ctx.Sender.Message("<green>Logs for</green> <yellow>%s</yellow>", tp.Name())
			count := 0
			for _, dtc := range tp.Detections() {
				m := dtc.Metadata()
				if m.Violations < 0.1 {
					continue
				}
				ctx.Sender.Message("<yellow>%s</yellow> <grey>(</grey><red>%s</red><grey>)</grey> <grey>[</grey><red>x%.2f</red><grey>]</grey>",
					dtc.Type(), dtc.SubType(), m.Violations)
				count++
			}
			if count == 0 {
				ctx.Sender.Message("<grey>No active detections.</grey>")
			}
		},
	})

	command.Register(&command.Command{
		Name:        "debug",
		Description: "Toggle debug modes",
		Permission:  PermissionDebug,
		Usage:       "debug [mode|type_message|type_log]",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionDebug) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:debug", []string{"debug"})
			modes := make([]string, 0, len(DebugModeList)+2)
			modes = append(modes, DebugModeList...)
			modes = append(modes, "type_message", "type_log")
			modeIdx := command.FindOrCreateDynamicEnum(pk, "KillLime:debug_modes", modes)
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("debug", enumIdx, false, false),
					command.MakeEnumParam("mode", modeIdx, true, true),
				},
			}
		},
		Run: func(ctx *command.Context) {
			p := ctx.Sender.(*Player)
			if len(ctx.Args) == 0 {
				ctx.Sender.Message("<red>Usage: /ac debug [mode]</red>")
				return
			}
			val := ctx.Arg(0)
			if mode, ok := DebugModeMap[val]; ok {
				p.Dbg.Toggle(mode)
				if p.Dbg.Enabled(mode) {
					ctx.Sender.Message("<green>Debug mode <yellow>%s</yellow> enabled</green>", val)
				} else {
					ctx.Sender.Message("<red>Debug mode <yellow>%s</yellow> disabled</red>", val)
				}
			} else if val == "type_message" {
				p.Dbg.LoggingType = LoggingTypeMessage
				ctx.Sender.Message("<green>Debug type set to <yellow>message</yellow></green>")
			} else if val == "type_log" {
				p.Dbg.LoggingType = LoggingTypeLogFile
				ctx.Sender.Message("<green>Debug type set to <yellow>log file</yellow></green>")
			} else if val == "gmc" && len(os.Getenv("KILLLIME_GAMEMODE_TEST_BECAUSE_DEV")) > 0 {
				p.SendPacketToClient(&packet.SetPlayerGameType{
					GameType: packet.GameTypeCreative,
				})
				ctx.Sender.Message("<green>Set client-side gamemode to <yellow>creative</yellow></green>")
			} else {
				ctx.Sender.Message("<red>Invalid debug mode</red>")
			}
		},
	})

	command.Register(&command.Command{
		Name:        "op",
		Description: "Grant operator to a player",
		Permission:  PermissionAdmin,
		Usage:       "op <player>",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:op", []string{"op"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("op", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
				},
			}
		},
		Run: func(ctx *command.Context) {
			name := ctx.Arg(0)
			targets := ctx.ResolveTargets(name)
			var xuid string
			var online bool
			var target *Player
			if len(targets) > 0 {
				if tp, ok := targets[0].(*Player); ok {
					target = tp
					xuid = tp.IdentityDat.XUID
					online = true
				}
			}
			if h.admin.AddOperator(name, xuid) {
				if online && target != nil {
					target.AddPerm(PermissionAdmin)
					target.Message("<green>You are now an operator.</green>")
				}
				ctx.Sender.Message("<green>%s is now an operator.</green>", name)
			} else {
				ctx.Sender.Message("<yellow>%s is already an operator.</yellow>", name)
			}
		},
	})

	command.Register(&command.Command{
		Name:        "deop",
		Description: "Revoke operator from a player",
		Permission:  PermissionAdmin,
		Usage:       "deop <player>",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:deop", []string{"deop"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("deop", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
				},
			}
		},
		Run: func(ctx *command.Context) {
			name := ctx.Arg(0)
			targets := ctx.ResolveTargets(name)
			var xuid string
			var online bool
			var target *Player
			if len(targets) > 0 {
				if tp, ok := targets[0].(*Player); ok {
					target = tp
					xuid = tp.IdentityDat.XUID
					online = true
				}
			}
			if h.admin.RemoveOperator(name, xuid) {
				if online && target != nil {
					target.RemovePerm(PermissionAdmin)
					target.Message("<red>You are no longer an operator.</red>")
				}
				ctx.Sender.Message("<green>%s is no longer an operator.</green>", name)
			} else {
				ctx.Sender.Message("<yellow>%s is not an operator.</yellow>", name)
			}
		},
	})

	command.Register(&command.Command{
		Name:        "kick",
		Description: "Kick a player",
		Permission:  PermissionAdmin,
		Usage:       "kick <player> [reason]",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:kick", []string{"kick"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("kick", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
					command.MakeNormalParam("reason", protocol.CommandArgTypeMessage, true),
				},
			}
		},
		Run: func(ctx *command.Context) {
			name := ctx.Arg(0)
			targets := ctx.ResolveTargets(name)
			if len(targets) == 0 {
				ctx.Sender.Message("<red>%s is not online.</red>", name)
				return
			}
			tp := targets[0].(*Player)
			reason := ctx.Join(1)
			if reason == "" {
				reason = "kicked by an operator"
			}
			tp.Message("<red>You were kicked: <yellow>%s</yellow></red>", reason)
			tp.Disconnect(reason)
			ctx.Sender.Message("<green>%s was kicked.</green>", name)
		},
	})

	command.Register(&command.Command{
		Name:        "ban",
		Description: "Ban a player",
		Permission:  PermissionAdmin,
		Usage:       "ban <player> [reason]",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:ban", []string{"ban"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("ban", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
					command.MakeNormalParam("reason", protocol.CommandArgTypeMessage, true),
				},
			}
		},
		Run: func(ctx *command.Context) {
			name := ctx.Arg(0)
			targets := ctx.ResolveTargets(name)
			var xuid string
			var online bool
			var target *Player
			if len(targets) > 0 {
				if tp, ok := targets[0].(*Player); ok {
					target = tp
					xuid = tp.IdentityDat.XUID
					online = true
				}
			}
			reason := ctx.Join(1)
			h.admin.Ban(name, xuid, reason, ctx.Sender.Name())
			if online && target != nil {
				target.RemovePerm(PermissionAdmin)
				target.Message("<red>You were banned: <yellow>%s</yellow></red>", reason)
				target.Disconnect(reason)
			}
			ctx.Sender.Message("<green>%s is now banned.</green>", name)
		},
	})

	command.Register(&command.Command{
		Name:        "unban",
		Description: "Unban a player",
		Permission:  PermissionAdmin,
		Usage:       "unban <player>",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:unban", []string{"unban"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("unban", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
				},
			}
		},
		Run: func(ctx *command.Context) {
			name := ctx.Arg(0)
			targets := ctx.ResolveTargets(name)
			var xuid string
			if len(targets) > 0 {
				if tp, ok := targets[0].(*Player); ok {
					xuid = tp.IdentityDat.XUID
				}
			}
			if h.admin.Unban(name, xuid) {
				ctx.Sender.Message("<green>%s has been unbanned.</green>", name)
			} else {
				ctx.Sender.Message("<yellow>%s is not banned.</yellow>", name)
			}
		},
	})

	command.Register(&command.Command{
		Name:        "perms",
		Description: "Grant or revoke permissions for a player",
		Permission:  PermissionAdmin,
		Usage:       "perms <grant|revoke> <player> <permission>",
		MinArgs:     3,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:perms", []string{"perms"})
			actionIdx := command.FindOrCreateEnum(pk, "KillLime:perms_action", []string{"grant", "revoke"})
			permIdx := command.FindOrCreateEnum(pk, "KillLime:perms_name", []string{"alerts", "logs", "debug", "admin"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("perms", enumIdx, false, false),
					command.MakeEnumParam("action", actionIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
					command.MakeEnumParam("permission", permIdx, false, false),
				},
			}
		},
		Run: func(ctx *command.Context) {
			action := strings.ToLower(ctx.Arg(0))
			playerName := ctx.Arg(1)
			permName := ctx.Arg(2)

			perm, ok := PermByName(permName)
			if !ok {
				ctx.Sender.Message("<red>Unknown permission <yellow>%s</yellow>. Valid: alerts, logs, debug, admin</red>", permName)
				return
			}

			targets := ctx.ResolveTargets(playerName)
			if len(targets) == 0 {
				ctx.Sender.Message("<red>Player <yellow>%s</yellow> is not online.</red>", playerName)
				return
			}
			tp, ok := targets[0].(*Player)
			if !ok {
				ctx.Sender.Message("<red>Player not found.</red>")
				return
			}

			switch action {
			case "grant":
				tp.AddPerm(perm)
				tp.Message("<green>Your permission <yellow>%s</yellow> was granted.</green>", permName)
				ctx.Sender.Message("<green>Granted <yellow>%s</yellow> to %s.</green>", permName, tp.Name())
			case "revoke":
				tp.RemovePerm(perm)
				tp.Message("<red>Your permission <yellow>%s</yellow> was revoked.</red>", permName)
				ctx.Sender.Message("<green>Revoked <yellow>%s</yellow> from %s.</green>", permName, tp.Name())
			default:
				ctx.Sender.Message("<red>Usage: /ac perms <grant|revoke> <player> <permission></red>")
			}
		},
	})

	command.Register(&command.Command{
		Name:        "tp",
		Description: "Teleport to a player (client-side only)",
		Permission:  PermissionAdmin,
		Usage:       "tp <player>",
		MinArgs:     1,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:tp", []string{"tp"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("tp", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
				},
			}
		},
		Run: func(ctx *command.Context) {
			name := ctx.Arg(0)
			targets := ctx.ResolveTargets(name)
			if len(targets) == 0 {
				ctx.Sender.Message("<red>%s is not online.</red>", name)
				return
			}
			tp := targets[0].(*Player)
			sender := ctx.Sender.(*Player)
			targetPos := tp.movement.Pos()
			rot := tp.movement.Rotation()
			sender.SendPacketToClient(&packet.MovePlayer{
				EntityRuntimeID: sender.RuntimeId,
				Position:        targetPos,
				Pitch:           rot.X(),
				Yaw:             rot.Z(),
				HeadYaw:         rot.Y(),
				Mode:            packet.MoveModeTeleport,
				OnGround:        true,
				Tick:            uint64(sender.ClientTick),
			})
			sender.Message("<green>Teleported to <yellow>%s</yellow>.</green>", tp.Name())
		},
	})

	command.Register(&command.Command{
		Name:        "sudo",
		Description: "Execute a command as another player",
		Permission:  PermissionAdmin,
		Usage:       "sudo <player> <command...>",
		MinArgs:     2,
		Overloads: func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
			if !p.HasPerm(PermissionAdmin) {
				return nil
			}
			enumIdx := command.FindOrCreateEnum(pk, "KillLime:sudo", []string{"sudo"})
			return &protocol.CommandOverload{
				Parameters: []protocol.CommandParameter{
					command.MakeEnumParam("sudo", enumIdx, false, false),
					command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
					command.MakeNormalParam("command", protocol.CommandArgTypeCommand, false),
				},
			}
		},
		Run: func(ctx *command.Context) {
			name := ctx.Arg(0)
			targets := ctx.ResolveTargets(name)
			if len(targets) == 0 {
				ctx.Sender.Message("<red>%s is not online.</red>", name)
				return
			}
			tp, ok := targets[0].(*Player)
			if !ok {
				ctx.Sender.Message("<red>Player not found.</red>")
				return
			}
			cmdLine := ctx.Join(1)
			tp.Message("<grey>[sudo] %s: %s</grey>", ctx.Sender.Name(), cmdLine)
			tp.SendPacketToClient(&packet.CommandRequest{
				CommandLine: cmdLine,
				CommandOrigin: protocol.CommandOrigin{
					Origin: protocol.CommandOriginPlayer,
				},
				Internal: false,
				Version:  "",
			})
			ctx.Sender.Message("<green>Executed <yellow>%s</yellow> as %s.</green>", cmdLine, tp.Name())
		},
	})

	command.Register(&command.Command{
		Name:        "reload",
		Description: "Reload Lua scripts",
		Permission:  PermissionAdmin,
		Usage:       "reload",
		Run: func(ctx *command.Context) {
			if h.lua == nil {
				ctx.Sender.Message("<red>Lua scripting is not enabled.</red>")
				return
			}
			if err := h.lua.Reload(); err != nil {
				ctx.Sender.Message("<red>Failed to reload: %s</red>", err.Error())
				return
			}
			ctx.Sender.Message("<green>Scripts reloaded.</green>")
		},
	})

	go h.refreshAlertList()
	return h
}

func (h *ExampleEventHandler) resolveTarget(src command.Sender, selector string) []command.Sender {
	if selector == "@s" {
		return []command.Sender{src}
	}
	h.pMu.RLock()
	defer h.pMu.RUnlock()

	if selector == "@a" {
		out := make([]command.Sender, 0, len(h.connected))
		for _, p := range h.connected {
			out = append(out, p)
		}
		return out
	}
	if selector == "@p" {
		for _, p := range h.connected {
			if p.HasPerm(PermissionAdmin) {
				return []command.Sender{p}
			}
		}
		for _, p := range h.connected {
			return []command.Sender{p}
		}
		return nil
	}
	for name, p := range h.connected {
		if strings.EqualFold(name, selector) {
			return []command.Sender{p}
		}
	}
	return nil
}

func (h *ExampleEventHandler) HandleJoin(ctx *event.Context[*Player]) {
	p := ctx.Val()
	if ban, ok := h.admin.IsBanned(p.Name(), p.IdentityDat.XUID); ok {
		ctx.Cancel()
		reason := ban.Reason
		if reason == "" {
			reason = "banned"
		}
		p.Message("<red>You are banned: <yellow>%s</yellow></red>", reason)
		p.Disconnect(reason)
		return
	}
	if h.admin.PromoteIfFirstOperator(p.Name(), p.IdentityDat.XUID) {
		p.AddPerm(PermissionAdmin)
		p.Message("<green>You have been granted operator as the first player to join.</green>")
	} else if h.admin.IsOperator(p.Name(), p.IdentityDat.XUID) {
		p.AddPerm(PermissionAdmin)
	}
	h.pMu.Lock()
	defer h.pMu.Unlock()
	h.connected[p.Name()] = p
	p.BroadcastChat = func(msg string) {
		h.pMu.RLock()
		defer h.pMu.RUnlock()
		for _, other := range h.connected {
			if other != p {
				other.RawMessage(msg)
			}
		}
	}
	if p.HasPerm(PermissionAlerts) {
		h.allowedAlerts[p.Name()] = p
	}
	// Lua hook: on_join
	if h.lua != nil {
		h.lua.CallHandler("on_join", &lua.PlayerEvent{
			Name:        p.Name(),
			XUID:        p.IdentityDat.XUID,
			DisplayName: p.IdentityDat.DisplayName,
			Version:     p.Version,
			GameMode:    p.GameMode,
			InputMode:   p.InputMode,
			TicksAlive:  p.TicksAlive,
			IsOp:        p.HasPerm(PermissionAdmin),
			Perms:       p.Perms(),
		})
	}
}

func (h *ExampleEventHandler) HandleQuit(ctx *event.Context[*Player]) {
	p := ctx.Val()
	if h.lua != nil {
		h.lua.CallHandler("on_quit", &lua.PlayerEvent{
			Name:        p.Name(),
			XUID:        p.IdentityDat.XUID,
			DisplayName: p.IdentityDat.DisplayName,
		})
	}
	h.pMu.Lock()
	defer h.pMu.Unlock()
	delete(h.connected, p.Name())
	delete(h.allowedAlerts, p.Name())
}

func (h *ExampleEventHandler) HandleCommand(ctx *event.Context[*Player], cmd string, args []string) {
	p := ctx.Val()
	if !command.Dispatch(p, cmd, args) {
		return
	}
	ctx.Cancel()
}

func (h *ExampleEventHandler) HandlePunishment(ctx *event.Context[*Player], detection Detection, message *string) {
}

func (h *ExampleEventHandler) HandleFlag(ctx *event.Context[*Player], dtc Detection, extraData []any) {
	p := ctx.Val()
	m := dtc.Metadata()
	dtcKey := dtc.Type() + "_" + dtc.SubType()
	msgTmpl := oconfig.DtcOpts(dtcKey).FlagMsg
	viol := strconv.FormatFloat(m.Violations, 'f', 2, 64)
	alertMsg := strings.NewReplacer(
		"{prefix}", oconfig.Global.Prefix,
		"{player}", p.IdentityDat.DisplayName,
		"{xuid}", p.IdentityDat.XUID,
		"{detection_type}", dtc.Type(),
		"{detection_subtype}", dtc.SubType(),
		"{violations}", viol,
	).Replace(msgTmpl)
	h.pMu.RLock()
	recipients := make([]*Player, 0, len(h.allowedAlerts))
	for _, r := range h.allowedAlerts {
		recipients = append(recipients, r)
	}
	h.pMu.RUnlock()
	for _, r := range recipients {
		r.ReceiveAlert(alertMsg)
	}
	// Lua hook: on_flag
	if h.lua != nil {
		h.lua.CallHandler("on_flag", &lua.PlayerEvent{
			Name:        p.Name(),
			XUID:        p.IdentityDat.XUID,
			DisplayName: p.IdentityDat.DisplayName,
			Version:     p.Version,
			GameMode:    p.GameMode,
		})
	}
}

func (h *ExampleEventHandler) refreshAlertList() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		h.pMu.Lock()
		maps.Clear(h.allowedAlerts)
		for _, p := range h.connected {
			if p.HasPerm(PermissionAlerts) && p.ReceiveAlerts && time.Since(p.LastAlert) >= p.AlertDelay {
				h.allowedAlerts[p.Name()] = p
			}
		}
		h.pMu.Unlock()
	}
}

func (h *ExampleEventHandler) playerByName(name string) (*Player, bool) {
	h.pMu.RLock()
	defer h.pMu.RUnlock()
	for n, p := range h.connected {
		if strings.EqualFold(n, name) {
			return p, true
		}
	}
	return nil, false
}

func xuidOf(p *Player) string {
	if p == nil {
		return ""
	}
	return p.IdentityDat.XUID
}

// detectionRegistry returns all registered detection types for version info.
func detectionRegistry() []string {
	return nil
}
