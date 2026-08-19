package detection

import (
	"github.com/killlime/killlime/game"
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketQ struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketQ(p *player.Player) *BadPacketQ {
	return &BadPacketQ{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 10,
		},
	}
}

func (*BadPacketQ) Type() string {
	return TypeBadPacket
}

func (*BadPacketQ) SubType() string {
	return "Q"
}

func (*BadPacketQ) Description() string {
	return "Checks if a player is reporting a vertical velocity faster than the vanilla terminal velocity, which is only possible when a client directly sets the motion vector to descend faster than physically possible (e.g - Solstice's fast fall disabler with the 'Predict' or 'Set Vel' modes)."
}

func (*BadPacketQ) Punishable() bool {
	return true
}

func (d *BadPacketQ) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketQ) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// Vanilla fall physics: the Y velocity compounds with the 0.98 gravity
	// multiplier and caps at 0.08*0.98/(1-0.98) = 3.92 blocks/tick. Anyfall
	// faster than this cannot be produced by normal gravity, e.g. the fast
	// fall disabler setting the Y delta directly each tick.
	if i.Delta[1] < -game.TerminalVelocity {
		d.mPlayer.FailDetection(d, "vel_y", i.Delta[1])
		return
	}
	d.mPlayer.PassDetection(d, 0.2)
}