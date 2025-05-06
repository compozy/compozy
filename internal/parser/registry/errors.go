package registry

import (
	"fmt"
)

// Error codes
const (
	ErrCodeFileOpen         = "FILE_OPEN_ERROR"
	ErrCodeDecode           = "DECODE_ERROR"
	ErrCodeMissingPath      = "MISSING_FILE_PATH"
	ErrCodeInvalidType      = "INVALID_COMPONENT_TYPE"
	ErrCodeMainPathNotFound = "MAIN_PATH_NOT_FOUND"
	ErrCodeFileClose        = "FILE_CLOSE_ERROR"
)

// Error messages
const (
	ErrMsgFileOpen         = "Failed to open registry config file: %s"
	ErrMsgDecode           = "Failed to decode registry config: %s"
	ErrMsgMissingPath      = "Missing file path for registry"
	ErrMsgInvalidType      = "Invalid component type: %s"
	ErrMsgMainPathNotFound = "Main path does not exist: %s"
	ErrMsgFileClose        = "Failed to close registry config file: %s"
)

// RegistryError represents errors that can occur during registry configuration
type RegistryError struct {
	Message string
	Code    string
}

func (e *RegistryError) Error() string {
	return e.Message
}

// NewError creates a new RegistryError with the given code and message
func NewError(code, message string) *RegistryError {
	return &RegistryError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new RegistryError with the given code and formatted message
func NewErrorf(code, format string, args ...interface{}) *RegistryError {
	return &RegistryError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewFileOpenError(err error) *RegistryError {
	return NewErrorf(ErrCodeFileOpen, ErrMsgFileOpen, err.Error())
}

func NewDecodeError(err error) *RegistryError {
	return NewErrorf(ErrCodeDecode, ErrMsgDecode, err.Error())
}

func NewMissingPathError() *RegistryError {
	return NewError(ErrCodeMissingPath, ErrMsgMissingPath)
}

func NewInvalidTypeError(componentType string) *RegistryError {
	return NewErrorf(ErrCodeInvalidType, ErrMsgInvalidType, componentType)
}

func NewMainPathNotFoundError(mainPath string) *RegistryError {
	return NewErrorf(ErrCodeMainPathNotFound, ErrMsgMainPathNotFound, mainPath)
}

func NewFileCloseError(err error) *RegistryError {
	return NewErrorf(ErrCodeFileClose, ErrMsgFileClose, err.Error())
}
