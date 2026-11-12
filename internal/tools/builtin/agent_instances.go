package builtin

import (
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func agentInstanceDescriptors() []toolspkg.Descriptor {
	var result []toolspkg.Descriptor
	for _, operation := range []struct {
		id   toolspkg.ToolID
		name string
		read bool
	}{
		{toolspkg.ToolIDAgentInstanceCheck, "check", false},
		{toolspkg.ToolIDAgentInstanceChecks, "checks", false},
		{toolspkg.ToolIDAgentInstanceOccurrence, "occurrence", true},
		{toolspkg.ToolIDAgentInstanceCreate, descriptorKeywordCreate, false},
		{toolspkg.ToolIDAgentInstanceList, "list", true},
		{toolspkg.ToolIDAgentInstancePause, descriptorKeywordPause, false},
		{toolspkg.ToolIDAgentInstanceResume, descriptorKeywordResume, false},
		{toolspkg.ToolIDAgentInstanceArchive, descriptorKeywordArchive, false},
		{toolspkg.ToolIDAgentInstanceEnsureMain, "ensure_main", false},
		{toolspkg.ToolIDAgentInstanceClearMain, agentInstanceClearMainOperation, false},
		{toolspkg.ToolIDAgentInstanceReports, "reports", true},
		{toolspkg.ToolIDAgentInstanceGet, "get", true},
		{toolspkg.ToolIDAgentInstanceAway, "away", true},
		{toolspkg.ToolIDAgentInstanceSeen, "mark_seen", false},
		{toolspkg.ToolIDAgentInstanceCheckIns, "check_ins", true},
	} {
		risk := toolspkg.RiskMutating
		if operation.read {
			risk = toolspkg.RiskRead
		}
		schema := agentInstanceInputSchema
		required := ""
		switch operation.id {
		case toolspkg.ToolIDAgentInstanceChecks:
			required = "enabled"
		case toolspkg.ToolIDAgentInstanceOccurrence:
			required = "occurrence_id"
		}
		if required != "" {
			schema = strings.TrimSuffix(schema, "}") + `,"required":["` + required + `"]}`
		}
		description := "Manage the caller's always-on agent instance: " + operation.name
		switch operation.name {
		case "list":
			description += ". Defaults to the caller agent and workspace; only authorized instances are returned. " +
				"Ordinary active_count spans all workspaces in selected profiles, ignoring filters and pagination; " +
				"max_active_instances is per profile. Use peers for reachable agents: both capacity fields are zero (unavailable)."
		case "ensure_main":
			description += ". Requires an existing instance; dry_run is not supported. Use agent_instance_get to inspect it."
		case agentInstanceClearMainOperation:
			description = "Clear an always-on agent instance's main session: discard its conversation and start a " +
				"fresh provider thread under the same session id. Queued inputs, agent memory and the instance " +
				"are kept. Operator only; refused while the main is running a turn; dry_run is not supported. " +
				"Use it when the main cannot resume, for example after a provider or account change."
		}
		result = append(result, withAgentInstanceResultViews(nativeDescriptor(
			operation.id,
			"agent_instance_"+operation.name,
			"Agent Instance "+operation.name,
			description,
			schema,
			risk,
			operation.read,
			operation.name == descriptorKeywordArchive || operation.name == agentInstanceClearMainOperation,
			false,
			[]toolspkg.ToolsetID{
				toolspkg.ToolsetIDAuthoredContext,
			},
			[]string{descriptorKeywordAgents, "instances"},
			[]string{"always-on instance", operation.name},
		)))
	}
	result = append(
		result,
		nativeDescriptor(
			toolspkg.ToolIDAgentActivity,
			"agent_activity",
			"Agent Activity",
			"Read agent activity in an authorized workspace",
			agentInstanceInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext},
			[]string{descriptorKeywordAgents, "activity"},
			[]string{"agent activity"},
		),
	)
	return result
}

// agentInstanceClearMainOperation names the operator-only main-session clear.
const agentInstanceClearMainOperation = "clear_main"

const agentInstanceInputSchema = `{"type":"object","properties":{
"enabled":{"type":"boolean"},"dry_run":{"type":"boolean"},"occurrence_id":{"type":"string"},
"agent":{"type":"string"},"workspace":{"type":"string"},"id":{"type":"string"},
"peers":{"type":"boolean"},"all_workspaces":{"type":"boolean"},"state":{"type":"string"},"cursor":{"type":"string"},
"days":{"type":"integer","minimum":1,"maximum":365},
"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`

func withAgentInstanceResultViews(descriptor toolspkg.Descriptor) toolspkg.Descriptor {
	switch descriptor.ID {
	case toolspkg.ToolIDAgentInstanceList:
		descriptor = withResultViews(
			descriptor,
			agentInstanceListCompactOutputSchema,
			string(descriptor.OutputSchema),
		)
	case toolspkg.ToolIDAgentInstanceChecks, toolspkg.ToolIDAgentInstanceCreate,
		toolspkg.ToolIDAgentInstancePause, toolspkg.ToolIDAgentInstanceResume,
		toolspkg.ToolIDAgentInstanceArchive, toolspkg.ToolIDAgentInstanceEnsureMain,
		toolspkg.ToolIDAgentInstanceClearMain, toolspkg.ToolIDAgentInstanceGet:
		descriptor = withResultViews(
			descriptor,
			agentInstanceRecordCompactOutputSchema,
			string(descriptor.OutputSchema),
		)
	}
	return descriptor
}
