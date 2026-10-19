package testutil_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspacefiles"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

// Canonical suite: IT-005 searchSessionFiles transport parity over HTTP and UDS with the real
// git-backed index. Handler status mapping is owned by internal/api/core/session_files_test.go and
// the worktree-bound session journey by internal/daemon/session_files_integration_test.go.
func TestSearchSessionFilesHTTPUDSTransportParityIT005(t *testing.T) {
	t.Parallel()

	runner, err := worktree.NewRealGitRunner(workspacefiles.DefaultGitTimeout)
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	worktreeRoot := newSessionFilesGitRepo(t, runner)
	removedDir := filepath.Join(t.TempDir(), "removed-worktree")
	sessions := sessionFilesSessionManager{
		infos: map[string]*session.Info{
			"sess-wt":      {ID: "sess-wt", WorkspaceID: "ws-registry", ProfileID: store.DefaultProfileID},
			"sess-gone":    {ID: "sess-gone", WorkspaceID: "ws-registry", ProfileID: store.DefaultProfileID},
			"sess-foreign": {ID: "sess-foreign", WorkspaceID: "ws-other", ProfileID: store.DefaultProfileID},
		},
		dirs: map[string]string{"sess-wt": worktreeRoot, "sess-gone": removedDir},
	}
	index := workspacefiles.New(runner)
	httpRouter := newSessionFilesParityHTTPRouter(t, sessions, index)
	udsRouter := newSessionFilesParityUDSRouter(t, sessions, index)

	t.Run("Should rank files from the session working directory without taking the git index lock", func(t *testing.T) {
		t.Parallel()
		path := "/api/workspaces/ws-public/sessions/sess-wt/files?q=main&limit=5"
		httpResponse := performWorktreeParityRequest(t, httpRouter, http.MethodGet, path, "")
		udsResponse := performWorktreeParityRequest(t, udsRouter, http.MethodGet, path, "")
		assertWorktreeParityResponse(t, httpResponse, udsResponse, http.StatusOK)
		var payload contract.SessionFilesResponse
		if err := json.Unmarshal(httpResponse.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode files response: %v; body=%s", err, httpResponse.Body.String())
		}
		want := []contract.SessionFileEntryPayload{
			{Path: "main.go", Name: "main.go", Kind: contract.SessionFileKindFile},
			{Path: "cmd/main.go", Name: "main.go", Kind: contract.SessionFileKindFile},
		}
		if len(payload.Entries) < len(want) || payload.Entries[0] != want[0] || payload.Entries[1] != want[1] {
			t.Fatalf("files entries = %#v, want prefix %#v", payload.Entries, want)
		}
		if len(payload.Entries) > 5 || payload.Truncated {
			t.Fatalf(
				"files = %d entries truncated=%v, want at most 5 untruncated",
				len(payload.Entries),
				payload.Truncated,
			)
		}
		if _, err := os.Stat(filepath.Join(worktreeRoot, ".git", "index.lock")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("index.lock stat error = %v, want not exist", err)
		}
	})

	t.Run("Should default the limit when it is omitted", func(t *testing.T) {
		t.Parallel()
		path := "/api/workspaces/ws-public/sessions/sess-wt/files"
		httpResponse := performWorktreeParityRequest(t, httpRouter, http.MethodGet, path, "")
		udsResponse := performWorktreeParityRequest(t, udsRouter, http.MethodGet, path, "")
		assertWorktreeParityResponse(t, httpResponse, udsResponse, http.StatusOK)
		var payload contract.SessionFilesResponse
		if err := json.Unmarshal(httpResponse.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode files response: %v", err)
		}
		if len(payload.Entries) != 5 || payload.Entries[0].Path != ".gitignore" {
			t.Fatalf("files entries = %#v, want every entry shallowest first", payload.Entries)
		}
	})

	for _, test := range []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "reject a zero limit",
			path:       "/api/workspaces/ws-public/sessions/sess-wt/files?limit=0",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "hide a session of another workspace",
			path:       "/api/workspaces/ws-public/sessions/sess-foreign/files",
			wantStatus: http.StatusNotFound,
			wantCode:   "session_not_found",
		},
		{
			name:       "report a removed working directory as unavailable",
			path:       "/api/workspaces/ws-public/sessions/sess-gone/files",
			wantStatus: http.StatusConflict,
			wantCode:   "session_directory_unavailable",
		},
	} {
		t.Run("Should "+test.name, func(t *testing.T) {
			t.Parallel()
			httpResponse := performWorktreeParityRequest(t, httpRouter, http.MethodGet, test.path, "")
			udsResponse := performWorktreeParityRequest(t, udsRouter, http.MethodGet, test.path, "")
			assertWorktreeParityResponse(t, httpResponse, udsResponse, test.wantStatus)
			var payload contract.ErrorPayload
			if err := json.Unmarshal(httpResponse.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error payload: %v; body=%s", err, httpResponse.Body.String())
			}
			if payload.Code != test.wantCode || payload.Error == "" {
				t.Fatalf("error payload = %#v, want code %q with a message", payload, test.wantCode)
			}
		})
	}
}

type sessionFilesSessionManager struct {
	testutil.StubSessionManager
	infos map[string]*session.Info
	dirs  map[string]string
}

func (m sessionFilesSessionManager) Status(_ context.Context, id string) (*session.Info, error) {
	info, ok := m.infos[id]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	return info, nil
}

func (m sessionFilesSessionManager) WorkingDir(_ context.Context, id string) (string, error) {
	return m.dirs[id], nil
}

func newSessionFilesGitRepo(t *testing.T, runner *worktree.RealGitRunner) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	for name, content := range map[string]string{
		"main.go":     "package main\n",
		"cmd/main.go": "package main\n",
		"README.md":   "# repo\n",
		".gitignore":  "build/\n",
		"build/out":   "ignored\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-q", "-m", "init"},
	} {
		if _, stderr, err := runner.Run(t.Context(), root, args...); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, stderr)
		}
	}
	return root
}

func newSessionFilesParityHTTPRouter(
	t *testing.T,
	sessions core.SessionManager,
	index core.SessionFileSearcher,
) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigForTest(homePaths)
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", 2123
	if _, err := httpapi.New(
		httpapi.WithEngine(engine),
		httpapi.WithHomePaths(homePaths),
		httpapi.WithConfig(&cfg),
		httpapi.WithHost(cfg.HTTP.Host),
		httpapi.WithPort(cfg.HTTP.Port),
		httpapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		httpapi.WithStartedAt(parityWorktreeTime()),
		httpapi.WithNow(parityWorktreeTime),
		httpapi.WithSessionManager(
			sessions,
		),
		httpapi.WithSessionFileSearcher(index),
		httpapi.WithTaskService(&testutil.StubTaskManager{}),
		httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithWorkspaceResolver(parityWorkspaceService()),
	); err != nil {
		t.Fatalf("httpapi.New(session files parity) error = %v", err)
	}
	return engine
}

func newSessionFilesParityUDSRouter(
	t *testing.T,
	sessions core.SessionManager,
	index core.SessionFileSearcher,
) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigForTest(homePaths)
	if _, err := udsapi.New(
		udsapi.WithEngine(engine),
		udsapi.WithHomePaths(homePaths),
		udsapi.WithConfig(&cfg),
		udsapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		udsapi.WithStartedAt(parityWorktreeTime()),
		udsapi.WithNow(parityWorktreeTime),
		udsapi.WithSessionManager(
			sessions,
		),
		udsapi.WithSessionFileSearcher(index),
		udsapi.WithTaskService(&testutil.StubTaskManager{}),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(parityWorkspaceService()),
	); err != nil {
		t.Fatalf("udsapi.New(session files parity) error = %v", err)
	}
	return engine
}
