package contract

// TaskRunConversationThreadID is the deterministic thread identifier used by Live task runs.
const TaskRunConversationThreadID = "thread_agent_channel"

// TaskRunConversationRefPayload identifies the deterministic conversation attached to a Live run.
type TaskRunConversationRefPayload struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	Surface     string `json:"surface"`
	ThreadID    string `json:"thread_id"`
	StreamURL   string `json:"stream_url"`
}

// TaskRunNetworkPayload carries the run-scoped conversation and truthful bound consumption.
type TaskRunNetworkPayload struct {
	Conversation TaskRunConversationRefPayload `json:"conversation"`
	Usage        NetworkUsageResponse          `json:"usage"`
}

// TaskRunConversationStreamPayload is one typed message-or-usage SSE frame.
type TaskRunConversationStreamPayload struct {
	Message *NetworkConversationMessagePayload `json:"message,omitempty"`
	Usage   *NetworkUsageResponse              `json:"usage,omitempty"`
}
