package main

import (
	"context"
	"fmt"
	"os"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

type searchInput struct {
	Query string `json:"query"`
}

var searchInputSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"query"},
	"properties": map[string]any{
		"query": map[string]any{"type": "string"},
	},
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:        "__EXTENSION_NAME__",
		Version:     "0.1.0",
		Description: "Search extension-owned data",
		Subprocess:  compozysdk.DescribeSubprocess{Command: "./bin"},
		Permissions: compozysdk.PermissionsConfig{},
	})
	if err := compozysdk.Tool[searchInput](
		extension,
		"search",
		compozysdk.ToolOptions{
			Description: "Search extension-owned data",
			ReadOnly:    true,
			InputSchema: searchInputSchema,
		},
		func(_ context.Context, req compozysdk.ToolRequest[searchInput]) (compozysdk.ToolResult, error) {
			return compozysdk.TextResult("No results for " + req.Input.Query), nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "register tool: %v\n", err)
		os.Exit(1)
	}
	if err := extension.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "run extension: %v\n", err)
		os.Exit(1)
	}
}
