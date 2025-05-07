package server

import (
	"fmt"
	"os"
	"path/filepath"
)

// ServerConfig holds the configuration for the HTTP server
type ServerConfig struct {
	CWD         string
	Host        string
	Port        int
	CORSEnabled bool
}

// NewServerConfig creates a new server configuration with defaults
func NewServerConfig() *ServerConfig {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	return &ServerConfig{
		CWD:         cwd,
		Host:        "0.0.0.0",
		Port:        3000,
		CORSEnabled: true,
	}
}

// FullAddress returns the full address string (host:port)
func (c *ServerConfig) FullAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ResolvePath resolves a path relative to the CWD
func (c *ServerConfig) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.CWD, path)
}
