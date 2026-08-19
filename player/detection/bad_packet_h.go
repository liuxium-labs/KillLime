package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketH struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketH(p *player.Player) *BadPacketH {
	return &BadPacketH{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 3,
		},
	}
}

func (*BadPacketH) Type() string {
	return TypeBadPacket
}

func (*BadPacketH) SubType() string {
	return "H"
}

func (*BadPacketH) Description() string {
	return "Checks if a player is sending acknowledgments that do not match any pending acknowledgment, which is typically caused by tampering with the NetworkStackLatency timestamp (ping spoofing)."
}

func (*BadPacketH) Punishable() bool {
	return true
}

func (d *BadPacketH) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketH) Detect(pk packet.Packet) {
	if _, ok := pk.(*packet.NetworkStackLatency); !ok {
		return
	}

	// If the packet reached this point, the client response to our acknowledgment
	// did not match any pending ack timestamp. Legitimate clients only echo the
	// exact timestamp KillLime sent. A mismatch while we still have pending acks
	// is a strong indicator of a ping spoof disabler (e.g - Flareon's ping spoof
	// subtracts a delay from the timestamp before responding).
	if d.mPlayer.ACKs().Pending() > 0 {
		d.mPlayer.FailDetection(d, "pending_acks", d.mPlayer.ACKs().Pending())
	} else {
		d.mPlayer.PassDetection(d, 0.5)
	}
}