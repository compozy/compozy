package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) ListMemoryDreams(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}
	query, err := h.memoryDreamListQuery(c)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	records, err := h.MemoryStore.ListDreamRunRecords(c.Request.Context(), query)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	payloads := make([]contract.MemoryDreamPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, memoryDreamPayload(record))
	}
	c.JSON(http.StatusOK, contract.MemoryDreamListResponse{Dreams: payloads})
}

func (h *BaseHandlers) GetMemoryDream(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}
	record, err := h.MemoryStore.LoadDreamRunRecord(c.Request.Context(), c.Param("dream_id"))
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryDreamResponse{Dream: memoryDreamPayload(record)})
}

func (h *BaseHandlers) RetryMemoryDream(c *gin.Context) {
	var req contract.MemoryDreamRetryRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory dream retry request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	runID := firstNonEmptyString(req.FailureID, c.Param("dream_id"))
	if h.DreamTrigger == nil || !h.DreamTrigger.Enabled() {
		c.JSON(http.StatusOK, contract.MemoryDreamRetryResponse{
			Dream: contract.MemoryDreamPayload{
				ID:        strings.TrimSpace(runID),
				Status:    contract.MemoryDreamStateSkipped,
				StartedAt: h.nowUTC(),
			},
			Retried: false,
		})
		return
	}
	triggered, reason, err := h.DreamTrigger.Trigger(c.Request.Context(), "")
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	status := contract.MemoryDreamStateSkipped
	if triggered {
		status = contract.MemoryDreamStateRunning
	}
	c.JSON(http.StatusOK, contract.MemoryDreamRetryResponse{
		Dream: contract.MemoryDreamPayload{
			ID:            strings.TrimSpace(runID),
			Status:        status,
			FailureReason: strings.TrimSpace(reason),
			StartedAt:     h.nowUTC(),
		},
		Retried: triggered,
	})
}

// GetMemoryDreamStatus returns a truthful empty status until daemon wiring
// provides live dreaming runtime state.
func (h *BaseHandlers) GetMemoryDreamStatus(c *gin.Context) {
	c.JSON(http.StatusOK, contract.MemoryDreamListResponse{Dreams: []contract.MemoryDreamPayload{}})
}

func (h *BaseHandlers) ListMemoryDailyLogs(c *gin.Context) {
	if h.MemoryStore == nil {
		h.respondMemoryError(c, http.StatusInternalServerError, errors.New("memory store is not configured"), nil)
		return
	}
	query, err := h.memoryDailyLogListQuery(c)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	records, err := h.MemoryStore.ListDailyLogRecords(c.Request.Context(), query)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	payloads := make([]contract.MemoryDailyLogPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, memoryDailyLogPayload(record))
	}
	c.JSON(http.StatusOK, contract.MemoryDailyLogListResponse{Logs: payloads})
}

func (h *BaseHandlers) GetMemoryExtractorStatus(c *gin.Context) {
	if h.MemoryExtractor == nil {
		c.JSON(http.StatusOK, contract.MemoryExtractorStatusResponse{
			Extractor: contract.MemoryExtractorStatusPayload{Status: contract.MemoryExtractorStateStopped},
		})
		return
	}
	status, err := h.MemoryExtractor.Status(c.Request.Context())
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryExtractorStatusResponse{
		Extractor: status,
	})
}

func (h *BaseHandlers) ListMemoryExtractorFailures(c *gin.Context) {
	if h.MemoryExtractor == nil {
		c.JSON(http.StatusOK, contract.MemoryExtractorFailuresResponse{
			Failures: []contract.MemoryExtractorFailurePayload{},
		})
		return
	}
	failures, err := h.MemoryExtractor.ListFailures(c.Request.Context())
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryExtractorFailuresResponse{Failures: failures})
}

func (h *BaseHandlers) RetryMemoryExtractor(c *gin.Context) {
	if h.MemoryExtractor == nil {
		h.respondUnsupportedMemoryOperation(c, "retryMemoryExtractor")
		return
	}
	var req contract.MemoryExtractorRetryRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondMemoryError(c, http.StatusBadRequest, err, nil)
		return
	}
	response, err := h.MemoryExtractor.Retry(c.Request.Context(), req)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) DrainMemoryExtractor(c *gin.Context) {
	if h.MemoryExtractor == nil {
		h.respondUnsupportedMemoryOperation(c, "drainMemoryExtractor")
		return
	}
	response, err := h.MemoryExtractor.Drain(c.Request.Context())
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) ListMemoryProviders(c *gin.Context) {
	if h.MemoryProviders != nil {
		providers, err := h.MemoryProviders.List(c.Request.Context(), strings.TrimSpace(c.Query("workspace_id")))
		if err != nil {
			h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
			return
		}
		c.JSON(http.StatusOK, contract.MemoryProviderListResponse{Providers: providers})
		return
	}
	c.JSON(http.StatusOK, contract.MemoryProviderListResponse{Providers: h.memoryProviderPayloads()})
}

func (h *BaseHandlers) GetMemoryProvider(c *gin.Context) {
	name := strings.TrimSpace(c.Param("provider_name"))
	if h.MemoryProviders != nil {
		provider, err := h.MemoryProviders.Get(c.Request.Context(), strings.TrimSpace(c.Query("workspace_id")), name)
		if err != nil {
			h.respondMemoryError(c, StatusForMemoryError(err), err, map[string]any{"provider_name": name})
			return
		}
		c.JSON(http.StatusOK, contract.MemoryProviderResponse{Provider: provider})
		return
	}
	for _, provider := range h.memoryProviderPayloads() {
		if provider.Name == name {
			c.JSON(http.StatusOK, contract.MemoryProviderResponse{Provider: provider})
			return
		}
	}
	err := fmt.Errorf("%w: provider %q not found", os.ErrNotExist, name)
	h.respondMemoryError(c, StatusForMemoryError(err), err, map[string]any{"provider_name": name})
}

func (h *BaseHandlers) SelectMemoryProvider(c *gin.Context) {
	if h.MemoryProviders == nil {
		h.respondUnsupportedMemoryOperation(c, "selectMemoryProvider")
		return
	}
	var req contract.MemoryProviderSelectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(c, http.StatusBadRequest, err, nil)
		return
	}
	provider, err := h.MemoryProviders.Select(
		c.Request.Context(),
		strings.TrimSpace(c.Query("workspace_id")),
		firstNonEmptyString(req.Name, c.Param("provider_name")),
	)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryProviderResponse{Provider: provider})
}

func (h *BaseHandlers) EnableMemoryProvider(c *gin.Context) {
	if h.MemoryProviders == nil {
		h.respondUnsupportedMemoryOperation(c, "enableMemoryProvider")
		return
	}
	var req contract.MemoryProviderLifecycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(c, http.StatusBadRequest, err, nil)
		return
	}
	response, err := h.MemoryProviders.Enable(
		c.Request.Context(),
		strings.TrimSpace(c.Query("workspace_id")),
		strings.TrimSpace(c.Param("provider_name")),
		req.Reason,
	)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) DisableMemoryProvider(c *gin.Context) {
	if h.MemoryProviders == nil {
		h.respondUnsupportedMemoryOperation(c, "disableMemoryProvider")
		return
	}
	var req contract.MemoryProviderLifecycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(c, http.StatusBadRequest, err, nil)
		return
	}
	response, err := h.MemoryProviders.Disable(
		c.Request.Context(),
		strings.TrimSpace(c.Query("workspace_id")),
		strings.TrimSpace(c.Param("provider_name")),
		req.Reason,
	)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) CreateMemoryAdhocNote(c *gin.Context) {
	var req contract.MemoryAdhocNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode memory ad-hoc note request: %w", h.transportName(), err),
			nil,
		)
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		err := NewMemoryValidationError(errors.New("content is required"))
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	selector := memoryAdhocSelector(req)
	resolved, err := h.resolveMemorySelector(c.Request.Context(), selector, true)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	store, err := h.memoryRecallStoreForSelector(c.Request.Context(), resolved)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	createdAt := h.nowUTC()
	filename := memoryAdhocFilename(req.Slug, content, createdAt)
	decision, err := store.ProposeCandidate(c.Request.Context(), memcontract.Candidate{
		WorkspaceID: resolved.WorkspaceID,
		Scope:       resolved.Scope,
		AgentName:   resolved.AgentName,
		AgentTier:   resolved.AgentTier,
		Origin:      h.memoryOrigin(),
		Content:     content,
		Frontmatter: memcontract.Header{
			Name:        "Ad Hoc Memory Note",
			Description: memoryAdhocDescription(content),
			Type:        memoryTypeForScope(resolved.Scope),
			Scope:       resolved.Scope,
			AgentName:   resolved.AgentName,
			AgentTier:   resolved.AgentTier,
		},
		Metadata: map[string]string{
			memoryMetadataTargetFilenameKey: filename,
		},
		SubmittedAt: createdAt,
	})
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	if decision.Op == memcontract.OpReject {
		h.respondDecisionRejected(c, decision)
		return
	}
	c.JSON(http.StatusOK, contract.MemoryAdhocNoteResponse{
		Path:      firstNonEmptyString(decision.TargetFilename, filename),
		Accepted:  memoryDecisionApplied(decision),
		CreatedAt: createdAt,
	})
}

func (h *BaseHandlers) GetMemorySessionLedger(c *gin.Context) {
	if h.MemorySessionLedger == nil {
		h.respondUnsupportedMemoryOperation(c, "getMemorySessionLedger")
		return
	}
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	response, err := h.MemorySessionLedger.Get(c.Request.Context(), sessionID)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) ReplayMemorySession(c *gin.Context) {
	if h.MemorySessionLedger == nil {
		h.respondUnsupportedMemoryOperation(c, "replayMemorySession")
		return
	}
	var req contract.MemorySessionReplayRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondMemoryError(c, http.StatusBadRequest, err, nil)
		return
	}
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	response, err := h.MemorySessionLedger.Replay(c.Request.Context(), sessionID, req)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) PruneMemorySessions(c *gin.Context) {
	if h.MemorySessionLedger == nil {
		h.respondUnsupportedMemoryOperation(c, "pruneMemorySessions")
		return
	}
	var req contract.MemorySessionsPruneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondMemoryError(c, http.StatusBadRequest, err, nil)
		return
	}
	response, err := h.MemorySessionLedger.Prune(c.Request.Context(), req)
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) RepairMemorySessions(c *gin.Context) {
	if h.MemorySessionLedger == nil {
		h.respondUnsupportedMemoryOperation(c, "repairMemorySessions")
		return
	}
	response, err := h.MemorySessionLedger.Repair(c.Request.Context())
	if err != nil {
		h.respondMemoryError(c, StatusForMemoryError(err), err, nil)
		return
	}
	c.JSON(http.StatusOK, response)
}
