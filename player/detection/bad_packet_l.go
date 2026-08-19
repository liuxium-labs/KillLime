package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketL struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketL(p *player.Player) *BadPacketL {
	return &BadPacketL{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 10,
		},
	}
}

func (*BadPacketL) Type() string {
	return TypeBadPacket
}

func (*BadPacketL) SubType() string {
	return "L"
}

func (*BadPacketL) Description() string {
	return "Checks if a player is reporting a downward velocity while simultaneously claiming vertical ground collision, which is only possible when a client spoofs a constant gravity delta (e.g - NetherGames or Custom disablers)."
}

func (*BadPacketL) Punishable() bool {
	return true
}

func (d *BadPacketL) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketL) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// The client claims to be touching the ground vertically, so any significant
	// downward velocity is impossible in that same tick (the Y velocity is
	// clamped to roughly zero while standing on a block). The vanilla
	// gravitational step (-0.0784) is only ever sent while airborne.
	verticalCollision := i.InputData.Load(packet.InputFlagVerticalCollision)
	if verticalCollision && i.Delta[1] < -0.05 {
		d.mPlayer.FailDetection(d, "vel_y", i.Delta[1])
		return
	}
	d.mPlayer.PassDetection(d, 0.2)
}