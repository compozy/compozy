package acp

import (
	"context"
	"encoding/json"
	"fmt"

	acpsdk "github.com/coder/acp-go-sdk"
)

func (p *AgentProcess) handleSessionUpdate(params json.RawMessage) error {
	return p.handleSessionUpdateWithContext(context.Background(), params)
}

func (p *AgentProcess) handleSessionUpdateWithContext(ctx context.Context, params json.RawMessage) error {
	var raw wireSessionNotification
	if err := json.Unmarshal(params, &raw); err != nil {
		return fmt.Errorf("acp: decode session/update notification: %w", err)
	}
	var envelope wireSessionUpdateEnvelope
	if err := json.Unmarshal(raw.Update, &envelope); err != nil {
		return fmt.Errorf("acp: decode session/update envelope: %w", err)
	}

	if envelope.SessionUpdate == "usage_update" {
		var update wireUsageUpdate
		if err := json.Unmarshal(raw.Update, &update); err != nil {
			return fmt.Errorf("acp: decode usage_update: %w", err)
		}
		usage := tokenUsageFromUsageUpdate(p.activeTurnID(), update)
		if !usage.IsZero() {
			merged := p.mergePromptUsage(usage)
			p.emitPromptEvent(AgentEvent{
				Type:      EventTypeUsage,
				SessionID: string(raw.SessionID),
				TurnID:    merged.TurnID,
				Timestamp: usage.Timestamp,
				Usage:     &merged,
				Raw:       CloneRawMessage(raw.Update),
			})
		}
		return nil
	}

	var notification acpsdk.SessionNotification
	if err := json.Unmarshal(params, &notification); err != nil {
		return fmt.Errorf("acp: decode session notification: %w", err)
	}
	if notification.Update.ConfigOptionUpdate != nil {
		p.setConfigOptions(sessionConfigOptionsFromSDK(notification.Update.ConfigOptionUpdate.ConfigOptions))
	}
	if notification.Update.CurrentModeUpdate != nil {
		p.setConfigOptionCurrent("mode", string(notification.Update.CurrentModeUpdate.CurrentModeId))
	}

	event, err := translateSessionUpdate(notification, raw.Update, p.activeTurnID())
	if err != nil {
		return err
	}
	event = p.markToolEventPrechecked(event)
	p.emitPromptEvent(event)
	p.injectSteerAfterToolResult(ctx, event)
	return nil
}
