package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type NukerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_NukerA(p *player.Player) *NukerA {
	return &NukerA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},
	}
}

func (*NukerA) Type() string {
	return TypeNuker
}

func (*NukerA) SubType() string {
	return "A"
}

func (*NukerA) Description() string {
	return "Checks if a player sends the wrong packet for breaking blocks."
}

func (*NukerA) Punishable() bool {
	return true
}

func (d *NukerA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *NukerA) Detect(pk packet.Packet) {
	// A nuker sends many block-breaking actions in a single tick instead of
	// waiting for each block to break. Vanilla clients only ever break one
	// block at a time, so multiple destroy/stop actions per input is
	// unmistakably a nuker (this works in creative mode too, where the
	// vanilla client also only destroys one block per input).
	if authInput, ok := pk.(*packet.PlayerAuthInput); ok {
		brokenInTick := 0
		for _, action := range authInput.BlockActions {
			if action.Action == protocol.PlayerActionPredictDestroyBlock || action.Action == protocol.PlayerActionStopBreak {
				brokenInTick++
			}
		}
		if brokenInTick > 3 {
			d.mPlayer.FailDetection(d)
			d.mPlayer.Log().Debug("nuker(A)", "brokenInTick", brokenInTick, "vl", d.metadata.Violations)
		}
		return
	}
	invPk, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return
	}
	trDat, ok := invPk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return
	}
	if trDat.ActionType == protocol.UseItemActionBreakBlock && (d.mPlayer.GameMode == packet.GameTypeSurvival || d.mPlayer.GameMode == packet.GameTypeAdventure) {
		d.mPlayer.FailDetection(d)
	}
}
