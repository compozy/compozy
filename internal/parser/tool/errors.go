package tool

import (
	"fmt"
)

// Error codes
const (
	ErrCodeFileOpen            = "FILE_OPEN_ERROR"
	ErrCodeDecode              = "DECODE_ERROR"
	ErrCodeMissingPath         = "MISSING_FILE_PATH"
	ErrCodeInvalidPackageRef   = "INVALID_PACKAGE_REF"
	ErrCodeInvalidType         = "INVALID_COMPONENT_TYPE"
	ErrCodeInvalidExecutePath  = "INVALID_EXECUTE_PATH"
	ErrCodeMissingToolID       = "MISSING_TOOL_ID"
	ErrCodeInvalidToolExecute  = "INVALID_TOOL_EXECUTE"
	ErrCodeInvalidInputSchema  = "INVALID_INPUT_SCHEMA"
	ErrCodeInvalidOutputSchema = "INVALID_OUTPUT_SCHEMA"
	ErrCodeMerge               = "MERGE_ERROR"
	ErrCodeFileClose           = "FILE_CLOSE_ERROR"
)

// Error messages
const (
	ErrMsgFileOpen            = "Failed to open tool config file: %s"
	ErrMsgDecode              = "Failed to decode tool config: %s"
	ErrMsgMissingPath         = "Missing file path for tool"
	ErrMsgInvalidPackageRef   = "Invalid package reference: %s"
	ErrMsgInvalidType         = "Package reference must be a tool"
	ErrMsgInvalidExecutePath  = "Invalid execute path: %s"
	ErrMsgMissingToolID       = "Tool ID is required for TypeScript execution"
	ErrMsgInvalidToolExecute  = "Invalid tool execute path: %s"
	ErrMsgInvalidInputSchema  = "Invalid input schema: %s"
	ErrMsgInvalidOutputSchema = "Invalid output schema: %s"
	ErrMsgMerge               = "Failed to merge tool configs: %s"
	ErrMsgFileClose           = "Failed to close tool config file: %s"
)

// ToolError represents errors that can occur during tool configuration
type ToolError struct {
	Message string
	Code    string
}

func (e *ToolError) Error() string {
	return e.Message
}

// NewError creates a new ToolError with the given code and message
func NewError(code, message string) *ToolError {
	return &ToolError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new ToolError with the given code and formatted message
func NewErrorf(code, format string, args ...interface{}) *ToolError {
	return &ToolError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewFileOpenError(err error) *ToolError {
	return NewErrorf(ErrCodeFileOpen, ErrMsgFileOpen, err.Error())
}

func NewDecodeError(err error) *ToolError {
	return NewErrorf(ErrCodeDecode, ErrMsgDecode, err.Error())
}

func NewMissingPathError() *ToolError {
	return NewError(ErrCodeMissingPath, ErrMsgMissingPath)
}

func NewInvalidPackageRefError(err error) *ToolError {
	return NewErrorf(ErrCodeInvalidPackageRef, ErrMsgInvalidPackageRef, err.Error())
}

func NewInvalidTypeError() *ToolError {
	return NewError(ErrCodeInvalidType, ErrMsgInvalidType)
}

func NewInvalidExecutePathError(err error) *ToolError {
	return NewErrorf(ErrCodeInvalidExecutePath, ErrMsgInvalidExecutePath, err.Error())
}

func NewMissingToolIDError() *ToolError {
	return NewError(ErrCodeMissingToolID, ErrMsgMissingToolID)
}

func NewInvalidToolExecuteError(path string) *ToolError {
	return NewErrorf(ErrCodeInvalidToolExecute, ErrMsgInvalidToolExecute, path)
}

func NewInvalidInputSchemaError(err error) *ToolError {
	return NewErrorf(ErrCodeInvalidInputSchema, ErrMsgInvalidInputSchema, err.Error())
}

func NewInvalidOutputSchemaError(err error) *ToolError {
	return NewErrorf(ErrCodeInvalidOutputSchema, ErrMsgInvalidOutputSchema, err.Error())
}

func NewMergeError(err error) *ToolError {
	return NewErrorf(ErrCodeMerge, ErrMsgMerge, err.Error())
}

func NewFileCloseError(err error) *ToolError {
	return NewErrorf(ErrCodeFileClose, ErrMsgFileClose, err.Error())
}
