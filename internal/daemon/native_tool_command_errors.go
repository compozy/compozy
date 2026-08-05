package daemon

import (
	"fmt"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeCommandInvalidInputError(id toolspkg.ToolID, message string) error {
	return nativeCommandToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		message,
		toolspkg.ErrToolInvalidInput,
		toolspkg.ReasonSchemaInvalid,
	)
}

func nativeCommandDependencyError(id toolspkg.ToolID, message string) error {
	return nativeCommandToolError(
		toolspkg.ErrorCodeUnavailable,
		id,
		message,
		toolspkg.ErrToolUnavailable,
		toolspkg.ReasonDependencyMissing,
	)
}

func nativeCommandNotFoundError(id toolspkg.ToolID, message string) error {
	return nativeCommandToolError(
		toolspkg.ErrorCodeNotFound,
		id,
		message,
		toolspkg.ErrToolNotFound,
		toolspkg.ReasonToolUnknown,
	)
}

func nativeCommandToolError(
	code toolspkg.ErrorCode,
	id toolspkg.ToolID,
	message string,
	cause error,
	reason toolspkg.ReasonCode,
) error {
	trimmed := strings.TrimSpace(message)
	return toolspkg.NewToolError(
		code,
		id,
		trimmed,
		fmt.Errorf("%w: %s", cause, trimmed),
		reason,
	)
}
