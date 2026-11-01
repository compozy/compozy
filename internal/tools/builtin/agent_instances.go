package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func agentInstanceDescriptors() []toolspkg.Descriptor {
	var result []toolspkg.Descriptor
	for _, operation := range []struct {
		id   toolspkg.ToolID
		name string
		read bool
	}{
		{toolspkg.ToolIDAgentInstanceCreate, "create", false},
		{toolspkg.ToolIDAgentInstanceList, "list", true},
		{toolspkg.ToolIDAgentInstancePause, "pause", false},
		{toolspkg.ToolIDAgentInstanceResume, "resume", false},
		{toolspkg.ToolIDAgentInstanceArchive, "archive", false},
		{toolspkg.ToolIDAgentInstanceEnsureMain, "ensure_main", false},
		{toolspkg.ToolIDAgentInstanceReports, "reports", true},
	} {
		risk := toolspkg.RiskMutating
		if operation.read {
			risk = toolspkg.RiskRead
		}
		result = append(result, nativeDescriptor(operation.id, "agent_instance_"+operation.name,
			"Agent Instance "+operation.name, "Manage the caller's stays-on agent instance: "+operation.name,
			agentInstanceInputSchema, risk, operation.read, operation.name == "archive", false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext}, []string{"agents", "instances"}, []string{"stays-on instance", operation.name}))
	}
	return result
}

const agentInstanceInputSchema = `{"type":"object","properties":{
"agent":{"type":"string"},"workspace":{"type":"string"},"id":{"type":"string"},
"all_workspaces":{"type":"boolean"},"state":{"type":"string"},"cursor":{"type":"string"},
"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`
