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

func (h *responseHandler) HandleNoToolCalls(
	ctx context.Context,
	response *llmadapter.LLMResponse,
	request Request,
	llmReq *llmadapter.LLMRequest,
	state *loopState,
) (*core.Output, bool, error) {
	log := logger.FromContext(ctx)

	if cont, err := h.handleContentError(ctx, response.Content, llmReq, state); cont || err != nil {
		if err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}

	if cont, err := h.handleJSONMode(ctx, response.Content, llmReq, state); cont || err != nil {
		if err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}

	output, err := h.parseContent(ctx, response.Content, request.Action)
	if err != nil {
		key := "output_validator"
		state.toolErrors[key]++
		log.Debug("Output validation failed; continuing loop",
			"error", core.RedactError(err),
			"attempt", state.toolErrors[key],
			"max", state.budgetFor(key),
		)
		if state.toolErrors[key] >= state.budgetFor(key) {
			log.Warn("Error budget exceeded - output validation", "key", key)
			return nil, false, fmt.Errorf("tool error budget exceeded for %s: %v", key, err)
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
			"details": err.Error(),
			"attempt": state.toolErrors[key],
		}
		if payload, merr := json.Marshal(obs); merr == nil {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: "tool",
				ToolResults: []llmadapter.ToolResult{{
					ID:          pseudoID,
					Name:        key,
					Content:     string(payload),
					JSONContent: json.RawMessage(payload),
				}},
			})
		} else {
			llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
				Role: "tool",
				ToolResults: []llmadapter.ToolResult{{
					ID:      pseudoID,
					Name:    key,
					Content: fmt.Sprintf("{\\\"error\\\":%q}", err.Error()),
				}},
			})
		}
		return nil, true, nil
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

	key := "output_parser"
	state.toolErrors[key]++
	logger.FromContext(ctx).Debug("Non-JSON content with JSON mode; continuing loop",
		"consecutive_errors", state.toolErrors[key],
		"max", state.budgetFor(key),
	)
	if state.toolErrors[key] >= state.budgetFor(key) {
		return false, fmt.Errorf("tool error budget exceeded for %s: expected JSON object in JSON mode", key)
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
		"error":   "Invalid final response: expected JSON object (json_mode=true)",
		"example": map[string]any{"response": "..."},
	}
	if payload, err := json.Marshal(obs); err == nil {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: "tool",
			ToolResults: []llmadapter.ToolResult{{
				ID:          pseudoID,
				Name:        key,
				Content:     string(payload),
				JSONContent: json.RawMessage(payload),
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

	key := "content_validator"
	state.toolErrors[key]++
	logger.FromContext(ctx).Debug("Content-level error detected; continuing loop",
		"error_message", msg,
		"consecutive_errors", state.toolErrors[key],
		"max", state.budgetFor(key),
	)
	if state.toolErrors[key] >= state.budgetFor(key) {
		return false, fmt.Errorf("tool error budget exceeded for %s: %s", key, msg)
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

	obs := map[string]any{"error": msg}
	if payload, err := json.Marshal(obs); err == nil {
		llmReq.Messages = append(llmReq.Messages, llmadapter.Message{
			Role: "tool",
			ToolResults: []llmadapter.ToolResult{{
				ID:          pseudoID,
				Name:        key,
				Content:     string(payload),
				JSONContent: json.RawMessage(payload),
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
		switch v := ev.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v, true
			}
		case map[string]any:
			if msg, ok := v["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return msg, true
			}
			if b, err := json.Marshal(v); err == nil && len(b) > 0 {
				return string(b), true
			}
			return fmt.Sprintf("%v", v), true
		default:
			if b, err := json.Marshal(v); err == nil && len(b) > 0 {
				return string(b), true
			}
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}
