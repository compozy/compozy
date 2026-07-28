package config

import (
	"errors"
	"fmt"
	"strings"
)

// HTTPConfig controls the HTTP server bind address.
type HTTPConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// Validate ensures the HTTP bind settings are valid.
func (c HTTPConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("http.host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535: %d", c.Port)
	}
	return nil
}
