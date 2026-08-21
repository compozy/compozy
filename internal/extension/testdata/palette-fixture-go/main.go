package main

import (
	"context"
	"fmt"
	"os"

	compozysdk "github.com/compozy/compozy/sdk/go"
	"github.com/compozy/compozy/sdk/go/contracts"
)

type captureNoteInput struct {
	Title string `json:"title"`
	Tag   string `json:"tag,omitempty"`
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name: "notes", Version: "0.1.0", Description: "Command palette integration fixture",
		Resources:  compozysdk.DescribeResources{CmdPalette: notesPalette()},
		Subprocess: compozysdk.DescribeSubprocess{Command: "./bin"},
	})
	if err := registerNotesTools(extension); err != nil {
		fatal("register tools", err)
	}
	if err := extension.Run(context.Background()); err != nil {
		fatal("run extension", err)
	}
}

func notesPalette() contracts.CmdPaletteConfig {
	return contracts.CmdPaletteConfig{
		Commands: []contracts.CmdPaletteCommand{
			{
				ID: "capture", Title: "Capture note", Section: "Notes", Icon: "pencil",
				Keywords: []string{"jot", "memo", "quick"},
				Arguments: []contracts.CmdPaletteArgument{
					{Name: "title", Type: "text", Placeholder: "Note title", Required: true},
					{Name: "tag", Type: "dropdown", Options: []string{"inbox", "idea"}},
				},
				Action:          contracts.CmdPaletteAction{Kind: "tool", Tool: "capture_note"},
				DefaultShortcut: "alt+shift+KeyN",
			},
			{
				ID: "recent", Title: "Recent notes", Section: "Notes", Icon: "clock",
				Action: contracts.CmdPaletteAction{Kind: "view", View: "recent"},
			},
			{
				ID: "purge", Title: "Purge archived notes", Section: "Notes", Icon: "trash",
				Action:      contracts.CmdPaletteAction{Kind: "tool", Tool: "purge_archived"},
				Destructive: true,
				Confirmation: &contracts.CmdPaletteConfirmation{
					Title:   "Purge archived notes?",
					Body:    "Permanently deletes every archived note in this workspace.",
					Confirm: "Purge",
				},
			},
		},
		Views: []contracts.CmdPaletteView{{
			ID: "recent", Title: "Recent notes", Kind: "list",
			Source: &contracts.CmdPaletteViewSource{Tool: "list_recent"},
		}},
	}
}

func registerNotesTools(extension *compozysdk.Extension) error {
	if err := compozysdk.Tool[captureNoteInput](
		extension,
		"capture_note",
		compozysdk.ToolOptions{
			Description: "Capture a note", Risk: compozysdk.RiskMutating,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{"type": "string"},
					"tag":   map[string]any{"type": "string", "enum": []string{"inbox", "idea"}},
				},
				"required": []string{"title"},
			},
		},
		func(_ context.Context, request compozysdk.ToolRequest[captureNoteInput]) (compozysdk.ToolResult, error) {
			return compozysdk.StructuredResult(map[string]any{
				"id": "note-fixture", "title": request.Input.Title, "tag": request.Input.Tag,
			})
		},
	); err != nil {
		return err
	}
	if err := compozysdk.Tool[struct{}](
		extension,
		"list_recent",
		compozysdk.ToolOptions{
			Description: "List recent notes", ReadOnly: true, Risk: compozysdk.RiskRead,
			InputSchema: map[string]any{"type": "object"},
		},
		func(context.Context, compozysdk.ToolRequest[struct{}]) (compozysdk.ToolResult, error) {
			return compozysdk.StructuredResult(map[string]any{
				"view": "v1",
				"sections": []map[string]any{{
					"title": "Recent",
					"rows": []map[string]any{{
						"id": "note-1", "title": "Standup follow-ups", "icon": "file-text",
					}},
				}},
			})
		},
	); err != nil {
		return err
	}
	return compozysdk.Tool[struct{}](
		extension,
		"purge_archived",
		compozysdk.ToolOptions{
			Description: "Purge archived notes", Risk: compozysdk.RiskDestructive,
			RequiresInteraction: true, InputSchema: map[string]any{"type": "object"},
		},
		func(context.Context, compozysdk.ToolRequest[struct{}]) (compozysdk.ToolResult, error) {
			return compozysdk.StructuredResult(map[string]any{"purged": 2})
		},
	)
}

func fatal(operation string, err error) {
	if _, writeErr := fmt.Fprintf(os.Stderr, "%s: %v\n", operation, err); writeErr != nil {
		os.Exit(2)
	}
	os.Exit(1)
}
