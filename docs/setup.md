# Setting up KillLime

Choose the setup that matches the software accepting players behind KillLime:

| Server software | Recommended integration |
| --- | --- |
| PocketMine-MP | [Standalone KillLime proxy with KillLime-PM](#pocketmine-mp) |
| Dragonfly | [Native Dragonfly listener](#dragonfly) |
| PowerNukkitX or another Bedrock server | [Standalone KillLime proxy](#other-server-software) |

Do not place both integrations in front of the same server. Dragonfly embeds
KillLime in the server process; the other paths run KillLime as the public-facing
RakNet listener.

## PocketMine-MP

Use KillLime's standalone proxy and install
[KillLime-PM](https://@@FGKillLimePM@@) on every PocketMine-MP
backend.

```text
Minecraft client -> KillLime :19132 -> PocketMine-MP 127.0.0.1:19133
                                      + KillLime-PM
```

### 1. Configure PocketMine-MP

Keep PocketMine-MP on a private address and let KillLime own the public Bedrock
port. For a proxy and backend on the same machine, the relevant
`server.properties` values are:

```properties
server-ip=127.0.0.1
server-port=19133
xbox-auth=off
```

KillLime authenticates the public client connection. The private backend must
accept KillLime's connection instead of trying to authenticate it with Xbox Live
again.

### 2. Install KillLime-PM

Build or download the current KillLime-PM plugin package, place the `.phar` in
PocketMine-MP's `plugins` directory, and start PocketMine-MP once to create its
configuration. In `plugin_data/KillLime/config.yml`, allow only the address from
which KillLime connects:

```yaml
Trusted-Proxy-Addresses:
  - "127.0.0.1"
  - "::1"
```

Use the proxy container or host address instead of loopback when the processes
run on different machines. KillLime-PM receives detection events and supplies the
PocketMine-side alerts, logs, punishments, and plugin events; it does not
replace PocketMine's network interface.

### 3. Start KillLime

Build the checked-in [`example/default`](../example/default) standalone
example:

```bash
cd example/default
go build -o killlime-proxy .
./killlime-proxy :19132 127.0.0.1:19133
```

Start PocketMine-MP before KillLime. Players join the KillLime address on port
`19132`, not the backend port.

## Dragonfly

Embed KillLime directly in Dragonfly. Do not run the standalone proxy in front of
the same Dragonfly listener.

```text
Minecraft client -> Dragonfly with KillLime :19132
```

Add KillLime and use the Dragonfly version selected by KillLime's
[`example/dragonfly/go.mod`](../example/dragonfly/go.mod). Go module `replace`
directives are not inherited from dependencies, so the application must carry
that Dragonfly replacement itself.

Configure the server listener like this:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/df-mc/dragonfly/server"
	killlimeDragonfly "github.com/killlime/killlime/integration/dragonfly"
	"github.com/killlime/killlime/player"
)

func main() {
	ctx := context.Background()
	log := slog.Default()
	conf, err := server.DefaultConfig().Config(log)
	if err != nil {
		panic(err)
	}

	events := player.NewExampleEventHandler()
	conf.Listeners = []func(server.Config) (server.Listener, error){
		killlimeDragonfly.Listener(ctx, killlimeDragonfly.Config{
			Address: ":19132",
			Configure: func(p *player.Player) {
				p.HandleEvents(events)
			},
		}),
	}

	srv := conf.New()
	srv.CloseOnProgramEnd()
	srv.Listen()
	for range srv.Accept() {
	}
}
```

The listener uses Snappy compression by default unless the Dragonfly server
configuration explicitly selects another compression implementation. The
complete runnable version is in [`example/dragonfly`](../example/dragonfly).

## Other server software

Use KillLime's standalone proxy for Bedrock server software that does not have a
native integration. PowerNukkitX is shown here, but the same topology applies
to another backend that can accept a private, unauthenticated proxy
connection.

```text
Minecraft client -> KillLime :19132 -> PowerNukkitX 127.0.0.1:19133
```

### PowerNukkitX example

Set these values in PowerNukkitX's `server.properties`:

```properties
server-ip=127.0.0.1
server-port=19133
xbox-auth=off
```

Then build and start KillLime:

```bash
cd example/default
go build -o killlime-proxy .
./killlime-proxy :19132 127.0.0.1:19133
```

For containers or separate hosts, replace `127.0.0.1:19133` with the private
backend address reachable from KillLime. Keep `:19132` as the public listener (or
choose another public port) and restrict the backend UDP port to the KillLime host
at the firewall.

Backend-specific alerts or punishment hooks require an adapter comparable to
KillLime-PM. Packet validation and mitigation still run in the standalone proxy
without one.

## Verify the installation

After starting the backend and KillLime:

1. Ping the public KillLime address and confirm it reports the backend status.
2. Join through the public address and remain in the world for at least one
   minute.
3. Confirm the backend sees the player connection from the KillLime host.
4. Confirm the backend UDP port is not reachable from the public internet.
5. For PocketMine-MP, confirm KillLime-PM is enabled and trusts only the KillLime
   source address.

If the client reaches KillLime but the backend rejects login, first confirm that
the backend is private and that backend Xbox authentication is disabled. The
public KillLime listener should continue to authenticate players.
