package daemon

// Suite: daemon window-layout resources
// Invariant: projected resources preserve known-good state and expose only global plus matching-workspace layouts.
// Boundary IN: typed resource records, projector, and window-manager layout registry adapter.
// Boundary OUT: raw resource persistence, transport serialization, and window-manager command reduction.

import (
	"errors"
	"testing"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

func TestWindowManagerLayoutResources(t *testing.T) {
	t.Run("Should prefer workspace layouts, bind documents, sort results, and isolate clones", func(t *testing.T) {
		t.Parallel()
		catalog := newResourceCatalog(windowmanager.CloneLayoutResource)
		catalog.Replace(4, []resources.Record[windowmanager.LayoutResource]{
			windowLayoutRecord(
				"focus",
				resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				"Global Focus",
			),
			windowLayoutRecord(
				"alpha",
				resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				"Alpha",
			),
			windowLayoutRecord(
				"focus",
				resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-a"},
				"Workspace Focus",
			),
			windowLayoutRecord(
				"private",
				resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-b"},
				"Private",
			),
		})
		registry := newWindowManagerLayoutRegistry(catalog)
		visible, err := registry.List(t.Context(), "ws-a")
		if err != nil || len(visible) != 2 || visible[0].ID != "alpha" ||
			visible[1].DisplayName != "Workspace Focus" {
			t.Fatalf("List(ws-a) = %+v, error = %v", visible, err)
		}
		for _, resource := range visible {
			if resource.Document.WorkspaceID != "ws-a" {
				t.Fatalf("resource %q workspace = %q", resource.ID, resource.Document.WorkspaceID)
			}
		}
		visible[0].Document.Desktops[0].Name = "Tampered"
		resolved, err := registry.Resolve(t.Context(), "ws-c", "alpha")
		if err != nil || resolved.Desktops[0].Name != "Alpha" || resolved.WorkspaceID != "ws-c" {
			t.Fatalf("Resolve(global) = %+v, error = %v", resolved, err)
		}
		if _, err := registry.Resolve(t.Context(), "ws-a", "missing"); !errors.Is(
			err,
			windowmanager.ErrLayoutResourceNotFound,
		) {
			t.Fatalf("Resolve(missing) error = %v", err)
		}
	})

	t.Run("Should reject mismatched identities before replacing known-good catalog state", func(t *testing.T) {
		t.Parallel()
		catalog := newResourceCatalog(windowmanager.CloneLayoutResource)
		projector := newWindowLayoutProjector(catalog)
		good := windowLayoutRecord(
			"focus",
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			"Focus",
		)
		plan, err := projector.Build(t.Context(), []resources.Record[windowmanager.LayoutResource]{good})
		if err != nil {
			t.Fatalf("Build(good) error = %v", err)
		}
		if err := projector.Apply(t.Context(), plan); err != nil {
			t.Fatalf("Apply(good) error = %v", err)
		}
		bad := good
		bad.ID = "wrong"
		if _, err := projector.Build(
			t.Context(),
			[]resources.Record[windowmanager.LayoutResource]{bad},
		); !errors.Is(err, resources.ErrValidation) {
			t.Fatalf("Build(mismatched) error = %v", err)
		}
		if records := catalog.Snapshot(); len(records) != 1 || records[0].ID != "focus" {
			t.Fatalf("catalog after rejected build = %+v", records)
		}
	})
}

func windowLayoutRecord(
	id string,
	scope resources.ResourceScope,
	desktopName string,
) resources.Record[windowmanager.LayoutResource] {
	return resources.Record[windowmanager.LayoutResource]{
		Kind:    windowmanager.WindowLayoutResourceKind,
		ID:      id,
		Scope:   scope,
		Version: 1,
		Spec: windowmanager.LayoutResource{
			Version: windowmanager.LayoutResourceVersion,
			ID:      id, DisplayName: desktopName,
			AspectVariant:  windowmanager.LayoutAspectAny,
			OverflowPolicy: windowmanager.LayoutOverflowStack,
			Document: windowmanager.LayoutDocument{
				Version: windowmanager.SnapshotVersion,
				Desktops: []windowmanager.Desktop{{
					ID: "desktop-default", Name: desktopName,
					Purpose: windowmanager.DesktopPurposeStandard,
					Groups:  []windowmanager.LayoutGroup{}, Floating: []windowmanager.WindowID{},
				}},
				Windows: map[windowmanager.WindowID]windowmanager.Window{},
			},
		},
	}
}
