package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

const (
	cmdPaletteShortcutConflict = "shortcut_conflict"
	cmdPaletteAliasConflict    = "alias_conflict"
	cmdPaletteInvalidAlias     = "invalid_alias"
)

type cmdPaletteMutationAPIError struct {
	statusCode int
	payload    contract.SettingsWindowManagerMutationError
}

func (e *cmdPaletteMutationAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	switch e.payload.Error {
	case cmdPaletteShortcutConflict:
		return fmt.Sprintf(
			"shortcut conflict — %s is used by %q. Re-run with --overwrite to take it.",
			e.payload.Chord,
			e.payload.Owner,
		)
	case cmdPaletteAliasConflict:
		return fmt.Sprintf(
			"alias conflict — %q is owned by %q. Re-run with --overwrite to take it.",
			e.payload.Alias,
			e.payload.Owner,
		)
	case cmdPaletteInvalidAlias:
		return strings.TrimSpace(e.payload.Message)
	default:
		return apiErrorMessage(e.payload.Error, "")
	}
}

func (e *cmdPaletteMutationAPIError) cliExitCode() int {
	if e != nil && e.statusCode == 422 {
		return 2
	}
	return 1
}

func (e *cmdPaletteMutationAPIError) errorPayload() contract.ErrorPayload {
	if e == nil {
		return contract.ErrorPayload{}
	}
	details := map[string]string{}
	if owner := strings.TrimSpace(e.payload.Owner); owner != "" {
		details["owner"] = owner
	}
	if chord := strings.TrimSpace(e.payload.Chord); chord != "" {
		details["chord"] = chord
	}
	if alias := strings.TrimSpace(e.payload.Alias); alias != "" {
		details["alias"] = alias
	}
	if message := strings.TrimSpace(e.payload.Message); message != "" {
		details["message"] = message
	}
	payload := contract.ErrorPayload{Error: e.payload.Error, Code: e.payload.Error}
	if len(details) > 0 {
		payload.Details = details
	}
	return payload
}

func parseCmdPaletteMutationAPIError(statusCode int, _ string, body []byte) (bool, error) {
	var payload contract.SettingsWindowManagerMutationError
	if json.Unmarshal(body, &payload) != nil {
		return false, nil
	}
	switch strings.TrimSpace(payload.Error) {
	case cmdPaletteShortcutConflict, cmdPaletteAliasConflict, cmdPaletteInvalidAlias:
		payload.Message = redactToolDiagnostic(payload.Message)
		return true, &cmdPaletteMutationAPIError{statusCode: statusCode, payload: payload}
	default:
		return false, nil
	}
}
