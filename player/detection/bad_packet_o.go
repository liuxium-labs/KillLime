package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketO struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketO(p *player.Player) *BadPacketO {
	return &BadPacketO{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 3,
		},
	}
}

func (*BadPacketO) Type() string {
	return TypeBadPacket
}

func (*BadPacketO) SubType() string {
	return "O"
}

func (*BadPacketO) Description() string {
	return "Checks if a player is holding and releasing the jump key in the same input, which is only possible when a client forces contradictory jump flags to desync movement prediction (e.g - Lifeboat and Sentinel disablers)."
}

func (*BadPacketO) Punishable() bool {
	return true
}

func (d *BadPacketO) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketO) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// A vanilla client can never claim to be holding the jump key (WANT_UP /
	// JUMPING) while simultaneously reporting the jump key being released
	// (JUMP_DOWN) in the same input tick. The Lifeboat and Sentinel disablers
	// force all three flags on every packet to spoof a constant jump state.
	holdingJump := i.InputData.Load(packet.InputFlagWantUp) || i.InputData.Load(packet.InputFlagJumping)
	releasedJump := i.InputData.Load(packet.InputFlagJumpDown)
	if holdingJump && releasedJump {
		d.mPlayer.FailDetection(d, "reason", "contradictory_jump_flags")
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}