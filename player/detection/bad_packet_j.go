package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketJ struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	prevTick uint64
}

func New_BadPacketJ(p *player.Player) *BadPacketJ {
	return &BadPacketJ{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 10,
		},
	}
}

func (*BadPacketJ) Type() string {
	return TypeBadPacket
}

func (*BadPacketJ) SubType() string {
	return "J"
}

func (*BadPacketJ) Description() string {
	return "Checks if a player's client tick is moving backwards, which is typically caused by a client subtracting an offset from its tick to desync from the server (e.g - SentinelNew's tick shifter)."
}

func (*BadPacketJ) Punishable() bool {
	return true
}

func (d *BadPacketJ) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketJ) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// A small allowance is provided because a client may jump backwards within
	// its tick rate when re-syncing after server corrections. A large backward
	// jump, however, is only ever produced by tick manipulation clients.
	if d.prevTick != 0 && i.Tick < d.prevTick {
		jump := d.prevTick - i.Tick
		if jump > 60 {
			d.mPlayer.FailDetection(d, "jump", jump, "prev_tick", d.prevTick, "tick", i.Tick)
			return
		}
	}
	d.prevTick = i.Tick
	d.mPlayer.PassDetection(d, 0.1)
}