package player

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultAdminStorePath is the default file the example event handler persists
// its operator and ban lists to when none is configured explicitly.
const DefaultAdminStorePath = "killlime_admin.json"

// AdminIdentity identifies an operator or a banned player. Bans and operators
// match on either the XUID (which survives display-name changes) or the
// display name itself.
type AdminIdentity struct {
	Name string `json:"name"`
	XUID string `json:"xuid"`
}

// BanEntry describes an active ban.
type BanEntry struct {
	Identity AdminIdentity `json:"identity"`
	Reason   string        `json:"reason"`
	By       string        `json:"by"`
	At       time.Time     `json:"at"`
}

// AdminStore keeps the operator list and the active bans that are managed from
// the game through the /ac op, /ac deop, /ac kick, /ac ban and /ac unban
// commands. State is persisted to disk so operator and ban status survive
// restarts.
type AdminStore struct {
	mu   sync.RWMutex
	path string
	log  *slog.Logger
	ops  []AdminIdentity
	bans []BanEntry
}

// NewAdminStore creates an admin store backed by the JSON file at path. If path
// is empty the store is kept in memory only and is never written to disk. A nil
// logger falls back to the default logger.
func NewAdminStore(path string, log *slog.Logger) *AdminStore {
	if log == nil {
		log = slog.Default()
	}
	s := &AdminStore{path: path, log: log}
	s.load()
	return s
}

func (s *AdminStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
}

func (s *AdminStore) loadLocked() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			s.log.Warn("admin store: failed to read", "path", s.path, "err", err)
		}
		return
	}
	type disk struct {
		Operators []AdminIdentity `json:"operators"`
		Bans      []BanEntry      `json:"bans"`
	}
	var d disk
	if err := json.Unmarshal(data, &d); err != nil {
		s.log.Warn("admin store: failed to parse", "path", s.path, "err", err)
		return
	}
	s.ops = d.Operators
	s.bans = d.Bans
}

func (s *AdminStore) save() {
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		s.log.Warn("admin store: failed to create directory", "path", s.path, "err", err)
		return
	}
	data, err := json.MarshalIndent(struct {
		Operators []AdminIdentity `json:"operators"`
		Bans      []BanEntry      `json:"bans"`
	}{s.ops, s.bans}, "", "  ")
	if err != nil {
		s.log.Warn("admin store: failed to encode", "path", s.path, "err", err)
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		s.log.Warn("admin store: failed to write", "path", s.path, "err", err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		s.log.Warn("admin store: failed to rename", "path", s.path, "err", err)
	}
}

// Operators returns a copy of the current operator list.
func (s *AdminStore) Operators() []AdminIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]AdminIdentity(nil), s.ops...)
}

// FirstOperator reports whether no operators are configured yet.
func (s *AdminStore) FirstOperator() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.ops) == 0
}

// PromoteIfFirstOperator grants operator to the player if the operator list is
// empty and reports whether the player was granted. The check and the add are
// atomic so exactly one of several concurrent joiners can claim the slot.
func (s *AdminStore) PromoteIfFirstOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ops) != 0 {
		return false
	}
	s.ops = append(s.ops, AdminIdentity{Name: strings.TrimSpace(name), XUID: strings.TrimSpace(xuid)})
	s.save()
	return true
}

// IsOperator reports whether the player described by name and xuid is an
// operator.
func (s *AdminStore) IsOperator(name, xuid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, o := range s.ops {
		if identityMatches(o, name, xuid) {
			return true
		}
	}
	return false
}

// AddOperator adds the player to the operator list and persists it. It reports
// whether the store changed.
func (s *AdminStore) AddOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isOperatorLocked(name, xuid) {
		return false
	}
	s.ops = append(s.ops, AdminIdentity{Name: strings.TrimSpace(name), XUID: strings.TrimSpace(xuid)})
	s.save()
	return true
}

// RemoveOperator removes the player from the operator list and persists it. It
// reports whether an operator was removed.
func (s *AdminStore) RemoveOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := len(s.ops)
	filtered := s.ops[:0]
	for _, o := range s.ops {
		if !identityMatches(o, name, xuid) {
			filtered = append(filtered, o)
		}
	}
	s.ops = filtered
	if len(s.ops) != before {
		s.save()
		return true
	}
	return false
}

func (s *AdminStore) isOperatorLocked(name, xuid string) bool {
	for _, o := range s.ops {
		if identityMatches(o, name, xuid) {
			return true
		}
	}
	return false
}

// IsBanned returns the active ban for the player, if any.
func (s *AdminStore) IsBanned(name, xuid string) (BanEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.bans {
		if identityMatches(b.Identity, name, xuid) {
			return b, true
		}
	}
	return BanEntry{}, false
}

// Ban bans the player for the given reason and persists the ban. Banning an
// already-banned player resets the reason and timestamp.
func (s *AdminStore) Ban(name, xuid, reason, by string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := BanEntry{
		Identity: AdminIdentity{Name: strings.TrimSpace(name), XUID: strings.TrimSpace(xuid)},
		Reason:   reason,
		By:       by,
		At:       time.Now(),
	}
	filtered := s.bans[:0]
	for _, b := range s.bans {
		if !identityMatches(b.Identity, name, xuid) {
			filtered = append(filtered, b)
		}
	}
	s.bans = append(filtered, entry)
	s.save()
}

// Unban removes any ban matching the player and persists the change. It reports
// whether a ban existed.
func (s *AdminStore) Unban(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := len(s.bans)
	filtered := s.bans[:0]
	for _, b := range s.bans {
		if !identityMatches(b.Identity, name, xuid) {
			filtered = append(filtered, b)
		}
	}
	s.bans = filtered
	if len(s.bans) != before {
		s.save()
		return true
	}
	return false
}

// Bans returns a copy of the active bans.
func (s *AdminStore) Bans() []BanEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]BanEntry(nil), s.bans...)
}

func identityMatches(id AdminIdentity, name, xuid string) bool {
	if id.XUID != "" && xuid != "" && strings.EqualFold(id.XUID, xuid) {
		return true
	}
	return id.Name != "" && strings.EqualFold(id.Name, strings.TrimSpace(name))
}
