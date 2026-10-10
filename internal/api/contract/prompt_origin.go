package contract

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
