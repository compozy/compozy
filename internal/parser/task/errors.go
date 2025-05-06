package task

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
	ErrCodeInvalidTaskType     = "INVALID_TASK_TYPE"
	ErrCodeInvalidInputSchema  = "INVALID_INPUT_SCHEMA"
	ErrCodeInvalidOutputSchema = "INVALID_OUTPUT_SCHEMA"
	ErrCodeInvalidDecisionTask = "INVALID_DECISION_TASK"
	ErrCodeMerge               = "MERGE_ERROR"
	ErrCodeFileClose           = "FILE_CLOSE_ERROR"
)

// Error messages
const (
	ErrMsgFileOpen            = "Failed to open task config file: %s"
	ErrMsgDecode              = "Failed to decode task config: %s"
	ErrMsgMissingPath         = "Missing file path for task"
	ErrMsgInvalidPackageRef   = "Invalid package reference: %s"
	ErrMsgInvalidType         = "Package reference must be a task, agent, or tool"
	ErrMsgInvalidTaskType     = "Invalid task type: %s"
	ErrMsgInvalidInputSchema  = "Invalid input schema: %s"
	ErrMsgInvalidOutputSchema = "Invalid output schema: %s"
	ErrMsgInvalidDecisionTask = "Decision task must have at least one route"
	ErrMsgMerge               = "Failed to merge task configs: %s"
	ErrMsgFileClose           = "Failed to close task config file: %s"
)

// TaskError represents errors that can occur during task configuration
type TaskError struct {
	Message string
	Code    string
}

func (e *TaskError) Error() string {
	return e.Message
}

// NewError creates a new TaskError with the given code and message
func NewError(code, message string) *TaskError {
	return &TaskError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new TaskError with the given code and formatted message
func NewErrorf(code, format string, args ...interface{}) *TaskError {
	return &TaskError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewFileOpenError(err error) *TaskError {
	return NewErrorf(ErrCodeFileOpen, ErrMsgFileOpen, err.Error())
}

func NewDecodeError(err error) *TaskError {
	return NewErrorf(ErrCodeDecode, ErrMsgDecode, err.Error())
}

func NewMissingPathError() *TaskError {
	return NewError(ErrCodeMissingPath, ErrMsgMissingPath)
}

func NewInvalidPackageRefError(err error) *TaskError {
	return NewErrorf(ErrCodeInvalidPackageRef, ErrMsgInvalidPackageRef, err.Error())
}

func NewInvalidTypeError() *TaskError {
	return NewError(ErrCodeInvalidType, ErrMsgInvalidType)
}

func NewInvalidTaskTypeError(taskType string) *TaskError {
	return NewErrorf(ErrCodeInvalidTaskType, ErrMsgInvalidTaskType, taskType)
}

func NewInvalidInputSchemaError(err error) *TaskError {
	return NewErrorf(ErrCodeInvalidInputSchema, ErrMsgInvalidInputSchema, err.Error())
}

func NewInvalidOutputSchemaError(err error) *TaskError {
	return NewErrorf(ErrCodeInvalidOutputSchema, ErrMsgInvalidOutputSchema, err.Error())
}

func NewInvalidDecisionTaskError() *TaskError {
	return NewError(ErrCodeInvalidDecisionTask, ErrMsgInvalidDecisionTask)
}

func NewMergeError(err error) *TaskError {
	return NewErrorf(ErrCodeMerge, ErrMsgMerge, err.Error())
}

func NewFileCloseError(err error) *TaskError {
	return NewErrorf(ErrCodeFileClose, ErrMsgFileClose, err.Error())
}
