package heartbeat

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
)

type InstanceWakeState struct {
	InstanceID, NextWakeAt, LastWakeAt, LastDigest, LastDigestAt, AutomationCursor string
	OccurrenceID, TriageSessionID, OccurrenceStartedAt                             string
	QuietStreak, FailureStreak                                                     int64
	Canceled                                                                       bool
}
type AdmitRequest struct {
	InstanceID, OccurrenceID string
	Now, NextWakeAt          time.Time
	ManualCheck              bool
	MaxConcurrent            int
}
type FinishRequest struct {
	QuietBackoffCap                                          time.Duration
	FatigueLimit                                             int64
	Interval, MaxInterval                                    time.Duration
	InstanceID, OccurrenceID, Source, Result, Reason, Detail string
	Digest, AutomationCursor                                 string
	DeniedCalls                                              int64
	Now, NextWakeAt, ExpiresAt                               time.Time
	Report                                                   *agentinstance.ReportInput
}
type SkipRequest struct {
	InstanceID, ExpectedDue, Reason, Detail string
	Now, NextWakeAt, ExpiresAt              time.Time
}
type InstanceWakeStore interface {
	Due(context.Context, time.Time, int) ([]InstanceWakeState, error)
	Inflight(context.Context) ([]InstanceWakeState, error)
	WakeState(context.Context, string) (InstanceWakeState, error)
	Admit(context.Context, AdmitRequest) (bool, error)
	Skip(context.Context, SkipRequest) (bool, error)
	BindTriage(context.Context, string, string, string) (bool, error)
	Finish(context.Context, FinishRequest) (bool, error)
}
