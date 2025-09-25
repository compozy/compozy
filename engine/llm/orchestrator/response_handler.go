package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
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

const (
	keyContentValidator = "content_validator"
	keyOutputParser     = "output_parser"
	keyOutputValidator  = "output_validator"
)

func (h *responseHandler) HandleNoToolCalls(
	ctx context.Context,
	response *llmadapter.LLMResponse,
	request Request,
	llmReq *llmadapter.LLMRequest,
	state *loopState,
) (*core.Output, bool, error) {
	cont, err := h.handleContentError(ctx, response.Content, llmReq, state)
	if cont || err != nil {
		if err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	cont, err = h.handleJSONMode(ctx, response.Content, llmReq, state)
	if cont || err != nil {
		if err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	output, err := h.parseContent(ctx, response.Content, request.Action)
	if err != nil {
		cont, hErr := h.continueAfterOutputValidationFailure(ctx, err, llmReq, state)
		if hErr != nil {
			return nil, false, hErr
		}
		if cont {
			return nil, true, nil
		}
	}
	return output, false, nil
}

func (h *responseHandler) handleJSONMode(
	ctx context.Context,
	content string,
	llmReq *llmadapter.LLMRequest,
	state *loopState,
) (bool, error) {
	if !llmReq.Options.UseJSONMode {
		return false, nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(content), &obj); err == nil && obj != nil {
		return false, nil
	}
	key := keyOutputParser
	state.toolErrors[key]++
	logger.FromContext(ctx).Debug("Non-JSON content with JSON mode; continuing loop",
		"consecutive_errors", state.toolErrors[key],
		"max", state.budgetFor(key),
	)
	if state.toolErrors[key] >= state.budgetFor(key) {
		return false, core.NewError(
			fmt.Errorf("tool error budget exceeded for %s", key),
			ErrCodeBudgetExceeded,
			map[string]any{
				"key":     key,
				"attempt": state.toolErrors[key],
				"max":     state.budgetFor(key),
				"details": "expected JSON object in JSON mode",
			},
		)
	}
	pseudoID := fmt.Sprintf("call_%s_%d", key, time.Now().UnixNano())
	llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
		Role: llmadapter.RoleAssistant,
		ToolCalls: []llmadapter.ToolCall{{
			ID:        pseudoID,
			Name:      key,
			Arguments: json.RawMessage("{}"),
		}},
	})
	obs := map[string]any{
		"error":   "Invalid final response: expected JSON object (json_mode=true)",
		"example": map[string]any{"response": "..."},
	}
	if payload, err := json.Marshal(obs); err == nil {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: llmadapter.RoleTool,
			ToolResults: []llmadapter.ToolResult{{
				ID:          pseudoID,
				Name:        key,
				Content:     string(payload),
				JSONContent: json.RawMessage(payload),
			}},
		})
	} else {
		fb := map[string]any{"error": "Invalid final response"}
		if b, e := json.Marshal(fb); e == nil {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: llmadapter.RoleTool,
				ToolResults: []llmadapter.ToolResult{{
					ID:          pseudoID,
					Name:        key,
					Content:     string(b),
					JSONContent: json.RawMessage(b),
				}},
			})
		} else {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: llmadapter.RoleTool,
				ToolResults: []llmadapter.ToolResult{{
					ID:      pseudoID,
					Name:    key,
					Content: `{"error":"Invalid final response"}`,
				}},
			})
		}
	}
	return true, nil
}

func (h *responseHandler) continueAfterOutputValidationFailure(
	ctx context.Context,
	valErr error,
	llmReq *llmadapter.LLMRequest,
	state *loopState,
) (bool, error) {
	log := logger.FromContext(ctx)
	key := keyOutputValidator
	state.toolErrors[key]++
	log.Debug("Output validation failed; continuing loop",
		"error", core.RedactError(valErr),
		"attempt", state.toolErrors[key],
		"max", state.budgetFor(key),
	)
	if state.toolErrors[key] >= state.budgetFor(key) {
		log.Warn("Error budget exceeded - output validation", "key", key)
		return false, core.NewError(
			fmt.Errorf("tool error budget exceeded for %s", key),
			ErrCodeBudgetExceeded,
			map[string]any{
				"key":     key,
				"attempt": state.toolErrors[key],
				"max":     state.budgetFor(key),
				"details": valErr.Error(),
			},
		)
	}
	pseudoID := fmt.Sprintf("call_%s_%d", key, time.Now().UnixNano())
	llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
		Role: llmadapter.RoleAssistant,
		ToolCalls: []llmadapter.ToolCall{{
			ID:        pseudoID,
			Name:      key,
			Arguments: json.RawMessage("{}"),
		}},
	})
	obs := map[string]any{
		"error":   "Invalid final response: schema/format check failed",
		"details": valErr.Error(),
		"attempt": state.toolErrors[key],
	}
	if payload, merr := json.Marshal(obs); merr == nil {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: llmadapter.RoleTool,
			ToolResults: []llmadapter.ToolResult{{
				ID:          pseudoID,
				Name:        key,
				Content:     string(payload),
				JSONContent: json.RawMessage(payload),
			}},
		})
	} else {
		fb := map[string]any{"error": valErr.Error()}
		if b, e := json.Marshal(fb); e == nil {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: llmadapter.RoleTool,
				ToolResults: []llmadapter.ToolResult{{
					ID:          pseudoID,
					Name:        key,
					Content:     string(b),
					JSONContent: json.RawMessage(b),
				}},
			})
		} else {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: llmadapter.RoleTool,
				ToolResults: []llmadapter.ToolResult{{
					ID:      pseudoID,
					Name:    key,
					Content: `{"error":"output validation failed"}`,
				}},
			})
		}
	}
	return true, nil
}

func (h *responseHandler) handleContentError(
	ctx context.Context,
	content string,
	llmReq *llmadapter.LLMRequest,
	state *loopState,
) (bool, error) {
	msg, hasErr := extractTopLevelErrorMessage(content)
	if !hasErr {
		return false, nil
	}
	key := keyContentValidator
	state.toolErrors[key]++
	logger.FromContext(ctx).Debug("Content-level error detected; continuing loop",
		"error_message", msg,
		"consecutive_errors", state.toolErrors[key],
		"max", state.budgetFor(key),
	)
	if state.toolErrors[key] >= state.budgetFor(key) {
		return false, core.NewError(
			fmt.Errorf("tool error budget exceeded for %s", key),
			ErrCodeBudgetExceeded,
			map[string]any{"key": key, "attempt": state.toolErrors[key], "max": state.budgetFor(key), "details": msg},
		)
	}
	pseudoID := fmt.Sprintf("call_%s_%d", key, time.Now().UnixNano())
	llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
		Role: llmadapter.RoleAssistant,
		ToolCalls: []llmadapter.ToolCall{{
			ID:        pseudoID,
			Name:      key,
			Arguments: json.RawMessage("{}"),
		}},
	})
	obs := map[string]any{"error": msg}
	if payload, err := json.Marshal(obs); err == nil {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: llmadapter.RoleTool,
			ToolResults: []llmadapter.ToolResult{{
				ID:          pseudoID,
				Name:        key,
				Content:     string(payload),
				JSONContent: json.RawMessage(payload),
			}},
		})
	} else {
		fb := map[string]any{"error": msg}
		if b, e := json.Marshal(fb); e == nil {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: llmadapter.RoleTool,
				ToolResults: []llmadapter.ToolResult{{
					ID:          pseudoID,
					Name:        key,
					Content:     string(b),
					JSONContent: json.RawMessage(b),
				}},
			})
		} else {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: llmadapter.RoleTool,
				ToolResults: []llmadapter.ToolResult{{
					ID:      pseudoID,
					Name:    key,
					Content: `{"error":"content validation failed"}`,
				}},
			})
		}
	}
	return true, nil
}

func (h *responseHandler) parseContent(
	ctx context.Context,
	content string,
	action *agent.ActionConfig,
) (*core.Output, error) {
	var data any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		if obj, ok := data.(map[string]any); ok {
			output := core.Output(obj)
			if err := h.validateOutput(ctx, &output, action); err != nil {
				return nil, err
			}
			return &output, nil
		}
		return nil, NewLLMError(
			fmt.Errorf("expected JSON object, got %T", data),
			ErrCodeInvalidResponse,
			map[string]any{"content": data},
		)
	}

	if action != nil && action.ShouldUseJSONOutput() {
		return nil, NewLLMError(
			fmt.Errorf("expected structured JSON output but received plain text"),
			ErrCodeInvalidResponse,
			map[string]any{
				"action":  action.ID,
				"content": content,
			},
		)
	}

	output := core.Output(map[string]any{"response": content})
	return &output, nil
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
