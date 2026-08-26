package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/killlime/killlime/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ScaffoldA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_ScaffoldA(p *player.Player) *ScaffoldA {
	return &ScaffoldA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 5,
		},
	}
}

func (*ScaffoldA) Type() string {
	return TypeScaffold
}

func (*ScaffoldA) SubType() string {
	return "A"
}

func (*ScaffoldA) Description() string {
	return "Checks if the click vector is zero during an initial right click input."
}

func (*ScaffoldA) Punishable() bool {
	return true
}

func (d *ScaffoldA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *ScaffoldA) Detect(pk packet.Packet) {
	invPk, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return
	}
	trData, ok := invPk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return
	}
	if inHand, _ := d.mPlayer.HeldItems(); utils.IsBlockPlaceAlwaysSimBased(inHand.Item()) {
		return
	}
	// The click vector is only meaningful for a block click. Right clicking on
	// air (eating food, charging a bow, throwing ender pearls, using a shield)
	// sends a ClickAir transaction with a zero ClickedPosition, so those must
	// never be compared against. Scaffold cheats send a block click with a
	// zeroed click point, which is what this check is after.
	if trData.ActionType == protocol.UseItemActionClickBlock && trData.ClickedPosition.LenSqr() == 0 && trData.TriggerType == protocol.TriggerTypePlayerInput {
		d.mPlayer.FailDetection(d)
		return
	}
	d.mPlayer.PassDetection(d, 0.5)
}
