package core

import (
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func (e *PromptStreamEncoder) ensureMessageStarted(writer FlushWriter, event acp.AgentEvent) error {
	if e.messageStarted {
		return nil
	}

	messageID := strings.TrimSpace(event.TurnID)
	if messageID == "" {
		messageID = e.messageID
	}
	if messageID == "" {
		messageID = "msg-" + strings.ReplaceAll(e.now(), ":", "-")
	}
	e.messageID = messageID
	e.messageStarted = true

	return WriteSSE(writer, SSEMessage{
		Data: promptStartPayload{Type: "start", MessageID: e.messageID},
	})
}

func (e *PromptStreamEncoder) ensureTextStarted(writer FlushWriter) error {
	if e.textStarted {
		return nil
	}
	if err := e.closeReasoningBlock(writer); err != nil {
		return err
	}
	e.textBlockSeq++
	e.textBlockID = e.messageID + "-text-" + strconv.Itoa(e.textBlockSeq)
	e.textStarted = true
	return WriteSSE(writer, SSEMessage{
		Data: promptBlockPayload{Type: "text-start", ID: e.textBlockID},
	})
}

func (e *PromptStreamEncoder) ensureReasoningStarted(writer FlushWriter) error {
	if e.reasoningStarted {
		return nil
	}
	if err := e.closeTextBlock(writer); err != nil {
		return err
	}
	e.reasoningBlockSeq++
	e.reasoningBlockID = e.messageID + "-reasoning-" + strconv.Itoa(e.reasoningBlockSeq)
	e.reasoningStarted = true
	return WriteSSE(writer, SSEMessage{
		Data: promptBlockPayload{Type: "reasoning-start", ID: e.reasoningBlockID},
	})
}

func (e *PromptStreamEncoder) closeTextBlock(writer FlushWriter) error {
	if !e.textStarted {
		return nil
	}
	if err := WriteSSE(writer, SSEMessage{
		Data: promptBlockPayload{Type: "text-end", ID: e.textBlockID},
	}); err != nil {
		return err
	}
	e.textStarted = false
	e.textBlockID = ""
	return nil
}

func (e *PromptStreamEncoder) closeReasoningBlock(writer FlushWriter) error {
	if e.reasoningStarted {
		if err := WriteSSE(writer, SSEMessage{
			Data: promptBlockPayload{Type: "reasoning-end", ID: e.reasoningBlockID},
		}); err != nil {
			return err
		}
		e.reasoningStarted = false
		e.reasoningBlockID = ""
	}
	return nil
}

func (e *PromptStreamEncoder) closeOpenBlocks(writer FlushWriter) error {
	if err := e.closeTextBlock(writer); err != nil {
		return err
	}
	return e.closeReasoningBlock(writer)
}

func (e *PromptStreamEncoder) finish(writer FlushWriter, event acp.AgentEvent) error {
	if e.finished {
		return nil
	}
	if err := e.ensureMessageStarted(writer, event); err != nil {
		return err
	}
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	e.finished = true

	finishPayload := promptFinishPayload{Type: "finish"}
	if finishReason := promptAISDKFinishReason(event.PromptStopReason); finishReason != "" {
		finishPayload.FinishReason = finishReason
	}

	if err := WriteSSE(writer, SSEMessage{
		Data: finishPayload,
	}); err != nil {
		return err
	}
	return WriteSSERaw(writer, "", "[DONE]")
}

func promptAISDKFinishReason(stopReason acp.PromptStopReason) string {
	switch strings.TrimSpace(string(stopReason)) {
	case "", "end_turn":
		return promptStreamStopKey
	case "max_tokens":
		return "length"
	case toolInvokeStatusCanceled, "max_turn_requests", "refusal":
		return "other"
	default:
		return "other"
	}
}

func promptPermissionDataPartID(event acp.AgentEvent) string {
	if requestID := strings.TrimSpace(event.RequestID); requestID != "" {
		return requestID
	}
	if turnID := strings.TrimSpace(event.TurnID); turnID != "" {
		if toolCallID := strings.TrimSpace(event.ToolCallID); toolCallID != "" {
			return turnID + ":" + toolCallID
		}
	}
	if toolCallID := strings.TrimSpace(event.ToolCallID); toolCallID != "" {
		return toolCallID
	}
	return ""
}
