package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/pkgref"
)

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
		path = filepath.Join(path, ref.Type.Value)

		// Read the file and extract the ID directly
		file, err := os.Open(path)
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
