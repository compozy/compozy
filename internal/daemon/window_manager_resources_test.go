package daemon

// Suite: daemon window-layout resources
// Invariant: projected resources preserve known-good state and expose only global plus matching-workspace layouts.
// Boundary IN: typed resource records, projector, and window-manager layout registry adapter.
// Boundary OUT: raw resource persistence, transport serialization, and window-manager command reduction.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/windowmanager"
)

func TestWindowManagerLayoutResources(t *testing.T) {
	t.Run("Should prefer workspace layouts, bind documents, sort results, and isolate clones", func(t *testing.T) {
		t.Parallel()
		catalog := newResourceCatalog(windowmanager.CloneLayoutResource)
		catalog.Replace(4, []resources.Record[windowmanager.LayoutResource]{
			windowLayoutRecord(
				"focus",
				resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				"Global Focus",
			),
			windowLayoutRecord(
				"alpha",
				resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
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

	// Invariant: restored resources repair retired windows once without changing their source or surviving layout.
	// Owning layer: daemon persisted resource load; canonical suite: this file.
	t.Run("Should persist reconciled window resources once and preserve source ownership", func(t *testing.T) {
		t.Parallel()
		database := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(database.DB())
		if err != nil {
			t.Fatal(err)
		}
		codecs := resources.NewCodecRegistry()
		if err := registerDaemonResourceCodecs(codecs); err != nil {
			t.Fatal(err)
		}
		state := &bootState{resourceKernel: kernel, resourceCodecs: codecs, logger: discardLogger()}
		_, store, err := state.resolveDaemonResourceStore[windowmanager.LayoutResource](
			windowmanager.WindowLayoutResourceKind,
			"window layout",
		)
		if err != nil {
			t.Fatal(err)
		}
		actor := resources.MutationActor{
			Kind: resources.MutationActorKindOperator, ID: "operator",
			Source:   resources.ResourceSource{Kind: "dynamic", ID: "operator"},
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}
		fixture := windowLayoutRecord("restored", actor.MaxScope, "Restored")
		fixture.Spec.Document.Windows = map[windowmanager.WindowID]windowmanager.Window{
			"retired": {ID: "retired", App: "knowledge"},
			"kept":    {ID: "kept", App: "settings", Route: windowmanager.RouteIntent{Pathname: "/settings/memory"}},
		}
		for id, window := range fixture.Spec.Document.Windows {
			window.DesktopID = "desktop-default"
			window.Placement = windowmanager.WindowPlacementFloating
			window.FloatingRect = windowmanager.NormalizedRect{Width: 0.5, Height: 0.5}
			window.Route.Search = windowmanager.RouteSearch{}
			if window.Route.Pathname == "" {
				window.Route.Pathname = "/"
			}
			fixture.Spec.Document.Windows[id] = window
		}
		fixture.Spec.Document.Desktops[0].Floating = []windowmanager.WindowID{"retired", "kept"}
		fixture.Spec.ParticipantSlots = []windowmanager.WindowID{"retired", "kept", "virtual"}
		original, err := store.Put(
			t.Context(),
			actor,
			resources.Draft[windowmanager.LayoutResource]{ID: fixture.ID, Scope: fixture.Scope, Spec: fixture.Spec},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := reconcileStoredWindowLayouts(t.Context(), state); err != nil {
			t.Fatal(err)
		}
		got, err := store.Get(t.Context(), actor, fixture.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != original.Version+1 || got.Source != original.Source || got.Owner != original.Owner ||
			len(got.Spec.Document.Windows) != 1 || got.Spec.Document.Windows["kept"].Route.Pathname != "/settings" ||
			!reflect.DeepEqual(got.Spec.ParticipantSlots, []windowmanager.WindowID{"kept", "virtual"}) ||
			!reflect.DeepEqual(got.Spec.Document.Desktops[0].Floating, []windowmanager.WindowID{"kept"}) {
			t.Fatalf("reconciled resource = %+v", got)
		}
		if err := reconcileStoredWindowLayouts(t.Context(), state); err != nil {
			t.Fatal(err)
		}
		again, err := store.Get(t.Context(), actor, fixture.ID)
		if err != nil || !reflect.DeepEqual(again, got) {
			t.Fatalf("second reconciliation = %+v, error = %v; want unchanged %+v", again, err, got)
		}
		catalog := newResourceCatalog(windowmanager.CloneLayoutResource)
		catalog.Replace(original.Version, []resources.Record[windowmanager.LayoutResource]{original})
		resolved, err := newWindowManagerLayoutRegistry(catalog).Resolve(t.Context(), "workspace", fixture.ID)
		if err != nil || len(resolved.Windows) != 1 || resolved.Windows["kept"].Route.Pathname != "/settings" {
			t.Fatalf("Resolve(legacy) = %+v, error = %v", resolved, err)
		}
	})

	t.Run("Should reject mismatched identities before replacing known-good catalog state", func(t *testing.T) {
		t.Parallel()
		catalog := newResourceCatalog(windowmanager.CloneLayoutResource)
		projector := newWindowLayoutProjector(catalog)
		good := windowLayoutRecord(
			"focus",
			resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
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
					Groups: []windowmanager.LayoutGroup{}, Floating: []windowmanager.WindowID{},
				}},
				Windows: map[windowmanager.WindowID]windowmanager.Window{},
			},
		},
	}
}
