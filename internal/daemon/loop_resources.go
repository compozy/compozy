package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/resources"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const loopManagedIDPrefix = "daemon.sync.loop."

type loopResourcePublisher interface {
	Sync(context.Context) error
}

type loopPublicationInput struct {
	sourceKey string
	scope     resources.ResourceScope
	spec      looppkg.ResourceSpec
}

type loopDeclarationProvider func(context.Context) ([]loopPublicationInput, error)

type loopSourceSyncer struct {
	store     resources.Store[looppkg.ResourceSpec]
	codec     resources.KindCodec[looppkg.ResourceSpec]
	actor     resources.MutationActor
	logger    *slog.Logger
	trigger   func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	providers []loopDeclarationProvider
}

var _ loopResourcePublisher = (*loopSourceSyncer)(nil)

func newLoopProjector(
	catalog *resourceCatalog[looppkg.ResourceSpec],
) resources.TypedProjector[looppkg.ResourceSpec] {
	if catalog == nil {
		return nil
	}
	return &resourceCatalogProjector[looppkg.ResourceSpec]{
		kind:      looppkg.ResourceKind,
		catalog:   catalog,
		cloneSpec: looppkg.CloneResourceSpec,
	}
}

func appendLoopProjectorRegistration(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	if deps.LoopCatalog == nil {
		return registrations, nil
	}
	return appendTypedProjectorRegistration(
		registrations,
		deps.CodecRegistry,
		looppkg.ResourceKind,
		newLoopProjector(deps.LoopCatalog),
	)
}

func newLoopSourceSyncer(
	store resources.Store[looppkg.ResourceSpec],
	codec resources.KindCodec[looppkg.ResourceSpec],
	actor resources.MutationActor,
	logger *slog.Logger,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
	providers ...loopDeclarationProvider,
) loopResourcePublisher {
	if store == nil || codec == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &loopSourceSyncer{
		store:     store,
		codec:     codec,
		actor:     actor,
		logger:    logger,
		trigger:   trigger,
		providers: append([]loopDeclarationProvider(nil), providers...),
	}
}

func (d *Daemon) newLoopPublisher(
	state *bootState,
	registry *extensionpkg.Registry,
) (loopResourcePublisher, error) {
	if state == nil || state.resourceKernel == nil || state.resourceCodecs == nil {
		return nil, nil
	}
	codec, err := resources.ResolveCodec[looppkg.ResourceSpec](state.resourceCodecs, looppkg.ResourceKind)
	if err != nil {
		return nil, err
	}
	store, err := resources.NewStore[looppkg.ResourceSpec](resourceRawStore(state.resourceKernel), codec)
	if err != nil {
		return nil, err
	}
	return newLoopSourceSyncer(
		store,
		codec,
		loopSyncActor(),
		state.logger,
		func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
		daemonLoopDeclarationProvider(d.homePaths, state.registry, state.workspaceResolver, state.logger),
		extensionLoopDeclarationProvider(registry, state.currentExtensionRuntime, state.logger),
	), nil
}

func loopSyncActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "loop-sync",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "loop-sync",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (s *loopSourceSyncer) Sync(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: loop sync context is required")
	}
	desired, err := s.desiredLoops(ctx)
	if err != nil {
		return err
	}
	changed, err := s.syncLoops(ctx, desired)
	if err != nil {
		return err
	}
	if changed && s.trigger != nil {
		if err := s.trigger(ctx, looppkg.ResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}
	return nil
}

type desiredLoopResource struct {
	id      string
	scope   resources.ResourceScope
	spec    looppkg.ResourceSpec
	encoded []byte
}

func (s *loopSourceSyncer) desiredLoops(ctx context.Context) (map[string]desiredLoopResource, error) {
	desired := make(map[string]desiredLoopResource)
	for _, provider := range s.providers {
		if provider == nil {
			continue
		}
		items, err := provider(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			spec, encoded, err := validateAndEncodeLoop(ctx, s.codec, item.scope, item.spec)
			if err != nil {
				return nil, err
			}
			id := managedResourceID(loopManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded)
			desired[id] = desiredLoopResource{
				id:      id,
				scope:   item.scope.Normalize(),
				spec:    spec,
				encoded: encoded,
			}
		}
	}
	return desired, nil
}

func (s *loopSourceSyncer) syncLoops(
	ctx context.Context,
	desired map[string]desiredLoopResource,
) (bool, error) {
	source := s.actor.Source
	current, err := s.store.List(ctx, s.actor, resources.ResourceFilter{
		Kind:   looppkg.ResourceKind,
		Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed loops: %w", err)
	}
	currentByID := make(map[string]resources.Record[looppkg.ResourceSpec], len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredLoop := range desired {
		existing, ok := currentByID[id]
		if ok && s.sameLoop(existing, desiredLoop.scope, desiredLoop.encoded) {
			delete(currentByID, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.store.Put(ctx, s.actor, resources.Draft[looppkg.ResourceSpec]{
			ID:              desiredLoop.id,
			Scope:           desiredLoop.scope,
			ExpectedVersion: expectedVersion,
			Spec:            desiredLoop.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync loop %q: %w", id, err)
		}
		changed = true
		delete(currentByID, id)
	}
	for _, stale := range currentByID {
		if err := s.store.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale loop %q: %w", stale.ID, err)
		}
		changed = true
	}
	return changed, nil
}

func (s *loopSourceSyncer) sameLoop(
	record resources.Record[looppkg.ResourceSpec],
	scope resources.ResourceScope,
	encoded []byte,
) bool {
	if record.Scope != scope {
		return false
	}
	currentEncoded, err := s.codec.Encode(record.Spec)
	if err != nil {
		return false
	}
	return bytes.Equal(currentEncoded, encoded)
}

func daemonLoopDeclarationProvider(
	homePaths aghconfig.HomePaths,
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	logger *slog.Logger,
) loopDeclarationProvider {
	return func(ctx context.Context) ([]loopPublicationInput, error) {
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		var desired []loopPublicationInput

		global, err := scanLoopResourceDir(ctx, homePaths.LoopsDir, looppkg.SourceUser)
		if err != nil {
			return nil, fmt.Errorf("daemon: discover global loops: %w", err)
		}
		appendLoopResources(&desired, globalScope, "loops/global", global)

		workspaces, err := registeredWorkspaces(ctx, registry, workspaceResolver, logger)
		if err != nil {
			return nil, err
		}
		for idx := range workspaces {
			resolved := &workspaces[idx]
			scope := resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspace,
				ID:   strings.TrimSpace(resolved.ID),
			}
			for _, root := range aghconfig.WorkspaceDiscoveryRoots(
				resolved.RootDir,
				resolved.AdditionalDirs,
				homePaths,
			) {
				if root.Source == aghconfig.WorkspaceDiscoverySourceGlobal {
					continue
				}
				source := loopSourceForDiscoveryRoot(root.Source)
				loops, err := scanLoopResourceDir(ctx, root.LoopsDir(), source)
				if err != nil {
					return nil, fmt.Errorf("daemon: discover workspace %q loops: %w", scope.ID, err)
				}
				sourceKey := "loops/" + string(root.Source) + "/" + scope.ID + "/" + root.LoopsDir()
				appendLoopResources(&desired, scope, sourceKey, loops)
			}
		}
		return desired, nil
	}
}

func extensionLoopDeclarationProvider(
	registry *extensionpkg.Registry,
	runtime func() extensionRuntime,
	logger *slog.Logger,
) loopDeclarationProvider {
	return func(ctx context.Context) ([]loopPublicationInput, error) {
		if registry == nil || runtime == nil {
			return nil, nil
		}
		manager := runtime()
		if manager == nil {
			return nil, nil
		}
		infos, err := registry.List()
		if err != nil {
			return nil, fmt.Errorf("daemon: list extensions for loop sync: %w", err)
		}
		slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
			return strings.Compare(left.Name, right.Name)
		})

		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		var desired []loopPublicationInput
		for _, info := range infos {
			if !info.Enabled {
				continue
			}
			ext, err := loadExtensionSnapshot(ctx, registry, manager, logger, info.Name)
			if err != nil {
				return nil, fmt.Errorf("daemon: load extension %q for loop sync: %w", info.Name, err)
			}
			if ext == nil || ext.Manifest == nil || !ext.Status.Registered {
				continue
			}
			for _, spec := range ext.Loops {
				desired = append(desired, loopPublicationInput{
					sourceKey: "extension/" + ext.Info.Name + "/loops/" + strings.TrimSpace(spec.Name),
					scope:     globalScope,
					spec:      looppkg.CloneResourceSpec(spec),
				})
			}
		}
		return desired, nil
	}
}
