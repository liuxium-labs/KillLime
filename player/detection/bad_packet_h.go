package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketH struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	unmatchedCount         int
	lastUnmatchedTimestamp int64
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
	ns, ok := pk.(*packet.NetworkStackLatency)
	if !ok {
		return
	}

	// If the packet reached this point, the client response to our acknowledgment
	// did not match any pending ack timestamp. Legitimate clients only echo the
	// exact timestamp KillLime sent. However, RakNet can redeliver a client
	// response when the server's acknowledgement is lost, producing the same
	// (already consumed) timestamp again, so a duplicate of a previously seen
	// unmatched timestamp is never counted. A single network glitch also must
	// not punish; only a repeated pattern of unmatched timestamps is treated
	// as tampering (a ping spoof disabler e.g. Flareon subtracts a delay from
	// the timestamp before responding).
	if d.mPlayer.ACKs().Pending() > 0 {
		if ns.Timestamp != d.lastUnmatchedTimestamp {
			d.lastUnmatchedTimestamp = ns.Timestamp
			d.unmatchedCount++
			if d.unmatchedCount >= 3 {
				d.mPlayer.FailDetection(d, "pending_acks", d.mPlayer.ACKs().Pending())
			}
		}
		return
	}
	d.lastUnmatchedTimestamp = 0
	d.unmatchedCount = 0
	d.mPlayer.PassDetection(d, 0.5)
}