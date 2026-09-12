package contextusage

import (
	"time"

	"github.com/compozy/compozy/internal/store"
)

type UsageEvent struct {
	Sequence int64
	At       time.Time
	TurnID   string
	Usage    store.TokenUsage
}

type Delivery struct {
	Sequence int64
	At       time.Time
	SentAt   time.Time
	TurnID   string
	Estimate string
	Spans    []Span
}

type Span struct {
	Key, Kind, Delivery, Name             string
	Bytes                                 int64
	Tokens                                *int64
	Unchanged, StartupDedup, HookModified bool
}

type Compaction struct {
	Sequence                 int64
	At                       time.Time
	TurnID                   string
	FromSequence, ToSequence int64
	Used, Size               int64
	Pressure                 float64
	Strategy                 string
	SpanArchived             bool
}

type SettledTurn struct {
	TurnID   string
	Sequence int64
}

type Input struct {
	UsageEvents   []UsageEvent
	Deliveries    []Delivery
	Compactions   []Compaction
	Settled       *SettledTurn
	CatalogWindow *int64
	Threshold     *float64
	Available     bool
}

type State string

const (
	StateReported      State = "reported"
	StateEstimatedSize State = "estimated_size"
	StateUnknown       State = "unknown"
	StateUnavailable   State = "unavailable"
)

type ContextUsage struct {
	State             State
	Used, Size        *int64
	Ratio             *float64
	SizeSource        string
	Stale             *bool
	Sequence          *int64
	ReportedTurnID    string
	ReportedAt        *time.Time
	PressureThreshold *float64
	Injected          *Injected
}

type Injected struct {
	Estimate string
	Tokens   int64
	Stale    bool
	Rows     []Row
}

type Row struct {
	Key, Label, Kind, Delivery, Name string
	OwnerKind                        string
	Bytes                            int64
	Tokens                           *int64
	DeliveredTurnID                  string
	DeliverySequence                 int64
	SentAt                           time.Time
	LastSeenTurnID                   string
	Unchanged, Stale, HookModified   bool
}

type Turn struct {
	TurnID        string
	Sequence      int64
	Usage         *store.TokenUsage
	UsageSequence *int64
	Injected      *Delivery
}
