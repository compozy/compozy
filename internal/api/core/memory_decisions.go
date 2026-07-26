package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	aghworkspace "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

// ListMemoryDecisions returns filtered Memory v2 controller decisions.
func (h *BaseHandlers) ListMemoryDecisions(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}
	query, err := h.memoryDecisionListQuery(c)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	records, err := h.MemoryStore.ListDecisionRecords(c.Request.Context(), query)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	payloads := make([]contract.MemoryDecisionPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, MemoryDecisionRecordPayload(record))
	}
	c.JSON(http.StatusOK, contract.MemoryDecisionListResponse{Decisions: payloads})
}

// GetMemoryDecision returns one Memory v2 controller decision.
func (h *BaseHandlers) GetMemoryDecision(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}
	record, err := h.MemoryStore.LoadDecisionRecord(c.Request.Context(), c.Param("decision_id"))
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryDecisionResponse{Decision: MemoryDecisionRecordPayload(record)})
}

// RevertMemoryDecision re-applies prior content for one persisted decision.
func (h *BaseHandlers) RevertMemoryDecision(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}
	var req contract.MemoryDecisionRevertRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory decision revert request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	id := strings.TrimSpace(c.Param("decision_id"))
	record, err := h.MemoryStore.LoadDecisionRecord(c.Request.Context(), id)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	reverted := false
	if !req.DryRun {
		store, err := h.memoryStoreForDecisionRecord(c.Request.Context(), record)
		if err != nil {
			h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
			return
		}
		result, err := store.RevertDecision(c.Request.Context(), id)
		if err != nil {
			h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
			return
		}
		reverted = result.Reverted
	}
	c.JSON(http.StatusOK, contract.MemoryDecisionRevertResponse{
		Decision: MemoryDecisionRecordPayload(record),
		Reverted: reverted,
		DryRun:   req.DryRun,
	})
}

func (h *BaseHandlers) memoryStoreForDecisionRecord(
	ctx context.Context,
	record memory.DecisionRecord,
) (*memory.Store, error) {
	if h.MemoryStore == nil {
		return nil, errors.New("memory store is not configured")
	}
	scope := record.Decision.Frontmatter.Scope.Normalize()
	if scope == memcontract.ScopeWorkspace {
		workspace, err := h.workspaceForMemoryDecision(ctx, record.WorkspaceID)
		if err != nil {
			return nil, err
		}
		return h.MemoryStore.ForWorkspace(workspace.RootDir), nil
	}
	if scope == memcontract.ScopeAgent && record.AgentTier.Normalize() == memcontract.AgentTierWorkspace {
		workspace, err := h.workspaceForMemoryDecision(ctx, record.WorkspaceID)
		if err != nil {
			return nil, err
		}
		return h.MemoryStore.ForWorkspace(workspace.RootDir), nil
	}
	return h.MemoryStore, nil
}

func (h *BaseHandlers) workspaceForMemoryDecision(
	ctx context.Context,
	workspaceID string,
) (aghworkspace.Workspace, error) {
	id := strings.TrimSpace(workspaceID)
	if id == "" {
		return aghworkspace.Workspace{}, errors.New("memory: decision workspace_id is required")
	}
	if h.Workspaces == nil {
		return aghworkspace.Workspace{}, errors.New("memory: workspace service is not configured")
	}
	workspace, err := h.Workspaces.Get(ctx, id)
	if err != nil {
		return aghworkspace.Workspace{}, fmt.Errorf("memory: resolve decision workspace %q: %w", id, err)
	}
	if strings.TrimSpace(workspace.RootDir) == "" {
		return aghworkspace.Workspace{}, fmt.Errorf("memory: decision workspace %q root_dir is required", id)
	}
	return workspace, nil
}

func (h *BaseHandlers) memoryDecisionListQuery(c *gin.Context) (memory.DecisionListQuery, error) {
	selector, err := h.resolveMemorySelector(c.Request.Context(), memorySelectorFromQuery(c), false)
	if err != nil {
		return memory.DecisionListQuery{}, err
	}
	since, err := ParseOptionalTime(c.Query("since"))
	if err != nil {
		return memory.DecisionListQuery{}, NewMemoryValidationError(err)
	}
	limit, err := parseMemoryLimit(c.Query("limit"))
	if err != nil {
		return memory.DecisionListQuery{}, err
	}
	return memory.DecisionListQuery{
		Scope:          selector.Scope,
		WorkspaceID:    selector.WorkspaceID,
		AgentName:      selector.AgentName,
		AgentTier:      selector.AgentTier,
		Operation:      firstNonEmptyString(c.Query("operation"), c.Query("op")),
		TargetFilename: strings.TrimSpace(c.Query("filename")),
		Since:          since,
		Reason:         c.Query("reason"),
		Limit:          limit,
	}, nil
}
