package devcycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

type runtimeProvider struct {
	artifacts *reviewArtifactStore
}

func newRuntimeProvider() *runtimeProvider {
	return &runtimeProvider{artifacts: newReviewArtifactStore()}
}

func (p *runtimeProvider) ProvideTools() ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	return runtimeToolDescriptors()
}

func (p *runtimeProvider) CallTool(
	ctx context.Context,
	req toolspkg.ExtensionToolCallRequest,
) (toolspkg.ToolResult, error) {
	started := time.Now()
	var payload any
	switch req.Handler {
	case toolImportTasks:
		var input importTasksInput
		if err := decodeToolInput(req.Input, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := importTasks(input)
		if err != nil {
			return toolspkg.ToolResult{}, importTasksToolError(req.ToolID, input, err)
		}
		payload = result
	case toolWriteArtifacts:
		var input writeReviewArtifactsInput
		if err := decodeToolInput(req.Input, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := p.artifacts.Write(ctx, req.TrustedWorkspace, input)
		if err != nil {
			return toolspkg.ToolResult{}, reviewArtifactToolError(req.ToolID, err)
		}
		payload = result
	case toolFinalizeRound:
		var input finalizeReviewRoundInput
		if err := decodeToolInput(req.Input, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := p.artifacts.Finalize(ctx, req.TrustedWorkspace, input)
		if err != nil {
			return toolspkg.ToolResult{}, reviewArtifactToolError(req.ToolID, err)
		}
		payload = result
	default:
		return toolspkg.ToolResult{}, fmt.Errorf("dev-cycle: unsupported tool handler %q", req.Handler)
	}
	return toolResult(payload, started)
}

func reviewArtifactToolError(id toolspkg.ToolID, err error) error {
	if !errors.Is(err, errReviewArtifactInput) {
		return err
	}
	cause := err.Error()
	return toolspkg.NewOperatorToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		cause,
		errors.Join(toolspkg.ErrToolInvalidInput, err),
		cause,
		"Correct the named field and retry the review round.",
		toolspkg.ReasonSchemaInvalid,
	)
}

func decodeToolInput(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("dev-cycle: decode tool input: %w", err)
	}
	return nil
}

func toolResult(payload any, started time.Time) (toolspkg.ToolResult, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return toolspkg.ToolResult{}, fmt.Errorf("dev-cycle: encode tool result: %w", err)
	}
	return toolspkg.ToolResult{
		Structured: json.RawMessage(encoded),
		Preview:    toolPreview(encoded),
		Bytes:      int64(len(encoded)),
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func toolPreview(encoded []byte) string {
	const maxPreviewBytes = 240
	if len(encoded) <= maxPreviewBytes {
		return string(encoded)
	}
	return string(encoded[:maxPreviewBytes]) + "..."
}
