package core

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/transcript"
)

func promptOriginPayload(origin *acp.PromptOriginMeta) *contract.PromptOriginMeta {
	redacted := transcript.RedactAgentEvent((acp.AgentEvent{}).WithPromptOrigin(origin)).PromptOrigin()
	if redacted == nil {
		return nil
	}
	return &contract.PromptOriginMeta{
		Kind: redacted.Kind, SessionID: redacted.SessionID, WorkspaceID: redacted.WorkspaceID,
		AgentName: redacted.AgentName, TitleAtSend: redacted.TitleAtSend, Hop: redacted.Hop,
		NotifyOnComplete: redacted.NotifyOnComplete, ReplyWatchID: redacted.ReplyWatchID,
	}
}

func sessionEventPromptOrigin(content string) *contract.PromptOriginMeta {
	event, err := transcript.UnmarshalAgentEvent(content)
	if err != nil {
		return nil
	}
	return promptOriginPayload(event.PromptOrigin())
}

func storedPromptOriginPayload(raw json.RawMessage) *contract.PromptOriginMeta {
	if len(raw) == 0 {
		return nil
	}
	var origin acp.PromptOriginMeta
	if err := json.Unmarshal(raw, &origin); err != nil {
		return nil
	}
	return promptOriginPayload(&origin)
}
