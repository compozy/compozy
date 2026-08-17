package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const pluginDataEnvironmentName = "PLUGIN_DATA"

func (e *CallExecutor) prepareMCPDataDir(server compozyconfig.MCPServer) error {
	path := strings.TrimSpace(server.Env[pluginDataEnvironmentName])
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("mcp: plugin data directory for server %q must be absolute", server.Name)
	}
	if e == nil || e.dataDirCreator == nil {
		return errors.New("mcp: plugin data directory creator is unavailable")
	}
	if err := e.dataDirCreator(path); err != nil {
		return fmt.Errorf("mcp: prepare plugin data directory for server %q: %w", server.Name, err)
	}
	return nil
}

func createWritableMCPDataDir(path string) (err error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	probe, err := os.CreateTemp(path, ".compozy-write-check-*")
	if err != nil {
		return fmt.Errorf("verify %q is writable: %w", path, err)
	}
	probePath := probe.Name()
	defer func() {
		err = errors.Join(err, os.Remove(probePath))
	}()
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close writability probe for %q: %w", path, err)
	}
	return nil
}
