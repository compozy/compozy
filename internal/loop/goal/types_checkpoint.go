// Package goal defines the durable convergence contracts implemented by the Loop store.
package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
)

// TurnKey identifies one workspace-owned Goal node instance.
type TurnKey struct {
	WorkspaceID loop.WorkspaceID
	LoopRunID   loop.RunID
	Generation  int
	NodeID      dsl.NodeID
	ItemIndex   int
}

// Validate enforces the complete workspace-safe Goal identity.
func (k TurnKey) Validate() error {
	if strings.TrimSpace(string(k.WorkspaceID)) == "" {
		return fmt.Errorf("%w: goal workspace_id is required", loop.ErrValidation)
	}
	if strings.TrimSpace(string(k.LoopRunID)) == "" {
		return fmt.Errorf("%w: goal loop_run_id is required", loop.ErrValidation)
	}
	if k.Generation < 1 {
		return fmt.Errorf("%w: goal generation must be at least 1", loop.ErrValidation)
	}
	if strings.TrimSpace(string(k.NodeID)) == "" {
		return fmt.Errorf("%w: goal node_id is required", loop.ErrValidation)
	}
	if k.ItemIndex < 0 {
		return fmt.Errorf("%w: goal item_index must be nonnegative", loop.ErrValidation)
	}
	return nil
}

// Checkpoint is the mutable CAS state for one Goal node instance.
type Checkpoint struct {
	Key                        TurnKey
	ControlEpoch               int64
	Phase                      string
	Status                     string
	ControlCause               loop.ReasonCode
	TurnsUsed                  int
	TurnLimit                  int
	BrokenStreak               int
	RecoveryStreak             int
	TaskRunID                  string
	QueueEntryID               string
	PromptID                   string
	PromptKind                 string
	PromptAttempt              int
	SessionID                  string
	BindingHandle              string
	BindingEpoch               int64
	ContextState               string
	UsageSequence              *int64
	UsagePendingAfterSequence  *int64
	CompactionBaselineUsed     *int64
	CompactionRecoveryRequired bool
	ControlGrant               *ControlGrant
	JudgeAttemptID             string
	ReportIntent               *ReportIntent
	CompactionCancel           *CompactionCancelIntent
	ControlActorKind           string
	ControlActorID             string
	ControlRequestedAt         *time.Time
	ContextNudgeRatio          float64
	UpdatedAt                  time.Time
}

// CreateCheckpointRequest seeds one Goal checkpoint without overwriting existing state.
type CreateCheckpointRequest struct {
	Checkpoint Checkpoint
}

// ControlGrantKind classifies one checkpoint-local approval.
type ControlGrantKind string

const (
	ControlGrantTurnExtension ControlGrantKind = "turn-extension"
	ControlGrantBudget        ControlGrantKind = "budget"
	ControlGrantReseed        ControlGrantKind = "reseed"
	ControlGrantPlainResume   ControlGrantKind = "plain-resume"
)

// ControlGrantScope constrains the exact effect authorized by one approval.
type ControlGrantScope string

const (
	ControlGrantScopeTurnLimit     ControlGrantScope = "turn-limit"
	ControlGrantScopeSettleCurrent ControlGrantScope = "settle-current"
	ControlGrantScopeWorkAndSettle ControlGrantScope = "work-and-settle"
	ControlGrantScopeRotateBinding ControlGrantScope = "rotate-binding"
	ControlGrantScopeReactivate    ControlGrantScope = "reactivate"
)

// ControlGrant is the latest monotonic checkpoint-local approval identity.
type ControlGrant struct {
	ID       int64
	Kind     ControlGrantKind
	Cause    loop.ReasonCode
	Turn     int
	Scope    ControlGrantScope
	Consumed bool
}

// GrantRequest atomically advances control and records a typed approval.
type GrantRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	TurnIncrement        int
	Kind                 ControlGrantKind
	Cause                loop.ReasonCode
	Turn                 int
	Scope                ControlGrantScope
	ActorKind            string
	ActorID              string
}

// ControlCheckpointRequest writes one typed boundary under control CAS.
type ControlCheckpointRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	ExpectedPhase        string
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	Disposition          loop.ActionDisposition
	Status               string
	Cause                loop.ReasonCode
	ActorKind            string
	ActorID              string
}

// ReportIntent is the durable one-prompt request written by goal_report.
type ReportIntent struct {
	PromptID     string
	Status       string
	EvidenceRef  string
	BindingEpoch int64
	ActorKind    string
	ActorID      string
	RecordedAt   time.Time
}

// RecordReportIntentRequest records one report under prompt/control/binding CAS.
type RecordReportIntentRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	PromptID             string
	Status               string
	EvidenceRef          string
	ActorKind            string
	ActorID              string
}

// CompactionCancelIntent persists the timeout cancellation decision before effect.
type CompactionCancelIntent struct {
	PromptID    string
	Cause       string
	RequestedAt time.Time
}

// BeginCompactionCancelRequest records one correlated cancel intent.
type BeginCompactionCancelRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	TaskRunID            string
	QueueEntryID         string
	PromptID             string
	Cause                string
}

// CheckpointStore is the durable checkpoint/control subset used by Goal execution.
type CheckpointStore interface {
	CreateCheckpoint(context.Context, CreateCheckpointRequest) (Checkpoint, error)
	LoadCheckpoint(context.Context, TurnKey) (Checkpoint, error)
	CheckpointControl(context.Context, ControlCheckpointRequest) error
	GrantAndReactivate(context.Context, GrantRequest) error
	RecordReportIntent(context.Context, RecordReportIntentRequest) (ReportIntent, error)
	BeginCompactionCancel(context.Context, BeginCompactionCancelRequest) error
	RecordContextUsage(context.Context, RecordContextUsageRequest) (Checkpoint, error)
}
