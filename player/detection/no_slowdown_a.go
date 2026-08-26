package detection

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/game"
	"github.com/killlime/killlime/player"
	"github.com/killlime/killlime/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type NoSlowdownA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	usingTicksRemaining int
}

func New_NoSlowdownA(p *player.Player) *NoSlowdownA {
	return &NoSlowdownA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 10,
		},
	}
}

func (*NoSlowdownA) Type() string {
	return TypeNoSlowdown
}

func (*NoSlowdownA) SubType() string {
	return "A"
}

func (*NoSlowdownA) Description() string {
	return "Checks if the player is moving at full speed while using an item (bow, food, shield, etc)."
}

func (*NoSlowdownA) Punishable() bool {
	return true
}

func (d *NoSlowdownA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *NoSlowdownA) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	if i.Tick != d.mPlayer.SimulationFrame {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// StartUsingItem is a one-shot flag: it only fires on the first tick the
	// player starts using an item. Track a countdown so we know the player is
	// "using" for the next several ticks.
	if i.InputData.Load(packet.InputFlagStartUsingItem) {
		d.usingTicksRemaining = 20
	}
	if d.usingTicksRemaining > 0 {
		d.usingTicksRemaining--
	}

	if d.usingTicksRemaining == 0 {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	if d.mPlayer.Movement().Flying() || d.mPlayer.Movement().Gliding() ||
		d.mPlayer.Movement().Immobile() || d.mPlayer.Movement().NoClip() ||
		d.mPlayer.Movement().JustDisabledFlight() ||
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

	// When using an item, the player should be slowed to ~40% of normal speed.
	useSlowdown := float32(0.4)
	speed := d.mPlayer.Movement().MovementSpeed() * useSlowdown

	friction := game.DefaultAirFriction * game.DefaultBlockFriction
	if support := d.mPlayer.Movement().SupportingBlockPos(); support != nil {
		block := d.mPlayer.World().Block([3]int(*support))
		if utils.BlockName(block) == "minecraft:soul_sand" {
			speed *= 0.543
		}
		friction = game.DefaultAirFriction * utils.BlockFriction(block)
	}
	moveRelativeSpeed := speed * (0.16277136 / (friction * friction * friction))
	maxSpeed := moveRelativeSpeed / (1 - friction)

	buffer := float32(0.06)
	clientVel := d.mPlayer.Movement().Client().Vel()
	horizontalSpeed := mgl32.Vec2{clientVel[0], clientVel[2]}.Len()

	if horizontalSpeed > maxSpeed+buffer {
		d.mPlayer.FailDetection(d, "speed", horizontalSpeed, "max_speed", maxSpeed+buffer, "using_ticks", d.usingTicksRemaining)
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}
