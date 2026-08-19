package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type PhaseA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_PhaseA(p *player.Player) *PhaseA {
	return &PhaseA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 10,
		},
	}
}

func (*PhaseA) Type() string {
	return TypePhase
}

func (*PhaseA) SubType() string {
	return "A"
}

func (*PhaseA) Description() string {
	return "Checks if the player is penetrating solid blocks without the ability to do so."
}

func (*PhaseA) Punishable() bool {
	return true
}

func (d *PhaseA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *PhaseA) Detect(pk packet.Packet) {
	if _, ok := pk.(*packet.PlayerAuthInput); !ok {
		return
	}

	// If the player is currently in a state where they are allowed to move
	// through blocks, skip the check.
	if d.mPlayer.Movement().Flying() || d.mPlayer.Movement().NoClip() || d.mPlayer.Movement().Immobile() {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	if d.mPlayer.Movement().PenetratedLastFrame() {
		d.mPlayer.FailDetection(d)
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}
