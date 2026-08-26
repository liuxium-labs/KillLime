# Changelog

## v2.4.0 - 2026-08-26

### Fixed
- **BadPacket/L threshold**: Changed from `-0.05` to `-0.0784 * 1.5` (normal gravity is `-0.0784`) to properly account for Bedrock 1.26.x gravity.
- **BadPacket/O**: Detection fully disabled — `JUMP_DOWN` semantics changed in 1.26.x to mean "key held" instead of "press edge", making the detection impossible to implement correctly.
- **Nuker/A**: Regressed — vanilla clients break exactly 1 block per tick. Threshold lowered to `> 1` to catch multi-block breaks.

### Added
- **NoSlowdown/A**: Detects players moving at full speed while using items (bow, food, shield, crossbow, etc). Tracks `StartUsingItem` with a 20-tick state timer and applies the vanilla 0.4x slowdown factor to max speed.
- **Blink/A**: Detects packet freezing / movement desync. Measures 250ms+ gaps between `PlayerAuthInput` packets and checks if post-freeze movement exceeds expected distance.
- **Kick codes (Flareon-style)**: Each detection now sends a short kick code on punishment instead of a generic message. Codes: `Abagnale`, `Bluebird`, `Platinumo`, `SugarRush`, `CIA`, `Woodpecker`, `400`, `403`, `405`, `Proxy`.
- **KickCode config field**: Detection config now accepts a `kick_code` JSON field.

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