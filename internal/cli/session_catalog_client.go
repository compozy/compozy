package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// SessionListQuery captures the CLI filters for one session catalog page.
type SessionListQuery struct {
	Workspace     string
	State         string
	Type          string
	Agent         string
	Query         string
	Resumable     bool
	IncludeHealth bool
	Limit         int
	Sort          string
	Cursor        string
}

// SessionListPage is the shared bounded session catalog response.
type SessionListPage = contract.SessionCatalogResponse

func (c *unixSocketClient) ListSessions(ctx context.Context, query SessionListQuery) (SessionListPage, error) {
	var response SessionListPage
	if err := c.doJSON(ctx, http.MethodGet, "/api/sessions", sessionListValues(query), nil, &response); err != nil {
		return SessionListPage{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetSession(ctx context.Context, id string) (SessionRecord, error) {
	target, err := requireNetworkPathValue("session_id", id)
	if err != nil {
		return SessionRecord{}, err
	}
	var response contract.SessionResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(target), nil, nil, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *unixSocketClient) sessionScopedPath(ctx context.Context, id string, suffix string) (string, error) {
	sessionID, err := requireNetworkPathValue("session_id", id)
	if err != nil {
		return "", err
	}
	workspaceRef, err := c.sessionWorkspaceRef(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return "/api/workspaces/" + url.PathEscape(workspaceRef) + "/sessions/" +
		url.PathEscape(sessionID) + suffix, nil
}

func (c *unixSocketClient) sessionWorkspaceRef(ctx context.Context, sessionID string) (string, error) {
	record, err := c.GetSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("cli: resolve session %q workspace: %w", sessionID, err)
	}
	workspaceID := strings.TrimSpace(record.WorkspaceID)
	if workspaceID == "" {
		return "", fmt.Errorf("cli: session %q has no workspace_id", sessionID)
	}
	return workspaceID, nil
}

func sessionListValues(query SessionListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Workspace); trimmed != "" {
		values.Set("workspace", trimmed)
	}
	if trimmed := strings.TrimSpace(query.State); trimmed != "" {
		values.Set("state", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Type); trimmed != "" {
		values.Set("type", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Agent); trimmed != "" {
		values.Set("agent", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Query); trimmed != "" {
		values.Set("q", trimmed)
	}
	if query.Resumable {
		values.Set("resumable", "true")
	}
	if query.IncludeHealth {
		values.Set("include_health", "true")
	}
	if query.Limit != 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if sortKey := strings.TrimSpace(query.Sort); sortKey != "" {
		values.Set("sort", sortKey)
	}
	if cursor := strings.TrimSpace(query.Cursor); cursor != "" {
		values.Set("cursor", cursor)
	}
	return values
}
