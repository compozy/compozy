package workflow

import (
	"fmt"
)

// Error codes
const (
	ErrCodeFileOpen               = "FILE_OPEN_ERROR"
	ErrCodeDecode                 = "DECODE_ERROR"
	ErrCodeMissingPath            = "MISSING_FILE_PATH"
	ErrCodeInvalidComponent       = "INVALID_COMPONENT"
	ErrCodeNoAgentsDefined        = "NO_AGENTS_DEFINED"
	ErrCodeAgentNotFound          = "AGENT_NOT_FOUND"
	ErrCodeNoToolsDefined         = "NO_TOOLS_DEFINED"
	ErrCodeToolNotFound           = "TOOL_NOT_FOUND"
	ErrCodeNoTasksDefined         = "NO_TASKS_DEFINED"
	ErrCodeTaskNotFound           = "TASK_NOT_FOUND"
	ErrCodeNotImplemented         = "NOT_IMPLEMENTED"
	ErrCodeInvalidRefType         = "INVALID_REF_TYPE"
	ErrCodeAgentValidationError   = "AGENT_VALIDATION_ERROR"
	ErrCodeToolValidationError    = "TOOL_VALIDATION_ERROR"
	ErrCodeTaskValidationError    = "TASK_VALIDATION_ERROR"
	ErrCodeTriggerValidationError = "TRIGGER_VALIDATION_ERROR"
	ErrCodeMerge                  = "MERGE_ERROR"
	ErrCodeFileClose              = "FILE_CLOSE_ERROR"
)

// Error messages
const (
	ErrMsgFileOpen               = "Failed to open workflow config file: %s"
	ErrMsgDecode                 = "Failed to decode workflow config: %s"
	ErrMsgMissingPath            = "Missing file path for workflow"
	ErrMsgInvalidComponent       = "Invalid component for %s reference"
	ErrMsgNoAgentsDefined        = "There's no agents defined in your workflow"
	ErrMsgAgentNotFound          = "Agent not found with reference: %s"
	ErrMsgNoToolsDefined         = "There's no tools defined in your workflow"
	ErrMsgToolNotFound           = "Tool not found with reference: %s"
	ErrMsgNoTasksDefined         = "There's no tasks defined in your workflow"
	ErrMsgTaskNotFound           = "Task not found with reference: %s"
	ErrMsgNotImplemented         = "Not implemented yet"
	ErrMsgInvalidRefType         = "Invalid reference type for %s"
	ErrMsgAgentValidationError   = "Agent validation error: %s"
	ErrMsgToolValidationError    = "Tool validation error: %s"
	ErrMsgTaskValidationError    = "Task validation error: %s"
	ErrMsgTriggerValidationError = "Trigger validation error: %s"
	ErrMsgMerge                  = "Failed to merge workflow configs: %s"
	ErrMsgFileClose              = "Failed to close workflow config file: %s"
)

// WorkflowError represents errors that can occur during workflow configuration
type WorkflowError struct {
	Message string
	Code    string
}

func (e *WorkflowError) Error() string {
	return e.Message
}

// NewError creates a new WorkflowError with the given code and message
func NewError(code, message string) *WorkflowError {
	return &WorkflowError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new WorkflowError with the given code and formatted message
func NewErrorf(code, format string, args ...any) *WorkflowError {
	return &WorkflowError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewFileOpenError(err error) *WorkflowError {
	return NewErrorf(ErrCodeFileOpen, ErrMsgFileOpen, err.Error())
}

func NewDecodeError(err error) *WorkflowError {
	return NewErrorf(ErrCodeDecode, ErrMsgDecode, err.Error())
}

func NewMissingPathError() *WorkflowError {
	return NewError(ErrCodeMissingPath, ErrMsgMissingPath)
}

func NewInvalidComponentError(componentType string) *WorkflowError {
	return NewErrorf(ErrCodeInvalidComponent, ErrMsgInvalidComponent, componentType)
}

func NewNoAgentsDefinedError() *WorkflowError {
	return NewError(ErrCodeNoAgentsDefined, ErrMsgNoAgentsDefined)
}

func NewAgentNotFoundError(ref string) *WorkflowError {
	return NewErrorf(ErrCodeAgentNotFound, ErrMsgAgentNotFound, ref)
}

func NewNoToolsDefinedError() *WorkflowError {
	return NewError(ErrCodeNoToolsDefined, ErrMsgNoToolsDefined)
}

func NewToolNotFoundError(ref string) *WorkflowError {
	return NewErrorf(ErrCodeToolNotFound, ErrMsgToolNotFound, ref)
}

func NewNoTasksDefinedError() *WorkflowError {
	return NewError(ErrCodeNoTasksDefined, ErrMsgNoTasksDefined)
}

func NewTaskNotFoundError(ref string) *WorkflowError {
	return NewErrorf(ErrCodeTaskNotFound, ErrMsgTaskNotFound, ref)
}

func NewNotImplementedError() *WorkflowError {
	return NewError(ErrCodeNotImplemented, ErrMsgNotImplemented)
}

func NewInvalidRefTypeError(componentType string) *WorkflowError {
	return NewErrorf(ErrCodeInvalidRefType, ErrMsgInvalidRefType, componentType)
}

func NewAgentValidationError(err error) *WorkflowError {
	return NewErrorf(ErrCodeAgentValidationError, ErrMsgAgentValidationError, err.Error())
}

func NewToolValidationError(err error) *WorkflowError {
	return NewErrorf(ErrCodeToolValidationError, ErrMsgToolValidationError, err.Error())
}

func NewTaskValidationError(err error) *WorkflowError {
	return NewErrorf(ErrCodeTaskValidationError, ErrMsgTaskValidationError, err.Error())
}

func NewTriggerValidationError(err error) *WorkflowError {
	return NewErrorf(ErrCodeTriggerValidationError, ErrMsgTriggerValidationError, err.Error())
}

func NewMergeError(err error) *WorkflowError {
	return NewErrorf(ErrCodeMerge, ErrMsgMerge, err.Error())
}

func NewFileCloseError(err error) *WorkflowError {
	return NewErrorf(ErrCodeFileClose, ErrMsgFileClose, err.Error())
}
