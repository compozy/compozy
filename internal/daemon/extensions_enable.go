package daemon

import (
	"context"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) enableUnlocked(
	ctx context.Context, name string, req contract.EnableExtensionRequest, actor taskpkg.ActorContext,
) (contract.ExtensionEnableResult, error) {
	var result contract.ExtensionEnableResult
	info, getErr := s.registry.Get(name)
	if getErr != nil {
		return result, getErr
	}
	snapshot := snapshotGlobalExtensionLifecycle(info)
	preview, previewErr := s.previewExtension(ctx, name)
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
	confirmation, confirmErr := s.confirmGatewayForEnable(
		extensionpkg.GlobalInstanceKey(name),
		preview.GatewayRequirementDigest,
		strings.TrimSpace(req.ConfirmGatewayDigest),
		actor,
	)
	if confirmErr != nil {
		return result, confirmErr
	}
	if enableErr := s.registry.Enable(name); enableErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, enableErr)
	}
	if reloadErr := s.reloadExtensions(ctx, name); reloadErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, reloadErr)
	}
	item, statusErr := s.Status(ctx, name)
	if statusErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, statusErr)
	}
	automationStarted := slices.Clone(preview.AutomationStarting)
	result = contract.ExtensionEnableResult{Extension: item, AutomationStarted: automationStarted}
	if eventErr := s.recordExtensionEnableEvents(
		ctx,
		actor,
		extensionpkg.GlobalInstanceKey(name),
		confirmation,
		&result,
	); eventErr != nil {
		return result, s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, eventErr)
	}
	return result, nil
}
