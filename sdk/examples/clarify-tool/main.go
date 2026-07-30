package main

import (
	"context"
	"fmt"
	"os"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

type askInput struct {
	Question string   `json:"question"`
	Choices  []string `json:"choices,omitempty"`
}

const schemaTypeKey = "type"

var askInputSchema = map[string]any{
	schemaTypeKey: "object",
	"required":    []string{"question"},
	"properties": map[string]any{
		"question": map[string]any{schemaTypeKey: "string", "minLength": 1},
		"choices": map[string]any{
			schemaTypeKey: "array",
			"maxItems":    4,
			"items":       map[string]any{schemaTypeKey: "string", "minLength": 1},
		},
	},
	"additionalProperties": false,
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:        "clarify-tool",
		Version:     "0.1.0",
		Description: "Ask the operator one bounded clarification question",
		Subprocess:  compozysdk.DescribeSubprocess{Command: "./bin"},
		Permissions: compozysdk.PermissionsConfig{
			Requires: []compozysdk.HostAPIMethod{compozysdk.HostAPIMethodClarifyAsk},
		},
	})

	if err := compozysdk.Tool[askInput](
		extension,
		"ask",
		compozysdk.ToolOptions{
			Description: "Ask the operator one bounded clarification question",
			ReadOnly:    true,
			InputSchema: askInputSchema,
		},
		ask,
	); err != nil {
		fmt.Fprintf(os.Stderr, "register clarify tool: %v\n", err)
		os.Exit(1)
	}

	if err := extension.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "run clarify tool: %v\n", err)
		os.Exit(1)
	}
}

func ask(ctx context.Context, request compozysdk.ToolRequest[askInput]) (compozysdk.ToolResult, error) {
	answer, err := request.AskClarification(ctx, compozysdk.ClarifyQuestion{
		Question: request.Input.Question,
		Choices:  request.Input.Choices,
	})
	if err != nil {
		return compozysdk.ToolResult{}, fmt.Errorf("ask operator: %w", err)
	}
	result, err := compozysdk.StructuredResult(answer)
	if err != nil {
		return compozysdk.ToolResult{}, fmt.Errorf("encode clarification answer: %w", err)
	}
	return result, nil
}
