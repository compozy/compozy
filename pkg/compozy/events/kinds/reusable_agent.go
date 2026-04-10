package kinds

// ReusableAgentLifecycleStage identifies one reusable-agent lifecycle phase.
type ReusableAgentLifecycleStage string

const (
	// ReusableAgentLifecycleStageResolved marks successful agent resolution.
	ReusableAgentLifecycleStageResolved ReusableAgentLifecycleStage = "resolved"
	// ReusableAgentLifecycleStagePromptAssembled marks canonical system-prompt assembly.
	ReusableAgentLifecycleStagePromptAssembled ReusableAgentLifecycleStage = "prompt-assembled"
	// ReusableAgentLifecycleStageMCPMerged marks the resolved MCP server merge result.
	ReusableAgentLifecycleStageMCPMerged ReusableAgentLifecycleStage = "mcp-merged"
	// ReusableAgentLifecycleStageNestedStarted marks a nested `run_agent` invocation.
	ReusableAgentLifecycleStageNestedStarted ReusableAgentLifecycleStage = "nested-started"
	// ReusableAgentLifecycleStageNestedCompleted marks a completed nested `run_agent` invocation.
	ReusableAgentLifecycleStageNestedCompleted ReusableAgentLifecycleStage = "nested-completed"
	// ReusableAgentLifecycleStageNestedBlocked marks a blocked nested `run_agent` invocation.
	ReusableAgentLifecycleStageNestedBlocked ReusableAgentLifecycleStage = "nested-blocked"
)

// ReusableAgentBlockedReason classifies a blocked reusable-agent execution.
type ReusableAgentBlockedReason string

const (
	// ReusableAgentBlockedReasonDepthLimit indicates the nested depth ceiling was reached.
	ReusableAgentBlockedReasonDepthLimit ReusableAgentBlockedReason = "depth-limit"
	// ReusableAgentBlockedReasonCycleDetected indicates the nested agent path would recurse.
	ReusableAgentBlockedReasonCycleDetected ReusableAgentBlockedReason = "cycle-detected"
	// ReusableAgentBlockedReasonAccessDenied indicates the parent runtime denied the requested access.
	ReusableAgentBlockedReasonAccessDenied ReusableAgentBlockedReason = "access-denied"
	// ReusableAgentBlockedReasonInvalidAgent indicates the selected agent could not be resolved or validated.
	ReusableAgentBlockedReasonInvalidAgent ReusableAgentBlockedReason = "invalid-agent"
	// ReusableAgentBlockedReasonInvalidMCP indicates the resolved agent MCP configuration is invalid.
	ReusableAgentBlockedReasonInvalidMCP ReusableAgentBlockedReason = "invalid-mcp"
)

// ReusableAgentLifecyclePayload describes one reusable-agent lifecycle signal.
type ReusableAgentLifecyclePayload struct {
	Stage             ReusableAgentLifecycleStage `json:"stage"`
	AgentName         string                      `json:"agent_name,omitempty"`
	AgentSource       string                      `json:"agent_source,omitempty"`
	ParentAgentName   string                      `json:"parent_agent_name,omitempty"`
	AvailableAgents   int                         `json:"available_agents,omitempty"`
	SystemPromptBytes int                         `json:"system_prompt_bytes,omitempty"`
	MCPServers        []string                    `json:"mcp_servers,omitempty"`
	Resumed           bool                        `json:"resumed,omitempty"`
	ToolCallID        string                      `json:"tool_call_id,omitempty"`
	NestedDepth       int                         `json:"nested_depth"`
	MaxNestedDepth    int                         `json:"max_nested_depth"`
	OutputRunID       string                      `json:"output_run_id,omitempty"`
	Success           bool                        `json:"success"`
	Blocked           bool                        `json:"blocked"`
	BlockedReason     ReusableAgentBlockedReason  `json:"blocked_reason,omitempty"`
	Error             string                      `json:"error,omitempty"`
}
