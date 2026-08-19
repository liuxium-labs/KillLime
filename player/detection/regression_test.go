package detection

import (
	"log/slog"
	"testing"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/entity"
	"github.com/killlime/killlime/player"
	"github.com/killlime/killlime/player/component"
	playerctx "github.com/killlime/killlime/player/context"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func newTestPlayer(t *testing.T) *player.Player {
	t.Helper()
	p := player.New(slog.Default(), player.MonitoringState{IsReplay: true, CurrentTime: time.Now()}, nil)
	component.Register(p)
	Register(p)
	p.Ready = true
	p.Version = protocol.CurrentProtocol
	p.GameMode = packet.GameTypeSurvival
	// The very first input is rejected by the movement rate limiter
	// (allowedInputs starts at 0). Tick the server once to grant allowance,
	// mirroring how StartTicking runs in the real proxy.
	sendAuthInput(p, 0)
	p.Tick()
	return p
}

func handleClient(p *player.Player, pk packet.Packet) {
	ctx := playerctx.NewHandlePacketContext(&pk)
	p.HandleClientPacket(ctx)
}

// sendAuthInput simulates the client sending a PlayerAuthInput packet.
func sendAuthInput(p *player.Player, tick int64) {
	handleClient(p, &packet.PlayerAuthInput{
		Tick:      uint64(tick),
		Position:  mgl32.Vec3{0, 71.62, 0},
		Delta:     mgl32.Vec3{},
		Yaw:       0,
		HeadYaw:   0,
		Pitch:     0,
		InputMode: packet.InputModeMouse,
		InputData: protocol.NewBitset(packet.PlayerAuthInputBitsetSize),
		MoveVector: mgl32.Vec2{},
	})
}

// tickInput sends a PlayerAuthInput and then ticks the server. The movement
// rate limiter only grants one input allowance per server tick, so inputs must
// be interleaved with ticks to actually be processed (and advance
// SimulationFrame) in the same way as in the real proxy.
func tickInput(p *player.Player, tick int64) {
	sendAuthInput(p, tick)
	p.Tick()
}

func sendAttack(p *player.Player, rid uint64) {
	handleClient(p, &packet.InventoryTransaction{
		TransactionData: &protocol.UseItemOnEntityTransactionData{
			TargetEntityRuntimeID: rid,
			ActionType:            protocol.UseItemOnEntityActionAttack,
			HotBarSlot:            0,
		},
	})
}

func violations(d player.Detection) float64 {
	return d.Metadata().Violations
}

func findDetection(p *player.Player, typ, sub string) player.Detection {
	for _, d := range p.Detections() {
		if d.Type() == typ && d.SubType() == sub {
			return d
		}
	}
	return nil
}

// TestKillauraAFlagsAttackWithoutSwing verifies that attacking without ever
// swinging the arm flags KillauraA in survival mode.
func TestKillauraAFlagsAttackWithoutSwing(t *testing.T) {
	p := newTestPlayer(t)
	// Warm up past the default 10 tick grace period.
	for tick := int64(1); tick <= 15; tick++ {
		tickInput(p, tick)
	}
	// Killaura: attack without swinging.
	sendAttack(p, 999)
	tickInput(p, 16)

	d := findDetection(p, TypeKillaura, "A")
	if d == nil {
		t.Fatal("KillauraA not registered")
	}
	if vl := violations(d); vl < 1 {
		t.Fatalf("KillauraA violations = %v, want >= 1 (attack without swing should flag)", vl)
	}
}

// TestKillauraAFlagsAttackWithoutSwingCreative verifies killaura also flags in
// creative mode, since the check is not gamemode dependent.
func TestKillauraAFlagsAttackWithoutSwingCreative(t *testing.T) {
	p := newTestPlayer(t)
	p.GameMode = packet.GameTypeCreative
	for tick := int64(1); tick <= 15; tick++ {
		tickInput(p, tick)
	}
	sendAttack(p, 999)
	tickInput(p, 16)

	d := findDetection(p, TypeKillaura, "A")
	if vl := violations(d); vl < 1 {
		t.Fatalf("KillauraA violations = %v, want >= 1 in creative", vl)
	}
}

// TestReachBFlagsFarAttackSurvival verifies attacking a player entity 15 blocks
// away flags ReachB in survival mode.
func TestReachBFlagsFarAttackSurvival(t *testing.T) {
	p := newTestPlayer(t)
	logger := p.Log()
	e := entity.New(100, "minecraft:player", nil, mgl32.Vec3{15, 70, 0}, 6, true, 0.6, 1.8, 1.0, &logger)
	p.ClientEntityTracker().AddEntity(100, e)
	p.EntityTracker().AddEntity(100, e)
	for tick := int64(1); tick <= 25; tick++ {
		tickInput(p, tick)
	}
	// ReachB needs two fails to exceed its FailBuffer of 1.01.
	for i := 0; i < 3; i++ {
		sendAttack(p, 100)
		tickInput(p, 26+int64(i))
	}

	d := findDetection(p, TypeReach, "B")
	if d == nil {
		t.Fatal("ReachB not registered")
	}
	if vl := violations(d); vl < 1 {
		t.Fatalf("ReachB violations = %v, want >= 1 (15 block attack should flag)", vl)
	}
}

// TestReachBFlagsFarAttackCreative verifies reach checks run in creative mode as
// well, since a 15 block attack is beyond even the creative reach limit.
func TestReachBFlagsFarAttackCreative(t *testing.T) {
	p := newTestPlayer(t)
	p.GameMode = packet.GameTypeCreative
	logger := p.Log()
	e := entity.New(100, "minecraft:player", nil, mgl32.Vec3{15, 70, 0}, 6, true, 0.6, 1.8, 1.0, &logger)
	p.ClientEntityTracker().AddEntity(100, e)
	p.EntityTracker().AddEntity(100, e)
	for tick := int64(1); tick <= 25; tick++ {
		tickInput(p, tick)
	}
	for i := 0; i < 3; i++ {
		sendAttack(p, 100)
		tickInput(p, 26+int64(i))
	}

	d := findDetection(p, TypeReach, "B")
	if vl := violations(d); vl < 1 {
		t.Fatalf("ReachB violations = %v, want >= 1 in creative (15 block attack should flag)", vl)
	}
}

// TestReachBFlagsCrystalAttackSurvival verifies attacking a non-player entity
// (e.g. an end crystal) from 15 blocks away flags ReachB in survival mode.
func TestReachBFlagsCrystalAttackSurvival(t *testing.T) {
	p := newTestPlayer(t)
	logger := p.Log()
	e := entity.New(100, "minecraft:end_crystal", nil, mgl32.Vec3{15, 70, 0}, 6, false, 0.6, 1.8, 1.0, &logger)
	p.ClientEntityTracker().AddEntity(100, e)
	p.EntityTracker().AddEntity(100, e)
	for tick := int64(1); tick <= 25; tick++ {
		tickInput(p, tick)
	}
	for i := 0; i < 3; i++ {
		sendAttack(p, 100)
		tickInput(p, 26+int64(i))
	}

	d := findDetection(p, TypeReach, "B")
	if vl := violations(d); vl < 1 {
		t.Fatalf("ReachB violations = %v, want >= 1 (15 block crystal attack should flag)", vl)
	}
}

// TestNukerARegistered verifies the nuker detection is registered.
func TestNukerARegistered(t *testing.T) {
	p := newTestPlayer(t)
	if d := findDetection(p, TypeNuker, "A"); d == nil {
		t.Fatal("NukerA is not registered")
	}
}

// TestNukerAFlagsBurstBreak verifies that a single PlayerAuthInput carrying more
// than 3 block-breaking actions flags NukerA (a nuker breaking 8 blocks per
// tick).
func TestNukerAFlagsBurstBreak(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeNuker, "A")
	if d == nil {
		t.Fatal("NukerA is not registered")
	}
	// MaxViolations=1 would punish on the first flag, which panics in replay
	// mode. Raise it so the test can observe the violation instead.
	d.Metadata().MaxViolations = 100
	actions := make([]protocol.PlayerBlockAction, 0, 8)
	for i := 0; i < 8; i++ {
		actions = append(actions, protocol.PlayerBlockAction{
			Action:   protocol.PlayerActionPredictDestroyBlock,
			BlockPos: protocol.BlockPos{int32(i), 70, 0},
		})
	}
	d.Detect(&packet.PlayerAuthInput{
		Tick:         100,
		InputData:    protocol.NewBitset(packet.PlayerAuthInputBitsetSize),
		BlockActions: actions,
	})
	if vl := violations(d); vl < 1 {
		t.Fatalf("NukerA violations = %v, want >= 1 (8 block breaks in one tick should flag)", vl)
	}
}

// TestScaffoldARegistered verifies the scaffold detection is registered.
func TestScaffoldARegistered(t *testing.T) {
	p := newTestPlayer(t)
	if d := findDetection(p, TypeScaffold, "A"); d == nil {
		t.Fatal("ScaffoldA is not registered")
	}
}

// TestTimerAFlagsOverInputs verifies TimerA flags when the client sends more
// inputs than the server tick rate allows. Rate-limited inputs now still reach
// the detections, so TimerA sees every input regardless of the rate limiter.
func TestTimerAFlagsOverInputs(t *testing.T) {
	p := newTestPlayer(t)
	// Round 1: 40 inputs over 20 server ticks (double the legit rate).
	for i := 0; i < 20; i++ {
		sendAuthInput(p, int64(i))
		sendAuthInput(p, int64(i))
		p.Tick()
	}
	// Window check fires on the next input: 41 inputs > 25 -> first fail.
	sendAuthInput(p, 100)
	// Round 2: another 40 inputs over 20 ticks -> second fail -> violation.
	for i := 0; i < 20; i++ {
		sendAuthInput(p, 100+int64(i))
		sendAuthInput(p, 100+int64(i))
		p.Tick()
	}
	sendAuthInput(p, 200)

	d := findDetection(p, TypeTimer, "A")
	if d == nil {
		t.Fatal("TimerA not registered")
	}
	if vl := violations(d); vl < 1 {
		t.Fatalf("TimerA violations = %v, want >= 1 (40 inputs per 20 tick window should flag)", vl)
	}
}
