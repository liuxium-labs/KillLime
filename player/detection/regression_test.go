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
		InputData: protocol.NewInputFlags(packet.InputFlagCount),
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
		InputData:    protocol.NewInputFlags(packet.InputFlagCount),
		BlockActions: protocol.Option(actions),
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

// tickInputAt sends a PlayerAuthInput at the given position and ticks the
// server, mirroring tickInput but with an arbitrary reported position.
func tickInputAt(p *player.Player, tick int64, pos mgl32.Vec3) {
	auth := &packet.PlayerAuthInput{
		Tick:      uint64(tick),
		Position:  pos,
		InputMode: packet.InputModeMouse,
		InputData: protocol.NewInputFlags(packet.InputFlagCount),
	}
	handleClient(p, auth)
	p.Tick()
}

// inputWithData builds a PlayerAuthInput with the given input flags set.
func inputWithData(flags ...int) *packet.PlayerAuthInput {
	bits := protocol.NewInputFlags(packet.InputFlagCount)
	for _, f := range flags {
		bits.Set(f)
	}
	return &packet.PlayerAuthInput{
		Tick:      0,
		InputMode: packet.InputModeMouse,
		InputData: bits,
	}
}

// raiseMaxViolations prevents a detection from punishing (disconnecting) mid
// test, which would otherwise happen since replay players disconnect on punish.
func raiseMaxViolations(d player.Detection) {
	d.Metadata().MaxViolations = 100
}

// TestBadPacketOLegitJumpPressNotFlagged verifies that a normal jump press
// does not flag BadPacketO. The detection is currently disabled because
// JUMP_DOWN semantics changed in Bedrock 1.26.x.
func TestBadPacketOLegitJumpPressNotFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "O")
	if d == nil {
		t.Fatal("BadPacketO not registered")
	}
	cycle := [][]int{
		{packet.InputFlagWantUp, packet.InputFlagJumping, packet.InputFlagJumpDown}, // press
		{packet.InputFlagWantUp, packet.InputFlagJumping},                           // hold
		{packet.InputFlagWantUp, packet.InputFlagJumping},                           // hold
		{},                                                                          // release
	}
	tick := int64(1)
	for i := 0; i < 3; i++ {
		for _, flags := range cycle {
			auth := inputWithData(flags...)
			auth.Tick = uint64(tick)
			handleClient(p, auth)
			p.Tick()
			tick++
		}
	}
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketO violations = %v, want 0 (legit jump presses must not flag)", vl)
	}
}

// TestBadPacketOForcedJumpFlagsFlagged verifies that forced jump flags don't
// cause violations. The detection is disabled because JUMP_DOWN semantics
// changed in Bedrock 1.26.x.
func TestBadPacketOForcedJumpFlagsFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "O")
	if d == nil {
		t.Fatal("BadPacketO not registered")
	}
	for i := 0; i < 3; i++ {
		auth := inputWithData(packet.InputFlagWantUp, packet.InputFlagJumping, packet.InputFlagJumpDown)
		auth.Tick = uint64(i + 1)
		handleClient(p, auth)
		p.Tick()
	}
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketO violations = %v, want 0 (detection disabled for 1.26.x)", vl)
	}
}

// TestBadPacketCPlayerActionBreak flags block breaking via PlayerAction on
// 1.26.40+, where all block breaking is routed through PlayerAuthInput.
func TestBadPacketCPlayerActionBreak(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "C")
	raiseMaxViolations(d)
	d.Detect(&packet.PlayerAction{ActionType: protocol.PlayerActionStopBreak})
	if vl := violations(d); vl == 0 {
		t.Fatalf("BadPacketC violations = %v, want >= 1 (PlayerAction break should flag on 1.26.40+)", vl)
	}
}

// TestBadPacketCCreativeDestroyFlagsInSurvival verifies that a creative-only
// block destroy action sent outside of creative mode flags BadPacketC.
func TestBadPacketCCreativeDestroyFlagsInSurvival(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "C")
	raiseMaxViolations(d)
	d.Detect(&packet.PlayerAuthInput{
		InputData: protocol.NewInputFlags(packet.InputFlagCount),
		BlockActions: protocol.Option([]protocol.PlayerBlockAction{{
			Action:   protocol.PlayerActionCreativePlayerDestroyBlock,
			BlockPos: protocol.BlockPos{0, 70, 0},
		}}),
	})
	if vl := violations(d); vl < 1 {
		t.Fatalf("BadPacketC violations = %v, want >= 1 (creative destroy in survival should flag)", vl)
	}
}

// TestBadPacketCCreativeDestroyAllowedInCreative verifies the same action is
// not flagged when the player actually is in creative mode.
func TestBadPacketCCreativeDestroyAllowedInCreative(t *testing.T) {
	p := newTestPlayer(t)
	p.GameMode = packet.GameTypeCreative
	d := findDetection(p, TypeBadPacket, "C")
	d.Detect(&packet.PlayerAuthInput{
		InputData: protocol.NewInputFlags(packet.InputFlagCount),
		BlockActions: protocol.Option([]protocol.PlayerBlockAction{{
			Action:   protocol.PlayerActionCreativePlayerDestroyBlock,
			BlockPos: protocol.BlockPos{0, 70, 0},
		}}),
	})
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketC violations = %v, want 0 (creative destroy in creative must not flag)", vl)
	}
}

// TestBadPacketCItemInteractionBreakFlagged verifies an item interaction break
// action in survival mode flags BadPacketC.
func TestBadPacketCItemInteractionBreakFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "C")
	raiseMaxViolations(d)
	d.Detect(&packet.PlayerAuthInput{
		InputData:           bitSetWith(packet.InputFlagPerformItemInteraction),
		ItemInteractionData: protocol.Option(protocol.UseItemTransactionData{ActionType: protocol.UseItemActionBreakBlock}),
	})
	if vl := violations(d); vl < 1 {
		t.Fatalf("BadPacketC violations = %v, want >= 1 (item interaction break in survival should flag)", vl)
	}
}

// TestBadPacketCItemInteractionClickBlockAllowed verifies normal block clicks
// are not flagged.
func TestBadPacketCItemInteractionClickBlockAllowed(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "C")
	d.Detect(&packet.PlayerAuthInput{
		InputData:           bitSetWith(packet.InputFlagPerformItemInteraction),
		ItemInteractionData: protocol.Option(protocol.UseItemTransactionData{ActionType: protocol.UseItemActionClickBlock}),
	})
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketC violations = %v, want 0 (normal block click must not flag)", vl)
	}
}

// bitSetWith returns a fresh PlayerAuthInput InputFlags with the given flag set.
func bitSetWith(flag int) protocol.InputFlags {
	bits := protocol.NewInputFlags(packet.InputFlagCount)
	bits.Set(flag)
	return bits
}

// TestBadPacketNSpawnFarFromOriginAllowed verifies that a player who spawns far
// from the world origin is not flagged by BadPacketN's position distance check.
// The baseline must come from the first accepted input, never the zero vector.
func TestBadPacketNSpawnFarFromOriginAllowed(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "N")
	if d == nil {
		t.Fatal("BadPacketN not registered")
	}
	// First accepted input establishes the baseline far from the origin.
	tickInputAt(p, 1, mgl32.Vec3{100, 71.62, 100})
	// A small legitimate step.
	tickInputAt(p, 2, mgl32.Vec3{100.5, 71.62, 100})
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketN violations = %v, want 0 (spawn far from origin must not flag)", vl)
	}
}

// TestBadPacketNFakeTeleportFlagged verifies that an injected position jump of
// more than 50 blocks without a server-authorized teleport flags BadPacketN.
func TestBadPacketNFakeTeleportFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "N")
	if d == nil {
		t.Fatal("BadPacketN not registered")
	}
	tickInputAt(p, 1, mgl32.Vec3{100, 71.62, 100})
	tickInputAt(p, 2, mgl32.Vec3{100.5, 71.62, 100})
	for i := 0; i < 3; i++ {
		tickInputAt(p, int64(3+i), mgl32.Vec3{250, 71.62, 100})
	}
	if vl := violations(d); vl < 1 {
		t.Fatalf("BadPacketN violations = %v, want >= 1 (150 block jump should flag)", vl)
	}
}

// TestBadPacketPGlideStartLevelAllowed verifies a start-glide input with a
// level or descending delta (the only legitimate ways to start gliding) is not
// flagged.
func TestBadPacketPGlideStartLevelAllowed(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "P")
	if d == nil {
		t.Fatal("BadPacketP not registered")
	}
	for _, delta := range []mgl32.Vec3{{2, 0, 0}, {0, -3, 0}, {0, 0, 0}} {
		auth := inputWithData(packet.InputFlagStartGliding)
		auth.Delta = delta
		d.Detect(auth)
	}
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketP violations = %v, want 0 (non-ascending glide start must not flag)", vl)
	}
}

// TestBadPacketPGlideStartAscendingFlagged verifies a start-glide input while
// clearly ascending flags BadPacketP.
func TestBadPacketPGlideStartAscendingFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "P")
	if d == nil {
		t.Fatal("BadPacketP not registered")
	}
	auth := inputWithData(packet.InputFlagStartGliding)
	auth.Delta = mgl32.Vec3{0, 0.5, 0}
	d.Detect(auth)
	if vl := violations(d); vl < 1 {
		t.Fatalf("BadPacketP violations = %v, want >= 1 (ascending glide start should flag)", vl)
	}
}

// TestBadPacketQTerminalVelocityBoundary verifies the terminal velocity margin:
// a fall exactly at the vanilla cap is allowed, but a clearly faster descent is
// flagged.
func TestBadPacketQTerminalVelocityBoundary(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "Q")
	if d == nil {
		t.Fatal("BadPacketQ not registered")
	}
	// Warm up so TicksSinceKnockback is no longer in its exemption window.
	for i := 0; i < 5; i++ {
		tickInput(p, int64(i+1))
	}
	// Exactly at the cap (with margin rounds correctly to not flag).
	for i := 0; i < 3; i++ {
		auth := inputWithData()
		auth.Tick = uint64(100 + i)
		auth.Delta = mgl32.Vec3{0, -3.95, 0}
		d.Detect(auth)
	}
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketQ violations = %v, want 0 (terminal velocity fall must not flag)", vl)
	}
	// Far beyond the cap.
	for i := 0; i < 3; i++ {
		auth := inputWithData()
		auth.Tick = uint64(200 + i)
		auth.Delta = mgl32.Vec3{0, -10, 0}
		d.Detect(auth)
	}
	if vl := violations(d); vl < 1 {
		t.Fatalf("BadPacketQ violations = %v, want >= 1 (fast fall should flag)", vl)
	}
}

// TestBadPacketKSingleSpinToggleAllowed verifies a single same-tick
// spin start+stop (possible on fast taps) is not flagged.
func TestBadPacketKSingleSpinToggleAllowed(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "K")
	if d == nil {
		t.Fatal("BadPacketK not registered")
	}
	auth := inputWithData(packet.InputFlagStartSpinAttack, packet.InputFlagStopSpinAttack)
	d.Detect(auth)
	if vl := violations(d); vl != 0 {
		t.Fatalf("BadPacketK violations = %v, want 0 (single spin toggle must not flag)", vl)
	}
}

// TestBadPacketKRepeatedSpinToggleFlagged verifies the repeated same-tick spin
// start+stop pattern flags BadPacketK. MaxViolations is 1 so the third call
// triggers a disconnect panic in replay mode — we catch it.
func TestBadPacketKRepeatedSpinToggleFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeBadPacket, "K")
	if d == nil {
		t.Fatal("BadPacketK not registered")
	}
	for i := 0; i < 3; i++ {
		auth := inputWithData(packet.InputFlagStartSpinAttack, packet.InputFlagStopSpinAttack)
		func() {
			defer func() { recover() }()
			d.Detect(auth)
		}()
	}
	if vl := violations(d); vl < 1 {
		t.Fatalf("BadPacketK violations = %v, want >= 1 (repeated spin toggle should flag)", vl)
	}
}

// TestScaffoldAClickAirWithZeroPosAllowed verifies an initial right click on
// air (eating, using an item) with a zero click point is not flagged. Only a
// BLOCK click with a zeroed click point is impossible.
func TestScaffoldAClickAirWithZeroPosAllowed(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeScaffold, "A")
	if d == nil {
		t.Fatal("ScaffoldA not registered")
	}
	d.Detect(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{
		ActionType:      protocol.UseItemActionClickAir,
		TriggerType:     protocol.TriggerTypePlayerInput,
		ClickedPosition: mgl32.Vec3{},
	}})
	if vl := violations(d); vl != 0 {
		t.Fatalf("ScaffoldA violations = %v, want 0 (click-air with zero position must not flag)", vl)
	}
}

// TestScaffoldAClickBlockZeroPosFlagged verifies a block click carrying a zero
// clicked position flags ScaffoldA.
func TestScaffoldAClickBlockZeroPosFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeScaffold, "A")
	if d == nil {
		t.Fatal("ScaffoldA not registered")
	}
	raiseMaxViolations(d)
	d.Detect(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{
		ActionType:      protocol.UseItemActionClickBlock,
		TriggerType:     protocol.TriggerTypePlayerInput,
		ClickedPosition: mgl32.Vec3{},
	}})
	if vl := violations(d); vl < 1 {
		t.Fatalf("ScaffoldA violations = %v, want >= 1 (block click with zero position should flag)", vl)
	}
}

// TestNukerABreakTransactionFlagsNewVersion verifies a break_block item
// transaction flags NukerA on 1.21.20+ clients, where vanilla clients never
// send such transactions.
func TestNukerABreakTransactionFlagsNewVersion(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeNuker, "A")
	raiseMaxViolations(d)
	d.Detect(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{
		ActionType: protocol.UseItemActionBreakBlock,
	}})
	if vl := violations(d); vl < 1 {
		t.Fatalf("NukerA violations = %v, want >= 1 (break_block transaction on new version should flag)", vl)
	}
}

// TestNukerABreakTransaction flags break_block transactions on 1.26.40+,
// since vanilla clients never send them on modern versions.
func TestNukerABreakTransaction(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeNuker, "A")
	raiseMaxViolations(d)
	d.Detect(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{
		ActionType: protocol.UseItemActionBreakBlock,
	}})
	if vl := violations(d); vl == 0 {
		t.Fatalf("NukerA violations = %v, want >= 1 (break_block transaction should flag on 1.26.40+)", vl)
	}
}

// TestInvMoveAGUIRequestWhileMovingFlagged verifies that an item stack request
// touching inventory-slots while the player is moving flags InvMoveA.
func TestInvMoveAGUIRequestWhileMovingFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeInvMove, "A")
	if d == nil {
		t.Fatal("InvMoveA not registered")
	}
	raiseMaxViolations(d)
	// Start moving: the accepted input sets a non-zero impulse.
	moving := func(tick int64) {
		auth := &packet.PlayerAuthInput{
			Tick:      uint64(tick),
			Position:  mgl32.Vec3{0, 71.62, 0},
			InputMode: packet.InputModeMouse,
			InputData: protocol.NewInputFlags(packet.InputFlagCount),
			MoveVector: mgl32.Vec2{0.5, 0},
		}
		handleClient(p, auth)
		p.Tick()
	}
	moving(1)
	// A request moving an item out of the inventory (not the hotbar) while
	// moving. d.Detect is called directly so the test does not depend on the
	// inventory solver running.
	guiTake := &protocol.TakeStackRequestAction{}
	guiTake.Source = protocol.StackRequestSlotInfo{Container: protocol.FullContainerName{ContainerID: protocol.ContainerInventory}, Slot: 20, StackNetworkID: 1}
	guiTake.Destination = protocol.StackRequestSlotInfo{Container: protocol.FullContainerName{ContainerID: protocol.ContainerHotBar}, Slot: 0, StackNetworkID: 1}
	d.Detect(&packet.ItemStackRequest{Requests: []protocol.ItemStackRequest{{
		Actions: []protocol.StackRequestAction{guiTake},
	}}})
	// Still moving on the next input: the pre-flag must turn into a violation.
	moving(2)
	if vl := violations(d); vl < 1 {
		t.Fatalf("InvMoveA violations = %v, want >= 1 (inventory move while moving should flag)", vl)
	}
}

// TestInvMoveAHotBarSwapAllowed verifies that swapping items between hotbar
// slots while moving is not flagged (vanilla clients do this freely).
func TestInvMoveAHotBarSwapAllowed(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeInvMove, "A")
	if d == nil {
		t.Fatal("InvMoveA not registered")
	}
	moving := func(tick int64) {
		auth := &packet.PlayerAuthInput{
			Tick:       uint64(tick),
			Position:   mgl32.Vec3{0, 71.62, 0},
			InputMode:  packet.InputModeMouse,
			InputData:  protocol.NewInputFlags(packet.InputFlagCount),
			MoveVector: mgl32.Vec2{0.5, 0},
		}
		handleClient(p, auth)
		p.Tick()
	}
	moving(1)
	hotbarSwap := &protocol.TakeStackRequestAction{}
	hotbarSwap.Source = protocol.StackRequestSlotInfo{Container: protocol.FullContainerName{ContainerID: protocol.ContainerHotBar}, Slot: 1, StackNetworkID: 1}
	hotbarSwap.Destination = protocol.StackRequestSlotInfo{Container: protocol.FullContainerName{ContainerID: protocol.ContainerHotBar}, Slot: 2, StackNetworkID: 1}
	d.Detect(&packet.ItemStackRequest{Requests: []protocol.ItemStackRequest{{
		Actions: []protocol.StackRequestAction{hotbarSwap},
	}}})
	moving(2)
	if vl := violations(d); vl != 0 {
		t.Fatalf("InvMoveA violations = %v, want 0 (hotbar swap while moving must not flag)", vl)
	}
}

// sendAuthInputDelta simulates the client sending a PlayerAuthInput packet
// with the given movement delta.
func sendAuthInputDelta(p *player.Player, tick int64, delta mgl32.Vec3) {
	handleClient(p, &packet.PlayerAuthInput{
		Tick:      uint64(tick),
		Position:  mgl32.Vec3{0, 71.62, 0},
		Delta:     delta,
		InputMode: packet.InputModeMouse,
		InputData: protocol.NewInputFlags(packet.InputFlagCount),
	})
	p.Tick()
}

// TestSpeedALegitWalkNotFlagged verifies a player reporting the vanilla walk
// speed on solid ground does not flag SpeedA. The equilibrium ground speed is
// moveRelative/(1-friction) ~= 2.20 * movementSpeed (0.22 b/t walking), not
// friction*moveRelative/(1-friction), which would only cover the post-friction
// velocity and falsely flag every walking player.
func TestSpeedALegitWalkNotFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeSpeed, "A")
	if d == nil {
		t.Fatal("SpeedA not registered")
	}
	// Warm up so teleport/knockback exemptions clear.
	for tick := int64(1); tick <= 5; tick++ {
		tickInput(p, tick)
	}
	p.Movement().SetOnGround(true)
	// Vanilla walk speed is 0.2159 b/t; report exactly that.
	for tick := int64(6); tick <= 10; tick++ {
		p.Movement().SetOnGround(true)
		sendAuthInputDelta(p, tick, mgl32.Vec3{0, 0, 0.215})
	}
	if vl := violations(d); vl != 0 {
		t.Fatalf("SpeedA violations = %v, want 0 (vanilla walk speed must not flag)", vl)
	}
}

//TestSpeedASuperhumanSpeedFlagged verifies a player reporting well beyond the
// vanilla sprint speed while on the ground flags SpeedA.
func TestSpeedASuperhumanSpeedFlagged(t *testing.T) {
	p := newTestPlayer(t)
	d := findDetection(p, TypeSpeed, "A")
	if d == nil {
		t.Fatal("SpeedA not registered")
	}
	for tick := int64(1); tick <= 5; tick++ {
		tickInput(p, tick)
	}
	p.Movement().SetOnGround(true)
	// 0.5 b/t is nearly twice the max sprint speed and unreachable on foot.
	for tick := int64(6); tick <= 10; tick++ {
		p.Movement().SetOnGround(true)
		sendAuthInputDelta(p, tick, mgl32.Vec3{0, 0, 0.5})
	}
	if vl := violations(d); vl < 1 {
		t.Fatalf("SpeedA violations = %v, want >= 1 (superhuman walk speed should flag)", vl)
	}
}
