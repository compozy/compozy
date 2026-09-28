package acp

import (
	"encoding/json"
	"fmt"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/redact"
)

func (p *AgentProcess) handleSessionUpdate(params json.RawMessage) error {
	var raw wireSessionNotification
	if err := json.Unmarshal(params, &raw); err != nil {
		return fmt.Errorf("acp: decode session/update notification: %w", err)
	}
	var envelope wireSessionUpdateEnvelope
	if err := json.Unmarshal(raw.Update, &envelope); err != nil {
		return fmt.Errorf("acp: decode session/update envelope: %w", err)
	}
	switch p.routeSessionUpdate(raw.SessionID) {
	case sessionUpdateRouteFork:
		p.captureForkSessionUpdate(params, raw.SessionID, envelope.SessionUpdate)
		return nil
	case sessionUpdateRouteForeign:
		p.dropForeignSessionTraffic(raw.SessionID, envelope.SessionUpdate)
		return nil
	case sessionUpdateRouteBound:
	}
	if envelope.SessionUpdate == "user_message_chunk" {
		return nil
	}

	if envelope.SessionUpdate == "usage_update" {
		var update wireUsageUpdate
		if err := json.Unmarshal(raw.Update, &update); err != nil {
			return fmt.Errorf("acp: decode usage_update: %w", err)
		}
		usage := p.validatedUsage(tokenUsageFromUsageUpdate(p.activeTurnID(), update))
		if !usage.IsZero() {
			merged := p.mergePromptUsage(usage)
			p.emitPromptEvent(AgentEvent{
				Type:      EventTypeUsage,
				SessionID: string(raw.SessionID),
				TurnID:    merged.TurnID,
				Timestamp: usage.Timestamp,
				Usage:     &merged,
				Raw:       redact.ClaimTokensJSON(raw.Update),
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
		p.setConfigOptionCurrent(sessionConfigModeKey, string(notification.Update.CurrentModeUpdate.CurrentModeId))
	}
	event, err := translateSessionUpdate(notification, raw.Update, p.activeTurnID())
	if err != nil {
		return err
	}
	event = p.markToolEventPrechecked(event)
	p.emitPromptEvent(event)
	return nil
}

// captureForkSessionUpdate hands a clone-id notification to the open fork
// capture. It never touches the bound session's turn, stream, or config state.
func (p *AgentProcess) captureForkSessionUpdate(params json.RawMessage, id acpsdk.SessionId, kind string) {
	var notification acpsdk.SessionNotification
	if err := json.Unmarshal(params, &notification); err != nil {
		p.dropForeignSessionTraffic(id, kind)
		return
	}
	if !p.captureForkUpdate(notification) {
		p.dropForeignSessionTraffic(id, kind)
	}
}
