package extensionpkg

import "github.com/compozy/compozy/internal/mcppolicy"

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
		scan(server.Auth.ClientSecretInput)
	}
	return found
}
