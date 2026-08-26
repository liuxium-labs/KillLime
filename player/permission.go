package player

import "strings"

// Permission bits. Admin is the top of the ladder and implies every other
// permission below it through HasPerm.
const (
	PermissionAlerts uint64 = 1 << iota
	PermissionLogs
	PermissionDebug
	PermissionAdmin
)

// permissionParents maps a permission to the permissions that imply it. Admin
// implies every leaf permission.
var permissionParents = map[uint64][]uint64{
	PermissionAlerts: {PermissionAdmin},
	PermissionLogs:   {PermissionAdmin},
	PermissionDebug:  {PermissionAdmin},
}

// permissionNames lists every named permission for parsing and display.
var permissionNames = map[string]uint64{
	"alerts": PermissionAlerts,
	"logs":   PermissionLogs,
	"debug":  PermissionDebug,
	"admin":  PermissionAdmin,
}

// AddPerm grants one or more permission bits.
func (p *Player) AddPerm(perms ...uint64) {
	for _, perm := range perms {
		p.perms |= perm
	}
}

// RemovePerm revokes one or more permission bits.
func (p *Player) RemovePerm(perms ...uint64) {
	for _, perm := range perms {
		p.perms &^= perm
	}
}

// SetPerms replaces the entire permission set.
func (p *Player) SetPerms(perms uint64) {
	p.perms = perms
}

// Perms returns the raw permission bits.
func (p *Player) Perms() uint64 {
	return p.perms
}

// HasPerm reports whether p holds perm directly or through an implying parent.
func (p *Player) HasPerm(perm uint64) bool {
	if p.perms == 0 {
		return false
	}
	if p.perms&perm != 0 {
		return true
	}
	for _, parent := range permissionParents[perm] {
		if p.perms&parent != 0 {
			return true
		}
	}
	return false
}

// PermByName resolves a human-readable permission name (case-insensitive).
func PermByName(name string) (uint64, bool) {
	perm, ok := permissionNames[strings.ToLower(strings.TrimSpace(name))]
	return perm, ok
}

// PermName returns the stable name for a permission bit.
func PermName(perm uint64) string {
	for name, bit := range permissionNames {
		if bit == perm {
			return name
		}
	}
	return "unknown"
}

// AllPermissions returns every defined permission bit.
func AllPermissions() []uint64 {
	return []uint64{PermissionAlerts, PermissionLogs, PermissionDebug, PermissionAdmin}
}
