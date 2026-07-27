package main

import (
	"context"
	"fmt"
	"os"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

type SearchInput struct {
	Query string `json:"query"`
}

var searchInputSchema = map[string]any{
	"type":     "object",
	"required": []string{"query"},
	"properties": map[string]any{
		"query": map[string]any{"type": "string"},
	},
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:    "__EXTENSION_NAME__",
		Version: "0.1.0",
		Capabilities: compozysdk.CapabilitiesConfig{
			Provides: []string{"tool.provider"},
		},
	})

	if err := compozysdk.Tool[SearchInput](
		extension,
		"search",
		compozysdk.ToolOptions{
			ReadOnly:    true,
			InputSchema: searchInputSchema,
		},
		func(_ context.Context, req compozysdk.ToolRequest[SearchInput]) (compozysdk.ToolResult, error) {
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
