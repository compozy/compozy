// Command notes-commands is a public-SDK example that contributes operator verbs.
//
// It registers two tools and presents them as commands: a flat `add` leaf and a
// nested `list/recent` leaf under a declared `list` group. Every flag on this
// page is projected from the tool's own input schema, so the command surface and
// the agent-callable tool stay one declaration.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

type addInput struct {
	Text string   `json:"text"`
	Tags []string `json:"tags,omitempty"`
}

type recentInput struct {
	Limit  int64  `json:"limit,omitempty"`
	Format string `json:"format,omitempty"`
}

type note struct {
	Text string
	Tags []string
}

type store struct {
	mu    sync.Mutex
	notes []note
}

func (s *store) add(entry note) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notes = append(s.notes, entry)
	return len(s.notes)
}

func (s *store) recent(limit int) []note {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.notes) {
		limit = len(s.notes)
	}
	tail := s.notes[len(s.notes)-limit:]
	return slices.Clone(tail)
}

var addInputSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["text"],
  "properties": {
    "text": { "type": "string", "minLength": 1 },
    "tags": { "type": "array", "items": { "type": "string" } }
  }
}`)

var recentInputSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "limit": { "type": "integer", "minimum": 1, "maximum": 50, "default": 5 },
    "format": { "type": "string", "enum": ["text", "markdown"] }
  }
}`)

func main() {
	notes := &store{}
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:        "notes",
		Version:     "0.1.0",
		Description: "Keep short workspace notes and expose them as operator commands",
		Subprocess:  compozysdk.DescribeSubprocess{Command: "./bin"},
	})
	registerAdd(extension, notes)
	registerRecent(extension, notes)
	if err := extension.CommandGroup("list", "Read stored notes"); err != nil {
		fail("declare list command group", err)
	}
	if err := extension.Run(context.Background()); err != nil {
		fail("run extension", err)
	}
}

func registerAdd(extension *compozysdk.Extension, notes *store) {
	err := compozysdk.Tool[addInput](
		extension,
		"add",
		compozysdk.ToolOptions{
			Description: "Store one note",
			InputSchema: addInputSchema,
			Command: &compozysdk.ExtensionCommandSpec{
				Verb:    "add",
				Summary: "Store one note",
				Example: `--text "ship the beta" --tag release --tag beta`,
				Flags:   map[string]string{"text": "text", "tag": "tags"},
			},
		},
		func(_ context.Context, request compozysdk.ToolRequest[addInput]) (compozysdk.ToolResult, error) {
			total := notes.add(note{Text: request.Input.Text, Tags: request.Input.Tags})
			return compozysdk.TextResult(fmt.Sprintf("stored note %d", total)), nil
		},
	)
	if err != nil {
		fail("register add command", err)
	}
}

func registerRecent(extension *compozysdk.Extension, notes *store) {
	err := compozysdk.Tool[recentInput](
		extension,
		"list_recent",
		compozysdk.ToolOptions{
			Description: "List the most recent notes",
			ReadOnly:    true,
			InputSchema: recentInputSchema,
			Command: &compozysdk.ExtensionCommandSpec{
				Verb:    "list/recent",
				Summary: "List the most recent notes",
				Example: "--limit 3 --format markdown",
				Flags:   map[string]string{"limit": "limit", "format": "format"},
			},
		},
		func(_ context.Context, request compozysdk.ToolRequest[recentInput]) (compozysdk.ToolResult, error) {
			return compozysdk.TextResult(renderNotes(notes.recent(int(request.Input.Limit)), request.Input.Format)), nil
		},
	)
	if err != nil {
		fail("register list/recent command", err)
	}
}

func renderNotes(entries []note, format string) string {
	if len(entries) == 0 {
		return "no notes"
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		line := entry.Text
		if len(entry.Tags) > 0 {
			line += " [" + strings.Join(entry.Tags, ", ") + "]"
		}
		if format == "markdown" {
			line = "- " + line
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func fail(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
