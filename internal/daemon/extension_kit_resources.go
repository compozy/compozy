package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/windowmanager"
)

type extensionKitResourcePublisher interface {
	Sync(context.Context) error
}

type extensionKitSourceSyncer struct {
	jobs         resources.Store[automationpkg.Job]
	jobCodec     resources.KindCodec[automationpkg.Job]
	triggers     resources.Store[automationpkg.Trigger]
	triggerCodec resources.KindCodec[automationpkg.Trigger]
	layouts      resources.Store[windowmanager.LayoutResource]
	layoutCodec  resources.KindCodec[windowmanager.LayoutResource]
	actor        resources.MutationActor
	registry     *extensionpkg.Registry
	runtime      func() extensionRuntime
	logger       *slog.Logger
	trigger      func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
}

func (d *Daemon) newExtensionKitResourcePublisher(
	state *bootState,
	registry *extensionpkg.Registry,
) (extensionKitResourcePublisher, error) {
	if state == nil || state.resourceKernel == nil || state.resourceCodecs == nil {
		return nil, nil
	}
	jobCodec, jobStore, err := resolveDaemonResourceStore[automationpkg.Job](
		state, state.resourceKernel, automationpkg.JobResourceKind, "extension automation job",
	)
	if err != nil {
		return nil, err
	}
	triggerCodec, triggerStore, err := resolveDaemonResourceStore[automationpkg.Trigger](
		state, state.resourceKernel, automationpkg.TriggerResourceKind, "extension automation trigger",
	)
	if err != nil {
		return nil, err
	}
	layoutCodec, layoutStore, err := resolveDaemonResourceStore[windowmanager.LayoutResource](
		state, state.resourceKernel, windowmanager.WindowLayoutResourceKind, "extension window layout",
	)
	if err != nil {
		return nil, err
	}
	return &extensionKitSourceSyncer{
		jobs: jobStore, jobCodec: jobCodec,
		triggers: triggerStore, triggerCodec: triggerCodec,
		layouts: layoutStore, layoutCodec: layoutCodec,
		actor: extensionKitSyncActor(), registry: registry, runtime: state.currentExtensionRuntime,
		logger: state.logger,
		trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
	}, nil
}

func extensionKitSyncActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "extension-kit-sync",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "extension-kit-sync",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (s *extensionKitSourceSyncer) Sync(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: extension kit sync context is required")
	}
	jobs, triggers, layouts, err := s.desired(ctx)
	if err != nil {
		return err
	}
	jobChanged, err := syncManagedResources(
		ctx, s.actor, s.jobs, s.jobCodec, jobs,
		func(value managedResourceValue[automationpkg.Job]) managedResourceValue[automationpkg.Job] {
			return value
		},
		"extension automation job",
	)
	if err != nil {
		return err
	}
	triggerChanged, err := syncManagedResources(
		ctx, s.actor, s.triggers, s.triggerCodec, triggers,
		func(value managedResourceValue[automationpkg.Trigger]) managedResourceValue[automationpkg.Trigger] {
			return value
		},
		"extension automation trigger",
	)
	if err != nil {
		return err
	}
	layoutChanged, err := syncManagedResources(
		ctx,
		s.actor,
		s.layouts,
		s.layoutCodec,
		layouts,
		func(value managedResourceValue[windowmanager.LayoutResource]) managedResourceValue[windowmanager.LayoutResource] {
			return value
		},
		"extension window layout",
	)
	if err != nil {
		return err
	}
	for _, changed := range []struct {
		value bool
		kind  resources.ResourceKind
	}{
		{value: jobChanged, kind: automationpkg.JobResourceKind},
		{value: triggerChanged, kind: automationpkg.TriggerResourceKind},
		{value: layoutChanged, kind: windowmanager.WindowLayoutResourceKind},
	} {
		if changed.value && s.trigger != nil {
			if err := s.trigger(ctx, changed.kind, resources.ReconcileReasonWrite); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *extensionKitSourceSyncer) desired(
	ctx context.Context,
) (
	map[string]managedResourceValue[automationpkg.Job],
	map[string]managedResourceValue[automationpkg.Trigger],
	map[string]managedResourceValue[windowmanager.LayoutResource],
	error,
) {
	jobs := make(map[string]managedResourceValue[automationpkg.Job])
	triggers := make(map[string]managedResourceValue[automationpkg.Trigger])
	layouts := make(map[string]managedResourceValue[windowmanager.LayoutResource])
	if s.registry == nil || s.runtime == nil || s.runtime() == nil {
		return jobs, triggers, layouts, nil
	}
	infos, err := s.registry.List()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("daemon: list extensions for kit sync: %w", err)
	}
	slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
		return strings.Compare(left.Name, right.Name)
	})
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	for _, info := range infos {
		if !info.Enabled {
			continue
		}
		ext, err := loadExtensionSnapshot(s.registry, s.runtime(), s.logger, info.Name)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("daemon: load extension %q for kit sync: %w", info.Name, err)
		}
		if ext == nil || ext.Manifest == nil || !ext.Status.Registered {
			continue
		}
		owner := extensionOwner(ext.Info.Name)
		for _, job := range ext.AutomationJobs {
			value, encoded, err := validateAndEncodeResource(ctx, s.jobCodec, globalScope, job)
			if err != nil {
				return nil, nil, nil, fmt.Errorf(
					"daemon: validate extension %q job %q: %w",
					ext.Info.Name,
					job.Name,
					err,
				)
			}
			id := "extension/" + ext.Info.Name + "/automation.job/" + strings.TrimPrefix(job.Name, ext.Info.Name+"/")
			jobs[id] = managedResourceValue[automationpkg.Job]{
				id: id, scope: globalScope, owner: owner, spec: value, encoded: encoded,
			}
		}
		for _, trigger := range ext.AutomationTriggers {
			value, encoded, err := validateAndEncodeResource(ctx, s.triggerCodec, globalScope, trigger)
			if err != nil {
				return nil, nil, nil, fmt.Errorf(
					"daemon: validate extension %q trigger %q: %w", ext.Info.Name, trigger.Name, err,
				)
			}
			id := "extension/" + ext.Info.Name + "/automation.trigger/" +
				strings.TrimPrefix(trigger.Name, ext.Info.Name+"/")
			triggers[id] = managedResourceValue[automationpkg.Trigger]{
				id: id, scope: globalScope, owner: owner, spec: value, encoded: encoded,
			}
		}
		for _, layout := range ext.Layouts {
			id := "extension/" + ext.Info.Name + "/window_layout/" + strings.TrimSpace(layout.ID)
			materialized := windowmanager.CloneLayoutResource(layout)
			materialized.ID = id
			value, encoded, err := validateAndEncodeResource(ctx, s.layoutCodec, globalScope, materialized)
			if err != nil {
				return nil, nil, nil, fmt.Errorf(
					"daemon: validate extension %q layout %q: %w", ext.Info.Name, layout.ID, err,
				)
			}
			layouts[id] = managedResourceValue[windowmanager.LayoutResource]{
				id: id, scope: globalScope, owner: owner, spec: value, encoded: encoded,
			}
		}
	}
	return jobs, triggers, layouts, nil
}
