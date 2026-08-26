package command

import (
	"strings"
	"sync"
)

// Sender is anything capable of receiving command output and holding
// permissions. *player.Player implements it, as does the console wrapper.
type Sender interface {
	Name() string
	HasPerm(perm uint64) bool
	Message(format string, args ...any)
}

// Context is the runtime context a command executes with.
type Context struct {
	Sender  Sender
	Command *Command
	Args    []string
}

// Arg returns the argument at index i, or "" when out of range.
func (c *Context) Arg(i int) string {
	if i < 0 || i >= len(c.Args) {
		return ""
	}
	return c.Args[i]
}

// Join joins arguments from index i with spaces.
func (c *Context) Join(i int) string {
	if i < 0 || i >= len(c.Args) {
		return ""
	}
	return strings.Join(c.Args[i:], " ")
}

// TargetResolver resolves a player selector or name to online senders. It is
// installed by the active event handler so commands can reach online players.
var TargetResolver func(src Sender, selector string) []Sender

// ResolveTargets resolves a selector: "@a" (all), "@s" (self), "@p" (nearest /
// self on the proxy), or a case-insensitive player name.
func (c *Context) ResolveTargets(selector string) []Sender {
	if TargetResolver == nil {
		return nil
	}
	return TargetResolver(c.Sender, selector)
}

// Command is a fully described KillLime command with a handler.
type Command struct {
	Name        string
	Description string
	Aliases     []string
	// Permission required to see and run the command. 0 means usable by anyone
	// who holds any permission at all.
	Permission uint64
	Usage      string
	MinArgs    int
	// Overloads contributes client-visible overloads to AvailableCommands.
	// It may be nil for hidden commands.
	Overloads SubCommandsFn
	// Run executes the command. It receives the parsed context.
	Run func(ctx *Context)
}

var (
	cmdMu      sync.RWMutex
	registry   = map[string]*Command{}
	aliasIndex = map[string]*Command{}
)

// Register adds a command. Registering a name or alias that already exists
// replaces the previous entry so handlers may be re-built safely.
func Register(c *Command) {
	cmdMu.Lock()
	defer cmdMu.Unlock()
	if existing, ok := registry[c.Name]; ok {
		for _, a := range existing.Aliases {
			delete(aliasIndex, a)
		}
	}
	registry[c.Name] = c
	for _, a := range c.Aliases {
		aliasIndex[strings.ToLower(a)] = c
	}
}

// ByName returns the command registered under name or any of its aliases.
func ByName(name string) (*Command, bool) {
	cmdMu.RLock()
	defer cmdMu.RUnlock()
	if c, ok := registry[name]; ok {
		return c, true
	}
	c, ok := aliasIndex[strings.ToLower(name)]
	return c, ok
}

// Commands returns every registered command in a stable order.
func Commands() []*Command {
	cmdMu.RLock()
	defer cmdMu.RUnlock()
	out := make([]*Command, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	return out
}

// Dispatch parses and runs a subcommand already split from the /ac prefix.
// It reports false when the command is unknown so callers may fall through.
func Dispatch(s Sender, name string, args []string) bool {
	c, ok := ByName(name)
	if !ok {
		if s != nil {
			s.Message("<red>Unknown command <yellow>%s</yellow>. Try <yellow>help</yellow>.</red>", name)
		}
		return false
	}
	if c.Permission != 0 && !s.HasPerm(c.Permission) {
		s.Message("<red>You do not have permission to use <yellow>%s</yellow>.</red>", c.Name)
		return true
	}
	if len(args) < c.MinArgs {
		s.Message("<red>Usage: <yellow>%s</yellow></red>", c.Usage)
		return true
	}
	c.Run(&Context{Sender: s, Command: c, Args: args})
	return true
}
