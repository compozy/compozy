package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

// SessionListQuery captures the CLI filters for one session catalog page.
type SessionListQuery struct {
	Columns       []string
	HideChildren  bool
	Subagents     string
	Workspace     string
	Worktree      string
	State         string
	Type          string
	Agent         string
	Parent        string
	Root          string
	Query         string
	Resumable     bool
	Attention     bool
	Badges        []session.Badge
	Archive       string
	Later         string
	Pinned        string
	IncludeHealth bool
	Limit         int
	Sort          string
	Cursor        string
}

// SessionListPage is the counted catalog required by CLI list output.
type SessionListPage struct {
	Sessions   []SessionRecord                   `json:"sessions"`
	Page       contract.CountedCursorPagePayload `json:"page"`
	computerID string
}

func (c *daemonClient) ListSessions(ctx context.Context, query SessionListQuery) (SessionListPage, error) {
	var response contract.SessionCatalogResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/sessions", sessionListValues(query), nil, &response); err != nil {
		return SessionListPage{}, err
	}
	if response.Page.Total == nil {
		return SessionListPage{}, fmt.Errorf("cli: counted session catalog omitted total")
	}
	return SessionListPage{
		Sessions:   response.Sessions,
		computerID: c.target.computerID,
		Page: contract.CountedCursorPagePayload{
			NextCursor: response.Page.NextCursor,
			HasMore:    response.Page.HasMore,
			Total:      *response.Page.Total,
			Limit:      response.Page.Limit,
		},
	}, nil
}

func (c *daemonClient) GetSession(ctx context.Context, id string) (SessionRecord, error) {
	target, err := requirePathValue("session_id", id)
	if err != nil {
		return SessionRecord{}, err
	}
	var response contract.SessionResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(target), nil, nil, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *daemonClient) sessionScopedPath(ctx context.Context, id string, suffix string) (string, error) {
	sessionID, err := requirePathValue("session_id", id)
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

func (c *daemonClient) sessionWorkspaceRef(ctx context.Context, sessionID string) (string, error) {
	record, err := c.GetSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("cli: resolve session %q workspace: %w", sessionID, err)
	}
	workspaceID := strings.TrimSpace(record.WorkspaceID)
	if workspaceID == "" {
		// A session without a project belongs to the User scope.
		return workspaceaccess.UserSelector, nil
	}
	return workspaceID, nil
}

func sessionListValues(query SessionListQuery) url.Values {
	values := url.Values{}
	appendSessionBoardValues(values, query)
	if query.Subagents != "" {
		values.Set("subagents", query.Subagents)
	}
	if trimmed := strings.TrimSpace(query.Workspace); trimmed != "" {
		values.Set("workspace_id", trimmed)
	} else {
		values.Set("all_workspaces", "true")
	}
	setTrimmedValue(values, "worktree", query.Worktree)
	setTrimmedValue(values, "state", query.State)
	setTrimmedValue(values, "type", query.Type)
	setTrimmedValue(values, "agent", query.Agent)
	setTrimmedValue(values, "parent", query.Parent)
	setTrimmedValue(values, "root", query.Root)
	setTrimmedValue(values, "q", query.Query)
	setTrimmedValue(values, "archive", query.Archive)
	setTrimmedValue(values, "later", query.Later)
	setTrimmedValue(values, "pinned", query.Pinned)
	setTrimmedValue(values, "sort", query.Sort)
	setTrimmedValue(values, "cursor", query.Cursor)
	if query.Resumable {
		values.Set("resumable", "true")
	}
	if query.Attention {
		values.Set("attention", "true")
	}
	if len(query.Badges) > 0 {
		badges := make([]string, 0, len(query.Badges))
		for _, badge := range query.Badges {
			badges = append(badges, string(badge))
		}
		values.Set("badge", strings.Join(badges, ","))
	}
	if query.IncludeHealth {
		values.Set("include_health", "true")
	}
	if query.Limit != 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

// setTrimmedValue sets a non-blank value under its key, trimmed.
func setTrimmedValue(values url.Values, key string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		values.Set(key, trimmed)
	}
}

func appendSessionBoardValues(values url.Values, query SessionListQuery) {
	for _, column := range query.Columns {
		values.Add("board_column", column)
	}
	if query.HideChildren {
		values.Set("children", "exclude")
	}
}
