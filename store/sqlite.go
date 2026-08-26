package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/killlime/killlime/player"
)

type SqliteStore struct {
	db    *sql.DB
	log   *slog.Logger
	mu    sync.Mutex
	stop  chan struct{}
}

func OpenSqlite(dsn string, log *slog.Logger) (*SqliteStore, error) {
	if log == nil {
		log = slog.Default()
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &SqliteStore{db: db, log: log, stop: make(chan struct{})}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	go s.expiryLoop()
	return s, nil
}

func (s *SqliteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS operators (
			name  TEXT NOT NULL,
			xuid  TEXT NOT NULL PRIMARY KEY
		);
		CREATE TABLE IF NOT EXISTS bans (
			name       TEXT NOT NULL,
			xuid       TEXT NOT NULL PRIMARY KEY,
			reason     TEXT DEFAULT '',
			by_name    TEXT DEFAULT '',
			at         DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_bans_name ON bans(name);
	`)
	return err
}

func (s *SqliteStore) expiryLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.cleanExpired()
		}
	}
}

func (s *SqliteStore) cleanExpiredLocked() {
	_, err := s.db.Exec(`DELETE FROM bans WHERE expires_at IS NOT NULL AND expires_at < ?`, time.Now().UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		s.log.Warn("sqlite: failed to clean expired bans", "err", err)
	}
}

func (s *SqliteStore) cleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanExpiredLocked()
}

func (s *SqliteStore) Operators() []player.AdminIdentity {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT name, xuid FROM operators`)
	if err != nil {
		s.log.Warn("sqlite: query operators", "err", err)
		return nil
	}
	defer rows.Close()
	var out []player.AdminIdentity
	for rows.Next() {
		var id player.AdminIdentity
		if err := rows.Scan(&id.Name, &id.XUID); err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

func (s *SqliteStore) FirstOperator() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM operators`).Scan(&count)
	return count == 0
}

func (s *SqliteStore) PromoteIfFirstOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM operators`).Scan(&count)
	if count != 0 {
		return false
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO operators (name, xuid) VALUES (?, ?)`,
		strings.TrimSpace(name), strings.TrimSpace(xuid))
	if err != nil {
		s.log.Warn("sqlite: insert first operator", "err", err)
		return false
	}
	return true
}

func (s *SqliteStore) IsOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var exists bool
	if xuid != "" {
		s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM operators WHERE xuid = ?)`, xuid).Scan(&exists)
		if exists {
			return true
		}
	}
	s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM operators WHERE LOWER(name) = LOWER(?))`,
		strings.TrimSpace(name)).Scan(&exists)
	return exists
}

func (s *SqliteStore) AddOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isOperatorLocked(name, xuid) {
		return false
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO operators (name, xuid) VALUES (?, ?)`,
		strings.TrimSpace(name), strings.TrimSpace(xuid))
	return err == nil
}

func (s *SqliteStore) RemoveOperator(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeOperatorLocked(name, xuid)
}

func (s *SqliteStore) removeOperatorLocked(name, xuid string) bool {
	if xuid != "" {
		res, _ := s.db.Exec(`DELETE FROM operators WHERE xuid = ?`, xuid)
		n, _ := res.RowsAffected()
		if n > 0 {
			return true
		}
	}
	res, _ := s.db.Exec(`DELETE FROM operators WHERE LOWER(name) = LOWER(?)`, strings.TrimSpace(name))
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *SqliteStore) isOperatorLocked(name, xuid string) bool {
	if xuid != "" {
		var exists bool
		s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM operators WHERE xuid = ?)`, xuid).Scan(&exists)
		if exists {
			return true
		}
	}
	var exists bool
	s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM operators WHERE LOWER(name) = LOWER(?))`,
		strings.TrimSpace(name)).Scan(&exists)
	return exists
}

func (s *SqliteStore) IsBanned(name, xuid string) (player.BanEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()

	query := `SELECT name, xuid, reason, by_name, at, expires_at FROM bans WHERE `
	var args []any
	if xuid != "" {
		query += `xuid = ?`
		args = append(args, xuid)
	} else {
		query += `LOWER(name) = LOWER(?)`
		args = append(args, strings.TrimSpace(name))
	}
	query += ` LIMIT 1`

	var b player.BanEntry
	var expiresAt sql.NullTime
	err := s.db.QueryRow(query, args...).Scan(
		&b.Identity.Name, &b.Identity.XUID, &b.Reason, &b.By, &b.At, &expiresAt,
	)
	if err != nil {
		return player.BanEntry{}, false
	}
	if expiresAt.Valid {
		b.ExpiresAt = &expiresAt.Time
		if time.Now().After(expiresAt.Time) {
			s.deleteBanLocked(b.Identity.XUID)
			return player.BanEntry{}, false
		}
	}
	return b, true
}

func (s *SqliteStore) Ban(name, xuid, reason, by string) {
	s.BanTemp(name, xuid, reason, by, time.Time{})
}

func (s *SqliteStore) BanTemp(name, xuid, reason, by string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var exp sql.NullTime
	if !expiresAt.IsZero() {
		exp = sql.NullTime{Time: expiresAt, Valid: true}
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO bans (name, xuid, reason, by_name, at, expires_at) VALUES (?, ?, ?, ?, datetime('now'), ?)`,
		strings.TrimSpace(name), strings.TrimSpace(xuid), reason, by, exp)
	if err != nil {
		s.log.Warn("sqlite: insert ban", "err", err)
	}
}

func (s *SqliteStore) Unban(name, xuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if xuid != "" {
		res, _ := s.db.Exec(`DELETE FROM bans WHERE xuid = ?`, xuid)
		n, _ := res.RowsAffected()
		if n > 0 {
			return true
		}
	}
	res, _ := s.db.Exec(`DELETE FROM bans WHERE LOWER(name) = LOWER(?)`, strings.TrimSpace(name))
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *SqliteStore) deleteBanLocked(xuid string) {
	if xuid != "" {
		s.db.Exec(`DELETE FROM bans WHERE xuid = ?`, xuid)
	}
}

func (s *SqliteStore) Bans() []player.BanEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanExpiredLocked()
	rows, err := s.db.Query(`SELECT name, xuid, reason, by_name, at, expires_at FROM bans`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []player.BanEntry
	for rows.Next() {
		var b player.BanEntry
		var exp sql.NullTime
		if err := rows.Scan(&b.Identity.Name, &b.Identity.XUID, &b.Reason, &b.By, &b.At, &exp); err != nil {
			continue
		}
		if exp.Valid {
			b.ExpiresAt = &exp.Time
		}
		out = append(out, b)
	}
	return out
}

func (s *SqliteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	return s.db.Close()
}
