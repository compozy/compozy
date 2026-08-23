package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
)

// LoopGenerationOrigin identifies why a generation exists.
type LoopGenerationOrigin string

const (
	LoopGenerationOriginInitial            LoopGenerationOrigin = "initial"
	LoopGenerationOriginStopWhen           LoopGenerationOrigin = "stop_when"
	LoopGenerationOriginReattempt          LoopGenerationOrigin = "reattempt"
	LoopGenerationOriginGateRevise         LoopGenerationOrigin = "gate_revise"
	LoopGenerationOriginGateNextGeneration LoopGenerationOrigin = "gate_next_generation"
	LoopGenerationOriginDoDRetry           LoopGenerationOrigin = "dod_retry"
	LoopGenerationOriginRatchetRestore     LoopGenerationOrigin = "ratchet_restore"
	LoopGenerationOriginRequeue            LoopGenerationOrigin = "requeue"
	LoopGenerationOriginOperatorRerun      LoopGenerationOrigin = "operator_rerun"
	LoopGenerationOriginForkSeed           LoopGenerationOrigin = "fork_seed"
)

type LoopForkRef struct {
	RunID      string `json:"run_id"`
	Generation int64  `json:"generation"`
}

// LoopGateVerdictOutcome is the machine gate verdict vocabulary.
type LoopGateVerdictOutcome string

const (
	LoopGateVerdictApproved         LoopGateVerdictOutcome = "approved"
	LoopGateVerdictRejected         LoopGateVerdictOutcome = "rejected"
	LoopGateVerdictAwaitingApproval LoopGateVerdictOutcome = "awaiting_approval"
	LoopGateVerdictBlocked          LoopGateVerdictOutcome = "blocked"
	LoopGateVerdictError            LoopGateVerdictOutcome = "error"
	LoopGateVerdictTimeout          LoopGateVerdictOutcome = "timeout"
	LoopGateVerdictInvalidOutput    LoopGateVerdictOutcome = "invalid_output"
)

// LoopRunPayload is the public loop_run aggregate projection.
type LoopRunPayload struct {
	ID                           string                `json:"id"`
	WorkspaceID                  string                `json:"workspace_id"`
	LoopName                     string                `json:"loop_name"`
	Status                       LoopRunStatus         `json:"status"`
	Historical                   bool                  `json:"historical"`
	CompletionState              LoopCompletionState   `json:"completion_state"`
	Generation                   int64                 `json:"generation"`
	BestGeneration               *int64                `json:"best_generation,omitempty"`
	BestScore                    *float64              `json:"best_score,omitempty"`
	ForkedFrom                   *LoopForkRef          `json:"forked_from,omitempty"`
	Forks                        []LoopForkRef         `json:"forks"`
	ReattemptStrategy            LoopReattemptStrategy `json:"reattempt_strategy"`
	CreatedAt                    time.Time             `json:"created_at"`
	StartedAt                    time.Time             `json:"started_at"`
	LastProgressAt               time.Time             `json:"last_progress_at"`
	StartedByKind                string                `json:"started_by_kind,omitempty"`
	StartedByRef                 string                `json:"started_by_ref,omitempty"`
	StartedOriginKind            string                `json:"started_origin_kind,omitempty"`
	StartedOriginRef             string                `json:"started_origin_ref,omitempty"`
	DefinitionVersion            int                   `json:"definition_version"`
	DefinitionDigest             string                `json:"definition_digest,omitempty"`
	ActiveGateID                 string                `json:"active_gate_id,omitempty"`
	BudgetApprovalSeq            int                   `json:"budget_approval_seq,omitempty"`
	StartMetadata                map[string]any        `json:"start_metadata,omitempty"`
	IterationCap                 int                   `json:"iteration_cap"`
	BudgetTokens                 int                   `json:"budget_tokens"`
	BudgetWallSec                int                   `json:"budget_wall_sec"`
	BudgetOnExceeded             LoopBudgetExceeded    `json:"budget_on_exceeded"`
	TokensUsed                   int64                 `json:"tokens_used"`
	ParentLoopRunID              string                `json:"parent_loop_run_id,omitempty"`
	PauseRequested               bool                  `json:"pause_requested"`
	Inputs                       map[string]any        `json:"inputs,omitempty"`
	ResolvedNetworkParticipation *participation.Spec   `json:"resolved_network_participation"`
	Attention                    *LoopRunAttention     `json:"attention,omitempty"`
	Progress                     LoopRunProgress       `json:"progress"`
}

// LoopRunAttention summarizes operator action required by one run.
type LoopRunAttention struct {
	Kind  string    `json:"kind"`
	Count int       `json:"count"`
	Since time.Time `json:"since"`
}

// LoopRunProgress summarizes the current round's completed steps.
type LoopRunProgress struct {
	Round      int `json:"round"`
	StepsDone  int `json:"steps_done"`
	StepsTotal int `json:"steps_total"`
}

// LoopRunsResponse lists workspace-scoped loop runs.
type LoopRunsResponse struct {
	Runs       []LoopRunPayload         `json:"runs"`
	Aggregates LoopRunsAggregatePayload `json:"aggregates"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}

// LoopRunsAggregatePayload summarizes the returned run set.
type LoopRunsAggregatePayload struct {
	Total      int `json:"total"`
	Live       int `json:"live"`
	Terminal   int `json:"terminal"`
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
	Historical int `json:"historical"`
}

// LoopRunResponse returns one run with generation detail.
type LoopRunResponse struct {
	Run                  LoopRunPayload             `json:"run"`
	ExecutedDefinition   *LoopDefinitionDocument    `json:"executed_definition,omitempty"`
	MaterializedContract LoopContract               `json:"materialized_contract"`
	Generations          []LoopGenerationPayload    `json:"generations,omitempty"`
	NodeControls         []LoopNodeControlPayload   `json:"node_controls"`
	Waits                []LoopNodeWaitPayload      `json:"waits"`
	Requests             []LoopRequestPayload       `json:"requests"`
	Amendments           []LoopNodeAmendmentPayload `json:"amendments"`
	// WatchEvents is the parked watch-events read-model (present only while dormant).
	WatchEvents *LoopWatchEventsState `json:"watch_events,omitempty"`
}

// LoopGenerationPayload groups durable provenance, verdicts, and node state by generation.
type LoopGenerationPayload struct {
	Generation       int64                    `json:"generation"`
	ParentGeneration int64                    `json:"parent_generation"`
	Origin           LoopGenerationOrigin     `json:"origin"`
	RouteCauses      []LoopRouteCausePayload  `json:"route_causes"`
	Verdicts         []LoopGateVerdictPayload `json:"verdicts"`
	Outputs          []LoopGenerationOutput   `json:"outputs"`
}

// LoopRouteCausePayload is one durable route decision in a generation.
type LoopRouteCausePayload struct {
	NodeID      string    `json:"node_id"`
	ItemIndex   int       `json:"item_index"`
	Route       string    `json:"route"`
	Cause       string    `json:"cause"`
	MatchedWhen string    `json:"matched_when,omitempty"`
	Default     bool      `json:"default,omitempty"`
	At          time.Time `json:"at"`
}

// LoopGateVerdictPayload is one queryable machine verdict in generation detail.
type LoopGateVerdictPayload struct {
	GateID         string                           `json:"gate_id"`
	ItemIndex      int                              `json:"item_index"`
	Outcome        LoopGateVerdictOutcome           `json:"outcome"`
	Score          *float64                         `json:"score,omitempty"`
	RouteCauseRank *int                             `json:"route_cause_rank,omitempty"`
	BlockingIssues []LoopGateBlockingIssuePayload   `json:"blocking_issues"`
	Criteria       []LoopGateCriterionDetailPayload `json:"criteria"`
}

// LoopGateCriterionDetailPayload is one durable criterion diagnostic in generation detail.
type LoopGateCriterionDetailPayload struct {
	ID             string                         `json:"id"`
	Type           string                         `json:"type"`
	Outcome        LoopGateVerdictOutcome         `json:"outcome"`
	Passed         bool                           `json:"passed"`
	Broken         bool                           `json:"broken,omitempty"`
	ExitCode       *int                           `json:"exit_code,omitempty"`
	Stdout         string                         `json:"stdout,omitempty"`
	Stderr         string                         `json:"stderr,omitempty"`
	Score          *float64                       `json:"score,omitempty"`
	Evidence       json.RawMessage                `json:"evidence,omitempty"`
	BlockingIssues []LoopGateBlockingIssuePayload `json:"blocking_issues,omitempty"`
	Warnings       []LoopGateDiagnosticWarning    `json:"warnings,omitempty"`
	Payload        json.RawMessage                `json:"payload,omitempty"`
}

// LoopGateDiagnosticWarning is one non-terminal criterion diagnostic.
type LoopGateDiagnosticWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// LoopGenerationOutput is one public generation output row.
type LoopGenerationOutput struct {
	Generation      int64                `json:"generation,omitempty"`
	NodeID          string               `json:"node_id"`
	ItemIndex       int                  `json:"item_index,omitempty"`
	Status          string               `json:"status"`
	OutputRef       string               `json:"output_ref,omitempty"`
	TaskRunID       string               `json:"task_run_id,omitempty"`
	SessionID       string               `json:"session_id,omitempty"`
	ChildLoopRunID  string               `json:"child_loop_run_id,omitempty"`
	ResolvedRuntime *LoopResolvedRuntime `json:"resolved_runtime,omitempty"`
	Attempt         int                  `json:"attempt,omitempty"`
	NextAttemptAt   *time.Time           `json:"next_attempt_at,omitempty"`
	FailureClass    string               `json:"failure_class,omitempty"`
	Disposition     string               `json:"disposition,omitempty"`
}

// LoopRunEventPayload is one SSE/audit event for a loop run.
type LoopRunEventPayload struct {
	ID          string           `json:"id"`
	LoopRunID   string           `json:"loop_run_id"`
	WorkspaceID string           `json:"workspace_id"`
	Seq         int64            `json:"seq"`
	Kind        LoopRunEventKind `json:"kind"`
	Payload     json.RawMessage  `json:"payload"`
	At          time.Time        `json:"at"`
}

// LoopGenerationStartedRunEventPayload is the typed generation_started event envelope.
type LoopGenerationStartedRunEventPayload struct {
	ID          string                            `json:"id"`
	LoopRunID   string                            `json:"loop_run_id"`
	WorkspaceID string                            `json:"workspace_id"`
	Seq         int64                             `json:"seq"`
	Kind        LoopRunEventKind                  `json:"kind"`
	Payload     LoopGenerationStartedEventPayload `json:"payload"`
	At          time.Time                         `json:"at"`
}

// LoopGateVerdictRunEventPayload is the typed gate_verdict event envelope.
type LoopGateVerdictRunEventPayload struct {
	ID          string                      `json:"id"`
	LoopRunID   string                      `json:"loop_run_id"`
	WorkspaceID string                      `json:"workspace_id"`
	Seq         int64                       `json:"seq"`
	Kind        LoopRunEventKind            `json:"kind"`
	Payload     LoopGateVerdictEventPayload `json:"payload"`
	At          time.Time                   `json:"at"`
}

// LoopRouteTakenRunEventPayload is the typed route_taken event envelope.
type LoopRouteTakenRunEventPayload struct {
	ID          string                     `json:"id"`
	LoopRunID   string                     `json:"loop_run_id"`
	WorkspaceID string                     `json:"workspace_id"`
	Seq         int64                      `json:"seq"`
	Kind        LoopRunEventKind           `json:"kind"`
	Payload     LoopRouteTakenEventPayload `json:"payload"`
	At          time.Time                  `json:"at"`
}

// LoopGenerationStartedEventPayload is the exact generation_started SSE payload.
type LoopGenerationStartedEventPayload struct {
	Generation        int64                 `json:"generation"`
	ParentGeneration  int64                 `json:"parent_generation"`
	Origin            LoopGenerationOrigin  `json:"origin"`
	ReattemptStrategy LoopReattemptStrategy `json:"reattempt_strategy"`
	LoopName          string                `json:"loop_name"`
}

// LoopGateVerdictEventPayload is the exact gate_verdict SSE payload.
type LoopGateVerdictEventPayload struct {
	NodeID         string                          `json:"node_id"`
	Generation     int64                           `json:"generation"`
	GateID         string                          `json:"gate_id"`
	ItemIndex      int                             `json:"item_index"`
	Verdict        string                          `json:"verdict"`
	Reason         string                          `json:"reason"`
	Route          string                          `json:"route"`
	BlockingIssues []LoopGateBlockingIssuePayload  `json:"blocking_issues"`
	Criteria       []LoopGateCriterionEventPayload `json:"criteria"`
	Score          *float64                        `json:"score,omitempty"`
	BestGeneration *int64                          `json:"best_generation,omitempty"`
}

// LoopRouteTakenEventPayload is the exact route_taken SSE payload.
type LoopRouteTakenEventPayload struct {
	Generation  int64  `json:"generation"`
	NodeID      string `json:"node_id"`
	ItemIndex   int    `json:"item_index"`
	Route       string `json:"route"`
	Cause       string `json:"cause"`
	MatchedWhen string `json:"matched_when,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// LoopGateBlockingIssuePayload is one sanitized gate_verdict blocker.
type LoopGateBlockingIssuePayload struct {
	ID   string `json:"id"`
	Note string `json:"note"`
}

// LoopGateCriterionEventPayload is one sanitized gate_verdict criterion result.
type LoopGateCriterionEventPayload struct {
	ID     string   `json:"id"`
	Type   string   `json:"type"`
	Status string   `json:"status"`
	Note   string   `json:"note"`
	Score  *float64 `json:"score,omitempty"`
}

// ApproveLoopRunRequest applies a human-gate decision.
type ApproveLoopRunRequest struct {
	GateID   string           `json:"gate_id"`
	Decision LoopGateDecision `json:"decision"`
}
