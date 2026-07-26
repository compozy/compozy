package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/gin-gonic/gin"
)

type memoryListQuery struct {
	selector memorySelector
	typ      memcontract.Type
	sort     string
	cursor   string
	limit    int
}

// ListMemory lists one bounded page of memory headers for the requested selector.
func (h *BaseHandlers) ListMemory(c *gin.Context) {
	query, err := parseMemoryListQuery(c)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	page, err := h.listMemoryHeaderPage(c.Request.Context(), query)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	c.JSON(http.StatusOK, contract.MemoryListResponse{
		Memories: memorySummaryPayloads(page.Headers),
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	})
}

func parseMemoryListQuery(c *gin.Context) (memoryListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return memoryListQuery{}, NewMemoryValidationError(err)
	}
	includeSystem, err := ParseOptionalBool(c.Query("include_system"))
	if err != nil {
		return memoryListQuery{}, NewMemoryValidationError(err)
	}
	selector := memorySelectorFromQuery(c)
	selector.IncludeSystem = includeSystem
	query := memoryListQuery{
		selector: selector,
		typ:      memcontract.Type(strings.TrimSpace(c.Query("type"))).Normalize(),
		sort:     strings.TrimSpace(c.Query("sort")),
		cursor:   strings.TrimSpace(c.Query("cursor")),
		limit:    limit,
	}
	if query.typ != "" {
		if err := query.typ.Validate(); err != nil {
			return memoryListQuery{}, NewMemoryValidationError(err)
		}
	}
	return query, nil
}

func (h *BaseHandlers) listMemoryHeaderPage(
	ctx context.Context,
	query memoryListQuery,
) (memory.HeaderListPage, error) {
	if h.MemoryStore == nil {
		return memory.HeaderListPage{}, errors.New("memory store is not configured")
	}

	resolved, err := h.resolveMemorySelector(ctx, query.selector, false)
	if err != nil {
		return memory.HeaderListPage{}, err
	}
	headers, err := h.scanMemoryHeaders(ctx, resolved)
	if err != nil {
		return memory.HeaderListPage{}, err
	}
	page, err := memory.BuildHeaderListPage(headers, memory.HeaderListQuery{
		Scope:         resolved.Scope,
		WorkspaceID:   resolved.WorkspaceID,
		AgentName:     resolved.AgentName,
		AgentTier:     resolved.AgentTier,
		Type:          query.typ,
		IncludeSystem: resolved.IncludeSystem,
		Sort:          query.sort,
		Cursor:        query.cursor,
		Limit:         query.limit,
	})
	if err != nil {
		return memory.HeaderListPage{}, NewMemoryValidationError(err)
	}
	return page, nil
}

func (h *BaseHandlers) scanMemoryHeaders(
	ctx context.Context,
	resolved memorySelector,
) ([]memcontract.Header, error) {
	scopes := []memcontract.Scope{memcontract.ScopeGlobal}
	if resolved.Scope != "" {
		scopes = []memcontract.Scope{resolved.Scope}
	}
	if resolved.Scope == "" && resolved.Workspace != "" {
		scopes = append(scopes, memcontract.ScopeWorkspace)
	}
	if resolved.Scope == "" && resolved.AgentName != "" && resolved.AgentTier != "" {
		scopes = append(scopes, memcontract.ScopeAgent)
	}

	headers := make([]memcontract.Header, 0, len(scopes))
	for _, currentScope := range scopes {
		current := resolved
		current.Scope = currentScope
		store, err := h.memoryStoreForSelector(ctx, current)
		if err != nil {
			return nil, err
		}
		items, err := store.CatalogHeaders(ctx, currentScope)
		if err != nil {
			return nil, err
		}
		for index := range items {
			items[index].Scope = currentScope
			items[index].WorkspaceID = current.WorkspaceID
			if currentScope == memcontract.ScopeAgent {
				items[index].AgentName = current.AgentName
				items[index].AgentTier = current.AgentTier
			}
		}
		headers = append(headers, items...)
	}
	return headers, nil
}
