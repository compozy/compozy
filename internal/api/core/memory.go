package core

import (
	"context"
	"errors"
	"fmt"

	"net/http"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/gin-gonic/gin"
)

const (
	memoryHealthStatusOK          = "ok"
	memoryHealthStatusDisabled    = "disabled"
	memoryHealthStatusDegraded    = "degraded"
	memoryHealthStatusUnavailable = "unavailable"

	memoryErrorCodeInternal    = "memory.internal"
	memoryErrorCodeNotFound    = "memory.not_found"
	memoryErrorCodeRejected    = "memory.rejected"
	memoryErrorCodeUnsupported = "memory.unsupported"
	memoryErrorCodeValidation  = "memory.validation"

	memoryMetadataIDKey              = "idempotency_key"
	memoryMetadataReasonKey          = "reason"
	memoryMetadataTargetAttributeKey = "target_attribute"
	memoryMetadataTargetEntityKey    = "target_entity"
	memoryMetadataTargetFilenameKey  = "target_filename"

	memoryUnsupportedStatus = http.StatusNotImplemented
	memoryLocalProviderName = "local"
)

var (
	// ErrMemoryRejected marks controller rejections that should surface as 422.
	ErrMemoryRejected = errors.New("memory rejected")
	// ErrMemoryUnsupported marks registered Slice 1 routes whose backing runtime
	// service is intentionally not wired yet.
	ErrMemoryUnsupported = errors.New("memory operation unsupported")
)

// MemoryLocation identifies the storage location for a memory document.
type MemoryLocation struct {
	Scope       memcontract.Scope
	Workspace   string
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	Filename    string
}

type memorySelector struct {
	Scope         memcontract.Scope
	Workspace     string
	WorkspaceID   string
	AgentName     string
	AgentTier     memcontract.AgentTier
	IncludeSystem bool
}

// MemoryHealth returns the memory-specific health snapshot.
func (h *BaseHandlers) MemoryHealth(c *gin.Context) {
	payload, err := h.memoryHealth(c)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// MemoryConfigMetadata returns settings/config metadata that is safe for agents.
func (h *BaseHandlers) MemoryConfigMetadata(c *gin.Context) {
	payload := contract.MemoryConfigMetadataResponse{
		Config:       settingsMemoryConfigPayload(&h.Config.Memory),
		MutablePaths: h.memoryMutableConfigPaths(),
		LockedPaths:  h.memoryLockedConfigPaths(),
		Providers:    h.memoryProviderPayloads(),
	}
	c.JSON(http.StatusOK, payload)
}

// MemoryHistory returns bounded, redacted memory operation history.
func (h *BaseHandlers) MemoryHistory(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}

	query, err := parseMemoryHistoryQuery(c)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	records, err := h.MemoryStore.History(c.Request.Context(), query)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}

	c.JSON(http.StatusOK, contract.MemoryOperationHistoryResponse{Operations: MemoryOperationHistoryPayloads(records)})
}

// MemoryScopeShow reports the effective selector and precedence chain.
func (h *BaseHandlers) MemoryScopeShow(c *gin.Context) {
	selector, err := h.resolveMemorySelector(c.Request.Context(), memorySelectorFromQuery(c), false)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryScopeShowResponse{
		Selector:   memorySelectorPayload(selector),
		Precedence: memoryPrecedencePayloads(selector),
		Roots:      h.memorySelectorRoots(selector),
	})
}

// SearchMemory returns ranked durable memory matches.
func (h *BaseHandlers) SearchMemory(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}

	var req contract.MemorySearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory search request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	if strings.TrimSpace(req.QueryText) == "" {
		err := NewMemoryValidationError(errors.New("query_text is required"))
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
	recallWorkspaceID := memoryRecallQueryWorkspaceID(selector)
	recall, err := store.Recall(c.Request.Context(), memcontract.Query{
		WorkspaceID: recallWorkspaceID,
		AgentName:   selector.AgentName,
		QueryText:   req.QueryText,
		ContextHint: req.ContextHint,
	}, memcontract.RecallOptions{
		TopK:                   req.TopK,
		RawCandidates:          req.RawCandidates,
		IncludeAlreadySurfaced: req.IncludeAlreadySurfaced,
		IncludeSystem:          req.IncludeSystem,
		AlreadySurfaced:        req.AlreadySurfaced,
		AllowTrivialQuery:      true,
	})
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	results := memorySearchResultPayloads(recall)
	if len(results) == 0 {
		searchResults, searchErr := h.memoryExplicitSearchFallback(
			c.Request.Context(),
			selector,
			req.QueryText,
			req.TopK,
		)
		if searchErr != nil {
			h.respondMemoryError(c, StatusForMemoryError(searchErr), searchErr, nil)
			return
		}
		results = memorySearchResultPayloadsFromSearchResults(searchResults, selector.WorkspaceID)
	}

	c.JSON(http.StatusOK, contract.MemorySearchResponse{
		Results: results,
		Recall:  recall,
	})
}

func memoryRecallQueryWorkspaceID(selector memorySelector) string {
	switch selector.Scope.Normalize() {
	case memcontract.ScopeWorkspace:
		return strings.TrimSpace(selector.WorkspaceID)
	case memcontract.ScopeAgent:
		if selector.AgentTier.Normalize() == memcontract.AgentTierWorkspace {
			return strings.TrimSpace(selector.WorkspaceID)
		}
		return ""
	case "":
		return strings.TrimSpace(selector.WorkspaceID)
	default:
		return ""
	}
}

func (h *BaseHandlers) memoryExplicitSearchFallback(
	ctx context.Context,
	selector memorySelector,
	queryText string,
	limit int,
) ([]memcontract.SearchResult, error) {
	if h.MemoryStore == nil {
		return nil, errors.New("memory store is not configured")
	}
	workspace := ""
	switch selector.Scope.Normalize() {
	case memcontract.ScopeWorkspace:
		workspace = strings.TrimSpace(selector.Workspace)
	case memcontract.ScopeGlobal:
		workspace = ""
	default:
		return nil, nil
	}
	return h.MemoryStore.Search(ctx, queryText, memcontract.SearchOptions{
		Scope:     selector.Scope,
		Workspace: workspace,
		Limit:     limit,
	})
}

func memorySearchResultPayloadsFromSearchResults(
	results []memcontract.SearchResult,
	workspaceID string,
) []contract.MemorySearchResultPayload {
	if len(results) == 0 {
		return []contract.MemorySearchResultPayload{}
	}
	normalizedWorkspaceID := strings.TrimSpace(workspaceID)
	payloads := make([]contract.MemorySearchResultPayload, 0, len(results))
	for _, result := range results {
		payloads = append(payloads, contract.MemorySearchResultPayload{
			Memory: contract.MemoryEntrySummaryPayload{
				Filename:    result.Filename,
				Name:        result.Name,
				Description: result.Description,
				Type:        result.Type,
				Scope:       result.Scope,
				WorkspaceID: normalizedWorkspaceID,
				ModTime:     result.ModTime,
			},
			Score:   result.Score,
			Snippet: result.Snippet,
		})
	}
	return payloads
}
