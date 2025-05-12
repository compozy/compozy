package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/pkgref"
)

func resolvePath(cwd *CWD, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	if !filepath.IsAbs(path) {
		if cwd != nil {
			if err := cwd.Validate(); err != nil {
				return "", fmt.Errorf("invalid current working directory: %w", err)
			}
			return filepath.Abs(cwd.Join(path))
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

func LoadConfig[T Config](cwd *CWD, path string) (T, error) {
	var zero T

	resolvedPath, err := resolvePath(cwd, path)
	if err != nil {
		return zero, err
	}

	// Open the file
	file, err := os.Open(resolvedPath)
	if err != nil {
		return zero, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close() // Ensure file is closed after use

	var config T
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return zero, fmt.Errorf("failed to decode YAML config: %w", err)
	}

	config.SetCWD(filepath.Dir(resolvedPath))
	return config, nil
}

func LoadID(
	config Config,
	id string,
	use *pkgref.PackageRefConfig,
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
		// For file references, directly extract the ID from the YAML
		path := config.GetCWD()
		if path == "" {
			return "", fmt.Errorf("missing path: %s", "")
		}

		// Join the file reference path with the component's CWD
		filePath := filepath.Join(path, ref.Type.Value)

		// Resolve to absolute path
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}

		// Clean the path to resolve any .. or . segments
		cleanPath := filepath.Clean(absPath)

		// Read the file and extract the ID directly
		file, err := os.Open(cleanPath)
		if err != nil {
			return "", err
		}
		defer file.Close()

		// Decode only the ID field from the YAML file
		var yamlConfig struct {
			ID string `yaml:"id"`
		}

		decoder := yaml.NewDecoder(file)
		if err := decoder.Decode(&yamlConfig); err != nil {
			return "", err
		}

		if yamlConfig.ID == "" {
			return "", errors.New("missing ID field")
		}

		return yamlConfig.ID, nil
	case "dep":
		// TODO: implement dependency resolution
		return "", errors.New("dependency resolution not implemented for LoadID()")
	default:
		return "", errors.New("invalid reference type")
	}
}
