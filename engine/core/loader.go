package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func ResolvedPath(cwd *CWD, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	if !filepath.IsAbs(path) {
		if cwd != nil {
			if err := cwd.Validate(); err != nil {
				return "", fmt.Errorf("invalid current working directory: %w", err)
			}
			return cwd.JoinAndCheck(path)
		}
		// Fallback to os.Getwd() for relative paths when cwd is nil
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		return absPath, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	return absPath, nil
}

func LoadConfig[T Config](ctx context.Context, cwd *CWD, projectRoot string, filePath string) (T, error) {
	var zero T

	resolvedPath, err := ResolvedPath(cwd, filePath)
	if err != nil {
		return zero, err
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		return zero, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config T
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return zero, fmt.Errorf("failed to decode YAML config: %w", err)
	}

	metadata := ConfigMetadata{
		CWD:         cwd,
		FilePath:    filePath,
		ProjectRoot: projectRoot,
	}
	config.SetMetadata(&metadata)
	return config, nil
}

// LoadYAMLMap loads a YAML file from the given path and returns its contents as a map[string]any.
func LoadYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read YAML file %s", path)
	}

	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, errors.Wrapf(err, "failed to parse YAML in %s", path)
	}

	// Ensure the result is a map[string]any
	if docMap, ok := doc.(map[string]any); ok {
		return docMap, nil
	}
	return nil, errors.Errorf("YAML file %s does not contain a map", path)
}
