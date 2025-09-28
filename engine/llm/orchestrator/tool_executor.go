package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	registry ToolRegistry
	cfg      settings
	logPath  string
	logMu    sync.Mutex
}

type toolLogEntry struct {
	Timestamp  string `json:"timestamp"`
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Input      string `json:"input"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	Status     string `json:"status"`
	Duration   string `json:"duration"`
}

const (
	toolLogStatusSuccess      = "success"
	toolLogStatusError        = "error"
	toolExecutionErrorPayload = `{"success":false,"error":{"code":"TOOL_EXECUTION_ERROR","message":"Tool execution failed"}}`
)

func NewToolExecutor(registry ToolRegistry, cfg *settings) ToolExecutor {
	if cfg == nil {
		cfg = &settings{}
	}
	storeDir := core.GetStoreDir(cfg.projectRoot)
	logPath := ""
	if storeDir != "" {
		logPath = filepath.Join(storeDir, "tools_log.json")
	}
	return &toolExecutor{registry: registry, cfg: *cfg, logPath: logPath}
}

func (e *toolExecutor) appendToolLog(ctx context.Context, entry *toolLogEntry) {
	if e.logPath == "" || entry == nil {
		return
	}
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	e.logMu.Lock()
	defer e.logMu.Unlock()
	dir := filepath.Dir(e.logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.FromContext(ctx).Debug("Skipping tool log write; mkdir failed", "error", err)
		return
	}
	f, err := os.OpenFile(e.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		logger.FromContext(ctx).Debug("Skipping tool log write; open failed", "error", err)
		return
	}
	defer f.Close()
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		logger.FromContext(ctx).Debug("Skipping tool log write; marshal failed", "error", err)
		return
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		logger.FromContext(ctx).Debug("Tool log write failed", "error", err)
	}
}

func (e *toolExecutor) Execute(ctx context.Context, toolCalls []llmadapter.ToolCall) ([]llmadapter.ToolResult, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}
	log := logger.FromContext(ctx)
	log.Debug("Executing tool calls", "tool_calls_count", len(toolCalls), "tools", extractToolNames(toolCalls))

	limit := e.cfg.maxConcurrentTools
	if limit <= 0 {
		limit = defaultMaxConcurrentTools
	}
	results := make([]llmadapter.ToolResult, len(toolCalls))
	workerCount := limit
	if workerCount > len(toolCalls) {
		workerCount = len(toolCalls)
	}
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
	start := time.Now()
	entry := toolLogEntry{
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Input:      string(call.Arguments),
		Status:     toolLogStatusSuccess,
	}
	defer func() {
		entry.Duration = time.Since(start).String()
		e.appendToolLog(ctx, &entry)
	}()
	t, found := e.registry.Find(ctx, call.Name)
	if !found || t == nil {
		return e.toolNotFoundResult(log, call, &entry)
	}
	log.Debug("Executing tool", "tool_name", call.Name, "tool_call_id", call.ID)
	if _, err := core.GetRequestID(ctx); err != nil {
		ctx = core.WithRequestID(ctx, call.ID)
	}
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
		return e.toolExecutionErrorResult(call, err, &entry)
	}
	log.Debug("Tool execution succeeded", "tool_name", call.Name, "tool_call_id", call.ID)
	entry.Output = raw
	var jsonContent json.RawMessage
	if json.Valid([]byte(raw)) {
		jsonContent = json.RawMessage(raw)
	}
	return llmadapter.ToolResult{
		ID:          call.ID,
		Name:        call.Name,
		Content:     raw,
		JSONContent: jsonContent,
	}
}

func (e *toolExecutor) toolNotFoundResult(
	log logger.Logger,
	call llmadapter.ToolCall,
	entry *toolLogEntry,
) llmadapter.ToolResult {
	log.Warn("Tool not found", "tool_name", call.Name, "tool_call_id", call.ID)
	errText := fmt.Sprintf("tool not found: %s", call.Name)
	errObj := map[string]any{"error": errText}
	payload, marshalErr := json.Marshal(errObj)
	var jsonContent json.RawMessage
	if marshalErr != nil {
		fields := []any{"tool_name", call.Name, "tool_call_id", call.ID, "error", marshalErr}
		log.Warn("Failed to marshal tool-not-found error payload", fields...)
		errText = "tool not found"
		payload = []byte(`{"error":"tool not found"}`)
	} else {
		jsonContent = json.RawMessage(payload)
	}
	entry.Status = toolLogStatusError
	entry.Error = errText
	entry.Output = string(payload)
	return llmadapter.ToolResult{
		ID:          call.ID,
		Name:        call.Name,
		Content:     string(payload),
		JSONContent: jsonContent,
	}
}

func (e *toolExecutor) toolExecutionErrorResult(
	call llmadapter.ToolCall,
	execErr error,
	entry *toolLogEntry,
) llmadapter.ToolResult {
	var coreErr *core.Error
	result := ToolExecutionResult{Success: false}
	if errors.As(execErr, &coreErr) && coreErr != nil {
		code := coreErr.Code
		msg := coreErr.Message
		if msg == "" {
			msg = "Tool execution failed"
		}
		result.Error = &ToolError{Code: code, Message: msg, Details: core.RedactError(execErr)}
	} else {
		result.Error = &ToolError{
			Code:    ErrCodeToolExecution,
			Message: "Tool execution failed",
			Details: core.RedactError(execErr),
		}
	}
	payload, marshalErr := json.Marshal(result)
	entry.Status = toolLogStatusError
	entry.Error = core.RedactError(execErr)
	if marshalErr != nil {
		entry.Output = toolExecutionErrorPayload
		return llmadapter.ToolResult{
			ID:          call.ID,
			Name:        call.Name,
			Content:     toolExecutionErrorPayload,
			JSONContent: nil,
		}
	}
	entry.Output = string(payload)
	return llmadapter.ToolResult{
		ID:          call.ID,
		Name:        call.Name,
		Content:     string(payload),
		JSONContent: json.RawMessage(payload),
	}
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
			fingerprint := toolResultFingerprint(result)
			if last, ok := state.lastToolResults[name]; ok && last == fingerprint {
				state.toolSuccess[name]++
			} else {
				state.toolSuccess[name] = 1
				state.lastToolResults[name] = fingerprint
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
					"%w: tool %s called successfully %d times without progress",
					ErrBudgetExceeded,
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
		delete(state.lastToolResults, name)
		state.toolErrors[name]++
		log.Debug("Tool error recorded", "tool", name, "consecutive_errors", state.toolErrors[name], "max", budget)
		if state.toolErrors[name] >= budget {
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
