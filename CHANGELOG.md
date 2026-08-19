# Changelog

## v0.2.2 - 2026-08-08

### Changed
- BadPacket checks (A-U) are no longer registered: they repeatedly false-flagged legitimate clients
  (e.g. BadPacket L flagged the vanilla per-tick gravity delta `-0.0784` as a violation, and BadPacket A
  punished on benign simulation-frame resets). The checks remain implemented and can be re-enabled in
  `player/detection/register.go`.

### Added
- Scaffold B — detects fast diagonal/extended block placements beyond the vanilla interaction distance
  (scaffold modules with an "extend" setting). Placed-block distance is measured from the player's eye,
  mirroring the proxy's own interaction-distance check; anything past 7.5 blocks is flagged. Creative
  players are exempt.

## v0.2.1 - 2026-08-08

### Added
- Minecraft Bedrock 1.26.40 support (gophertunnel v1.58.0, oomph-ac/dragonfly fork commit `c1faf42`).

### Changed
- Migrated to the gophertunnel v1.58.0 protocol API:
  - `PlayerList` now carries a per-entry `ActionType` (`protocol.PlayerListActionAdd/Remove`) instead of a packet-level field.
  - `protocol.Recipe` removed; `CraftingData` now exposes typed recipe slices (`ShapedRecipes`, `ShapelessRecipes`, `MultiRecipes`, `SmithingTransformRecipes`, `SmithingTrimRecipes`) — recipe map retyped to `map[uint32]any`.
  - `LevelChunk` sub-chunk request mode constants removed; limited chunks now indicated by an optional `SubChunkLimit`.
  - `PlayerAuthInput` `ItemStackRequest`, `BlockActions`, `ItemInteractionData`, and inventory action `WindowID` are now `protocol.Optional[...]` (unwrap with `.Value()`).
  - `MoveActorDeltaFlagTeleport` renamed to `MoveFlagTeleport` (`packet.MoveFlagTeleport`).
  - `DefaultItemDescriptor` now keyed by `Name`/`MetadataValue` (no `NetworkID`) — recipe ingredient lookup switched to `world.ItemByName`.
  - `internal/nbtconv.Item` removed in the dragonfly fork — `utils.ReadItem` and `player.ConvertToStack` now use the public `item.ReadNBT`.
- Updated `example/default`, `example/dragonfly`, and `deps/proxy` modules to gophertunnel v1.58.0 and the new dragonfly fork commit.

### Fixed
- Players are no longer kicked on join for benign conditions:
  - Cache-enabled `LevelChunk`/`SubChunk` packets are skipped (logged) instead of disconnecting with "Chunk cache is not supported."
  - Chunk decode failures skip the chunk instead of disconnecting.
  - Unknown device OS/title ID combinations in EditionFakerA only log instead of disconnecting.
  - ACK flush with no pending batch and unhandled movement packets log instead of disconnecting.

### Tuned (more lenient on legitimate players)
- 15-second punishment grace period after joining (`GraceTick`).
- Doubled the violation threshold for every detection (config `max_violations` is now effectively `x2`).
- Halved the fail-buffer accumulation rate — detections need roughly twice as many flagged inputs before counting a violation.
- Raised default `max_violations` and switched instant-kick checks from `ban` to `kick` in the default config (BadPacket A-G, EditionFaker A-C, InvMove, Nuker, Scaffold, Proxy B, Killaura, etc.).

### Added
- Creative-mode logic: players in creative (or creative-spectator) are exempt from checks that only apply to survival gameplay — Speed, Timer, Phase, Scaffold, Nuker, InvMove, Reach, Killaura, Hitbox, Autoclicker, Aim. The exemption is dynamic: it follows the player's current game mode mid-session (`player.IsCreative()` / `creativeExemptChecks` in `player/detection.go`). Packet-integrity checks (BadPacket) and device checks (EditionFaker) still apply in creative.
- Increased fail buffers on all checks so isolated/random flags cannot produce violations: BadPacket A-I/K/O-T, EditionFaker A-C, InvMove, Nuker, Scaffold now need ~4 flagged inputs per violation (FailBuffer 2/MaxBuffer 3); BadPacket J/L/N/Q and Speed/Phase (3/5); Timer (4/6); Reach (3/5); Killaura (2/3); Hitbox (8/10); Autoclicker (6/8); Aim (8/10).

## v0.2.0 - 2026-08-08

Second public release.

### Fixed
- BadPacket P (glide start): no longer false-positives on a legitimate jump-then-glide input (same-tick jump flags are now excluded).
- BadPacket Q (terminal velocity): added a 0.01 block/tick tolerance to the fall-cap so float32 gravity accumulation on vanilla fall speeds cannot false-positive.
- BadPacket N (unauthorized teleports): the position baseline now rebases after a violation so a single injected position cannot produce an endless violation chain.
- BadPacket M: replaced the `InputMode == 0` "uninitialized" sentinel with an explicit flag since input mode 0 (unknown) is a legitimate device value.
- All checks gofmt- and golangci-lint-clean (0 issues).

### Added
- New checks:
  - BadPacket R — sprint start/stop edge flags in a single input (sprint spam, "Always Sprint" disablers).
  - BadPacket S — sneak start/stop edge flags in a single input (sneak-desync togglers).
  - BadPacket T — flying start/stop edge flags in a single input (flight togglers).
  - BadPacket U — crawling start/stop edge flags in a single input (crawl bots).
- `build` and `release` targets to the Makefile.

## v0.1.2 - 2026-08-08

First public release.

### Added
- Standalone interception proxy (`example/default`) and Dragonfly server integration (`example/dragonfly`).
- Server authoritative movement with latency-aware position correction (timer mitigations included) and server authoritative combat with lag compensation.
- Detection suite, registered on a per-player basis with configurable fail buffers and punishments:

| Check | Type | Detects |
| --- | --- | --- |
| Timer A | movement | More movement inputs than the server tick rate allows |
| Speed A | movement | Movement faster than max speed for the player's speed/sprint state |
| Reach A/B | combat | Exceeding vanilla combat reach / entity distance limit |
| KillAura A | combat | Attacking without swinging the arm |
| Scaffold A | movement | Zero click vector during initial right-click |
| Phase A | movement | Penetrating solid blocks without the ability to do so |
| Nuker A | block | Wrong packet for breaking blocks |
| AutoClicker A | combat | Clicking above the configured CPS limit |
| InvMove A | inventory | Moving while moving items in the inventory |
| EditionFaker A/B/C | misc | Faked device OS or invalid input mode for the device |
| BadPacket A-O, P, Q | packet | Invalid simulation frames, self-hits, invalid block breaking, creative transactions without creative mode, invalid MoveVectors, invalid hotbar slots, unmatched acknowledgment timestamps (NSL tampering / ping spoof), non-finite or world-border positions, backwards client ticks (tick shifters), contradictory spin/swim flags, downward velocity with ground collision (gravity-delta spoof), input-mode randomization, un-authorized teleports, contradictory jump flags, forced glide flags, vertical velocity above the terminal velocity |
| BadPacket R-U | packet | Sprint, sneak, flying, and crawl start/stop edge flags in a single input, which is only possible when a client forces a state toggle rather than reporting it (sprint/sneak spam, flight toggler, crawl bots) |
- `asset` combat flow diagram, `oconfig` JSON configuration, standalone `deps/proxy` module.