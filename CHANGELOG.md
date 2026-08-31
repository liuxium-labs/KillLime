# KillLime v2.5.0 Changelog

### Updated
- gophertunnel v1.59.0 ? v1.61.0 — MC Bedrock 1.26.45 support

### Bug Fixes
- TrustDuration: First violation no longer silently suppressed
- FlyA: Jump apex false positives fixed
- ReachB: Added touch-mode exemption
- SpeedB: Creative mode players no longer flagged
- Punishment: Kick/ban fires regardless of UseLegacyEvents setting
- Disconnect: Kick codes now display on Bedrock disconnect screen
- Handler: Fixed append mutating shared DebugModeList slice
- game/math.go: Fixed MinVec3/MaxVec3 OR?AND logic

### New Detections
- SpeedB — AirSpeed detection for modified in-air movement speed

### Chat Broadcasts
All players see flags in chat: [KillLime] PlayerName flagged Speed(B) [x3]
