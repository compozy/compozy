package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const (
	authoredContextHeartbeatKey = "heartbeat"
)

const (
	authoredContextHealthKey = "health"
	authoredContextStatusKey = "status"
)

var authoredContextTools = []toolspkg.Descriptor{
	withResultViews(nativeDescriptor(
		toolspkg.ToolIDAgentHeartbeatSources,
		"agent_heartbeat_sources",
		"Agent Heartbeat Sources",
		"List available check-in source kinds, scripts, approvals and limits.",
		`{"type":"object","required":["agent"],"properties":{"agent":{"type":"string"},"workspace":{"type":"string"}},"additionalProperties":false}`,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext},
		[]string{descriptorKeywordAgents, authoredContextHeartbeatKey, "sources"},
		[]string{"check-in sources"},
	), agentHeartbeatSourcesCompactOutputSchema, `{"type":"object"}`),
	nativeDescriptor(
		toolspkg.ToolIDSessionHealth,
		"session_health",
		"Session Health",
		"Read one metadata-only session health row and wake eligibility through the managed session health service.",
		sessionIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext, toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, authoredContextHealthKey, authoredContextHeartbeatKey},
		[]string{"session health", "wake eligibility", "session liveness"},
	),
	withResultViews(nativeDescriptor(
		toolspkg.ToolIDAgentHeartbeatStatus,
		"agent_heartbeat_status",
		"Agent Heartbeat Status",
		"Read resolved Heartbeat policy, wake state, optional session health, "+
			"and retained wake audit through managed services.",
		agentHeartbeatStatusInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext},
		[]string{descriptorKeywordAgents, authoredContextHeartbeatKey, authoredContextStatusKey, wakeKeyword},
		[]string{"heartbeat status", "wake audit", "agent heartbeat policy"},
	), agentHeartbeatStatusCompactOutputSchema, `{"type":"object"}`),
	nativeDescriptor(
		toolspkg.ToolIDAgentHeartbeatWake,
		"agent_heartbeat_wake",
		"Agent Heartbeat Wake",
		"Request one managed advisory Heartbeat wake decision without creating task runs or bypassing task claim authority.",
		agentHeartbeatWakeInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext},
		[]string{descriptorKeywordAgents, authoredContextHeartbeatKey, wakeKeyword},
		[]string{"heartbeat wake", "manual wake", "advisory wake"},
	),
}

func authoredContextDescriptors() []toolspkg.Descriptor {
	return authoredContextTools
}

const agentHeartbeatStatusInputSchema = `{
	"type":"object",
	"required":["agent_name"],
	"properties":{
		"workspace":{"type":"string"},
		"agent_name":{"type":"string"},
		"session_id":{"type":"string"},
		"include_session_health":{"type":"boolean"},
		"include_recent_wake_events":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const agentHeartbeatWakeInputSchema = `{
	"type":"object",
	"required":["agent_name","session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"agent_name":{"type":"string"},
		"session_id":{"type":"string"},
		"source":{"type":"string"},
		"dry_run":{"type":"boolean"}
	},
	"additionalProperties":false
}`
