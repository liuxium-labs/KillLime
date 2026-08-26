package detection

import (
	"time"

	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type TimerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	inputsInWindow  int
	windowStartTick int64
	windowStartTime time.Time
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
		windowStartTime: p.Time(),
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
	elapsedTicks := d.mPlayer.ServerTick - d.windowStartTick
	elapsedMs := d.mPlayer.Time().Sub(d.windowStartTime).Milliseconds()
	if elapsedTicks < 20 && elapsedMs < 1000 {
		return
	}

	// The amount of PAI packets a legitimate client can send in the window's
	// real-time span. ServerTick advances in bursts after a stalled tick loop,
	// so a window measured purely in server ticks can span much more than one
	// second of client inputs; scaling the threshold by the elapsed wall time
	// keeps the check accurate regardless of tick loop stalls.
	expectedInputs := int(elapsedMs/50) + 1
	if elapsedMs < 1000 {
		expectedInputs = 20
	}
	if d.inputsInWindow > expectedInputs+5 {
		d.mPlayer.FailDetection(d, "inputs", d.inputsInWindow, "expected", expectedInputs, "window_ms", elapsedMs)
	} else {
		d.mPlayer.PassDetection(d, 0.2)
	}
	d.windowStartTick = d.mPlayer.ServerTick
	d.windowStartTime = d.mPlayer.Time()
	d.inputsInWindow = 0
}