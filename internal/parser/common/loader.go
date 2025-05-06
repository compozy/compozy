package common

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
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
