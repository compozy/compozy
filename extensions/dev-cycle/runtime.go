package devcycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	watchpkg "github.com/compozy/agh/internal/loop/watch"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type runtimeProvider struct {
	coderabbit *codeRabbitProvider
	git        *gitProvider
}

func newRuntimeProvider() (*runtimeProvider, error) {
	provider := &runtimeProvider{
		coderabbit: newCodeRabbitProvider(defaultCommandRunner{}),
		git:        newGitProvider(defaultCommandRunner{}),
	}
	if provider.coderabbit == nil || provider.git == nil {
		return nil, errors.New("dev-cycle: runtime dependencies are required")
	}
	return provider, nil
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
	case toolFetchUnresolved:
		var input codeRabbitFetchInput
		if err := decodeToolInput(req.Input, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := p.coderabbit.FetchUnresolved(ctx, input)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload = result
	case toolResolveThreads:
		var input codeRabbitResolveInput
		if err := decodeToolInput(req.Input, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := p.coderabbit.ResolveThreads(ctx, input)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload = result
	case toolGitPush:
		var input gitPushInput
		if err := decodeToolInput(req.Input, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := p.git.Push(ctx, input)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload = result
	default:
		return toolspkg.ToolResult{}, fmt.Errorf("dev-cycle: unsupported tool handler %q", req.Handler)
	}
	return toolResult(payload, started)
}

func (p *runtimeProvider) Poll(ctx context.Context, req watchpkg.PollRequest) (watchpkg.PollResponse, error) {
	var spec codeRabbitWatchSpec
	if err := json.Unmarshal(req.Spec, &spec); err != nil {
		return watchpkg.PollResponse{}, fmt.Errorf("dev-cycle: decode watch spec: %w", err)
	}
	if spec.Kind != watchKindCodeRabbitPR {
		return watchpkg.PollResponse{}, fmt.Errorf("dev-cycle: unsupported watch kind %q", spec.Kind)
	}
	return p.coderabbit.Poll(ctx, req, spec)
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
