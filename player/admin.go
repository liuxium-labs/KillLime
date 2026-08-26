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

const DefaultAdminStorePath = "killlime_admin.json"

type AdminIdentity struct {
	Name string `json:"name"`
	XUID string `json:"xuid"`
}

type BanEntry struct {
	Identity  AdminIdentity `json:"identity"`
	Reason    string        `json:"reason"`
	By        string        `json:"by"`
	At        time.Time     `json:"at"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
}

func (b BanEntry) IsExpired() bool {
	return b.ExpiresAt != nil && time.Now().After(*b.ExpiresAt)
}

type AdminStore interface {
	Operators() []AdminIdentity
	FirstOperator() bool
	PromoteIfFirstOperator(name, xuid string) bool
	IsOperator(name, xuid string) bool
	AddOperator(name, xuid string) bool
	RemoveOperator(name, xuid string) bool
	IsBanned(name, xuid string) (BanEntry, bool)
	Ban(name, xuid, reason, by string)
	BanTemp(name, xuid, reason, by string, expiresAt time.Time)
	Unban(name, xuid string) bool
	Bans() []BanEntry
	Close() error
}

type JSONAdminStore struct {
	mu   sync.RWMutex
	path string
	log  *slog.Logger
	ops  []AdminIdentity
	bans []BanEntry
}

func NewAdminStore(path string, log *slog.Logger) *JSONAdminStore {
	if log == nil {
		log = slog.Default()
	}
	s := &JSONAdminStore{path: path, log: log}
	s.load()
	return s
}

func (s *JSONAdminStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
}

func (s *JSONAdminStore) loadLocked() {
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

func (s *JSONAdminStore) save() {
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

func (s *JSONAdminStore) Operators() []AdminIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]AdminIdentity(nil), s.ops...)
}

func (s *JSONAdminStore) FirstOperator() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.ops) == 0
}

func (s *JSONAdminStore) PromoteIfFirstOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ops) != 0 {
		return false
	}
	s.ops = append(s.ops, AdminIdentity{Name: strings.TrimSpace(name), XUID: strings.TrimSpace(xuid)})
	s.save()
	return true
}

func (s *JSONAdminStore) IsOperator(name, xuid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, o := range s.ops {
		if identityMatches(o, name, xuid) {
			return true
		}
	}
	return false
}

func (s *JSONAdminStore) AddOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isOperatorLocked(name, xuid) {
		return false
	}
	s.ops = append(s.ops, AdminIdentity{Name: strings.TrimSpace(name), XUID: strings.TrimSpace(xuid)})
	s.save()
	return true
}

func (s *JSONAdminStore) RemoveOperator(name, xuid string) bool {
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

func (s *JSONAdminStore) isOperatorLocked(name, xuid string) bool {
	for _, o := range s.ops {
		if identityMatches(o, name, xuid) {
			return true
		}
	}
	return false
}

func (s *JSONAdminStore) IsBanned(name, xuid string) (BanEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.bans {
		if identityMatches(b.Identity, name, xuid) {
			if b.IsExpired() {
				return BanEntry{}, false
			}
			return b, true
		}
	}
	return BanEntry{}, false
}

func (s *JSONAdminStore) Ban(name, xuid, reason, by string) {
	s.BanTemp(name, xuid, reason, by, time.Time{})
}

func (s *JSONAdminStore) BanTemp(name, xuid, reason, by string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := BanEntry{
		Identity: AdminIdentity{Name: strings.TrimSpace(name), XUID: strings.TrimSpace(xuid)},
		Reason:   reason,
		By:       by,
		At:       time.Now(),
	}
	if !expiresAt.IsZero() {
		entry.ExpiresAt = &expiresAt
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

func (s *JSONAdminStore) Unban(name, xuid string) bool {
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

func (s *JSONAdminStore) Bans() []BanEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]BanEntry(nil), s.bans...)
}

func (s *JSONAdminStore) Close() error { return nil }

func identityMatches(id AdminIdentity, name, xuid string) bool {
	if id.XUID != "" && xuid != "" && strings.EqualFold(id.XUID, xuid) {
		return true
	}
	return id.Name != "" && strings.EqualFold(id.Name, strings.TrimSpace(name))
}
