package core_test

// Suite: searchSessionFiles core handler
// Invariant: file search is scoped to a session of the resolved workspace and read profile, validates
// limit, and maps working-directory and index failures to the documented status codes.
// Boundary IN: workspace/session path params, q/limit query, session WorkingDir, file index search.
// Boundary OUT: HTTP/UDS registration parity (internal/api/testutil) and index ranking (workspacefiles).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/projectfiles/changes"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspacefiles"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

func TestSearchSessionFilesHandler(t *testing.T) {
	t.Parallel()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	for _, name := range []string{"main.go", "cmd/main.go", "README.md"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	engine, workingDirCalls := newSessionFilesHandlerEngine(t, root, workspacefiles.New(nil))

	t.Run("Should rank files under the session working directory", func(t *testing.T) {
		t.Parallel()
		response := serveSessionFilesRequest(
			t,
			engine,
			"/api/workspaces/workspace/sessions/sess-ok/files?q=main&limit=5",
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.SessionFilesResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		want := []contract.SessionFileEntryPayload{
			{Path: "main.go", Name: "main.go", Kind: contract.SessionFileKindFile},
			{Path: "cmd/main.go", Name: "main.go", Kind: contract.SessionFileKindFile},
		}
		if len(payload.Entries) < len(want) || payload.Entries[0] != want[0] || payload.Entries[1] != want[1] {
			t.Fatalf("entries = %#v, want prefix %#v", payload.Entries, want)
		}
		if payload.Truncated {
			t.Fatalf("truncated = true, want false")
		}
	})

	t.Run("Should default the limit to 50 when omitted", func(t *testing.T) {
		t.Parallel()
		response := serveSessionFilesRequest(t, engine, "/api/workspaces/workspace/sessions/sess-ok/files")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.SessionFilesResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if len(payload.Entries) != 4 {
			t.Fatalf("entries = %#v, want every file and folder", payload.Entries)
		}
	})

	for _, test := range []struct {
		name        string
		path        string
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:       "reject a zero limit",
			path:       "/api/workspaces/workspace/sessions/sess-ok/files?limit=0",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "reject a limit above the maximum",
			path:       "/api/workspaces/workspace/sessions/sess-ok/files?limit=201",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "reject a non-integer limit",
			path:       "/api/workspaces/workspace/sessions/sess-ok/files?limit=many",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "report an unknown session as not found",
			path:       "/api/workspaces/workspace/sessions/sess-unknown/files",
			wantStatus: http.StatusNotFound,
			wantCode:   "session_not_found",
		},
		{
			name:       "hide a session of another workspace",
			path:       "/api/workspaces/workspace/sessions/sess-foreign/files",
			wantStatus: http.StatusNotFound,
			wantCode:   "session_not_found",
		},
		{
			name:       "hide a session of another profile",
			path:       "/api/workspaces/workspace/sessions/sess-other-profile/files",
			wantStatus: http.StatusNotFound,
			wantCode:   "session_not_found",
		},
		{
			name:        "report a removed working directory as unavailable",
			path:        "/api/workspaces/workspace/sessions/sess-gone/files",
			wantStatus:  http.StatusConflict,
			wantCode:    "session_directory_unavailable",
			wantMessage: "session working directory no longer exists",
		},
		{
			name:        "report a missing bound worktree as unavailable",
			path:        "/api/workspaces/workspace/sessions/sess-missing-worktree/files",
			wantStatus:  http.StatusConflict,
			wantCode:    "session_directory_unavailable",
			wantMessage: "session working directory no longer exists",
		},
	} {
		t.Run("Should "+test.name, func(t *testing.T) {
			t.Parallel()
			response := serveSessionFilesRequest(t, engine, test.path)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, response, &payload)
			if payload.Code != test.wantCode || payload.Error == "" {
				t.Fatalf("error payload = %#v, want code %q with a message", payload, test.wantCode)
			}
			if test.wantMessage != "" && payload.Error != test.wantMessage {
				t.Fatalf("error message = %q, want %q without any path", payload.Error, test.wantMessage)
			}
		})
	}

	t.Run("Should never resolve the working directory of a hidden session", func(t *testing.T) {
		t.Parallel()
		for _, id := range []string{"sess-foreign", "sess-other-profile"} {
			response := serveSessionFilesRequest(t, engine, "/api/workspaces/workspace/sessions/"+id+"/files")
			if response.Code != http.StatusNotFound {
				t.Fatalf("%s status = %d, want 404", id, response.Code)
			}
			if calls := workingDirCalls.count(id); calls != 0 {
				t.Fatalf("%s WorkingDir calls = %d, want 0", id, calls)
			}
		}
	})

	t.Run("Should answer 500 with a path-free message when a lookup or the index fails", func(t *testing.T) {
		t.Parallel()
		failing, calls := newSessionFilesHandlerEngine(t, root, failingSessionFileSearcher{root: root})
		for _, id := range []string{"sess-ok", "sess-broken"} {
			response := serveSessionFilesRequest(t, failing, "/api/workspaces/workspace/sessions/"+id+"/files")
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("%s status = %d, want 500; body=%s", id, response.Code, response.Body.String())
			}
			if body := response.Body.String(); strings.Contains(body, root) {
				t.Fatalf("%s body = %s, want no absolute root", id, body)
			}
			if logged := calls.logs.String(); strings.Contains(logged, root) {
				t.Fatalf("%s logs = %s, want no absolute root", id, logged)
			}
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, response, &payload)
			if payload.Error != "session file search failed" || payload.Code == "session_directory_unavailable" {
				t.Fatalf("%s error payload = %#v, want the fixed internal error", id, payload)
			}
		}
	})
}

type failingSessionFileSearcher struct {
	root string
}

func (s failingSessionFileSearcher) Search(context.Context, string, string, int) (workspacefiles.Result, error) {
	return workspacefiles.Result{}, fmt.Errorf(
		"workspacefiles: index root: walk root: open %s: %w",
		s.root,
		fs.ErrPermission,
	)
}

type workingDirCallLog struct {
	calls chan string
	logs  *bytes.Buffer
}

func (l workingDirCallLog) count(id string) int {
	total := 0
	for {
		select {
		case got := <-l.calls:
			if got == id {
				total++
			}
		default:
			return total
		}
	}
}

func newSessionFilesHandlerEngine(
	t *testing.T,
	root string,
	searcher core.SessionFileSearcher,
	engines ...changes.Engine,
) (*gin.Engine, workingDirCallLog) {
	t.Helper()
	gone := filepath.Join(t.TempDir(), "removed")
	infos := map[string]*session.Info{}
	for id, workspaceID := range map[string]string{
		"sess-ok":               "registry-workspace",
		"sess-user":             "",
		"sess-gone":             "registry-workspace",
		"sess-missing-worktree": "registry-workspace",
		"sess-broken":           "registry-workspace",
		"sess-other-profile":    "registry-workspace",
		"sess-foreign":          "registry-other-workspace",
		"sess-worktree":         "registry-workspace",
	} {
		info := testutil.NewSessionInfo(id)
		info.WorkspaceID = workspaceID
		infos[id] = info
	}
	infos["sess-other-profile"].ProfileID = "prof-other"
	infos["sess-worktree"].WorktreeID = "wt-sub"
	log := workingDirCallLog{calls: make(chan string, 64), logs: &bytes.Buffer{}}
	manager := &testutil.StubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			info, ok := infos[id]
			if !ok {
				return nil, session.ErrSessionNotFound
			}
			return info, nil
		},
		WorkingDirFn: func(_ context.Context, id string) (string, error) {
			select {
			case log.calls <- id:
			default:
			}
			switch id {
			case "sess-ok":
				return root, nil
			case "sess-gone":
				return gone, nil
			case "sess-missing-worktree":
				return "", fmt.Errorf("session: resolve working-directory worktree: %w", worktree.ErrMissing)
			case "sess-broken":
				return "", fmt.Errorf("session: resolve working directory %s: %w", root, fs.ErrPermission)
			default:
				return "", fmt.Errorf("unexpected working dir lookup for %s: %w", id, fs.ErrInvalid)
			}
		},
	}
	workspaces := testutil.StubWorkspaceService{
		ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if strings.TrimSpace(ref) != "workspace" {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
			}
			return workspacepkg.ResolvedWorkspace{
				ID: "registry-workspace", Name: "workspace", WorkspaceID: "stable-workspace", RootDir: root,
			}, nil
		},
	}
	var changeEngine changes.Engine
	if len(engines) > 0 {
		changeEngine = engines[0]
	}
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		Sessions:       manager,
		Workspaces:     workspaces,
		SessionFiles:   searcher,
		SessionChanges: changeEngine,
		Worktrees:      subdirWorktreeService{root: root},
		Logger:         slog.New(slog.NewTextHandler(log.logs, nil)),
	})
	engine := gin.New()
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/files", handlers.SearchSessionFiles)
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/files/entries", handlers.ListSessionFileEntries)
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/files/content", handlers.ReadSessionFileContent)
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/files/raw/*path", handlers.ReadSessionFileBytes)
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/changes", handlers.GetSessionChanges)
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/changes/patches", handlers.GetSessionChangePatches)
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/changes/contents", handlers.GetSessionChangeContents)
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/changes/bases", handlers.ListSessionChangeBases)

	engine.GET("/api/workspaces/:workspace_id/files/entries", handlers.ListFileEntries)
	engine.GET("/api/workspaces/:workspace_id/files/content", handlers.ReadFileContent)
	engine.GET("/api/workspaces/:workspace_id/files/raw/*path", handlers.ReadFileBytes)
	engine.GET("/api/workspaces/:workspace_id/worktrees/:worktree_id/files/raw/*path", handlers.ReadWorktreeFileBytes)

	return engine, log
}

// subdirWorktreeService resolves the one test worktree to a directory below the workspace root.
type subdirWorktreeService struct {
	core.WorktreeService
	root string
}

func (s subdirWorktreeService) Resolve(_ context.Context, _ string, ref string) (*worktree.Worktree, error) {
	if ref != "wt-sub" {
		return nil, worktree.ErrNotFound
	}
	return &worktree.Worktree{
		ID: "wt-sub", ProfileID: store.DefaultProfileID, Path: filepath.Join(s.root, "worktree-sub"),
	}, nil
}

func serveSessionFilesRequest(t *testing.T, engine http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody))
	return recorder
}

// File content handlers own the app-token reveal boundary and response path redaction.
func TestProjectFileContentReveal(t *testing.T) {
	t.Parallel()
	t.Run("Should gate app reveals and redact host paths for workspace and session forms", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		for name, body := range map[string]string{"README.md": "public", ".env": "secret"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		engine, _ := newSessionFilesHandlerEngine(t, root, workspacefiles.New(nil))
		for _, prefix := range []string{"/api/workspaces/workspace/files", "/api/workspaces/workspace/sessions/sess-ok/files"} {
			for _, tc := range []struct {
				query  string
				app    bool
				status int
				kind   string
			}{
				{"path=README.md", true, 200, "text"}, {"path=.env", true, 200, "sensitive"}, {"path=.env&reveal=true", true, 403, ""}, {"path=.env&reveal=true", false, 200, "text"}, {"path=missing", false, 404, ""}, {"path=%2Fetc%2Fpasswd", false, 400, ""},
			} {
				request := httptest.NewRequestWithContext(
					t.Context(),
					http.MethodGet,
					prefix+"/content?"+tc.query,
					http.NoBody,
				)
				if tc.app {
					request = request.WithContext(core.ContextWithAppToken(request.Context(), gateway.AppToken{}))
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				if response.Code != tc.status {
					t.Fatalf("%s status %d: %s", tc.query, response.Code, response.Body.String())
				}
				if strings.Contains(response.Body.String(), root) {
					t.Fatal("response leaked host root")
				}
				if tc.kind != "" {
					var got contract.FileContentResponse
					if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					if got.Kind != tc.kind {
						t.Fatalf("kind=%s want %s", got.Kind, tc.kind)
					}
					// Clients route by the root's workspace ID, so it must be the public one.
					if got.Root.WorkspaceID != "registry-workspace" {
						t.Fatalf("root workspace_id=%q want the public registry ID", got.Root.WorkspaceID)
					}
					if got.Kind == "sensitive" && got.Text != nil {
						t.Fatal("sensitive bytes leaked")
					}
				} else if !strings.Contains(response.Body.String(), "code") {
					t.Fatal("missing structured error")
				}
			}
		}
	})
}

// Raw file routes stream confined bytes with an extension-only type, browser-safe headers, and
// HTTP conditional/range semantics, from the workspace, worktree, or session root they name.
func TestProjectFileBytes(t *testing.T) {
	t.Parallel()
	newRoot := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		for name, body := range map[string]string{
			"docs/logo.svg":            "<svg/>",
			"docs/page.html":           "<script>alert(1)</script>",
			"media/clip.mp4":           "0123456789",
			".env":                     "SECRET",
			"worktree-sub/only-wt.txt": "worktree",
		} {
			target := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	t.Run("Should type and harden every streamed file by extension", func(t *testing.T) {
		t.Parallel()
		root := newRoot(t)
		engine, _ := newSessionFilesHandlerEngine(t, root, workspacefiles.New(nil))
		for _, tc := range []struct{ path, mime, disposition, body string }{
			{"docs/logo.svg", "image/svg+xml", `inline; filename=logo.svg`, "<svg/>"},
			{"docs/page.html", "application/octet-stream", `attachment; filename=page.html`, "<script>alert(1)</script>"},
			{"media/clip.mp4", "video/mp4", `inline; filename=clip.mp4`, "0123456789"},
		} {
			response := serveSessionFilesRequest(t, engine, "/api/workspaces/workspace/files/raw/"+tc.path)
			headers := response.Header()
			if response.Code != http.StatusOK || response.Body.String() != tc.body {
				t.Fatalf("%s: %d %q", tc.path, response.Code, response.Body.String())
			}
			for name, want := range map[string]string{
				"Content-Type":                 tc.mime,
				"Content-Disposition":          tc.disposition,
				"X-Content-Type-Options":       "nosniff",
				"Content-Security-Policy":      "sandbox; default-src 'none'; style-src 'unsafe-inline'",
				"Cross-Origin-Resource-Policy": "same-origin",
				"Cache-Control":                "private, no-cache",
				"Referrer-Policy":              "no-referrer",
				"Accept-Ranges":                "bytes",
			} {
				if got := headers.Get(name); got != want {
					t.Fatalf("%s %s = %q, want %q", tc.path, name, got, want)
				}
			}
			if !strings.HasPrefix(headers.Get("ETag"), `W/"`) {
				t.Fatalf("%s ETag = %q", tc.path, headers.Get("ETag"))
			}
		}
	})
	t.Run("Should honor ranges and validators", func(t *testing.T) {
		t.Parallel()
		root := newRoot(t)
		engine, _ := newSessionFilesHandlerEngine(t, root, workspacefiles.New(nil))
		const route = "/api/workspaces/workspace/files/raw/media/clip.mp4"
		etag := serveSessionFilesRequest(t, engine, route).Header().Get("ETag")
		for _, tc := range []struct {
			header, value string
			status        int
			body, rangeHd string
		}{
			{"Range", "bytes=2-5", http.StatusPartialContent, "2345", "bytes 2-5/10"},
			{"Range", "bytes=20-30", http.StatusRequestedRangeNotSatisfiable, "", "bytes */10"},
			{"If-None-Match", etag, http.StatusNotModified, "", ""},
		} {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, route, http.NoBody)
			request.Header.Set(tc.header, tc.value)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != tc.status || response.Header().Get("Content-Range") != tc.rangeHd ||
				(tc.body != "" && response.Body.String() != tc.body) {
				t.Fatalf("%s %s: %d range=%q body=%q", tc.header, tc.value, response.Code,
					response.Header().Get("Content-Range"), response.Body.String())
			}
		}
	})
	t.Run("Should map refusals to structured errors without host paths", func(t *testing.T) {
		t.Parallel()
		root := newRoot(t)
		engine, _ := newSessionFilesHandlerEngine(t, root, workspacefiles.New(nil))
		for _, tc := range []struct {
			path   string
			status int
			code   string
		}{
			{"/api/workspaces/workspace/files/raw/.env", http.StatusForbidden, "sensitive_file"},
			{"/api/workspaces/workspace/sessions/sess-ok/files/raw/.env", http.StatusForbidden, "sensitive_file"},
			{"/api/workspaces/workspace/files/raw/missing.png", http.StatusNotFound, "file_not_found"},
			{"/api/workspaces/workspace/files/raw/docs", http.StatusUnprocessableEntity, "not_a_file"},
			{"/api/workspaces/workspace/files/raw/docs/%2E%2E/%2E%2E/x", http.StatusBadRequest, "invalid_path"},
			{"/api/workspaces/workspace/worktrees/wt-gone/files/raw/a.txt", http.StatusNotFound, "worktree_not_found"},
		} {
			response := serveSessionFilesRequest(t, engine, tc.path)
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) ||
				strings.Contains(response.Body.String(), root) {
				t.Fatalf("%s: %d %s", tc.path, response.Code, response.Body.String())
			}
		}
	})
	t.Run("Should resolve the worktree and session roots each route names", func(t *testing.T) {
		t.Parallel()
		root := newRoot(t)
		engine, _ := newSessionFilesHandlerEngine(t, root, workspacefiles.New(nil))
		for path, want := range map[string]int{
			"/api/workspaces/workspace/worktrees/wt-sub/files/raw/only-wt.txt":       http.StatusOK,
			"/api/workspaces/workspace/sessions/sess-worktree/files/raw/only-wt.txt": http.StatusOK,
			"/api/workspaces/workspace/sessions/sess-ok/files/raw/docs/logo.svg":     http.StatusOK,
			"/api/workspaces/workspace/files/raw/only-wt.txt":                        http.StatusNotFound,
			"/api/workspaces/workspace/sessions/sess-ok/files/raw/only-wt.txt":       http.StatusNotFound,
			"/api/workspaces/workspace/files/raw/only-wt.txt?worktree_id=wt-sub":     http.StatusNotFound,
		} {
			if response := serveSessionFilesRequest(t, engine, path); response.Code != want {
				t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
			}
		}
	})
}

func TestProjectChangesReveal(t *testing.T) {
	t.Parallel()
	t.Run("Should reject app token reveal before running git", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		engine, _ := newSessionFilesHandlerEngine(t, root, workspacefiles.New(nil))
		for _, suffix := range []string{"/patches", "/contents"} {
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"/api/workspaces/workspace/sessions/sess-ok/changes"+suffix+"?path=.env&revision=x&reveal=true",
				http.NoBody,
			)
			req = req.WithContext(core.ContextWithAppToken(req.Context(), gateway.AppToken{}))
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			if response.Code != 403 || !strings.Contains(response.Body.String(), "reveal_forbidden") ||
				strings.Contains(response.Body.String(), root) {
				t.Fatalf("%d: %s", response.Code, response.Body.String())
			}
		}
	})
}

func TestProjectChangesUserScope(t *testing.T) {
	t.Parallel()
	t.Run("Should report user session roots as unavailable for every Changes operation", func(t *testing.T) {
		t.Parallel()
		engine, _ := newSessionFilesHandlerEngine(t, t.TempDir(), workspacefiles.New(nil))
		for _, suffix := range []string{"", "/patches", "/contents", "/bases"} {
			response := serveSessionFilesRequest(t, engine, "/api/workspaces/@user/sessions/sess-user/changes"+suffix)
			if response.Code != 409 || !strings.Contains(response.Body.String(), "root_unavailable") {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
		}
	})
}

func TestProjectChangesDiagnostics(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"", "/patches?revision=r&path=a.go", "/contents?revision=r&path=a.go", "/bases"} {
		t.Run("Should log sanitized cause for "+suffix, func(t *testing.T) {
			t.Parallel()
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			external := "/outside/private-orderfile"
			cause := fmt.Errorf(
				"%w: git diff at %s orderfile '%s': %w",
				changes.ErrUnavailable,
				root,
				external,
				context.DeadlineExceeded,
			)
			engine, log := newSessionFilesHandlerEngine(
				t,
				root,
				workspacefiles.New(nil),
				failedChangesEngine{cause: cause},
			)
			response := serveSessionFilesRequest(t, engine, "/api/workspaces/workspace/sessions/sess-ok/changes"+suffix)
			if response.Code != 500 || !strings.Contains(response.Body.String(), "changes_unavailable") ||
				strings.Contains(response.Body.String(), "deadline") {
				t.Fatalf("response %d %s", response.Code, response.Body)
			}
			if strings.Contains(log.logs.String(), external) || strings.Contains(log.logs.String(), root) ||
				!strings.Contains(log.logs.String(), "deadline exceeded") ||
				!strings.Contains(log.logs.String(), "WARN") {
				t.Fatalf("logs %s", log.logs)
			}
		})
	}
}

type failedChangesEngine struct{ cause error }

func (e failedChangesEngine) Summary(context.Context, projectfiles.Root, changes.Request) (changes.Summary, error) {
	return changes.Summary{}, e.cause
}

func (e failedChangesEngine) Patches(
	context.Context,
	projectfiles.Root,
	changes.Request,
	[]changes.PatchTarget,
) ([]changes.Patch, error) {
	return nil, e.cause
}

func (e failedChangesEngine) Contents(
	context.Context,
	projectfiles.Root,
	changes.Request,
	changes.PatchTarget,
) (changes.Contents, error) {
	return changes.Contents{}, e.cause
}
func (e failedChangesEngine) Bases(context.Context, projectfiles.Root, string, int) (changes.BaseList, error) {
	return changes.BaseList{}, e.cause
}
