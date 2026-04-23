package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	webassets "github.com/compozy/compozy/web"
)

type staticHandler struct {
	staticFS  fs.FS
	startedAt time.Time

	etagMu sync.RWMutex
	etags  map[string]string
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
		etags:     make(map[string]string),
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
	clean := strings.TrimPrefix(assetPath, "/")
	data, err := fs.ReadFile(h.staticFS, clean)
	if err != nil {
		respondStaticNotFound(c)
		return
	}

	etag := h.etagFor(clean, data)
	applyStaticCacheHeaders(c, clean, etag)
	if match := c.Request.Header.Get("If-None-Match"); match != "" && etagMatches(match, etag) {
		c.Status(http.StatusNotModified)
		return
	}

	http.ServeContent(c.Writer, c.Request, path.Base(assetPath), h.startedAt, bytes.NewReader(data))
}

func (h *staticHandler) etagFor(assetPath string, data []byte) string {
	h.etagMu.RLock()
	if tag, ok := h.etags[assetPath]; ok {
		h.etagMu.RUnlock()
		return tag
	}
	h.etagMu.RUnlock()

	sum := sha256.Sum256(data)
	tag := fmt.Sprintf("%q", hex.EncodeToString(sum[:16]))

	h.etagMu.Lock()
	h.etags[assetPath] = tag
	h.etagMu.Unlock()
	return tag
}

func applyStaticCacheHeaders(c *gin.Context, assetPath string, etag string) {
	header := c.Writer.Header()
	header.Set("ETag", etag)
	header.Set("X-Content-Type-Options", "nosniff")
	if isImmutableAsset(assetPath) {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	header.Set("Cache-Control", "no-cache")
}

func isImmutableAsset(assetPath string) bool {
	clean := strings.TrimPrefix(strings.TrimSpace(assetPath), "/")
	if clean == "" {
		return false
	}
	if strings.HasPrefix(clean, "assets/") {
		return true
	}
	return false
}

func etagMatches(ifNoneMatch string, etag string) bool {
	if ifNoneMatch == "*" {
		return true
	}
	candidates := strings.Split(ifNoneMatch, ",")
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		trimmed = strings.TrimPrefix(trimmed, "W/")
		if trimmed == etag {
			return true
		}
	}
	return false
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
