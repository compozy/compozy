package spec

import (
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
)

func applyAgentIdentityContract(operations []OperationSpec) []OperationSpec {
	for index := range operations {
		if !strings.HasPrefix(operations[index].Path, "/api/agent/") {
			continue
		}
		operations[index].Parameters = append(agentIdentityHeaderParameters(), operations[index].Parameters...)
	}
	return operations
}

func agentIdentityHeaderParameters() []ParameterSpec {
	return []ParameterSpec{
		headerParam(agentidentity.HeaderSessionID, "Daemon-issued active session id"),
		headerParam(agentidentity.HeaderAgent, "Daemon-issued session agent name"),
	}
}
