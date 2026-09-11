//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"

	opendesign "github.com/compozy/compozy/extensions/open-design"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	definition, err := opendesign.Describe()
	if err != nil {
		return err
	}
	tools := make(map[string]any, len(definition.Tools))
	for _, tool := range definition.Tools {
		tools[tool.Handler] = map[string]any{
			"description": tool.Description, "handler": tool.Handler, "profile": tool.Profile,
			"read_only": tool.ReadOnly, "risk": tool.Risk, "concurrency_safe": true, "visibility": "model",
			"input_schema": tool.InputSchema, "output_schema": tool.OutputSchema,
			"backend": map[string]any{"kind": "extension_host", "handler": tool.Handler},
		}
	}
	manifest := map[string]any{
		"extension": map[string]any{
			"name":                definition.Name,
			"version":             definition.Version,
			"description":         definition.Description,
			"min_compozy_version": definition.SDK.MinCompozyVersion,
		},
		"profiles":     definition.Profiles,
		"capabilities": map[string]any{"provides": definition.Provides},
		"permissions":  map[string]any{"requires": definition.Permissions},
		"resources": map[string]any{
			"skills": definition.Resources.Skills,
			"agents": definition.Resources.Agents,
			"loops":  definition.Resources.Loops,
			"tools":  tools,
		},
		"subprocess": definition.Subprocess,
	}
	encoded, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile("extension.json", append(encoded, '\n'), 0o644)
}
