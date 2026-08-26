package detection

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketN struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	lastPos   mgl32.Vec3
	hasLastPos bool
}

func New_BadPacketN(p *player.Player) *BadPacketN {
	return &BadPacketN{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 10,
		},
	}
}

func (*BadPacketN) Type() string {
	return TypeBadPacket
}

func (*BadPacketN) SubType() string {
	return "N"
}

func (*BadPacketN) Description() string {
	return "Checks if a player is teleporting large distances in their movement input without a server-authorized teleport, which is only possible when a client injects fake positions (e.g - Sentinel's origin-point flood or fake teleports)."
}

func (*BadPacketN) Punishable() bool {
	return true
}

func (d *BadPacketN) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketN) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// Track the raw (non-authoritative) client position, since the packet's
	// Position field is rewritten to KillLime's predicted position before
	// detections run.
	clientPos := d.mPlayer.Movement().Client().Pos()

	// The very first accepted input establishes the baseline. Client().Pos()
	// starts at the zero vector before the first input is processed, which is
	// not a real client position, so it must never be used as a baseline or
	// be compared against (otherwise players spawning far from the world
	// origin would be flagged).
	if clientPos == (mgl32.Vec3{}) {
		return
	}

	// ignore the very first input sample
	if !d.hasLastPos {
		d.hasLastPos = true
		d.lastPos = clientPos
		return
	}

	if d.mPlayer.Movement().HasTeleport() || d.mPlayer.Movement().PendingTeleports() > 0 ||
		d.mPlayer.Movement().HasKnockback() || d.mPlayer.Movement().Immobile() {
		d.lastPos = clientPos
		return
	}

	// A player riding an entity reports the vehicle's position, which can move
	// faster than the player could ever walk. Skip the distance check entirely.
	if _, hasVehicle := i.ClientPredictedVehicle.Value(); hasVehicle {
		d.lastPos = clientPos
		return
	}

	// A player can never legitimately move more than a few blocks per input
	// tick. Anything beyond ~50 blocks without a teleport is injected.
	dist := clientPos.Sub(d.lastPos).Len()
	if dist > 50 {
		d.mPlayer.FailDetection(d, "dist", dist, "pos", clientPos, "last_pos", d.lastPos)
		return
	}
	d.lastPos = clientPos
	d.mPlayer.PassDetection(d, 0.5)
}