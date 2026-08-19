package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketP struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketP(p *player.Player) *BadPacketP {
	return &BadPacketP{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 3,
		},
	}
}

func (*BadPacketP) Type() string {
	return TypeBadPacket
}

func (*BadPacketP) SubType() string {
	return "P"
}

func (*BadPacketP) Description() string {
	return "Checks if a player is reporting the start-gliding flag while not descending, which is only possible when a client forces the glide flag to desync movement prediction (e.g - Solstice's 'Glide' disabler setting)."
}

func (*BadPacketP) Punishable() bool {
	return true
}

func (d *BadPacketP) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketP) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// Vanilla gliding can only ever start while the player is airborne and
	// falling (gliding into a dive). A client-side flag spoof (e.g. Solstice's
	// "Whether or not to send start gliding packet") sets the start-gliding
	// flag regardless of the player's motion.
	if i.InputData.Load(packet.InputFlagStartGliding) && i.Delta[1] >= 0 {
		d.mPlayer.FailDetection(d, "reason", "glide_start_no_descent")
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}