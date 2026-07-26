package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	storepkg "github.com/compozy/agh/internal/store"
)

var _ Service = (*service)(nil)

const (
	reasonMetaActiveRunID     = "active_run_id"
	reasonMetaAncestorRunID   = "ancestor_run_id"
	reasonMetaCause           = "cause"
	reasonMetaFrom            = "from"
	reasonMetaLoopName        = "loop_name"
	reasonMetaParentLoopRunID = "parent_loop_run_id"
	reasonMetaRunID           = "run_id"
	reasonMetaStatus          = "status"
	reasonMetaTo              = "to"
)

type service struct {
	store                 Store
	resolver              DefinitionResolver
	goalPolicy            GoalRunPolicyResolver
	hooks                 HookDispatcher
	defaults              LoopDefaults
	defaultsResolver      DefaultsResolver
	goalRunActivator      GoalRunActivator
	goalLeaseRevoker      GoalPromptLeaseRevoker
	participationResolver participation.Resolver
	logger                *slog.Logger
	now                   func() time.Time
	newRunID              func() RunID
}

// NewService creates the loop aggregate service.
func NewService(
	loopStore Store,
	resolver DefinitionResolver,
	goalPolicy GoalRunPolicyResolver,
	opts ...Option,
) (Service, error) {
	if loopStore == nil {
		return nil, fmt.Errorf("%w: loop store is required", ErrValidation)
	}
	if resolver == nil {
		return nil, fmt.Errorf("%w: loop definition resolver is required", ErrValidation)
	}
	if goalPolicy == nil {
		return nil, fmt.Errorf("%w: Goal run policy resolver is required", ErrValidation)
	}
	svc := &service{
		store:      loopStore,
		resolver:   resolver,
		goalPolicy: goalPolicy,
		defaults:   DefaultLoopDefaults(),
		logger:     slog.Default(),
		now:        func() time.Time { return time.Now().UTC() },
		newRunID: func() RunID {
			return RunID(storepkg.NewID("looprun"))
		},
	}
	for _, opt := range opts {
		opt(svc)
	}
	if svc.now == nil {
		return nil, fmt.Errorf("%w: loop clock is required", ErrValidation)
	}
	if svc.newRunID == nil {
		return nil, fmt.Errorf("%w: loop run ID factory is required", ErrValidation)
	}
	return svc, nil
}

func (s *service) DryRun(
	ctx context.Context,
	ws WorkspaceID,
	name string,
	inputs Inputs,
) (*PlanPreview, error) {
	resolved, loopName, err := s.resolveDefinition(ctx, ws, name)
	if err != nil {
		return nil, err
	}
	resolvedInputs, err := ResolveInputs(resolved.Definition, inputs)
	if err != nil {
		return nil, err
	}
	effective, err := s.effectiveConfig(ctx, ws, loopName, resolved, inputs.ConfigOverrides)
	if err != nil {
		return nil, err
	}
	previewRunID := s.newRunID()
	networkSpec, err := s.resolveRunParticipation(
		ctx,
		ws,
		previewRunID,
		inputs.NetworkParticipation,
		inputs.NetworkParticipationSource,
		resolved.Definition.NetworkParticipation,
	)
	if err != nil {
		return nil, err
	}
	if err := validateLoopParticipation(resolved.Definition.Graph, networkSpec); err != nil {
		return nil, err
	}
	return &PlanPreview{
		LoopName:                     loopName,
		ResolvedInputs:               resolvedInputs,
		Generation:                   1,
		Nodes:                        previewNodes(resolved.Definition.Graph),
		Contract:                     resolved.Definition.Contract,
		EffectiveConfig:              effective,
		ResolvedNetworkParticipation: networkSpec,
	}, nil
}

func cloneStartMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(metadata))
	maps.Copy(cloned, metadata)
	return cloned
}

func (s *service) Configure(
	ctx context.Context,
	ws WorkspaceID,
	name string,
	cfg LoopConfig,
) error {
	loopName, err := normalizeLoopName(name)
	if err != nil {
		return err
	}
	clamped := ClampLoopConfig(cfg)
	if err := validateConfigJSON(clamped); err != nil {
		return err
	}
	return s.store.UpsertLoopConfig(ctx, ws, loopName, clamped)
}

func (s *service) GetConfig(ctx context.Context, ws WorkspaceID, name string) (*LoopConfig, error) {
	loopName, err := normalizeLoopName(name)
	if err != nil {
		return nil, err
	}
	return s.store.GetLoopConfig(ctx, ws, loopName)
}

func (s *service) GetConfigSnapshot(
	ctx context.Context,
	ws WorkspaceID,
	name string,
) (ConfigSnapshot, error) {
	resolved, loopName, err := s.resolveDefinition(ctx, ws, name)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	stored, err := s.store.GetLoopConfig(ctx, ws, loopName)
	if err != nil && !errors.Is(err, ErrConfigNotFound) {
		return ConfigSnapshot{}, err
	}
	if errors.Is(err, ErrConfigNotFound) {
		stored = nil
	}
	defaults, err := s.resolveDefaults(ctx, ws)
	if err != nil {
		return ConfigSnapshot{}, fmt.Errorf("resolve loop defaults: %w", err)
	}
	effective, err := ResolveEffectiveConfig(resolved, defaults, stored, LoopConfig{})
	if err != nil {
		return ConfigSnapshot{}, err
	}
	return ConfigSnapshot{Stored: stored, Effective: effective}, nil
}

func (s *service) Get(ctx context.Context, ws WorkspaceID, runID RunID) (*Run, error) {
	run, err := s.store.GetLoopRun(ctx, ws, runID)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *service) Transition(
	ctx context.Context,
	runID RunID,
	to Status,
	cause TransitionCause,
) error {
	if to == "" {
		return fmt.Errorf("%w: transition target status is required", ErrValidation)
	}
	if cause == "" {
		return fmt.Errorf("%w: transition cause is required", ErrValidation)
	}
	run, err := s.store.GetLoopRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if !allowedTransition(run.Status, to, cause) {
		return reasonError(
			ReasonCodeInvalidStatusTransition,
			ErrInvalidTransition,
			map[string]string{
				reasonMetaCause: string(cause),
				reasonMetaFrom:  string(run.Status),
				reasonMetaTo:    string(to),
			},
		)
	}
	at := s.now().UTC()
	if err := s.store.CompareAndSwapLoopRunStatus(ctx, runID, run.Status, to, cause, at); err != nil {
		return err
	}
	if to.Terminal() {
		run.Status = to
		s.dispatchCoordinatorTerminal(ctx, run, cause, at)
	}
	return nil
}

func (s *service) resolveDefinition(
	ctx context.Context,
	ws WorkspaceID,
	name string,
) (*ResolvedDefinition, string, error) {
	loopName, err := normalizeLoopName(name)
	if err != nil {
		return nil, "", err
	}
	resolved, err := s.resolver.ResolveLoop(ctx, ws, loopName)
	if err != nil {
		return nil, "", err
	}
	if resolved == nil {
		return nil, "", fmt.Errorf("%w: loop %q", ErrDefinitionNotFound, loopName)
	}
	resolved.Definition.Normalize()
	return resolved, loopName, nil
}

func (s *service) effectiveConfig(
	ctx context.Context,
	ws WorkspaceID,
	loopName string,
	resolved *ResolvedDefinition,
	perRun LoopConfig,
) (EffectiveConfig, error) {
	stored, err := s.store.GetLoopConfig(ctx, ws, loopName)
	if err != nil && !errors.Is(err, ErrConfigNotFound) {
		return EffectiveConfig{}, err
	}
	if errors.Is(err, ErrConfigNotFound) {
		stored = nil
	}
	defaults, err := s.resolveDefaults(ctx, ws)
	if err != nil {
		return EffectiveConfig{}, fmt.Errorf("resolve loop defaults: %w", err)
	}
	return ResolveEffectiveConfig(resolved, defaults, stored, perRun)
}

func (s *service) ensureAncestry(ctx context.Context, parentID RunID, targetLoop string) error {
	if parentID == "" {
		return nil
	}
	currentID := parentID
	for depth := 1; currentID != ""; depth++ {
		if depth >= LoopMaxAncestryDepth {
			return reasonError(
				ReasonCodeAncestryDepthExceeded,
				ErrAncestryRejected,
				map[string]string{reasonMetaParentLoopRunID: string(parentID)},
			)
		}
		parent, err := s.store.GetLoopRunByID(ctx, currentID)
		if err != nil {
			return err
		}
		if parent.LoopName == targetLoop {
			return reasonError(
				ReasonCodeAncestryCycle,
				ErrAncestryRejected,
				map[string]string{
					reasonMetaAncestorRunID:   string(parent.ID),
					reasonMetaLoopName:        targetLoop,
					reasonMetaParentLoopRunID: string(parentID),
				},
			)
		}
		currentID = parent.ParentLoopRunID
	}
	return nil
}

func allowedTransition(from Status, to Status, cause TransitionCause) bool {
	switch from {
	case StatusQueued:
		return to == StatusRunning || (to == StatusFailed && cause == TransitionCauseOperatorStop)
	case StatusRunning:
		switch to {
		case StatusWatching, StatusNeedsApproval, StatusPaused,
			StatusDone, StatusNoOp, StatusBlocked, StatusFailed, StatusExhausted, StatusStalled:
			return true
		default:
			return false
		}
	case StatusWatching:
		switch to {
		case StatusRunning, StatusNeedsApproval,
			StatusDone, StatusNoOp, StatusBlocked, StatusFailed, StatusExhausted, StatusStalled:
			return true
		default:
			return false
		}
	case StatusNeedsApproval:
		return to == StatusRunning || to == StatusBlocked ||
			(to == StatusFailed && cause == TransitionCauseOperatorStop)
	case StatusPaused:
		return to == StatusRunning || (to == StatusFailed && cause == TransitionCauseOperatorStop)
	case StatusDone, StatusNoOp, StatusBlocked, StatusFailed, StatusExhausted, StatusStalled:
		return false
	default:
		return false
	}
}

// Terminal reports whether the status is terminal.
func (s Status) Terminal() bool {
	switch s {
	case StatusDone, StatusNoOp, StatusBlocked, StatusFailed, StatusExhausted, StatusStalled:
		return true
	case StatusQueued, StatusRunning, StatusWatching, StatusNeedsApproval, StatusPaused:
		return false
	default:
		return false
	}
}

// Live reports whether the status is non-terminal.
func (s Status) Live() bool {
	return s.Valid() && !s.Terminal()
}

// Valid reports whether the status belongs to the closed vocabulary.
func (s Status) Valid() bool {
	switch s {
	case StatusQueued, StatusRunning, StatusWatching, StatusNeedsApproval, StatusPaused,
		StatusDone, StatusNoOp, StatusBlocked, StatusFailed, StatusExhausted, StatusStalled:
		return true
	default:
		return false
	}
}

func validateConfigJSON(cfg LoopConfig) error {
	if len(cfg.EnabledChecks) == 0 {
		return nil
	}
	if !json.Valid(cfg.EnabledChecks) {
		return fmt.Errorf("%w: enabled_checks_json must be valid JSON", ErrValidation)
	}
	return nil
}

func previewNodes(graph dsl.Graph) []PlanNodePreview {
	dependencies := make(map[dsl.NodeID][]dsl.NodeID, len(graph.Nodes))
	for _, edge := range graph.Edges {
		dependencies[edge.To] = append(dependencies[edge.To], edge.From)
	}
	nodes := make([]PlanNodePreview, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes = append(nodes, PlanNodePreview{
			ID:        node.ID,
			Class:     node.Class,
			Kind:      node.Kind,
			DependsOn: append([]dsl.NodeID(nil), dependencies[node.ID]...),
		})
	}
	return nodes
}
