package daemon

import "encoding/json"

type nativeCallTask struct {
	Agent           string              `json:"agent"`
	SessionID       string              `json:"session_id"`
	Prompt          string              `json:"prompt"`
	Expect          json.RawMessage     `json:"expect"`
	IdleTTLSeconds  int64               `json:"idle_ttl_seconds"`
	DeadlineSeconds json.RawMessage     `json:"deadline_seconds"`
	Strict          bool                `json:"strict"`
	ResultBudget    string              `json:"result_budget"`
	ResultOverflow  string              `json:"result_overflow"`
	IdempotencyKey  string              `json:"idempotency_key"`
	Runtime         *nativeCallRuntime  `json:"runtime"`
	Narrow          nativeCallNarrowing `json:"narrow"`
}

type nativeCallInput struct {
	nativeCallTask
	Tasks *[]nativeCallTask `json:"tasks"`
}

type nativeCallRuntime struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Speed           string `json:"speed"`
}

type nativeCallNarrowing struct {
	Tools           []string `json:"tools"`
	Skills          []string `json:"skills"`
	MCPServers      []string `json:"mcp_servers"`
	WorkspacePaths  []string `json:"workspace_paths"`
	NetworkChannels []string `json:"network_channels"`
	SandboxProfiles []string `json:"sandbox_profiles"`
}

type nativeCallReturnInput struct {
	CallID    string          `json:"call_id"`
	Result    json.RawMessage `json:"result"`
	FinalText string          `json:"final_text"`
}

type nativeCallAwaitInput struct {
	CallIDs   []string `json:"call_ids"`
	TimeoutMS int64    `json:"timeout_ms"`
	Resume    string   `json:"resume"`
}

type nativeCallCancelInput struct {
	CallID string `json:"call_id"`
	Reason string `json:"reason"`
}

type nativeCallIDInput struct {
	CallID string `json:"call_id"`
}

type nativeCallPublishInput struct {
	CallID   string `json:"call_id"`
	Channel  string `json:"channel"`
	ThreadID string `json:"thread_id"`
}

type nativeAgentMessageInput struct {
	To     string `json:"to"`
	Text   string `json:"text"`
	CallID string `json:"call_id"`
}
