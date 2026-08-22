package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	profiles     extensionProfileCatalog
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
		state, automationpkg.JobResourceKind, "extension automation job",
	)
	if err != nil {
		return nil, err
	}
	triggerCodec, triggerStore, err := resolveDaemonResourceStore[automationpkg.Trigger](
		state, automationpkg.TriggerResourceKind, "extension automation trigger",
	)
	if err != nil {
		return nil, err
	}
	layoutCodec, layoutStore, err := resolveDaemonResourceStore[windowmanager.LayoutResource](
		state, windowmanager.WindowLayoutResourceKind, "extension window layout",
	)
	if err != nil {
		return nil, err
	}
	return &extensionKitSourceSyncer{
		jobs: jobStore, jobCodec: jobCodec,
		triggers: triggerStore, triggerCodec: triggerCodec,
		layouts: layoutStore, layoutCodec: layoutCodec,
		actor: extensionKitSyncActor(), registry: registry, runtime: state.currentExtensionRuntime,
		profiles: state.deps.Profiles,
		logger:   state.logger,
		trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			_, err := state.resourceReconcile.Trigger(ctx, kind, reason)
			return err
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
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
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
		"extension automation job",
	)
	if err != nil {
		return err
	}
	triggerChanged, err := syncManagedResources(
		ctx, s.actor, s.triggers, s.triggerCodec, triggers,
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
	if s.registry == nil || s.runtime == nil {
		return jobs, triggers, layouts, nil
	}
	manager := s.runtime()
	if manager == nil {
		return jobs, triggers, layouts, nil
	}
	var snapshots []scopedExtensionResourceSnapshot
	var err error
	if s.profiles != nil {
		snapshots, err = extensionResourceSnapshotsForProfiles(
			ctx, s.registry, manager, s.logger, s.profiles,
		)
	} else {
		snapshots, err = extensionResourceSnapshots(s.registry, manager, s.logger)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	for _, snapshot := range snapshots {
		ext := snapshot.extension
		if ext == nil || ext.Manifest == nil || !ext.Status.Registered {
			continue
		}
		if err := s.collectDesiredExtensionKitResources(
			ctx,
			ext,
			snapshot.scope,
			jobs,
			triggers,
			layouts,
		); err != nil {
			return nil, nil, nil, err
		}
	}
	return jobs, triggers, layouts, nil
}

func (s *extensionKitSourceSyncer) collectDesiredExtensionKitResources(
	ctx context.Context,
	ext *extensionpkg.Extension,
	scope resources.ResourceScope,
	jobs map[string]managedResourceValue[automationpkg.Job],
	triggers map[string]managedResourceValue[automationpkg.Trigger],
	layouts map[string]managedResourceValue[windowmanager.LayoutResource],
) error {
	owner := extensionOwner(ext.Info.Name)
	for _, job := range ext.AutomationJobs {
		job = bindExtensionJobScope(job, scope)
		value, encoded, err := validateAndEncodeResource(ctx, s.jobCodec, scope, job)
		if err != nil {
			return fmt.Errorf(
				"daemon: validate extension %q job %q: %w",
				ext.Info.Name,
				job.Name,
				err,
			)
		}
		resourceID := value.ID
		if strings.TrimSpace(resourceID) == "" {
			return fmt.Errorf(
				"daemon: extension %q job %q has an empty resource ID",
				ext.Info.Name,
				job.Name,
			)
		}
		id := managedPublicationID("", scope, resourceID, encoded, owner)
		jobs[id] = managedResourceValue[automationpkg.Job]{
			id: id, scope: scope, owner: owner, spec: value, encoded: encoded,
		}
	}
	for _, trigger := range ext.AutomationTriggers {
		trigger = bindExtensionTriggerScope(trigger, scope)
		value, encoded, err := validateAndEncodeResource(ctx, s.triggerCodec, scope, trigger)
		if err != nil {
			return fmt.Errorf(
				"daemon: validate extension %q trigger %q: %w",
				ext.Info.Name,
				trigger.Name,
				err,
			)
		}
		resourceID := value.ID
		if strings.TrimSpace(resourceID) == "" {
			return fmt.Errorf(
				"daemon: extension %q trigger %q has an empty resource ID",
				ext.Info.Name,
				trigger.Name,
			)
		}
		id := managedPublicationID("", scope, resourceID, encoded, owner)
		triggers[id] = managedResourceValue[automationpkg.Trigger]{
			id: id, scope: scope, owner: owner, spec: value, encoded: encoded,
		}
	}
	for _, layout := range ext.Layouts {
		id := "extension/" + ext.Info.Name + "/window_layout/" + strings.TrimSpace(layout.ID)
		publicationID := managedPublicationID("", scope, id, nil, owner)
		materialized := windowmanager.CloneLayoutResource(layout)
		materialized.ID = publicationID
		value, encoded, err := validateAndEncodeResource(ctx, s.layoutCodec, scope, materialized)
		if err != nil {
			return fmt.Errorf(
				"daemon: validate extension %q layout %q: %w",
				ext.Info.Name,
				layout.ID,
				err,
			)
		}
		layouts[publicationID] = managedResourceValue[windowmanager.LayoutResource]{
			id: publicationID, scope: scope, owner: owner, spec: value, encoded: encoded,
		}
	}
	return nil
}

func bindExtensionJobScope(job automationpkg.Job, scope resources.ResourceScope) automationpkg.Job {
	if normalized := scope.Normalize(); normalized.Kind == resources.ResourceScopeKindWorkspace {
		job.Scope = automationpkg.AutomationScopeWorkspace
		job.WorkspaceID = normalized.ID
	}
	return job
}

func bindExtensionTriggerScope(trigger automationpkg.Trigger, scope resources.ResourceScope) automationpkg.Trigger {
	if normalized := scope.Normalize(); normalized.Kind == resources.ResourceScopeKindWorkspace {
		trigger.Scope = automationpkg.AutomationScopeWorkspace
		trigger.WorkspaceID = normalized.ID
	}
	return trigger
}
