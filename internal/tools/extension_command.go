package tools

import "encoding/json"

// ExtensionCommandSpec presents an extension tool as an operator-facing command.
// Execution still flows through the canonical tool runtime.
type ExtensionCommandSpec struct {
	Verb    string            `json:"verb"`
	Summary string            `json:"summary"`
	Example string            `json:"example,omitempty"`
	Flags   map[string]string `json:"flags,omitempty"`
}

// CommandFlagType is the closed scalar type accepted by projected command flags.
type CommandFlagType string

const (
	// CommandFlagString projects a JSON string.
	CommandFlagString CommandFlagType = "string"
	// CommandFlagBoolean projects a JSON boolean.
	CommandFlagBoolean CommandFlagType = "boolean"
	// CommandFlagInteger projects a JSON integer.
	CommandFlagInteger CommandFlagType = "integer"
	// CommandFlagNumber projects a JSON number.
	CommandFlagNumber CommandFlagType = "number"
)

// CommandFlagTypeValues returns the complete generated-contract enum domain.
func CommandFlagTypeValues() []string {
	return []string{
		string(CommandFlagString),
		string(CommandFlagBoolean),
		string(CommandFlagInteger),
		string(CommandFlagNumber),
	}
}

// CommandFlag is the schema-derived CLI projection for one top-level input field.
type CommandFlag struct {
	Name       string          `json:"name"`
	Field      string          `json:"field"`
	Type       CommandFlagType `json:"type"`
	Repeatable bool            `json:"repeatable"`
	Required   bool            `json:"required"`
	Nullable   bool            `json:"nullable"`
	Enum       []string        `json:"enum,omitempty"`
	Default    json.RawMessage `json:"default,omitempty"`
	Minimum    *float64        `json:"minimum,omitempty"`
	Maximum    *float64        `json:"maximum,omitempty"`
}
