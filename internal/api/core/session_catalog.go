package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

var errSessionListWorkspaceResolution = errors.New("api: session list workspace resolution failed")

// ListSessions returns one bounded page from the shared public session catalog.
func (h *BaseHandlers) ListSessions(c *gin.Context) {
	if !h.requireOperatorSurface(c, "session catalog") {
		return
	}
	query, includeHealth, err := h.parseSessionListQuery(c)
	if err != nil {
		status := http.StatusBadRequest
		if isSessionListWorkspaceError(err) {
			status = StatusForWorkspaceError(err)
		}
		h.respondError(c, status, err)
		return
	}
	pager, ok := h.Sessions.(SessionPageManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: paged session catalog is required"))
		return
	}
	page, err := pager.ListPage(c.Request.Context(), query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, session.ErrListQueryInvalid) || errors.Is(err, session.ErrListCursorInvalid) {
			status = http.StatusBadRequest
		}
		h.respondError(c, status, err)
		return
	}
	payloads, err := h.sessionPayloadsWithOptionalHealth(c.Request.Context(), page.Sessions, includeHealth)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionCatalogResponse{
		Sessions: payloads,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	})
}

func isSessionListWorkspaceError(err error) bool {
	return errors.Is(err, errSessionListWorkspaceResolution)
}

func (h *BaseHandlers) parseSessionListQuery(c *gin.Context) (session.ListQuery, bool, error) {
	includeHealth, err := parseBoolQuery(c, "include_health")
	if err != nil {
		return session.ListQuery{}, false, err
	}
	resumable, err := parseBoolQuery(c, "resumable")
	if err != nil {
		return session.ListQuery{}, false, err
	}
	attentionOnly, err := parseBoolQuery(c, "attention")
	if err != nil {
		return session.ListQuery{}, false, err
	}
	badges, err := session.ParseBadgeFilters(c.QueryArray("badge"))
	if err != nil {
		return session.ListQuery{}, false, fmt.Errorf("%w: %w", session.ErrListQueryInvalid, err)
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return session.ListQuery{}, false, err
	}
	allWorkspaces, err := parseBoolQuery(c, "all_workspaces")
	if err != nil {
		return session.ListQuery{}, false, err
	}
	workspaceRef := strings.TrimSpace(c.Query("workspace_id"))
	if (workspaceRef != "") == allWorkspaces {
		return session.ListQuery{}, false, fmt.Errorf(
			"%w: choose exactly one workspace_id or all_workspaces=true",
			session.ErrListQueryInvalid,
		)
	}
	workspaceID := ""
	if workspaceRef != "" {
		workspaceID, err = h.lookupWorkspaceID(c.Request.Context(), workspaceRef)
		if err != nil {
			return session.ListQuery{}, false, fmt.Errorf("%w: %w", errSessionListWorkspaceResolution, err)
		}
	}
	sessionType, err := parseSessionListType(c.Query("type"))
	if err != nil {
		return session.ListQuery{}, false, err
	}
	query := session.ListQuery{
		WorkspaceID:     workspaceID,
		AllWorkspaces:   allWorkspaces,
		WorktreeID:      strings.TrimSpace(c.Query("worktree")),
		State:           strings.TrimSpace(c.Query("state")),
		SessionType:     sessionType,
		AgentName:       strings.TrimSpace(c.Query("agent")),
		ParentSessionID: strings.TrimSpace(c.Query("parent")),
		RootSessionID:   strings.TrimSpace(c.Query("root")),
		Search:          strings.TrimSpace(c.Query("q")),
		Resumable:       resumable,
		AttentionOnly:   attentionOnly,
		Badges:          badges,
		Archive:         store.SessionArchiveFilter(strings.TrimSpace(c.Query("archive"))),
		Sort:            strings.TrimSpace(c.Query("sort")),
		Cursor:          strings.TrimSpace(c.Query("cursor")),
		Limit:           limit,
	}
	return query, includeHealth, nil
}

func parseSessionListType(raw string) (session.Type, error) {
	sessionType := session.Type(strings.TrimSpace(raw))
	switch sessionType {
	case "", session.SessionTypeUser, session.SessionTypeSystem,
		session.SessionTypeCoordinator, session.SessionTypeSpawned:
		return sessionType, nil
	default:
		return "", fmt.Errorf("%w: unsupported type %q", session.ErrListQueryInvalid, sessionType)
	}
}
