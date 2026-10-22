package core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

type terminalAggregateReader interface {
	ListAggregate(context.Context, store.ReadScope) ([]terminalpkg.Info, error)
}

func (h *BaseHandlers) aggregateTerminalItems(
	c *gin.Context,
	breadth WorkspaceBreadth,
	scope store.ReadScope,
) ([]terminalInfoPayload, error) {
	reader, ok := h.Terminal.(terminalAggregateReader)
	if !ok {
		return nil, fmt.Errorf("terminal aggregate unavailable")
	}
	infos, err := reader.ListAggregate(c.Request.Context(), scope)
	if err != nil {
		return nil, err
	}
	allowed, err := h.aggregateWorkspaceSet(c, breadth)
	if err != nil {
		return nil, err
	}
	selected := infos[:0]
	for _, info := range infos {
		if allowed[info.WS] {
			selected = append(selected, info)
		}
	}
	return h.terminalInfoPayloads(c, selected)
}

func (h *BaseHandlers) aggregateWorkspaceSet(c *gin.Context, breadth WorkspaceBreadth) (map[string]bool, error) {
	if h.Workspaces == nil {
		return nil, fmt.Errorf("workspace catalog unavailable")
	}
	workspaces, err := h.Workspaces.List(c.Request.Context())
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{"": true}
	for _, workspace := range workspaces {
		allowed[workspace.ID] = true
	}
	if len(breadth.WorkspaceIDs) > 0 {
		selected := make(map[string]bool)
		for _, id := range breadth.WorkspaceIDs {
			if id == workspaceaccess.UserSelector {
				id = ""
			}
			if allowed[id] {
				selected[id] = true
			}
		}
		return selected, nil
	}
	return allowed, nil
}

func (h *BaseHandlers) ListTerminalAggregate(c *gin.Context) {
	breadth, ok := h.aggregateBreadth(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	items, err := h.aggregateTerminalItems(c, breadth, scope)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{terminalsKey: items})
}
