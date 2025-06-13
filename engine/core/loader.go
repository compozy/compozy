package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/pkg/ref"
	"gopkg.in/yaml.v3"
)

func ResolvePath(cwd *PathCWD, path string) (string, error) {
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
		// Fallback to os.Getwd() for relative paths when CWD is nil
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

func LoadConfig[T Config](filePath string) (T, string, error) {
	var zero T

	file, err := os.Open(filePath)
	if err != nil {
		return zero, "", fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config T
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return zero, "", fmt.Errorf("failed to decode YAML config: %w", err)
	}

	config.SetFilePath(filePath)
	if err := config.SetCWD(filepath.Dir(filePath)); err != nil {
		return zero, "", err
	}

	return config, filePath, nil
}

func LoadConfigWithEvaluator[T Config](filePath string, ev *ref.Evaluator) (T, string, error) {
	var zero T

	file, err := os.Open(filePath)
	if err != nil {
		return zero, "", fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	node, err := ref.ProcessReaderWithEvaluator(file, ev)
	if err != nil {
		return zero, "", fmt.Errorf("failed to process file: %w", err)
	}

	processedData, err := yaml.Marshal(node)
	if err != nil {
		return zero, "", fmt.Errorf("failed to marshal processed config: %w", err)
	}

	var config T
	if err := yaml.Unmarshal(processedData, &config); err != nil {
		return zero, "", fmt.Errorf("failed to unmarshal processed config: %w", err)
	}

	config.SetFilePath(filePath)
	if err := config.SetCWD(filepath.Dir(filePath)); err != nil {
		return zero, "", err
	}

	return config, filePath, nil
}

func MapFromFilePath(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var itemMap map[string]any
	err = yaml.Unmarshal(data, &itemMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal local scope: %w", err)
	}

	return itemMap, nil
}
