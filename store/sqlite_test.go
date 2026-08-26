package store

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func tempDB(t *testing.T) *SqliteStore {
	t.Helper()
	s, err := OpenSqlite(":memory:", slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSqliteOperatorCRUD(t *testing.T) {
	s := tempDB(t)

	if !s.FirstOperator() {
		t.Fatal("expected FirstOperator to be true")
	}

	s.AddOperator("Alice", "xuid_a")
	s.AddOperator("Bob", "xuid_b")

	if s.FirstOperator() {
		t.Fatal("expected FirstOperator to be false after adding")
	}

	if !s.IsOperator("Alice", "xuid_a") {
		t.Fatal("expected Alice to be operator")
	}
	if !s.IsOperator("Bob", "xuid_b") {
		t.Fatal("expected Bob to be operator")
	}
	if s.IsOperator("Charlie", "xuid_c") {
		t.Fatal("expected Charlie not to be operator")
	}

	ops := s.Operators()
	if len(ops) != 2 {
		t.Fatalf("expected 2 operators, got %d", len(ops))
	}

	s.RemoveOperator("Alice", "xuid_a")
	if s.IsOperator("Alice", "xuid_a") {
		t.Fatal("expected Alice to not be operator after remove")
	}
	if !s.IsOperator("Bob", "xuid_b") {
		t.Fatal("expected Bob to still be operator")
	}
}

func TestSqlitePromoteIfFirst(t *testing.T) {
	s := tempDB(t)

	if !s.PromoteIfFirstOperator("First", "xuid_1") {
		t.Fatal("expected first promote to succeed")
	}
	if s.PromoteIfFirstOperator("Second", "xuid_2") {
		t.Fatal("expected second promote to fail")
	}
	if !s.IsOperator("First", "xuid_1") {
		t.Fatal("expected First to be operator")
	}
}

func TestSqliteBanUnban(t *testing.T) {
	s := tempDB(t)

	s.Ban("BadGuy", "xuid_bad", "cheating", "admin")
	if ban, ok := s.IsBanned("BadGuy", "xuid_bad"); !ok {
		t.Fatal("expected BadGuy to be banned")
	} else if ban.Reason != "cheating" {
		t.Fatalf("expected reason 'cheating', got %q", ban.Reason)
	}

	bans := s.Bans()
	if len(bans) != 1 {
		t.Fatalf("expected 1 ban, got %d", len(bans))
	}

	if !s.Unban("BadGuy", "xuid_bad") {
		t.Fatal("expected unban to return true")
	}
	if _, ok := s.IsBanned("BadGuy", "xuid_bad"); ok {
		t.Fatal("expected BadGuy to not be banned")
	}
}

func TestSqliteBanTemp(t *testing.T) {
	s := tempDB(t)

	past := time.Now().Add(-1 * time.Hour)
	s.BanTemp("Expired", "xuid_exp", "old", "admin", past)
	if _, ok := s.IsBanned("Expired", "xuid_exp"); ok {
		t.Fatal("expected expired ban to not be active")
	}

	future := time.Now().Add(1 * time.Hour)
	s.BanTemp("Temp", "xuid_temp", "temp ban", "admin", future)
	if _, ok := s.IsBanned("Temp", "xuid_temp"); !ok {
		t.Fatal("expected Temp to be banned")
	}
}

func TestSqlitePersistence(t *testing.T) {
	tmp := t.TempDir() + "/test.db"
	s2, err := OpenSqlite(tmp, nil)
	if err != nil {
		t.Fatal(err)
	}

	s2.AddOperator("FileUser", "xuid_f")
	if !s2.IsOperator("FileUser", "xuid_f") {
		t.Fatal("expected FileUser to be operator after reopen")
	}

	s2.Close()
	s3, err := OpenSqlite(tmp, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()

	if !s3.IsOperator("FileUser", "xuid_f") {
		t.Fatal("expected FileUser to persist across restarts")
	}
}
