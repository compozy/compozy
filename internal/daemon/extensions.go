package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	registrypkg "github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
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

func (s *daemonExtensionService) Status(ctx context.Context, name string) (contract.ExtensionPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionPayload{}, err
	}

	ext, err := s.lookup(ctx, name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.payloadFromExtension(ctx, ext)
}

func (s *daemonExtensionService) reload(ctx context.Context) error {
	if s.runtime == nil {
		return nil
	}

	reloadErr := s.runtime.Reload(ctx)
	return errors.Join(reloadErr, s.syncExtensionConsumers(ctx))
}

func (s *daemonExtensionService) syncExtensionConsumers(ctx context.Context) error {
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
	if s.extensionKit != nil {
		syncErr = errors.Join(syncErr, s.extensionKit.Sync(ctx))
	}
	return syncErr
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

func (s *daemonExtensionService) payloadFromExtension(
	ctx context.Context,
	ext *extensionpkg.Extension,
) (contract.ExtensionPayload, error) {
	payload := extensionpkg.DescribeExtension(ext, s.runtime != nil, s.now())
	if ext == nil {
		return payload, nil
	}
	key := extensionpkg.InstanceKey{Name: ext.Info.Name, WorkspaceID: ext.Status.WorkspaceID}.Normalize()
	if s.envBindings != nil {
		bindings, err := s.envBindings.ListEnvBindings(ctx, key.Name, key.WorkspaceID)
		if err != nil {
			return contract.ExtensionPayload{}, fmt.Errorf("daemon: list extension secret bindings for status: %w", err)
		}
		boundDeclared := make(map[string]struct{}, len(bindings))
		declared := make(map[string]struct{}, len(payload.RequiresEnv))
		for _, name := range payload.RequiresEnv {
			declared[strings.TrimSpace(name)] = struct{}{}
		}
		for _, binding := range bindings {
			name := strings.TrimSpace(binding.EnvName)
			payload.BoundEnvKeys = append(payload.BoundEnvKeys, name)
			if _, ok := declared[name]; ok {
				boundDeclared[name] = struct{}{}
			}
		}
		slices.Sort(payload.BoundEnvKeys)
		missing := make([]string, 0, len(payload.MissingEnv))
		for _, name := range payload.MissingEnv {
			if _, bound := boundDeclared[strings.TrimSpace(name)]; !bound {
				missing = append(missing, name)
			}
		}
		payload.MissingEnv = missing
	}
	if strings.TrimSpace(payload.NetworkRequirementDigest) != "" {
		confirmation, err := s.registry.NetworkConfirmation(key)
		if err != nil {
			return contract.ExtensionPayload{}, err
		}
		payload.NetworkRequirementDigest = confirmation.Digest
		payload.NetworkConfirmationRequired = confirmation.Digest != "" &&
			(strings.TrimSpace(confirmation.ConfirmedBy) == "" || confirmation.ConfirmedAt.IsZero())
	}
	return payload, nil
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
		Error:          value.Error,
	}
}
