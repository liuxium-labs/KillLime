package detection

import (
	"math"

	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketI struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketI(p *player.Player) *BadPacketI {
	return &BadPacketI{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 3,
		},
	}
}

func (*BadPacketI) Type() string {
	return TypeBadPacket
}

func (*BadPacketI) SubType() string {
	return "I"
}

func (*BadPacketI) Description() string {
	return "Checks if a player is sending positions that are either non-finite (NaN/infinity) or beyond the world border, which indicates a client attempting to desync its position from the server (e.g - spamming ±INT_MAX positions)."
}

func (*BadPacketI) Punishable() bool {
	return true
}

func (d *BadPacketI) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketI) Detect(pk packet.Packet) {
	if _, ok := pk.(*packet.PlayerAuthInput); !ok {
		return
	}

	// The packet's position field is rewritten to KillLime's predicted position before
	// detections run, so we must check the client's raw (non-authoritative) position.
	clientPos := d.mPlayer.Movement().Client().Pos()

	// X/Z upper bound is set to 10x the world border (30,000,000) to allow for
	// legitimate edge-of-world movement without catching false positives. Y has
	// no world border, but a sane vertical limit still applies.
	const maxXZ, maxY float32 = 3e8, 1e8

	for i, v := range clientPos {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			d.mPlayer.FailDetection(d, "axis", i, "value", v, "reason", "not_finite")
			return
		}
	}
	if clientPos[0] > maxXZ || clientPos[0] < -maxXZ || clientPos[2] > maxXZ || clientPos[2] < -maxXZ || clientPos[1] > maxY || clientPos[1] < -maxY {
		d.mPlayer.FailDetection(d, "pos", clientPos)
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}