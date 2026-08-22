package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type automationLoopStarter struct {
	service        automationLoopService
	resolver       looppkg.DefinitionResolver
	inputEntities  looppkg.InputEntityCatalog
	runtimeCatalog looppkg.WorkspaceRuntimeCatalog
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
	homePaths compozyconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
	participationResolver participation.Resolver,
	inputEntities looppkg.InputEntityCatalog,
	runtimeCatalog looppkg.WorkspaceRuntimeCatalog,
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
		looppkg.WithInputDefaultsResolver(newLoopInputDefaultsResolver(homePaths, workspaceResolver)),
	}
	if inputEntities != nil {
		options = append(options, looppkg.WithInputEntityCatalog(inputEntities))
	}
	if runtimeCatalog != nil {
		options = append(options, looppkg.WithRuntimeCatalog(runtimeCatalog))
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
		service:        service,
		resolver:       resolver,
		inputEntities:  inputEntities,
		runtimeCatalog: runtimeCatalog,
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
	validation := looppkg.StartTargetValidation{
		WorkspaceID:  looppkg.WorkspaceID(strings.TrimSpace(req.WorkspaceID)),
		LoopName:     strings.TrimSpace(req.LoopName),
		Kind:         kind,
		Inputs:       req.Inputs,
		InputMapping: req.InputMapping,
	}
	if err := looppkg.ValidateStartTarget(ctx, s.resolver, validation); err != nil {
		return err
	}
	resolved, err := s.resolver.ResolveLoop(ctx, validation.WorkspaceID, validation.LoopName)
	if err != nil {
		return err
	}
	values := make(map[string]any, len(validation.Inputs))
	origins := make(map[string]looppkg.InputOrigin, len(validation.Inputs))
	for key, value := range validation.Inputs {
		values[key] = value
		origins[key] = looppkg.InputOriginAutomation
	}
	return looppkg.ValidateInputEntities(
		ctx,
		validation.WorkspaceID,
		validation.LoopName,
		resolved.Definition,
		looppkg.ResolvedInputs{Values: values, Origins: origins},
		s.inputEntities,
		s.runtimeCatalog,
	)
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
		ProfileID:                  strings.TrimSpace(req.ProfileID),
		Values:                     values,
		StartMetadata:              automationLoopStartMetadata(req),
		NetworkParticipation:       req.NetworkParticipation,
		NetworkParticipationSource: participation.SourceAutomationJob,
		Admission:                  automationLoopAdmission(req),
	}, req.Actor)
	if err != nil {
		if errors.Is(err, looppkg.ErrConcurrencyConflict) {
			return automationpkg.LoopStartResult{}, fmt.Errorf("%w: %w", automationpkg.ErrLoopConcurrencyConflict, err)
		}
		return automationpkg.LoopStartResult{}, err
	}
	result := automationpkg.LoopStartResult{RunID: string(run.ID)}
	if run.RunStartState != nil && run.Admission != nil {
		result.Suppressed = run.Admission.Suppressed
		if run.Admission.Claim != nil {
			result.SuppressedCount = run.Admission.Claim.SuppressedCount
		}
	}
	return result, nil
}

func automationLoopAdmission(req automationpkg.LoopStartRequest) *looppkg.AdmissionIdentity {
	sourceKey := strings.TrimSpace(req.SourceKey)
	eventKey := strings.TrimSpace(req.EventKey)
	if sourceKey == "" || eventKey == "" {
		return nil
	}
	return &looppkg.AdmissionIdentity{SourceKey: sourceKey, EventKey: eventKey}
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
