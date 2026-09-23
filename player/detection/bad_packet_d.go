package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketD struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketD(p *player.Player) *BadPacketD {
	return &BadPacketD{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},
	}
}

func (*BadPacketD) Type() string {
	return TypeBadPacket
}

func (*BadPacketD) SubType() string {
	return "D"
}

func (*BadPacketD) Description() string {
	return "Checks if a player is attempting to run a creative transaction whilst not in creative mode."
}

func (*BadPacketD) Punishable() bool {
	return true
}

func (d *BadPacketD) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketD) Detect(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.PlayerAuthInput:
		if !pk.InputData.Load(packet.InputFlagPerformItemStackRequest) {
			return
		}
		if request, ok := pk.ItemStackRequest.Value(); ok {
			for _, action := range request.Actions {
				d.checkRequestAction(action)
			}
		}
	case *packet.ItemStackRequest:
		for _, request := range pk.Requests {
			for _, action := range request.Actions {
				d.checkRequestAction(action)
			}
		}
	}
}

func (d *BadPacketD) checkRequestAction(action protocol.StackRequestAction) {
	// GameMode only updates after the client acknowledges a gamemode change,
	// so a player that switches to creative and immediately crafts (possible
	// within a single round trip) must not be flagged against the stale value.
	if d.mPlayer.PendingGameMode != -1 {
		return
	}
	if _, ok := action.(*protocol.CraftCreativeStackRequestAction); ok && d.mPlayer.GameMode != packet.GameTypeCreative {
		d.mPlayer.FailDetection(d)
	}
}
