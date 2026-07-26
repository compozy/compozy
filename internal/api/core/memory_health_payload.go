package core

import (
	"context"
	"errors"
	"fmt"

	"net/http"
	"os"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/memory"
	ssepkg "github.com/compozy/agh/internal/sse"

	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) memoryHealth(c *gin.Context) (contract.MemoryHealthPayload, error) {
	return h.memoryHealthSnapshot(
		c.Request.Context(),
		firstNonEmptyString(c.Query("workspace_id"), c.Query("workspace")),
	)
}

func memoryHealthConfigPayload(cfg *aghconfig.Config) contract.MemoryHealthPayload {
	dreamAgent := strings.TrimSpace(cfg.Roles.Dream.Agent)
	if dreamAgent == "" {
		dreamAgent = aghconfig.BuiltinDreamingCuratorAgentName
	}
	return contract.MemoryHealthPayload{
		Status:             memoryHealthStatusOK,
		Enabled:            cfg.Memory.Enabled,
		Configured:         strings.TrimSpace(cfg.Memory.GlobalDir) != "",
		GlobalDir:          strings.TrimSpace(cfg.Memory.GlobalDir),
		DreamAgent:         dreamAgent,
		DreamMinHours:      cfg.Memory.Dream.MinHours,
		DreamMinSessions:   cfg.Memory.Dream.MinSessions,
		DreamCheckInterval: cfg.Memory.Dream.CheckInterval.String(),
	}
}

func (h *BaseHandlers) memoryHealthSnapshot(
	ctx context.Context,
	rawWorkspace string,
) (contract.MemoryHealthPayload, error) {
	payload := memoryHealthConfigPayload(&h.Config)
	if !payload.Enabled {
		payload.Status = memoryHealthStatusDisabled
		payload.Reason = "memory is disabled"
		return payload, nil
	}
	if h.DreamTrigger != nil {
		payload.DreamEnabled = h.DreamTrigger.Enabled()
		lastConsolidation, err := h.DreamTrigger.LastConsolidatedAt()
		if err != nil {
			payload.Status = memoryHealthStatusDegraded
			payload.Reason = err.Error()
		} else if !lastConsolidation.IsZero() {
			lastConsolidation = lastConsolidation.UTC()
			payload.LastConsolidation = &lastConsolidation
		}
	}
	if h.MemoryStore == nil {
		payload.Status = memoryHealthStatusUnavailable
		payload.Configured = false
		payload.Reason = "memory store is not configured"
		return payload, nil
	}

	globalCount, err := h.MemoryStore.SourceHeaderCount(memcontract.ScopeGlobal)
	if err != nil {
		payload.Status = memoryHealthStatusUnavailable
		payload.Reason = err.Error()
		return payload, nil
	}
	payload.GlobalFiles = globalCount

	workspaces, err := h.memoryHealthWorkspaces(
		ctx,
		rawWorkspace,
	)
	if err != nil {
		return contract.MemoryHealthPayload{}, err
	}
	payload.WorkspaceCount = len(workspaces)
	for _, workspace := range workspaces {
		store := h.MemoryStore.ForWorkspace(workspace)
		count, err := store.SourceHeaderCount(memcontract.ScopeWorkspace)
		if err != nil {
			payload.Status = memoryHealthStatusDegraded
			payload.Reason = err.Error()
			return payload, nil
		}
		payload.WorkspaceFiles += count
	}

	stats, err := h.MemoryStore.HealthStats(ctx, workspaces)
	if err != nil {
		payload.Status = memoryHealthStatusDegraded
		payload.Reason = err.Error()
		return payload, nil
	}
	payload.IndexedFiles = stats.IndexedFiles
	payload.OrphanedFiles = stats.OrphanedFiles
	payload.LastReindex = stats.LastReindex
	payload.OperationCount = stats.OperationCount
	payload.LastOperationAt = stats.LastOperationAt
	if payload.Status == memoryHealthStatusOK && payload.OrphanedFiles > 0 {
		payload.Status = memoryHealthStatusDegraded
		payload.Reason = "memory catalog has orphaned files"
	}

	return payload, nil
}

func (h *BaseHandlers) respondDecisionRejected(c *gin.Context, decision memcontract.Decision) {
	reason := strings.TrimSpace(decision.Reason)
	if reason == "" {
		reason = "memory write rejected by policy"
	}
	err := fmt.Errorf("%w: %s", ErrMemoryRejected, reason)
	h.respondMemoryError(c, StatusForMemoryError(err), err, map[string]any{
		"decision": MemoryDecisionPayload(decision, nil),
	})
}

func (h *BaseHandlers) respondMemoryError(c *gin.Context, status int, err error, details map[string]any) {
	if status == http.StatusOK {
		status = StatusForMemoryError(err)
	}
	payload := contract.MemoryErrorPayload{
		Code:    memoryErrorCodeForStatus(status, err),
		Message: memoryErrorMessage(status, err, h.MaskInternalErrors),
		Details: cloneDetails(details),
	}
	c.JSON(status, payload)
}

func (h *BaseHandlers) respondUnsupportedMemoryOperation(c *gin.Context, operation string) {
	normalized := strings.TrimSpace(operation)
	if normalized == "" {
		normalized = unknownValue
	}
	err := fmt.Errorf("%w: %s", ErrMemoryUnsupported, normalized)
	h.respondMemoryError(c, memoryUnsupportedStatus, err, map[string]any{handlersOperationKey: normalized})
}

func memoryErrorCodeForStatus(status int, err error) string {
	switch {
	case errors.Is(err, ErrMemoryUnsupported):
		return memoryErrorCodeUnsupported
	case errors.Is(err, ErrMemoryRejected):
		return memoryErrorCodeRejected
	case errors.Is(err, os.ErrNotExist):
		return memoryErrorCodeNotFound
	case errors.Is(err, memory.ErrValidation):
		return memoryErrorCodeValidation
	case status == http.StatusNotFound:
		return memoryErrorCodeNotFound
	case status == http.StatusBadRequest:
		return memoryErrorCodeValidation
	default:
		return memoryErrorCodeInternal
	}
}

func memoryErrorMessage(status int, err error, maskInternal bool) string {
	message := http.StatusText(status)
	if err != nil && (!maskInternal || status < http.StatusInternalServerError) {
		message = err.Error()
	}
	if strings.TrimSpace(message) == "" {
		message = "memory request failed"
	}
	return ssepkg.ScrubMemoryContextString(message)
}

func cloneDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[strings.TrimSpace(key)] = value
	}
	return cloned
}

// MemoryOperationPayloads converts domain operation records into API DTOs.
func MemoryOperationPayloads(records []memcontract.OperationRecord) []contract.MemoryOperationPayload {
	payloads := make([]contract.MemoryOperationPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, contract.MemoryOperationPayload{
			ID:        strings.TrimSpace(record.ID),
			Operation: string(record.Operation.Normalize()),
			Scope:     string(record.Scope.Normalize()),
			Workspace: strings.TrimSpace(record.Workspace),
			Filename:  strings.TrimSpace(record.Filename),
			AgentName: strings.TrimSpace(record.AgentName),
			Summary:   strings.TrimSpace(ssepkg.ScrubMemoryContextString(record.Summary)),
			Timestamp: record.Timestamp.UTC(),
		})
	}
	return payloads
}

// MemoryOperationHistoryPayloads converts domain operation records into Memory v2 DTOs.
func MemoryOperationHistoryPayloads(records []memcontract.OperationRecord) []contract.MemoryOperationHistoryPayload {
	payloads := make([]contract.MemoryOperationHistoryPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, contract.MemoryOperationHistoryPayload{
			ID:          strings.TrimSpace(record.ID),
			Operation:   record.Operation.Normalize(),
			Scope:       record.Scope.Normalize(),
			WorkspaceID: strings.TrimSpace(record.Workspace),
			AgentName:   strings.TrimSpace(record.AgentName),
			Filename:    strings.TrimSpace(record.Filename),
			Summary:     strings.TrimSpace(ssepkg.ScrubMemoryContextString(record.Summary)),
			Timestamp:   record.Timestamp.UTC(),
		})
	}
	return payloads
}

// MemoryDecisionPayload converts a controller decision into its redaction-safe public form.
func MemoryDecisionPayload(
	decision memcontract.Decision,
	appliedAt *time.Time,
) contract.MemoryDecisionPayload {
	return contract.MemoryDecisionPayload{
		ID:              strings.TrimSpace(decision.ID),
		CandidateHash:   strings.TrimSpace(decision.CandidateHash),
		IdempotencyKey:  strings.TrimSpace(decision.IdempotencyKey),
		Op:              contract.MemoryDecisionOp(decision.Op.String()),
		Scope:           decision.Frontmatter.Scope.Normalize(),
		AgentName:       strings.TrimSpace(decision.Frontmatter.AgentName),
		AgentTier:       decision.Frontmatter.AgentTier.Normalize(),
		Targets:         cloneStrings(decision.Targets),
		TargetFilename:  strings.TrimSpace(decision.TargetFilename),
		Frontmatter:     decision.Frontmatter,
		PostContentHash: strings.TrimSpace(decision.PostContentHash),
		Confidence:      decision.Confidence,
		Source:          decision.Source.Normalize(),
		RuleTrace:       cloneRuleHits(decision.RuleTrace),
		LLMTrace:        memoryLLMTracePayload(decision.LLMTrace),
		Reason:          strings.TrimSpace(ssepkg.ScrubMemoryContextString(decision.Reason)),
		PromptVersion:   strings.TrimSpace(decision.PromptVersion),
		AppliedAt:       appliedAt,
		DecidedAt:       decision.DecidedAt.UTC(),
	}
}

func MemoryDecisionRecordPayload(record memory.DecisionRecord) contract.MemoryDecisionPayload {
	payload := MemoryDecisionPayload(record.Decision, record.AppliedAt)
	payload.WorkspaceID = strings.TrimSpace(record.WorkspaceID)
	payload.AgentName = firstNonEmptyString(payload.AgentName, record.AgentName)
	payload.AgentTier = firstNonEmptyAgentTier(payload.AgentTier, record.AgentTier)
	return payload
}

func memoryLLMTracePayload(trace *memcontract.LLMCall) *contract.MemoryLLMTracePayload {
	if trace == nil {
		return nil
	}
	return &contract.MemoryLLMTracePayload{
		Model:         strings.TrimSpace(trace.Model),
		PromptVersion: strings.TrimSpace(trace.PromptVersion),
		LatencyMs:     trace.Latency.Milliseconds(),
		Error:         strings.TrimSpace(ssepkg.ScrubMemoryContextString(trace.Error)),
	}
}

func cloneRuleHits(hits []memcontract.RuleHit) []memcontract.RuleHit {
	if len(hits) == 0 {
		return nil
	}
	cloned := make([]memcontract.RuleHit, len(hits))
	copy(cloned, hits)
	for idx := range cloned {
		cloned[idx].Reason = ssepkg.ScrubMemoryContextString(cloned[idx].Reason)
		cloned[idx].Details = ssepkg.ScrubMemoryContextString(cloned[idx].Details)
	}
	return cloned
}
