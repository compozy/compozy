package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

// RunID identifies a durable task_run row without exposing raw lease credentials.
type RunID string

// Tx is the narrow SQL transaction surface exposed to generation-state finalizers.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// CoordinatorRunner builds a pure coordinator completion plan for one claimed coordinator run.
type CoordinatorRunner interface {
	Run(ctx context.Context, taskRunID RunID) (CoordinatorCompletionPlan, error)
}

// CoordinatorCompletionPlan contains all coordinator mutations that task applies atomically.
type CoordinatorCompletionPlan struct {
	NodeTasks           []CoordinatorTaskSpec
	Dependencies        []CoordinatorDependencySpec
	NodeRuns            []EnqueueSpec
	RunStops            []CoordinatorStopSpec
	Snapshot            GenerationSnapshot
	PostReserveSnapshot *GenerationSnapshot
	// GenerationInFlight means at least one current-generation node is still live.
	GenerationInFlight bool
	// NextCoordinator is emitted by the gate/carry-forward planner; the task_05 reconciler does not yet populate it.
	NextCoordinator *EnqueueSpec
	Terminal        *CoordinatorTerminal
	Yield           bool
	PostCommitWakes []CoordinatorWakeSpec
}

// CoordinatorTaskSpec describes a node task that must exist before node runs are queued.
type CoordinatorTaskSpec struct {
	TaskID      string          `json:"task_id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// CoordinatorDependencySpec describes a node task dependency that must exist before enqueueing runs.
type CoordinatorDependencySpec struct {
	TaskID          string         `json:"task_id"`
	DependsOnTaskID string         `json:"depends_on_task_id"`
	Kind            DependencyKind `json:"kind,omitempty"`
}

// EnqueueSpec describes one task_run reservation requested by the coordinator plan.
type EnqueueSpec struct {
	TaskID                       string              `json:"task_id"`
	RunID                        string              `json:"run_id,omitempty"`
	RunKind                      RunKind             `json:"run_kind,omitempty"`
	LoopRunID                    string              `json:"loop_run_id,omitempty"`
	IdempotencyKey               string              `json:"idempotency_key,omitempty"`
	DesignationGroupID           string              `json:"designation_group_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation,omitempty"`
	Metadata                     json.RawMessage     `json:"metadata,omitempty"`
}

// CoordinatorStopSpec describes a child loop_run stop requested by a pure coordinator plan.
type CoordinatorStopSpec struct {
	LoopRunID  string `json:"loop_run_id"`
	ReasonCode string `json:"reason_code,omitempty"`
}

// CoordinatorWakeSpec describes a coordinator wake to enqueue after the
// boundary transaction commits.
type CoordinatorWakeSpec struct {
	LoopRunID      string `json:"loop_run_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// GenerationSnapshot is opaque to internal/task; internal/loop owns its payload schema.
type GenerationSnapshot struct {
	LoopRunID  string `json:"loop_run_id,omitempty"`
	Generation int    `json:"generation,omitempty"`
	Payload    any    `json:"-"`
}

// CoordinatorTerminal describes the correlated run status selected at a generation boundary.
type CoordinatorTerminal struct {
	Status     string          `json:"status"`
	Cause      string          `json:"cause,omitempty"`
	ReasonCode string          `json:"reason_code,omitempty"`
	GateID     string          `json:"gate_id,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

// GenerationStateFinalizer writes generation state inside task's coordinator transaction.
type GenerationStateFinalizer interface {
	WriteGenerationSnapshot(ctx context.Context, tx Tx, snap GenerationSnapshot) error
}

// CoordinatorCompletion captures the token-fenced apply request for one coordinator plan.
type CoordinatorCompletion struct {
	RunID      string                    `json:"run_id"`
	ClaimToken string                    `json:"claim_token"`
	Plan       CoordinatorCompletionPlan `json:"plan"`
	Actor      ActorContext              `json:"actor"`
	Now        time.Time                 `json:"now"`
}

// CoordinatorCompletionResult records the durable state written by coordinator finalization.
type CoordinatorCompletionResult struct {
	Run          Run                     `json:"run"`
	EnqueuedRuns []Run                   `json:"enqueued_runs,omitempty"`
	LoopRunID    string                  `json:"loop_run_id,omitempty"`
	Context      json.RawMessage         `json:"context,omitempty"`
	Paused       bool                    `json:"paused,omitempty"`
	Terminal     bool                    `json:"terminal,omitempty"`
	TokensUsed   int64                   `json:"tokens_used,omitempty"`
	Settlement   *CompletedRunSettlement `json:"-"`
}

// Normalize returns a validated coordinator completion request with default time applied.
func (c CoordinatorCompletion) Normalize(defaultNow time.Time) (CoordinatorCompletion, error) {
	normalized := c
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.ClaimToken = strings.TrimSpace(normalized.ClaimToken)
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	normalized.Plan = normalized.Plan.Normalize()
	if err := normalized.Validate("coordinator_completion"); err != nil {
		return CoordinatorCompletion{}, err
	}
	return normalized, nil
}

// Validate reports whether the coordinator completion request is internally consistent.
func (c CoordinatorCompletion) Validate(path string) error {
	if err := validateLeaseRunToken(c.RunID, c.ClaimToken, path); err != nil {
		return err
	}
	if err := c.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: %s.actor", err, path)
	}
	return c.Plan.Validate(nestedPath(path, "plan"))
}

// Validate reports whether a coordinator completion plan is internally consistent.
func (p CoordinatorCompletionPlan) Validate(path string) error {
	if err := p.validateShape(path); err != nil {
		return err
	}
	if err := validateCoordinatorTaskSpecs(p.NodeTasks, path); err != nil {
		return err
	}
	if err := validateCoordinatorDependencySpecs(p.Dependencies, path); err != nil {
		return err
	}
	if err := validateCoordinatorEnqueueSpecs(p.NodeRuns, path); err != nil {
		return err
	}
	if err := validateCoordinatorStopSpecs(p.RunStops, path); err != nil {
		return err
	}
	if err := validateCoordinatorWakeSpecs(p.PostCommitWakes, path); err != nil {
		return err
	}
	if p.NextCoordinator != nil {
		if err := p.NextCoordinator.Validate(nestedPath(path, "next_coordinator")); err != nil {
			return err
		}
	}
	if p.Terminal != nil {
		if err := p.Terminal.Validate(nestedPath(path, "terminal")); err != nil {
			return err
		}
	}
	return nil
}

func (p CoordinatorCompletionPlan) validateShape(path string) error {
	if len(p.NodeRuns) == 0 &&
		len(p.RunStops) == 0 &&
		p.NextCoordinator == nil &&
		p.Terminal == nil &&
		!p.Yield {
		return fmt.Errorf(
			"%w: %s must enqueue work, terminalize, or schedule a next coordinator",
			ErrValidation,
			path,
		)
	}
	if p.Yield && (len(p.NodeRuns) > 0 ||
		len(p.RunStops) > 0 ||
		p.NextCoordinator != nil ||
		p.Terminal != nil) {
		return fmt.Errorf("%w: %s yield must not enqueue work or terminalize", ErrValidation, path)
	}
	return nil
}

func validateCoordinatorTaskSpecs(specs []CoordinatorTaskSpec, path string) error {
	for idx, spec := range specs {
		if err := spec.Validate(nestedPath(path, fmt.Sprintf("node_tasks[%d]", idx))); err != nil {
			return err
		}
	}
	return nil
}

func validateCoordinatorDependencySpecs(specs []CoordinatorDependencySpec, path string) error {
	for idx, spec := range specs {
		if err := spec.Validate(nestedPath(path, fmt.Sprintf("dependencies[%d]", idx))); err != nil {
			return err
		}
	}
	return nil
}

func validateCoordinatorEnqueueSpecs(specs []EnqueueSpec, path string) error {
	for idx, spec := range specs {
		if err := spec.Validate(nestedPath(path, fmt.Sprintf("node_runs[%d]", idx))); err != nil {
			return err
		}
	}
	return nil
}

func validateCoordinatorStopSpecs(specs []CoordinatorStopSpec, path string) error {
	for idx, spec := range specs {
		if err := spec.Validate(nestedPath(path, fmt.Sprintf("loop_stops[%d]", idx))); err != nil {
			return err
		}
	}
	return nil
}

// Normalize returns a coordinator plan with normalized enqueue specs and terminal data.
func (p CoordinatorCompletionPlan) Normalize() CoordinatorCompletionPlan {
	normalized := p
	normalized.Snapshot.LoopRunID = strings.TrimSpace(normalized.Snapshot.LoopRunID)
	if normalized.PostReserveSnapshot != nil {
		snapshot := *normalized.PostReserveSnapshot
		snapshot.LoopRunID = strings.TrimSpace(snapshot.LoopRunID)
		normalized.PostReserveSnapshot = &snapshot
	}
	for idx := range normalized.NodeTasks {
		normalized.NodeTasks[idx] = normalized.NodeTasks[idx].Normalize()
	}
	for idx := range normalized.Dependencies {
		normalized.Dependencies[idx] = normalized.Dependencies[idx].Normalize()
	}
	for idx := range normalized.NodeRuns {
		normalized.NodeRuns[idx] = normalized.NodeRuns[idx].Normalize()
	}
	for idx := range normalized.RunStops {
		normalized.RunStops[idx] = normalized.RunStops[idx].Normalize()
	}
	for idx := range normalized.PostCommitWakes {
		normalized.PostCommitWakes[idx] = normalized.PostCommitWakes[idx].Normalize()
	}
	if normalized.NextCoordinator != nil {
		next := normalized.NextCoordinator.Normalize()
		normalized.NextCoordinator = &next
	}
	if normalized.Terminal != nil {
		terminal := normalized.Terminal.Normalize()
		normalized.Terminal = &terminal
	}
	return normalized
}

func validateCoordinatorWakeSpecs(specs []CoordinatorWakeSpec, path string) error {
	for idx, spec := range specs {
		if err := spec.Validate(nestedPath(path, fmt.Sprintf("post_commit_wakes[%d]", idx))); err != nil {
			return err
		}
	}
	return nil
}

// Normalize returns a normalized coordinator task spec.
func (s CoordinatorTaskSpec) Normalize() CoordinatorTaskSpec {
	normalized := s
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.Title = strings.TrimSpace(normalized.Title)
	normalized.Description = strings.TrimSpace(normalized.Description)
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	return normalized
}

// Validate reports whether a coordinator task spec has enough data to create a node task.
func (s CoordinatorTaskSpec) Validate(path string) error {
	if strings.TrimSpace(s.TaskID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "task_id"))
	}
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "title"))
	}
	if err := ValidateMetadataSize(s.Metadata, nestedPath(path, "metadata")); err != nil {
		return err
	}
	return nil
}

// Normalize returns a normalized coordinator dependency spec.
func (s CoordinatorDependencySpec) Normalize() CoordinatorDependencySpec {
	normalized := s
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.DependsOnTaskID = strings.TrimSpace(normalized.DependsOnTaskID)
	normalized.Kind = normalized.Kind.Normalize()
	if normalized.Kind == "" {
		normalized.Kind = DependencyKindBlocks
	}
	return normalized
}

// Validate reports whether a coordinator dependency spec has a valid dependency edge.
func (s CoordinatorDependencySpec) Validate(path string) error {
	if strings.TrimSpace(s.TaskID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "task_id"))
	}
	if strings.TrimSpace(s.DependsOnTaskID) == "" {
		return fmt.Errorf(
			"%w: %s is required",
			ErrValidation,
			nestedPath(path, "depends_on_task_id"),
		)
	}
	if strings.TrimSpace(s.TaskID) == strings.TrimSpace(s.DependsOnTaskID) {
		return fmt.Errorf(
			"%w: %s must not equal %s",
			ErrValidation,
			nestedPath(path, "task_id"),
			nestedPath(path, "depends_on_task_id"),
		)
	}
	kind := s.Kind.Normalize()
	if kind == "" {
		kind = DependencyKindBlocks
	}
	if err := kind.Validate(nestedPath(path, "kind")); err != nil {
		return err
	}
	return nil
}

// Normalize returns a normalized enqueue spec.
func (s EnqueueSpec) Normalize() EnqueueSpec {
	normalized := s
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.RunKind = normalizeRunKindOrDefault(normalized.RunKind)
	normalized.LoopRunID = strings.TrimSpace(normalized.LoopRunID)
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	normalized.DesignationGroupID = strings.TrimSpace(normalized.DesignationGroupID)
	if normalized.ResolvedNetworkParticipation != nil {
		normalized.ResolvedNetworkParticipation = participation.CloneSpec(
			*normalized.ResolvedNetworkParticipation,
		)
	}
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	return normalized
}

// Validate reports whether an enqueue spec can become a queue reservation.
func (s EnqueueSpec) Validate(path string) error {
	if strings.TrimSpace(s.TaskID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "task_id"))
	}
	runKind := normalizeRunKindOrDefault(s.RunKind)
	if err := runKind.Validate(nestedPath(path, "run_kind")); err != nil {
		return err
	}
	if runKind == RunKindCoordinator && strings.TrimSpace(s.LoopRunID) == "" {
		return fmt.Errorf(
			"%w: %s is required for coordinator runs",
			ErrValidation,
			nestedPath(path, "loop_run_id"),
		)
	}
	if s.ResolvedNetworkParticipation != nil {
		if err := participation.ValidateSpec(*s.ResolvedNetworkParticipation); err != nil {
			return fmt.Errorf(
				"%w: %s: %v",
				ErrValidation,
				nestedPath(path, "resolved_network_participation"),
				err,
			)
		}
	}
	if err := ValidateMetadataSize(s.Metadata, nestedPath(path, "metadata")); err != nil {
		return err
	}
	return nil
}

// Normalize returns a normalized loop stop request.
func (s CoordinatorStopSpec) Normalize() CoordinatorStopSpec {
	normalized := s
	normalized.LoopRunID = strings.TrimSpace(normalized.LoopRunID)
	normalized.ReasonCode = strings.TrimSpace(normalized.ReasonCode)
	return normalized
}

// Validate reports whether a loop stop request names a child loop run.
func (s CoordinatorStopSpec) Validate(path string) error {
	if strings.TrimSpace(s.LoopRunID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "loop_run_id"))
	}
	return nil
}

// Normalize returns a normalized post-commit wake request.
func (s CoordinatorWakeSpec) Normalize() CoordinatorWakeSpec {
	return CoordinatorWakeSpec{
		LoopRunID:      strings.TrimSpace(s.LoopRunID),
		IdempotencyKey: strings.TrimSpace(s.IdempotencyKey),
	}
}

// Validate reports whether a post-commit wake has a stable target.
func (s CoordinatorWakeSpec) Validate(path string) error {
	if strings.TrimSpace(s.LoopRunID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "loop_run_id"))
	}
	if strings.TrimSpace(s.IdempotencyKey) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "idempotency_key"))
	}
	return nil
}

// Normalize returns normalized coordinator terminal data.
func (t CoordinatorTerminal) Normalize() CoordinatorTerminal {
	normalized := t
	normalized.Status = strings.TrimSpace(normalized.Status)
	normalized.Cause = strings.TrimSpace(normalized.Cause)
	normalized.ReasonCode = strings.TrimSpace(normalized.ReasonCode)
	normalized.GateID = strings.TrimSpace(normalized.GateID)
	normalized.Details = normalizeRawJSON(normalized.Details)
	return normalized
}

// Validate reports whether coordinator terminal data has the minimum persisted shape.
func (t CoordinatorTerminal) Validate(path string) error {
	status := strings.TrimSpace(t.Status)
	if status == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "status"))
	}
	if err := ValidatePayloadSize(t.Details, nestedPath(path, "details")); err != nil {
		return err
	}
	return nil
}
