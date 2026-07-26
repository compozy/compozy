package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	observeObserveKey = "observe"
)

const (
	listLogsKey      = "events"
	observeHealthKey = "health"
)

var observeTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDListLogs,
		"logs",
		"Logs",
		"Read redacted runtime logs through the logs query surface.",
		listLogsInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDObserve},
		[]string{"logs", listLogsKey},
		[]string{"runtime logs", "log events"},
	),
	nativeDescriptor(
		toolspkg.ToolIDObserveMetrics,
		"observe_metrics",
		"Observe Metrics",
		"Read daemon observability health and metrics through the current observe surface.",
		emptyInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDObserve},
		[]string{observeObserveKey, "metrics", observeHealthKey},
		[]string{"observe health", "runtime metrics"},
	),
	nativeDescriptor(
		toolspkg.ToolIDObserveSearch,
		"observe_search",
		"Observe Search",
		"Search redacted observability events using current observe filters.",
		observeSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDObserve},
		[]string{observeObserveKey, "search"},
		[]string{"observe search", "search runtime events"},
	),
}

func observeDescriptors() []toolspkg.Descriptor {
	return observeTools
}

const listLogsInputSchema = `{
	"type":"object",
	"required":["workspace_id"],
	"properties":{
		"workspace_id":{"type":"string"},
		"session_id":{"type":"string"},
		"agent_name":{"type":"string"},
		"type":{"type":"string"},
		"run":{"type":"string"},
		"actor_kind":{"type":"string"},
		"actor_id":{"type":"string"},
		"provider":{"type":"string"},
		"outcome":{"type":"string"},
		"component":{"type":"string"},
		"error_only":{"type":"boolean"},
		"after_seq":{"type":"integer"},
		"since":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const observeSearchInputSchema = `{
	"type":"object",
	"required":["workspace_id","query"],
	"properties":{
		"workspace_id":{"type":"string"},
		"query":{"type":"string"},
		"session_id":{"type":"string"},
		"agent_name":{"type":"string"},
		"type":{"type":"string"},
		"run":{"type":"string"},
		"actor_kind":{"type":"string"},
		"actor_id":{"type":"string"},
		"provider":{"type":"string"},
		"outcome":{"type":"string"},
		"component":{"type":"string"},
		"error_only":{"type":"boolean"},
		"after_seq":{"type":"integer"},
		"since":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`
