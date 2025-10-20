package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
)

type ResponseHandler interface {
	HandleNoToolCalls(
		ctx context.Context,
		response *llmadapter.LLMResponse,
		request Request,
		llmReq *llmadapter.LLMRequest,
		state *loopState,
	) (*core.Output, bool, error)
}

type responseHandler struct {
	cfg settings
}

func NewResponseHandler(cfg *settings) ResponseHandler {
	if cfg == nil {
		cfg = &settings{}
	}
	return &responseHandler{cfg: *cfg}
}

//nolint:gocritic // Request copied to avoid mutating caller state while handling finalization.
func (h *responseHandler) HandleNoToolCalls(
	ctx context.Context,
	response *llmadapter.LLMResponse,
	request Request,
	llmReq *llmadapter.LLMRequest,
	state *loopState,
) (*core.Output, bool, error) {
	if llmReq == nil {
		return nil, false, fmt.Errorf("llm request is required for completion handling")
	}
	if state == nil {
		state = &loopState{}
	}
	if msg, hasErr := extractTopLevelErrorMessage(response.Content); hasErr {
		err := core.NewError(
			fmt.Errorf("llm returned error payload"),
			ErrCodeOutputValidation,
			map[string]any{"detail": msg},
		)
		return h.retryFinalize(ctx, err, msg, llmReq, state, request)
	}
	output, err := h.parseContent(ctx, response.Content, request.Action)
	if err != nil {
		return h.retryFinalize(ctx, err, err.Error(), llmReq, state, request)
	}
	return output, false, nil
}

//nolint:gocritic // Request copied to ensure logging reflects the original action metadata across retries.
func (h *responseHandler) retryFinalize(
	ctx context.Context,
	finalErr error,
	detail string,
	llmReq *llmadapter.LLMRequest,
	state *loopState,
	request Request,
) (*core.Output, bool, error) {
	if !h.shouldRetryFinalize(state) {
		return nil, false, finalErr
	}
	attempt := state.finalizeAttemptNumber()
	remaining := state.remainingFinalizeRetries()
	h.logFinalizeRetry(ctx, finalErr, attempt, remaining)
	h.injectFinalizeFeedback(llmReq, detail, finalErr, state)
	h.recordFinalizeRetry(ctx, finalErr, attempt, remaining, request.Action)
	return nil, true, nil
}

func (h *responseHandler) shouldRetryFinalize(state *loopState) bool {
	return state != nil && state.allowFinalizeRetry()
}

func (h *responseHandler) logFinalizeRetry(
	ctx context.Context,
	finalErr error,
	attempt int,
	remaining int,
) {
	logger.FromContext(ctx).Warn(
		"Final response invalid; requesting retry",
		"error", core.RedactError(finalErr),
		"attempt", attempt,
		"remaining_retries", remaining,
	)
}

func (h *responseHandler) injectFinalizeFeedback(
	llmReq *llmadapter.LLMRequest,
	detail string,
	finalErr error,
	state *loopState,
) {
	if llmReq == nil {
		return
	}
	if state.runtime.finalizeFeedbackBase < 0 {
		state.runtime.finalizeFeedbackBase = len(llmReq.Messages)
	}
	base := len(llmReq.Messages)
	if state.runtime.finalizeFeedbackBase >= 0 &&
		state.runtime.finalizeFeedbackBase <= len(llmReq.Messages) {
		base = state.runtime.finalizeFeedbackBase
	}
	feedbackDetail := detail
	if redacted := core.RedactError(finalErr); redacted != "" {
		feedbackDetail = redacted
	}
	feedback := llmadapter.Message{
		Role:    "user",
		Content: h.buildFinalizeFeedback(feedbackDetail, state),
	}
	if base < 0 || base > len(llmReq.Messages) {
		llmReq.Messages = append(llmReq.Messages, feedback)
		return
	}
	llmReq.Messages = append(llmReq.Messages[:base], feedback)
}

func (h *responseHandler) recordFinalizeRetry(
	ctx context.Context,
	finalErr error,
	attempt int,
	remaining int,
	action *agent.ActionConfig,
) {
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:    "finalize_retry",
		Severity: telemetry.SeverityWarn,
		Payload: map[string]any{
			"attempt":           attempt,
			"remaining_retries": remaining,
			"detail":            core.RedactError(finalErr),
			"action_id":         actionIDOrEmpty(action),
		},
	})
}

func (h *responseHandler) buildFinalizeFeedback(issue string, state *loopState) string {
	attempt := state.finalizeAttemptNumber()
	budget := state.finalizeBudget()
	remaining := state.remainingFinalizeRetries()
	instruction := h.finalizationInstruction(state)
	total := budget
	if total <= 0 {
		total = attempt
	}
	lines := []string{
		fmt.Sprintf("FINALIZATION_FEEDBACK (attempt %d of %d):", attempt, total),
		fmt.Sprintf("Issue: %s", issue),
		instruction,
	}
	if remaining > 0 {
		lines = append(lines, fmt.Sprintf("Retries remaining: %d.", remaining))
	}
	return strings.Join(lines, "\n")
}

func (h *responseHandler) finalizationInstruction(state *loopState) string {
	if requiresJSONOutputForState(state) {
		schemaHint := "that matches the expected schema."
		if action := state.actionConfig(); action != nil {
			if id := schema.GetID(action.OutputSchema); id != "" {
				schemaHint = fmt.Sprintf("that matches the %q schema.", id)
			}
		}
		return fmt.Sprintf(
			"Instruction: Respond ONLY with a valid JSON object %s\n"+
				"Reminder: Do not include commentary, markdown, or tool calls.",
			schemaHint,
		)
	}
	return "Instruction: Respond with a plain-text answer that addresses the request.\n" +
		"Reminder: Do not wrap the response in JSON or add tool calls."
}

func actionIDOrEmpty(action *agent.ActionConfig) string {
	if action == nil {
		return ""
	}
	return action.ID
}

func (h *responseHandler) parseContent(
	ctx context.Context,
	content string,
	action *agent.ActionConfig,
) (*core.Output, error) {
	if !requiresJSONOutputForAction(action) {
		output := core.Output(map[string]any{"response": content})
		return &output, nil
	}
	value, err := h.parseStructuredValue(content, action)
	if err != nil {
		return nil, err
	}
	return h.buildStructuredOutput(ctx, value, action)
}

func (h *responseHandler) parseStructuredValue(content string, action *agent.ActionConfig) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(content), &value); err == nil {
		return value, nil
	}
	if snippet, ok := extractJSONObject(content); ok {
		if err := json.Unmarshal([]byte(snippet), &value); err == nil {
			return value, nil
		}
	}
	actionID := ""
	if action != nil {
		actionID = action.ID
	}
	return nil, NewLLMError(
		fmt.Errorf("expected structured JSON output but received plain text"),
		ErrCodeInvalidResponse,
		map[string]any{
			"action":  actionID,
			"content": content,
		},
	)
}

func (h *responseHandler) buildStructuredOutput(
	ctx context.Context,
	value any,
	action *agent.ActionConfig,
) (*core.Output, error) {
	if err := h.validateStructuredValue(ctx, value, action); err != nil {
		return nil, err
	}
	if obj, ok := value.(map[string]any); ok {
		output := core.Output(obj)
		return &output, nil
	}
	output := core.Output{
		core.OutputRootKey: value,
	}
	return &output, nil
}

func (h *responseHandler) validateStructuredValue(
	ctx context.Context,
	value any,
	action *agent.ActionConfig,
) error {
	if action == nil || action.OutputSchema == nil {
		return nil
	}
	validator := schema.NewParamsValidator(value, action.OutputSchema, action.ID)
	return validator.Validate(ctx)
}

func requiresJSONOutputForState(state *loopState) bool {
	if state == nil {
		return false
	}
	return requiresJSONOutputForAction(state.actionConfig())
}

func requiresJSONOutputForAction(action *agent.ActionConfig) bool {
	if action == nil {
		return false
	}
	if action.OutputSchema != nil {
		return true
	}
	return action.ShouldUseJSONOutput()
}

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
		if msg, ok := stringifyErrorValue(ev); ok {
			return msg, true
		}
	}
	return "", false
}

// extractJSONObject scans the provided string and returns the first complete
// top-level JSON value (object or array). It keeps track of whether the parser
// is inside a quoted string and ignores structural characters that appear
// within strings or escaped sequences.
func extractJSONObject(s string) (string, bool) {
	inString := false
	escaped := false
	start := -1
	var stack []byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			if len(stack) == 0 {
				start = i
			}
			if ch == '{' {
				stack = append(stack, '}')
			} else {
				stack = append(stack, ']')
			}
		case '}', ']':
			if len(stack) == 0 {
				continue
			}
			expected := stack[len(stack)-1]
			if ch != expected {
				return "", false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 && start >= 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

func stringifyErrorValue(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return stringifyErrorString(x)
	case map[string]any:
		return stringifyErrorMap(x)
	case []any:
		return stringifyErrorSlice(x)
	default:
		return stringifyErrorDefault(x)
	}
}

func stringifyErrorString(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func stringifyErrorMap(m map[string]any) (string, bool) {
	if msg, ok := m["message"].(string); ok {
		if s, ok := stringifyErrorString(msg); ok {
			return s, true
		}
	}
	if b, err := json.Marshal(m); err == nil && len(b) > 0 {
		return string(b), true
	}
	return fmt.Sprintf("%v", m), true
}

func stringifyErrorSlice(arr []any) (string, bool) {
	parts := make([]string, 0, len(arr))
	for _, it := range arr {
		if s, ok := it.(string); ok {
			if s, ok := stringifyErrorString(s); ok {
				parts = append(parts, s)
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; "), true
	}
	if b, err := json.Marshal(arr); err == nil && len(b) > 0 {
		return string(b), true
	}
	return fmt.Sprintf("%v", arr), true
}

func stringifyErrorDefault(x any) (string, bool) {
	if b, err := json.Marshal(x); err == nil && len(b) > 0 {
		return string(b), true
	}
	return fmt.Sprintf("%v", x), true
}
