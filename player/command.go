package player

import (
	"github.com/killlime/killlime/oconfig"
	"github.com/killlime/killlime/player/command"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (p *Player) initKillLimeCommand(pk *packet.AvailableCommands) {
	if p.perms == 0 {
		return
	}
	overloads := command.AllOverloads(p, pk)
	pk.Commands = append(pk.Commands, protocol.Command{
		Name:                     oconfig.Global.CommandName,
		Description:              oconfig.Global.CommandDescription,
		Flags:                    0,
		PermissionLevel:          0,
		AliasesOffset:            ^uint32(0),
		ChainedSubcommandOffsets: []uint32{},
		Overloads:                overloads,
	})
}

func mkNormalParam(name string, pType uint32, optional bool) protocol.CommandParameter {
	return protocol.CommandParameter{
		Name:     name,
		Type:     protocol.CommandArgValid | pType,
		Optional: optional,
		Options:  0,
	}
}
