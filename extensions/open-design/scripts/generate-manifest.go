//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	opendesign "github.com/compozy/compozy/extensions/open-design"
)

func main() {
	check := flag.Bool("check", false, "Check the manifest without writing it")
	flag.Parse()
	if err := generate(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(check bool) error {
	definition, err := opendesign.Describe()
	if err != nil {
		return err
	}
	tools := make(map[string]any, len(definition.Tools))
	for _, tool := range definition.Tools {
		tools[tool.Handler] = map[string]any{
			"display_title": "Lint design artifact",
			"description":   tool.Description, "handler": tool.Handler, "profile": tool.Profile,
			"read_only": tool.ReadOnly, "risk": tool.Risk, "concurrency_safe": tool.ReadOnly, "visibility": "model",
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
	if check {
		current, err := os.ReadFile("extension.json")
		if err != nil {
			return err
		}
		var expectedJSON, currentJSON bytes.Buffer
		if err := json.Compact(&expectedJSON, encoded); err != nil {
			return err
		}
		if err := json.Compact(&currentJSON, current); err != nil {
			return err
		}
		if !bytes.Equal(expectedJSON.Bytes(), currentJSON.Bytes()) {
			return fmt.Errorf("open-design: extension.json is stale; run go generate ./extensions/open-design")
		}
		return nil
	}
	return os.WriteFile("extension.json", append(encoded, '\n'), 0o644)
}
