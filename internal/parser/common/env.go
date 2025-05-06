package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvError represents errors that can occur when handling environment variables
type EnvError struct {
	Op  string
	Err error
}

func (e *EnvError) Error() string {
	if e.Err == nil {
		return e.Op
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *EnvError) Unwrap() error {
	return e.Err
}

// NewEnvError creates a new environment error
func NewEnvError(op string, err error) error {
	return &EnvError{Op: op, Err: err}
}

// EnvMap represents environment variables for a component
type EnvMap map[string]string

// FromFile loads environment variables from a file into an EnvMap
func (e EnvMap) FromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Return empty map if file doesn't exist
		}
		return NewEnvError("open env file", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		e[key] = value
	}

	if err := scanner.Err(); err != nil {
		return NewEnvError("read env file", err)
	}

	return nil
}

// LoadFromFile loads environment variables from a file and merges them with existing values
func (e EnvMap) LoadFromFile(path string) error {
	// Convert relative path to absolute if needed
	absPath, err := filepath.Abs(path)
	if err != nil {
		return NewEnvError("resolve env file path", err)
	}

	// Create a new map to store the loaded values
	newEnv := make(EnvMap)
	if err := newEnv.FromFile(absPath); err != nil {
		return err
	}

	// Merge the new values with existing ones
	e.Merge(newEnv)
	return nil
}

// Merge merges another environment map into this one
func (e EnvMap) Merge(other EnvMap) {
	for k, v := range other {
		e[k] = v
	}
}
