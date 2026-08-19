package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type TimerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	inputsInWindow  int
	windowStartTick int64
}

func New_TimerA(p *player.Player) *TimerA {
	return &TimerA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 5,
		},
		windowStartTick: p.ServerTick,
	}
}

func (*TimerA) Type() string {
	return TypeTimer
}

func (*TimerA) SubType() string {
	return "A"
}

func (*TimerA) Description() string {
	return "Checks if the player is sending more movement inputs than the server tick rate allows."
}

func (*TimerA) Punishable() bool {
	return true
}

func (d *TimerA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *TimerA) Detect(pk packet.Packet) {
	if _, ok := pk.(*packet.PlayerAuthInput); !ok {
		return
	}

	d.inputsInWindow++
	if d.mPlayer.ServerTick-d.windowStartTick >= 20 {
		if d.inputsInWindow > 20+5 {
			d.mPlayer.FailDetection(d, "inputs", d.inputsInWindow, "window_ticks", 20)
		} else {
			d.mPlayer.PassDetection(d, 0.2)
		}
		d.windowStartTick = d.mPlayer.ServerTick
		d.inputsInWindow = 0
	}
}