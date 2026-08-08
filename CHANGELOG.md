# Changelog

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