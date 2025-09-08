package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/sethvargo/go-retry"
	"golang.org/x/sync/errgroup"
)

// Default configuration constants
const (
	defaultMaxConcurrentTools = 10
	defaultRetryAttempts      = 3
	defaultRetryBackoffBase   = 100 * time.Millisecond
	defaultRetryBackoffMax    = 10 * time.Second
	// defaultMaxToolIterations caps the tool-call iteration loop when no overrides are provided
	defaultMaxToolIterations       = 10
	defaultMaxConsecutiveSuccesses = 3
	defaultNoProgressThreshold     = 3
)

// AsyncHook provides hooks for monitoring async operations
type AsyncHook interface {
	// OnMemoryStoreComplete is called when async memory storage completes
	OnMemoryStoreComplete(err error)
}

// Orchestrator coordinates LLM interactions, tool calls, and response processing
type Orchestrator interface {
	Execute(ctx context.Context, request Request) (*core.Output, error)
	Close() error
}

// Request represents a request to the orchestrator
type Request struct {
	Agent  *agent.Config
	Action *agent.ActionConfig
}

// OrchestratorConfig configures the LLM orchestrator
type OrchestratorConfig struct {
	ToolRegistry       ToolRegistry
	PromptBuilder      PromptBuilder
	RuntimeManager     runtime.Runtime
	LLMFactory         llmadapter.Factory
	MemoryProvider     MemoryProvider // Optional: provides memory instances for agents
	AsyncHook          AsyncHook      // Optional: hook for monitoring async operations
	Timeout            time.Duration  // Optional: timeout for LLM operations
	MaxConcurrentTools int            // Maximum concurrent tool executions
	// MaxToolIterations optionally caps the tool-iteration loop. If <= 0, defaults apply.
	// Precedence at runtime: model-specific (agent.Config.MaxToolIterations) > this value > default.
	MaxToolIterations int
	// MaxSequentialToolErrors limits how many sequential failures for the same tool
	// (or content-level error) are tolerated before aborting. When <= 0, defaults to 8.
	MaxSequentialToolErrors int
	// Retry configuration
	RetryAttempts    int           // Number of retry attempts for LLM operations
	RetryBackoffBase time.Duration // Base delay for exponential backoff retry strategy
	RetryBackoffMax  time.Duration // Maximum delay between retry attempts
	RetryJitter      bool          // Enable random jitter in retry delays
	// Repetition and progress detection
	MaxConsecutiveSuccesses int  // Threshold for consecutive successes without progress (<=0 uses default)
	EnableProgressTracking  bool // Enable progress/repetition detection in loop
	NoProgressThreshold     int  // Iterations without progress before abort (<=0 uses default)
}

// Implementation of LLMOrchestrator
type llmOrchestrator struct {
	config     OrchestratorConfig
	memorySync *MemorySync
}

// NewOrchestrator creates a new LLM orchestrator
func NewOrchestrator(config *OrchestratorConfig) Orchestrator {
	if config == nil {
		config = &OrchestratorConfig{}
	}
	return &llmOrchestrator{config: *config, memorySync: NewMemorySync()}
}

// Execute processes an LLM request end-to-end
func (o *llmOrchestrator) Execute(ctx context.Context, request Request) (*core.Output, error) {
	if err := o.validateInput(ctx, request); err != nil {
		return nil, NewValidationError(err, "request", request)
	}
	return o.executeWithValidatedRequest(ctx, request)
}

func (o *llmOrchestrator) executeWithValidatedRequest(
	ctx context.Context,
	request Request,
) (*core.Output, error) {
	memories := o.prepareMemoryContext(ctx, request)
	llmClient, err := o.createLLMClient(request)
	if err != nil {
		return nil, err
	}
	defer o.closeLLMClient(ctx, llmClient)
	return o.executeWithClient(ctx, request, memories, llmClient)
}

func (o *llmOrchestrator) executeWithClient(
	ctx context.Context,
	request Request,
	memories map[string]Memory,
	llmClient llmadapter.LLMClient,
) (*core.Output, error) {
	llmReq, err := o.buildLLMRequest(ctx, request, memories)
	if err != nil {
		return nil, err
	}
	// Track sequential error counts per tool name; "__content__" used for content-level errors
	errBudget := o.computeErrorBudget()
	consecutiveErrors := map[string]int{}

	// Track successes and last results to detect non-progressive loops
	successCounters := map[string]int{}
	lastResults := map[string]string{}
	noProgressCount := 0
	lastFingerprint := ""
	// Iteratively handle tool calls by feeding tool results back to the LLM
	// to allow multi-step workflows (e.g., read_file -> analyze -> write_file).
	// Hard cap iterations to avoid infinite loops.
	maxIterations := o.maxToolIterationsFor(request)
	for iter := range maxIterations {
		o.logLoopStart(ctx, request, &llmReq, iter)

		response, err := o.generateLLMResponse(ctx, llmClient, &llmReq, request)
		if err != nil {
			return nil, err
		}

		// If no tool calls, decide whether to finish or loop (agentic error handling)
		if len(response.ToolCalls) == 0 {
			out, cont, perr := o.handleNoToolCalls(
				ctx,
				response,
				request,
				&llmReq,
				consecutiveErrors,
				errBudget,
				memories,
			)
			if perr != nil {
				return nil, perr
			}
			if cont {
				continue
			}
			return out, nil
		}

		// Append structured assistant tool calls and tool responses for provider-native semantics
		llmReq.Messages = append(llmReq.Messages,
			llmadapter.Message{Role: "assistant", ToolCalls: response.ToolCalls},
		)

		// Execute tool calls and convert to tool results for the tool role message
		toolResults, execErr := o.executeToolCallsRaw(ctx, response.ToolCalls, request)
		if execErr != nil {
			return nil, execErr
		}
		if err := o.updateErrorBudgetForResults(
			ctx,
			toolResults,
			consecutiveErrors,
			successCounters,
			lastResults,
			errBudget,
		); err != nil {
			return nil, err
		}
		// Include raw results even if some failed (model can handle errors)
		llmReq.Messages = append(llmReq.Messages,
			llmadapter.Message{Role: "tool", ToolResults: toolResults},
		)
		// Detect no progress after appending assistant/tool messages using a stable fingerprint
		if o.config.EnableProgressTracking {
			if o.noProgressExceeded(
				o.effectiveNoProgressThreshold(),
				response.ToolCalls,
				toolResults,
				&lastFingerprint,
				&noProgressCount,
			) {
				return nil, fmt.Errorf("no progress for %d consecutive iterations", noProgressCount)
			}
		}
		// Continue the loop with the augmented messages
	}

	return nil, fmt.Errorf("max tool iterations reached without final response")
}

// executeToolCallsRaw executes tool calls and returns generic tool results for the tool role message
func (o *llmOrchestrator) executeToolCallsRaw(
	ctx context.Context,
	toolCalls []llmadapter.ToolCall,
	_ Request,
) ([]llmadapter.ToolResult, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}
	log := logger.FromContext(ctx)
	log.Debug("Executing tool calls",
		"tool_calls_count", len(toolCalls),
		"tools", extractToolNames(toolCalls),
	)
	// Concurrency bounded by MaxConcurrentTools with early-cancel support via errgroup
	concurrency := o.config.MaxConcurrentTools
	if concurrency <= 0 {
		concurrency = defaultMaxConcurrentTools
	}
	sem := make(chan struct{}, concurrency)
	results := make([]llmadapter.ToolResult, len(toolCalls))
	g, ctx := errgroup.WithContext(ctx)
	for i := range toolCalls {
		idx := i
		call := toolCalls[i]
		g.Go(func() error {
			// Acquire semaphore or respect context cancellation
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}
			results[idx] = o.executeSingleToolCallRaw(ctx, call)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	log.Debug("All tool calls completed",
		"results_count", len(results),
		"successful_count", countSuccessfulResults(results),
	)
	return results, nil
}

// executeSingleToolCallRaw executes a single tool call and returns the result
func (o *llmOrchestrator) executeSingleToolCallRaw(
	ctx context.Context,
	tc llmadapter.ToolCall,
) llmadapter.ToolResult {
	log := logger.FromContext(ctx)
	log.Debug("Processing tool call",
		"tool_name", tc.Name,
		"tool_call_id", tc.ID,
	)
	// Find tool
	t, found := o.config.ToolRegistry.Find(ctx, tc.Name)
	if !found || t == nil {
		log.Debug("Tool not found",
			"tool_name", tc.Name,
			"tool_call_id", tc.ID,
		)
		errJSON := fmt.Sprintf("{\"error\":\"tool not found: %s\"}", tc.Name)
		return llmadapter.ToolResult{
			ID:          tc.ID,
			Name:        tc.Name,
			Content:     errJSON,
			JSONContent: json.RawMessage(errJSON),
		}
	}
	log.Debug("Executing tool",
		"tool_name", tc.Name,
		"tool_call_id", tc.ID,
	)
	// Execute tool and capture raw content; include errors as sanitized JSON so the model can react
	raw, err := t.Call(ctx, string(tc.Arguments))
	if err != nil {
		log.Debug("Tool execution failed",
			"tool_name", tc.Name,
			"tool_call_id", tc.ID,
			"error", core.RedactError(err),
		)
		// Return structured error with safe details (avoid leaking secrets)
		errObj := ToolExecutionResult{
			Success: false,
			Error: &ToolError{
				Code:    ErrCodeToolExecution,
				Message: "Tool execution failed",
				Details: core.RedactError(err),
			},
		}
		b, merr := json.Marshal(errObj)
		var errJSON string
		if merr != nil {
			errJSON = `{"success":false,"error":{"code":"TOOL_EXECUTION_ERROR","message":"Tool execution failed"}}`
		} else {
			errJSON = string(b)
		}
		return llmadapter.ToolResult{
			ID:          tc.ID,
			Name:        tc.Name,
			Content:     errJSON,
			JSONContent: json.RawMessage(errJSON),
		}
	}
	log.Debug("Tool execution succeeded",
		"tool_name", tc.Name,
		"tool_call_id", tc.ID,
	)
	// Populate JSONContent when the tool returns valid JSON to avoid double-encoding downstream
	var jsonContent json.RawMessage
	if json.Valid([]byte(raw)) {
		jsonContent = json.RawMessage(raw)
	}
	return llmadapter.ToolResult{
		ID:          tc.ID,
		Name:        tc.Name,
		Content:     raw,
		JSONContent: jsonContent,
	}
}

// extractToolNames extracts tool names from tool calls for logging
func extractToolNames(toolCalls []llmadapter.ToolCall) []string {
	names := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		names[i] = tc.Name
	}
	return names
}

// countSuccessfulResults counts successful tool results
func countSuccessfulResults(results []llmadapter.ToolResult) int {
	count := 0
	for _, r := range results {
		// Prefer JSONContent if present
		if len(r.JSONContent) > 0 {
			if ok, parsed := isSuccessJSONRaw(r.JSONContent); parsed {
				if ok {
					count++
				}
				continue
			}
			// Fallback heuristic when JSON parsing fails
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

// buildIterationFingerprint generates a stable fingerprint for a loop iteration
// based on assistant tool calls and resulting tool observations. This helps detect
// no-progress situations where the LLM repeats identical actions and results.
func buildIterationFingerprint(calls []llmadapter.ToolCall, results []llmadapter.ToolResult) string {
	var b bytes.Buffer
	for _, tc := range calls {
		b.WriteString(tc.Name)
		b.WriteByte('|')
		if len(tc.Arguments) > 0 {
			b.WriteString(stableJSONFingerprint(tc.Arguments))
		}
		b.WriteByte(';')
	}
	for _, r := range results {
		b.WriteString(r.Name)
		b.WriteByte('|')
		switch {
		case len(r.JSONContent) > 0:
			b.WriteString(stableJSONFingerprint(r.JSONContent))
		default:
			b.WriteString(stableJSONFingerprint([]byte(r.Content)))
		}
		b.WriteByte(';')
	}
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:])
}

// isSuccessJSONRaw attempts to parse a JSON object and checks for a top-level "error" key.
// Returns (success, parsed). When parsed is false, caller may apply a fallback heuristic.
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

// isSuccessText checks non-JSON or ambiguous content for success by parsing JSON object
// when possible and otherwise using a conservative substring heuristic.
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
	// Check for common error indicators in plain text responses
	lowerContent := strings.ToLower(trimmed)
	errorIndicators := []string{
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
	}
	for _, indicator := range errorIndicators {
		if strings.Contains(lowerContent, indicator) {
			return false
		}
	}
	// Plain text without error indicators is considered success
	return true
}

// isToolResultSuccess determines if a single tool result indicates success
func isToolResultSuccess(r llmadapter.ToolResult) bool {
	if len(r.JSONContent) > 0 {
		if ok, parsed := isSuccessJSONRaw(r.JSONContent); parsed {
			return ok
		}
		return isSuccessText(string(r.JSONContent))
	}
	return isSuccessText(r.Content)
}

// stableJSONFingerprint returns a stable hash string for JSON content.
// It parses JSON, serializes with sorted object keys recursively, and hashes the result.
// If input is not valid JSON, it returns the raw string as the fingerprint.
func stableJSONFingerprint(raw []byte) string {
	if len(raw) == 0 || !json.Valid(raw) {
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	var b bytes.Buffer
	writeStableJSON(&b, v)
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:])
}

// writeStableJSON writes a canonical JSON-like representation with sorted keys.
func writeStableJSON(b *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			if bs, mErr := json.Marshal(k); mErr == nil {
				b.Write(bs)
			} else {
				b.WriteString("\"")
				b.WriteString(k)
				b.WriteString("\"")
			}
			b.WriteByte(':')
			writeStableJSON(b, t[k])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeStableJSON(b, e)
		}
		b.WriteByte(']')
	case string:
		if bs, mErr := json.Marshal(t); mErr == nil {
			b.Write(bs)
		} else {
			b.WriteString("\"")
			b.WriteString(t)
			b.WriteString("\"")
		}
	case float64, bool, nil:
		if bs, mErr := json.Marshal(t); mErr == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	default:
		if bs, mErr := json.Marshal(t); mErr == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	}
}

// detectNoProgress updates counters and detects no-progress condition
func (o *llmOrchestrator) detectNoProgress(threshold int, current string, last *string, counter *int) bool {
	if current == *last {
		*counter++
		return *counter >= threshold
	}
	*counter = 0
	*last = current
	return false
}

// computeErrorBudget returns the effective max sequential error budget per tool/content.
func (o *llmOrchestrator) computeErrorBudget() int {
	b := o.config.MaxSequentialToolErrors
	if b <= 0 {
		b = 8
	}
	return b
}

// effectiveNoProgressThreshold resolves the no-progress threshold with defaults.
func (o *llmOrchestrator) effectiveNoProgressThreshold() int {
	t := o.config.NoProgressThreshold
	if t <= 0 {
		t = defaultNoProgressThreshold
	}
	return t
}

// noProgressExceeded determines if no-progress threshold has been exceeded for this iteration.
func (o *llmOrchestrator) noProgressExceeded(
	threshold int,
	calls []llmadapter.ToolCall,
	results []llmadapter.ToolResult,
	last *string,
	counter *int,
) bool {
	fp := buildIterationFingerprint(calls, results)
	return o.detectNoProgress(threshold, fp, last, counter)
}

// logLoopStart centralizes loop-start debug logging to reduce function length
func (o *llmOrchestrator) logLoopStart(ctx context.Context, request Request, llmReq *llmadapter.LLMRequest, iter int) {
	logger.FromContext(ctx).Debug(
		"Generating LLM response",
		"agent_id", request.Agent.ID,
		"action_id", request.Action.ID,
		"messages_count", len(llmReq.Messages),
		"tools_count", len(llmReq.Tools),
		"iteration", iter,
	)
}

// handleJSONModeNoToolCalls enforces JSON object output when JSON mode is active.
// Returns (continueLoop, error).
func (o *llmOrchestrator) handleJSONModeNoToolCalls(
	ctx context.Context,
	content string,
	llmReq *llmadapter.LLMRequest,
	counters map[string]int,
	budget int,
) (bool, error) {
	log := logger.FromContext(ctx)
	if !llmReq.Options.UseJSONMode {
		return false, nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(content), &obj); err == nil && obj != nil {
		return false, nil
	}
	// Use a pseudo-tool observation to align with ReAct: assistant(tool_calls) -> tool(observation)
	key := "output_parser"
	counters[key]++
	log.Debug("Non-JSON content with JSON mode; continuing loop",
		"consecutive_errors", counters[key], "max", budget,
	)
	if counters[key] >= budget {
		log.Warn("Error budget exceeded - non-JSON content in JSON mode", "key", key, "max", budget)
		return false, fmt.Errorf(
			"tool error budget exceeded for %s: expected JSON object in JSON mode",
			key,
		)
	}
	pseudoID := fmt.Sprintf("call_%s_%d", key, time.Now().UnixNano())
	// Assistant tool_call
	llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
		Role: "assistant",
		ToolCalls: []llmadapter.ToolCall{{
			ID:        pseudoID,
			Name:      key,
			Arguments: json.RawMessage("{}"),
		}},
	})
	// Tool observation with structured error
	obs := map[string]any{
		"error":   "Invalid final response: expected JSON object (json_mode=true)",
		"example": map[string]any{"response": "..."},
	}
	if b, mErr := json.Marshal(obs); mErr == nil {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: "tool",
			ToolResults: []llmadapter.ToolResult{{
				ID:          pseudoID,
				Name:        key,
				Content:     string(b),
				JSONContent: json.RawMessage(b),
			}},
		})
	} else {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: "tool",
			ToolResults: []llmadapter.ToolResult{{
				ID:      pseudoID,
				Name:    key,
				Content: `{"error":"Invalid final response: expected JSON object"}`,
			}},
		})
	}
	return true, nil
}

// handleContentErrorMessageNoToolCalls handles a top-level JSON {"error":...} message
// by incrementing the content error counter and appending the assistant message.
func (o *llmOrchestrator) handleContentErrorMessageNoToolCalls(
	ctx context.Context,
	content string,
	llmReq *llmadapter.LLMRequest,
	counters map[string]int,
	budget int,
) (bool, error) {
	log := logger.FromContext(ctx)
	if msg, hasErr := extractTopLevelErrorMessage(content); hasErr {
		key := "content_validator"
		counters[key]++
		log.Debug("Content-level error detected; continuing loop",
			"error_message", msg, "tool_key", key,
			"consecutive_errors", counters[key], "max", budget,
		)
		if counters[key] >= budget {
			log.Warn("Error budget exceeded - content error", "key", key, "max", budget)
			return false, fmt.Errorf("tool error budget exceeded for %s: %s", key, msg)
		}
		pseudoID := fmt.Sprintf("call_%s_%d", key, time.Now().UnixNano())
		// Assistant tool_call
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: "assistant",
			ToolCalls: []llmadapter.ToolCall{{
				ID:        pseudoID,
				Name:      key,
				Arguments: json.RawMessage("{}"),
			}},
		})
		// Tool observation
		obs := map[string]any{"error": msg}
		if b, mErr := json.Marshal(obs); mErr == nil {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: "tool",
				ToolResults: []llmadapter.ToolResult{{
					ID:          pseudoID,
					Name:        key,
					Content:     string(b),
					JSONContent: json.RawMessage(b),
				}},
			})
		} else {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: "tool",
				ToolResults: []llmadapter.ToolResult{{
					ID:      pseudoID,
					Name:    key,
					Content: fmt.Sprintf("{\\\"error\\\":%q}", msg),
				}},
			})
		}
		return true, nil
	}
	return false, nil
}

// updateErrorBudgetForResults applies a sliding error budget per tool result.
// Success decrements the counter toward zero; failure increments it. Exceeds trigger warning and error.
func (o *llmOrchestrator) updateErrorBudgetForResults(
	ctx context.Context,
	results []llmadapter.ToolResult,
	counters map[string]int,
	successCounters map[string]int,
	lastResults map[string]string,
	budget int,
) error {
	log := logger.FromContext(ctx)
	maxSucc := o.config.MaxConsecutiveSuccesses
	if maxSucc <= 0 {
		maxSucc = defaultMaxConsecutiveSuccesses
	}
	for _, r := range results {
		name := r.Name
		if isToolResultSuccess(r) {
			if o.config.EnableProgressTracking {
				var fp string
				switch {
				case len(r.JSONContent) > 0:
					fp = stableJSONFingerprint(r.JSONContent)
				default:
					fp = stableJSONFingerprint([]byte(r.Content))
				}
				if last, ok := lastResults[name]; ok && last == fp {
					successCounters[name]++
				} else {
					successCounters[name] = 1
					lastResults[name] = fp
				}
			} else {
				successCounters[name]++
			}
			if successCounters[name] >= maxSucc {
				log.Warn(
					"Tool called successfully too many times without progress",
					"tool",
					name,
					"consecutive_successes",
					successCounters[name],
				)
				return fmt.Errorf("tool %s called successfully %d times without progress", name, successCounters[name])
			}
			if counters[name] > 0 {
				counters[name]--
			}
			continue
		}
		successCounters[name] = 0
		counters[name]++
		log.Debug("Tool error recorded", "tool", name, "consecutive_errors", counters[name], "max", budget)
		if counters[name] >= budget {
			log.Warn("Error budget exceeded - tool", "tool", name, "max", budget)
			return fmt.Errorf("tool error budget exceeded for %s", name)
		}
	}
	return nil
}

// extractTopLevelErrorMessage inspects a JSON object string and returns the error message, if present.
// It supports either a string error or an object with a "message" field.
func extractTopLevelErrorMessage(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "{") {
		return "", false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return "", false
	}
	if ev, ok := obj["error"]; ok && ev != nil {
		switch v := ev.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v, true
			}
		case map[string]any:
			if msg, ok := v["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return msg, true
			}
			// Fallback: stringify the map
			if b, mErr := json.Marshal(v); mErr == nil && len(b) > 0 {
				return string(b), true
			}
			return fmt.Sprintf("%v", v), true
		default:
			// Fallback: stringify
			if b, mErr := json.Marshal(v); mErr == nil && len(b) > 0 {
				return string(b), true
			}
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}

// maxToolIterationsFor resolves the effective max tool iterations for a request
// with precedence: agent model override > orchestrator config > default.
func (o *llmOrchestrator) maxToolIterationsFor(request Request) int {
	maxIterations := o.config.MaxToolIterations
	if request.Agent != nil && request.Agent.Config.MaxToolIterations > 0 {
		maxIterations = request.Agent.Config.MaxToolIterations
	}
	if maxIterations <= 0 {
		maxIterations = defaultMaxToolIterations
	}
	return maxIterations
}

func (o *llmOrchestrator) generateLLMResponse(
	ctx context.Context,
	llmClient llmadapter.LLMClient,
	llmReq *llmadapter.LLMRequest,
	request Request,
) (*llmadapter.LLMResponse, error) {
	// Apply timeout if configured
	if o.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.config.Timeout)
		defer cancel()
	}
	var response *llmadapter.LLMResponse
	// Use configurable retry with exponential backoff
	attempts := o.config.RetryAttempts
	if attempts <= 0 {
		attempts = defaultRetryAttempts
	}
	backoffBase := o.config.RetryBackoffBase
	if backoffBase <= 0 {
		backoffBase = defaultRetryBackoffBase
	}
	backoffMax := o.config.RetryBackoffMax
	if backoffMax <= 0 {
		backoffMax = defaultRetryBackoffMax
	}

	var backoff retry.Backoff
	exponential := retry.NewExponential(backoffBase)
	exponential = retry.WithMaxDuration(backoffMax, exponential)
	// Validate attempts is positive and within reasonable bounds to prevent overflow
	if attempts < 0 || attempts > 100 {
		attempts = defaultRetryAttempts
	}
	// Safe conversion: attempts is validated to be in range [0, 100]
	maxRetries := uint64(attempts) //nolint:gosec // G115: bounds checked above
	if o.config.RetryJitter {
		backoff = retry.WithMaxRetries(maxRetries, retry.WithJitter(time.Millisecond*50, exponential))
	} else {
		backoff = retry.WithMaxRetries(maxRetries, exponential)
	}

	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		var err error
		response, err = llmClient.GenerateContent(ctx, llmReq)
		if err != nil {
			// Check if error is retryable
			if isRetryableErrorWithContext(ctx, err) {
				return retry.RetryableError(err)
			}
			// Non-retryable error
			return err
		}
		return nil
	})
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMGeneration, map[string]any{
			"agent":  request.Agent.ID,
			"action": request.Action.ID,
		})
	}
	return response, nil
}

func (o *llmOrchestrator) prepareMemoryContext(
	ctx context.Context,
	request Request,
) map[string]Memory {
	log := logger.FromContext(ctx)
	memoryRefs := request.Agent.Memory

	log.Debug("Preparing memory context for agent",
		"agent_id", request.Agent.ID,
		"memory_refs_count", len(memoryRefs),
	)

	if o.config.MemoryProvider == nil {
		log.Debug("No memory provider available")
		return nil
	}
	if len(memoryRefs) == 0 {
		log.Debug("No memory references configured for agent")
		return nil
	}

	memories := make(map[string]Memory)
	for _, ref := range memoryRefs {
		log.Debug("Retrieving memory for agent",
			"memory_id", ref.ID,
			"has_key", ref.Key != "",
		)

		memory, err := o.config.MemoryProvider.GetMemory(ctx, ref.ID, ref.Key)
		if err != nil {
			log.Error("Failed to get memory instance", "memory_id", ref.ID, "error", err)
			continue
		}
		if memory != nil {
			log.Debug("Memory instance retrieved successfully",
				"memory_id", ref.ID,
				"instance_id", memory.GetID())
			memories[ref.ID] = memory
		} else {
			log.Warn("Memory instance is nil", "memory_id", ref.ID)
		}
	}

	log.Debug("Memory context prepared", "count", len(memories))
	return memories
}

func (o *llmOrchestrator) createLLMClient(request Request) (llmadapter.LLMClient, error) {
	factory := o.config.LLMFactory
	if factory == nil {
		factory = llmadapter.NewDefaultFactory()
	}
	llmClient, err := factory.CreateClient(&request.Agent.Config)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMCreation, map[string]any{
			"provider": request.Agent.Config.Provider,
			"model":    request.Agent.Config.Model,
		})
	}
	return llmClient, nil
}

func (o *llmOrchestrator) closeLLMClient(ctx context.Context, llmClient llmadapter.LLMClient) {
	if closeErr := llmClient.Close(); closeErr != nil {
		logger.FromContext(ctx).Error("Failed to close LLM client", "error", closeErr)
	}
}

func (o *llmOrchestrator) buildLLMRequest(
	ctx context.Context,
	request Request,
	memories map[string]Memory,
) (llmadapter.LLMRequest, error) {
	promptData, err := o.buildPromptData(ctx, request)
	if err != nil {
		return llmadapter.LLMRequest{}, err
	}
	toolDefs, err := o.buildToolDefinitions(ctx, request.Agent.Tools)
	if err != nil {
		return llmadapter.LLMRequest{}, NewLLMError(err, "TOOL_DEFINITIONS_ERROR", map[string]any{
			"agent": request.Agent.ID,
		})
	}
	messages := o.buildMessages(ctx, promptData.enhancedPrompt, memories)

	// Determine temperature: use agent's configured value (explicit zero allowed; upstream default applies)
	temperature := request.Agent.Config.Params.Temperature

	// Determine tool choice: default to "auto" when tools are available
	toolChoice := ""
	if len(toolDefs) > 0 {
		toolChoice = "auto"
	}

	// Log a concise snapshot of the outbound request
	logger.FromContext(ctx).Debug("LLM request prepared",
		"agent_id", request.Agent.ID,
		"action_id", request.Action.ID,
		"messages_count", len(messages),
		"tools_count", len(toolDefs),
		"tool_choice", toolChoice,
		"json_mode", request.Action.JSONMode || (promptData.shouldUseStructured && len(toolDefs) == 0),
		"structured_output", promptData.shouldUseStructured,
	)

	return llmadapter.LLMRequest{
		SystemPrompt: request.Agent.Instructions,
		Messages:     messages,
		Tools:        toolDefs,
		Options: llmadapter.CallOptions{
			Temperature:      temperature,
			UseJSONMode:      request.Action.JSONMode || (promptData.shouldUseStructured && len(toolDefs) == 0),
			ToolChoice:       toolChoice,
			StructuredOutput: promptData.shouldUseStructured,
		},
	}, nil
}

type promptBuildData struct {
	enhancedPrompt      string
	shouldUseStructured bool
}

func (o *llmOrchestrator) buildPromptData(ctx context.Context, request Request) (*promptBuildData, error) {
	basePrompt, err := o.config.PromptBuilder.Build(ctx, request.Action)
	if err != nil {
		return nil, NewLLMError(err, "PROMPT_BUILD_ERROR", map[string]any{
			"action": request.Action.ID,
		})
	}
	shouldUseStructured := o.config.PromptBuilder.ShouldUseStructuredOutput(
		string(request.Agent.Config.Provider),
		request.Action,
		request.Agent.Tools,
	)
	enhancedPrompt := o.enhancePromptIfNeeded(ctx, basePrompt, shouldUseStructured, request)
	return &promptBuildData{
		enhancedPrompt:      enhancedPrompt,
		shouldUseStructured: shouldUseStructured,
	}, nil
}

func (o *llmOrchestrator) enhancePromptIfNeeded(
	ctx context.Context,
	basePrompt string,
	shouldUseStructured bool,
	request Request,
) string {
	if !shouldUseStructured {
		return basePrompt
	}
	return o.config.PromptBuilder.EnhanceForStructuredOutput(
		ctx,
		basePrompt,
		request.Action.OutputSchema,
		len(request.Agent.Tools) > 0,
	)
}

func (o *llmOrchestrator) buildMessages(
	ctx context.Context,
	enhancedPrompt string,
	memories map[string]Memory,
) []llmadapter.Message {
	messages := []llmadapter.Message{{
		Role:    "user",
		Content: enhancedPrompt,
	}}
	if len(memories) > 0 {
		messages = PrepareMemoryContext(ctx, memories, messages)
	}
	return messages
}

func (o *llmOrchestrator) storeResponseInMemoryAsync(
	ctx context.Context,
	memories map[string]Memory,
	response *llmadapter.LLMResponse,
	messages []llmadapter.Message,
	request Request,
) {
	if len(memories) == 0 || response.Content == "" {
		return
	}
	go func() {
		log := logger.FromContext(ctx)
		// Create a detached context with timeout to prevent goroutine leaks
		// context.WithoutCancel preserves values from the parent context
		// while allowing the goroutine to continue even if the parent is canceled
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()

		// Collect memory IDs for multi-lock synchronization
		var memoryIDs []string
		for _, memory := range memories {
			if memory != nil {
				memoryIDs = append(memoryIDs, memory.GetID())
			}
		}

		// Use WithMultipleLocks for safe concurrent memory access
		var err error
		o.memorySync.WithMultipleLocks(memoryIDs, func() {
			assistantMsg := llmadapter.Message{
				Role:    "assistant",
				Content: response.Content,
			}
			if len(messages) == 0 {
				// Nothing to store alongside; skip user message association
				err = nil
				return
			}
			lastMsg := messages[len(messages)-1]
			err = StoreResponseInMemory(
				bgCtx,
				memories,
				request.Agent.Memory,
				&assistantMsg,
				&lastMsg,
			)
		})
		if err != nil {
			log.Error("Failed to store response in memory",
				"error", err,
				"agent_id", request.Agent.ID,
				"action_id", request.Action.ID)
			// Consider sending to a metrics/alerting system
			// - **Example**: metrics.RecordMemoryStorageFailure(request.Agent.ID, err)
		}
		// Call async hook if configured
		if o.config.AsyncHook != nil {
			o.config.AsyncHook.OnMemoryStoreComplete(err)
		}
	}()
}

// validateInput validates the input request
func (o *llmOrchestrator) validateInput(ctx context.Context, request Request) error {
	if request.Agent == nil {
		return fmt.Errorf("agent config is required")
	}

	if request.Action == nil {
		return fmt.Errorf("action config is required")
	}

	if request.Agent.Instructions == "" {
		return fmt.Errorf("agent instructions are required")
	}

	if request.Action.Prompt == "" {
		return fmt.Errorf("action prompt is required")
	}

	// Validate input schema if defined
	if request.Action.InputSchema != nil {
		if err := request.Action.ValidateInput(ctx, request.Action.GetInput()); err != nil {
			return fmt.Errorf("input validation failed: %w", err)
		}
	}

	return nil
}

// buildToolDefinitions converts agent tools to LLM adapter format
func (o *llmOrchestrator) buildToolDefinitions(
	ctx context.Context,
	tools []tool.Config,
) ([]llmadapter.ToolDefinition, error) {
	defs, included, err := o.collectConfiguredToolDefs(ctx, tools)
	if err != nil {
		return nil, err
	}
	defs = o.appendRegistryToolDefs(ctx, defs, included)
	return defs, nil
}

// collectConfiguredToolDefs converts explicitly configured tools to adapter definitions
// and returns a set of canonicalised names already included.
func (o *llmOrchestrator) collectConfiguredToolDefs(
	ctx context.Context,
	tools []tool.Config,
) ([]llmadapter.ToolDefinition, map[string]struct{}, error) {
	// Helper to canonicalize names consistently (match registry behavior)
	canonical := func(name string) string {
		return strings.ToLower(strings.TrimSpace(name))
	}

	var defs []llmadapter.ToolDefinition
	included := make(map[string]struct{})

	for i := range tools {
		toolConfig := &tools[i]
		// Find the tool in registry for name/description
		t, found := o.config.ToolRegistry.Find(ctx, toolConfig.ID)
		if !found {
			return nil, nil, NewToolError(
				fmt.Errorf("tool not found"),
				ErrCodeToolNotFound,
				toolConfig.ID,
				map[string]any{"configured_tools": len(tools)},
			)
		}

		def := llmadapter.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
		}
		if toolConfig.InputSchema != nil {
			def.Parameters = *toolConfig.InputSchema
		} else {
			// Initialize empty parameters object for API compatibility
			def.Parameters = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}

		defs = append(defs, def)
		included[canonical(def.Name)] = struct{}{}
	}
	return defs, included, nil
}

// appendRegistryToolDefs adds any additional tools from the registry (e.g., MCP tools)
// that are not already included, assigning a minimal parameters schema when unknown.
func (o *llmOrchestrator) appendRegistryToolDefs(
	ctx context.Context,
	defs []llmadapter.ToolDefinition,
	included map[string]struct{},
) []llmadapter.ToolDefinition {
	// Helper to canonicalize names consistently
	canonical := func(name string) string {
		return strings.ToLower(strings.TrimSpace(name))
	}
	if o.config.ToolRegistry == nil {
		return defs
	}
	log := logger.FromContext(ctx)
	allTools, err := o.config.ToolRegistry.ListAll(ctx)
	if err != nil {
		// Non-fatal: proceed with configured tools only
		log.Warn("Failed to list tools from registry", "error", err)
		return defs
	}
	for _, rt := range allTools {
		name := rt.Name()
		if _, ok := included[canonical(name)]; ok {
			continue // already included via configured tools
		}
		// Default minimal schema
		params := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		// If the registry-provided tool exposes an argument schema, prefer it
		type argsTyper interface{ ArgsType() any }
		if at, ok := any(rt).(argsTyper); ok {
			if v := at.ArgsType(); v != nil {
				if m, isMap := v.(map[string]any); isMap && len(m) > 0 {
					params = m
				}
			}
		}

		def := llmadapter.ToolDefinition{
			Name:        name,
			Description: rt.Description(),
			Parameters:  params,
		}
		defs = append(defs, def)
		included[canonical(name)] = struct{}{}
	}
	return defs
}

func (o *llmOrchestrator) parseContentResponse(
	ctx context.Context,
	content string,
	action *agent.ActionConfig,
) (*core.Output, error) {
	output, err := o.parseContent(ctx, content, action)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeInvalidResponse, map[string]any{
			"content": content,
		})
	}
	return output, nil
}

// executeToolCalls executes tool calls and returns the result
func (o *llmOrchestrator) executeToolCalls(
	ctx context.Context,
	toolCalls []llmadapter.ToolCall,
	request Request,
) (*core.Output, error) {
	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls to execute")
	}
	log := logger.FromContext(ctx)
	log.Debug("Executing tool calls (structured output mode)",
		"tool_calls_count", len(toolCalls),
		"tools", extractToolNames(toolCalls),
	)
	// Use parallel execution with semaphore for concurrency control
	maxConcurrent := o.config.MaxConcurrentTools
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentTools
	}
	// Create error group for parallel execution
	g, ctx := errgroup.WithContext(ctx)
	// Create semaphore to limit concurrent executions
	sem := make(chan struct{}, maxConcurrent)
	// Results need to be collected in a thread-safe way
	results := make([]map[string]any, len(toolCalls))
	var resultsMu sync.Mutex
	// Execute tool calls in parallel with concurrency limit
	for i, tc := range toolCalls {
		index := i
		toolCall := tc
		g.Go(func() error {
			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}
			// Execute the tool call
			result, err := o.executeSingleToolCall(ctx, toolCall, request)
			if err != nil {
				return err
			}
			// Store result thread-safely
			resultsMu.Lock()
			results[index] = map[string]any{
				"tool_call_id": toolCall.ID,
				"tool_name":    toolCall.Name,
				"result":       result,
			}
			resultsMu.Unlock()
			return nil
		})
	}
	// Wait for all tool calls to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}
	// If only one tool call, return its result directly
	if len(results) == 1 {
		if result, ok := results[0]["result"].(*core.Output); ok {
			return result, nil
		}
	}
	// Multiple tool calls - return combined results
	output := core.Output(map[string]any{
		"results": results,
	})
	return &output, nil
}

// executeSingleToolCall executes a single tool call
func (o *llmOrchestrator) executeSingleToolCall(
	ctx context.Context,
	toolCall llmadapter.ToolCall,
	request Request,
) (*core.Output, error) {
	logger.FromContext(ctx).Debug("Processing single tool call (structured)",
		"tool_name", toolCall.Name,
		"tool_call_id", toolCall.ID,
	)
	// Find the tool
	tool, err := o.findToolForStructured(ctx, toolCall)
	if err != nil {
		return nil, err
	}
	// Execute the tool with structured logs and redaction policy applied
	result, err := o.callToolStructured(ctx, tool, toolCall)
	if err != nil {
		return nil, err
	}
	// Check for tool execution errors using improved error detection
	if toolErr, isError := IsToolExecutionError(result); isError {
		return nil, NewToolError(
			fmt.Errorf("tool execution failed: %s", toolErr.Message),
			ErrCodeToolExecution,
			toolCall.Name,
			map[string]any{
				"error_code":    toolErr.Code,
				"error_details": toolErr.Details,
			},
		)
	}
	// Parse the tool result with appropriate schema
	// Note: Tool output schema should come from the tool configuration, not action
	// For now, use action schema as fallback until tool schemas are properly wired
	return o.parseContent(ctx, result, request.Action)
}

// findToolForStructured locates a tool for structured execution with standardized logging and error type
func (o *llmOrchestrator) findToolForStructured(
	ctx context.Context,
	toolCall llmadapter.ToolCall,
) (Tool, error) {
	log := logger.FromContext(ctx)
	t, found := o.config.ToolRegistry.Find(ctx, toolCall.Name)
	if !found {
		log.Debug("Tool not found (structured mode)",
			"tool_name", toolCall.Name,
			"tool_call_id", toolCall.ID,
		)
		return nil, NewToolError(
			fmt.Errorf("tool not found for execution"),
			ErrCodeToolNotFound,
			toolCall.Name,
			map[string]any{"call_id": toolCall.ID},
		)
	}
	log.Debug("Executing tool (structured mode)",
		"tool_name", toolCall.Name,
		"tool_call_id", toolCall.ID,
	)
	return t, nil
}

// callToolStructured executes a tool call with structured logging and standardized error wrapping
func (o *llmOrchestrator) callToolStructured(
	ctx context.Context,
	t Tool,
	toolCall llmadapter.ToolCall,
) (string, error) {
	log := logger.FromContext(ctx)
	result, err := t.Call(ctx, string(toolCall.Arguments))
	if err != nil {
		log.Debug("Tool execution failed (structured mode)",
			"tool_name", toolCall.Name,
			"tool_call_id", toolCall.ID,
			"error", err.Error(),
		)
		return "", NewToolError(err, ErrCodeToolExecution, toolCall.Name, map[string]any{
			"call_id": toolCall.ID,
			// arguments intentionally redacted
		})
	}
	log.Debug("Tool execution succeeded (structured mode)",
		"tool_name", toolCall.Name,
		"tool_call_id", toolCall.ID,
	)
	return result, nil
}

// parseContent parses content and validates against schema if provided
func (o *llmOrchestrator) parseContent(
	ctx context.Context,
	content string,
	action *agent.ActionConfig,
) (*core.Output, error) {
	// Try to parse as JSON first
	var data any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		// Successfully parsed as JSON - check if it's an object
		if obj, ok := data.(map[string]any); ok {
			output := core.Output(obj)

			// Validate against schema if provided
			if err := o.validateOutput(ctx, &output, action); err != nil {
				return nil, NewValidationError(err, "output", obj)
			}

			return &output, nil
		}

		// Valid JSON but not an object - return error since core.Output expects map
		return nil, NewLLMError(
			fmt.Errorf("expected JSON object, got %T", data),
			ErrCodeInvalidResponse,
			map[string]any{"content": data},
		)
	}

	// Not valid JSON, return as text response
	output := core.Output(map[string]any{
		"response": content,
	})
	return &output, nil
}

// validateOutput validates output against schema
func (o *llmOrchestrator) validateOutput(ctx context.Context, output *core.Output, action *agent.ActionConfig) error {
	if action.OutputSchema == nil {
		return nil
	}
	return action.ValidateOutput(ctx, output)
}

// Close cleans up resources
func (o *llmOrchestrator) Close() error {
	if o.config.ToolRegistry != nil {
		return o.config.ToolRegistry.Close()
	}
	return nil
}

// handleNoToolCalls centralizes the logic for content-error, JSON-mode enforcement,
// and final parse/validation when the model returns no tool calls.
func (o *llmOrchestrator) handleNoToolCalls(
	ctx context.Context,
	response *llmadapter.LLMResponse,
	request Request,
	llmReq *llmadapter.LLMRequest,
	counters map[string]int,
	budget int,
	memories map[string]Memory,
) (*core.Output, bool, error) {
	log := logger.FromContext(ctx)
	// Content-level error observation
	if cont, cerr := o.handleContentErrorMessageNoToolCalls(
		ctx, response.Content, llmReq, counters, budget,
	); cont || cerr != nil {
		if cerr != nil {
			return nil, false, cerr
		}
		return nil, true, nil
	}
	// JSON-mode enforcement observation
	if cont, jerr := o.handleJSONModeNoToolCalls(
		ctx, response.Content, llmReq, counters, budget,
	); cont || jerr != nil {
		if jerr != nil {
			return nil, false, jerr
		}
		return nil, true, nil
	}
	// Attempt parse and validation; on failure, feed parser observation
	output, perr := o.parseContentResponse(ctx, response.Content, request.Action)
	if perr != nil {
		key := "output_validator"
		counters[key]++
		log.Debug("Final parse failed; continuing loop",
			"error", perr.Error(), "consecutive_errors", counters[key], "max", budget,
		)
		if counters[key] >= budget {
			log.Warn("Error budget exceeded - output validation", "key", key, "max", budget)
			return nil, false, fmt.Errorf("tool error budget exceeded for %s: %v", key, perr)
		}
		pseudoID := fmt.Sprintf("call_%s_%d", key, time.Now().UnixNano())
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: "assistant",
			ToolCalls: []llmadapter.ToolCall{{
				ID:        pseudoID,
				Name:      key,
				Arguments: json.RawMessage("{}"),
			}},
		})
		obs := map[string]any{
			"error":   "Invalid final response: schema/format check failed",
			"details": perr.Error(),
		}
		if b, mErr := json.Marshal(obs); mErr == nil {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: "tool",
				ToolResults: []llmadapter.ToolResult{{
					ID:          pseudoID,
					Name:        key,
					Content:     string(b),
					JSONContent: json.RawMessage(b),
				}},
			})
		} else {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: "tool",
				ToolResults: []llmadapter.ToolResult{{
					ID:      pseudoID,
					Name:    key,
					Content: fmt.Sprintf("{\\\"error\\\":%q}", perr.Error()),
				}},
			})
		}
		return nil, true, nil
	}
	// Success path
	o.storeResponseInMemoryAsync(ctx, memories, response, llmReq.Messages, request)
	return output, false, nil
}
