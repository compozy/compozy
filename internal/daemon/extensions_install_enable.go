package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) enableInstalledExtension(
	ctx context.Context, prepared preparedDaemonExtensionInstall, req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext, item *contract.ExtensionPayload,
) error {
	var result contract.ExtensionEnableResult
	var err error
	switch {
	case item.Enabled:
		result.Extension = *item
	case prepared.target.profile.ID == extensionDefaultProfileLens().ID:
		result, err = s.enableTargetUnlocked(ctx, prepared.name, contract.EnableExtensionRequest{
			ConfirmGatewayDigest: req.ConfirmGatewayDigest,
		}, actor, prepared.target)
	default:
		err = s.applyProfileEnablement(
			ctx, prepared.name, prepared.target.profile.ID, prepared.target.profile.Name, true, actor,
		)
		if err == nil {
			result.Extension, err = s.installedTargetStatus(ctx, prepared.name, prepared.target)
			if err != nil {
				err = s.rollbackProfileEnablement(ctx, prepared.name, prepared.target.profile.ID, false, err)
			}
		}
	}

	if err == nil {
		*item = result.Extension
	} else {
		item.Enabled = false
		item.InstallEnableError = installEnableError(err)
	}
	event := extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionInstallCompleted, ExtensionName: prepared.name,
		SourceKind: string(req.Source), DigestMatched: item.DigestMatched, Enabled: item.Enabled,
	}
	if item.InstallEnableError != nil {
		event.EnableErrorClass = item.InstallEnableError.Code
	}
	return s.recordCanonicalExtensionLifecycleEvent(ctx, actor, event)
}

func installEnableError(err error) *contract.ExtensionEnableError {
	// Install outcomes also reach CLI and native tools without an HTTP masking boundary.
	payload := core.ExtensionEnableFailure(err, true)
	return &payload
}
