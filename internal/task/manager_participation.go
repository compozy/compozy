package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

type taskParticipationProfileStore interface {
	GetExecutionProfile(context.Context, string) (ExecutionProfile, error)
}

func (m *Service) resolveQueuedRunParticipationWithStore(
	ctx context.Context,
	store taskParticipationProfileStore,
	taskRecord Task,
	runID string,
	designationGroupID string,
	runKind RunKind,
	loopRunID string,
	request *participation.Request,
	requestSource participation.Source,
) (participation.Spec, error) {
	definition, err := taskParticipationDefinitionWithStore(ctx, store, taskRecord.ID)
	if err != nil {
		return participation.Spec{}, err
	}
	if taskRecord.Scope.Normalize() == ScopeGlobal {
		spec, resolved, resolveErr := resolveGlobalLocalParticipation(
			request,
			requestSource,
			definition,
		)
		if resolveErr != nil {
			return participation.Spec{}, resolveErr
		}
		if resolved {
			return spec, nil
		}
	}
	if m.participationResolver == nil {
		if request == nil && definition == nil {
			return participation.Spec{
				Version: participation.SpecVersion,
				Mode:    participation.ModeLocal,
				Source:  participation.SourceBuiltInLocal,
			}, nil
		}
		return participation.Spec{}, fmt.Errorf(
			"task: participation resolver is required for task run %q with network intent",
			runID,
		)
	}
	conversationRunID := strings.TrimSpace(runID)
	if groupID := strings.TrimSpace(designationGroupID); groupID != "" {
		conversationRunID = groupID
	}
	return m.participationResolver.Resolve(ctx, participation.ResolveInput{
		WorkspaceID: strings.TrimSpace(taskRecord.WorkspaceID),
		Owner: participation.OwnerRef{
			WorkspaceID: strings.TrimSpace(taskRecord.WorkspaceID),
			Kind:        participation.OwnerKindTaskRun,
			ID:          strings.TrimSpace(runID),
		},
		Request:       request,
		RequestSource: requestSource,
		Definition:    definition,
		RunID:         conversationRunID,
		LoopRunID:     strings.TrimSpace(loopRunID),
		Coordinated: taskRecord.Scope.Normalize() == ScopeWorkspace &&
			runKind.Normalize() == RunKindWorker && strings.TrimSpace(loopRunID) == "",
	})
}

func resolveGlobalLocalParticipation(
	request *participation.Request,
	requestSource participation.Source,
	definition *participation.Request,
) (participation.Spec, bool, error) {
	selected := request
	var source participation.Source
	switch {
	case hasParticipationIntent(request):
		var err error
		source, err = normalizeTaskParticipationRequestSource(requestSource)
		if err != nil {
			return participation.Spec{}, false, err
		}
	case hasParticipationIntent(definition):
		selected = definition
		source = participation.SourceTaskProfile
	default:
		return participation.LocalSpec(), true, nil
	}

	normalized, err := participation.NormalizeIntent(*selected)
	if err != nil {
		return participation.Spec{}, false, fmt.Errorf(
			"task: normalize global participation intent: %w",
			err,
		)
	}
	if normalized.Mode != nil && *normalized.Mode == participation.ModeLive {
		return participation.Spec{}, false, nil
	}
	return participation.Spec{
		Version: participation.SpecVersion,
		Mode:    participation.ModeLocal,
		Source:  source,
	}, true, nil
}

func hasParticipationIntent(request *participation.Request) bool {
	return request != nil && (request.Mode != nil || request.ChannelStrategy != nil ||
		request.ChannelID != nil || request.Bounds != nil)
}

func normalizeTaskParticipationRequestSource(source participation.Source) (participation.Source, error) {
	switch normalized := participation.Source(strings.TrimSpace(string(source))); normalized {
	case "", participation.SourceExplicitRequest:
		return participation.SourceExplicitRequest, nil
	case participation.SourceAutomationJob:
		return participation.SourceAutomationJob, nil
	default:
		return "", fmt.Errorf(
			"%w: network participation request source %q is not allowed",
			ErrValidation,
			normalized,
		)
	}
}

func (m *Service) observeCommittedRunParticipation(
	ctx context.Context,
	observation *participation.ResolvedObservation,
) {
	if m == nil || observation == nil {
		return
	}
	observationCtx := context.WithoutCancel(ctx)
	if err := participation.ObserveResolved(
		observationCtx,
		m.participationResolver,
		*observation,
	); err != nil {
		slog.ErrorContext(
			observationCtx,
			"task: committed network participation observation failed",
			"error", err,
			"workspace_id", observation.WorkspaceID,
			"owner_id", observation.Owner.ID,
		)
	}
}

func taskParticipationDefinitionWithStore(
	ctx context.Context,
	store taskParticipationProfileStore,
	taskID string,
) (*participation.Request, error) {
	profile, err := store.GetExecutionProfile(ctx, strings.TrimSpace(taskID))
	switch {
	case errors.Is(err, ErrExecutionProfileNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	case profile.NetworkParticipation == nil:
		return nil, nil
	default:
		request := *profile.NetworkParticipation
		return &request, nil
	}
}

type taskRunIdempotencyReader interface {
	GetTaskRunByIdempotencyKey(context.Context, string, Origin) (Run, error)
}

func existingQueuedRunWithStore(
	ctx context.Context,
	store taskRunIdempotencyReader,
	taskID string,
	idempotencyKey string,
	origin Origin,
) (*Run, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, false, nil
	}
	run, err := store.GetTaskRunByIdempotencyKey(ctx, idempotencyKey, origin)
	switch {
	case errors.Is(err, ErrTaskRunIdempotencyNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	case strings.TrimSpace(run.TaskID) != strings.TrimSpace(taskID):
		return nil, false, fmt.Errorf(
			"%w: idempotency key %q is already bound to task %q",
			ErrValidation,
			idempotencyKey,
			run.TaskID,
		)
	default:
		return &run, true, nil
	}
}
