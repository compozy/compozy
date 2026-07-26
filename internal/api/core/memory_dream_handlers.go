package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"strconv"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/memory"

	"github.com/gin-gonic/gin"
)

// TriggerMemoryDream triggers dream consolidation when enabled.
func (h *BaseHandlers) TriggerMemoryDream(c *gin.Context) {
	var req contract.MemoryDreamTriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory dream trigger request: %w", h.transportName(), err),
			nil,
		)
		return
	}

	if h.DreamTrigger == nil || !h.DreamTrigger.Enabled() {
		c.JSON(http.StatusOK, contract.MemoryDreamTriggerResponse{
			Dream: contract.MemoryDreamPayload{
				Status:      contract.MemoryDreamStateSkipped,
				Scope:       req.Scope.Normalize(),
				WorkspaceID: req.WorkspaceID,
				AgentName:   strings.TrimSpace(req.AgentName),
				AgentTier:   req.AgentTier.Normalize(),
				StartedAt:   h.nowUTC(),
			},
			Triggered: false,
			Reason:    "dream consolidation is disabled",
		})
		return
	}

	triggered, reason, err := h.DreamTrigger.Trigger(c.Request.Context(), strings.TrimSpace(req.WorkspaceID))
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	status := contract.MemoryDreamStateSkipped
	if triggered {
		status = contract.MemoryDreamStateRunning
	}
	c.JSON(http.StatusOK, contract.MemoryDreamTriggerResponse{
		Dream: contract.MemoryDreamPayload{
			Status:      status,
			Scope:       req.Scope.Normalize(),
			WorkspaceID: strings.TrimSpace(req.WorkspaceID),
			AgentName:   strings.TrimSpace(req.AgentName),
			AgentTier:   req.AgentTier.Normalize(),
			StartedAt:   h.nowUTC(),
		},
		Triggered: triggered,
		Reason:    strings.TrimSpace(reason),
	})
}

func (h *BaseHandlers) PromoteMemory(c *gin.Context) {
	var req contract.MemoryPromoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory promote request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	if strings.TrimSpace(req.Filename) == "" {
		err := NewMemoryValidationError(errors.New("filename is required"))
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	targetStore, candidate, err := h.promoteMemoryCandidate(c.Request.Context(), req)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	decision, err := targetStore.ProposeCandidate(c.Request.Context(), candidate)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	if decision.Op == memcontract.OpReject {
		h.respondDecisionRejected(c, decision)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryPromoteResponse{
		Decision: MemoryDecisionPayload(decision, nil),
		Applied:  memoryDecisionApplied(decision),
		DryRun:   req.DryRun,
	})
}

func (h *BaseHandlers) promoteMemoryCandidate(
	ctx context.Context,
	req contract.MemoryPromoteRequest,
) (*memory.Store, memcontract.Candidate, error) {
	sourceLocation, err := h.resolveMemoryLocation(ctx, req.Filename, memorySelectorFromScopePayload(req.From))
	if err != nil {
		return nil, memcontract.Candidate{}, err
	}
	sourceStore, err := h.memoryStoreForLocation(ctx, sourceLocation)
	if err != nil {
		return nil, memcontract.Candidate{}, err
	}
	raw, err := sourceStore.Read(sourceLocation.Scope, sourceLocation.Filename)
	if err != nil {
		return nil, memcontract.Candidate{}, err
	}
	header, body, err := memoryHeaderAndBody(raw)
	if err != nil {
		return nil, memcontract.Candidate{}, err
	}
	targetSelector, err := h.resolveMemorySelector(ctx, memorySelectorFromScopePayload(req.To), true)
	if err != nil {
		return nil, memcontract.Candidate{}, err
	}
	targetStore, err := h.memoryRecallStoreForSelector(ctx, targetSelector)
	if err != nil {
		return nil, memcontract.Candidate{}, err
	}
	header.Scope = targetSelector.Scope
	header.AgentName = targetSelector.AgentName
	header.AgentTier = targetSelector.AgentTier
	return targetStore, memcontract.Candidate{
		WorkspaceID: targetSelector.WorkspaceID,
		Scope:       targetSelector.Scope,
		AgentName:   targetSelector.AgentName,
		AgentTier:   targetSelector.AgentTier,
		Origin:      h.memoryOrigin(),
		Content:     strings.TrimSpace(body),
		Frontmatter: header,
		Metadata: map[string]string{
			memoryMetadataIDKey:             strings.TrimSpace(req.IdempotencyKey),
			memoryMetadataTargetFilenameKey: sourceLocation.Filename,
		},
		SubmittedAt: h.nowUTC(),
	}, nil
}

func (h *BaseHandlers) ResetMemory(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}
	var req contract.MemoryResetRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory reset request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	if !req.Confirm {
		c.JSON(http.StatusOK, contract.MemoryResetResponse{
			ResetAt:     h.nowUTC(),
			DerivedOnly: req.DerivedOnly,
		})
		return
	}
	if !req.DerivedOnly {
		err := NewMemoryValidationError(errors.New("only derived memory reset is supported in Slice 1"))
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
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
	selector.Scope = defaultMemorySelectorScope(selector)
	store, err := h.memoryRecallStoreForSelector(c.Request.Context(), selector)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	result, err := store.ResetDerived(c.Request.Context(), memcontract.ReindexOptions{
		Scope:     selector.Scope,
		Workspace: selector.Workspace,
	})
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryResetResponse{
		ResetAt:      result.ResetAt.UTC(),
		DerivedOnly:  true,
		DeletedRows:  result.DeletedRows,
		DeletedFiles: 0,
	})
}

func (h *BaseHandlers) ReloadMemory(c *gin.Context) {
	selector, err := h.decodeMemoryReloadSelector(c)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	if selector.Scope != "" || selector.WorkspaceID != "" || selector.AgentName != "" || selector.AgentTier != "" {
		if _, err := h.resolveMemorySelector(c.Request.Context(), selector, false); err != nil {
			h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
			return
		}
	}
	reloadedAt := h.nowUTC()
	c.JSON(http.StatusOK, contract.MemoryReloadResponse{
		ReloadedAt: reloadedAt,
		Generation: reloadedAt.UnixNano(),
	})
}

func (h *BaseHandlers) GetMemoryRecallTrace(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		err := NewMemoryValidationError(errors.New("session_id is required"))
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	turnSeq, err := strconv.ParseInt(strings.TrimSpace(c.Param("turn_seq")), 10, 64)
	if err != nil || turnSeq <= 0 {
		validationErr := NewMemoryValidationError(errors.New("turn_seq must be a positive integer"))
		h.respondMemoryError(c, StatusForMemoryError(validationErr), validationErr, nil)
		return
	}
	notFound := fmt.Errorf("%w: recall trace %s/%d is not materialized", os.ErrNotExist, sessionID, turnSeq)
	h.respondMemoryError(c, StatusForMemoryError(notFound), notFound, nil)
}
