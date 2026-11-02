package httpapi

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestStaticRoutesServeEmbeddedIndexForRootAndDeepLinks(t *testing.T) {
	t.Run("Should serve the revalidated SPA shell for the root and deep links", func(t *testing.T) {
		t.Setenv(webDistDirEnvVar, "")

		homePaths := newTestHomePaths(t)
		engine := newTestRouter(t, newTestHandlers(t, &stubSessionManager{}, stubObserver{}, homePaths))

		rootResp := performRequest(t, engine, http.MethodGet, "/", nil)
		if rootResp.Code != http.StatusOK {
			t.Fatalf("GET / status = %d, want %d; body=%s", rootResp.Code, http.StatusOK, rootResp.Body.String())
		}
		if !strings.Contains(rootResp.Body.String(), `<div id="app"></div>`) {
			t.Fatalf("GET / body = %q, want SPA shell", rootResp.Body.String())
		}
		const expectedContentSecurityPolicy = "default-src 'self'; script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; " +
			"connect-src 'self'; worker-src 'self' blob:; frame-src 'none'; frame-ancestors 'none'; " +
			"object-src 'none'; base-uri 'self'; form-action 'self'"
		if got := rootResp.Header().Get("Content-Security-Policy"); got != expectedContentSecurityPolicy {
			t.Fatalf("GET / Content-Security-Policy = %q, want exact production policy", got)
		}
		for _, forbidden := range []string{"script-src 'self' 'unsafe-inline'", "'unsafe-eval'"} {
			if strings.Contains(rootResp.Header().Get("Content-Security-Policy"), forbidden) {
				t.Fatalf("GET / Content-Security-Policy contains forbidden source %q", forbidden)
			}
		}
		// A cached shell must be revalidated, or an updated runtime boots the previous bundle (#742).
		if got := rootResp.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("GET / Cache-Control = %q, want no-cache", got)
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
		if got := deepLinkResp.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("GET /session/sess-001 Cache-Control = %q, want no-cache", got)
		}

		loopTaskPath := "/tasks/loop.looprun-001.g1.node.review.0"
		loopTaskResp := performRequestWithHeaders(t, engine, http.MethodGet, loopTaskPath, nil, map[string]string{
			"Accept": "text/html,application/xhtml+xml",
		})
		if loopTaskResp.Code != http.StatusOK {
			t.Fatalf(
				"GET %s status = %d, want %d; body=%s",
				loopTaskPath,
				loopTaskResp.Code,
				http.StatusOK,
				loopTaskResp.Body.String(),
			)
		}
		if !strings.Contains(loopTaskResp.Body.String(), `<div id="app"></div>`) {
			t.Fatalf("GET %s body = %q, want SPA shell", loopTaskPath, loopTaskResp.Body.String())
		}
	})
}

func TestStaticRoutesServeEmbeddedAssets(t *testing.T) {
	t.Setenv(webDistDirEnvVar, "")

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, &stubSessionManager{}, stubObserver{}, homePaths))

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

// Invariant: every listener revalidates the shell and unversioned files, so an updated runtime
// never boots the previous release's bundle from a cache (#742); fingerprinted build output stays
// cacheable for good. Owner: the shared static route behind every surface's NoRoute fallback.
func TestStaticRoutesCachePolicy(t *testing.T) {
	// not parallel: t.Setenv selects the process-wide local Web bundle.
	t.Run("Should revalidate the shell on every listener surface and keep fingerprinted assets", func(t *testing.T) {
		const fingerprinted = "assets/session-thread-Bg_R4-Fj.js"
		distDir := writeLocalStaticDist(t, "cache policy shell", map[string]string{
			fingerprinted:                   "console.log('chunk');",
			"assets/emojibase/en/data.json": "[]",
			"theme-boot.js":                 "void 0;",
		})
		t.Setenv(webDistDirEnvVar, distDir)
		homePaths := newTestHomePaths(t)
		want := map[string]string{
			"/":                              "no-cache",
			"/session/sess-001":              "no-cache",
			"/theme-boot.js":                 "no-cache",
			"/assets/emojibase/en/data.json": "no-cache",
			"/" + fingerprinted:              "public, max-age=31536000, immutable",
		}
		// Every surface that serves the Web; public ingress carries webhooks only.
		for _, surfaceSet := range []SurfaceSet{
			SurfaceSetLocal,
			SurfaceSetPrivate,
			SurfaceSetPublicOperator,
			SurfaceSetPublicCombined,
		} {
			router := gin.New()
			RegisterSurfaceRoutes(
				router,
				newTestHandlers(t, &stubSessionManager{}, stubObserver{}, homePaths),
				surfaceSet,
			)
			for requestPath, cacheControl := range want {
				resp := performRequestWithHeaders(t, router, http.MethodGet, requestPath, nil, map[string]string{
					"Accept": "text/html,application/xhtml+xml",
				})
				if resp.Code != http.StatusOK {
					t.Fatalf("%s GET %s status = %d; body=%s", surfaceSet, requestPath, resp.Code, resp.Body.String())
				}
				if got := resp.Header().Get("Cache-Control"); got != cacheControl {
					t.Fatalf("%s GET %s Cache-Control = %q, want %q", surfaceSet, requestPath, got, cacheControl)
				}
			}
		}
	})
}

func TestStaticRoutesDoNotFallbackForMissingAssetsOrAPIRoutes(t *testing.T) {
	t.Setenv(webDistDirEnvVar, "")

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, &stubSessionManager{}, stubObserver{}, homePaths))

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
	t.Run("Should serve COMPOZY_WEB_DIST_DIR index and assets from disk", func(t *testing.T) {
		distDir := writeLocalStaticDist(t, "local shell one", map[string]string{
			"assets/local.js": "console.log('local asset');",
		})
		t.Setenv(webDistDirEnvVar, distDir)

		homePaths := newTestHomePaths(t)
		engine := newTestRouter(t, newTestHandlers(t, &stubSessionManager{}, stubObserver{}, homePaths))

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
	// not parallel: t.Setenv selects the process-wide local Web bundle.
	for _, tt := range []struct {
		name        string
		requestPath string
		asset       string
		before      string
		after       string
	}{
		{
			name:        "Should revalidate rewritten local index without restarting",
			requestPath: "/",
			asset:       "index.html",
			before:      "local shell before",
			after:       "<!doctype html><div>local shell after</div>",
		},
		{
			name:        "Should revalidate a deep link after the local index changes",
			requestPath: "/jobs/job-001",
			asset:       "index.html",
			before:      "local shell before",
			after:       "<!doctype html><div>local shell after</div>",
		},
		{
			name:        "Should revalidate rewritten local assets without restarting",
			requestPath: "/assets/local.js",
			asset:       "assets/local.js",
			before:      "console.log('before');",
			after:       "console.log('after');",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			distDir := writeLocalStaticDist(t, "local shell before", map[string]string{
				"assets/local.js": "console.log('before');",
			})
			t.Setenv(webDistDirEnvVar, distDir)
			assetPath := filepath.Join(distDir, filepath.FromSlash(tt.asset))
			modified := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
			if err := os.Chtimes(assetPath, modified, modified); err != nil {
				t.Fatalf("os.Chtimes before rewrite error = %v", err)
			}

			homePaths := newTestHomePaths(t)
			engine := newTestRouter(t, newTestHandlers(t, &stubSessionManager{}, stubObserver{}, homePaths))
			before := performRequest(t, engine, http.MethodGet, tt.requestPath, nil)
			if got := before.Body.String(); before.Code != http.StatusOK || !strings.Contains(got, tt.before) {
				t.Fatalf("GET %s before status = %d body = %q, want %q", tt.requestPath, before.Code, got, tt.before)
			}
			validator := before.Header().Get("Last-Modified")
			if validator == "" {
				t.Fatal("Last-Modified is empty, want conditional request validator")
			}
			headers := map[string]string{"If-Modified-Since": validator}
			unchanged := performRequestWithHeaders(t, engine, http.MethodGet, tt.requestPath, nil, headers)
			if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 {
				t.Fatalf(
					"unchanged status = %d body = %q, want 304 with no body",
					unchanged.Code,
					unchanged.Body.String(),
				)
			}

			if err := os.WriteFile(assetPath, []byte(tt.after), 0o644); err != nil {
				t.Fatalf("os.WriteFile rewrite error = %v", err)
			}
			modified = modified.Add(time.Hour)
			if err := os.Chtimes(assetPath, modified, modified); err != nil {
				t.Fatalf("os.Chtimes after rewrite error = %v", err)
			}
			after := performRequestWithHeaders(t, engine, http.MethodGet, tt.requestPath, nil, headers)
			if got := after.Body.String(); after.Code != http.StatusOK || got != tt.after {
				t.Fatalf("GET %s after status = %d body = %q, want %q", tt.requestPath, after.Code, got, tt.after)
			}
		})
	}
}

func TestStaticRoutesRejectMissingLocalWebDistIndex(t *testing.T) {
	t.Run("Should fail clearly when COMPOZY_WEB_DIST_DIR has no index", func(t *testing.T) {
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
