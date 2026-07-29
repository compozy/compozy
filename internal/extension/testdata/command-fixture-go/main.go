package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

var invocationSequence atomic.Uint64

type greetInput struct {
	Name string `json:"name"`
}

type reviewFetchInput struct {
	Round int `json:"round"`
}

type approvedInput struct {
	Message string `json:"message"`
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:        "cmd-fixture",
		Version:     "0.1.0",
		Description: "Contributed command E2E fixture",
		Subprocess:  compozysdk.DescribeSubprocess{Command: "./bin"},
	})
	mustRegisterGreet(extension)
	mustRegisterReviewFetch(extension)
	mustRegisterApproved(extension)
	if err := extension.CommandGroup("review", "Review fixture rounds"); err != nil {
		fatal("register review command group", err)
	}
	if err := extension.Run(context.Background()); err != nil {
		fatal("run extension", err)
	}
}

func mustRegisterGreet(extension *compozysdk.Extension) {
	err := compozysdk.Tool[greetInput](
		extension,
		"greet",
		compozysdk.ToolOptions{
			Description: "Greet one person",
			ReadOnly:    true,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []string{"name"},
			},
			Command: &compozysdk.ExtensionCommandSpec{
				Verb: "greet", Summary: "Greet one person", Example: "--name Ada",
				Flags: map[string]string{"name": "name"},
			},
		},
		func(_ context.Context, request compozysdk.ToolRequest[greetInput]) (compozysdk.ToolResult, error) {
			fmt.Fprintln(os.Stderr, "invoke:greet")
			return compozysdk.TextResult(fmt.Sprintf(
				"hello %s #%d",
				request.Input.Name,
				invocationSequence.Add(1),
			)), nil
		},
	)
	if err != nil {
		fatal("register greet command", err)
	}
}

func mustRegisterReviewFetch(extension *compozysdk.Extension) {
	err := compozysdk.Tool[reviewFetchInput](
		extension,
		"review_fetch",
		compozysdk.ToolOptions{
			Description: "Fetch one review round",
			ReadOnly:    true,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"round": map[string]any{"type": "integer", "minimum": 1},
				},
				"required": []string{"round"},
			},
			Command: &compozysdk.ExtensionCommandSpec{
				Verb: "review/fetch", Summary: "Fetch one review round", Example: "--round 2",
				Flags: map[string]string{"round": "round"},
			},
		},
		func(
			_ context.Context,
			request compozysdk.ToolRequest[reviewFetchInput],
		) (compozysdk.ToolResult, error) {
			fmt.Fprintln(os.Stderr, "invoke:review/fetch")
			return compozysdk.TextResult(fmt.Sprintf(
				"review round %d #%d",
				request.Input.Round,
				invocationSequence.Add(1),
			)), nil
		},
	)
	if err != nil {
		fatal("register review fetch command", err)
	}
}

func mustRegisterApproved(extension *compozysdk.Extension) {
	err := compozysdk.Tool[approvedInput](
		extension,
		"approved",
		compozysdk.ToolOptions{
			Description:         "Run an approval-gated fixture command",
			ReadOnly:            true,
			RequiresInteraction: true,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]any{"type": "string"},
				},
				"required": []string{"message"},
			},
			Command: &compozysdk.ExtensionCommandSpec{
				Verb: "approved", Summary: "Run with explicit approval", Example: "--message ship",
				Flags: map[string]string{"message": "message"},
			},
		},
		func(_ context.Context, request compozysdk.ToolRequest[approvedInput]) (compozysdk.ToolResult, error) {
			fmt.Fprintln(os.Stderr, "invoke:approved")
			return compozysdk.TextResult(fmt.Sprintf(
				"approved %s #%d",
				request.Input.Message,
				invocationSequence.Add(1),
			)), nil
		},
	)
	if err != nil {
		fatal("register approval command", err)
	}
}

func fatal(operation string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", operation, err)
	os.Exit(1)
}
