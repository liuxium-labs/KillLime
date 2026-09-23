# Changelog

## v2.5.0 - 2026-09-18

### Added
- Support for Minecraft 1.26.51 (protocol 2193). The standalone proxy (`example/default`) and the native Dragonfly integration (`example/dragonfly`) now accept 1.26.51 clients.

### Changed
- Upgraded to the official Dragonfly v0.11.5 release (which includes gophertunnel v1.62.0 / protocol 2193 support), removing the vendored fork.
- Migrated to the gophertunnel v1.62.0 protocol API across the proxy, detections, and tests:
  - `PlayerAuthInput.BlockActions`, `ItemInteractionData`, and `ClientPredictedVehicle` are now `Optional[...]`; `InputData` is the new `InputFlags` type.
  - `PlayerList` actions moved from the packet to the entry level (`PlayerListEntry.ActionType`).
  - Crafting data uses typed recipe lists, and item descriptors are name-based (`DefaultItemDescriptor{Name, MetadataValue}`).
  - Movement packet flags use the new `MoveFlag*` constants; the `SetPlayerGameType` enum drops `GameTypeCreativeSpectator`.
  - LevelChunk request-mode handling is driven by the optional `SubChunkLimit`; sub-chunk payloads are optional.
  - Dragonfly server-side chunk heightmaps and player-action packet handling follow the new protocol layout.

### Fixed
- Example modules no longer fail to link: both binaries build cleanly against the migrated protocol.
- Tests updated for the new API (InputFlags bitfields, `Optional` wrappers, per-entry `PlayerList` actions).
- **BadPacket L instant kick on join**: a grounded Bedrock client reports the passive gravity step (`delta.y ≈ -0.0784`) together with the vertical collision flag on every tick, and the proxy does not always have authoritative ground state, so the old "downward velocity contradicts vertical collision" test flagged every legitimate player within a second of joining. The check now only flags a downward velocity faster than a grounded player can physically fall (beyond one gravity step), which movement physics cannot produce, so it no longer depends on the world being loaded.

## v2.3.0 - 2026-08-25

### Fixed
- **Item duplication via inventory transfers**: `transferAction.execute()` now validates `count <= available` before writing the source slot. The `Inventory.SetSlot` path does not clamp counts, so an oversized client-supplied count would write a negative-count stack into the source slot (item duplication in the predicted inventory).
- **Inventory slot out-of-bounds panic**: `TakeStack`, `PlaceStack`, `SwapStack`, `DestroyStack`, and `Drop` handlers now validate slot indices against the target inventory size before constructing actions. Previously a malformed packet with a slot byte (0–255) exceeding a 36/54/4/1-slot inventory would panic inside `Slot()` / `SetSlot()`.
- **`WithPacketCtx` nil-check inversion**: the wrapper called `f(nil)` when a packet context was present and ignored it otherwise. Fixed the logic so the context is passed when non-nil and nil is passed when absent.
- **`Close()` double-close on non-replay connections**: `p.conn` and `serverConn` were closed in both the `!IsReplay` block and the general cleanup block, producing a second `Close` call on already-closed net.Conn values. Removed the duplicate close.
- **`Tick()` inflated server tick count**: `delta > 50` (with a 50ms ticker) meant 1ms of scheduling jitter counted as a second tick, inflating `ServerTick` over time and allowing movement input allowance to drift. Now only genuine stalls ≥ 100ms are compensated (`delta / 50`).

### Added
- Bounds-guard tests covering slot out-of-bounds (all action types), count-overflow transfer, WithPacketCtx nil handling, Close double-close, and Tick jitter scenarios: `player/detection/regression_test.go` (expanded).

## v2.2.0 - 2026-08-19

### Fixed
- Detections now actually run in creative mode: creative attacks are fed to the client combat component (reach/hitbox checks), and the gamemode bypass in combat calculation no longer skips distance hooks. Attack packets are still forwarded untouched in creative.
- Client tracker no longer ignores non-player entities, so end-crystal attacks are validated.
- Timer A now sees rate-limited PlayerAuthInputs, so input flooding above the tick rate is flagged instead of being swallowed by the rate limiter.
- Reach A/B use the vanilla creative reach limit (5 blocks) instead of the survival limit.
- Nuker A now flags bursts of more than 3 block-breaking actions per tick (8-block nukers), regardless of gamemode.
- Nuker A and Scaffold A are now registered.

### Added
- Regression tests covering killaura, reach (players and end crystals, survival and creative), nuker bursts, scaffold registration, and timer flooding: `player/detection/regression_test.go`.

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
- `asset` combat flow diagram, `oconfig` JSON configuration, standalone `deps/proxy` module.