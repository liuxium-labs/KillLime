package detection

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/killlime/killlime/player"
	"github.com/killlime/killlime/player/component"
	playerctx "github.com/killlime/killlime/player/context"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestDebugKillaura(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{IsReplay: true, CurrentTime: time.Now()}, nil)
	component.Register(p)
	Register(p)
	p.Ready = true
	p.Version = protocol.CurrentProtocol
	p.GameMode = packet.GameTypeSurvival

	auth0 := packet.Packet(&packet.PlayerAuthInput{
		Tick:      uint64(0),
		Position:  mgl32.Vec3{0, 71.62, 0},
		InputData: protocol.NewInputFlags(packet.InputFlagCount),
		InputMode: packet.InputModeMouse,
	})
	ctx := playerctx.NewHandlePacketContext(&auth0)
	p.HandleClientPacket(ctx)
	fmt.Println("after first input (rejected): simFrame =", p.SimulationFrame)
	p.Tick()
	fmt.Println("after tick: allowedInputs granted")

	for tick := int64(1); tick <= 15; tick++ {
		auth := packet.Packet(&packet.PlayerAuthInput{
			Tick:      uint64(tick),
			Position:  mgl32.Vec3{0, 71.62, 0},
			InputData: protocol.NewInputFlags(packet.InputFlagCount),
			InputMode: packet.InputModeMouse,
		})
		ctx := playerctx.NewHandlePacketContext(&auth)
		p.HandleClientPacket(ctx)
	}
	fmt.Println("after inputs: simFrame =", p.SimulationFrame, "lastSwing =", p.Combat().LastSwing(), "detections =", len(p.Detections()))

	var d *KillauraA
	for _, det := range p.Detections() {
		if det.Type() == "Killaura" {
			d = det.(*KillauraA)
		}
	}
	if d == nil {
		t.Fatal("no killaura")
	}

	attack := packet.Packet(&packet.InventoryTransaction{
		TransactionData: &protocol.UseItemOnEntityTransactionData{
			TargetEntityRuntimeID: 999,
			ActionType:            protocol.UseItemOnEntityActionAttack,
			HotBarSlot:            0,
		},
	})
	var attackCtx = playerctx.NewHandlePacketContext(&attack)
	p.HandleClientPacket(attackCtx)
	fmt.Println("after attack: deferredSwing =", d.checkDeferredSwing, "vl =", d.Metadata().Violations)

	auth := packet.Packet(&packet.PlayerAuthInput{
		Tick:      uint64(16),
		Position:  mgl32.Vec3{0, 71.62, 0},
		InputData: protocol.NewInputFlags(packet.InputFlagCount),
		InputMode: packet.InputModeMouse,
	})
	ctx = playerctx.NewHandlePacketContext(&auth)
	p.HandleClientPacket(ctx)
	fmt.Println("after authinput: deferredSwing =", d.checkDeferredSwing, "vl =", d.Metadata().Violations)
}
