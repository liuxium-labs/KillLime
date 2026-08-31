package detection

import (
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BlinkA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	lastInputTime time.Time
	freezeStart   time.Time
	freezePos     mgl32.Vec3
	freezeActive  bool
}

func New_BlinkA(p *player.Player) *BlinkA {
	return &BlinkA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     2,
			MaxViolations: 5,
		},
		lastInputTime: p.Time(),
		freezePos:     p.Movement().Pos(),
	}
}

func (*BlinkA) Type() string {
	return TypeBlink
}

func (*BlinkA) SubType() string {
	return "A"
}

func (*BlinkA) Description() string {
	return "Checks if the player freezes movement packets and then teleports forward (blink)."
}

func (*BlinkA) Punishable() bool {
	return true
}

func (d *BlinkA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BlinkA) Detect(pk packet.Packet) {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	now := d.mPlayer.Time()
	sinceLast := now.Sub(d.lastInputTime)

	// Check if a freeze just ended: we had an active freeze and now
	// received a new input after the gap.
	if d.freezeActive {
		d.freezeActive = false
		pos := d.mPlayer.Movement().Pos()
		dist := mgl32.Vec3{pos[0] - d.freezePos[0], 0, pos[2] - d.freezePos[2]}.Len()
		freezeMs := now.Sub(d.freezeStart).Milliseconds()

		// Max legitimate distance: normal walk speed (4.317 blocks/s) * freeze duration + buffer.
		maxDist := float32(freezeMs)/1000.0*4.317 + 1.5

		if dist > maxDist && dist > 3.0 {
			d.mPlayer.FailDetection(d, "dist", dist, "freeze_ms", freezeMs, "max_dist", maxDist)
			d.lastInputTime = now
			d.freezePos = pos
			return
		}
		d.lastInputTime = now
		d.freezePos = pos
		d.mPlayer.PassDetection(d, 0.3)
		return
	}

	// Detect the start of a freeze: a large gap between inputs.
	if sinceLast > 250*time.Millisecond {
		d.freezeStart = d.lastInputTime
		// freezePos still holds the pre-gap position from the last tick.
		d.freezeActive = true
		d.lastInputTime = now
		// Do NOT update freezePos here — we need the pre-gap position.
		d.mPlayer.PassDetection(d, 0.3)
		return
	}

	d.lastInputTime = now
	d.freezePos = d.mPlayer.Movement().Pos()
	d.mPlayer.PassDetection(d, 0.3)
}
