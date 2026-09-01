package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/task"
)

func (s *service) Start(
	ctx context.Context,
	ws WorkspaceID,
	name string,
	inputs Inputs,
	actor task.ActorContext,
) (*Run, error) {
	if err := actor.Validate(); err != nil {
		return nil, fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	resolved, loopName, err := s.resolveDefinition(ctx, ws, inputs.ProfileID, name)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAncestry(ctx, ws, inputs.ParentLoopRunID, loopName); err != nil {
		return nil, err
	}
	prepared, err := s.prepareResolvedStart(
		ctx,
		ws,
		loopName,
		resolved,
		inputs,
		RunOrigin{Kind: RunOriginCatalog},
		actor,
	)
	if err != nil {
		return nil, err
	}
	created, err := s.store.CreateLoopRunForStart(ctx, prepared, resolved.Defaults.Concurrency)
	if err != nil {
		return nil, err
	}
	if created.RunStartState != nil && created.Admission != nil && created.Admission.Suppressed {
		return &created, nil
	}
	s.observeCommittedRunParticipation(ctx, created)
	s.dispatchLoopStarted(ctx, created, actor)
	return &created, nil
}

func (s *service) StartInline(
	ctx context.Context,
	ws WorkspaceID,
	definition dsl.Definition,
	inputs Inputs,
	origin RunOrigin,
	actor task.ActorContext,
) (*Run, error) {
	if err := actor.Validate(); err != nil {
		return nil, fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	inlineStore, ok := s.store.(InlineRunStore)
	if !ok {
		return nil, fmt.Errorf("%w: inline Run store is unavailable", ErrActionDependencyMissing)
	}
	resolved, err := compileInlineGoalDefinition(definition)
	if err != nil {
		return nil, err
	}
	prepared, err := s.prepareResolvedStart(ctx, ws, InlineGoalLoopName, resolved, inputs, origin, actor)
	if err != nil {
		return nil, err
	}
	created, err := inlineStore.CreateInlineLoopRunForStart(ctx, prepared)
	if err != nil {
		return nil, err
	}
	s.observeCommittedRunParticipation(ctx, created)
	s.dispatchLoopStarted(ctx, created, actor)
	return &created, nil
}

func (s *service) ReplaceInline(
	ctx context.Context,
	expectedRunID RunID,
	ws WorkspaceID,
	definition dsl.Definition,
	inputs Inputs,
	origin RunOrigin,
	actor task.ActorContext,
) (InlineReplaceResult, error) {
	if strings.TrimSpace(string(expectedRunID)) == "" {
		return InlineReplaceResult{}, fmt.Errorf("%w: expected Run ID is required", ErrValidation)
	}
	if err := actor.Validate(); err != nil {
		return InlineReplaceResult{}, fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	inlineStore, ok := s.store.(InlineRunStore)
	if !ok {
		return InlineReplaceResult{}, fmt.Errorf("%w: inline Run store is unavailable", ErrActionDependencyMissing)
	}
	resolved, err := compileInlineGoalDefinition(definition)
	if err != nil {
		return InlineReplaceResult{}, err
	}
	prepared, err := s.prepareResolvedStart(ctx, ws, InlineGoalLoopName, resolved, inputs, origin, actor)
	if err != nil {
		return InlineReplaceResult{}, err
	}
	replacedAt := s.now().UTC()
	committed, err := inlineStore.ReplaceInlineLoopRun(ctx, InlineReplaceStoreRequest{
		ExpectedRunID: expectedRunID,
		Run:           &prepared,
		Actor:         actor,
		ReplacedAt:    replacedAt,
	})
	if err != nil {
		return InlineReplaceResult{}, err
	}
	s.observeCommittedRunParticipation(ctx, committed.Run)
	s.revokeGoalPromptLeases(ctx, committed.RevokedPromptLeases, TransitionCauseGoalReplace)
	s.dispatchCoordinatorTerminal(ctx, committed.ReplacedRun, TransitionCauseGoalReplace, replacedAt)
	s.dispatchLoopStarted(ctx, committed.Run, actor)
	created := committed.Run
	return InlineReplaceResult{ReplacedRunID: committed.ReplacedRunID, Run: &created}, nil
}

func compileInlineGoalDefinition(definition dsl.Definition) (*ResolvedDefinition, error) {
	definition.Meta.Name = strings.TrimSpace(definition.Meta.Name)
	if definition.Meta.Name != InlineGoalLoopName {
		return nil, fmt.Errorf("%w: inline Goal definition name must be %q", ErrValidation, InlineGoalLoopName)
	}
	if definition.Concurrency != dsl.ConcurrencyAllow {
		return nil, fmt.Errorf("%w: inline Goal definition concurrency must be allow", ErrValidation)
	}
	resolved, err := NewCompiler().Compile(definition)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func (s *service) prepareResolvedStart(
	ctx context.Context,
	ws WorkspaceID,
	loopName string,
	resolved *ResolvedDefinition,
	inputs Inputs,
	origin RunOrigin,
	actor task.ActorContext,
) (Run, error) {
	inputs.ProfileID = strings.TrimSpace(inputs.ProfileID)
	if inputs.ProfileID == "" {
		return Run{}, fmt.Errorf("%w: profile id is required", ErrValidation)
	}
	origin = origin.Normalize()
	if err := origin.Validate(); err != nil {
		return Run{}, err
	}
	resolvedInputs, err := s.resolveEffectiveInputs(ctx, ws, loopName, resolved.Definition, inputs.Values)
	if err != nil {
		return Run{}, err
	}
	if err := s.validateResolvedInputEntities(
		ctx, ws, inputs.ProfileID, loopName, resolved.Definition, resolvedInputs,
	); err != nil {
		return Run{}, err
	}
	effective, err := s.effectiveConfig(
		ctx,
		ws,
		loopName,
		resolved,
		inputs.InheritedEnvironment,
		inputs.ConfigOverrides,
	)
	if err != nil {
		return Run{}, err
	}
	if origin.Kind == RunOriginSession && strings.TrimSpace(effective.RuntimeDefaults.Judge.Model) == "" {
		return Run{}, reasonError(
			ReasonCodeGoalJudgeUnavailable,
			ErrActionDependencyMissing,
			map[string]string{reasonMetaLoopName: loopName},
		)
	}
	policy, err := s.resolveGoalRunPolicy(ctx, ws)
	if err != nil {
		return Run{}, err
	}
	runID, err := s.newRunID()
	if err != nil {
		return Run{}, fmt.Errorf("loop: generate run id: %w", err)
	}
	networkSpec, err := s.resolveRunParticipation(
		ctx,
		ws,
		runID,
		inputs.NetworkParticipation,
		inputs.NetworkParticipationSource,
		resolved.Definition.NetworkParticipation,
		inputs.NetworkParticipationSnapshot,
	)
	if err != nil {
		return Run{}, err
	}
	if err := validateLoopParticipation(resolved.Definition.Graph, networkSpec); err != nil {
		return Run{}, err
	}
	return s.startResolved(
		runID,
		ws,
		loopName,
		resolved,
		effective,
		resolvedInputs.Values,
		inputs,
		origin,
		policy,
		networkSpec,
		actor,
	)
}

func (s *service) resolveGoalRunPolicy(ctx context.Context, ws WorkspaceID) (GoalRunPolicy, error) {
	policy, err := s.goalPolicy.ResolveGoalRunPolicy(ctx, ws)
	if err != nil {
		return GoalRunPolicy{}, fmt.Errorf("resolve Goal run policy: %w", err)
	}
	if policy == nil {
		return GoalRunPolicy{}, fmt.Errorf("%w: Goal run policy resolver returned nil", ErrValidation)
	}
	if err := policy.Validate(); err != nil {
		return GoalRunPolicy{}, err
	}
	return *policy, nil
}

func (s *service) startResolved(
	runID RunID,
	ws WorkspaceID,
	loopName string,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	resolvedInputs map[string]any,
	inputs Inputs,
	origin RunOrigin,
	policy GoalRunPolicy,
	networkSpec participation.Spec,
	actor task.ActorContext,
) (Run, error) {
	snapshot, digest, err := BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		return Run{}, err
	}
	now := s.now().UTC()
	run := Run{
		ID: runID, ProfileID: inputs.ProfileID, WorkspaceID: ws,
		LoopName: loopName, Status: StatusRunning, Generation: 0,
		ReattemptStrategy: effective.ReattemptStrategy, CreatedAt: now, StartedAt: now, LastProgressAt: now,
		StartedBy: actor.Actor, StartedOrigin: actor.Origin,
		DefinitionVersion: resolved.DefinitionVersion, DefinitionDigest: digest, DefinitionSnapshot: snapshot,
		StartMetadata: cloneStartMetadata(inputs.StartMetadata),
		IterationCap:  effective.IterationCap, BudgetTokens: effective.BudgetTokens,
		BudgetWallSec: effective.BudgetWallSec, BudgetOnExceeded: effective.BudgetOnExceeded,
		ParentLoopRunID: inputs.ParentLoopRunID, GoalContextNudgeRatio: policy.ContextNudgeRatio,
		Origin: &origin, Inputs: resolvedInputs,
	}
	run.SetActiveHumanCriteria(json.RawMessage(`[]`))
	run.SetNetworkSpec(networkSpec)
	if inputs.Admission != nil {
		admission := *inputs.Admission
		if admission.Horizon <= 0 && effective.Lifecycle.AdmissionHorizon != nil {
			admission.Horizon = *effective.Lifecycle.AdmissionHorizon
		}
		normalized, err := NormalizeAdmissionIdentity(admission)
		if err != nil {
			return Run{}, err
		}
		run.SetAdmission(normalized)
	}
	return run, nil
}
