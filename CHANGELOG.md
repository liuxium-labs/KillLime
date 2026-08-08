# Changelog

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