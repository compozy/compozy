package loop

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

// RunHistoryEvent is one already-emitted run event replayed by a history import.
type RunHistoryEvent struct {
	Kind    RunEventKind
	Payload any
	At      time.Time
}

// RunHistoryVerdict pairs one machine gate verdict with the moment it was decided.
type RunHistoryVerdict struct {
	Generation int
	Intent     gate.VerdictIntent
	DecidedAt  time.Time
}

// RunHistoryBest carries the ratchet state a finished run settled on.
type RunHistoryBest struct {
	Generation int64
	Score      float64
}

// RunHistoryGoalTurn is one completed Goal turn replayed as immutable audit history.
type RunHistoryGoalTurn struct {
	Seq             int64
	Generation      int
	NodeID          NodeID
	ItemIndex       int
	Turn            int
	SessionID       string
	BindingHandle   string
	BindingEpoch    int64
	PromptID        string
	PromptAttempt   int
	UsageBaseTokens int64
	StopReason      ActionStopReason
	VerdictOutcome  gate.VerdictOutcome
	BlockingIssues  json.RawMessage
	Criteria        json.RawMessage
	Warnings        json.RawMessage
	EvidenceRef     string
	PromptRef       string
	TokensUsed      int64
	ActorKind       string
	ActorID         string
	StartedAt       time.Time
	EndedAt         time.Time
}

// RunHistoryGeneration is the already-settled state of one generation.
type RunHistoryGeneration struct {
	Intent      GenerationIntent
	CreatedAt   time.Time
	Outputs     []GenerationOutput
	OutputBlobs []GenerationOutputBlob
	Attempts    []NodeAttempt
	Controls    []NodeControlMutation
	Waits       []NodeWaitIntent
	Requests    []RequestIntent
	Verdicts    []RunHistoryVerdict
}

// RunHistorySnapshot is the caller-authored ledger of one already-executed run.
type RunHistorySnapshot struct {
	Run         Run
	Generations []RunHistoryGeneration
	Decisions   []GateDecisionRecord
	Events      []RunHistoryEvent
	GoalTurns   []RunHistoryGoalTurn
	Best        *RunHistoryBest
	Actor       task.ActorContext
}

// RunHistoryImport is the explicit bootstrap-only boundary for one already-executed
// historical Loop run. Runtime execution must go through the coordinator instead.
type RunHistoryImport struct {
	snapshot RunHistorySnapshot
}

// NewRunHistoryImport validates and isolates one executed run ledger before persistence.
func NewRunHistoryImport(snapshot *RunHistorySnapshot) (RunHistoryImport, error) {
	if snapshot == nil {
		return RunHistoryImport{}, fmt.Errorf("%w: run history snapshot is required", ErrValidation)
	}
	if err := snapshot.Actor.Validate(); err != nil {
		return RunHistoryImport{}, err
	}
	if !snapshot.Actor.Authority.Write {
		return RunHistoryImport{}, fmt.Errorf(
			"%w: run history import requires write authority",
			task.ErrPermissionDenied,
		)
	}
	if err := validateRunHistoryRun(snapshot.Run); err != nil {
		return RunHistoryImport{}, err
	}
	highest, err := validateRunHistoryGenerations(snapshot.Generations, snapshot.Run)
	if err != nil {
		return RunHistoryImport{}, err
	}
	if snapshot.Run.Generation > highest {
		return RunHistoryImport{}, fmt.Errorf(
			"%w: run generation cursor %d exceeds highest imported generation %d",
			ErrValidation,
			snapshot.Run.Generation,
			highest,
		)
	}
	if err := validateRunHistoryBest(snapshot.Best, snapshot.Run.Generation); err != nil {
		return RunHistoryImport{}, err
	}
	if err := validateRunHistoryDecisions(snapshot.Decisions, snapshot.Run, highest); err != nil {
		return RunHistoryImport{}, err
	}
	if err := validateRunHistoryEvents(snapshot.Events, snapshot.Run.CreatedAt); err != nil {
		return RunHistoryImport{}, err
	}
	if err := validateRunHistoryGoalTurns(snapshot.GoalTurns, snapshot.Run); err != nil {
		return RunHistoryImport{}, err
	}
	isolated := cloneRunHistorySnapshot(snapshot)
	if err := normalizeRunHistoryEventPayloads(isolated.Events); err != nil {
		return RunHistoryImport{}, err
	}
	isolated.Run.Historical = true
	return RunHistoryImport{snapshot: isolated}, nil
}

func validateRunHistoryGoalTurns(turns []RunHistoryGoalTurn, run Run) error {
	for index, turn := range turns {
		if err := validateRunHistoryGoalTurn(turn, int64(index+1), run); err != nil {
			return err
		}
	}
	return nil
}

func validateRunHistoryGoalTurn(turn RunHistoryGoalTurn, wantSeq int64, run Run) error {
	if turn.Seq != wantSeq || turn.Generation < 1 || turn.Generation > run.Generation {
		return fmt.Errorf("%w: goal turns require gap-free sequence and an imported generation", ErrValidation)
	}
	if strings.TrimSpace(string(turn.NodeID)) == "" || turn.ItemIndex < 0 || turn.Turn < 1 {
		return fmt.Errorf("%w: goal turn %d has an invalid node or turn identity", ErrValidation, turn.Seq)
	}
	if err := validateRunHistoryGoalTurnBinding(turn); err != nil {
		return err
	}
	if turn.UsageBaseTokens < 0 || turn.TokensUsed < 0 || !turn.StopReason.Valid() {
		return fmt.Errorf("%w: goal turn %d has invalid usage or stop reason", ErrValidation, turn.Seq)
	}
	if turn.VerdictOutcome != "" && !turn.VerdictOutcome.Valid() {
		return fmt.Errorf("%w: goal turn %d has an invalid verdict", ErrValidation, turn.Seq)
	}
	if err := validateRunHistoryGoalTurnJSON(turn); err != nil {
		return err
	}
	if strings.TrimSpace(turn.ActorKind) == "" || strings.TrimSpace(turn.ActorID) == "" ||
		turn.StartedAt.Before(run.CreatedAt) || turn.EndedAt.Before(turn.StartedAt) {
		return fmt.Errorf("%w: goal turn %d has invalid actor or timestamps", ErrValidation, turn.Seq)
	}
	return nil
}

func validateRunHistoryGoalTurnBinding(turn RunHistoryGoalTurn) error {
	if strings.TrimSpace(turn.SessionID) == "" || strings.TrimSpace(turn.BindingHandle) == "" ||
		turn.BindingEpoch < 1 || strings.TrimSpace(turn.PromptID) == "" || turn.PromptAttempt < 0 {
		return fmt.Errorf("%w: goal turn %d has an invalid session or prompt identity", ErrValidation, turn.Seq)
	}
	return nil
}

func validateRunHistoryGoalTurnJSON(turn RunHistoryGoalTurn) error {
	for name, raw := range map[string]json.RawMessage{
		"blocking": turn.BlockingIssues, "criteria": turn.Criteria, "warnings": turn.Warnings,
	} {
		if len(raw) == 0 || !json.Valid(raw) {
			return fmt.Errorf("%w: goal turn %d %s must be valid JSON", ErrValidation, turn.Seq, name)
		}
	}
	return nil
}

func normalizeRunHistoryEventPayloads(events []RunHistoryEvent) error {
	for index := range events {
		payload, err := json.Marshal(events[index].Payload)
		if err != nil {
			return fmt.Errorf("%w: encode run event %q payload: %v", ErrValidation, events[index].Kind, err)
		}
		events[index].Payload = json.RawMessage(payload)
	}
	return nil
}

// Snapshot reports an isolated copy of the validated ledger.
func (i *RunHistoryImport) Snapshot() RunHistorySnapshot {
	return cloneRunHistorySnapshot(&i.snapshot)
}

// Actor reports the authority responsible for the import.
func (i *RunHistoryImport) Actor() task.ActorContext { return i.snapshot.Actor }

func validateRunHistoryRun(run Run) error {
	if err := requireRunHistoryIdentity(run); err != nil {
		return err
	}
	if run.CreatedAt.IsZero() {
		return fmt.Errorf("%w: run history import requires created_at", ErrValidation)
	}
	if run.Generation < 0 {
		return fmt.Errorf("%w: run generation cursor must be non-negative", ErrValidation)
	}
	if run.IterationCap < 0 || run.BudgetTokens < 0 || run.BudgetWallSec < 0 || run.TokensUsed < 0 {
		return fmt.Errorf("%w: imported run counters must be non-negative", ErrValidation)
	}
	if len(run.DefinitionSnapshot) == 0 {
		return fmt.Errorf("%w: run history import requires a definition snapshot", ErrValidation)
	}
	resolved, err := LoadExecutedDefinitionSnapshot(run.DefinitionSnapshot, run.DefinitionDigest)
	if err != nil {
		return err
	}
	if resolved.DefinitionVersion != run.DefinitionVersion {
		return fmt.Errorf("%w: definition snapshot version does not match the run", ErrValidation)
	}
	if len(run.ActiveHumanCriteria) > 0 && !json.Valid(run.ActiveHumanCriteria) {
		return fmt.Errorf("%w: active_human_criteria must be valid JSON", ErrValidation)
	}
	return nil
}

func requireRunHistoryIdentity(run Run) error {
	if strings.TrimSpace(string(run.ID)) == "" {
		return fmt.Errorf("%w: run history import requires a run id", ErrValidation)
	}
	if strings.TrimSpace(string(run.WorkspaceID)) == "" {
		return fmt.Errorf("%w: run history import requires a workspace_id", ErrValidation)
	}
	if strings.TrimSpace(run.LoopName) == "" {
		return fmt.Errorf("%w: run history import requires a loop_name", ErrValidation)
	}
	if !run.Status.Valid() {
		return fmt.Errorf("%w: run status is invalid: %q", ErrValidation, run.Status)
	}
	return nil
}

func validateRunHistoryGenerations(generations []RunHistoryGeneration, run Run) (int, error) {
	if len(generations) == 0 {
		return 0, fmt.Errorf("%w: run history import requires at least one generation", ErrValidation)
	}
	highest := 0
	previousCreatedAt := time.Time{}
	for index := range generations {
		generation := generations[index]
		if err := generation.Intent.Validate(); err != nil {
			return 0, fmt.Errorf("loop: validate imported generation: %w", err)
		}
		number := int(generation.Intent.Generation)
		if number != index+1 {
			return 0, fmt.Errorf(
				"%w: generations must be imported in gap-free ascending order starting at 1",
				ErrValidation,
			)
		}
		if generation.CreatedAt.IsZero() {
			return 0, fmt.Errorf("%w: generation %d requires created_at", ErrValidation, number)
		}
		if generation.CreatedAt.Before(run.CreatedAt) {
			return 0, fmt.Errorf("%w: generations cannot predate run creation", ErrValidation)
		}
		if generation.CreatedAt.Before(previousCreatedAt) {
			return 0, fmt.Errorf("%w: generations must be imported in ascending time order", ErrValidation)
		}
		previousCreatedAt = generation.CreatedAt
		if err := validateRunHistoryVerdicts(generation); err != nil {
			return 0, err
		}
		for _, request := range generation.Requests {
			if request.WorkspaceID != run.WorkspaceID {
				return 0, fmt.Errorf(
					"%w: request workspace %q does not match run workspace %q",
					ErrValidation,
					request.WorkspaceID,
					run.WorkspaceID,
				)
			}
		}
		highest = number
	}
	return highest, nil
}

func validateRunHistoryVerdicts(generation RunHistoryGeneration) error {
	number := int(generation.Intent.Generation)
	for _, verdict := range generation.Verdicts {
		if verdict.Generation != 0 && verdict.Generation != number {
			return fmt.Errorf(
				"%w: verdict for gate %q belongs to generation %d, not %d",
				ErrValidation,
				verdict.Intent.GateID,
				verdict.Generation,
				number,
			)
		}
		if verdict.DecidedAt.IsZero() {
			return fmt.Errorf(
				"%w: verdict for gate %q requires decided_at",
				ErrValidation,
				verdict.Intent.GateID,
			)
		}
	}
	return nil
}

func validateRunHistoryBest(best *RunHistoryBest, cursor int) error {
	if best == nil {
		return nil
	}
	if best.Generation < 1 || best.Generation > int64(cursor) {
		return fmt.Errorf(
			"%w: best generation %d must fall within the run generation cursor %d",
			ErrValidation,
			best.Generation,
			cursor,
		)
	}
	return nil
}

func validateRunHistoryDecisions(decisions []GateDecisionRecord, run Run, highest int) error {
	for _, decision := range decisions {
		if decision.WorkspaceID != run.WorkspaceID {
			return fmt.Errorf(
				"%w: gate decision workspace %q does not match run workspace %q",
				ErrValidation,
				decision.WorkspaceID,
				run.WorkspaceID,
			)
		}
		if decision.RunID != "" && decision.RunID != run.ID {
			return fmt.Errorf(
				"%w: gate decision references run %q instead of %q",
				ErrValidation,
				decision.RunID,
				run.ID,
			)
		}
		if decision.Generation < 1 || decision.Generation > highest {
			return fmt.Errorf(
				"%w: gate decision generation %d is outside the imported range",
				ErrValidation,
				decision.Generation,
			)
		}
	}
	return nil
}

func validateRunHistoryEvents(events []RunHistoryEvent, createdAt time.Time) error {
	kinds := RunEventKindValues()
	previous := time.Time{}
	for _, event := range events {
		if !slices.Contains(kinds, string(event.Kind)) {
			return fmt.Errorf("%w: run event kind is invalid: %q", ErrValidation, event.Kind)
		}
		if event.At.IsZero() {
			return fmt.Errorf("%w: run event %q requires a timestamp", ErrValidation, event.Kind)
		}
		if event.At.Before(createdAt) {
			return fmt.Errorf("%w: run events cannot predate run creation", ErrValidation)
		}
		if event.At.Before(previous) {
			return fmt.Errorf("%w: run events must be imported in ascending time order", ErrValidation)
		}
		previous = event.At
	}
	return nil
}
