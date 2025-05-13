package server

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the configuration for the HTTP server
type Config struct {
	CWD         string
	Host        string
	Port        int
	CORSEnabled bool
}

// NewServerConfig creates a new server configuration with defaults
func NewServerConfig() *Config {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	return &Config{
		CWD:         cwd,
		Host:        "0.0.0.0",
		Port:        3000,
		CORSEnabled: true,
	}
}

// FullAddress returns the full address string (host:port)
func (c *Config) FullAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ResolvePath resolves a path relative to the CWD
func (c *Config) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.CWD, path)
}
