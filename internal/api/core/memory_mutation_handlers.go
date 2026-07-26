package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/memory"

	"github.com/gin-gonic/gin"
)

// ReadMemory returns one memory document.
func (h *BaseHandlers) ReadMemory(c *gin.Context) {
	selector := memorySelectorFromQuery(c)
	location, err := h.resolveMemoryLocation(c.Request.Context(), c.Param("filename"), selector)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	if !selector.IncludeSystem && memory.IsSystemManagedFilename(location.Filename) {
		err := fmt.Errorf("%w: memory %q not found", os.ErrNotExist, location.Filename)
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	store, err := h.memoryStoreForLocation(c.Request.Context(), location)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	content, err := store.Read(location.Scope, c.Param("filename"))
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	entry, err := h.memoryEntryPayload(store, location, content)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	c.JSON(http.StatusOK, contract.MemoryEntryResponse{Memory: entry})
}

// WriteMemory creates or proposes one Memory v2 entry.
func (h *BaseHandlers) WriteMemory(c *gin.Context) {
	var req contract.MemoryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory write request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	if err := validateMemoryCreateRequest(req); err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	selector, err := h.resolveMemoryCreateSelector(c.Request.Context(), req)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	selector.Scope = defaultMemorySelectorScope(selector)
	store, err := h.memoryRecallStoreForSelector(c.Request.Context(), selector)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	decision, err := store.ProposeCandidate(
		c.Request.Context(),
		h.memoryCandidateFromCreate(selector, req),
	)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	if decision.Op == memcontract.OpReject {
		h.respondDecisionRejected(c, decision)
		return
	}

	c.JSON(http.StatusOK, contract.MemoryMutationDecisionResponse{
		Decision: MemoryDecisionPayload(decision, nil),
		Applied:  memoryDecisionApplied(decision),
		DryRun:   req.DryRun,
	})
}

// EditMemory edits one memory document through the controller.
func (h *BaseHandlers) EditMemory(c *gin.Context) {
	var req contract.MemoryEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory edit request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	location, err := h.resolveMemoryLocation(c.Request.Context(), c.Param("filename"), memorySelector{
		Scope:       req.Scope,
		WorkspaceID: req.WorkspaceID,
		AgentName:   req.AgentName,
		AgentTier:   req.AgentTier,
	})
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	store, err := h.memoryStoreForLocation(c.Request.Context(), location)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	content, err := store.Read(location.Scope, c.Param("filename"))
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	candidate, err := h.memoryCandidateFromEdit(location, req, content)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	decision, err := store.ProposeCandidate(c.Request.Context(), candidate)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	if decision.Op == memcontract.OpReject {
		h.respondDecisionRejected(c, decision)
		return
	}

	c.JSON(http.StatusOK, contract.MemoryMutationDecisionResponse{
		Decision: MemoryDecisionPayload(decision, nil),
		Applied:  memoryDecisionApplied(decision),
		DryRun:   req.DryRun,
	})
}

// DeleteMemory deletes one memory document.
func (h *BaseHandlers) DeleteMemory(c *gin.Context) {
	location, err := h.resolveMemoryLocation(c.Request.Context(), c.Param("filename"), memorySelectorFromQuery(c))
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	store, err := h.memoryStoreForLocation(c.Request.Context(), location)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	result, err := store.ProposeDelete(
		c.Request.Context(),
		location.Scope,
		c.Param("filename"),
		h.memoryOrigin(),
	)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	c.JSON(http.StatusOK, contract.MemoryDeleteResponse{
		Decision: MemoryDecisionPayload(result.Decision, nil),
		Applied:  result.Applied,
	})
}

// ReindexMemory rebuilds the derived memory catalog from Markdown memory files.
func (h *BaseHandlers) ReindexMemory(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}

	var req contract.MemoryReindexV2Request
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory reindex request: %w", h.transportName(), err),
			nil,
		)
		return
	}

	selector, err := h.resolveMemorySelector(c.Request.Context(), memorySelector{
		Scope:       req.Scope,
		WorkspaceID: req.WorkspaceID,
		AgentName:   req.AgentName,
		AgentTier:   req.AgentTier,
	}, false)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	store, err := h.memoryRecallStoreForSelector(c.Request.Context(), selector)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	result, err := store.Reindex(c.Request.Context(), memcontract.ReindexOptions{
		Scope:     selector.Scope,
		Workspace: selector.Workspace,
	})
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	c.JSON(http.StatusOK, contract.MemoryReindexResponse{
		IndexedFiles: result.IndexedFiles,
		Scope:        result.Scope,
		WorkspaceID:  firstNonEmptyString(selector.WorkspaceID, result.Workspace),
		AgentName:    selector.AgentName,
		AgentTier:    selector.AgentTier,
		CompletedAt:  result.CompletedAt.UTC(),
	})
}
