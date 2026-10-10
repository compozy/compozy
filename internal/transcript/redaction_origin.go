package transcript

import "github.com/compozy/compozy/internal/acp"

func redactPromptOrigin(meta *acp.PromptOriginMeta) *acp.PromptOriginMeta {
	m := acp.ClonePromptOriginMeta(meta)
	if m == nil {
		return nil
	}
	m.SessionID = redactStructuralString(m.SessionID)
	m.WorkspaceID = redactStructuralString(m.WorkspaceID)
	m.AgentName = redactDisplayString(m.AgentName)
	m.TitleAtSend = redactDisplayString(m.TitleAtSend)
	m.ReplyWatchID = redactStructuralString(m.ReplyWatchID)
	return m
}
