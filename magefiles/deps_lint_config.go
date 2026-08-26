//go:build mage

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

const golangciConfigPath = ".golangci.yml"

type golangciConfig struct {
	Linters struct {
		Enable []string `yaml:"enable"`
	} `yaml:"linters"`
}

// golangciEnabledLinters returns the config's enable list for --enable-only,
// which drops formatters from `run` without narrowing linter coverage.
func golangciEnabledLinters(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read golangci config: %w", err)
	}
	var config golangciConfig
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse golangci config %s: %w", path, err)
	}
	linters := make([]string, 0, len(config.Linters.Enable))
	for _, name := range config.Linters.Enable {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			linters = append(linters, trimmed)
		}
	}
	if len(linters) == 0 {
		return nil, fmt.Errorf(
			"golangci config %s has no linters.enable entries; the split lint lane requires an explicit list",
			path,
		)
	}
	return linters, nil
}
