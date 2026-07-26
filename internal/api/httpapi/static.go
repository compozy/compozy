package httpapi

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	webassets "github.com/compozy/agh-web-assets"
	"github.com/gin-gonic/gin"
)

const webDistDirEnvVar = "AGH_WEB_DIST_DIR"

type staticSourceFS struct {
	fs     fs.FS
	source string
}

func newStaticFS() (staticSourceFS, error) {
	if override := strings.TrimSpace(os.Getenv(webDistDirEnvVar)); override != "" {
		return newLocalStaticFS(override)
	}
	return newEmbeddedStaticFS()
}

func newEmbeddedStaticFS() (staticSourceFS, error) {
	staticFS, err := fs.Sub(webassets.DistFS, webassets.DistDir)
	if err != nil {
		return staticSourceFS{}, fmt.Errorf("embedded web bundle directory %q: %w", webassets.DistDir, err)
	}
	if _, err := fs.Stat(staticFS, "index.html"); err != nil {
		return staticSourceFS{}, fmt.Errorf("embedded web bundle missing index.html: %w", err)
	}
	return staticSourceFS{
		fs:     staticFS,
		source: fmt.Sprintf("embedded %s/%s", "github.com/compozy/agh-web-assets", webassets.DistDir),
	}, nil
}

func newLocalStaticFS(dir string) (staticSourceFS, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return staticSourceFS{}, fmt.Errorf("%s resolve %q: %w", webDistDirEnvVar, dir, err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return staticSourceFS{}, fmt.Errorf("%s stat %q: %w", webDistDirEnvVar, absDir, err)
	}
	if !info.IsDir() {
		return staticSourceFS{}, fmt.Errorf("%s %q is not a directory", webDistDirEnvVar, absDir)
	}
	indexPath := filepath.Join(absDir, "index.html")
	indexInfo, err := os.Stat(indexPath)
	if err != nil {
		return staticSourceFS{}, fmt.Errorf(
			"%s missing readable index.html at %q: %w",
			webDistDirEnvVar,
			indexPath,
			err,
		)
	}
	if indexInfo.IsDir() {
		return staticSourceFS{}, fmt.Errorf("%s index.html at %q is a directory", webDistDirEnvVar, indexPath)
	}
	file, err := os.Open(indexPath)
	if err != nil {
		return staticSourceFS{}, fmt.Errorf("%s open index.html at %q: %w", webDistDirEnvVar, indexPath, err)
	}
	if err := file.Close(); err != nil {
		return staticSourceFS{}, fmt.Errorf("%s close index.html at %q: %w", webDistDirEnvVar, indexPath, err)
	}
	return staticSourceFS{
		fs:     os.DirFS(absDir),
		source: webDistDirEnvVar + "=" + absDir,
	}, nil
}

func (h *Handlers) serveStaticRoute(c *gin.Context) {
	if c == nil {
		return
	}
	if h == nil || h.staticFS == nil {
		respondNotFound(c)
		return
	}

	requestPath := normalizedRequestPath(c.Request.URL.Path)
	if isStaticBypassPath(requestPath) || !isStaticRequestMethod(c.Request.Method) {
		respondNotFound(c)
		return
	}

	if asset, ok := h.resolveStaticAsset(requestPath); ok {
		h.serveAsset(c, asset)
		return
	}
	if shouldServeSPAIndex(requestPath) {
		h.serveAsset(c, "index.html")
		return
	}

	respondNotFound(c)
}

func (h *Handlers) resolveStaticAsset(requestPath string) (string, bool) {
	if h == nil || h.staticFS == nil {
		return "", false
	}

	asset := strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(requestPath)), "/")
	if asset == "." || asset == "" {
		return "index.html", true
	}
	if info, err := fs.Stat(h.staticFS, asset); err == nil && !info.IsDir() {
		return asset, true
	}

	return "", false
}

func (h *Handlers) serveAsset(c *gin.Context, asset string) {
	cleanAsset := strings.TrimPrefix(asset, "/")
	file, err := h.staticFS.Open(cleanAsset)
	if err != nil {
		respondNotFound(c)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			h.Logger.Debug("httpapi: close static asset failed", "asset", cleanAsset, "error", err)
		}
	}()

	if seeker, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, path.Base(asset), h.StartedAt, seeker)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		respondNotFound(c)
		return
	}
	http.ServeContent(c.Writer, c.Request, path.Base(asset), h.StartedAt, bytes.NewReader(data))
}

func normalizedRequestPath(rawPath string) string {
	clean := path.Clean("/" + strings.TrimSpace(rawPath))
	if clean == "." {
		return "/"
	}
	return clean
}

func isStaticBypassPath(requestPath string) bool {
	return requestPath == "/api" ||
		strings.HasPrefix(requestPath, "/api/") ||
		requestPath == "/ws" ||
		strings.HasPrefix(requestPath, "/ws/")
}

func isStaticRequestMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}

func shouldServeSPAIndex(requestPath string) bool {
	if requestPath == "/" {
		return true
	}

	lastSegment := path.Base(requestPath)
	return !strings.Contains(lastSegment, ".")
}

func respondNotFound(c *gin.Context) {
	c.String(http.StatusNotFound, "404 page not found")
}
