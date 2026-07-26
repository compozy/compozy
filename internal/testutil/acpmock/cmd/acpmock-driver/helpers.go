package main

import (
	"context"
	"encoding/json"

	"fmt"

	"strings"

	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

func (a *mockAgent) emitSandboxFailure(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
	toolCallID string,
	title string,
	observedError string,
) error {
	if toolCallID != "" {
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update: acpsdk.UpdateToolCall(
				acpsdk.ToolCallId(toolCallID),
				acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusFailed),
				acpsdk.WithUpdateTitle(title),
				acpsdk.WithUpdateContent(textToolContent(observedError)),
			),
		}); err != nil {
			return err
		}
		if err := pauseForDelivery(ctx); err != nil {
			return err
		}
	}
	if text := strings.TrimSpace(step.EmitText); text != "" {
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateAgentMessageText(text),
		}); err != nil {
			return err
		}
		return pauseForDelivery(ctx)
	}
	return nil
}

func defaultPermissionOptions() []acpsdk.PermissionOption {
	return []acpsdk.PermissionOption{
		{
			Kind:     acpsdk.PermissionOptionKindAllowOnce,
			Name:     "allow once",
			OptionId: acpsdk.PermissionOptionId("allow-once"),
		},
		{
			Kind:     acpsdk.PermissionOptionKindAllowAlways,
			Name:     "allow always",
			OptionId: acpsdk.PermissionOptionId("allow-always"),
		},
		{
			Kind:     acpsdk.PermissionOptionKindRejectOnce,
			Name:     "reject once",
			OptionId: acpsdk.PermissionOptionId("reject-once"),
		},
		{
			Kind:     acpsdk.PermissionOptionKindRejectAlways,
			Name:     "reject always",
			OptionId: acpsdk.PermissionOptionId("reject-always"),
		},
	}
}

func selectedDecision(response acpsdk.RequestPermissionResponse) string {
	if response.Outcome.Selected != nil {
		return string(response.Outcome.Selected.OptionId)
	}
	return "canceled"
}

func normalizedChunks(step acpmock.Step) []string {
	if len(step.Chunks) > 0 {
		chunks := make([]string, 0, len(step.Chunks))
		chunks = append(chunks, step.Chunks...)
		return chunks
	}
	if strings.TrimSpace(step.Text) != "" {
		return []string{step.Text}
	}
	return []string{""}
}

func decodePromptMeta(raw any) (acp.PromptMeta, error) {
	if raw == nil {
		return acp.PromptMeta{}, nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return acp.PromptMeta{}, fmt.Errorf("encode prompt metadata: %w", err)
	}

	var meta acp.PromptMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return acp.PromptMeta{}, fmt.Errorf("decode prompt metadata: %w", err)
	}
	if err := meta.Validate(); err != nil {
		return acp.PromptMeta{}, err
	}
	return meta.Normalize(), nil
}

func extractPromptText(blocks []acpsdk.ContentBlock) string {
	lastText := ""
	for _, block := range blocks {
		if block.Text != nil {
			lastText = block.Text.Text
		}
	}
	return strings.TrimSpace(lastText)
}

func toolLocations(rawPath string) []acpsdk.ToolCallLocation {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return nil
	}
	return []acpsdk.ToolCallLocation{{Path: trimmed}}
}

func textToolContent(text string) []acpsdk.ToolCallContent {
	return []acpsdk.ToolCallContent{acpsdk.ToolContent(acpsdk.TextBlock(text))}
}

func toolKind(raw string, fallback acpsdk.ToolKind) acpsdk.ToolKind {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	return acpsdk.ToolKind(trimmed)
}

func toolStatus(raw string, fallback acpsdk.ToolCallStatus) acpsdk.ToolCallStatus {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	return acpsdk.ToolCallStatus(trimmed)
}

func parseRawJSON(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return value, true
}

func mustRawJSON(raw json.RawMessage) any {
	value, _ := parseRawJSON(raw)
	return value
}

func pauseForDelivery(ctx context.Context) error {
	timer := time.NewTimer(5 * time.Millisecond)
	defer timer.Stop()

	select {
	case <-contextDone(ctx):
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func contextDone(ctx context.Context) <-chan struct{} {
	if ctx == nil {
		return nil
	}
	return ctx.Done()
}

func driverControlContextErr(promptCtx context.Context, lifetimeCtx context.Context) error {
	if promptCtx != nil {
		if err := promptCtx.Err(); err != nil {
			return err
		}
	}
	if lifetimeCtx != nil {
		if err := lifetimeCtx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (a *mockAgent) lifecycleContext() context.Context {
	if a == nil || a.lifecycleCtx == nil {
		return context.Background()
	}
	return a.lifecycleCtx
}

func (a *mockAgent) waitForAsyncControls() {
	if a == nil {
		return
	}
	a.asyncWG.Wait()
}
