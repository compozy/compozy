package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	authoredContextHeartbeatKey = "heartbeat"
)

const (
	authoredContextHealthKey = "health"
	authoredContextStatusKey = "status"
)

var authoredContextTools = []toolspkg.Descriptor{
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
		[]string{"sessions", authoredContextHealthKey, authoredContextHeartbeatKey},
		[]string{"session health", "wake eligibility", "session liveness"},
	),
	nativeDescriptor(
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
		[]string{"agents", authoredContextHeartbeatKey, authoredContextStatusKey, "wake"},
		[]string{"heartbeat status", "wake audit", "agent heartbeat policy"},
	),
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
		[]string{"agents", authoredContextHeartbeatKey, "wake"},
		[]string{"heartbeat wake", "manual wake", "advisory wake"},
	),
}

func authoredContextDescriptors() []toolspkg.Descriptor {
	return authoredContextTools
}

const agentHeartbeatStatusInputSchema = `{
	"type":"object",
	"required":["workspace_id","agent_name"],
	"properties":{
		"workspace_id":{"type":"string"},
		"agent_name":{"type":"string"},
		"session_id":{"type":"string"},
		"include_session_health":{"type":"boolean"},
		"include_recent_wake_events":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const agentHeartbeatWakeInputSchema = `{
	"type":"object",
	"required":["workspace_id","agent_name","session_id"],
	"properties":{
		"workspace_id":{"type":"string"},
		"agent_name":{"type":"string"},
		"session_id":{"type":"string"},
		"source":{"type":"string"},
		"dry_run":{"type":"boolean"}
	},
	"additionalProperties":false
}`
