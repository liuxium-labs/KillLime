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
			FailBuffer:    2,
			MaxBuffer:     5,
			MaxViolations: 15,
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
	return "DISABLED: JUMP_DOWN semantics changed in Bedrock 1.26.x to mean 'key held' rather than 'press edge', making the original detection impossible to implement correctly."
}

func (*BadPacketO) Punishable() bool {
	return false
}

func (d *BadPacketO) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketO) Detect(pk packet.Packet) {
}
