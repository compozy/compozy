package core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) ListWorktreeAggregate(c *gin.Context) {
	breadth, ok := h.aggregateBreadth(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	reader, ok := h.Worktrees.(interface {
		ListAggregate(context.Context, store.ReadScope, map[string]bool) (*worktree.DetailedListing, error)
	})
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, fmt.Errorf("worktree aggregate unavailable"))
		return
	}
	allowed, err := h.aggregateWorkspaceSet(c, breadth)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	listing, err := reader.ListAggregate(c.Request.Context(), scope, allowed)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	if err := h.enrichWorktreeListingOwners(c.Request.Context(), listing); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, WorktreesPayloadFromListing(listing))
}
