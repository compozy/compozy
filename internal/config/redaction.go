package config

const redactedConfigValue = "[redacted]"

// RedactedValue is the placeholder used when a public surface needs to reveal
// that a secret-bearing value exists without exposing the value itself.
func RedactedValue() string {
	return redactedConfigValue
}

// RedactStringMap returns the same keys with all values replaced by the shared
// redaction placeholder.
func RedactStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	redacted := make(map[string]string, len(values))
	for key := range values {
		redacted[key] = redactedConfigValue
	}
	return redacted
}

// RedactedMCPServer returns a server copy suitable for public API and CLI
// rendering. It preserves non-secret endpoint metadata and redacts env values.
func RedactedMCPServer(server MCPServer) MCPServer {
	redacted := cloneMCPServer(server)
	redacted.Env = RedactStringMap(server.Env)
	redacted.SecretEnv = RedactStringMap(server.SecretEnv)
	return redacted
}
