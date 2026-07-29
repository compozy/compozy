package contract

import toolspkg "github.com/compozy/compozy/internal/tools"

// ExtensionCommandSpec presents an extension tool as an operator-facing command.
type ExtensionCommandSpec = toolspkg.ExtensionCommandSpec

// ExtensionCommandGroupSpec declares a presentation-only command group.
type ExtensionCommandGroupSpec struct {
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

// CommandFlagType is the closed scalar type accepted by projected command flags.
type CommandFlagType = toolspkg.CommandFlagType

const (
	// CommandFlagString projects a JSON string.
	CommandFlagString = toolspkg.CommandFlagString
	// CommandFlagBoolean projects a JSON boolean.
	CommandFlagBoolean = toolspkg.CommandFlagBoolean
	// CommandFlagInteger projects a JSON integer.
	CommandFlagInteger = toolspkg.CommandFlagInteger
	// CommandFlagNumber projects a JSON number.
	CommandFlagNumber = toolspkg.CommandFlagNumber
)

// CommandFlagTypeValues returns the complete generated-contract enum domain.
func CommandFlagTypeValues() []string {
	return toolspkg.CommandFlagTypeValues()
}

// CommandFlag is the schema-derived CLI projection for one top-level input field.
type CommandFlag = toolspkg.CommandFlag
