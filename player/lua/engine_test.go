package lua

import (
	"testing"
	"time"
)

func TestEngineNewAndClose(t *testing.T) {
	e := NewEngine(nil)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	e.Close()
}

func TestEngineLoadScript(t *testing.T) {
	e := NewEngine(nil)
	defer e.Close()
	e.LoadScript("test.lua", `x = 1 + 2`)
	if len(e.scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(e.scripts))
	}
}

func TestEngineReload(t *testing.T) {
	e := NewEngine(nil)
	defer e.Close()
	e.LoadScript("test.lua", `y = 42`)
	if err := e.Reload(); err != nil {
		t.Fatal(err)
	}
}

func TestEventToTable(t *testing.T) {
	e := NewEngine(nil)
	defer e.Close()
	ev := &PlayerEvent{
		Name:        "test",
		XUID:        "12345",
		DisplayName: "TestPlayer",
		Version:     2168,
		GameMode:    1,
		InputMode:   1,
		TicksAlive:  100,
		IsOp:        true,
		Perms:       8,
	}
	tbl := e.eventToTable(ev)
	if tbl.RawGetString("name").String() != "test" {
		t.Fatal("name mismatch")
	}
	if tbl.RawGetString("xuid").String() != "12345" {
		t.Fatal("xuid mismatch")
	}
}

func TestCallHandlerMissing(t *testing.T) {
	e := NewEngine(nil)
	defer e.Close()
	e.CallHandler("on_nonexistent", &PlayerEvent{Name: "test"})
}

func TestScriptExecution(t *testing.T) {
	e := NewEngine(nil)
	defer e.Close()

	done := make(chan struct{})
	e.LoadScript("test.lua", `
		_G.test_result = string.format("%s_%d", "hello", 42)
		if timer then
			timer.after(0.01, function()
				_G.timer_fired = true
			end)
		end
	`)

	e.RunAll()
	time.Sleep(200 * time.Millisecond)

	// Check if the script executed (via a follow-up handler call)
	e.CallHandler("on_check", &PlayerEvent{Name: "test"})

	// The script ran asynchronously, just verify no panic
	close(done)
	_ = done
}
