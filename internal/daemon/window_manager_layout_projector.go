package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/cmdpalette/corecmds"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/windowmanager"
)

const windowLayoutReconcileActorID = "window-layout-reconcile"

type windowLayoutProjector struct {
	delegate *resourceCatalogProjector[windowmanager.LayoutResource]
}

func newWindowLayoutProjector(
	catalog *resourceCatalog[windowmanager.LayoutResource],
) resources.TypedProjector[windowmanager.LayoutResource] {
	if catalog == nil {
		return nil
	}
	return &windowLayoutProjector{delegate: &resourceCatalogProjector[windowmanager.LayoutResource]{
		kind: windowmanager.WindowLayoutResourceKind, catalog: catalog,
		cloneSpec: windowmanager.CloneLayoutResource,
	}}
}

func (p *windowLayoutProjector) Kind() resources.ResourceKind {
	return windowmanager.WindowLayoutResourceKind
}

func (p *windowLayoutProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *windowLayoutProjector) Build(
	ctx context.Context,
	records []resources.Record[windowmanager.LayoutResource],
) (resources.ProjectionPlan, error) {
	if p == nil || p.delegate == nil {
		return nil, errors.New("daemon: window layout projector is required")
	}
	for _, record := range records {
		if record.ID != record.Spec.ID {
			return nil, fmt.Errorf(
				"%w: window layout record ID %q does not match spec ID %q",
				resources.ErrValidation,
				record.ID,
				record.Spec.ID,
			)
		}
	}
	return p.delegate.Build(ctx, records)
}

func (p *windowLayoutProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.delegate == nil {
		return errors.New("daemon: window layout projector is required")
	}
	return p.delegate.Apply(ctx, plan)
}

func reconcileWindowLayoutResource(resource windowmanager.LayoutResource) (windowmanager.LayoutResource, bool) {
	resource = windowmanager.CloneLayoutResource(resource)
	document := resource.Document
	reconciled, changed := windowmanager.ReconcileRegisteredApps(windowmanager.Snapshot{
		Version: document.Version, WorkspaceID: document.WorkspaceID,
		Desktops: document.Desktops, Windows: document.Windows, Overrides: document.Overrides,
	}, corecmds.RegisteredApp)
	if !changed {
		return resource, false
	}
	resource.Document.Desktops = reconciled.Desktops
	resource.Document.Windows = reconciled.Windows
	resource.ParticipantSlots = slices.DeleteFunc(resource.ParticipantSlots, func(id windowmanager.WindowID) bool {
		_, existed := document.Windows[id]
		_, remains := reconciled.Windows[id]
		return existed && !remains
	})
	return resource, true
}

func reconcileStoredWindowLayouts(ctx context.Context, state *bootState) error {
	if state.resourceKernel == nil {
		return nil
	}
	codec, store, err := state.resolveDaemonResourceStore[windowmanager.LayoutResource](
		windowmanager.WindowLayoutResourceKind,
		"window layout",
	)
	if err != nil {
		return err
	}
	actor := resources.MutationActor{
		Kind: resources.MutationActorKindDaemon, ID: windowLayoutReconcileActorID,
		Owner:    resources.ResourceOwner{Kind: "daemon", ID: windowLayoutReconcileActorID},
		Source:   resources.ResourceSource{Kind: "dynamic", ID: windowLayoutReconcileActorID},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}
	records, err := state.resourceKernel.ListRaw(ctx, actor, resources.ResourceFilter{
		Kind: windowmanager.WindowLayoutResourceKind,
	})
	if err != nil {
		return err
	}
	for _, record := range records {
		resource, err := codec.DecodeAndValidate(ctx, record.Scope, record.SpecJSON)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			state.logger.Warn("window_manager.layout_reconcile_failed", "resource_id", record.ID, "error", err)
			continue
		}
		var stored struct {
			Document struct {
				Version uint32 `json:"version"`
			} `json:"document"`
		}
		if err := json.Unmarshal(record.SpecJSON, &stored); err != nil {
			return fmt.Errorf("daemon: decode stored window layout %q version: %w", record.ID, err)
		}
		reconciled, changed := reconcileWindowLayoutResource(resource)
		if !changed && stored.Document.Version == windowmanager.SnapshotVersion {
			continue
		}
		actor.Source = record.Source
		if _, err := store.Put(ctx, actor, resources.Draft[windowmanager.LayoutResource]{
			ID: record.ID, Scope: record.Scope, Owner: &record.Owner,
			ExpectedVersion: record.Version, Spec: reconciled,
		}); err != nil {
			return fmt.Errorf("daemon: reconcile window layout %q: %w", record.ID, err)
		}
		state.logger.Info("window_manager.layout_reconciled", "resource_id", record.ID)
	}
	return nil
}
