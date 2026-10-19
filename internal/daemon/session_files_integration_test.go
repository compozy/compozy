//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/worktree"
)

// IT-005: searchSessionFiles on a real worktree-bound session lists the bound worktree, not the
// workspace root, identically over HTTP and UDS.
func TestDaemonSearchSessionFilesWorktreeBoundIT005(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	root := initWorktreeE2ERepository(t)
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatalf("mkdir cmd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write cmd/main.go: %v", err)
	}
	runWorktreeE2EGit(t, t.Context(), root, "add", "cmd/main.go")
	runWorktreeE2EGit(t, t.Context(), root, "commit", "-m", "add main")

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: root},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    "local-default",
			DefaultProvider: acpmock.ProviderName,
			PermissionMode:  config.PermissionModeApproveAll,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "local_fixture.json"),
			FixtureAgent: "local-default",
			AgentName:    "local-default",
		}},
	})
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	t.Cleanup(cancel)

	created := createWorktreeE2E(t, ctx, harness, harness.WorkspaceID, "mention-files")
	ready := waitForWorktreeE2EState(t, ctx, harness, harness.WorkspaceID, created.ID, worktree.StateReady)
	if err := os.WriteFile(filepath.Join(root, "root_only.go"), []byte("package root\n"), 0o600); err != nil {
		t.Fatalf("write workspace-root file: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(ready.Worktree.Path, "worktree_only.go"), []byte("package wt\n"), 0o600,
	); err != nil {
		t.Fatalf("write worktree file: %v", err)
	}
	bound, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName: "local-default", Name: "mention-files",
		Workspace: harness.WorkspaceID, Worktree: created.ID,
	})
	if err != nil {
		t.Fatalf("CreateSession(bound worktree) error = %v", err)
	}
	if bound.WorktreeID != created.ID {
		t.Fatalf("bound session worktree = %q, want %q", bound.WorktreeID, created.ID)
	}

	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/sessions/" +
		url.PathEscape(bound.ID) + "/files?q=" + url.QueryEscape("only") + "&limit=5"
	var httpPayload, udsPayload compozycontract.SessionFilesResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpPayload); err != nil {
		t.Fatalf("HTTP searchSessionFiles error = %v", err)
	}
	if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &udsPayload); err != nil {
		t.Fatalf("UDS searchSessionFiles error = %v", err)
	}
	assertWorktreeE2EJSONParity(t, "searchSessionFiles", udsPayload, httpPayload)
	want := compozycontract.SessionFileEntryPayload{
		Path: "worktree_only.go", Name: "worktree_only.go", Kind: compozycontract.SessionFileKindFile,
	}
	if len(httpPayload.Entries) != 1 || httpPayload.Entries[0] != want {
		t.Fatalf("entries = %#v, want only the bound worktree file %#v", httpPayload.Entries, want)
	}

	var mainPayload compozycontract.SessionFilesResponse
	mainPath := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/sessions/" +
		url.PathEscape(bound.ID) + "/files?q=main&limit=5"
	if err := harness.HTTPJSON(ctx, http.MethodGet, mainPath, nil, &mainPayload); err != nil {
		t.Fatalf("HTTP searchSessionFiles(main) error = %v", err)
	}
	if len(mainPayload.Entries) == 0 || mainPayload.Entries[0].Path != "cmd/main.go" {
		t.Fatalf("entries = %#v, want committed cmd/main.go from the worktree first", mainPayload.Entries)
	}
}
