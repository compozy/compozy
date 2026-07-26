package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

func networkUsageDescriptor() toolspkg.Descriptor {
	return nativeDescriptor(
		toolspkg.ToolIDNetworkUsage,
		"network_usage",
		"Network Usage",
		"Read one bounded page of workspace-scoped Network wake usage and full-filter totals.",
		networkUsageInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCoordination},
		[]string{networkNetworkKey, "usage"},
		[]string{"network usage", "wake budget"},
	)
}

const networkUsageInputSchema = `{
	"type":"object",
	"required":["workspace_id"],
	"properties":{
		"workspace_id":{"type":"string"},
		"owner_kind":{"type":"string","enum":["session","task_run","loop_run","automation_run"]},
		"owner_id":{"type":"string"},
		"run_id":{"type":"string"},
		"channel":{"type":"string"},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":200}
	},
	"additionalProperties":false
}`
