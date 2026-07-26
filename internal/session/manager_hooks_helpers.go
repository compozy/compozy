package session

import (
	"context"
	"encoding/json"

	"fmt"
	"maps"
	"strings"

	"github.com/compozy/agh/internal/acp"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Manager) warnHookDispatch(ctx context.Context, session *Session, event hookspkg.HookEvent, err error) {
	if err == nil {
		return
	}
	ctx = m.hookLifecycleContext(ctx)

	m.sessionLogger(session).WarnContext(
		ctx,
		"session: hook dispatch failed",
		"hook_event", event.String(),
		"error", err,
	)
}

func hookMessageDetails(eventType string) (string, string, bool) {
	switch strings.TrimSpace(eventType) {
	case acp.EventTypeAgentMessage:
		return hookMessageRoleAssistant, hookMessageDeltaTypeText, true
	case acp.EventTypeThought:
		return hookMessageRoleAssistant, hookMessageDeltaTypeThought, true
	default:
		return "", "", false
	}
}

func nextPromptMessageID(turnID string, sequence int) string {
	base := strings.TrimSpace(turnID)
	if base == "" {
		base = "msg"
	}
	if sequence <= 0 {
		return base + "-message"
	}
	return fmt.Sprintf("%s-message-%d", base, sequence)
}

func cloneSessionRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneSessionContextBlocks(blocks []hookspkg.ContextBlock) []hookspkg.ContextBlock {
	if len(blocks) == 0 {
		return nil
	}

	cloned := make([]hookspkg.ContextBlock, 0, len(blocks))
	for _, block := range blocks {
		cloned = append(cloned, hookspkg.ContextBlock{
			Kind:     strings.TrimSpace(block.Kind),
			Text:     block.Text,
			Metadata: cloneStringMap(block.Metadata),
		})
	}
	return cloned
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func hookDispatchContext(ctx context.Context, manager *Manager, session *Session) context.Context {
	if ctx == nil || session == nil {
		return ctx
	}

	if manager != nil {
		ctx = hookspkg.WithDispatchEventEmitter(ctx, hookDispatchEventEmitter{
			manager: manager,
			session: session,
		})
	}

	writer, ok := session.recorderHandle().(hookspkg.HookRunWriter)
	if !ok || writer == nil {
		return ctx
	}

	return hookspkg.WithHookRunWriter(ctx, writer)
}
