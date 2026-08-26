package dragonfly

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/player"
	"github.com/killlime/killlime/player/component"
	playerctx "github.com/killlime/killlime/player/context"
	"github.com/killlime/killlime/player/detection"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func diagPlayer(t *testing.T) (*player.Player, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p := player.New(logger, player.MonitoringState{IsReplay: true, CurrentTime: time.Now()}, nil)
	component.Register(p)
	detection.Register(p)
	p.HandleEvents(player.NewExampleEventHandlerWithAdmin(player.NewAdminStore("", logger)))
	p.Ready = true
	p.Version = protocol.CurrentProtocol
	p.GameMode = packet.GameTypeSurvival
	return p, &buf
}

func diagHandleClient(p *player.Player, pk packet.Packet) {
	ctx := playerctx.NewHandlePacketContext(&pk)
	p.HandleClientPacket(ctx)
}

func diagAuthInput(p *player.Player, tick int64) {
	diagHandleClient(p, &packet.PlayerAuthInput{
		Tick:       uint64(tick),
		Position:   mgl32.Vec3{0, 71.62, 0},
		Delta:      mgl32.Vec3{},
		Yaw:        0,
		HeadYaw:    0,
		Pitch:      0,
		InputMode:  packet.InputModeMouse,
		InputData:  protocol.NewInputFlags(packet.InputFlagCount),
		MoveVector: mgl32.Vec2{},
	})
}

func diagTickInput(p *player.Player, tick int64) {
	diagAuthInput(p, tick)
	p.Tick()
}

func diagFindDetection(p *player.Player, typ, sub string) player.Detection {
	for _, d := range p.Detections() {
		if d.Type() == typ && d.SubType() == sub {
			return d
		}
	}
	return nil
}

// TestDragonflyFlowTimerAFlagsAndLogs reproduces the embedded Dragonfly wiring
// (components + detections + events handler + real time base) and confirms a
// timer cheat reaches the detections and emits the console warning.
func TestDragonflyFlowTimerAFlagsAndLogs(t *testing.T) {
	p, buf := diagPlayer(t)

	// Warm up past the movement rate limiter like the real proxy tick loop.
	diagAuthInput(p, 0)
	p.Tick()

	for i := 0; i < 20; i++ {
		diagAuthInput(p, int64(i))
		diagAuthInput(p, int64(i))
		p.Tick()
	}
	// Window check fires on the next input.
	diagAuthInput(p, 100)
	for i := 0; i < 20; i++ {
		diagAuthInput(p, 100+int64(i))
		diagAuthInput(p, 100+int64(i))
		p.Tick()
	}
	diagAuthInput(p, 200)

	d := diagFindDetection(p, detection.TypeTimer, "A")
	if d == nil {
		t.Fatal("TimerA not registered")
	}
	if vl := d.Metadata().Violations; vl < 1 {
		t.Fatalf("TimerA violations = %v, want >= 1 (40 inputs per 20 tick window)", vl)
	}
	logs := buf.String()
	t.Logf("logs:\n%s", logs)
	if !strings.Contains(logs, "failed detection") {
		t.Fatalf("console (slog) output does not contain \"failed detection\"; got:\n%s", logs)
	}
}

// TestDragonflyFlowReadyViaAckEcho confirms the PlayerInitalized ACK handshake
// that sets p.Ready in the real flow: chunk update -> ack queued -> flush sends
// NSL -> client echoes -> Execute runs the ack -> Ready true.
func TestDragonflyFlowReadyViaAckEcho(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p := player.New(logger, player.MonitoringState{CurrentTime: time.Now()}, nil)
	component.Register(p)
	detection.Register(p)
	p.Version = protocol.CurrentProtocol
	p.GameMode = packet.GameTypeSurvival

	// Server writes a chunk update to the client through the HandleServerPacket
	// path, which queues the PlayerInitalized ack (world component).
	serverPk := packet.Packet(&packet.UpdateSubChunkBlocks{})
	ctx := playerctx.NewHandlePacketContext(&serverPk)
	p.HandleServerPacket(ctx)

	// Deterministic timestamp for the current ack batch so the echo is exact.
	const ts = 123456789
	p.ACKs().SetTimestamp(ts)
	p.ACKs().Flush() // what Tick's !replay path does every 50ms

	if p.ACKs().Pending() != 1 {
		t.Fatalf("pending ack batches = %d, want 1 after flush", p.ACKs().Pending())
	}

	// Client echoes the NSL timestamp back (Bedrock scales to microseconds).
	echoPk := packet.Packet(&packet.NetworkStackLatency{Timestamp: ts * 1_000_000})
	echoCtx := playerctx.NewHandlePacketContext(&echoPk)
	p.HandleClientPacket(echoCtx)

	if !p.Ready {
		t.Fatal("p.Ready still false after NSL echo; ACK handshake broken in Dragonfly flow")
	}
}
