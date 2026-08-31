package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type FlyA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	airTicks    int
	lastAirY    float32
	peakY       float32
}

func New_FlyA(p *player.Player) *FlyA {
	return &FlyA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 5,
		},
	}
}

func (*FlyA) Type() string {
	return "Fly"
}

func (*FlyA) SubType() string {
	return "A"
}

func (*FlyA) Description() string {
	return "Checks if the player is hovering or moving upward without flight permission."
}

func (*FlyA) Punishable() bool {
	return true
}

func (d *FlyA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *FlyA) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	if i.Tick != d.mPlayer.SimulationFrame {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	m := d.mPlayer.Movement()

	// Bypass states where flying is expected or movement is not player-controlled.
	if m.Flying() || m.Immobile() || m.NoClip() ||
		m.HasTeleport() || m.PendingTeleports() > 0 ||
		m.TicksSinceTeleport() <= 2 || m.TicksSinceKnockback() <= 2 ||
		m.PenetratedLastFrame() || m.StuckInCollider() || m.JustDisabledFlight() {
		d.reset()
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// Player riding a vehicle.
	if _, hasVehicle := i.ClientPredictedVehicle.Value(); hasVehicle {
		d.reset()
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// Creative and spectator are exempt.
	gm := d.mPlayer.GameMode
	if gm == packet.GameTypeCreative || gm == packet.GameTypeSpectator {
		d.reset()
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// If player has gravity disabled by the server (effects etc), exempt.
	if !m.HasGravity() {
		d.reset()
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// Player is on the ground — reset.
	if m.OnGround() {
		d.reset()
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	pos := m.Pos()
	d.airTicks++

	// Track the peak Y during this air session.
	if pos[1] > d.peakY {
		d.peakY = pos[1]
	}

	// Allow jump apex to resolve (first few ticks).
	if d.airTicks <= 4 {
		d.lastAirY = pos[1]
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// --- Check 1: Suspicious Y velocity (Motion / Elytra / Pregame fly) ---
	clientVel := m.Client().Vel()
	velY := clientVel[1]

	// In vanilla, a player past jump apex should be falling (velY < 0).
	// Motion/Elytra/Pregame fly set velY to 0 or positive values.
	// Use 0 as threshold — at jump apex velY is briefly ~0 but quickly
	// becomes negative. Cheats sustain velY >= 0.
	if velY > 0 {
		d.mPlayer.FailDetection(d, "vel_y", velY, "air_ticks", d.airTicks, "reason", "suspicious_velocity")
		d.lastAirY = pos[1]
		return
	}

	// --- Check 2: Sustained altitude (Jump fly / repeated air-jumps) ---
	// If the player has been airborne for many ticks and hasn't dropped
	// more than a few blocks from their peak, they're likely jump-flying.
	// A legitimate jump only reaches ~1.25 blocks above ground and falls
	// back within ~12 ticks. After 20+ air ticks, the player should have
	// dropped significantly if gravity is working normally.
	if d.airTicks > 20 {
		dropFromPeak := d.peakY - pos[1]
		// If player hasn't dropped at least 3 blocks from their peak after
		// 20+ ticks in the air, they're sustaining altitude.
		if dropFromPeak < 3.0 {
			d.mPlayer.FailDetection(d, "drop", dropFromPeak, "air_ticks", d.airTicks, "peak", d.peakY, "reason", "sustained_altitude")
			d.lastAirY = pos[1]
			return
		}
	}

	// --- Check 3: Y position not decreasing (slow hover) ---
	// Even with jump-fly oscillations, the net Y change over several ticks
	// should be negative. If the player is at the same height or higher
	// than 6 ticks ago, something is wrong.
	if d.airTicks > 8 {
		yDelta := pos[1] - d.lastAirY
		// After 6 ticks of net zero or positive movement, suspicious.
		if yDelta >= 0 {
			d.mPlayer.FailDetection(d, "y_delta", yDelta, "air_ticks", d.airTicks, "reason", "no_descent")
			d.lastAirY = pos[1]
			return
		}
	}

	d.lastAirY = pos[1]
	d.mPlayer.PassDetection(d, 0.5)
}

func (d *FlyA) reset() {
	d.airTicks = 0
	d.lastAirY = 0
	d.peakY = 0
}
