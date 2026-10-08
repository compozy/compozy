package windowmanager

// Suite: declarative window-layout resource codec.
// Invariant: only strict v1 resource documents with valid v2 topology become discoverable.
// Boundary IN: resource JSON and global/workspace scope.
// Boundary OUT: resource persistence/reconcile, owned by internal/resources and daemon.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/resources"
)

func TestLayoutResourceCodec(t *testing.T) {
	// Invariant: a previous-version public layout preserves all windows while aliasing retired apps; codec suite owns resource admission.
	t.Run("Should rewrite retired resource apps and navigation [IT-014]", func(t *testing.T) {
		t.Parallel()
		var warnings []string
		codec, err := NewLayoutResourceCodec(func(_ context.Context, app, replacement, source string) {
			warnings = append(warnings, app+":"+replacement+":"+source)
		})
		if err != nil {
			t.Fatal(err)
		}
		resource := testLayoutResource("legacy")
		resource.Document.Version = PreviousSnapshotVersion
		for id, window := range resource.Document.Windows {
			window.App = "jobs"
			window.Route = testRoute("/jobs/morning-digest")
			resource.Document.Windows[id] = window
		}
		raw, err := codec.Encode(resource)
		if err != nil {
			t.Fatal(err)
		}
		got, err := codec.DecodeAndValidate(
			t.Context(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			raw,
		)
		if err != nil {
			t.Fatal(err)
		}
		if got.Document.Version != SnapshotVersion || len(got.Document.Windows) != len(resource.Document.Windows) ||
			len(warnings) != len(resource.Document.Windows) {
			t.Fatalf("resource=%+v warnings=%v", got, warnings)
		}
		for _, window := range got.Document.Windows {
			if window.App != "automations" || window.Route.Pathname != "/automations/jobs/morning-digest" {
				t.Fatalf("window=%+v", window)
			}
		}
		for _, warning := range warnings {
			if warning != "jobs:automations:resource" {
				t.Fatal(warning)
			}
		}
	})
	t.Parallel()
	codec, err := NewLayoutResourceCodec()
	if err != nil {
		t.Fatalf("NewLayoutResourceCodec() error = %v", err)
	}

	t.Run("Should validate and canonicalize a workspace-independent resource", func(t *testing.T) {
		t.Parallel()
		resource := testLayoutResource("two-up")
		resource.ParticipantSlots = []WindowID{" primary ", "secondary"}
		encoded, err := codec.Encode(resource)
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		got, err := codec.DecodeAndValidate(
			t.Context(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			encoded,
		)
		if err != nil {
			t.Fatalf("DecodeAndValidate() error = %v", err)
		}
		if got.ID != "two-up" || got.AspectVariant != LayoutAspectAny ||
			got.OverflowPolicy != LayoutOverflowStack || got.Document.WorkspaceID != "" ||
			len(got.ParticipantSlots) != 2 || got.ParticipantSlots[0] != "primary" {
			t.Fatalf("DecodeAndValidate() = %#v, want canonical user-scoped resource", got)
		}
	})

	t.Run("Should reject participant slots duplicated after canonicalization", func(t *testing.T) {
		t.Parallel()
		resource := testLayoutResource("two-up")
		resource.ParticipantSlots = []WindowID{"primary", " primary "}
		encoded, err := codec.Encode(resource)
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		_, err = codec.DecodeAndValidate(
			t.Context(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			encoded,
		)
		if !errors.Is(err, resources.ErrValidation) {
			t.Fatalf("DecodeAndValidate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reject unknown fields instead of accepting a shadow schema", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"version":1,"id":"two-up","display_name":"Two up",` +
			`"aspect_variant":"any","overflow_policy":"stack","participant_slots":[],` +
			`"document":{"version":2,"workspace_id":"","desktops":[],"windows":{},"overrides":{}},` +
			`"legacy":true}`)
		_, err := codec.DecodeAndValidate(
			t.Context(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			raw,
		)
		if !errors.Is(err, resources.ErrValidation) || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("DecodeAndValidate() error = %v, want strict unknown-field rejection", err)
		}
	})

	t.Run("Should reject a workspace binding outside its scope", func(t *testing.T) {
		t.Parallel()
		resource := testLayoutResource("two-up")
		resource.Document.WorkspaceID = "workspace-b"
		encoded, err := codec.Encode(resource)
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		_, err = codec.DecodeAndValidate(
			t.Context(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"},
			encoded,
		)
		if !errors.Is(err, resources.ErrInvalidScopeBinding) {
			t.Fatalf("DecodeAndValidate() error = %v, want ErrInvalidScopeBinding", err)
		}
	})
}

func testLayoutResource(id string) LayoutResource {
	return LayoutResource{
		Version: LayoutResourceVersion, ID: id, DisplayName: "Two up",
		Document: LayoutDocument{
			Version: SnapshotVersion,
			Desktops: []Desktop{{
				ID: "desktop-default", Name: "Desktop 1",
				Groups: []LayoutGroup{}, Floating: []WindowID{},
			}},
			Windows: map[WindowID]Window{},
		},
	}
}
