package detection

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/player"
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
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// Only check when the player is on the ground and not in a state where
	// additional movement speed is expected.
	if !d.mPlayer.Movement().OnGround() || d.mPlayer.Movement().Flying() || d.mPlayer.Movement().Gliding() ||
		d.mPlayer.Movement().Immobile() || d.mPlayer.Movement().NoClip() || d.mPlayer.Movement().HasTeleport() ||
		d.mPlayer.Movement().HasKnockback() || d.mPlayer.Movement().PenetratedLastFrame() || d.mPlayer.Movement().StuckInCollider() {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// The maximum speed a player can reach on the ground. When sprinting, the
	// movement speed is multiplied by 1.3. Give a small buffer to account for
	// slopes and client-side movement smoothing.
	maxSpeed := d.mPlayer.Movement().MovementSpeed() * 1.3
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
