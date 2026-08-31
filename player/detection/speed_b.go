package detection

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/game"
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Vanilla air speed values (blocks/tick acceleration).
const (
	airSpeedNoSprint = float32(0.020)
	airSpeedSprint   = float32(0.026)
)

type SpeedB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_SpeedB(p *player.Player) *SpeedB {
	return &SpeedB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 10,
		},
	}
}

func (*SpeedB) Type() string {
	return TypeSpeed
}

func (*SpeedB) SubType() string {
	return "B"
}

func (*SpeedB) Description() string {
	return "Checks if the player is moving faster than the maximum air speed allowed by vanilla physics."
}

func (*SpeedB) Punishable() bool {
	return true
}

func (d *SpeedB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *SpeedB) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	if i.Tick != d.mPlayer.SimulationFrame {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// Only check when the player is airborne and in survival/adventure mode.
	// Creative/spectator modes have their own flight physics that this check can't handle.
	gm := d.mPlayer.GameMode
	if gm == packet.GameTypeCreative || gm == packet.GameTypeSpectator {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	if d.mPlayer.Movement().OnGround() || d.mPlayer.Movement().Flying() || d.mPlayer.Movement().Gliding() ||
		d.mPlayer.Movement().Immobile() || d.mPlayer.Movement().NoClip() || d.mPlayer.Movement().JustDisabledFlight() ||
		d.mPlayer.Movement().HasTeleport() || d.mPlayer.Movement().PendingTeleports() > 0 ||
		d.mPlayer.Movement().TicksSinceTeleport() <= 1 || d.mPlayer.Movement().TicksSinceKnockback() <= 1 ||
		d.mPlayer.Movement().PenetratedLastFrame() || d.mPlayer.Movement().StuckInCollider() {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	if _, hasVehicle := i.ClientPredictedVehicle.Value(); hasVehicle {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// The cheat modifies the in-air movement speed memory values:
	//   Vanilla: 0.020 (no sprint), 0.026 (sprint)
	//   Cheat:   0.020 * multiplier, 0.026 * multiplier
	// At equilibrium, horizontal velocity = airSpeed / (1 - airFriction).
	// Any velocity above the vanilla maximum indicates modified air speed.
	expectedAirSpeed := airSpeedNoSprint
	if d.mPlayer.Movement().Sprinting() {
		expectedAirSpeed = airSpeedSprint
	}

	// Equilibrium horizontal velocity = airSpeed / (1 - friction).
	maxSpeed := expectedAirSpeed / (1 - game.DefaultAirFriction)

	// Generous buffer for slopes, landing, and float imprecision.
	buffer := float32(0.06)

	clientVel := d.mPlayer.Movement().Client().Vel()
	horizontalSpeed := mgl32.Vec2{clientVel[0], clientVel[2]}.Len()

	if horizontalSpeed > maxSpeed+buffer {
		d.mPlayer.FailDetection(d, "speed", horizontalSpeed, "max_speed", maxSpeed+buffer, "sprint", d.mPlayer.Movement().Sprinting())
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}
