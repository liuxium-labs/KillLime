package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketM struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	lastInputMode uint32
}

func New_BadPacketM(p *player.Player) *BadPacketM {
	return &BadPacketM{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     2,
			MaxViolations: 10,
		},
	}
}

func (*BadPacketM) Type() string {
	return TypeBadPacket
}

func (*BadPacketM) SubType() string {
	return "M"
}

func (*BadPacketM) Description() string {
	return "Checks if a player is continuously changing their input mode, which is only possible when a client randomizes the input mode field to bypass input validation (e.g - NetherGames disabler)."
}

func (*BadPacketM) Punishable() bool {
	return true
}

func (d *BadPacketM) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketM) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	if d.lastInputMode == 0 {
		d.lastInputMode = i.InputMode
		return
	}

	// A legitimate client only ever uses a single input mode determined by the
	// physical input device, and can never switch between keyboard, touch, and
	// gamepad. Rapid mode flips are a strong sign of a randomized spoof.
	if i.InputMode != d.lastInputMode {
		d.mPlayer.FailDetection(d, "last_mode", d.lastInputMode, "mode", i.InputMode)
		d.lastInputMode = i.InputMode
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}