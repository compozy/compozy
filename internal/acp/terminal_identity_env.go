package acp

import (
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	identityprotocol "github.com/compozy/compozy/internal/agentidentity/protocol"
)

const managedEnvCompozyHome = "COMPOZY_HOME"

func managedTerminalEnvFromStartEnv(env []string) []acpsdk.EnvVariable {
	variables := make([]acpsdk.EnvVariable, 0, 6)
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if !isManagedTerminalEnvName(name) {
			continue
		}
		variables = upsertTerminalEnv(variables, acpsdk.EnvVariable{Name: name, Value: value})
	}
	return variables
}

func isManagedTerminalEnvName(name string) bool {
	switch name {
	case identityprotocol.EnvAgent,
		"COMPOZY_AGENT_NAME",
		identityprotocol.EnvTransportSocket,
		managedEnvCompozyHome,
		"COMPOZY_PEER_ID",
		"COMPOZY_SESSION_CHANNEL",
		identityprotocol.EnvSessionID:
		return true
	default:
		return false
	}
}

func mergeManagedTerminalEnv(
	request []acpsdk.EnvVariable,
	managed []acpsdk.EnvVariable,
) []acpsdk.EnvVariable {
	if len(request) == 0 && len(managed) == 0 {
		return nil
	}
	merged := make([]acpsdk.EnvVariable, 0, len(request)+len(managed))
	for _, variable := range request {
		merged = upsertTerminalEnv(merged, variable)
	}
	for _, variable := range managed {
		merged = upsertTerminalEnv(merged, variable)
	}
	return merged
}

func upsertTerminalEnv(env []acpsdk.EnvVariable, variable acpsdk.EnvVariable) []acpsdk.EnvVariable {
	name := strings.ToUpper(strings.TrimSpace(variable.Name))
	for i := range env {
		if strings.ToUpper(strings.TrimSpace(env[i].Name)) == name {
			env[i] = variable
			return env
		}
	}
	return append(env, variable)
}
