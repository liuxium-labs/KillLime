package detection

import (
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type InvMoveA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	preFlag bool
}

func New_InvMoveA(p *player.Player) *InvMoveA {
	return &InvMoveA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},
	}
}

func (*InvMoveA) Type() string {
	return TypeInvMove
}

func (*InvMoveA) SubType() string {
	return "A"
}

func (*InvMoveA) Description() string {
	return "Checks if a player is moving while moving items in their inventory."
}

func (*InvMoveA) Punishable() bool {
	return true
}

func (d *InvMoveA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *InvMoveA) Detect(pk packet.Packet) {
	if req, ok := pk.(*packet.ItemStackRequest); ok {
		// A vanilla client can freely swap hotbar slots and auto-pick up mined
		// items while walking. Only requests that operate on slots the player
		// can only reach with the inventory UI open (crafting, containers,
		// armor, etc.) can indicate an inventory-move cheat.
		d.preFlag = false
		for _, request := range req.Requests {
			if requestTouchesInventoryGUI(request) && d.mPlayer.Movement().Impulse().LenSqr() > 0.0 {
				d.preFlag = true
				break
			}
		}
	} else if _, ok := pk.(*packet.PlayerAuthInput); ok {
		if d.preFlag && d.mPlayer.Movement().Impulse().LenSqr() > 0.0 {
			d.mPlayer.FailDetection(d)
		}
		d.preFlag = false
	}
}

// requestTouchesInventoryGUI returns true if any action in the request can only
// be performed with the inventory (or a container) screen open.
func requestTouchesInventoryGUI(request protocol.ItemStackRequest) bool {
	for _, action := range request.Actions {
		if actionRequiresGUI(action) {
			return true
		}
	}
	return false
}

// actionRequiresGUI returns true if the given stack request action operates on
// slots that are unreachable without opening a GUI screen.
func actionRequiresGUI(action protocol.StackRequestAction) bool {
	switch act := action.(type) {
	case *protocol.TakeStackRequestAction:
		return !isHotbarSlot(act.Source) || !isHotbarSlot(act.Destination)
	case *protocol.PlaceStackRequestAction:
		return !isHotbarSlot(act.Source) || !isHotbarSlot(act.Destination)
	case *protocol.SwapStackRequestAction:
		return !isHotbarSlot(act.Source) || !isHotbarSlot(act.Destination)
	case *protocol.DropStackRequestAction:
		return !isHotbarSlot(act.Source)
	case *protocol.DestroyStackRequestAction:
		return !isHotbarSlot(act.Source)
	case *protocol.ConsumeStackRequestAction:
		return !isHotbarSlot(act.Source)
	case *protocol.MineBlockStackRequestAction:
		// Auto-collecting a mined block is sent while freely moving and
		// does not require the inventory UI.
		return false
	default:
		// Crafting, creative, beacon/grindstone/loom and similar actions can
		// only be triggered from an open screen.
		return true
	}
}

// isHotbarSlot returns true if the given slot points at a hotbar container.
func isHotbarSlot(slot protocol.StackRequestSlotInfo) bool {
	return slot.Container.ContainerID == protocol.ContainerHotBar
}
