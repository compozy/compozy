package extensionpkg

import "github.com/compozy/compozy/internal/mcppolicy"

// AgentPluginHeaderSecretInputs maps optional header credential names to their typed input IDs.
func (m *Manifest) AgentPluginHeaderSecretInputs() map[string]string {
	if m == nil || m.Format != FormatAgentPlugin {
		return nil
	}
	names := make(map[string]struct{})
	for _, server := range m.Resources.MCPServers {
		for name := range mcppolicy.OptionalHeaderSecretNames(server.Headers) {
			names[name] = struct{}{}
		}
	}
	inputs := make(map[string]string)
	for _, input := range m.Inputs {
		if _, found := names[input.Binding.Name]; found && input.Binding.Type == manifestInputBindingTemplate &&
			input.Type == manifestInputTypeSecret && !input.Required {
			inputs[input.Binding.Name] = input.ID
		}
	}
	return inputs
}

func manifestServerReferencesInput(server MCPServerConfig, name string) bool {
	found := false
	scan := func(value string) {
		mcppolicy.ReplaceInputs(value, func(key, _ string, _ bool) string { found = found || key == name; return "" })
	}
	scan(server.URL)
	for _, values := range []map[string]string{server.Env, server.Headers} {
		for _, value := range values {
			scan(value)
		}
	}
	for _, value := range server.Args {
		scan(value)
	}
	if server.Auth != nil {
		scan(server.Auth.ClientID)
		scan(server.Auth.ClientSecretInput)
	}
	return found
}
