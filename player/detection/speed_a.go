package detection

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/game"
	"github.com/killlime/killlime/player"
	"github.com/killlime/killlime/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type SpeedA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_SpeedA(p *player.Player) *SpeedA {
	return &SpeedA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 10,
		},
	}
}

func (*SpeedA) Type() string {
	return TypeSpeed
}

func (*SpeedA) SubType() string {
	return "A"
}

func (*SpeedA) Description() string {
	return "Checks if the player is moving faster than the maximum speed allowed by their movement speed and sprint state."
}

func (*SpeedA) Punishable() bool {
	return true
}

func (d *SpeedA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *SpeedA) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// Detections also run for rate-limited inputs that never reach the
	// movement simulation, so only trust the movement state on inputs that
	// were actually simulated this frame.
	if i.Tick != d.mPlayer.SimulationFrame {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// Only check when the player is on the ground and not in a state where
	// additional movement speed is expected. HasKnockback/HasTeleport are
	// consumed by the simulation before detections run, so the tick counters
	// are used to exempt the frames where a server-applied velocity is still
	// actively decaying.
	if !d.mPlayer.Movement().OnGround() || d.mPlayer.Movement().Flying() || d.mPlayer.Movement().Gliding() ||
		d.mPlayer.Movement().Immobile() || d.mPlayer.Movement().NoClip() || d.mPlayer.Movement().JustDisabledFlight() ||
		d.mPlayer.Movement().HasTeleport() || d.mPlayer.Movement().PendingTeleports() > 0 ||
		d.mPlayer.Movement().TicksSinceTeleport() <= 1 || d.mPlayer.Movement().TicksSinceKnockback() <= 1 ||
		d.mPlayer.Movement().PenetratedLastFrame() || d.mPlayer.Movement().StuckInCollider() {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// A player riding an entity moves at the vehicle's velocity, which is not
	// bound by the player's movement speed.
	if _, hasVehicle := i.ClientPredictedVehicle.Value(); hasVehicle {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// Compute the maximum sustained ground speed using the same physics the
	// proxy's own simulation uses. Every tick the simulation first adds
	// moveRelative = movementSpeed * (0.16277136 / friction^3) to the
	// velocity and moves the entity by the resulting velocity, then multiplies
	// the velocity by the block friction. At equilibrium the per-tick position
	// delta is therefore moveRelative / (1 - friction) (the friction applied
	// to the stored velocity must not be applied to the equilibrium again).
	// The friction of the standing block must be accounted for, or
	// legitimate sprinting on ice/blue ice false flags. MovementSpeed()
	// already includes the 1.3 sprint multiplier, so it must not be applied
	// again here.
	friction := game.DefaultAirFriction * game.DefaultBlockFriction
	speed := d.mPlayer.Movement().MovementSpeed()
	if support := d.mPlayer.Movement().SupportingBlockPos(); support != nil {
		block := d.mPlayer.World().Block([3]int(*support))
		if utils.BlockName(block) == "minecraft:soul_sand" {
			speed *= 0.543
		}
		friction = game.DefaultAirFriction * utils.BlockFriction(block)
	}
	moveRelativeSpeed := speed * (0.16277136 / (friction * friction * friction))
	maxSpeed := moveRelativeSpeed / (1 - friction)

	// Give a small buffer to account for slopes and client-side movement
	// smoothing.
	buffer := float32(0.06)

	clientVel := d.mPlayer.Movement().Client().Vel()
	horizontalSpeed := mgl32.Vec2{clientVel[0], clientVel[2]}.Len()

	// Ignore vertical velocity from jumping, only the horizontal speed matters.
	if horizontalSpeed > maxSpeed+buffer {
		d.mPlayer.FailDetection(d, "speed", horizontalSpeed, "max_speed", maxSpeed+buffer)
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}
