package session

import "time"

// GoalReasonCode is a stable machine-readable Goal outcome or transition cause.
type GoalReasonCode string

// GoalBlockingIssue is one public verdict blocker without Loop/gate package coupling.
type GoalBlockingIssue struct {
	ID   string
	Note string
}

// GoalContextSnapshot is the checkpoint-pinned context telemetry projection.
type GoalContextSnapshot struct {
	State      string
	Used       *int64
	Size       *int64
	Ratio      *float64
	NudgeRatio float64
	ReportedAt *time.Time
}

// GoalVerdictSummary is the latest evaluated Goal verdict.
type GoalVerdictSummary struct {
	Outcome        string
	BlockingIssues []GoalBlockingIssue
	EvidenceRef    *string
	EvaluatedAt    time.Time
}

// GoalSnapshot is the canonical newest session-origin Goal projection.
type GoalSnapshot struct {
	RunID           string
	NodeID          string
	Objective       string
	OriginSessionID string
	BoundSessionID  string
	Status          string
	RunStatus       string
	Cause           *GoalReasonCode
	TurnsUsed       int
	TurnLimit       int
	Live            bool
	ContractSummary string
	LastVerdict     *GoalVerdictSummary
	Context         GoalContextSnapshot
}

// GoalTurn is one immutable-start, optionally terminal Goal audit row.
type GoalTurn struct {
	Seq            int64
	Generation     int64
	NodeID         string
	ItemIndex      int
	Turn           int
	PromptAttempt  int
	SessionID      string
	BindingHandle  string
	BindingEpoch   int64
	PromptID       string
	ResultStatus   *string
	ReasonCode     *GoalReasonCode
	StopReason     *string
	VerdictOutcome *string
	BlockingIssues []GoalBlockingIssue
	EvidenceRef    *string
	PromptRef      *string
	TokensUsed     *int64
	ActorKind      string
	ActorID        string
	StartedAt      time.Time
	EndedAt        *time.Time
}

// GoalTurnPage is one run-wide monotonic cursor page.
type GoalTurnPage struct {
	Turns        []GoalTurn
	NextAfterSeq *int64
}

// GoalPromptMeta classifies one engine-owned Goal prompt in persisted transcript events.
type GoalPromptMeta struct {
	Kind          string
	RunID         string
	NodeID        string
	Generation    int64
	ItemIndex     int
	Turn          *int
	PromptAttempt int
	PromptID      string
}
