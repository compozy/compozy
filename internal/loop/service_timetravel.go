package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
)

func (s *service) RerunFromNode(ctx context.Context, input RerunInput) (RerunResult, error) {
	if err := validateRerunInput(input); err != nil {
		return RerunResult{}, err
	}
	store, ok := s.store.(TimeTravelStore)
	if !ok {
		return RerunResult{}, fmt.Errorf("%w: time-travel store is unavailable", ErrActionDependencyMissing)
	}
	run, err := s.store.GetLoopRun(ctx, input.WorkspaceID, input.RunID)
	if err != nil {
		return RerunResult{}, err
	}
	digest, err := timeTravelRequestDigest(struct {
		Kind      string `json:"kind"`
		RunID     RunID  `json:"run_id"`
		FromNode  NodeID `json:"from_node"`
		ItemIndex *int   `json:"item_index,omitempty"`
		Reason    string `json:"reason,omitempty"`
	}{"rerun", run.ID, input.FromNode, input.ItemIndex, strings.TrimSpace(input.Reason)})
	if err != nil {
		return RerunResult{}, err
	}
	if replay, found, replayErr := store.LookupRerunReplay(
		ctx, input.WorkspaceID, strings.TrimSpace(input.RequestID), digest,
	); replayErr != nil {
		return RerunResult{}, replayErr
	} else if found {
		outputs, _, rerunNodes, planErr := s.planRerunGeneration(
			ctx, run, int(replay.ParentGeneration), input.FromNode, input.ItemIndex, int(replay.Generation),
		)
		if planErr != nil {
			return RerunResult{}, planErr
		}
		replay.RerunNodes = rerunNodes
		replay.Carried = len(outputs) - len(rerunNodes)
		replay.Replayed = true
		return replay, nil
	}
	if err := s.rejectExecutingTimeTravelSelfOperation(ctx, run, input.Actor); err != nil {
		return RerunResult{}, err
	}
	if !run.Status.Terminal() {
		return RerunResult{}, reasonError(ReasonCodeRerunBusy, ErrRerunBusy, map[string]string{
			"run_id": string(run.ID), "status": string(run.Status),
		})
	}
	nextGeneration := run.Generation + 1
	outputs, next, rerunNodes, err := s.planRerunGeneration(
		ctx, run, run.Generation, input.FromNode, input.ItemIndex, nextGeneration,
	)
	if err != nil {
		return RerunResult{}, err
	}
	op, err := newTimeTravelOp("rerun", input.RequestID, digest, run, input.Actor, input.Reason, input.FromNode,
		input.ItemIndex, run.ID, int64Pointer(int64(nextGeneration)), s.now())
	if err != nil {
		return RerunResult{}, err
	}
	result, replayed, err := store.CreateRerun(ctx, RerunStoreRequest{
		WorkspaceID: input.WorkspaceID, Source: run, NextOutputs: next,
		Intent: GenerationIntent{Generation: int64(nextGeneration), ParentGeneration: int64(run.Generation),
			Origin: OriginOperatorRerun},
		Operation: op, RequestDigest: digest, IdempotencyKey: strings.TrimSpace(input.RequestID), At: s.now(),
	})
	if err != nil {
		return RerunResult{}, err
	}
	result.RerunNodes = rerunNodes
	result.Carried = len(outputs) - len(rerunNodes)
	result.Replayed = replayed
	return result, nil
}

func (s *service) planRerunGeneration(
	ctx context.Context,
	run Run,
	sourceGeneration int,
	fromNode NodeID,
	itemIndex *int,
	nextGeneration int,
) ([]GenerationOutput, []GenerationOutput, []string, error) {
	outputs, err := requireTimeTravelOutputs(
		ctx, s.store, run, sourceGeneration, ReasonCodeRerunBusy, ErrRerunBusy,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, output := range outputs {
		if !generationOutputSettled(output.Status) && !GenerationOutputStatusParked(output.Status) {
			return nil, nil, nil, reasonError(ReasonCodeRerunBusy, ErrRerunBusy, map[string]string{
				"node_id": output.NodeID, "status": output.Status,
			})
		}
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, run.WorkspaceID, run.DefinitionDigest)
	if err != nil {
		return nil, nil, nil, err
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return nil, nil, nil, err
	}
	next, rerunNodes, err := planOperatorRerun(
		resolved.Definition.Graph, outputs, fromNode, itemIndex, nextGeneration,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return outputs, next, rerunNodes, nil
}

func (s *service) ForkRun(ctx context.Context, input ForkInput) (StartResult, error) {
	if err := validateForkInput(input); err != nil {
		return StartResult{}, err
	}
	store, ok := s.store.(TimeTravelStore)
	if !ok {
		return StartResult{}, fmt.Errorf("%w: time-travel store is unavailable", ErrActionDependencyMissing)
	}
	source, err := s.store.GetLoopRun(ctx, input.WorkspaceID, input.RunID)
	if err != nil {
		return StartResult{}, err
	}
	if err := s.rejectExecutingTimeTravelSelfOperation(ctx, source, input.Actor); err != nil {
		return StartResult{}, err
	}
	outputs, err := requireTimeTravelOutputs(
		ctx, s.store, source, int(input.Generation), ReasonCodeForkGenerationUnknown, ErrForkGenerationUnknown,
	)
	if err != nil {
		return StartResult{}, err
	}
	for _, output := range outputs {
		if !generationOutputSettled(output.Status) {
			return StartResult{}, reasonError(
				ReasonCodeForkGenerationUnknown,
				ErrForkGenerationUnknown,
				map[string]string{
					"generation": fmt.Sprintf("%d", input.Generation),
					"node_id":    output.NodeID,
					"status":     output.Status,
				},
			)
		}
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, source.WorkspaceID, source.DefinitionDigest)
	if err != nil {
		return StartResult{}, err
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, source.DefinitionDigest)
	if err != nil {
		return StartResult{}, err
	}
	values := make(map[string]any, len(source.Inputs)+len(input.Inputs))
	maps.Copy(values, source.Inputs)
	maps.Copy(values, input.Inputs)
	values, err = ResolveInputs(resolved.Definition, Inputs{Values: values})
	if err != nil {
		return StartResult{}, err
	}
	child, err := s.forkChildRun(source, snapshot.Definition, values, input)
	if err != nil {
		return StartResult{}, err
	}
	seed := forkSeedOutputs(outputs)
	digest, err := timeTravelRequestDigest(struct {
		Kind       string         `json:"kind"`
		RunID      RunID          `json:"run_id"`
		Generation int64          `json:"generation"`
		Inputs     map[string]any `json:"inputs"`
		Reason     string         `json:"reason,omitempty"`
	}{"fork", source.ID, input.Generation, input.Inputs, strings.TrimSpace(input.Reason)})
	if err != nil {
		return StartResult{}, err
	}
	op, err := newTimeTravelOp("fork", input.RequestID, digest, source, input.Actor, input.Reason, "", nil,
		child.ID, nil, s.now())
	if err != nil {
		return StartResult{}, err
	}
	op.SourceGeneration = int64Pointer(input.Generation)
	created, replayed, err := store.CreateFork(ctx, ForkStoreRequest{
		Source: source, Child: child, SeedOutputs: seed, Concurrency: resolved.Defaults.Concurrency,
		Operation: op, RequestDigest: digest, IdempotencyKey: strings.TrimSpace(input.RequestID), At: s.now(),
	})
	if err != nil {
		return StartResult{}, err
	}
	if !replayed {
		s.observeCommittedRunParticipation(ctx, created)
		s.dispatchLoopStarted(ctx, created, input.Actor)
	}
	return StartResult{Run: created, Replayed: replayed}, nil
}

func (s *service) forkChildRun(
	source Run,
	snapshot json.RawMessage,
	values map[string]any,
	input ForkInput,
) (Run, error) {
	runID, err := s.newRunID()
	if err != nil {
		return Run{}, fmt.Errorf("loop: generate fork run id: %w", err)
	}
	now := s.now().UTC()
	child := source
	child.ID = runID
	child.Status = StatusRunning
	child.CompletionState = CompletionComplete
	child.Generation = 1
	child.CreatedAt, child.StartedAt, child.LastProgressAt = now, now, now
	child.StartedBy, child.StartedOrigin = input.Actor.Actor, input.Actor.Origin
	child.DefinitionSnapshot = append(json.RawMessage(nil), snapshot...)
	child.Inputs = values
	child.TokensUsed = 0
	child.PauseRequested, child.CancelRequested = false, false
	child.CancelKind = ParseRunCancelKind("")
	child.ControlActor = task.ActorIdentity{}
	child.ControlRequestedAt = time.Time{}
	child.ActiveGateID = ""
	child.ActiveHumanCriteria = json.RawMessage(`[]`)
	child.BudgetApprovalSeq = 0
	child.ForkedFrom = &ForkRef{RunID: source.ID, Generation: input.Generation}
	child.Forks = nil
	child.ParentLoopRunID = ""
	child.StartMetadata = map[string]any{}
	if source.BestScore != nil {
		child.BestGeneration = int64Pointer(1)
		child.BestScore = float64Pointer(*source.BestScore)
	} else {
		child.BestGeneration, child.BestScore = nil, nil
	}
	child.RunStartState = nil
	child.Origin = &RunOrigin{Kind: RunOriginCatalog}
	child.SetNetworkSpec(source.NetworkSpecSnapshot())
	return child, nil
}

func requireTimeTravelOutputs(
	ctx context.Context,
	store Store,
	run Run,
	generation int,
	code ReasonCode,
	sentinel error,
) ([]GenerationOutput, error) {
	reader, ok := store.(GenerationOutputLister)
	if !ok {
		return nil, fmt.Errorf("%w: generation output reader is unavailable", ErrActionDependencyMissing)
	}
	if generation < 1 || generation > run.Generation {
		return nil, reasonError(code, sentinel, map[string]string{"generation": fmt.Sprintf("%d", generation)})
	}
	outputs, err := reader.ListGenerationOutputs(ctx, run.WorkspaceID, run.ID, generation)
	if err != nil {
		return nil, err
	}
	if len(outputs) == 0 {
		return nil, reasonError(code, sentinel, map[string]string{"generation": fmt.Sprintf("%d", generation)})
	}
	return outputs, nil
}

func forkSeedOutputs(source []GenerationOutput) []GenerationOutput {
	seed := make([]GenerationOutput, 0, len(source))
	for _, output := range source {
		seed = append(seed, GenerationOutput{
			Generation: 1, NodeID: output.NodeID, ItemIndex: output.ItemIndex,
			Status: generationOutputSucceeded, OutputRef: output.OutputRef, Attempt: 1,
		})
	}
	return seed
}

func (s *service) rejectExecutingTimeTravelSelfOperation(ctx context.Context, run Run, actor task.ActorContext) error {
	if actor.Actor.Kind.Normalize() != task.ActorKindAgentSession || !run.Status.Live() {
		return nil
	}
	if s.responderPolicy == nil {
		return reasonError(ReasonCodeTimeTravelSelfDenied, ErrTimeTravelSelfDenied, nil)
	}
	denied, err := s.responderPolicy.DeniesSelfOperation(ctx, string(run.WorkspaceID), string(run.ID), actor)
	if err != nil {
		return fmt.Errorf("loop: evaluate time-travel responder policy: %w", err)
	}
	if denied {
		return reasonError(ReasonCodeTimeTravelSelfDenied, ErrTimeTravelSelfDenied, nil)
	}
	return nil
}

func validateRerunInput(input RerunInput) error {
	if err := input.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	if strings.TrimSpace(string(input.WorkspaceID)) == "" || strings.TrimSpace(string(input.RunID)) == "" ||
		strings.TrimSpace(string(input.FromNode)) == "" {
		return fmt.Errorf("%w: rerun workspace_id, run_id, and from_node are required", ErrValidation)
	}
	if input.ItemIndex != nil && *input.ItemIndex < 0 {
		return fmt.Errorf("%w: item_index must be non-negative", ErrValidation)
	}
	return nil
}

func validateForkInput(input ForkInput) error {
	if err := input.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	if strings.TrimSpace(string(input.WorkspaceID)) == "" || strings.TrimSpace(string(input.RunID)) == "" || input.Generation < 1 {
		return fmt.Errorf("%w: fork workspace_id, run_id, and generation are required", ErrValidation)
	}
	return nil
}

func timeTravelRequestDigest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("loop: encode time-travel request: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func newTimeTravelOp(
	kind, requestID, digest string,
	source Run,
	actor task.ActorContext,
	reason string,
	fromNode NodeID,
	itemIndex *int,
	resultRunID RunID,
	resultGeneration *int64,
	at time.Time,
) (TimeTravelOp, error) {
	id, err := storepkg.NewID("loopop")
	if err != nil {
		return TimeTravelOp{}, fmt.Errorf("loop: generate time-travel operation id: %w", err)
	}
	return TimeTravelOp{
		ID: id, Kind: kind, IdempotencyKey: strings.TrimSpace(requestID), RequestDigest: digest,
		SourceRunID: source.ID, SourceGeneration: int64Pointer(int64(source.Generation)), FromNode: fromNode,
		ItemIndex: itemIndex, Actor: actor, Reason: strings.TrimSpace(reason), ResultRunID: resultRunID,
		ResultGeneration: resultGeneration, CreatedAt: at.UTC(),
	}, nil
}

func int64Pointer(value int64) *int64       { return &value }
func float64Pointer(value float64) *float64 { return &value }
