package player

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/event"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testPlayer(t *testing.T) *Player {
	t.Helper()
	return New(testLogger(), MonitoringState{CurrentTime: time.Now()}, nil)
}

func TestAdminStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "killlime_admin.json")
	store := NewAdminStore(path, testLogger())

	if !store.FirstOperator() {
		t.Fatal("expected empty operator list to be first operator")
	}
	if !store.AddOperator("Steve", "xuid-1") {
		t.Fatal("expected AddOperator to report a change")
	}
	if store.AddOperator("Steve", "xuid-1") {
		t.Fatal("expected adding the same operator to be a no-op")
	}
	store.Ban("Alex", "xuid-2", "cheating", "Steve")

	reloaded := NewAdminStore(path, testLogger())
	if reloaded.FirstOperator() {
		t.Fatal("expected operators to survive a reload")
	}
	if !reloaded.IsOperator("steve", "") {
		t.Fatal("expected operator to be found by name")
	}
	if !reloaded.IsOperator("Steve", "xuid-1") {
		t.Fatal("expected operator to be found by xuid")
	}
	if ban, ok := reloaded.IsBanned("ALEX", ""); !ok || ban.Reason != "cheating" {
		t.Fatalf("expected ban to survive reload, got %+v ok=%v", ban, ok)
	}
	if !reloaded.RemoveOperator("Steve", "") {
		t.Fatal("expected RemoveOperator to report a change")
	}
	if !reloaded.Unban("Alex", "xuid-2") {
		t.Fatal("expected Unban to report a change")
	}

	reloaded2 := NewAdminStore(path, testLogger())
	if reloaded2.IsOperator("Steve", "") {
		t.Fatal("expected operator to stay removed")
	}
	if _, ok := reloaded2.IsBanned("Alex", ""); ok {
		t.Fatal("expected ban to stay removed")
	}
}

func TestAdminStoreInMemory(t *testing.T) {
	store := NewAdminStore("", testLogger())
	store.AddOperator("Steve", "xuid-1")
	if !store.IsOperator("Steve", "") {
		t.Fatal("expected in-memory operator")
	}
	store.Ban("Alex", "xuid-2", "", "Steve")
	if _, ok := store.IsBanned("Alex", ""); !ok {
		t.Fatal("expected in-memory ban")
	}
}

func TestAdminStoreBanReplacesMatchingIdentity(t *testing.T) {
	store := NewAdminStore("", testLogger())
	store.Ban("Alex", "xuid-2", "first", "Steve")
	store.Ban("Alex", "xuid-2", "second", "Steve")
	bans := store.Bans()
	if len(bans) != 1 || bans[0].Reason != "second" {
		t.Fatalf("expected the last ban to replace the earlier one, got %+v", bans)
	}
}

func TestAdminStorePromoteIfFirstOperatorConcurrent(t *testing.T) {
	store := NewAdminStore("", testLogger())
	const n = 8
	wg := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			wg <- store.PromoteIfFirstOperator(fmt.Sprintf("P%d", i), fmt.Sprintf("xuid-%d", i))
		}(i)
	}
	promoted := 0
	for i := 0; i < n; i++ {
		if <-wg {
			promoted++
		}
	}
	if promoted != 1 {
		t.Fatalf("expected exactly one promotion out of %d concurrent joins, got %d", n, promoted)
	}
	if ops := store.Operators(); len(ops) != 1 {
		t.Fatalf("expected 1 operator persisted, got %d: %+v", len(ops), ops)
	}
}

func TestExampleEventHandlerFirstJoinerAutoOp(t *testing.T) {
	h := NewExampleEventHandlerWithAdmin(NewAdminStore("", testLogger()))
	p := testPlayer(t)
	p.IdentityDat.DisplayName = "Steve"
	p.IdentityDat.XUID = "xuid-1"
	p.HandleEvents(h)
	if !p.HasPerm(PermissionAdmin) {
		t.Fatal("expected first joiner to be granted operator")
	}
	if !h.admin.IsOperator(p.Name(), p.IdentityDat.XUID) {
		t.Fatal("expected first joiner to be persisted as operator")
	}
}

func TestExampleEventHandlerOpOnJoin(t *testing.T) {
	h := NewExampleEventHandlerWithAdmin(NewAdminStore("", testLogger()))
	h.admin.AddOperator("Steve", "xuid-1")

	p := testPlayer(t)
	p.IdentityDat.DisplayName = "Steve"
	p.IdentityDat.XUID = "xuid-1"
	p.HandleEvents(h)
	if !p.HasPerm(PermissionAdmin) {
		t.Fatal("expected existing operator to keep operator permission on join")
	}

	other := testPlayer(t)
	other.IdentityDat.DisplayName = "Bob"
	other.IdentityDat.XUID = "xuid-9"
	other.HandleEvents(h)
	if other.HasPerm(PermissionAdmin) {
		t.Fatal("expected non-operator player to not have operator permission")
	}
}

func TestExampleEventHandlerBanRejectsJoin(t *testing.T) {
	h := NewExampleEventHandlerWithAdmin(NewAdminStore("", testLogger()))
	h.admin.Ban("Steve", "xuid-1", "cheating", "console")

	p := testPlayer(t)
	p.IdentityDat.DisplayName = "Steve"
	p.IdentityDat.XUID = "xuid-1"
	ctx := event.C(p)
	h.HandleJoin(ctx)
	if !ctx.Cancelled() {
		t.Fatal("expected banned player's join to be cancelled")
	}
	if !p.Closed {
		t.Fatal("expected banned player to be closed")
	}
}

func TestExampleEventHandlerAdminCommands(t *testing.T) {
	h := NewExampleEventHandlerWithAdmin(NewAdminStore("", testLogger()))
	admin := testPlayer(t)
	admin.AddPerm(PermissionAdmin)

	ctx := event.C(admin)

	// op promotes a player (offline here).
	h.HandleCommand(ctx, "op", []string{"Steve"})
	if !h.admin.IsOperator("Steve", "") {
		t.Fatal("expected /ac op to record the operator")
	}

	// op on the same name again is a no-op and reports it.
	h.HandleCommand(ctx, "op", []string{"Steve"})
	ops := h.admin.Operators()
	if len(ops) != 1 {
		t.Fatalf("expected a single operator entry, got %d", len(ops))
	}

	// deop revokes the operator.
	h.HandleCommand(ctx, "deop", []string{"steve"})
	if h.admin.IsOperator("Steve", "") {
		t.Fatal("expected /ac deop to remove the operator")
	}

	// ban records the ban.
	h.HandleCommand(ctx, "ban", []string{"Alex", "cheating"})
	if _, ok := h.admin.IsBanned("Alex", ""); !ok {
		t.Fatal("expected /ac ban to record the ban")
	}

	// unban removes it.
	h.HandleCommand(ctx, "unban", []string{"alex"})
	if _, ok := h.admin.IsBanned("Alex", ""); ok {
		t.Fatal("expected /ac unban to remove the ban")
	}
}

func TestExampleEventHandlerKickOnline(t *testing.T) {
	h := NewExampleEventHandlerWithAdmin(NewAdminStore("", testLogger()))
	admin := testPlayer(t)
	admin.AddPerm(PermissionAdmin)

	target := testPlayer(t)
	target.IdentityDat.DisplayName = "victim"
	h.pMu.Lock()
	h.connected["victim"] = target
	h.pMu.Unlock()

	ctx := event.C(admin)
	h.HandleCommand(ctx, "kick", []string{"Victim", "test kick"})
	if !target.Closed {
		t.Fatal("expected kicked player to be closed")
	}
}

func TestExampleEventHandlerPermissionDenied(t *testing.T) {
	h := NewExampleEventHandlerWithAdmin(NewAdminStore("", testLogger()))
	admin := testPlayer(t)

	ctx := event.C(admin)
	h.HandleCommand(ctx, "ban", []string{"Alex"})
	if !ctx.Cancelled() {
		t.Fatal("expected command ctx to be cancelled for missing permission")
	}
	if _, ok := h.admin.IsBanned("Alex", ""); ok {
		t.Fatal("expected no ban to be recorded without operator permission")
	}
}
