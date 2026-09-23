package detection

import (
	"github.com/killlime/killlime/game"
	"github.com/killlime/killlime/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketL struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketL(p *player.Player) *BadPacketL {
	return &BadPacketL{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     3,
			MaxViolations: 10,
		},
	}
}

func (*BadPacketL) Type() string {
	return TypeBadPacket
}

func (*BadPacketL) SubType() string {
	return "L"
}

func (*BadPacketL) Description() string {
	return "Checks if a player reports a downward velocity while claiming vertical ground collision that is faster than a grounded player can physically fall (e.g a constant gravity-delta spoof, as used by NetherGames or Custom disablers)."
}

func (*BadPacketL) Punishable() bool {
	return true
}

func (d *BadPacketL) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketL) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// A player riding an entity reports the vehicle's motion and collision
	// flags, which are not bound by player gravity.
	if v, ok := i.ClientPredictedVehicle.Value(); ok && v != 0 {
		d.mPlayer.PassDetection(d, 0.5)
		return
	}

	// While standing on a block, the client's downward velocity is clamped by
	// the collision to a single passive gravity step (NormalGravity * 0.98, i.e.
	// ~0.0784). Bedrock 1.26.51+ clients report exactly this step together with
	// the vertical collision flag every grounded tick, so any velocity at or
	// below a grounded fall-step is not anomalous. Only a velocity beyond that
	// (which ground movement cannot produce) is a candidate for a spoof.
	maxGroundedFall := game.NormalGravity * game.NormalGravityMultiplier * 1.5
	if !i.InputData.Load(packet.InputFlagVerticalCollision) || i.Delta[1] >= -maxGroundedFall {
		d.mPlayer.PassDetection(d, 0.2)
		return
	}

	// Spawn teleports, fast transfers, and pending corrections snap the client
	// into place; the authoritative ground state is not meaningful while the
	// client is still settling in.
	if d.mPlayer.Movement().TicksSinceTeleport() <= 20 || d.mPlayer.Movement().InCorrectionCooldown() ||
		d.mPlayer.Movement().PendingTeleports() > 0 {
		d.mPlayer.PassDetection(d, 0.2)
		return
	}

	// A velocity this large is already impossible while grounded, but if the
	// authoritative simulation happens to confirm ground contact we defer to it.
	if d.mPlayer.Movement().YCollision() || d.mPlayer.Movement().OnGround() {
		d.mPlayer.PassDetection(d, 0.2)
		return
	}

	d.mPlayer.FailDetection(d, "vel_y", i.Delta[1])
}