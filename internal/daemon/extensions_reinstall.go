package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// The install coordinator holds the package and input-cell locks before inspecting the existing record.
func (s *daemonExtensionService) reinstallPreparedExtension(
	ctx context.Context, prepared preparedDaemonExtensionInstall, installed extensionpkg.ExtensionInfo,
	request contract.InstallExtensionRequest, actor taskpkg.ActorContext, item *contract.ExtensionPayload,
) error {
	if err := prepared.published.ValidateReinstall(installed); err != nil {
		return err
	}
	if !actor.Scope.Operator && installed.Provenance.SourceRef == "" && prepared.published.Origin().SourceRef != "" {
		return taskpkg.ErrPermissionDenied
	}
	attachment, err := s.registry.ResolveInstallation(ctx, installed.Name, extensionpkg.InstallationScope{
		ProfileID: prepared.target.profile.ID, WorkspaceID: prepared.target.scope.WorkspaceID,
	})
	if err != nil {
		return err
	}
	if attachment.Scope.WorkspaceID != prepared.target.scope.WorkspaceID {
		return &extensionpkg.ExtensionNotFoundError{Name: installed.Name}
	}
	update := extensionpkg.MarketplaceUpdateRequest{}
	confirmed := s.configureUpdateNetworkGate(&update, request.ConfirmNetworkDigest, actor)
	s.configureUpdateProfileGate(ctx, &update, actor)
	plans := s.configureUpdateInputGate(ctx, &update, request.Inputs, prepared.target)
	_, needsConfirmation, err := candidateNetworkConfirmationRequirement(installed, prepared.manifest)
	if err != nil {
		return err
	}
	if prepared.published.MatchesInstalled(installed) && !needsConfirmation {
		if err := update.PreflightCandidate(installed, prepared.manifest); err != nil {
			return err
		}
		plan := plans[installed.Name]
		if len(plan.rows) == 0 && len(plan.writes) == 0 {
			*item, err = s.installedTargetStatus(ctx, installed.Name, prepared.target)
			return err
		}
		// The candidate and input plan have already passed preflight under the lifecycle lock.
		update.PreflightCandidate = nil
	}
	complete := func(ctx context.Context) error {
		s.evictExtensionMCPHealth(installed.Name, "")
		var statusErr error
		*item, statusErr = s.installedTargetStatus(ctx, installed.Name, prepared.target)
		if statusErr != nil {
			return statusErr
		}
		if confirmation, ok := confirmed[installed.Name]; ok {
			if err := s.recordExtensionNetworkConfirmedEvent(
				ctx, actor, extensionpkg.GlobalInstanceKey(installed.Name), confirmation,
			); err != nil {
				return err
			}
		}
		return s.recordCanonicalExtensionLifecycleEvent(ctx, actor, extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionInstallCompleted, ExtensionName: installed.Name,
			SourceKind: string(request.Source), DigestMatched: item.DigestMatched,
		})
	}
	warnings, err := prepared.published.Reinstall(
		ctx, installed, update.PreflightCandidate, update.CommitCandidate, update.RollbackCandidate, s.reload, complete,
	)
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		s.logger.Warn("daemon: clean committed extension reinstall", "extension", installed.Name, "code", warning.Code)
	}
	return nil
}
