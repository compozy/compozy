package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type automationLoopStarter struct {
	service  automationLoopService
	resolver looppkg.DefinitionResolver
}

type automationLoopService interface {
	Start(
		ctx context.Context,
		ws looppkg.WorkspaceID,
		name string,
		inputs looppkg.Inputs,
		actor taskpkg.ActorContext,
	) (*looppkg.Run, error)
}

var _ automationpkg.LoopStarter = (*automationLoopStarter)(nil)
var _ automationpkg.LoopCatchUpPolicyResolver = (*automationLoopStarter)(nil)

func newAutomationLoopStarter(
	storeCandidate any,
	catalog *resourceCatalog[looppkg.ResourceSpec],
	toolRegistry toolspkg.Registry,
	homePaths aghconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
	participationResolver participation.Resolver,
) (automationpkg.LoopStarter, error) {
	if catalog == nil {
		return nil, nil
	}
	loopStore, ok := storeCandidate.(looppkg.Store)
	if !ok {
		return nil, errors.New("daemon: automation loop starter requires loop store")
	}
	resolver := &daemonLoopDefinitionResolver{
		catalog:         catalog,
		compilerFactory: newLoopCompilerFactory(toolRegistry),
	}
	options := []looppkg.Option{
		looppkg.WithDefaultsResolver(newLoopDefaultsResolver(homePaths, workspaceResolver)),
	}
	if participationResolver != nil {
		options = append(options, looppkg.WithParticipationResolver(participationResolver))
	}
	service, err := looppkg.NewService(
		loopStore,
		resolver,
		newGoalRunPolicyResolver(homePaths, workspaceResolver),
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create automation loop starter: %w", err)
	}
	return &automationLoopStarter{
		service:  service,
		resolver: resolver,
	}, nil
}

func (s *automationLoopStarter) ValidateLoopTarget(
	ctx context.Context,
	req automationpkg.LoopTargetValidationRequest,
) error {
	kind, err := automationStartKindToLoopStartKind(req.Kind)
	if err != nil {
		return err
	}
	return looppkg.ValidateStartTarget(ctx, s.resolver, looppkg.StartTargetValidation{
		WorkspaceID:  looppkg.WorkspaceID(strings.TrimSpace(req.WorkspaceID)),
		LoopName:     strings.TrimSpace(req.LoopName),
		Kind:         kind,
		Inputs:       req.Inputs,
		InputMapping: req.InputMapping,
	})
}

func (s *automationLoopStarter) DefaultLoopCatchUpPolicy(
	ctx context.Context,
	req automationpkg.LoopCatchUpPolicyRequest,
) (automationpkg.SchedulerCatchUpPolicy, error) {
	if _, err := automationStartKindToLoopStartKind(req.Kind); err != nil {
		return "", err
	}
	resolved, err := s.resolver.ResolveLoop(
		ctx,
		looppkg.WorkspaceID(strings.TrimSpace(req.WorkspaceID)),
		strings.TrimSpace(req.LoopName),
	)
	if err != nil {
		return "", err
	}
	for _, node := range resolved.Definition.Graph.Nodes {
		if node.Class == loopdsl.NodeClassSource && node.Kind == string(loopdsl.SourceWatchSource) {
			return automationpkg.SchedulerCatchUpPolicyCoalesce, nil
		}
	}
	return automationpkg.SchedulerCatchUpPolicySkipMissed, nil
}

func (s *automationLoopStarter) StartLoop(
	ctx context.Context,
	req automationpkg.LoopStartRequest,
) (automationpkg.LoopStartResult, error) {
	kind, err := automationStartKindToLoopStartKind(req.Kind)
	if err != nil {
		return automationpkg.LoopStartResult{}, err
	}
	workspaceID := looppkg.WorkspaceID(strings.TrimSpace(req.WorkspaceID))
	loopName := strings.TrimSpace(req.LoopName)
	values, err := looppkg.ResolveStartTargetInputs(ctx, s.resolver, looppkg.StartTargetResolution{
		StartTargetValidation: looppkg.StartTargetValidation{
			WorkspaceID:  workspaceID,
			LoopName:     loopName,
			Kind:         kind,
			Inputs:       req.Inputs,
			InputMapping: req.InputMapping,
		},
		TriggerPayload: req.TriggerPayload,
	})
	if err != nil {
		return automationpkg.LoopStartResult{}, err
	}
	run, err := s.service.Start(ctx, workspaceID, loopName, looppkg.Inputs{
		Values:                     values,
		StartMetadata:              automationLoopStartMetadata(req),
		NetworkParticipation:       req.NetworkParticipation,
		NetworkParticipationSource: participation.SourceAutomationJob,
	}, req.Actor)
	if err != nil {
		if errors.Is(err, looppkg.ErrConcurrencyConflict) {
			return automationpkg.LoopStartResult{}, fmt.Errorf("%w: %w", automationpkg.ErrLoopConcurrencyConflict, err)
		}
		return automationpkg.LoopStartResult{}, err
	}
	return automationpkg.LoopStartResult{RunID: string(run.ID)}, nil
}

func automationLoopStartMetadata(req automationpkg.LoopStartRequest) map[string]any {
	metadata := map[string]any{}
	if runID := strings.TrimSpace(req.AutomationRunID); runID != "" {
		metadata["automation_run_id"] = runID
	}
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() {
		scheduledAt := req.ScheduledAt.UTC().Format(time.RFC3339Nano)
		metadata["scheduled_at"] = scheduledAt
		metadata["original_due_at"] = scheduledAt
	}
	if req.CatchUp {
		metadata["catch_up"] = true
		if req.CatchUpPolicy != "" {
			metadata["catch_up_policy"] = string(req.CatchUpPolicy)
		}
	}
	return metadata
}

func automationStartKindToLoopStartKind(kind automationpkg.LoopStartKind) (loopdsl.StartKind, error) {
	switch kind {
	case automationpkg.LoopStartKindTrigger:
		return loopdsl.StartTrigger, nil
	case automationpkg.LoopStartKindSchedule:
		return loopdsl.StartSchedule, nil
	case automationpkg.LoopStartKindWebhook:
		return loopdsl.StartWebhook, nil
	default:
		return "", fmt.Errorf("daemon: unsupported automation loop start kind %q", kind)
	}
}
