package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/sync/errgroup"
)

type ToolExecutor interface {
	Execute(ctx context.Context, toolCalls []llmadapter.ToolCall) ([]llmadapter.ToolResult, error)
	UpdateBudgets(ctx context.Context, results []llmadapter.ToolResult, state *loopState) error
}

type toolExecutor struct {
	registry        ToolRegistry
	cfg             settings
	progressEnabled bool
}

func NewToolExecutor(registry ToolRegistry, cfg *settings) ToolExecutor {
	if cfg == nil {
		cfg = &settings{}
	}
	return &toolExecutor{registry: registry, cfg: *cfg, progressEnabled: cfg.enableProgressTracking}
}

func (e *toolExecutor) Execute(ctx context.Context, toolCalls []llmadapter.ToolCall) ([]llmadapter.ToolResult, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}
	log := logger.FromContext(ctx)
	log.Debug("Executing tool calls", "tool_calls_count", len(toolCalls), "tools", extractToolNames(toolCalls))

	limit := e.cfg.maxConcurrentTools
	sem := make(chan struct{}, limit)
	results := make([]llmadapter.ToolResult, len(toolCalls))
	g, ctx := errgroup.WithContext(ctx)

	for i := range toolCalls {
		i := i
		call := toolCalls[i]
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}
			results[i] = e.executeSingle(ctx, call)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	log.Debug(
		"All tool calls completed",
		"results_count",
		len(results),
		"successful_count",
		countSuccessfulResults(results),
	)
	return results, nil
}

func (e *toolExecutor) executeSingle(ctx context.Context, call llmadapter.ToolCall) llmadapter.ToolResult {
	log := logger.FromContext(ctx)
	log.Debug("Processing tool call", "tool_name", call.Name, "tool_call_id", call.ID)

	t, found := e.registry.Find(ctx, call.Name)
	if !found || t == nil {
		log.Debug("Tool not found", "tool_name", call.Name, "tool_call_id", call.ID)
		errJSON := fmt.Sprintf("{\"error\":\"tool not found: %s\"}", call.Name)
		return llmadapter.ToolResult{
			ID:          call.ID,
			Name:        call.Name,
			Content:     errJSON,
			JSONContent: json.RawMessage(errJSON),
		}
	}

	log.Debug("Executing tool", "tool_name", call.Name, "tool_call_id", call.ID)
	raw, err := t.Call(ctx, string(call.Arguments))
	if err != nil {
		log.Debug(
			"Tool execution failed",
			"tool_name",
			call.Name,
			"tool_call_id",
			call.ID,
			"error",
			core.RedactError(err),
		)
		errObj := ToolExecutionResult{
			Success: false,
			Error: &ToolError{
				Code:    ErrCodeToolExecution,
				Message: "Tool execution failed",
				Details: core.RedactError(err),
			},
		}
		b, merr := json.Marshal(errObj)
		if merr != nil {
			return llmadapter.ToolResult{
				ID:          call.ID,
				Name:        call.Name,
				Content:     `{"success":false,"error":{"code":"TOOL_EXECUTION_ERROR","message":"Tool execution failed"}}`,
				JSONContent: nil,
			}
		}
		return llmadapter.ToolResult{ID: call.ID, Name: call.Name, Content: string(b), JSONContent: json.RawMessage(b)}
	}

	log.Debug("Tool execution succeeded", "tool_name", call.Name, "tool_call_id", call.ID)
	var jsonContent json.RawMessage
	if json.Valid([]byte(raw)) {
		jsonContent = json.RawMessage(raw)
	}
	return llmadapter.ToolResult{ID: call.ID, Name: call.Name, Content: raw, JSONContent: jsonContent}
}

func (e *toolExecutor) UpdateBudgets(ctx context.Context, results []llmadapter.ToolResult, state *loopState) error {
	log := logger.FromContext(ctx)
	budget := e.cfg.maxSequentialToolErrors
	if budget <= 0 {
		budget = defaultMaxSequentialToolErrors
	}
	maxSucc := e.cfg.maxConsecutiveSuccesses
	if maxSucc <= 0 {
		maxSucc = defaultMaxConsecutiveSuccesses
	}

	for _, result := range results {
		name := result.Name
		if isToolResultSuccess(result) {
			if e.progressEnabled {
				var fingerprint string
				if len(result.JSONContent) > 0 {
					fingerprint = stableJSONFingerprint(result.JSONContent)
				} else {
					fingerprint = stableJSONFingerprint([]byte(result.Content))
				}
				if last, ok := state.lastToolResults[name]; ok && last == fingerprint {
					state.toolSuccess[name]++
				} else {
					state.toolSuccess[name] = 1
					state.lastToolResults[name] = fingerprint
				}
			} else {
				state.toolSuccess[name]++
			}

			if state.toolSuccess[name] >= maxSucc {
				log.Warn(
					"Tool called successfully too many times without progress",
					"tool",
					name,
					"consecutive_successes",
					state.toolSuccess[name],
				)
				return fmt.Errorf(
					"tool %s called successfully %d times without progress",
					name,
					state.toolSuccess[name],
				)
			}
			if state.toolErrors[name] > 0 {
				state.toolErrors[name]--
			}
			continue
		}

		state.toolSuccess[name] = 0
		state.toolErrors[name]++
		log.Debug("Tool error recorded", "tool", name, "consecutive_errors", state.toolErrors[name], "max", budget)
		if state.toolErrors[name] >= budget {
			log.Warn("Error budget exceeded - tool", "tool", name, "max", budget)
			return fmt.Errorf("tool error budget exceeded for %s", name)
		}
	}

	return nil
}

func extractToolNames(toolCalls []llmadapter.ToolCall) []string {
	names := make([]string, len(toolCalls))
	for i, call := range toolCalls {
		names[i] = call.Name
	}
	return names
}

func countSuccessfulResults(results []llmadapter.ToolResult) int {
	count := 0
	for _, r := range results {
		if len(r.JSONContent) > 0 {
			if ok, parsed := isSuccessJSONRaw(r.JSONContent); parsed {
				if ok {
					count++
				}
				continue
			}
			if isSuccessText(string(r.JSONContent)) {
				count++
			}
			continue
		}
		if isSuccessText(r.Content) {
			count++
		}
	}
	return count
}

func isSuccessJSONRaw(raw json.RawMessage) (bool, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false, false
	}
	if _, hasErr := obj["error"]; hasErr {
		return false, true
	}
	return true, true
}

func isSuccessText(s string) bool {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "{") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
			if _, hasErr := obj["error"]; hasErr {
				return false
			}
			return true
		}
	}
	lower := strings.ToLower(trimmed)
	for _, indicator := range []string{
		"error",
		"failed",
		"failure",
		"missing required",
		"invalid",
		"not found",
		"unauthorized",
		"forbidden",
		"bad request",
		"exception",
		"cannot",
		"unable to",
	} {
		if strings.Contains(lower, indicator) {
			return false
		}
	}
	return strings.TrimSpace(trimmed) != ""
}

func isToolResultSuccess(r llmadapter.ToolResult) bool {
	if len(r.JSONContent) > 0 {
		if ok, parsed := isSuccessJSONRaw(r.JSONContent); parsed {
			return ok
		}
		return isSuccessText(string(r.JSONContent))
	}
	return isSuccessText(r.Content)
}
