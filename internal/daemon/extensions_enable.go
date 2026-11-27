package daemon

import (
	"context"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) enableTargetUnlocked(
	ctx context.Context, name string, actor taskpkg.ActorContext,
	target extensionMutationTarget,
	additionalTargets ...extensionMutationTarget,
) (contract.ExtensionEnableResult, error) {
	var result contract.ExtensionEnableResult
	previous, getErr := s.registry.IsEnabledForProfileContext(ctx, name, extensionDefaultProfileLens().ID)
	if getErr != nil {
		return result, getErr
	}
	preview, previewErr := s.previewExtensionTarget(ctx, name, target, additionalTargets...)
	if previewErr != nil {
		return result, previewErr
	}
	if len(preview.MissingInputs) > 0 || len(preview.MissingEnv) > 0 {
		return result, &extensionpkg.InputsRequiredError{
			MissingInputs: slices.Clone(preview.MissingInputs), MissingEnv: slices.Clone(preview.MissingEnv),
		}
	}
	if len(preview.AgentConflicts) > 0 {
		return result, &extensionpkg.AgentConflictError{Agents: slices.Clone(preview.AgentConflicts)}
	}
	if enableErr := s.registry.Enable(name); enableErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, previous, enableErr)
	}
	if reloadErr := s.reloadExtensionProfile(ctx, name, extensionDefaultProfileLens().ID, true); reloadErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, previous, reloadErr)
	}
	item, statusErr := s.installedTargetStatus(ctx, name, target)
	if statusErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, previous, statusErr)
	}
	automationStarted := slices.Clone(preview.AutomationStarting)
	result = contract.ExtensionEnableResult{Extension: item, AutomationStarted: automationStarted}
	if eventErr := s.recordExtensionEnableEvents(
		ctx,
		actor,
		&result,
	); eventErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, previous, eventErr)
	}
	return result, nil
}
