package lua

type DetectionState struct {
	Position                [3]float64
	LastPos                 [3]float64
	VelX, VelY, VelZ        float64
	Yaw, Pitch              float64
	HeadYaw                 float64
	OnGround                bool
	AirTicks                int
	FlyTicks                int
	SprintTicks             int
	SwimTicks               int
	ClimbTicks              int
	Action                  int
	HyperSpeed              float64
	HoverTime               int
	StepHeight              float64
	AllowFlying             bool
	Collision               bool
	FallDistance             float64
	Speed                   float64
	VSpeed                  float64
	YawDiff, PitchDiff      float64
	HeadRotDiff             float64
	Ticks                   int64
}

type PlayerEvent struct {
	Name        string
	XUID        string
	DisplayName string
	Version     int32
	GameMode    int32
	InputMode   uint32
	TicksAlive  int64
	IsOp        bool
	Perms       uint64
	Detection   *DetectionState
}
