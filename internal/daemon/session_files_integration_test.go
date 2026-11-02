//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
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

	filesPath := func(query string) string {
		return "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/sessions/" +
			url.PathEscape(bound.ID) + "/files?q=" + url.QueryEscape(query) + "&limit=5"
	}

	t.Run("Should list the bound worktree instead of the workspace root over HTTP and UDS", func(t *testing.T) {
		var httpPayload, udsPayload compozycontract.SessionFilesResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, filesPath("only"), nil, &httpPayload); err != nil {
			t.Fatalf("HTTP searchSessionFiles error = %v", err)
		}
		if err := harness.UDSJSON(ctx, http.MethodGet, filesPath("only"), nil, &udsPayload); err != nil {
			t.Fatalf("UDS searchSessionFiles error = %v", err)
		}
		assertWorktreeE2EJSONParity(t, "searchSessionFiles", udsPayload, httpPayload)
		want := compozycontract.SessionFileEntryPayload{
			Path: "worktree_only.go", Name: "worktree_only.go", Kind: compozycontract.SessionFileKindFile,
		}
		if len(httpPayload.Entries) != 1 || httpPayload.Entries[0] != want {
			t.Fatalf("entries = %#v, want only the bound worktree file %#v", httpPayload.Entries, want)
		}
	})

	t.Run("Should rank committed files of the bound worktree", func(t *testing.T) {
		var payload compozycontract.SessionFilesResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, filesPath("main"), nil, &payload); err != nil {
			t.Fatalf("HTTP searchSessionFiles(main) error = %v", err)
		}
		if len(payload.Entries) == 0 || payload.Entries[0].Path != "cmd/main.go" {
			t.Fatalf("entries = %#v, want committed cmd/main.go from the worktree first", payload.Entries)
		}
	})
}

// IT-010: a session whose creation profile pins CWD to pkg/ (the loop-created shape; the public
// create route takes no cwd, so the in-process manager creates it) searches and resolves
// mentions against pkg/ through the booted daemon's real UDS route.
func TestDaemonSearchSessionFilesCreationProfileCWDIT010(t *testing.T) {
	acpmock.RequireDriver(t)
	h := newMentionIntegrationDaemon(t, true)
	for name, body := range map[string]string{"root.txt": "root\n", "pkg/main.go": "package pkg\n"} {
		writeDaemonFile(t, filepath.Join(h.root, filepath.FromSlash(name)), body)
	}
	created, err := h.manager.Create(t.Context(), new(session.CreateOpts{
		AgentName: mentionIntegrationAgent, Workspace: h.workspace, Name: "pkg session", CWD: "pkg",
	}))
	if err != nil {
		t.Fatalf("Create(cwd pkg) error = %v", err)
	}
	client := gatewayDaemonUDSClient(h.daemon.homePaths.DaemonSocket)

	t.Run("Should report the creation-profile directory as the working directory", func(t *testing.T) {
		workingDir, err := h.manager.WorkingDir(t.Context(), created.ID)
		if err != nil || filepath.Base(workingDir) != "pkg" {
			t.Fatalf("WorkingDir() = %q, %v; want the pkg creation cwd", workingDir, err)
		}
	})

	t.Run("Should list only the creation-profile directory over UDS", func(t *testing.T) {
		response := gatewayDaemonRequest(t, client, http.MethodGet,
			"http://unix/api/workspaces/"+url.PathEscape(h.workspace)+"/sessions/"+
				url.PathEscape(created.ID)+"/files?limit=50", "", nil)
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("searchSessionFiles status = %d, want 200", response.StatusCode)
		}
		var payload compozycontract.SessionFilesResponse
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode searchSessionFiles: %v", err)
		}
		want := compozycontract.SessionFileEntryPayload{
			Path: "main.go", Name: "main.go", Kind: compozycontract.SessionFileKindFile,
		}
		if len(payload.Entries) != 1 || payload.Entries[0] != want {
			t.Fatalf("entries = %#v, want only pkg's %#v", payload.Entries, want)
		}
	})

	t.Run("Should resolve @main.go against pkg and leave the root file as text", func(t *testing.T) {
		h.send(t, created.ID, "Read @main.go and @root.txt", humanMentionCaller(), "")

		mentions, _ := h.lastDelivery(t, created.ID, 1)
		if mentions == nil || mentions.Links != 1 {
			t.Fatalf("manifest mentions = %#v, want one link", mentions)
		}
		var links []string
		for _, block := range mentions.Blocks {
			if block.Kind == acp.DeliveredMentionBlockResourceLink {
				links = append(links, block.Name)
			}
		}
		if len(links) != 1 || links[0] != "main.go" {
			t.Fatalf("delivered links = %v, want only pkg's main.go", links)
		}
	})
}
