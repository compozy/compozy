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
	if state == nil || !state.allowFinalizeRetry() {
		return nil, false, finalErr
	}
	attempt := state.finalizeAttemptNumber()
	remaining := state.remainingFinalizeRetries()
	logger.FromContext(ctx).Warn(
		"Final response invalid; requesting retry",
		"error", core.RedactError(finalErr),
		"attempt", attempt,
		"remaining_retries", remaining,
	)
	if llmReq != nil {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role:    "user",
			Content: h.buildFinalizeFeedback(detail, state),
		})
	}
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:    "finalize_retry",
		Severity: telemetry.SeverityWarn,
		Payload: map[string]any{
			"attempt":           attempt,
			"remaining_retries": remaining,
			"detail":            core.RedactError(finalErr),
			"action_id":         actionIDOrEmpty(request.Action),
		},
	})
	return nil, true, nil
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
	expectStructured := requiresJSONOutputForAction(action)

	var data any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		if obj, ok := data.(map[string]any); ok {
			output := core.Output(obj)
			if expectStructured {
				if err := h.validateOutput(ctx, &output, action); err != nil {
					return nil, err
				}
			}
			return &output, nil
		}
		if expectStructured {
			return nil, NewLLMError(
				fmt.Errorf("expected JSON object, got %T", data),
				ErrCodeInvalidResponse,
				map[string]any{"content": data},
			)
		}
	}

	if expectStructured {
		if snippet, ok := extractJSONObject(content); ok {
			var obj map[string]any
			if err := json.Unmarshal([]byte(snippet), &obj); err == nil {
				output := core.Output(obj)
				if err := h.validateOutput(ctx, &output, action); err != nil {
					return nil, err
				}
				return &output, nil
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

	output := core.Output(map[string]any{"response": content})
	return &output, nil
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

func (h *responseHandler) validateOutput(ctx context.Context, output *core.Output, action *agent.ActionConfig) error {
	if action == nil || action.OutputSchema == nil {
		return nil
	}
	return action.ValidateOutput(ctx, output)
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

func extractJSONObject(s string) (string, bool) {
	inString := false
	escaped := false
	depth := 0
	start := -1
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
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
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
