package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	eventspkg "github.com/compozy/agh/internal/events"
	extensionpkg "github.com/compozy/agh/internal/extension"
	registrypkg "github.com/compozy/agh/internal/registry"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func extensionEventSummaryStore(registry Registry) store.EventSummaryStore {
	writer, ok := registry.(store.EventSummaryStore)
	if !ok {
		return nil
	}
	return writer
}

func (s *daemonExtensionService) List(ctx context.Context) ([]contract.ExtensionPayload, error) {
	if err := s.checkReady(); err != nil {
		return nil, err
	}

	infos, err := s.registry.List()
	if err != nil {
		return nil, err
	}

	items := make([]contract.ExtensionPayload, 0, len(infos))
	for _, info := range infos {
		item, err := s.Status(ctx, info.Name)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

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
	installedBy := extensionInstalledBy(actor)

	if strings.TrimSpace(req.Slug) != "" || strings.TrimSpace(req.Path) == "" {
		installReq, err := s.marketplaceInstallRequest(ctx, req, installedBy)
		if err != nil {
			return contract.ExtensionPayload{}, err
		}
		installReq.ObserveDigestVerification = func(
			trust *extensionpkg.MarketplaceTrustEvidence,
			verificationErr error,
		) {
			s.observeExtensionDigestVerification(ctx, actor, trust, verificationErr)
		}
		info, err := extensionpkg.InstallMarketplaceManaged(
			ctx,
			s.homePaths,
			s.registry,
			s.marketplaceSourceLoader(),
			installReq,
		)
		if err != nil {
			return contract.ExtensionPayload{}, err
		}
		if err := s.reload(ctx); err != nil {
			return contract.ExtensionPayload{}, s.rollbackFailedInstall(ctx, info.Name, err)
		}
		return s.completeExtensionInstall(ctx, actor, info.Name)
	}

	manifest, err := extensionpkg.LoadManifest(strings.TrimSpace(req.Path))
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	cfg := s.marketplaceConfig()
	if err := extensionpkg.ValidateUnverifiedSideLoad(
		manifest.Name,
		req.Path,
		cfg.AllowUnverified,
		req.AllowUnverified,
	); err != nil {
		return contract.ExtensionPayload{}, err
	}
	provenance := extensionpkg.LocalPathProvenance(manifest, req.Path, req.Checksum, s.now(), req.AllowUnverified)
	provenance.InstalledBy = installedBy
	if err := extensionpkg.InstallLocalManaged(
		s.homePaths,
		s.registry,
		manifest,
		req.Path,
		req.Checksum,
		extensionpkg.WithInstallProvenance(provenance),
	); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.reload(ctx); err != nil {
		return contract.ExtensionPayload{}, s.rollbackFailedInstall(ctx, manifest.Name, err)
	}
	return s.completeExtensionInstall(ctx, actor, manifest.Name)
}

func (s *daemonExtensionService) completeExtensionInstall(
	ctx context.Context,
	actor taskpkg.ActorContext,
	name string,
) (contract.ExtensionPayload, error) {
	item, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.recordExtensionEvent(ctx, eventspkg.ExtensionInstalled, actor, item); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return item, nil
}

func (s *daemonExtensionService) Update(
	ctx context.Context,
	name string,
	req contract.UpdateExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	items, err := s.UpdateBatch(ctx, extensionpkg.MarketplaceUpdateRequest{
		Names:           []string{name},
		Version:         req.Version,
		CheckOnly:       req.CheckOnly,
		AllowUnverified: req.AllowUnverified,
	}, actor)
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
	req extensionpkg.MarketplaceUpdateRequest,
	actor taskpkg.ActorContext,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	if err := s.checkReady(); err != nil {
		return nil, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return nil, err
	}
	req.InstalledBy = extensionInstalledBy(actor)
	cfg := s.marketplaceConfig()
	req.PolicyAllowsUnverified = cfg.AllowUnverified
	req.ResolveTrust = s.marketplaceTrustResolver()
	req.ObserveDigestVerification = func(
		trust *extensionpkg.MarketplaceTrustEvidence,
		verificationErr error,
	) {
		s.observeExtensionDigestVerification(ctx, actor, trust, verificationErr)
	}
	items, updateErr := extensionpkg.UpdateMarketplaceManaged(
		ctx,
		s.homePaths,
		s.registry,
		s.marketplaceSourceLoader(),
		req,
		s.reload,
	)
	return s.finalizeMarketplaceUpdateBatch(ctx, actor, items, updateErr)
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
	removed, err := extensionpkg.RemoveManagedExtension(ctx, s.registry, name, s.reload)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	item := contract.ManagedExtensionRemovePayload{
		Name:     removed.Name,
		Path:     removed.Path,
		Status:   removed.Status,
		Warnings: append([]contract.DiagnosticItem(nil), removed.Warnings...),
	}
	if err := s.recordExtensionRemoveEvent(ctx, actor, item); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	return item, nil
}

func (s *daemonExtensionService) Provenance(
	ctx context.Context,
	name string,
) (contract.ExtensionProvenancePayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionProvenancePayload{}, err
	}
	payload, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionProvenancePayload{}, err
	}
	if payload.Provenance == nil {
		return contract.ExtensionProvenancePayload{}, extensionpkg.ErrExtensionNotFound
	}
	return *payload.Provenance, nil
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

	var rollbackErr error
	if err := s.registry.Uninstall(trimmedName); err != nil && !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		rollbackErr = errors.Join(
			rollbackErr,
			fmt.Errorf("daemon: rollback extension registry row %q: %w", trimmedName, err),
		)
	}

	managedPath := extensionpkg.ManagedInstallPath(s.homePaths, trimmedName)
	if err := os.RemoveAll(managedPath); err != nil {
		rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: rollback extension files %q: %w", managedPath, err))
	}

	if err := s.reload(ctx); err != nil {
		rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: reload after extension install rollback: %w", err))
	}

	return errors.Join(installErr, rollbackErr)
}

func (s *daemonExtensionService) Enable(
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
	if err := s.registry.Enable(name); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.reload(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	item, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.recordExtensionEvent(ctx, eventspkg.ExtensionEnabled, actor, item); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return item, nil
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
	if err := s.registry.Disable(name); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.reload(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	item, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.recordExtensionEvent(ctx, eventspkg.ExtensionDisabled, actor, item); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return item, nil
}

func (s *daemonExtensionService) Status(ctx context.Context, name string) (contract.ExtensionPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionPayload{}, err
	}

	ext, err := s.lookup(ctx, name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.payloadFromExtension(ext), nil
}

func (s *daemonExtensionService) reload(ctx context.Context) error {
	if s.runtime == nil {
		return nil
	}

	reloadErr := s.runtime.Reload(ctx)
	var syncErr error
	if s.agentSkill != nil {
		syncErr = errors.Join(syncErr, s.agentSkill.Sync(ctx))
	}
	if s.hookBinds != nil {
		syncErr = errors.Join(syncErr, s.hookBinds.Sync(ctx))
	}
	if s.toolMCP != nil {
		syncErr = errors.Join(syncErr, s.toolMCP.Sync(ctx))
	}
	if s.bundles != nil {
		syncErr = errors.Join(syncErr, s.bundles.Sync(ctx))
	}
	if s.loops != nil {
		syncErr = errors.Join(syncErr, s.loops.Sync(ctx))
	}
	return errors.Join(reloadErr, syncErr)
}

func (s *daemonExtensionService) lookup(ctx context.Context, name string) (*extensionpkg.Extension, error) {
	return loadExtensionSnapshot(ctx, s.registry, s.runtime, s.logger, name)
}

func loadExtensionSnapshot(
	ctx context.Context,
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
	logger *slog.Logger,
	name string,
) (*extensionpkg.Extension, error) {
	if registry == nil {
		return nil, errors.New("daemon: extension registry is required")
	}

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, errors.New("extension: extension name is required")
	}

	if runtime != nil {
		ext, err := runtime.Get(trimmed)
		if err == nil {
			populateExtensionManifest(ctx, logger, ext)
			return ext, nil
		}
		if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			return nil, err
		}
	}

	info, err := registry.Get(trimmed)
	if err != nil {
		return nil, err
	}

	ext := &extensionpkg.Extension{
		Info: *info,
		Status: extensionpkg.ExtensionStatus{
			Name:    info.Name,
			Version: info.Version,
			Source:  info.Source,
			Enabled: info.Enabled,
		},
	}
	populateExtensionManifest(ctx, logger, ext)
	return ext, nil
}

func populateExtensionManifest(ctx context.Context, logger *slog.Logger, ext *extensionpkg.Extension) {
	if ext == nil || ext.Manifest != nil || strings.TrimSpace(ext.Info.ManifestPath) == "" {
		return
	}

	manifest, err := extensionpkg.LoadManifest(filepath.Dir(ext.Info.ManifestPath))
	if err != nil {
		if logger != nil {
			logger.Debug(
				"daemon: load extension manifest for status failed",
				"path",
				ext.Info.ManifestPath,
				"error",
				err,
			)
		}
		return
	}
	ext.Manifest = manifest
	if bundles, err := extensionpkg.LoadBundleSpecs(ctx, filepath.Dir(ext.Info.ManifestPath), manifest); err == nil {
		ext.Bundles = bundles
	} else if logger != nil {
		logger.Debug("daemon: load extension bundles for status failed", "path", ext.Info.ManifestPath, "error", err)
	}
}

func (s *daemonExtensionService) payloadFromExtension(ext *extensionpkg.Extension) contract.ExtensionPayload {
	return extensionpkg.DescribeExtension(ext, s.runtime != nil, s.now())
}

func (s *daemonExtensionService) marketplaceSourceLoader() extensionpkg.MarketplaceSourceLoader {
	return func(ctx context.Context) ([]registrypkg.Source, error) {
		loader := s.marketplaceLoader
		if loader == nil {
			loader = defaultDaemonExtensionMarketplaceSourceLoader
		}
		return loader(ctx, s.marketplaceConfig())
	}
}

func (s *daemonExtensionService) checkReady() error {
	if s == nil {
		return errors.New("daemon: extension service is required")
	}
	if s.registry == nil {
		return errors.New("daemon: extension registry is required")
	}
	return nil
}

func validateExtensionWriteActor(actor taskpkg.ActorContext) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if !actor.Authority.Write {
		return taskpkg.ErrPermissionDenied
	}
	return nil
}

func extensionInstalledBy(actor taskpkg.ActorContext) string {
	actorKind := strings.TrimSpace(string(actor.Actor.Kind.Normalize()))
	actorRef := strings.TrimSpace(actor.Actor.Ref)
	if actorKind == "" {
		return actorRef
	}
	if actorRef == "" {
		return actorKind
	}
	return actorKind + ":" + actorRef
}

func extensionUpdatePayload(value extensionpkg.MarketplaceUpdateResult) contract.ManagedExtensionUpdatePayload {
	return contract.ManagedExtensionUpdatePayload{
		Name:           value.Name,
		Slug:           value.Slug,
		Registry:       value.Registry,
		CurrentVersion: value.CurrentVersion,
		LatestVersion:  value.LatestVersion,
		Path:           value.Path,
		Status:         value.Status,
		Warnings:       append([]contract.DiagnosticItem(nil), value.Warnings...),
	}
}
