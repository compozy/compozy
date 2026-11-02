package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

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
		result = append(result, nativeDescriptor(
			operation.id,
			"agent_instance_"+operation.name,
			"Agent Instance "+operation.name,
			"Manage the caller's stays-on agent instance: "+operation.name,
			agentInstanceInputSchema,
			risk,
			operation.read,
			operation.name == descriptorKeywordArchive,
			false,
			[]toolspkg.ToolsetID{
				toolspkg.ToolsetIDAuthoredContext,
			},
			[]string{descriptorKeywordAgents, "instances"},
			[]string{"stays-on instance", operation.name},
		))
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

const agentInstanceInputSchema = `{"type":"object","properties":{
"enabled":{"type":"boolean"},"dry_run":{"type":"boolean"},"occurrence_id":{"type":"string"},
"agent":{"type":"string"},"workspace":{"type":"string"},"id":{"type":"string"},
"all_workspaces":{"type":"boolean"},"state":{"type":"string"},"cursor":{"type":"string"},
"days":{"type":"integer","minimum":1,"maximum":365},
"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`
