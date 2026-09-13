package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) finalizeMarketplaceUpdateBatch(
	ctx context.Context,
	actor taskpkg.ActorContext,
	items []extensionpkg.MarketplaceUpdateResult,
	updateErr error,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	payloads := make([]contract.ManagedExtensionUpdatePayload, 0, len(items))
	resultErr := updateErr
	for _, value := range items {
		item := extensionUpdatePayload(value)
		payloads = append(payloads, item)
		eventType := ""
		switch item.Status {
		case extensionpkg.MarketplaceUpdateStatusUpdated:
			s.evictExtensionMCPHealth(item.Name, "")
			eventType = eventspkg.ExtensionUpdateCompleted
		case extensionpkg.MarketplaceUpdateStatusFailed:
			eventType = eventspkg.ExtensionUpdateFailed
		default:
			continue
		}
		if err := s.recordCanonicalExtensionLifecycleEvent(ctx, actor, extensionpkg.LifecycleEvent{
			Type: eventType, ExtensionName: item.Name, SourceKind: item.Registry,
		}); err != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("daemon: record extension update %q: %w", item.Name, err),
			)
		}
	}
	return payloads, resultErr
}

func (s *daemonExtensionService) scopedMarketplaceUpdateNames(
	ctx context.Context, request contract.UpdateExtensionsRequest, target extensionMutationTarget,
) ([]string, error) {
	candidates, err := extensionpkg.SelectMarketplaceUpdateTargets(s.registry, request.Names, request.All)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(candidates))
	for _, info := range candidates {
		installation, err := s.registry.ResolveInstallation(ctx, info.Name, extensionpkg.InstallationScope{
			ProfileID: target.profile.ID, WorkspaceID: target.scope.WorkspaceID,
		})
		if err == nil && installation.Scope.WorkspaceID != target.scope.WorkspaceID {
			err = &extensionpkg.ExtensionNotFoundError{Name: info.Name}
		}
		if err != nil {
			if request.All && errors.Is(err, extensionpkg.ErrExtensionNotFound) {
				continue
			}
			return nil, err
		}
		names = append(names, info.Name)
	}
	return normalizeLifecycleNames(names), nil
}

func (s *daemonExtensionService) configureUpdateProfileGate(
	ctx context.Context, request *extensionpkg.MarketplaceUpdateRequest, actor taskpkg.ActorContext,
) {
	previousCommitCandidate := request.CommitCandidate
	request.CommitCandidate = func(
		info extensionpkg.ExtensionInfo,
		manifest *extensionpkg.Manifest,
	) error {
		if previousCommitCandidate != nil {
			if err := previousCommitCandidate(info, manifest); err != nil {
				return err
			}
		}
		if manifest == nil || len(manifest.Profiles) == 0 {
			return nil
		}
		if s.profiles == nil {
			return errors.New("daemon: profile manager is required for declared profiles")
		}
		results, err := extensionpkg.ApplyDeclaredProfiles(ctx, s.profiles, manifest)
		if err != nil {
			return err
		}
		return s.recordDeclaredProfileCreatedEvents(ctx, actor, info.Name, results)
	}
}
