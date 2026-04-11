package agents

import (
	"errors"

	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// BlockedReasonForError classifies reusable-agent errors into the stable
// machine-readable blocked-reason vocabulary used by runtime signals and
// nested-agent failures.
func BlockedReasonForError(err error) (kinds.ReusableAgentBlockedReason, bool) {
	switch {
	case err == nil:
		return "", false
	case errors.Is(err, ErrAgentNotFound),
		errors.Is(err, ErrInvalidAgentName),
		errors.Is(err, ErrReservedAgentName),
		errors.Is(err, ErrMissingAgentDefinition),
		errors.Is(err, ErrMalformedFrontmatter),
		errors.Is(err, ErrUnsupportedMetadataField),
		errors.Is(err, ErrInvalidRuntimeDefaults):
		return kinds.ReusableAgentBlockedReasonInvalidAgent, true
	case errors.Is(err, ErrMalformedMCPConfig),
		errors.Is(err, ErrMissingEnvironmentVariable),
		errors.Is(err, ErrReservedMCPServerName):
		return kinds.ReusableAgentBlockedReasonInvalidMCP, true
	default:
		return "", false
	}
}

// IsValidationError reports whether the provided error describes an invalid
// reusable-agent definition or MCP configuration.
func IsValidationError(err error) bool {
	_, ok := BlockedReasonForError(err)
	return ok
}
