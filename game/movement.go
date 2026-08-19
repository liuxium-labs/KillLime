package game

const (
	DefaultJumpHeight           = float32(0.42)
	DefaultAirFriction          = float32(0.91)
	DefaultBlockFriction        = float32(0.6)
	NormalGravityMultiplier     = float32(0.98)
	LevitationGravityMultiplier = float32(0.05)
	NormalGravity               = float32(0.08)
	SlowFallingGravity          = float32(0.01)
	// The maximum fall speed the vanilla physics engine allows (gravity)
	// applied with the 0.98 multiplier until it reaches the cap
	// (0.08*0.98 / (1-0.98) = 3.92 blocks/tick). A client cannot send a
	// downward Y delta faster than this without cheating.
	TerminalVelocity = float32(3.92)
	StepHeight                  = float32(0.6)
	SlideOffsetMultiplier       = float32(0.4)
	SlimeBounceMultiplier       = float32(-1)
	BedBounceMultiplier         = float32(-0.66)
	// This can be validated in Mob::ascendLadder()
	ClimbSpeed           = float32(0.2)
	MaxConsumingImpulse  = float32(0.1225)
	MaxSneakImpulse      = float32(0.3)
	MaxNormalizedImpulse = float32(0.70710678118) // 1/sqrt(2)

	DefaultPlayerHeightOffset  = float32(1.62)
	SneakingPlayerHeightOffset = float32(1.27)

	JumpDelayTicks  = 10
	GlideBoostTicks = 20
)
