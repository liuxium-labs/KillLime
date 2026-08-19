package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/killlime/killlime/integration/proxy"
	"github.com/killlime/killlime/utils"
	"github.com/sandertv/gophertunnel/minecraft"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: killlime-proxy <local-address> <remote-address>")
		fmt.Println("Example: killlime-proxy :19132 127.0.0.1:19133")
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	status, err := minecraft.NewForeignStatusProvider(os.Args[2])
	if err != nil {
		panic(err)
	}
	utils.InitializeBlockNameMapping()
	p, err := proxy.Listen(ctx, proxy.Config{
		LocalAddress:  os.Args[1],
		RemoteAddress: os.Args[2],
		Log:           slog.Default(),
		Listen: minecraft.ListenConfig{
			StatusProvider:      status,
			FlushRate:           -1,
			AllowUnknownPackets: true,
			AllowInvalidPackets: true,
		},
	})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	if err := p.Serve(ctx); err != nil && ctx.Err() == nil {
		panic(err)
	}
}
