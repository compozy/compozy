package common

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/pkgref"
)

// LoadConfig is a generic function that loads any config type that implements the Config interface
func LoadConfig[T Config](path string) (T, error) {
	file, err := os.Open(path)
	if err != nil {
		var zero T
		return zero, err
	}

	var config T
	decoder := yaml.NewDecoder(file)
	decodeErr := decoder.Decode(&config)
	closeErr := file.Close()

	if decodeErr != nil {
		var zero T
		return zero, decodeErr
	}
	if closeErr != nil {
		var zero T
		return zero, closeErr
	}

	config.SetCWD(filepath.Dir(path))
	return config, nil
}

// Error codes
const (
	ErrCodeMissingIdField = "MISSING_ID_FIELD"
	ErrCodeUnimplemented  = "UNIMPLEMENTED"
	ErrCodeInvalidRef     = "INVALID_REF"
	ErrCodeMissingPath    = "MISSING_PATH"
)

// Error messages
const (
	ErrMsgMissingIdField = "Missing ID field"
	ErrMsgUnimplemented  = "Feature not implemented: %s"
	ErrMsgInvalidRef     = "Invalid reference type"
	ErrMsgMissingPath    = "Missing path: %s"
)

// ConfigError represents errors that can occur during configuration operations
type ConfigError struct {
	Message string
	Code    string
}

func (e *ConfigError) Error() string {
	return e.Message
}

// NewError creates a new ConfigError with the given code and message
func NewError(code, message string) *ConfigError {
	return &ConfigError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new ConfigError with the given code and formatted message
func NewErrorf(code, format string, args ...any) *ConfigError {
	return &ConfigError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewMissingIdFieldError() *ConfigError {
	return NewError(ErrCodeMissingIdField, ErrMsgMissingIdField)
}

func NewUnimplementedError(feature string) *ConfigError {
	return NewErrorf(ErrCodeUnimplemented, ErrMsgUnimplemented, feature)
}

func NewInvalidRefError() *ConfigError {
	return NewError(ErrCodeInvalidRef, ErrMsgInvalidRef)
}

func NewMissingPathError(path string) *ConfigError {
	return NewErrorf(ErrCodeMissingPath, ErrMsgMissingPath, path)
}

// LoadID loads the ID from either the direct ID field or resolves it from a package reference
func LoadID(
	config Config,
	id string,
	use *pkgref.PackageRefConfig,
	loadFn func(string) (Config, error),
) (string, error) {
	// If ID is directly set, return it
	if id != "" {
		return id, nil
	}

	// Convert package reference to ref
	ref, err := use.IntoRef()
	if err != nil {
		return "", err
	}

	// Handle different reference types
	switch ref.Type.Type {
	case "id":
		return ref.Type.Value, nil
	case "file":
		// Load config from file and get its ID
		path := config.GetCWD()
		if path == "" {
			return "", NewMissingPathError("")
		}
		path = filepath.Join(path, ref.Type.Value)
		loadedConfig, err := loadFn(path)
		if err != nil {
			return "", err
		}
		return loadedConfig.LoadID()
	case "dep":
		// TODO: implement dependency resolution
		return "", NewUnimplementedError("dependency resolution not implemented")
	default:
		return "", NewInvalidRefError()
	}
}
