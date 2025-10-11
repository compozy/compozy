package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/engine/schema"
	"golang.org/x/sync/errgroup"
)

type ToolExecutor interface {
	Execute(ctx context.Context, toolCalls []llmadapter.ToolCall) ([]llmadapter.ToolResult, error)
	UpdateBudgets(ctx context.Context, results []llmadapter.ToolResult, state *loopState) error
}

type toolExecutor struct {
	registry ToolRegistry
	cfg      settings
}

const toolExecutionErrorPayload = `{"success":false,"error":{"code":"TOOL_EXECUTION_ERROR","message":"Tool execution failed"}}`

func NewToolExecutor(registry ToolRegistry, cfg *settings) ToolExecutor {
	if cfg == nil {
		cfg = &settings{}
	}
	return &toolExecutor{registry: registry, cfg: *cfg}
}

func (e *toolExecutor) Execute(ctx context.Context, toolCalls []llmadapter.ToolCall) ([]llmadapter.ToolResult, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}
	log := telemetry.Logger(ctx)
	log.Info("Executing tool calls", "tool_calls_count", len(toolCalls), "tools", extractToolNames(toolCalls))

	limit := e.cfg.maxConcurrentTools
	if limit <= 0 {
		limit = defaultMaxConcurrentTools
	}
	results := make([]llmadapter.ToolResult, len(toolCalls))
	workerCount := min(limit, len(toolCalls))
	if workerCount == 0 {
		workerCount = 1
	}
	g, workerCtx := errgroup.WithContext(ctx)
	jobs := make(chan struct {
		index int
		call  llmadapter.ToolCall
	})

	for w := 0; w < workerCount; w++ {
		g.Go(func() error {
			for job := range jobs {
				select {
				case <-workerCtx.Done():
					return workerCtx.Err()
				default:
				}
				results[job.index] = e.executeSingle(workerCtx, job.call)
			}
			return nil
		})
	}

	g.Go(func() error {
		defer close(jobs)
		for i, call := range toolCalls {
			select {
			case jobs <- struct {
				index int
				call  llmadapter.ToolCall
			}{index: i, call: call}:
			case <-workerCtx.Done():
				return workerCtx.Err()
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	log.Info(
		"All tool calls completed",
		"results_count", len(results),
		"successful_count", countSuccessfulResults(results),
	)
	return results, nil
}

func (e *toolExecutor) executeSingle(ctx context.Context, call llmadapter.ToolCall) llmadapter.ToolResult {
	log := telemetry.Logger(ctx)
	log.Info("Processing tool call", "tool_name", call.Name, "tool_call_id", call.ID)
	start := time.Now()
	capture := telemetry.CaptureContentEnabled(ctx)
	entry := telemetry.ToolLogEntry{
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Status:     telemetry.ToolStatusSuccess,
	}
	if capture {
		entry.Input = string(call.Arguments)
	} else if len(call.Arguments) > 0 {
		entry.Input = telemetry.RedactedValue
		entry.Redacted = true
	}
	defer func() {
		entry.Duration = time.Since(start)
		telemetry.RecordTool(ctx, &entry)
	}()

	t, found := e.registry.Find(ctx, call.Name)
	if !found || t == nil {
		return e.toolNotFoundResult(ctx, call, capture, &entry)
	}

	log.Info("Executing tool", "tool_name", call.Name, "tool_call_id", call.ID)
	if err := e.validateToolArguments(ctx, t, call); err != nil {
		entry.Status = telemetry.ToolStatusError
		entry.Error = core.RedactError(err)
		result := e.toolInvalidInputResult(call, err, capture, &entry)
		log.Warn(
			"Tool arguments failed validation",
			"tool_name", call.Name,
			"tool_call_id", call.ID,
			"error", core.RedactError(err),
		)
		return result
	}
	if _, err := core.GetRequestID(ctx); err != nil {
		ctx = core.WithRequestID(ctx, call.ID)
	}
	raw, err := t.Call(ctx, string(call.Arguments))
	if err != nil {
		entry.Status = telemetry.ToolStatusError
		entry.Error = core.RedactError(err)
		result := e.toolExecutionErrorResult(call, err, capture, &entry)
		log.Error(
			"Tool execution failed",
			"tool_name", call.Name,
			"tool_call_id", call.ID,
			"error", core.RedactError(err),
		)
		return result
	}

	log.Info("Tool execution succeeded", "tool_name", call.Name, "tool_call_id", call.ID)
	return e.buildSuccessResult(call, raw, capture, &entry)
}

func (e *toolExecutor) validateToolArguments(
	ctx context.Context,
	tool RegistryTool,
	call llmadapter.ToolCall,
) error {
	if tool == nil {
		return nil
	}
	sch := schema.FromMap(tool.ParameterSchema())
	if err := schema.ValidateRawMessage(ctx, sch, call.Arguments); err != nil {
		return core.NewError(
			fmt.Errorf("invalid tool arguments: %w", err),
			ErrCodeToolInvalidInput,
			map[string]any{
				"tool": tool.Name(),
			},
		)
	}
	return nil
}

func (e *toolExecutor) toolNotFoundResult(
	ctx context.Context,
	call llmadapter.ToolCall,
	capture bool,
	entry *telemetry.ToolLogEntry,
) llmadapter.ToolResult {
	log := telemetry.Logger(ctx)
	entry.Status = telemetry.ToolStatusError
	errText := fmt.Sprintf("tool not found: %s", call.Name)
	entry.Error = errText
	log.Warn("Tool not found", "tool_name", call.Name, "tool_call_id", call.ID)
	payload, errMarshal := json.Marshal(map[string]any{"error": errText})
	if errMarshal != nil {
		payload = []byte(fmt.Sprintf(`{"error":%q}`, errText))
	}
	jsonContent := json.RawMessage(payload)
	if capture {
		entry.Output = string(payload)
	} else {
		entry.Output = telemetry.RedactedValue
		entry.Redacted = true
	}
	return llmadapter.ToolResult{
		ID:          call.ID,
		Name:        call.Name,
		Content:     string(payload),
		JSONContent: jsonContent,
	}
}

func (e *toolExecutor) toolInvalidInputResult(
	call llmadapter.ToolCall,
	validErr error,
	capture bool,
	entry *telemetry.ToolLogEntry,
) llmadapter.ToolResult {
	result := ToolExecutionResult{
		Success: false,
		Error: &ToolError{
			Code:    ErrCodeToolInvalidInput,
			Message: "Invalid tool arguments",
			Details: core.RedactError(validErr),
		},
	}
	payload, marshalErr := json.Marshal(result)
	entry.Status = telemetry.ToolStatusError
	entry.Error = core.RedactError(validErr)
	if marshalErr != nil {
		const fallback = `{"success":false,"error":{"code":"TOOL_INVALID_INPUT","message":"Invalid tool arguments"}}`
		if capture {
			entry.Output = fallback
		} else {
			entry.Output = telemetry.RedactedValue
			entry.Redacted = true
		}
		return llmadapter.ToolResult{
			ID:      call.ID,
			Name:    call.Name,
			Content: fallback,
		}
	}
	if capture {
		entry.Output = string(payload)
	} else {
		entry.Output = telemetry.RedactedValue
		entry.Redacted = true
	}
	return llmadapter.ToolResult{
		ID:          call.ID,
		Name:        call.Name,
		Content:     string(payload),
		JSONContent: json.RawMessage(payload),
	}
}

func (e *toolExecutor) buildSuccessResult(
	call llmadapter.ToolCall,
	raw string,
	capture bool,
	entry *telemetry.ToolLogEntry,
) llmadapter.ToolResult {
	var jsonContent json.RawMessage
	if json.Valid([]byte(raw)) {
		jsonContent = json.RawMessage(raw)
	}
	if capture {
		entry.Output = raw
	} else if raw != "" {
		entry.Output = telemetry.RedactedValue
		entry.Redacted = true
	}
	return llmadapter.ToolResult{
		ID:          call.ID,
		Name:        call.Name,
		Content:     raw,
		JSONContent: jsonContent,
	}
}

func (e *toolExecutor) toolExecutionErrorResult(
	call llmadapter.ToolCall,
	execErr error,
	capture bool,
	entry *telemetry.ToolLogEntry,
) llmadapter.ToolResult {
	var coreErr *core.Error
	result := ToolExecutionResult{Success: false}
	if errors.As(execErr, &coreErr) && coreErr != nil {
		code := coreErr.Code
		msg := coreErr.Message
		if msg == "" {
			msg = "Tool execution failed"
		}
		result.Error = &ToolError{
			Code:            code,
			Message:         msg,
			Details:         core.RedactError(execErr),
			RemediationHint: remediationHintFromDetails(coreErr.Details),
		}
	} else {
		result.Error = &ToolError{
			Code:            ErrCodeToolExecution,
			Message:         "Tool execution failed",
			Details:         core.RedactError(execErr),
			RemediationHint: "",
		}
	}
	payload, marshalErr := json.Marshal(result)
	entry.Status = telemetry.ToolStatusError
	entry.Error = core.RedactError(execErr)
	if marshalErr != nil {
		if capture {
			entry.Output = toolExecutionErrorPayload
		} else {
			entry.Output = telemetry.RedactedValue
			entry.Redacted = true
		}
		return llmadapter.ToolResult{
			ID:          call.ID,
			Name:        call.Name,
			Content:     toolExecutionErrorPayload,
			JSONContent: nil,
		}
	}
	if capture {
		entry.Output = string(payload)
	} else {
		entry.Output = telemetry.RedactedValue
		entry.Redacted = true
	}
	return llmadapter.ToolResult{
		ID:          call.ID,
		Name:        call.Name,
		Content:     string(payload),
		JSONContent: json.RawMessage(payload),
	}
}

func remediationHintFromDetails(details map[string]any) string {
	if len(details) == 0 {
		return ""
	}
	for _, key := range []string{"remediation", "remediation_hint"} {
		if hint, ok := details[key]; ok {
			if text := flattenHintValue(hint); text != "" {
				return text
			}
		}
	}
	return ""
}

func flattenHintValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []string:
		return strings.Join(v, "; ")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		if msg, ok := v["message"].(string); ok {
			return msg
		}
	default:
		if v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func (e *toolExecutor) UpdateBudgets(ctx context.Context, results []llmadapter.ToolResult, state *loopState) error {
	log := telemetry.Logger(ctx)
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
		if state != nil {
			count := state.incrementUsage(name)
			if limit := state.limitFor(name); limit > 0 && count > limit {
				log.Warn(
					"Tool invocation cap exceeded",
					"tool", name,
					"invocations", count,
					"cap", limit,
				)
				return fmt.Errorf("%w: tool invocation cap exceeded for %s", ErrBudgetExceeded, name)
			}
		}
		if state == nil {
			continue
		}
		if isToolResultSuccess(result) {
			fingerprint := toolResultFingerprint(result)
			if last, ok := state.Budgets.LastToolResults[name]; ok && last == fingerprint {
				state.Budgets.ToolSuccess[name]++
			} else {
				state.Budgets.ToolSuccess[name] = 1
				state.Budgets.LastToolResults[name] = fingerprint
			}

			if state.Budgets.ToolSuccess[name] >= maxSucc {
				log.Warn(
					"Tool called successfully too many times without progress",
					"tool",
					name,
					"consecutive_successes",
					state.Budgets.ToolSuccess[name],
				)
				return fmt.Errorf(
					"%w: tool %s called successfully %d times without progress",
					ErrBudgetExceeded,
					name,
					state.Budgets.ToolSuccess[name],
				)
			}
			if state.Budgets.ToolErrors[name] > 0 {
				state.Budgets.ToolErrors[name]--
			}
			continue
		}

		state.Budgets.ToolSuccess[name] = 0
		delete(state.Budgets.LastToolResults, name)
		state.Budgets.ToolErrors[name]++
		log.Warn(
			"Tool error recorded",
			"tool",
			name,
			"consecutive_errors",
			state.Budgets.ToolErrors[name],
			"max",
			budget,
		)
		if state.Budgets.ToolErrors[name] >= budget {
			log.Warn("Error budget exceeded - tool", "tool", name, "max", budget)
			return fmt.Errorf("%w: tool error budget exceeded for %s", ErrBudgetExceeded, name)
		}
	}

	return nil
}

func toolResultFingerprint(result llmadapter.ToolResult) string {
	if len(result.JSONContent) > 0 {
		return stableJSONFingerprint(result.JSONContent)
	}
	return stableJSONFingerprint([]byte(result.Content))
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
	if v, ok := obj["success"]; ok {
		if b, ok := v.(bool); ok && !b {
			return false, true
		}
	}
	if v, ok := obj["ok"]; ok {
		if b, ok := v.(bool); ok && !b {
			return false, true
		}
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
