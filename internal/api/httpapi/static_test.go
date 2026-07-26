package httpapi

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticRoutesServeEmbeddedIndexForRootAndDeepLinks(t *testing.T) {
	t.Setenv(webDistDirEnvVar, "")

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	rootResp := performRequest(t, engine, http.MethodGet, "/", nil)
	if rootResp.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d; body=%s", rootResp.Code, http.StatusOK, rootResp.Body.String())
	}
	if !strings.Contains(rootResp.Body.String(), `<div id="app"></div>`) {
		t.Fatalf("GET / body = %q, want SPA shell", rootResp.Body.String())
	}

	deepLinkResp := performRequest(t, engine, http.MethodGet, "/session/sess-001", nil)
	if deepLinkResp.Code != http.StatusOK {
		t.Fatalf(
			"GET /session/sess-001 status = %d, want %d; body=%s",
			deepLinkResp.Code,
			http.StatusOK,
			deepLinkResp.Body.String(),
		)
	}
	if !strings.Contains(deepLinkResp.Body.String(), `<div id="app"></div>`) {
		t.Fatalf("GET /session/sess-001 body = %q, want SPA shell", deepLinkResp.Body.String())
	}
}

func TestStaticRoutesServeEmbeddedAssets(t *testing.T) {
	t.Setenv(webDistDirEnvVar, "")

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	requestPath, expected := firstEmbeddedAsset(t)
	resp := performRequest(t, engine, http.MethodGet, requestPath, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d; body=%s", requestPath, resp.Code, http.StatusOK, resp.Body.String())
	}
	if got, want := resp.Body.String(), string(expected); got != want {
		t.Fatalf("GET %s body mismatch", requestPath)
	}
	if strings.Contains(resp.Body.String(), "<!doctype html>") {
		t.Fatalf("GET %s returned SPA HTML instead of asset payload", requestPath)
	}
}

func TestStaticRoutesDoNotFallbackForMissingAssetsOrAPIRoutes(t *testing.T) {
	t.Setenv(webDistDirEnvVar, "")

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	missingAssetResp := performRequest(t, engine, http.MethodGet, "/assets/does-not-exist.js", nil)
	if missingAssetResp.Code != http.StatusNotFound {
		t.Fatalf(
			"GET missing asset status = %d, want %d; body=%s",
			missingAssetResp.Code,
			http.StatusNotFound,
			missingAssetResp.Body.String(),
		)
	}
	if strings.Contains(missingAssetResp.Body.String(), "<!doctype html>") {
		t.Fatalf("GET missing asset body = %q, want plain 404", missingAssetResp.Body.String())
	}

	missingAPIResp := performRequest(t, engine, http.MethodGet, "/api/missing", nil)
	if missingAPIResp.Code != http.StatusNotFound {
		t.Fatalf(
			"GET /api/missing status = %d, want %d; body=%s",
			missingAPIResp.Code,
			http.StatusNotFound,
			missingAPIResp.Body.String(),
		)
	}
	if strings.Contains(missingAPIResp.Body.String(), "<!doctype html>") {
		t.Fatalf("GET /api/missing body = %q, want plain 404", missingAPIResp.Body.String())
	}
}

func TestStaticRoutesServeLocalWebDistOverride(t *testing.T) {
	t.Run("Should serve AGH_WEB_DIST_DIR index and assets from disk", func(t *testing.T) {
		distDir := writeLocalStaticDist(t, "local shell one", map[string]string{
			"assets/local.js": "console.log('local asset');",
		})
		t.Setenv(webDistDirEnvVar, distDir)

		homePaths := newTestHomePaths(t)
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

		rootResp := performRequest(t, engine, http.MethodGet, "/", nil)
		if rootResp.Code != http.StatusOK {
			t.Fatalf("GET / status = %d, want %d; body=%s", rootResp.Code, http.StatusOK, rootResp.Body.String())
		}
		if got := rootResp.Body.String(); !strings.Contains(got, "local shell one") {
			t.Fatalf("GET / body = %q, want local override shell", got)
		}

		assetResp := performRequest(t, engine, http.MethodGet, "/assets/local.js", nil)
		if assetResp.Code != http.StatusOK {
			t.Fatalf(
				"GET /assets/local.js status = %d, want %d; body=%s",
				assetResp.Code,
				http.StatusOK,
				assetResp.Body.String(),
			)
		}
		if got, want := assetResp.Body.String(), "console.log('local asset');"; got != want {
			t.Fatalf("GET /assets/local.js body = %q, want %q", got, want)
		}
	})
}

func TestStaticRoutesObserveLocalWebDistRewrite(t *testing.T) {
	t.Run("Should serve rewritten AGH_WEB_DIST_DIR files without rebuilding Go", func(t *testing.T) {
		distDir := writeLocalStaticDist(t, "local shell before", nil)
		t.Setenv(webDistDirEnvVar, distDir)

		homePaths := newTestHomePaths(t)
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

		before := performRequest(t, engine, http.MethodGet, "/", nil)
		if got := before.Body.String(); before.Code != http.StatusOK || !strings.Contains(got, "local shell before") {
			t.Fatalf("GET / before status = %d body = %q, want local shell before", before.Code, got)
		}

		indexPath := filepath.Join(distDir, "index.html")
		if err := os.WriteFile(indexPath, []byte("<!doctype html><div>local shell after</div>"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(index.html rewrite) error = %v", err)
		}

		after := performRequest(t, engine, http.MethodGet, "/", nil)
		if got := after.Body.String(); after.Code != http.StatusOK || !strings.Contains(got, "local shell after") {
			t.Fatalf("GET / after status = %d body = %q, want local shell after", after.Code, got)
		}
	})
}

func TestStaticRoutesRejectMissingLocalWebDistIndex(t *testing.T) {
	t.Run("Should fail clearly when AGH_WEB_DIST_DIR has no index", func(t *testing.T) {
		distDir := t.TempDir()
		t.Setenv(webDistDirEnvVar, distDir)

		_, err := newStaticFS()
		if err == nil {
			t.Fatal("newStaticFS() error = nil, want missing index error")
		}
		message := err.Error()
		for _, want := range []string{webDistDirEnvVar, "index.html"} {
			if !strings.Contains(message, want) {
				t.Fatalf("newStaticFS() error = %q, want %q", message, want)
			}
		}
	})
}

func firstEmbeddedAsset(t *testing.T) (string, []byte) {
	t.Helper()

	staticFS := mustStaticFS(t)
	entries, err := fs.ReadDir(staticFS, "assets")
	if err != nil {
		t.Fatalf("fs.ReadDir(assets) error = %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch ext := path.Ext(entry.Name()); ext {
		case ".js", ".css":
			assetPath := path.Join("assets", entry.Name())
			data, readErr := fs.ReadFile(staticFS, assetPath)
			if readErr != nil {
				t.Fatalf("fs.ReadFile(%s) error = %v", assetPath, readErr)
			}
			return "/" + assetPath, data
		}
	}

	t.Fatal("expected at least one embedded .js or .css asset")
	return "", nil
}

func writeLocalStaticDist(t *testing.T, indexMarker string, assets map[string]string) string {
	t.Helper()

	distDir := t.TempDir()
	index := "<!doctype html><html><body><div>" + indexMarker + "</div></body></html>"
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatalf("os.WriteFile(index.html) error = %v", err)
	}
	for name, body := range assets {
		path := filepath.Join(distDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%s) error = %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%s) error = %v", name, err)
		}
	}
	return distDir
}
