package main

import (
	"context"
	"fmt"
	"os"

	aghsdk "github.com/compozy/agh/sdk/go"
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
	extension := aghsdk.NewExtension(aghsdk.ExtensionDefinition{
		Name:    "__EXTENSION_NAME__",
		Version: "0.1.0",
		Capabilities: aghsdk.CapabilitiesConfig{
			Provides: []string{"tool.provider"},
		},
	})

	if err := aghsdk.Tool[SearchInput](
		extension,
		"search",
		aghsdk.ToolOptions{
			ReadOnly:    true,
			InputSchema: searchInputSchema,
		},
		func(_ context.Context, req aghsdk.ToolRequest[SearchInput]) (aghsdk.ToolResult, error) {
			return aghsdk.TextResult("No results for " + req.Input.Query), nil
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
