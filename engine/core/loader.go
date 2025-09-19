package core

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	data, err := os.ReadFile(filePath)
	if err != nil {
		return zero, "", fmt.Errorf("failed to open config file: %w", err)
	}

	// Pre-scan YAML to reject any directive keys starting with '$' outside schema contexts
	if err := rejectDollarKeys(data, filePath); err != nil {
		return zero, "", err
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return zero, "", fmt.Errorf("failed to decode YAML config: %w", err)
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

// rejectDollarKeys scans YAML documents and returns an error when encountering
// any mapping key that starts with '$' (e.g., $ref, $use, $merge, $ptr).
// It preserves precise line/column information for actionable messages.
func rejectDollarKeys(data []byte, filePath string) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to parse YAML in %s: %w", filePath, err)
		}
		if err := walkAndReject(&doc, filePath, nil); err != nil {
			return err
		}
	}
	return nil
}

func walkAndReject(n *yaml.Node, filePath string, path []string) error {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			if err := walkAndReject(c, filePath, path); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			val := n.Content[i+1]
			if key != nil && key.Kind == yaml.ScalarNode && strings.HasPrefix(key.Value, "$") {
				if !inSchemaContext(path) {
					return fmt.Errorf(
						"%s:%d:%d: unsupported directive key '%s' detected; "+
							"directives like $ref/$use/$merge/$ptr in configuration are deprecated. "+
							"Use ID-based references and the compile/link step instead",
						filePath,
						key.Line,
						key.Column,
						key.Value,
					)
				}
			}
			nextPath := append(append([]string(nil), path...), key.Value)
			if err := walkAndReject(val, filePath, nextPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func inSchemaContext(path []string) bool {
	for i := len(path) - 1; i >= 0; i-- {
		switch path[i] {
		case "input", "output", "schema", "schemas", "input_schema", "output_schema":
			return true
		}
	}
	return false
}
