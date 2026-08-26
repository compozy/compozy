package daemon

type nativeSessionTargetInput struct {
	SessionID string `json:"session_id"`
}

type nativeSessionWaitInput struct {
	SessionID string   `json:"session_id"`
	Until     []string `json:"until"`
	TimeoutMS int64    `json:"timeout_ms"`
}

type nativeSessionSpawnInput struct {
	AgentName        string   `json:"agent_name"`
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	ReasoningEffort  string   `json:"reasoning_effort"`
	Speed            string   `json:"speed"`
	Name             string   `json:"name"`
	PromptOverlay    string   `json:"prompt_overlay"`
	SpawnRole        string   `json:"spawn_role"`
	TTLSeconds       int64    `json:"ttl_seconds"`
	AutoStopOnParent *bool    `json:"auto_stop_on_parent"`
	NotifyCreator    *bool    `json:"notify_creator"`
	Tools            []string `json:"tools"`
	Skills           []string `json:"skills"`
	MCPServers       []string `json:"mcp_servers"`
	WorkspacePaths   []string `json:"workspace_paths"`
	NetworkChannels  []string `json:"network_channels"`
	SandboxProfiles  []string `json:"sandbox_profiles"`
	IdempotencyKey   string   `json:"idempotency_key"`
}

type nativeSessionApproveInput struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
}

type nativeSessionClarifyAnswerInput struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Choice    *int   `json:"choice"`
	Text      string `json:"text"`
}
