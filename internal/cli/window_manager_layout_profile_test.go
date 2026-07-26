package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

func layoutProfileRecord(id string, scope resources.ResourceScope, version int64) contract.ResourceRecordPayload {
	stamp := time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC)
	return contract.ResourceRecordPayload{
		Kind:      windowmanager.WindowLayoutResourceKind,
		ID:        id,
		Version:   version,
		Scope:     scope,
		Owner:     resources.ResourceOwner{Kind: "daemon", ID: "test"},
		Source:    resources.ResourceSource{Kind: "operator", ID: "test"},
		Spec:      json.RawMessage(`{"version":1,"id":"` + id + `","display_name":"Two-up review"}`),
		CreatedAt: stamp,
		UpdatedAt: stamp,
	}
}

func TestLayoutProfileCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list the layouts a workspace can see, global scope included", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		client.listProfilesFn = func(_ context.Context, workspace string) (contract.ResourcesResponse, error) {
			if workspace != "w1" {
				t.Fatalf("list workspace = %q, want w1", workspace)
			}
			return contract.ResourcesResponse{Records: []contract.ResourceRecordPayload{
				layoutProfileRecord("two-up-review", resources.ResourceScope{
					Kind: resources.ResourceScopeKindWorkspace, ID: "w1",
				}, 4),
				layoutProfileRecord("focus-session", resources.ResourceScope{
					Kind: resources.ResourceScopeKindGlobal,
				}, 2),
			}}, nil
		}
		deps := newTestDeps(t, client)

		output, _, err := executeRootCommand(
			t, deps, "layout-profile", "list", "--workspace", "w1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("layout-profile list error = %v", err)
		}
		var listed contract.ResourcesResponse
		decodeWindowManagerCommandJSON(t, output, &listed)
		if len(listed.Records) != 2 {
			t.Fatalf("records = %d, want 2", len(listed.Records))
		}
		if listed.Records[0].ID != "two-up-review" || listed.Records[1].ID != "focus-session" {
			t.Fatalf("records = %#v, want the daemon's order preserved", listed.Records)
		}
	})

	t.Run("Should never surface a layout scoped to another workspace", func(t *testing.T) {
		t.Parallel()
		// The daemon filters by scope before responding
		// (internal/api/core/window_manager_layout_profiles.go). The CLI must not
		// widen that: `get` reads the same visible list and nothing else.
		client := newWindowManagerCommandStub()
		client.listProfilesFn = func(_ context.Context, workspace string) (contract.ResourcesResponse, error) {
			if workspace != "w1" {
				t.Fatalf("list workspace = %q, want w1", workspace)
			}
			return contract.ResourcesResponse{Records: []contract.ResourceRecordPayload{
				layoutProfileRecord("two-up-review", resources.ResourceScope{
					Kind: resources.ResourceScopeKindWorkspace, ID: "w1",
				}, 4),
			}}, nil
		}
		deps := newTestDeps(t, client)

		_, _, err := executeRootCommand(
			t, deps, "layout-profile", "get", "workspace-b-profile", "--workspace", "w1", "-o", "json",
		)
		if err == nil {
			t.Fatal("layout-profile get error = nil, want a not-visible failure")
		}
		if !strings.Contains(err.Error(), "workspace-b-profile") {
			t.Fatalf("layout-profile get error = %v, want it to name the missing profile", err)
		}
	})

	t.Run("Should read one visible layout by id", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		client.listProfilesFn = func(_ context.Context, _ string) (contract.ResourcesResponse, error) {
			return contract.ResourcesResponse{Records: []contract.ResourceRecordPayload{
				layoutProfileRecord("two-up-review", resources.ResourceScope{
					Kind: resources.ResourceScopeKindWorkspace, ID: "w1",
				}, 4),
			}}, nil
		}
		deps := newTestDeps(t, client)

		output, _, err := executeRootCommand(
			t, deps, "layout-profile", "get", "two-up-review", "--workspace", "w1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("layout-profile get error = %v", err)
		}
		var response contract.ResourceResponse
		decodeWindowManagerCommandJSON(t, output, &response)
		if response.Record.ID != "two-up-review" || response.Record.Version != 4 {
			t.Fatalf("record = %#v, want two-up-review at version 4", response.Record)
		}
	})

	t.Run("Should send the scope and the version a write expects to replace", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.json")
		spec := `{"version":1,"id":"two-up-review","display_name":"Two-up review"}`
		if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
			t.Fatalf("write layout profile spec: %v", err)
		}
		client := newWindowManagerCommandStub()
		client.putProfileFn = func(
			_ context.Context,
			workspace string,
			profileID string,
			request contract.PutResourceRequest,
		) (contract.ResourceResponse, error) {
			if workspace != "w1" || profileID != "two-up-review" {
				t.Fatalf("put target = %q/%q, want w1/two-up-review", workspace, profileID)
			}
			if request.Scope.Kind != resources.ResourceScopeKindGlobal || request.Scope.ID != "" {
				t.Fatalf("scope = %#v, want global", request.Scope)
			}
			if request.ExpectedVersion != 4 {
				t.Fatalf("expected version = %d, want 4", request.ExpectedVersion)
			}
			return contract.ResourceResponse{Record: layoutProfileRecord(profileID, request.Scope, 5)}, nil
		}
		deps := newTestDeps(t, client)

		output, _, err := executeRootCommand(
			t, deps, "layout-profile", "put", "two-up-review",
			"--workspace", "w1", "--scope", "global",
			"--file", path, "--expected-version", "4", "-o", "json",
		)
		if err != nil {
			t.Fatalf("layout-profile put error = %v", err)
		}
		var response contract.ResourceResponse
		decodeWindowManagerCommandJSON(t, output, &response)
		if response.Record.Version != 5 {
			t.Fatalf("record version = %d, want 5", response.Record.Version)
		}
	})

	t.Run("Should default a write to the workspace it was given", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, []byte(`{"version":1}`), 0o600); err != nil {
			t.Fatalf("write layout profile spec: %v", err)
		}
		client := newWindowManagerCommandStub()
		client.putProfileFn = func(
			_ context.Context,
			_ string,
			profileID string,
			request contract.PutResourceRequest,
		) (contract.ResourceResponse, error) {
			if request.Scope.Kind != resources.ResourceScopeKindWorkspace || request.Scope.ID != "w1" {
				t.Fatalf("scope = %#v, want workspace w1", request.Scope)
			}
			return contract.ResourceResponse{Record: layoutProfileRecord(profileID, request.Scope, 1)}, nil
		}
		deps := newTestDeps(t, client)

		if _, _, err := executeRootCommand(
			t, deps, "layout-profile", "put", "two-up-review",
			"--workspace", "w1", "--file", path, "-o", "json",
		); err != nil {
			t.Fatalf("layout-profile put error = %v", err)
		}
	})

	t.Run("Should refuse a scope the resource model does not have", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, []byte(`{"version":1}`), 0o600); err != nil {
			t.Fatalf("write layout profile spec: %v", err)
		}
		deps := newTestDeps(t, newWindowManagerCommandStub())

		_, _, err := executeRootCommand(
			t, deps, "layout-profile", "put", "two-up-review",
			"--workspace", "w1", "--scope", "session", "--file", path, "-o", "json",
		)
		if err == nil {
			t.Fatal("layout-profile put error = nil, want an unsupported-scope failure")
		}
	})

	t.Run("Should require the version a delete removes", func(t *testing.T) {
		t.Parallel()
		deps := newTestDeps(t, newWindowManagerCommandStub())

		_, _, err := executeRootCommand(
			t, deps, "layout-profile", "delete", "two-up-review", "--workspace", "w1", "-o", "json",
		)
		if err == nil {
			t.Fatal("layout-profile delete error = nil, want a required-flag failure")
		}
		if !strings.Contains(err.Error(), "expected-version") {
			t.Fatalf("layout-profile delete error = %v, want it to name --expected-version", err)
		}
	})

	t.Run("Should delete at the exact version it was given", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		client.deleteProfileFn = func(
			_ context.Context,
			workspace string,
			profileID string,
			request contract.DeleteResourceRequest,
		) error {
			if workspace != "w1" || profileID != "two-up-review" || request.ExpectedVersion != 4 {
				t.Fatalf(
					"delete target = %q/%q at version %d, want w1/two-up-review at 4",
					workspace, profileID, request.ExpectedVersion,
				)
			}
			return nil
		}
		deps := newTestDeps(t, client)

		output, _, err := executeRootCommand(
			t, deps, "layout-profile", "delete", "two-up-review",
			"--workspace", "w1", "--expected-version", "4", "-o", "json",
		)
		if err != nil {
			t.Fatalf("layout-profile delete error = %v", err)
		}
		var deleted struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspace_id"`
		}
		decodeWindowManagerCommandJSON(t, output, &deleted)
		if deleted.ID != "two-up-review" || deleted.WorkspaceID != "w1" {
			t.Fatalf("deleted = %#v, want two-up-review in w1", deleted)
		}
	})

	t.Run("Should require a workspace for every verb", func(t *testing.T) {
		t.Parallel()
		deps := newTestDeps(t, newWindowManagerCommandStub())

		if _, _, err := executeRootCommand(t, deps, "layout-profile", "list", "-o", "json"); err == nil {
			t.Fatal("layout-profile list error = nil, want a required-flag failure")
		}
	})
}
