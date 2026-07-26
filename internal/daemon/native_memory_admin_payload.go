package daemon

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/frontmatter"
	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	yaml "github.com/goccy/go-yaml"
)

func nativeMemoryAdminHeaderAndBody(content []byte) (memcontract.Header, string, error) {
	var header memcontract.Header
	body, err := frontmatter.Decode(content, func(metadata []byte) error {
		if err := yaml.Unmarshal(metadata, &header); err != nil {
			return fmt.Errorf("memory: decode memory frontmatter: %w", err)
		}
		return nil
	})
	if err != nil {
		return memcontract.Header{}, "", core.NewMemoryValidationError(err)
	}
	if err := header.Validate(); err != nil {
		return memcontract.Header{}, "", core.NewMemoryValidationError(err)
	}
	return header, strings.TrimSpace(body), nil
}

func nativeMemoryAdminDreamRetryTarget(id toolspkg.ToolID, input memoryAdminDreamRetryInput) error {
	failureID := strings.TrimSpace(input.FailureID)
	dreamID := strings.TrimSpace(input.DreamID)
	if (failureID == "") == (dreamID == "") {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"exactly one of failure_id or dream_id is required",
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return nil
}

func memoryAdminDreamPayload(record memorypkg.DreamRunRecord) contract.MemoryDreamPayload {
	return contract.MemoryDreamPayload{
		ID:             strings.TrimSpace(record.ID),
		Status:         memoryAdminDreamState(record),
		Scope:          record.Scope.Normalize(),
		WorkspaceID:    strings.TrimSpace(record.WorkspaceID),
		AgentName:      strings.TrimSpace(record.AgentName),
		AgentTier:      record.AgentTier.Normalize(),
		CandidateCount: record.InputCount,
		PromotedCount:  record.PromotedCount,
		FailureReason:  firstNonEmpty(record.Error, record.Metadata["reason"]),
		StartedAt:      record.StartedAt.UTC(),
		CompletedAt:    record.FinishedAt,
	}
}

func memoryAdminDreamState(record memorypkg.DreamRunRecord) contract.MemoryDreamState {
	switch strings.TrimSpace(record.Status) {
	case daemonRuntimeStatusRunning:
		return contract.MemoryDreamStateRunning
	case nativeMemoryAdminToolsFailedKey:
		return contract.MemoryDreamStateFailed
	case nativeMemoryAdminToolsCompletedKey:
		if record.PromotedCount > 0 {
			return contract.MemoryDreamStatePromoted
		}
		return contract.MemoryDreamStateSkipped
	case nativeMemoryAdminToolsCanceledKey:
		return contract.MemoryDreamStateSkipped
	default:
		return contract.MemoryDreamStateIdle
	}
}

func memoryAdminDailyLogPayload(record memorypkg.DailyLogRecord) contract.MemoryDailyLogPayload {
	selector := string(record.Scope.Normalize())
	if selector == "" {
		selector = "all"
	}
	return contract.MemoryDailyLogPayload{
		Date:           strings.TrimSpace(record.Date),
		Scope:          record.Scope.Normalize(),
		WorkspaceID:    strings.TrimSpace(record.WorkspaceID),
		AgentName:      strings.TrimSpace(record.AgentName),
		AgentTier:      record.AgentTier.Normalize(),
		Path:           "memory://daily/" + strings.TrimSpace(record.Date) + "/" + selector,
		OperationCount: record.OperationCount,
	}
}

func nativeMemoryAdminLimit(id toolspkg.ToolID, limit int) error {
	if limit < 0 {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"limit must be zero or positive",
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return nil
}

func nativeMemoryAdminToolError(id toolspkg.ToolID, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, memorypkg.ErrValidation):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, toolspkg.ErrToolInvalidInput):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, core.ErrMemoryRejected):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonPolicyDenied,
		)
	case errors.Is(err, toolspkg.ErrToolDenied):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonPolicyDenied,
		)
	case errors.Is(err, core.ErrMemoryUnsupported):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	case errors.Is(err, os.ErrNotExist):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonToolUnknown,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			err,
			toolspkg.ReasonBackendUnhealthy,
		)
	}
}
