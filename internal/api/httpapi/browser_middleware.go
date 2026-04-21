package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const csrfCookieLifetime = 24 * time.Hour

const localhostHost = "localhost"

func (s *Server) hostValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.hostAllowed(c.Request.Host) {
			core.RespondError(c, core.NewProblem(
				http.StatusForbidden,
				"host_invalid",
				"request host is not allowed",
				map[string]any{"host": strings.TrimSpace(c.Request.Host)},
				nil,
			))
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) originValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}
		if !s.originAllowed(origin) {
			core.RespondError(c, core.NewProblem(
				http.StatusForbidden,
				"origin_invalid",
				"request origin is not allowed",
				map[string]any{"origin": origin},
				nil,
			))
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) activeWorkspaceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		fullPath := c.FullPath()
		if fullPath == "" && c.Request != nil && c.Request.URL != nil {
			fullPath = c.Request.URL.Path
		}
		if !requiresActiveWorkspace(fullPath) {
			c.Next()
			return
		}

		workspaceID := strings.TrimSpace(c.GetHeader(core.HeaderActiveWorkspaceID))
		if workspaceID == "" {
			c.Next()
			return
		}
		if s.handlers == nil || s.handlers.Workspaces == nil {
			core.RespondError(c, core.NewProblem(
				http.StatusServiceUnavailable,
				"service_unavailable",
				"workspace service unavailable",
				nil,
				nil,
			))
			c.Abort()
			return
		}

		if _, err := s.handlers.Workspaces.Get(c.Request.Context(), workspaceID); err != nil {
			if errors.Is(err, globaldb.ErrWorkspaceNotFound) {
				core.RespondError(c, core.NewProblem(
					http.StatusPreconditionFailed,
					"workspace_context_stale",
					"active workspace context is stale",
					map[string]any{"workspace": workspaceID},
					err,
				))
			} else {
				core.RespondError(c, err)
			}
			c.Abort()
			return
		}

		c.Request = c.Request.WithContext(core.WithActiveWorkspaceID(c.Request.Context(), workspaceID))
		c.Next()
	}
}

func (s *Server) csrfMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := ensureCSRFCookie(c)
		if err != nil {
			core.RespondError(c, core.NewProblem(
				http.StatusInternalServerError,
				"csrf_unavailable",
				"failed to prepare csrf token",
				nil,
				err,
			))
			c.Abort()
			return
		}
		c.Header(core.HeaderCSRF, token)

		if !isMutatingMethod(c.Request.Method) || !looksLikeBrowserRequest(c) {
			c.Next()
			return
		}

		headerToken := strings.TrimSpace(c.GetHeader(core.HeaderCSRF))
		if headerToken == "" {
			core.RespondError(c, core.NewProblem(
				http.StatusForbidden,
				"csrf_missing",
				"csrf token is required",
				map[string]any{"header": core.HeaderCSRF},
				nil,
			))
			c.Abort()
			return
		}
		if headerToken != token {
			core.RespondError(c, core.NewProblem(
				http.StatusForbidden,
				"csrf_invalid",
				"csrf token is invalid",
				map[string]any{"header": core.HeaderCSRF},
				nil,
			))
			c.Abort()
			return
		}

		c.Next()
	}
}

func ensureCSRFCookie(c *gin.Context) (string, error) {
	if c == nil {
		return "", errors.New("csrf context is required")
	}

	token := ""
	if cookie, err := c.Request.Cookie(core.CookieCSRF); err == nil {
		token = strings.TrimSpace(cookie.Value)
	} else if err != nil && !errors.Is(err, http.ErrNoCookie) {
		return "", err
	}
	if token == "" {
		generated, err := newCSRFToken()
		if err != nil {
			return "", err
		}
		token = generated
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     core.CookieCSRF,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(csrfCookieLifetime / time.Second),
	})
	return token, nil
}

func newCSRFToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func requiresActiveWorkspace(fullPath string) bool {
	normalized := strings.TrimSpace(fullPath)
	switch {
	case normalized == "/api/ui/dashboard":
		return true
	case normalized == "/api/tasks":
		return true
	case strings.HasPrefix(normalized, "/api/tasks/") &&
		normalized != "/api/tasks/:slug/validate" &&
		!strings.HasSuffix(normalized, "/validate"):
		return true
	case normalized == "/api/sync":
		return true
	case strings.HasPrefix(normalized, "/api/reviews/") &&
		normalized != "/api/reviews/:slug/fetch" &&
		!strings.HasSuffix(normalized, "/fetch"):
		return true
	default:
		return false
	}
}

func isMutatingMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func looksLikeBrowserRequest(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if strings.TrimSpace(c.GetHeader("Origin")) != "" {
		return true
	}
	for _, header := range []string{"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest"} {
		if strings.TrimSpace(c.GetHeader(header)) != "" {
			return true
		}
	}
	return false
}

func (s *Server) originAllowed(rawOrigin string) bool {
	originURL, err := url.Parse(strings.TrimSpace(rawOrigin))
	if err != nil {
		return false
	}
	if !strings.EqualFold(originURL.Scheme, "http") {
		return false
	}
	return s.hostAllowed(originURL.Host)
}

func (s *Server) hostAllowed(authority string) bool {
	host, port := splitAuthority(authority)
	if host == "" {
		return false
	}
	expectedPort := s.Port()
	if expectedPort > 0 {
		if port == "" {
			return false
		}
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort != expectedPort {
			return false
		}
	}
	return s.allowedHostnames()[host]
}

func (s *Server) allowedHostnames() map[string]bool {
	allowed := map[string]bool{
		localhostHost: true,
		defaultHost:   true,
	}
	bindHost := normalizeAuthorityHost(s.host)
	if bindHost != "" {
		allowed[bindHost] = true
		if bindHost == localhostHost {
			allowed[defaultHost] = true
		}
		if ip := net.ParseIP(bindHost); ip != nil && ip.IsLoopback() {
			allowed[localhostHost] = true
		}
	}
	return allowed
}

func splitAuthority(authority string) (string, string) {
	trimmed := strings.TrimSpace(authority)
	if trimmed == "" {
		return "", ""
	}
	host, port, err := net.SplitHostPort(trimmed)
	if err == nil {
		return normalizeAuthorityHost(host), strings.TrimSpace(port)
	}
	if strings.Contains(err.Error(), "missing port in address") {
		return normalizeAuthorityHost(trimmed), ""
	}
	return "", ""
}

func normalizeAuthorityHost(host string) string {
	trimmed := strings.TrimSpace(strings.Trim(host, "[]"))
	trimmed = strings.TrimSuffix(trimmed, ".")
	return strings.ToLower(trimmed)
}
