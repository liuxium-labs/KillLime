package dragonfly

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/player"
	"github.com/killlime/killlime/player/component"
	"github.com/killlime/killlime/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TestLiveRakNetDetectsTimerFlood spins up a real RakNet listener wired exactly
// like KillLime's Dragonfly integration, connects a real gophertunnel client,
// and floods PlayerAuthInputs. It proves the full socket path (raknet + login +
// compression + sessionConn routing + Tick loop) emits the console detection
// warning.
func TestLiveRakNetDetectsTimerFlood(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	listenCfg := minecraft.ListenConfig{
		MaximumPlayers:         2,
		StatusProvider:         minecraft.NewStatusProvider("KillLime live test", "KillLime live test"),
		AuthenticationDisabled: true,
		Compression:            packet.SnappyCompression,
	}
	ln, err := listenCfg.Listen("raknet", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	acceptCh := make(chan *minecraft.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		conn, ok := c.(*minecraft.Conn)
		if !ok {
			_ = c.Close()
			acceptErr <- context.Canceled
			return
		}
		acceptCh <- conn
	}()

	// The client dial only completes after the full spawn sequence, which needs
	// the server's StartGameContext (gophertunnel auto-sends RequestChunkRadius
	// on ItemRegistry). Run both concurrently, exactly like a real client join.
	dialErr := make(chan error, 1)
	var client *minecraft.Conn
	go func() {
		c, err := (&minecraft.Dialer{}).DialContext(ctx, "raknet", ln.Addr().String())
		client = c
		dialErr <- err
	}()

	var serverSide *minecraft.Conn
	select {
	case serverSide = <-acceptCh:
	case err := <-acceptErr:
		t.Fatalf("accept: %v", err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for accept")
	}
	defer func() { _ = serverSide.Close() }()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p := player.New(logger.With(
		"name", serverSide.IdentityData().DisplayName,
		"xuid", serverSide.IdentityData().XUID,
	), player.MonitoringState{CurrentTime: time.Now()}, nil)
	p.SetConn(serverSide)
	component.Register(p)
	detection.Register(p)
	p.Ready = true
	sc := newSessionConn(serverSide, p)
	defer func() { _ = sc.Close() }()

	// Server sends StartGame; blocks until the client finishes its spawn
	// sequence (SetLocalPlayerAsInitialised), exactly like Dragonfly's
	// server.go:466 -> sessionConn.StartGameContext. We drive the raw conn
	// directly so we can control the Tick loop ourselves.
	startErr := make(chan error, 1)
	go func() {
		gd := minecraft.GameData{
			EntityUniqueID:  1,
			EntityRuntimeID: 1,
			PlayerGameMode:  packet.GameTypeSurvival,
			WorldGameMode:   packet.GameTypeSurvival,
			PlayerPosition:  mgl32.Vec3{0, 65, 0},
			Dimension:       0,
			WorldName:       "KillLime",
		}
		startErr <- serverSide.StartGameContext(ctx, gd)
	}()
	select {
	case err := <-startErr:
		if err != nil {
			t.Fatalf("start game: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for client spawn")
	}
	select {
	case err := <-dialErr:
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for client dial")
	}

	// Drive the 50ms server Tick loop.
	tickDone := make(chan struct{})
	go func() {
		defer close(tickDone)
		tick := time.NewTicker(time.Millisecond * 50)
		defer tick.Stop()
		for range tick.C {
			if !p.Tick() {
				return
			}
		}
	}()

	// Drive reads like Dragonfly's session.handlePackets: read the real socket
	// packets and feed them to the player exactly like Dragonfly's handler.
	var authInputs int64
	var pumpIn int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			pk, err := sc.ReadPacket()
			if err != nil {
				t.Logf("pump exit: %v", err)
				return
			}
			atomic.AddInt64(&pumpIn, 1)
			if _, ok := pk.(*packet.PlayerAuthInput); ok {
				atomic.AddInt64(&authInputs, 1)
			}
		}
	}()

	// Cheat client: flood far more movement inputs than 20/s supports. Keep the
	// stream alive until at least 3 window boundaries have passed so TimerA's
	// windowed evaluation (which only fires on the input that crosses a boundary)
	// gets a chance to run repeatedly.
	var tickIdx int64
	for p.ServerTick < 70 {
		time.Sleep(10 * time.Millisecond)
		for j := 0; j < 4; j++ {
			if err := client.WritePacket(&packet.PlayerAuthInput{
				Tick:       uint64(tickIdx),
				Position:   mgl32.Vec3{0, 65, 0},
				Delta:      mgl32.Vec3{},
				Yaw:        0,
				HeadYaw:    0,
				Pitch:      0,
				InputMode:  packet.InputModeMouse,
				InputData:  protocol.NewInputFlags(packet.InputFlagCount),
				MoveVector: mgl32.Vec2{},
			}); err != nil {
				t.Fatalf("write auth input: %v", err)
			}
			tickIdx++
		}
	}
	if err := client.Flush(); err != nil {
		t.Logf("client flush: %v", err)
	}

	// Let a couple of 20-tick (1s) windows close before asserting.
	time.Sleep(2000 * time.Millisecond)
	select {
	case <-done:
		t.Log("pump stopped")
	default:
		t.Log("pump still alive")
	}

	logs := buf.String()
	t.Logf("server tick=%d pumpIn=%d authInputs=%d", p.ServerTick, atomic.LoadInt64(&pumpIn), atomic.LoadInt64(&authInputs))
	if d := diagFindDetection(p, detection.TypeTimer, "A"); d != nil {
		t.Logf("TimerA violations=%v buffer=%v", d.Metadata().Violations, d.Metadata().Buffer)
	}
	t.Logf("logs:\n%s", logs)
	if !strings.Contains(logs, "failed detection") {
		t.Fatalf("no \"failed detection\" log through the live RakNet path")
	}
	if d := diagFindDetection(p, detection.TypeTimer, "A"); d == nil || d.Metadata().Violations < 1 {
		t.Fatalf("TimerA violations = %v, want >= 1 after live input flood", func() float64 {
			if d == nil {
				return 0
			}
			return d.Metadata().Violations
		}())
	}
}
