package core

import (
	"fmt"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/memory"

	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) memoryDreamListQuery(c *gin.Context) (memory.DreamRunListQuery, error) {
	selector, err := h.resolveMemorySelector(c.Request.Context(), memorySelectorFromQuery(c), false)
	if err != nil {
		return memory.DreamRunListQuery{}, err
	}
	limit, err := parseMemoryLimit(c.Query("limit"))
	if err != nil {
		return memory.DreamRunListQuery{}, err
	}
	return memory.DreamRunListQuery{
		Scope:       selector.Scope,
		WorkspaceID: selector.WorkspaceID,
		AgentName:   selector.AgentName,
		AgentTier:   selector.AgentTier,
		Limit:       limit,
	}, nil
}

func (h *BaseHandlers) memoryDailyLogListQuery(c *gin.Context) (memory.DailyLogListQuery, error) {
	selector, err := h.resolveMemorySelector(c.Request.Context(), memorySelectorFromQuery(c), false)
	if err != nil {
		return memory.DailyLogListQuery{}, err
	}
	limit, err := parseMemoryLimit(c.Query("limit"))
	if err != nil {
		return memory.DailyLogListQuery{}, err
	}
	date := strings.TrimSpace(c.Query("date"))
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return memory.DailyLogListQuery{}, NewMemoryValidationError(err)
		}
	}
	return memory.DailyLogListQuery{
		Date:        date,
		Scope:       selector.Scope,
		WorkspaceID: selector.WorkspaceID,
		AgentName:   selector.AgentName,
		AgentTier:   selector.AgentTier,
		Limit:       limit,
	}, nil
}

func memoryDreamPayload(record memory.DreamRunRecord) contract.MemoryDreamPayload {
	return contract.MemoryDreamPayload{
		ID:             strings.TrimSpace(record.ID),
		Status:         memoryDreamState(record),
		Scope:          record.Scope.Normalize(),
		WorkspaceID:    strings.TrimSpace(record.WorkspaceID),
		AgentName:      strings.TrimSpace(record.AgentName),
		AgentTier:      record.AgentTier.Normalize(),
		CandidateCount: record.InputCount,
		PromotedCount:  record.PromotedCount,
		FailureReason:  firstNonEmptyString(record.Error, record.Metadata["reason"]),
		StartedAt:      record.StartedAt.UTC(),
		CompletedAt:    record.FinishedAt,
	}
}

func memoryDreamState(record memory.DreamRunRecord) contract.MemoryDreamState {
	switch strings.TrimSpace(record.Status) {
	case statusStateRunning:
		return contract.MemoryDreamStateRunning
	case "failed":
		return contract.MemoryDreamStateFailed
	case toolInvokeStatusCompleted:
		if record.PromotedCount > 0 {
			return contract.MemoryDreamStatePromoted
		}
		return contract.MemoryDreamStateSkipped
	case toolInvokeStatusCanceled:
		return contract.MemoryDreamStateSkipped
	default:
		return contract.MemoryDreamStateIdle
	}
}

func memoryDailyLogPayload(record memory.DailyLogRecord) contract.MemoryDailyLogPayload {
	return contract.MemoryDailyLogPayload{
		Date:           strings.TrimSpace(record.Date),
		Scope:          record.Scope.Normalize(),
		WorkspaceID:    strings.TrimSpace(record.WorkspaceID),
		AgentName:      strings.TrimSpace(record.AgentName),
		AgentTier:      record.AgentTier.Normalize(),
		Path:           memoryDailyLogPath(record),
		OperationCount: record.OperationCount,
	}
}

func memoryDailyLogPath(record memory.DailyLogRecord) string {
	selector := string(record.Scope.Normalize())
	if selector == "" {
		selector = "all"
	}
	return "memory://daily/" + strings.TrimSpace(record.Date) + "/" + selector
}

func memoryAdhocSelector(req contract.MemoryAdhocNoteRequest) memorySelector {
	scope := req.Scope.Normalize()
	if scope == "" {
		switch {
		case strings.TrimSpace(req.AgentName) != "":
			scope = memcontract.ScopeAgent
		case strings.TrimSpace(req.WorkspaceID) != "":
			scope = memcontract.ScopeWorkspace
		default:
			scope = memcontract.ScopeGlobal
		}
	}
	return memorySelector{
		Scope:       scope,
		WorkspaceID: req.WorkspaceID,
		AgentName:   req.AgentName,
		AgentTier:   req.AgentTier,
	}
}

func memoryTypeForScope(scope memcontract.Scope) memcontract.Type {
	if scope.Normalize() == memcontract.ScopeWorkspace {
		return memcontract.TypeProject
	}
	return memcontract.TypeUser
}

func memoryAdhocFilename(rawSlug string, content string, at time.Time) string {
	slug := memorySlug(firstNonEmptyString(rawSlug, content, "note"))
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return fmt.Sprintf("ad_hoc_%s_%s.md", at.UTC().Format("20060102T150405Z"), slug)
}

func memoryAdhocDescription(content string) string {
	firstLine := strings.TrimSpace(strings.Split(strings.TrimSpace(content), "\n")[0])
	if len(firstLine) > 96 {
		firstLine = strings.TrimSpace(firstLine[:96])
	}
	if firstLine == "" {
		return "Ad-hoc memory note"
	}
	return firstLine
}

func memorySlug(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, ch := range strings.ToLower(strings.TrimSpace(value)) {
		isAlpha := ch >= 'a' && ch <= 'z'
		isDigit := ch >= '0' && ch <= '9'
		if isAlpha || isDigit {
			builder.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "note"
	}
	const maxMemorySlugLength = 48
	if len(slug) > maxMemorySlugLength {
		slug = strings.Trim(slug[:maxMemorySlugLength], "-")
	}
	if slug == "" {
		return "note"
	}
	return slug
}
