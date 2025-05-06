package pkgref

import (
	"fmt"
)

// Error codes
const (
	ErrCodeInvalidComponent = "INVALID_COMPONENT"
	ErrCodeInvalidType      = "INVALID_TYPE"
	ErrCodeInvalidFormat    = "INVALID_FORMAT"
	ErrCodeEmptyValue       = "EMPTY_VALUE"
	ErrCodeInvalidFile      = "INVALID_FILE"
	ErrCodeInvalidDep       = "INVALID_DEPENDENCY"
)

// Error messages
const (
	ErrMsgInvalidComponent = "Invalid component %q: %s"
	ErrMsgInvalidType      = "Invalid type %q: %s"
	ErrMsgInvalidFormat    = "Invalid package reference format: %s"
	ErrMsgEmptyValue       = "Reference value cannot be empty"
	ErrMsgInvalidFile      = "Invalid file %q: %s"
	ErrMsgInvalidDep       = "Invalid dependency %q: %s"
)

// PackageRefError represents errors that can occur during package reference operations
type PackageRefError struct {
	Message string
	Code    string
}

func (e *PackageRefError) Error() string {
	return e.Message
}

// NewError creates a new PackageRefError with the given code and message
func NewError(code, message string) *PackageRefError {
	return &PackageRefError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new PackageRefError with the given code and formatted message
func NewErrorf(code, format string, args ...any) *PackageRefError {
	return &PackageRefError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewInvalidComponentError(component string, err error) *PackageRefError {
	return NewErrorf(ErrCodeInvalidComponent, ErrMsgInvalidComponent, component, err.Error())
}

func NewInvalidTypeError(typeStr string, err error) *PackageRefError {
	return NewErrorf(ErrCodeInvalidType, ErrMsgInvalidType, typeStr, err.Error())
}

func NewInvalidFormatError(err error) *PackageRefError {
	return NewErrorf(ErrCodeInvalidFormat, ErrMsgInvalidFormat, err.Error())
}

func NewEmptyValueError() *PackageRefError {
	return NewError(ErrCodeEmptyValue, ErrMsgEmptyValue)
}

func NewInvalidFileError(path string, err error) *PackageRefError {
	return NewErrorf(ErrCodeInvalidFile, ErrMsgInvalidFile, path, err.Error())
}

func NewInvalidDependencyError(value string, err error) *PackageRefError {
	return NewErrorf(ErrCodeInvalidDep, ErrMsgInvalidDep, value, err.Error())
}
