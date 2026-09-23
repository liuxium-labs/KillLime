package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketO struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	prevHoldingJump bool
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

	// JUMP_DOWN is the press edge of the jump key: on the frame the key is
	// pressed, a vanilla client sets WANT_UP, JUMPING and JUMP_DOWN all at
	// once, so "holding + JUMP_DOWN" is perfectly legitimate. Pressing the
	// jump key again while it is already held down is impossible, however.
	// Disablers that force all jump flags on every packet (constant jump
	// spoof) produce exactly this impossible pattern every tick.
	holdingJump := i.InputData.Load(packet.InputFlagWantUp) || i.InputData.Load(packet.InputFlagJumping)
	jumpDown := i.InputData.Load(packet.InputFlagJumpDown)
	if jumpDown && d.prevHoldingJump {
		d.mPlayer.FailDetection(d, "reason", "contradictory_jump_flags")
		return
	}
	d.prevHoldingJump = holdingJump
	d.mPlayer.PassDetection(d, 0.5)
}