package cli

import (
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/spf13/cobra"
)

type toolCommandError struct {
	response ToolErrorResponseRecord
	err      error
}

func (e *toolCommandError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	if e.err != nil {
		return redactToolDiagnostic(e.err.Error())
	}
	apiErr := newToolAPIError(0, "", e.response)
	return apiErr.Error()
}

func writeToolCommandError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	response, ok := toolErrorResponseForError(err)
	if !ok {
		return err
	}
	mode, modeErr := resolveOutputFormat(cmd)
	if modeErr != nil {
		return modeErr
	}
	if mode == OutputJSON {
		if writeErr := writeJSON(cmd, response); writeErr != nil {
			return errors.Join(err, writeErr)
		}
	}
	return err
}

func toolErrorResponseForError(err error) (ToolErrorResponseRecord, bool) {
	if apiErr, ok := errors.AsType[*toolAPIError](err); ok {
		return apiErr.Response(), true
	}
	if commandErr, ok := errors.AsType[*toolCommandError](err); ok {
		return sanitizeToolErrorResponse(commandErr.response), true
	}
	if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); ok {
		var partialResult *toolspkg.ToolResult
		if toolErr.PartialResult != nil {
			partial := *toolErr.PartialResult
			partialResult = &partial
		}
		return sanitizeToolErrorResponse(ToolErrorResponseRecord{
			Error: contract.ToolErrorPayload{
				Code:          toolErr.Code,
				Message:       toolErr.Error(),
				ToolID:        toolErr.ToolID,
				ReasonCodes:   append([]toolspkg.ReasonCode(nil), toolErr.ReasonCodes...),
				Layer:         toolOperatorCLIKey,
				PartialResult: partialResult,
			},
		}), true
	}
	if validationErr, ok := errors.AsType[*toolspkg.ValidationError](err); ok {
		return sanitizeToolErrorResponse(ToolErrorResponseRecord{
			Error: contract.ToolErrorPayload{
				Code:        toolspkg.ErrorCodeInvalidInput,
				Message:     validationErr.Error(),
				ReasonCodes: []toolspkg.ReasonCode{validationErr.Reason},
				Layer:       toolOperatorCLIKey,
			},
		}), true
	}
	return ToolErrorResponseRecord{}, false
}

func toolValidationCommandError(
	id toolspkg.ToolID,
	message string,
	err error,
) *toolCommandError {
	reason := toolspkg.ReasonSchemaInvalid
	if extracted, ok := toolspkg.ReasonOf(err); ok {
		reason = extracted
	}
	payload := contract.ToolErrorPayload{
		Code:        toolspkg.ErrorCodeInvalidInput,
		Message:     message,
		ReasonCodes: []toolspkg.ReasonCode{reason},
		Layer:       toolOperatorCLIKey,
	}
	if id != "" {
		payload.ToolID = id
	}
	return &toolCommandError{
		response: ToolErrorResponseRecord{Error: payload},
		err:      fmt.Errorf("%s: %w", message, err),
	}
}
