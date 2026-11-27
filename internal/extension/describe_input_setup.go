package extensionpkg

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

const inputTargetSubprocess = "subprocess"

// InputSetupState classifies one declared input as set, missing, invalid or daemon_env (rule 5).
func InputSetupState(manifest *Manifest, input ManifestInput, state InputState, getenv func(string) string) string {
	_, _, present, err := manifestInputValue(manifest, input, state, getenv)
	switch {
	case err != nil:
		return contract.ExtensionInputStateInvalid
	case !present:
		return contract.ExtensionInputStateMissing
	case manifest.daemonEnvInput(input, state, getenv):
		return contract.ExtensionInputStateDaemonEnv
	default:
		return contract.ExtensionInputStateSet
	}
}

// DescribeInputSetup projects declared inputs, their receivers and state without values or Vault references.
// Instance identity and enablement are left for the caller.
func DescribeInputSetup(
	manifest *Manifest, state InputState, getenv func(string) string,
) contract.ExtensionInputsPayload {
	payload := contract.ExtensionInputsPayload{
		Inputs: []contract.ExtensionInputStatusPayload{}, Missing: []string{}, Invalid: []string{},
		DaemonEnv: []string{}, MissingEnv: []string{}, MCPAuthServers: []string{},
	}
	if manifest == nil {
		return payload
	}
	readiness := InputReadiness(manifest, state, getenv)
	for _, input := range manifest.Inputs {
		item := contract.ExtensionInputStatusPayload{
			ID: input.ID, Prompt: input.Prompt, Type: input.Type, Required: input.Required,
			Binding: contract.MarketplaceInputBindingPayload{Type: input.Binding.Type, Name: input.Binding.Name},
			Targets: manifest.inputTargets(input),
			State:   InputSetupState(manifest, input, state, getenv),
		}
		if input.Type != manifestInputTypeSecret {
			item.Default = slices.Clone(input.Default)
		}
		switch item.State {
		case contract.ExtensionInputStateInvalid:
			payload.Invalid = append(payload.Invalid, input.ID)
		case contract.ExtensionInputStateDaemonEnv:
			payload.DaemonEnv = append(payload.DaemonEnv, input.ID)
		case contract.ExtensionInputStateMissing:
			if input.Required {
				payload.Missing = append(payload.Missing, input.ID)
			}
		}
		payload.Inputs = append(payload.Inputs, item)
	}
	payload.MissingEnv = bareMissingEnv(manifest, readiness.MissingEnv)
	for _, name := range sortedMapKeys(manifest.Resources.MCPServers) {
		if auth := manifest.Resources.MCPServers[name].Auth; auth != nil && auth.Method == mcpAuthMethodOAuth {
			payload.MCPAuthServers = append(payload.MCPAuthServers, name)
		}
	}
	return payload
}

// inputTargets lists the receivers of one input: the subprocess and each referencing MCP server.
func (m *Manifest) inputTargets(input ManifestInput) []string {
	targets := []string{}
	if m.SubprocessInput(input) {
		targets = append(targets, inputTargetSubprocess)
	}
	for _, name := range sortedMapKeys(m.Resources.MCPServers) {
		if matches, err := manifestInputMatchesServer(input, m.Resources.MCPServers[name]); err == nil && matches {
			targets = append(targets, "mcp:"+name)
		}
	}
	return targets
}

// bareMissingEnv keeps the requires_env keys no declared input binds; those need `secrets set`.
func bareMissingEnv(manifest *Manifest, missing []string) []string {
	result := []string{}
	for _, name := range missing {
		bound := slices.ContainsFunc(manifest.Inputs, func(input ManifestInput) bool {
			return input.Binding.Type == manifestEnvKey && strings.TrimSpace(input.Binding.Name) == name
		})
		if !bound {
			result = append(result, name)
		}
	}
	return result
}
