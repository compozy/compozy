package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) Install(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionPayload{}, err
	}
	event := extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionInstallFailed, ExtensionName: req.Ref, SourceKind: string(req.Source),
	}
	installedBy := extensionInstalledBy(actor)
	prepared, err := s.prepareExtensionInstall(ctx, req, actor, installedBy)
	if err != nil {
		return contract.ExtensionPayload{}, errors.Join(
			err,
			s.recordCanonicalExtensionLifecycleEvent(ctx, actor, event),
		)
	}
	event.ExtensionName = prepared.name
	confirmation, err := s.prepareInstallNetworkConfirmation(
		prepared.manifest,
		strings.TrimSpace(req.ConfirmNetworkDigest),
		actor,
	)
	if err != nil {
		return contract.ExtensionPayload{}, errors.Join(err, prepared.Close())
	}
	var item contract.ExtensionPayload
	mutation := func() error {
		if err := prepared.commit(); err != nil {
			return err
		}
		if confirmation != nil {
			if err := s.registry.ConfirmNetworkRequirement(
				extensionpkg.GlobalInstanceKey(prepared.name),
				confirmation.Digest,
				confirmation.ConfirmedBy,
				confirmation.ConfirmedAt,
			); err != nil {
				return s.rollbackFailedInstall(ctx, prepared.name, err)
			}
			if err := s.recordExtensionNetworkConfirmedEvent(
				ctx, actor, extensionpkg.GlobalInstanceKey(prepared.name), *confirmation,
			); err != nil {
				return s.rollbackFailedInstall(ctx, prepared.name, err)
			}
		}
		if prepared.manifest != nil && len(prepared.manifest.Profiles) > 0 {
			if s.profiles == nil {
				return s.rollbackFailedInstall(
					ctx,
					prepared.name,
					errors.New("daemon: profile manager is required for declared profiles"),
				)
			}
			results, applyErr := extensionpkg.ApplyDeclaredProfiles(ctx, s.profiles, prepared.manifest)
			if applyErr != nil {
				return s.rollbackFailedInstall(ctx, prepared.name, applyErr)
			}
			if eventErr := s.recordDeclaredProfileCreatedEvents(
				ctx, actor, prepared.name, results,
			); eventErr != nil {
				return s.rollbackFailedInstall(ctx, prepared.name, eventErr)
			}
		}
		if err := s.reload(ctx); err != nil {
			return s.rollbackFailedInstall(ctx, prepared.name, err)
		}
		item, err = s.Status(ctx, prepared.name)
		if err != nil {
			return s.rollbackFailedInstall(ctx, prepared.name, err)
		}
		completedEvent := event
		completedEvent.Type = eventspkg.ExtensionInstallCompleted
		completedEvent.ExtensionName = item.Name
		completedEvent.DigestMatched = item.DigestMatched
		if err := s.recordCanonicalExtensionLifecycleEvent(ctx, actor, completedEvent); err != nil {
			return s.rollbackFailedInstall(ctx, prepared.name, err)
		}
		return nil
	}
	err = s.lifecycle.withInstance(ctx, extensionpkg.GlobalInstanceKey(prepared.name), mutation)
	if err = s.finishPreparedInstall(prepared, err); err != nil {
		return contract.ExtensionPayload{}, errors.Join(
			err,
			s.recordCanonicalExtensionLifecycleEvent(ctx, actor, event),
		)
	}
	return item, nil
}

func (s *daemonExtensionService) finishPreparedInstall(
	prepared preparedDaemonExtensionInstall,
	mutationErr error,
) error {
	cleanupErr := prepared.Close()
	if mutationErr != nil {
		return errors.Join(mutationErr, cleanupErr)
	}
	if cleanupErr != nil {
		s.logger.Warn(
			"daemon: clean committed extension install staging",
			"extension",
			prepared.name,
			"error",
			cleanupErr,
		)
	}
	return nil
}

func (s *daemonExtensionService) Update(
	ctx context.Context,
	name string,
	req contract.UpdateExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ManagedExtensionUpdatePayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ManagedExtensionUpdatePayload{}, err
	}
	var items []contract.ManagedExtensionUpdatePayload
	err := s.lifecycle.withName(ctx, name, func() error {
		var updateErr error
		items, updateErr = s.updateBatchUnlocked(ctx, contract.UpdateExtensionsRequest{
			Names:           []string{name},
			Version:         req.Version,
			CheckOnly:       req.CheckOnly,
			AllowUnverified: req.AllowUnverified,
		}, actor, strings.TrimSpace(req.ConfirmNetworkDigest))
		return updateErr
	})
	if err != nil {
		return contract.ManagedExtensionUpdatePayload{}, err
	}
	if len(items) == 0 {
		return contract.ManagedExtensionUpdatePayload{}, extensionpkg.ErrExtensionNotFound
	}
	return items[0], nil
}

func (s *daemonExtensionService) UpdateBatch(
	ctx context.Context,
	req contract.UpdateExtensionsRequest,
	actor taskpkg.ActorContext,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	if err := s.checkReady(); err != nil {
		return nil, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return nil, err
	}
	names, err := s.lifecycleUpdateNames(req)
	if err != nil {
		return nil, err
	}
	var items []contract.ManagedExtensionUpdatePayload
	err = s.lifecycle.withNames(ctx, names, func() error {
		var updateErr error
		items, updateErr = s.updateBatchUnlocked(ctx, req, actor, "")
		return updateErr
	})
	return items, err
}

func (s *daemonExtensionService) updateBatchUnlocked(
	ctx context.Context,
	req contract.UpdateExtensionsRequest,
	actor taskpkg.ActorContext,
	confirmNetworkDigest string,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	cfg := s.marketplaceConfig()
	domainReq := extensionpkg.MarketplaceUpdateRequest{
		Names:                  req.Names,
		All:                    req.All,
		CheckOnly:              req.CheckOnly,
		Version:                req.Version,
		AllowUnverified:        req.AllowUnverified,
		InstalledBy:            extensionInstalledBy(actor),
		PolicyAllowsUnverified: cfg.Trust.AllowUnverified,
		ResolveTrust:           s.marketplaceTrustResolver(),
	}
	domainReq.ObserveDigestVerification = func(
		trust *extensionpkg.MarketplaceTrustEvidence,
		verificationErr error,
	) {
		s.observeExtensionDigestVerification(ctx, actor, trust, verificationErr)
	}
	confirmed := s.configureUpdateNetworkGate(&domainReq, confirmNetworkDigest, actor)
	previousCommitCandidate := domainReq.CommitCandidate
	domainReq.CommitCandidate = func(
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
	items, updateErr := extensionpkg.UpdateMarketplaceManaged(
		ctx,
		s.homePaths,
		s.registry,
		s.marketplaceSourceLoader(),
		domainReq,
		s.reload,
	)
	payloads, finalizeErr := s.finalizeMarketplaceUpdateBatch(ctx, actor, items, updateErr)
	return payloads, errors.Join(
		finalizeErr,
		s.recordCommittedUpdateNetworkConfirmations(ctx, actor, payloads, confirmed),
	)
}

func (s *daemonExtensionService) Remove(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	var item contract.ManagedExtensionRemovePayload
	err := s.lifecycle.withName(ctx, name, func() error {
		retirement, retireErr := s.retireExtensionSecretBindings(ctx, extensionpkg.GlobalInstanceKey(name))
		if retireErr != nil {
			return retireErr
		}
		removed, removeErr := extensionpkg.RemoveManagedExtension(ctx, s.homePaths, s.registry, name, s.reload)
		if removeErr != nil {
			return errors.Join(removeErr, retirement.rollback(ctx, s))
		}
		s.evictExtensionMCPHealth(name, "")
		item = contract.ManagedExtensionRemovePayload{
			Name:           removed.Name,
			Path:           removed.Path,
			DataPath:       removed.DataPath,
			QuarantinePath: removed.QuarantinePath,
			Status:         removed.Status,
			Warnings:       append([]contract.DiagnosticItem(nil), removed.Warnings...),
		}
		return s.recordExtensionRemoveEvent(ctx, actor, item)
	})
	return item, err
}

func (s *daemonExtensionService) rollbackFailedInstall(
	ctx context.Context,
	name string,
	installErr error,
) error {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return installErr
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extensionLifecycleRollbackTimeout)
	defer cancel()

	var rollbackErr error
	if err := s.registry.Uninstall(trimmedName); err != nil && !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		rollbackErr = errors.Join(
			rollbackErr,
			fmt.Errorf("daemon: rollback extension registry row %q: %w", trimmedName, err),
		)
	}

	if err := extensionpkg.RemoveManagedInstall(s.homePaths, trimmedName); err != nil {
		rollbackErr = errors.Join(
			rollbackErr,
			fmt.Errorf("daemon: rollback extension files %q: %w", trimmedName, err),
		)
	}

	if err := s.reload(rollbackCtx); err != nil {
		rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: reload after extension install rollback: %w", err))
	}

	return errors.Join(installErr, rollbackErr)
}

func (s *daemonExtensionService) Enable(
	ctx context.Context,
	name string,
	req contract.EnableExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionEnableResult, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionEnableResult{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionEnableResult{}, err
	}
	var result contract.ExtensionEnableResult
	err := s.lifecycle.withName(ctx, name, func() error {
		info, getErr := s.registry.Get(name)
		if getErr != nil {
			return getErr
		}
		snapshot := snapshotGlobalExtensionLifecycle(info)
		preview, previewErr := s.previewExtension(ctx, name)
		if previewErr != nil {
			return previewErr
		}
		if len(preview.AgentConflicts) > 0 {
			return &extensionpkg.AgentConflictError{Agents: slices.Clone(preview.AgentConflicts)}
		}
		confirmation, confirmErr := s.confirmNetworkForEnable(
			extensionpkg.GlobalInstanceKey(name),
			preview.NetworkRequirementDigest,
			strings.TrimSpace(req.ConfirmNetworkDigest),
			actor,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if enableErr := s.registry.Enable(name); enableErr != nil {
			return s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, enableErr)
		}
		if reloadErr := s.reload(ctx); reloadErr != nil {
			return s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, reloadErr)
		}
		item, statusErr := s.Status(ctx, name)
		if statusErr != nil {
			return s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, statusErr)
		}
		automationStarted := slices.Clone(preview.AutomationStarting)
		result = contract.ExtensionEnableResult{Extension: item, AutomationStarted: automationStarted}
		if eventErr := s.recordExtensionEnableEvents(
			ctx,
			actor,
			extensionpkg.GlobalInstanceKey(name),
			confirmation,
			result,
		); eventErr != nil {
			return s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, eventErr)
		}
		return nil
	})
	return result, err
}

func (s *daemonExtensionService) Disable(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionPayload{}, err
	}
	var item contract.ExtensionPayload
	err := s.lifecycle.withName(ctx, name, func() error {
		info, getErr := s.registry.Get(name)
		if getErr != nil {
			return getErr
		}
		snapshot := snapshotGlobalExtensionLifecycle(info)
		if disableErr := s.registry.Disable(name); disableErr != nil {
			return disableErr
		}
		if reloadErr := s.reload(ctx); reloadErr != nil {
			return s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, reloadErr)
		}
		s.evictExtensionMCPHealth(name, "")
		var statusErr error
		item, statusErr = s.Status(ctx, name)
		if statusErr != nil {
			return s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, statusErr)
		}
		if eventErr := s.recordExtensionEvent(ctx, eventspkg.ExtensionDisabled, actor, item); eventErr != nil {
			return s.rollbackGlobalExtensionLifecycle(ctx, name, snapshot, eventErr)
		}
		return nil
	})
	return item, err
}
