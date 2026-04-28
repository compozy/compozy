package contract

import (
	"net/http"
	"net/url"
	"strings"
)

type RouteSpec struct {
	Method       string
	Path         string
	ResponseType string
	TimeoutClass TimeoutClass
}

var RouteInventory = []RouteSpec{
	{
		Method:       http.MethodGet,
		Path:         "/api/daemon/status",
		ResponseType: "DaemonStatusResponse",
		TimeoutClass: TimeoutProbe,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/daemon/health",
		ResponseType: "DaemonHealthResponse",
		TimeoutClass: TimeoutProbe,
	},
	{Method: http.MethodGet, Path: "/api/daemon/metrics", ResponseType: "text/plain", TimeoutClass: TimeoutRead},
	{
		Method:       http.MethodPost,
		Path:         "/api/daemon/stop",
		ResponseType: "MutationAcceptedResponse",
		TimeoutClass: TimeoutMutate,
	},
	{Method: http.MethodPost, Path: "/api/workspaces", ResponseType: "WorkspaceResponse", TimeoutClass: TimeoutMutate},
	{Method: http.MethodGet, Path: "/api/workspaces", ResponseType: "WorkspaceListResponse", TimeoutClass: TimeoutRead},
	{
		Method:       http.MethodPost,
		Path:         "/api/workspaces/sync",
		ResponseType: "WorkspaceSyncResult",
		TimeoutClass: TimeoutLongMutate,
	},
	{Method: http.MethodGet, Path: "/api/workspaces/:id", ResponseType: "WorkspaceResponse", TimeoutClass: TimeoutRead},
	{
		Method:       http.MethodPatch,
		Path:         "/api/workspaces/:id",
		ResponseType: "WorkspaceResponse",
		TimeoutClass: TimeoutMutate,
	},
	{
		Method:       http.MethodDelete,
		Path:         "/api/workspaces/:id",
		ResponseType: "MutationAcceptedResponse",
		TimeoutClass: TimeoutMutate,
	},
	{
		Method:       http.MethodPost,
		Path:         "/api/workspaces/resolve",
		ResponseType: "WorkspaceResponse",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/ui/dashboard",
		ResponseType: "DashboardPayload",
		TimeoutClass: TimeoutRead,
	},
	{Method: http.MethodGet, Path: "/api/tasks", ResponseType: "TaskWorkflowListResponse", TimeoutClass: TimeoutRead},
	{Method: http.MethodGet, Path: "/api/tasks/:slug", ResponseType: "TaskWorkflowResponse", TimeoutClass: TimeoutRead},
	{
		Method:       http.MethodGet,
		Path:         "/api/tasks/:slug/spec",
		ResponseType: "WorkflowSpecDocument",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/tasks/:slug/memory",
		ResponseType: "WorkflowMemoryIndex",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/tasks/:slug/memory/files/:file_id",
		ResponseType: "MarkdownDocument",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/tasks/:slug/board",
		ResponseType: "TaskBoardPayload",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/tasks/:slug/items",
		ResponseType: "TaskItemsResponse",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/tasks/:slug/items/:task_id",
		ResponseType: "TaskDetailPayload",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodPost,
		Path:         "/api/tasks/:slug/validate",
		ResponseType: "ValidationResponse",
		TimeoutClass: TimeoutMutate,
	},
	{
		Method:       http.MethodPost,
		Path:         "/api/tasks/:slug/runs",
		ResponseType: "RunResponse",
		TimeoutClass: TimeoutLongMutate,
	},
	{
		Method:       http.MethodPost,
		Path:         "/api/tasks/:slug/archive",
		ResponseType: "ArchiveResponse",
		TimeoutClass: TimeoutMutate,
	},
	{
		Method:       http.MethodPost,
		Path:         "/api/reviews/:slug/fetch",
		ResponseType: "ReviewFetchResponse",
		TimeoutClass: TimeoutLongMutate,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/reviews/:slug",
		ResponseType: "ReviewSummaryResponse",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/reviews/:slug/rounds/:round",
		ResponseType: "ReviewRoundResponse",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/reviews/:slug/rounds/:round/issues",
		ResponseType: "ReviewIssuesResponse",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/reviews/:slug/rounds/:round/issues/:issue_id",
		ResponseType: "ReviewDetailPayload",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodPost,
		Path:         "/api/reviews/:slug/rounds/:round/runs",
		ResponseType: "RunResponse",
		TimeoutClass: TimeoutLongMutate,
	},
	{Method: http.MethodGet, Path: "/api/runs", ResponseType: "RunListResponse", TimeoutClass: TimeoutRead},
	{Method: http.MethodGet, Path: "/api/runs/:run_id", ResponseType: "RunResponse", TimeoutClass: TimeoutRead},
	{
		Method:       http.MethodGet,
		Path:         "/api/runs/:run_id/snapshot",
		ResponseType: "RunSnapshotResponse",
		TimeoutClass: TimeoutRead,
	},
	{
		Method:       http.MethodGet,
		Path:         "/api/runs/:run_id/events",
		ResponseType: "RunEventPageResponse",
		TimeoutClass: TimeoutRead,
	},
	{Method: http.MethodGet, Path: "/api/runs/:run_id/stream", ResponseType: "sse", TimeoutClass: TimeoutStream},
	{
		Method:       http.MethodPost,
		Path:         "/api/runs/:run_id/cancel",
		ResponseType: "MutationAcceptedResponse",
		TimeoutClass: TimeoutMutate,
	},
	{Method: http.MethodPost, Path: "/api/sync", ResponseType: "SyncResponse", TimeoutClass: TimeoutLongMutate},
	{Method: http.MethodPost, Path: "/api/exec", ResponseType: "RunResponse", TimeoutClass: TimeoutLongMutate},
}

func FindRoute(method string, requestPath string) (RouteSpec, bool) {
	normalizedMethod := strings.TrimSpace(strings.ToUpper(method))
	normalizedPath := normalizeRoutePath(requestPath)
	for _, route := range RouteInventory {
		if route.Method != normalizedMethod {
			continue
		}
		if routePathMatches(route.Path, normalizedPath) {
			return route, true
		}
	}
	return RouteSpec{}, false
}

func TimeoutClassForRoute(method string, requestPath string) TimeoutClass {
	if route, ok := FindRoute(method, requestPath); ok {
		return route.TimeoutClass
	}
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead:
		return TimeoutRead
	default:
		return TimeoutMutate
	}
}

func normalizeRoutePath(requestPath string) string {
	trimmed := strings.TrimSpace(requestPath)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err == nil {
		return parsed.Path
	}
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

func routePathMatches(pattern string, requestPath string) bool {
	patternParts := splitRouteParts(pattern)
	pathParts := splitRouteParts(requestPath)
	if len(patternParts) != len(pathParts) {
		return false
	}
	for idx := range patternParts {
		if strings.HasPrefix(patternParts[idx], ":") {
			if strings.TrimSpace(pathParts[idx]) == "" {
				return false
			}
			continue
		}
		if patternParts[idx] != pathParts[idx] {
			return false
		}
	}
	return true
}

func splitRouteParts(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
