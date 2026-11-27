package httpapi

import (
	"net"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
)

func isAppTokenMutationRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func requestSourceIP(request *http.Request) string {
	if request == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	if remote := strings.TrimSpace(request.RemoteAddr); remote != "" {
		return remote
	}
	return "unknown"
}

func isAppTokenStreamRequest(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	path := strings.TrimSuffix(request.URL.Path, "/")
	if request.Method == http.MethodPost {
		return isSessionPromptStreamPath(path)
	}
	if request.Method != http.MethodGet {
		return false
	}
	if strings.HasSuffix(path, "/stream") || strings.HasSuffix(path, "/catalog-stream") ||
		strings.HasSuffix(path, "/log-tail") {
		return true
	}
	if strings.Contains(path, "/loop-runs/") && strings.HasSuffix(path, "/events") {
		return true
	}
	if !strings.Contains(path, "/extensions/") || !strings.HasSuffix(path, "/logs") {
		return false
	}
	follow, err := core.ParseOptionalBool(request.URL.Query().Get("follow"))
	return err == nil && follow
}

func isSessionPromptStreamPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 6 && parts[0] == "api" && parts[1] == "workspaces" &&
		strings.TrimSpace(parts[2]) != "" && parts[3] == "sessions" &&
		strings.TrimSpace(parts[4]) != "" && parts[5] == "prompt"
}
