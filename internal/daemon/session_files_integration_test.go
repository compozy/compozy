//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	t.Run("Should serve project files with worktree roots and HTTP UDS parity", func(t *testing.T) {
		base := "/api/workspaces/" + url.PathEscape(
			harness.WorkspaceID,
		) + "/sessions/" + url.PathEscape(
			bound.ID,
		) + "/files"
		var listing compozycontract.FileEntriesResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, base+"/entries", nil, &listing); err != nil {
			t.Fatal(err)
		}
		if listing.Root.Kind != "worktree" || listing.Root.WorktreeID != created.ID {
			t.Fatalf("root = %#v", listing.Root)
		}
		var a, b compozycontract.FileContentResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, base+"/content?path=cmd/main.go", nil, &a); err != nil {
			t.Fatal(err)
		}
		if err := harness.UDSJSON(ctx, http.MethodGet, base+"/content?path=cmd/main.go", nil, &b); err != nil {
			t.Fatal(err)
		}
		assertWorktreeE2EJSONParity(t, "file content", a, b)
		if a.Kind != "text" || a.Text == nil || *a.Text != "package main\n" {
			t.Fatalf("content = %#v", a)
		}
	})

	t.Run("Should find and search the bound root with HTTP UDS parity", func(t *testing.T) {
		base := "/api/workspaces/" + harness.WorkspaceID
		for _, prefix := range []string{base + "/files", base + "/sessions/" + bound.ID + "/files"} {
			var a, b compozycontract.FileFindResponse
			path := prefix + "/find?q=only"
			if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &a); err != nil {
				t.Fatal(err)
			}
			if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &b); err != nil {
				t.Fatal(err)
			}
			assertWorktreeE2EJSONParity(t, "find", a, b)
			want := "root_only.go"
			if strings.Contains(prefix, "/sessions/") {
				want = "worktree_only.go"
			}
			if len(a.Results) != 1 || a.Results[0].Path != want {
				t.Fatalf("find=%+v", a)
			}
			var x, y compozycontract.FileSearchResponse
			path = prefix + "/search?q=package&include=cmd/**&exclude=**/*.test.go"
			if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &x); err != nil {
				t.Fatal(err)
			}
			if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &y); err != nil {
				t.Fatal(err)
			}
			assertWorktreeE2EJSONParity(t, "search", x, y)
			if x.Engine != "git_grep" || x.Totals.Matches != 1 || len(x.Files) != 1 || x.Files[0].Path != "cmd/main.go" {
				t.Fatalf("search=%+v", x)
			}
			if strings.Contains(string(mustSearchJSON(t, x)), root) {
				t.Fatal("host path leaked")
			}
			response, err := http.Get(harness.HTTPURL(prefix + "/search?q=foo%28&regex=true"))
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != 400 || !bytes.Contains(data, []byte("invalid_query")) {
				t.Fatalf("invalid query status=%d body=%s err=%v", response.StatusCode, data, err)
			}
		}
	})

	t.Run("Should fence sensitive content for app tokens on both root forms", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(ready.Worktree.Path, ".env"), []byte("secret"), 0600); err != nil {
			t.Fatal(err)
		}
		var issued compozycontract.AppTokenIssuedResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodPost,
			"/api/app-tokens",
			compozycontract.CreateAppTokenRequest{Name: "File reader"},
			&issued,
		); err != nil {
			t.Fatal(err)
		}
		for _, prefix := range []string{"/api/workspaces/" + harness.WorkspaceID + "/files", "/api/workspaces/" + harness.WorkspaceID + "/sessions/" + bound.ID + "/files"} {
			query := ""
			if !strings.Contains(prefix, "/sessions/") {
				query = "&worktree_id=" + created.ID
			}
			for _, tc := range []struct {
				path     string
				status   int
				contains string
				token    bool
			}{
				{"/content?path=cmd/main.go", 200, `"kind":"text"`, true}, {"/content?path=.env", 200, `"kind":"sensitive"`, true}, {"/content?path=.env&reveal=true", 403, `"code":"reveal_forbidden"`, true}, {"/content?path=.env&reveal=true", 200, `"text":"secret"`, false}, {"/entries?path=missing", 404, `"code":"file_not_found"`, false},
			} {
				var bodies [][]byte
				for transportIndex, transport := range []struct {
					client *http.Client
					base   string
				}{{harness.HTTPClient, harness.HTTPBaseURL}, {harness.UDSClient, "http://unix"}} {
					if tc.token && transportIndex == 1 {
						continue
					}
					req, err := http.NewRequestWithContext(
						ctx,
						http.MethodGet,
						transport.base+prefix+tc.path+query,
						http.NoBody,
					)
					if err != nil {
						t.Fatal(err)
					}
					if tc.token {
						req.Header.Set("Authorization", "Bearer "+issued.Token)
					}
					response, err := transport.client.Do(req)
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(response.Body)
					closeErr := response.Body.Close()
					if err != nil || closeErr != nil {
						t.Fatalf("read: %v, close: %v", err, closeErr)
					}
					if response.StatusCode != tc.status || !bytes.Contains(body, []byte(tc.contains)) {
						t.Fatalf("status %d: %s", response.StatusCode, body)
					}
					if bytes.Contains(body, []byte(root)) || bytes.Contains(body, []byte(ready.Worktree.Path)) {
						t.Fatal("absolute root leaked")
					}
					bodies = append(bodies, body)
				}
				if len(bodies) == 2 && !bytes.Equal(bodies[0], bodies[1]) {
					t.Fatalf("HTTP/UDS differ: %s / %s", bodies[0], bodies[1])
				}
			}
		}
	})
	t.Run("Should report a removed worktree as root unavailable", func(t *testing.T) {
		if err := os.RemoveAll(ready.Worktree.Path); err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			harness.HTTPURL("/api/workspaces/"+harness.WorkspaceID+"/sessions/"+bound.ID+"/files/entries"),
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		response, err := harness.HTTPClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var payload compozycontract.ErrorPayload
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 409 || payload.Code != "root_unavailable" {
			t.Fatalf("status %d: %#v", response.StatusCode, payload)
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

func mustSearchJSON(t *testing.T, value compozycontract.FileSearchResponse) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
