package httpapi

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	webassets "github.com/compozy/compozy/web"
)

type staticHandler struct {
	staticFS  fs.FS
	startedAt time.Time
}

func newStaticFS() (fs.FS, error) {
	return staticFSFromRoot(webassets.DistFS, "dist")
}

func staticFSFromRoot(root fs.FS, dir string) (fs.FS, error) {
	staticFS, err := fs.Sub(root, dir)
	if err != nil {
		return nil, fmt.Errorf("embedded bundle directory %q: %w", dir, err)
	}
	if _, err := fs.Stat(staticFS, "index.html"); err != nil {
		return nil, fmt.Errorf("embedded bundle missing index.html: %w", err)
	}
	return staticFS, nil
}

func newStaticHandler(staticFS fs.FS, startedAt time.Time) *staticHandler {
	if staticFS == nil {
		return nil
	}
	return &staticHandler{
		staticFS:  staticFS,
		startedAt: startedAt,
	}
}

func (h *staticHandler) serve(c *gin.Context) {
	if c == nil {
		return
	}
	if h == nil || h.staticFS == nil {
		respondStaticNotFound(c)
		return
	}

	requestPath := normalizedRequestPath(c.Request.URL.Path)
	if isStaticBypassPath(requestPath) || !isStaticRequestMethod(c.Request.Method) {
		respondStaticNotFound(c)
		return
	}

	if assetPath, ok := h.resolveAsset(requestPath); ok {
		if assetPath == "" {
			respondStaticNotFound(c)
			return
		}
		h.serveAsset(c, assetPath)
		return
	}
	if shouldServeSPAIndex(requestPath) {
		h.serveAsset(c, "index.html")
		return
	}

	respondStaticNotFound(c)
}

func (h *staticHandler) resolveAsset(requestPath string) (string, bool) {
	if h == nil || h.staticFS == nil {
		return "", false
	}

	assetPath := strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(requestPath)), "/")
	if assetPath == "." || assetPath == "" {
		return "index.html", true
	}
	info, err := fs.Stat(h.staticFS, assetPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return "", true
	}
	return assetPath, true
}

func (h *staticHandler) serveAsset(c *gin.Context, assetPath string) {
	data, err := fs.ReadFile(h.staticFS, strings.TrimPrefix(assetPath, "/"))
	if err != nil {
		respondStaticNotFound(c)
		return
	}

	http.ServeContent(c.Writer, c.Request, path.Base(assetPath), h.startedAt, bytes.NewReader(data))
}

func normalizedRequestPath(rawPath string) string {
	clean := path.Clean("/" + strings.TrimSpace(rawPath))
	if clean == "." {
		return "/"
	}
	return clean
}

func isStaticBypassPath(requestPath string) bool {
	return requestPath == "/api" || strings.HasPrefix(requestPath, "/api/")
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
	return !strings.Contains(path.Base(requestPath), ".")
}

func respondStaticNotFound(c *gin.Context) {
	c.String(http.StatusNotFound, "404 page not found")
}
