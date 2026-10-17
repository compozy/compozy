package wire

const (
	AckGrainBytes    = 16 << 10
	AckHighWatermark = 256 << 10
	AckLowWatermark  = 64 << 10
	AckPendingLimit  = 1 << 20
	DropQueueLimit   = 64 << 10
)

type Flow string

const (
	FlowAck  Flow = "ack"
	FlowDrop Flow = "drop"
)
