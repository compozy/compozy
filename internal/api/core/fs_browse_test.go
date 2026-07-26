package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/gin-gonic/gin"
)

func newFilesystemFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := aghconfig.DefaultWithHome(homePaths)

	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "api-core-test",
		HomePaths:     homePaths,
		Config:        cfg,
		Logger:        testutil.DiscardLogger(),
		StartedAt:     time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		Now:           func() time.Time { return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC) },
	})

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.GET("/api/fs/browse", handlers.BrowseDirectory)
	return engine
}

func browse(
	t *testing.T,
	engine *gin.Engine,
	query url.Values,
) (*httptest.ResponseRecorder, contract.FSBrowseResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	target := "/api/fs/browse"
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	engine.ServeHTTP(rec, req)
	var resp contract.FSBrowseResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode browse response: %v (body=%s)", err, rec.Body.String())
		}
	}
	return rec, resp
}

func assertBrowseError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantText string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("browse status = %d, want %d (body=%s)", rec.Code, wantStatus, rec.Body.String())
	}
	var payload contract.ErrorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode browse error response: %v (body=%s)", err, rec.Body.String())
	}
	if payload.Error == "" {
		t.Fatalf("browse error payload = %#v, want non-empty error", payload)
	}
	if wantText != "" && !strings.Contains(payload.Error, wantText) {
		t.Fatalf("browse error = %q, want it to contain %q", payload.Error, wantText)
	}
}

func TestBrowseDirectoryHandler(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "zeta"))
	mustMkdir(t, filepath.Join(root, "alpha"))
	mustMkdir(t, filepath.Join(root, ".hidden"))
	mustWrite(t, filepath.Join(root, "readme.txt"))

	// The handler canonicalizes symlinks (macOS maps /var → /private/var), so
	// navigation anchors must be compared against the resolved root.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", root, err)
	}

	engine := newFilesystemFixture(t)

	t.Run("Should list directories first then files, hiding dotfiles", func(t *testing.T) {
		t.Parallel()
		rec, resp := browse(t, engine, url.Values{"path": {root}})
		if rec.Code != http.StatusOK {
			t.Fatalf("browse = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		names := make([]string, 0, len(resp.Entries))
		for _, entry := range resp.Entries {
			names = append(names, entry.Name)
		}
		want := []string{"alpha", "zeta", "readme.txt"}
		if len(names) != len(want) {
			t.Fatalf("entries = %v, want %v", names, want)
		}
		for i := range want {
			if names[i] != want[i] {
				t.Fatalf("entries = %v, want %v", names, want)
			}
		}
		if resp.Path != resolvedRoot {
			t.Fatalf("Path = %q, want resolved %q", resp.Path, resolvedRoot)
		}
		if resp.Parent != filepath.Dir(resolvedRoot) {
			t.Fatalf("Parent = %q, want %q", resp.Parent, filepath.Dir(resolvedRoot))
		}
	})

	t.Run("Should filter to directories only when requested", func(t *testing.T) {
		t.Parallel()
		_, resp := browse(t, engine, url.Values{"path": {root}, "dirs_only": {"true"}})
		for _, entry := range resp.Entries {
			if !entry.IsDir {
				t.Fatalf("dirs_only returned a file: %q", entry.Name)
			}
		}
		if len(resp.Entries) != 2 {
			t.Fatalf("dirs_only entries = %d, want 2", len(resp.Entries))
		}
	})

	t.Run("Should include dotfiles when show_hidden is set", func(t *testing.T) {
		t.Parallel()
		_, resp := browse(t, engine, url.Values{"path": {root}, "show_hidden": {"true"}})
		found := false
		for _, entry := range resp.Entries {
			if entry.Name == ".hidden" {
				found = true
			}
		}
		if !found {
			t.Fatal("show_hidden did not include .hidden")
		}
	})

	t.Run("Should reject relative paths", func(t *testing.T) {
		t.Parallel()
		rec, _ := browse(t, engine, url.Values{"path": {"relative/dir"}})
		assertBrowseError(t, rec, http.StatusBadRequest, "api: directory path must be absolute")
	})

	t.Run("Should return 404 for a missing directory", func(t *testing.T) {
		t.Parallel()
		rec, _ := browse(t, engine, url.Values{"path": {filepath.Join(root, "does-not-exist")}})
		assertBrowseError(t, rec, http.StatusNotFound, "no such file")
	})

	t.Run("Should return 400 when the path is a file", func(t *testing.T) {
		t.Parallel()
		rec, _ := browse(t, engine, url.Values{"path": {filepath.Join(root, "readme.txt")}})
		assertBrowseError(t, rec, http.StatusBadRequest, "path is not a directory")
	})
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
