package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
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

func LoadConfig[T Config](cwd *CWD, path string) (T, error) {
	var zero T

	resolvedPath, err := resolvePath(cwd, path)
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

	if err := config.SetCWD(filepath.Dir(resolvedPath)); err != nil {
		return zero, err
	}
	return config, nil
}

func LoadID(
	config Config,
	id string,
	use *PackageRefConfig,
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

	switch ref.Type.Type {
	case RefTypeNameID:
		return ref.Type.Value, nil
	case RefTypeNameFile:
		cwd := config.GetCWD()
		filePath, err := cwd.JoinAndCheck(ref.Type.Value)
		if err != nil {
			return "", err
		}

		file, err := os.Open(filePath)
		if err != nil {
			return "", err
		}
		defer file.Close()

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
	case RefTypeNameDep:
		// TODO: implement dependency resolution
		return "", errors.New("dependency resolution not implemented for LoadID()")
	default:
		return "", errors.New("invalid reference type")
	}
}
