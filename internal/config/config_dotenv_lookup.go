package config

import (
	"fmt"

	"strings"
)

func loadDotEnvLookup(workspaceRoot string) (envLookup, error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return nil, nil
	}

	path := WorkspaceDotEnvFile(workspaceRoot)
	_, data, exists, err := readDotEnvFile(path)
	if err != nil {
		return nil, fmt.Errorf("load .env file %q: %w", path, err)
	}
	if !exists {
		return nil, nil
	}

	parsed := parseDotEnvDocument(string(data))
	if parsed.unsupported {
		return nil, fmt.Errorf("load .env file %q: %w", path, dotEnvUnsupportedError(path, parsed.diagnostics))
	}
	if len(parsed.values) == 0 {
		return nil, nil
	}

	return func(key string) (string, bool) {
		value, ok := parsed.values[key]
		return value, ok
	}, nil
}
