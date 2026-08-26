package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketK struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	spinSameTickCount int
}

func New_BadPacketK(p *player.Player) *BadPacketK {
	return &BadPacketK{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 3,
		},
	}
}

func (*BadPacketK) Type() string {
	return TypeBadPacket
}

func (*BadPacketK) SubType() string {
	return "K"
}

func (*BadPacketK) Description() string {
	return "Checks if a player is toggling the spin attack and swimming states in the same tick, which is typically caused by a client attempting to desync its movement prediction (e.g - NukkitLagback disabler)."
}

func (*BadPacketK) Punishable() bool {
	return true
}

func (d *BadPacketK) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketK) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// A legitimate client can only start a spin attack while it is swimming and
	// moving through water with a trident, and never immediately stop it in the
	// exact same tick after starting it. A press+release falling inside one
	// input tick happens naturally on fast taps though, so only a repeated
	// same-tick start+stop pattern (which is what the NukkitLagback disabler
	// forces on every tick) is flagged.
	startSpin := i.InputData.Load(packet.InputFlagStartSpinAttack)
	stopSpin := i.InputData.Load(packet.InputFlagStopSpinAttack)
	if startSpin && stopSpin {
		d.spinSameTickCount++
		if d.spinSameTickCount >= 3 {
			d.mPlayer.FailDetection(d, "reason", "spin_toggle")
			return
		}
	} else {
		d.spinSameTickCount = 0
	}
	d.mPlayer.PassDetection(d, 0.5)
}