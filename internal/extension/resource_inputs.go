package extensionpkg

import (
	"net/url"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// Apply after template resolution: user input is literal data, never manifest syntax.
func applyManifestMCPInputs(
	inputs []ManifestInput,
	declaration MCPServerConfig,
	server *compozyconfig.MCPServer,
	state InputState,
	getenv func(string) string,
) error {
	for _, input := range inputs {
		matches, err := manifestInputMatchesServer(input, declaration)
		if err != nil {
			return &InputValidationError{InputID: input.ID, Reason: "invalid server binding"}
		}
		if !matches {
			continue
		}
		value, ref, present, err := effectiveInputValue(input, state, getenv)
		if err != nil {
			return err
		}
		if input.Binding.Type == "url_query" {
			if err := applyManifestURLInput(server, input, value, present); err != nil {
				return err
			}
			continue
		}
		name := input.Binding.Name
		delete(server.Env, name)
		delete(server.SecretEnv, name)
		if !present {
			continue
		}
		if input.Type == manifestInputTypeSecret {
			if server.SecretEnv == nil {
				server.SecretEnv = make(map[string]string)
			}
			server.SecretEnv[name] = ref
		} else {
			if server.Env == nil {
				server.Env = make(map[string]string)
			}
			server.Env[name] = value
		}
	}
	return nil
}

func applyManifestURLInput(
	server *compozyconfig.MCPServer,
	input ManifestInput,
	value string,
	present bool,
) error {
	parsed, err := url.Parse(server.URL)
	if err != nil {
		return &InputValidationError{InputID: input.ID, Reason: "invalid server URL"}
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return &InputValidationError{InputID: input.ID, Reason: "invalid server URL query"}
	}
	query.Del(input.Binding.Name)
	if present {
		query.Set(input.Binding.Name, value)
	}
	parsed.RawQuery = query.Encode()
	server.URL = parsed.String()
	return nil
}
