package server

import (
	"fmt"
	"path/filepath"
)

type Config struct {
	CWD         string
	Host        string
	Port        int
	CORSEnabled bool
	ConfigFile  string
}

func (c *Config) FullAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *Config) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.CWD, path)
}
