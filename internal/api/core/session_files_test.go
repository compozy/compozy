package core_test

// Suite: searchSessionFiles core handler
// Invariant: file search is scoped to a session of the resolved workspace and read profile, validates
// limit, and maps working-directory and index failures to the documented status codes.
// Boundary IN: workspace/session path params, q/limit query, session WorkingDir, file index search.
// Boundary OUT: HTTP/UDS registration parity (internal/api/testutil) and index ranking (workspacefiles).

import (
	"bytes"
	"context"
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
	"github.com/compozy/compozy/internal/session"
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
) (*gin.Engine, workingDirCallLog) {
	t.Helper()
	gone := filepath.Join(t.TempDir(), "removed")
	infos := map[string]*session.Info{}
	for id, workspaceID := range map[string]string{
		"sess-ok":               "registry-workspace",
		"sess-gone":             "registry-workspace",
		"sess-missing-worktree": "registry-workspace",
		"sess-broken":           "registry-workspace",
		"sess-other-profile":    "registry-workspace",
		"sess-foreign":          "registry-other-workspace",
	} {
		info := testutil.NewSessionInfo(id)
		info.WorkspaceID = workspaceID
		infos[id] = info
	}
	infos["sess-other-profile"].ProfileID = "prof-other"
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
				ID: "registry-workspace", Name: "workspace", WorkspaceID: "stable-workspace",
			}, nil
		},
	}
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		Sessions:     manager,
		Workspaces:   workspaces,
		SessionFiles: searcher,
		Logger:       slog.New(slog.NewTextHandler(log.logs, nil)),
	})
	engine := gin.New()
	engine.GET("/api/workspaces/:workspace_id/sessions/:session_id/files", handlers.SearchSessionFiles)
	return engine, log
}

func serveSessionFilesRequest(t *testing.T, engine http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody))
	return recorder
}
