package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type PhaseA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	consecutivePenetration int
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
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// Detections also run for rate-limited inputs that never reach the
	// movement simulation, where PenetratedLastFrame holds a stale value from
	// the last simulated frame. Only trust it on the frame that actually
	// produced it.
	if i.Tick != d.mPlayer.SimulationFrame {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// If the player is currently in a state where they are allowed to move
	// through blocks, skip the check.
	if d.mPlayer.Movement().Flying() || d.mPlayer.Movement().NoClip() || d.mPlayer.Movement().Immobile() ||
		d.mPlayer.Movement().HasTeleport() || d.mPlayer.Movement().PendingTeleports() > 0 {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// The player's bounding box can legitimately overlap a solid block for a
	// frame or two when the world changes under them (pistons, falling sand,
	// blocks placed on the player) before the simulation pushes them out.
	// Only sustained penetration is treated as cheating.
	if d.mPlayer.Movement().PenetratedLastFrame() {
		d.consecutivePenetration++
		if d.consecutivePenetration >= 3 {
			d.mPlayer.FailDetection(d)
			return
		}
		d.mPlayer.PassDetection(d, 0.5)
		return
	}
	d.consecutivePenetration = 0
	d.mPlayer.PassDetection(d, 0.5)
}
