package cmdpalette

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateDescriptor(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid identifiers, sources, actions, arguments, and predicates", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name   string
			mutate func(*Descriptor)
			want   string
		}{
			{
				name:   "Should reject an invalid command id",
				mutate: func(descriptor *Descriptor) { descriptor.ID = "Bad ID" },
				want:   "id must be a lowercase dotted identifier",
			},
			{
				name: "Should reject a core source that names an extension",
				mutate: func(descriptor *Descriptor) {
					descriptor.Source.Extension = "notes"
				},
				want: "core source cannot name an extension",
			},
			{
				name: "Should reject an extension command without its namespace prefix",
				mutate: func(descriptor *Descriptor) {
					descriptor.ID = "notes.capture"
					descriptor.Source = Source{Kind: SourceKindExtension, Extension: "notes"}
				},
				want: "extension command must use prefix",
			},
			{
				name:   "Should reject an unknown action kind",
				mutate: func(descriptor *Descriptor) { descriptor.Action = Action{Kind: "script"} },
				want:   "unknown action kind",
			},
			{
				name: "Should reject an action that carries a second target",
				mutate: func(descriptor *Descriptor) {
					descriptor.Action = Action{Kind: ActionKindTool, Tool: "compozy__test", URL: "https://example.com"}
				},
				want: "cannot carry",
			},
			{
				name: "Should reject a copy action without content",
				mutate: func(descriptor *Descriptor) {
					descriptor.Action = Action{Kind: ActionKindCopy}
				},
				want: "requires its target",
			},
			{
				name: "Should reject a copy action whose content is only whitespace",
				mutate: func(descriptor *Descriptor) {
					descriptor.Action = Action{Kind: ActionKindCopy, Args: map[string]any{"content": "   "}}
				},
				want: "requires its target",
			},
			{
				name: "Should reject a copy action whose content is not a string",
				mutate: func(descriptor *Descriptor) {
					descriptor.Action = Action{Kind: ActionKindCopy, Args: map[string]any{"content": 1}}
				},
				want: "requires its target",
			},
			{
				name: "Should reject a copy action with a field target",
				mutate: func(descriptor *Descriptor) {
					descriptor.Action = Action{
						Kind: ActionKindCopy,
						URL:  "https://example.com",
						Args: map[string]any{"content": "clipboard text"},
					}
				},
				want: "cannot carry",
			},
			{
				name: "Should reject a copy action with an extra argument",
				mutate: func(descriptor *Descriptor) {
					descriptor.Action = Action{
						Kind: ActionKindCopy,
						Args: map[string]any{"content": "clipboard text", "mime": "text/plain"},
					}
				},
				want: "cannot carry",
			},
			{
				name: "Should reject a copy action whose content exceeds the text limit",
				mutate: func(descriptor *Descriptor) {
					descriptor.Action = Action{
						Kind: ActionKindCopy,
						Args: map[string]any{"content": strings.Repeat("x", MaxViewTextBytes+1)},
					}
				},
				want: "content exceeds",
			},
			{
				name: "Should reject a duplicate argument name",
				mutate: func(descriptor *Descriptor) {
					descriptor.Arguments = []Argument{
						{Name: "path", Type: ArgumentTypeText},
						{Name: "path", Type: ArgumentTypeText},
					}
				},
				want: "duplicate argument",
			},
			{
				name: "Should reject an unsupported argument type",
				mutate: func(descriptor *Descriptor) {
					descriptor.Arguments = []Argument{{Name: "path", Type: "file"}}
				},
				want: "unknown type",
			},
			{
				name:   "Should reject a destructive command without confirmation",
				mutate: func(descriptor *Descriptor) { descriptor.Destructive = true },
				want:   "destructive commands require confirmation",
			},
			{
				name: "Should reject an unknown predicate operator",
				mutate: func(descriptor *Descriptor) {
					descriptor.When = []Predicate{{
						Key: ContextWindowFocused, Operator: "contains", Value: true,
					}}
				},
				want: "when[0]",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				descriptor := testDescriptor("core.command")
				test.mutate(&descriptor)
				err := ValidateDescriptor(descriptor)
				if !errors.Is(err, ErrInvalidDescriptor) {
					t.Fatalf("ValidateDescriptor() error = %v, want ErrInvalidDescriptor", err)
				}
				if !strings.Contains(err.Error(), test.want) {
					t.Fatalf("ValidateDescriptor() error = %v, want substring %q", err, test.want)
				}
			})
		}
	})

	t.Run("Should accept a host-target copy action with args.content", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("core.command")
		descriptor.Action = Action{
			Kind: ActionKindCopy,
			Args: map[string]any{"content": "clipboard text"},
		}
		if err := ValidateDescriptor(descriptor); err != nil {
			t.Fatalf("ValidateDescriptor() error = %v", err)
		}
	})
}
