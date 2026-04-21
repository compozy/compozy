package httpapi

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

func TestNewStaticFSLoadsEmbeddedBundle(t *testing.T) {
	t.Parallel()

	staticFS, err := newStaticFS()
	if err != nil {
		t.Fatalf("newStaticFS() error = %v", err)
	}

	indexHTML, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}
	if !strings.Contains(string(indexHTML), `<div id="app"></div>`) {
		t.Fatalf("index.html = %q, want SPA shell", string(indexHTML))
	}
}

func TestStaticFSFromRootRequiresIndexHTML(t *testing.T) {
	t.Parallel()

	_, err := staticFSFromRoot(fstest.MapFS{
		"dist/assets/app.js": &fstest.MapFile{Data: []byte("console.log('ok')")},
	}, "dist")
	if err == nil || !strings.Contains(err.Error(), "missing index.html") {
		t.Fatalf("staticFSFromRoot() error = %v, want missing index.html", err)
	}
}

func TestStaticRoutesServeEmbeddedIndexForRootAndDeepLinks(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := newStaticTestEngine(t)

	rootResponse := performStaticRequest(t, engine, http.MethodGet, "/")
	if rootResponse.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d; body=%s", rootResponse.Code, http.StatusOK, rootResponse.Body.String())
	}
	if !strings.Contains(rootResponse.Body.String(), `<div id="app"></div>`) {
		t.Fatalf("GET / body = %q, want SPA shell", rootResponse.Body.String())
	}

	deepLinkResponse := performStaticRequest(t, engine, http.MethodGet, "/workflows/daemon/tasks/task_08")
	if deepLinkResponse.Code != http.StatusOK {
		t.Fatalf(
			"GET deep link status = %d, want %d; body=%s",
			deepLinkResponse.Code,
			http.StatusOK,
			deepLinkResponse.Body.String(),
		)
	}
	if !strings.Contains(deepLinkResponse.Body.String(), `<div id="app"></div>`) {
		t.Fatalf("GET deep link body = %q, want SPA shell", deepLinkResponse.Body.String())
	}
}

func TestStaticRoutesServeEmbeddedAssets(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	staticFS := mustStaticFS(t)
	engine := newStaticTestEngine(t)

	requestPath, expectedBody := firstEmbeddedAsset(t, staticFS)
	response := performStaticRequest(t, engine, http.MethodGet, requestPath)
	if response.Code != http.StatusOK {
		t.Fatalf(
			"GET %s status = %d, want %d; body=%s",
			requestPath,
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}
	if got := response.Body.String(); got != string(expectedBody) {
		t.Fatalf("GET %s body mismatch", requestPath)
	}
	if strings.Contains(response.Body.String(), "<!doctype html>") {
		t.Fatalf("GET %s returned SPA HTML instead of asset payload", requestPath)
	}
}

func TestStaticRoutesBypassAPIAndMissingAssets(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := newStaticTestEngine(t)

	missingAssetResponse := performStaticRequest(t, engine, http.MethodGet, "/assets/does-not-exist.js")
	if missingAssetResponse.Code != http.StatusNotFound {
		t.Fatalf(
			"GET missing asset status = %d, want %d; body=%s",
			missingAssetResponse.Code,
			http.StatusNotFound,
			missingAssetResponse.Body.String(),
		)
	}
	if strings.Contains(missingAssetResponse.Body.String(), "<!doctype html>") {
		t.Fatalf("GET missing asset body = %q, want plain 404", missingAssetResponse.Body.String())
	}

	directoryResponse := performStaticRequest(t, engine, http.MethodGet, "/assets")
	if directoryResponse.Code != http.StatusNotFound {
		t.Fatalf(
			"GET /assets status = %d, want %d; body=%s",
			directoryResponse.Code,
			http.StatusNotFound,
			directoryResponse.Body.String(),
		)
	}
	if strings.Contains(directoryResponse.Body.String(), "<!doctype html>") {
		t.Fatalf("GET /assets body = %q, want plain 404", directoryResponse.Body.String())
	}

	missingAPIResponse := performStaticRequest(t, engine, http.MethodGet, "/api/missing")
	if missingAPIResponse.Code != http.StatusNotFound {
		t.Fatalf(
			"GET /api/missing status = %d, want %d; body=%s",
			missingAPIResponse.Code,
			http.StatusNotFound,
			missingAPIResponse.Body.String(),
		)
	}
	if strings.Contains(missingAPIResponse.Body.String(), "<!doctype html>") {
		t.Fatalf("GET /api/missing body = %q, want plain 404", missingAPIResponse.Body.String())
	}
}

func newStaticTestEngine(t *testing.T) *gin.Engine {
	t.Helper()

	engine := gin.New()
	staticHandler := newStaticHandler(mustStaticFS(t), time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC))
	RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{TransportName: "test"}), staticHandler.serve)
	return engine
}

func performStaticRequest(t *testing.T, engine http.Handler, method string, target string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	request.Host = "example.com"
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func mustStaticFS(t *testing.T) fs.FS {
	t.Helper()

	staticFS, err := newStaticFS()
	if err != nil {
		t.Fatalf("newStaticFS() error = %v", err)
	}
	return staticFS
}

func firstEmbeddedAsset(t *testing.T, staticFS fs.FS) (string, []byte) {
	t.Helper()

	entries, err := fs.ReadDir(staticFS, "assets")
	if err != nil {
		t.Fatalf("ReadDir(assets) error = %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch path.Ext(entry.Name()) {
		case ".js", ".css":
			assetPath := path.Join("assets", entry.Name())
			assetBody, err := fs.ReadFile(staticFS, assetPath)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", assetPath, err)
			}
			return "/" + assetPath, assetBody
		}
	}

	t.Fatal("expected at least one embedded .js or .css asset")
	return "", nil
}
