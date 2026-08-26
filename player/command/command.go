package command

import (
	"sort"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var subCmds = map[string][]SubCommandsFn{}

// Permissible is the minimal permission surface needed to render overloads.
type Permissible interface {
	HasPerm(perm uint64) bool
}

// SubCommandsFn renders one overload for the client-side command tree. Return
// nil when the caller should not see this overload.
type SubCommandsFn func(p Permissible, cmdPk *packet.AvailableCommands) *protocol.CommandOverload

// RegisterSubCommand registers a legacy overload-only entry. Prefer Register
// with a full *Command for new commands; this is kept for compatibility.
func RegisterSubCommand(name string, fn SubCommandsFn) {
	subCmds[name] = append(subCmds[name], fn)
}

// SubCommands returns the legacy overload map.
func SubCommands() map[string][]SubCommandsFn {
	return subCmds
}

// AllOverloads renders every overload visible to p, in deterministic order:
// registered commands first, then legacy entries.
func AllOverloads(p Permissible, pk *packet.AvailableCommands) []protocol.CommandOverload {
	cmds := Commands()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	var out []protocol.CommandOverload
	for _, c := range cmds {
		if c.Permission != 0 && !p.HasPerm(c.Permission) {
			continue
		}
		if c.Overloads == nil {
			continue
		}
		if o := c.Overloads(p, pk); o != nil {
			out = append(out, *o)
		}
	}
	legacyNames := make([]string, 0, len(subCmds))
	for name := range subCmds {
		legacyNames = append(legacyNames, name)
	}
	sort.Strings(legacyNames)
	for _, name := range legacyNames {
		for _, fn := range subCmds[name] {
			if o := fn(p, pk); o != nil {
				out = append(out, *o)
			}
		}
	}
	return out
}
