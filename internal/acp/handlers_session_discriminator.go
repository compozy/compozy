package acp

import "github.com/compozy/compozy/internal/redact"

func knownSessionUpdate(kind string) bool {
	switch kind {
	case "user_message_chunk", "agent_message_chunk", "agent_thought_chunk",
		"tool_call", "tool_call_update", "plan", "plan_update", "plan_removed",
		"available_commands_update", "current_mode_update", "config_option_update",
		"session_info_update", "usage_update":
		return true
	default:
		return false
	}
}

func (p *AgentProcess) emitUnknownSessionUpdate(raw wireSessionNotification, kind string) {
	p.compactionMu.Lock()
	if p.unknownUpdates == nil {
		p.unknownUpdates = make(map[string]struct{})
	}
	_, seen := p.unknownUpdates[kind]
	p.unknownUpdates[kind] = struct{}{}
	p.compactionMu.Unlock()
	if !seen && p.logger != nil {
		p.logger.Warn(
			"acp.session_update.unknown",
			"session_id",
			raw.SessionID,
			"session_update",
			redact.ClaimTokens(kind),
		)
	}
	p.emitRawSystemUpdate(raw, kind)
}

func (p *AgentProcess) emitRawSystemUpdate(raw wireSessionNotification, kind string) {
	p.emitPromptEvent(AgentEvent{
		Type: EventTypeSystem, SessionID: string(raw.SessionID), TurnID: p.activeTurnID(),
		Timestamp: timeNowUTC(), Title: redact.ClaimTokens(kind), Raw: redact.ClaimTokensJSON(raw.Update),
	})
}
