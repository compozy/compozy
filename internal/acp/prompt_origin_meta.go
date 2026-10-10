package acp

import "strings"

const PromptOriginKindSession = "session"
const MaxSessionMessageHops = 8

type PromptOriginMeta struct {
	Kind             string `json:"kind"`
	SessionID        string `json:"session_id"`
	WorkspaceID      string `json:"workspace_id"`
	AgentName        string `json:"agent_name,omitempty"`
	TitleAtSend      string `json:"title_at_send,omitempty"`
	Hop              int    `json:"hop"`
	NotifyOnComplete bool   `json:"notify_on_complete,omitempty"`
	ReplyWatchID     string `json:"reply_watch_id,omitempty"`
}

func (m PromptOriginMeta) Normalize() PromptOriginMeta {
	m.Kind = strings.TrimSpace(m.Kind)
	m.SessionID = strings.TrimSpace(m.SessionID)
	m.WorkspaceID = strings.TrimSpace(m.WorkspaceID)
	m.AgentName = strings.TrimSpace(m.AgentName)
	m.TitleAtSend = strings.Join(strings.Fields(m.TitleAtSend), " ")
	title := []rune(m.TitleAtSend)
	if len(title) > 200 {
		m.TitleAtSend = string(title[:200])
	}
	m.ReplyWatchID = strings.TrimSpace(m.ReplyWatchID)
	return m
}

func (m PromptOriginMeta) IsZero() bool { return m.Normalize() == (PromptOriginMeta{}) }

func (m PromptOriginMeta) Validate() error {
	m = m.Normalize()
	if m.Kind != PromptOriginKindSession || m.SessionID == "" || m.WorkspaceID == "" {
		return invalidPromptMetadata("acp: session prompt origin identity is incomplete")
	}
	if m.Hop < 1 || m.Hop > MaxSessionMessageHops {
		return invalidPromptMetadata("acp: session prompt origin hop must be between 1 and 8")
	}
	return nil
}

func ClonePromptOriginMeta(m *PromptOriginMeta) *PromptOriginMeta {
	if m == nil {
		return nil
	}
	cloned := m.Normalize()
	if cloned.IsZero() {
		return nil
	}
	return &cloned
}
