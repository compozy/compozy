package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
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
	historyRun, err := rerunHistoryRun(ctx, store, run)
	if err != nil {
		return RerunResult{}, err
	}
	digest, err := rerunRequestDigest(run, input)
	if err != nil {
		return RerunResult{}, err
	}
	if replay, found, replayErr := store.LookupRerunReplay(
		ctx, input.WorkspaceID, strings.TrimSpace(input.RequestID), digest,
	); replayErr != nil {
		return RerunResult{}, replayErr
	} else if found {
		outputs, _, rerunNodes, planErr := s.planRerunGeneration(
			ctx, historyRun, int(replay.ParentGeneration), input.FromNode, input.ItemIndex, int(replay.Generation),
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
			"run_id": string(run.ID), namespaceStatusKey: string(run.Status),
		})
	}
	nextGeneration := historyRun.Generation + 1
	outputs, next, rerunNodes, err := s.planRerunGeneration(
		ctx, historyRun, historyRun.Generation, input.FromNode, input.ItemIndex, nextGeneration,
	)
	if err != nil {
		return RerunResult{}, err
	}
	op, err := newTimeTravelOp(
		timeTravelKindRerun,
		input.RequestID,
		digest,
		historyRun,
		input.Actor,
		input.Reason,
		input.FromNode,
		input.ItemIndex,
		run.ID,
		new(int64(nextGeneration)),
		s.now(),
	)
	if err != nil {
		return RerunResult{}, err
	}
	result, replayed, err := store.CreateRerun(ctx, RerunStoreRequest{
		WorkspaceID: input.WorkspaceID, Source: &run, NextOutputs: next,
		Intent: GenerationIntent{Generation: int64(nextGeneration), ParentGeneration: int64(historyRun.Generation),
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

// Reruns use immutable generation history: a successor can be persisted before
// its first coordinator boundary updates the run's generation projection.
func rerunHistoryRun(ctx context.Context, reader GenerationLineageReader, run Run) (Run, error) {
	generations, err := reader.ListGenerations(ctx, string(run.WorkspaceID), string(run.ID))
	if err != nil {
		return Run{}, err
	}
	for _, generation := range generations {
		run.Generation = max(run.Generation, int(generation.Generation))
	}
	return run, nil
}

func rerunRequestDigest(run Run, input RerunInput) (string, error) {
	return timeTravelRequestDigest(struct {
		Kind      string `json:"kind"`
		RunID     RunID  `json:"run_id"`
		FromNode  NodeID `json:"from_node"`
		ItemIndex *int   `json:"item_index,omitempty"`
		Reason    string `json:"reason,omitempty"`
	}{timeTravelKindRerun, run.ID, input.FromNode, input.ItemIndex, strings.TrimSpace(input.Reason)})
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
	if err := validateTerminalRerunOutputs(outputs, rerunNodes); err != nil {
		return nil, nil, nil, err
	}
	return outputs, next, rerunNodes, nil
}

func validateTerminalRerunOutputs(outputs []GenerationOutput, rerunNodes []string) error {
	rerun := make(map[string]struct{}, len(rerunNodes))
	for _, node := range rerunNodes {
		rerun[node] = struct{}{}
	}
	for _, output := range outputs {
		if generationOutputSettled(output.Status) || GenerationOutputStatusParked(output.Status) {
			continue
		}
		_, selectedPending := rerun[generationOutputLabel(generationOutputKey{
			nodeID: output.NodeID, itemIndex: output.ItemIndex,
		})]
		if output.Status == generationOutputPending && selectedPending {
			continue
		}
		return reasonError(ReasonCodeRerunBusy, ErrRerunBusy, map[string]string{
			metadataNodeIDKey: output.NodeID, namespaceStatusKey: output.Status,
		})
	}
	return nil
}

func (s *service) ForkRun(ctx context.Context, input ForkInput) (StartResult, error) {
	if err := validateForkInput(input); err != nil {
		return StartResult{}, err
	}
	store, ok := s.store.(TimeTravelStore)
	if !ok {
		return StartResult{}, fmt.Errorf("%w: time-travel store is unavailable", ErrActionDependencyMissing)
	}
	prepared, err := s.prepareForkRun(ctx, input)
	if err != nil {
		return StartResult{}, err
	}
	child, err := s.forkChildRun(
		ctx, prepared.source, prepared.snapshot, prepared.values, input,
	)
	if err != nil {
		return StartResult{}, err
	}
	seed := forkSeedOutputs(prepared.outputs)
	digest, err := forkRequestDigest(prepared.source, input)
	if err != nil {
		return StartResult{}, err
	}
	op, err := newTimeTravelOp("fork", input.RequestID, digest, prepared.source, input.Actor, input.Reason, "", nil,
		child.ID, nil, s.now())
	if err != nil {
		return StartResult{}, err
	}
	op.SourceGeneration = new(input.Generation)
	created, replayed, err := store.CreateFork(ctx, ForkStoreRequest{
		Source: &prepared.source, Child: &child, SeedOutputs: seed,
		Concurrency: prepared.resolved.Defaults.Concurrency,
		Operation:   op, RequestDigest: digest, IdempotencyKey: strings.TrimSpace(input.RequestID), At: s.now(),
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

func forkRequestDigest(source Run, input ForkInput) (string, error) {
	return timeTravelRequestDigest(struct {
		Kind       string         `json:"kind"`
		RunID      RunID          `json:"run_id"`
		Generation int64          `json:"generation"`
		Inputs     map[string]any `json:"inputs"`
		Reason     string         `json:"reason,omitempty"`
	}{"fork", source.ID, input.Generation, input.Inputs, strings.TrimSpace(input.Reason)})
}

func (s *service) forkChildRun(
	ctx context.Context,
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
	child.RunStartState = nil
	child.ID = runID
	child.Status = StatusRunning
	child.SetCompletionState(CompletionComplete)
	child.Generation = 1
	child.CreatedAt, child.StartedAt, child.LastProgressAt = now, now, now
	child.StartedBy, child.StartedOrigin = input.Actor.Actor, input.Actor.Origin
	child.DefinitionSnapshot = append(json.RawMessage(nil), snapshot...)
	child.Inputs = values
	child.TokensUsed = 0
	child.PauseRequested = false
	child.ControlActor = task.ActorIdentity{}
	child.ControlRequestedAt = time.Time{}
	child.ActiveGateID = ""
	child.SetActiveHumanCriteria(json.RawMessage(`[]`))
	child.BudgetApprovalSeq = 0
	child.SetForkedFrom(&ForkRef{RunID: source.ID, Generation: input.Generation})
	child.SetForks(nil)
	child.ParentLoopRunID = ""
	child.StartMetadata = map[string]any{}
	if source.BestScore != nil {
		child.BestGeneration = new(int64(1))
		child.BestScore = new(*source.BestScore)
	} else {
		child.BestGeneration, child.BestScore = nil, nil
	}
	child.Origin = &RunOrigin{Kind: RunOriginCatalog}
	networkSpec, err := s.resolveForkParticipation(ctx, source.WorkspaceID, runID, source.NetworkSpecSnapshot())
	if err != nil {
		return Run{}, fmt.Errorf("loop: resolve fork network participation: %w", err)
	}
	child.SetNetworkSpec(networkSpec)
	return child, nil
}

func (s *service) resolveForkParticipation(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
	source participation.Spec,
) (participation.Spec, error) {
	if source.Mode != participation.ModeLive {
		return source, nil
	}
	if s == nil || s.participationResolver == nil {
		return participation.Spec{}, fmt.Errorf(
			"%w: loop participation resolver is required for live fork",
			ErrActionDependencyMissing,
		)
	}
	mode, strategy := source.Mode, source.ChannelStrategy
	request := &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		Bounds:          participationBoundsRequest(source.Bounds),
	}
	if source.ChannelStrategy == participation.StrategyNamed {
		channelID := source.ChannelID
		request.ChannelID = &channelID
	}
	resolveInput := participation.ResolveInput{
		WorkspaceID: string(workspaceID),
		Owner: participation.OwnerRef{
			WorkspaceID: string(workspaceID),
			Kind:        participation.OwnerKindLoopRun,
			ID:          string(runID),
		},
		Request:   request,
		LoopRunID: string(runID),
	}
	if resolver, ok := s.participationResolver.(participation.IntentResolver); ok {
		return resolver.ResolveIntent(ctx, resolveInput, participation.Intent{Request: request, Source: source.Source})
	}
	resolveInput.RequestSource = participation.SourceExplicitRequest
	resolved, err := s.participationResolver.Resolve(ctx, resolveInput)
	if err != nil {
		return participation.Spec{}, err
	}
	resolved.Source = source.Source
	return resolved, nil
}

func participationBoundsRequest(bounds participation.Bounds) *participation.BoundsRequest {
	return &participation.BoundsRequest{
		MaxWakes:         new(bounds.MaxWakes),
		MaxWakeWallTime:  new(bounds.MaxWakeWallTime),
		MaxTotalWallTime: new(bounds.MaxTotalWallTime),
		MaxInputTokens:   new(bounds.MaxInputTokens),
		MaxOutputTokens:  new(bounds.MaxOutputTokens),
		MaxWakeDepth:     new(bounds.MaxWakeDepth),
		CoalesceWindow:   new(bounds.CoalesceWindow),
	}
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
	if strings.TrimSpace(string(input.WorkspaceID)) == "" || strings.TrimSpace(string(input.RunID)) == "" ||
		input.Generation < 1 {
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
		SourceRunID: source.ID, SourceGeneration: new(int64(source.Generation)), FromNode: fromNode,
		ItemIndex: itemIndex, Actor: actor, Reason: strings.TrimSpace(reason), ResultRunID: resultRunID,
		ResultGeneration: resultGeneration, CreatedAt: at.UTC(),
	}, nil
}
