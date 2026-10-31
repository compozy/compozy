package daemon

import (
	"fmt"
	"path/filepath"

	mcppkg "github.com/compozy/compozy/internal/mcp"
)

// bootMCPStdioPool owns the daemon-wide stdio MCP process pool. It runs after the daemon lock is
// held, so every home under the pool root belongs to an earlier generation of this daemon home.
func (d *Daemon) bootMCPStdioPool(state *bootState, cleanup *bootCleanup) error {
	settings := state.cfg.MCP.Stdio
	pool, err := mcppkg.NewStdioSessionPool(mcppkg.StdioPoolConfig{
		Root:               filepath.Join(d.homePaths.HomeDir, "cache", "mcp"),
		IdleTimeout:        settings.IdleTimeout,
		ConnectTimeout:     settings.ConnectTimeout,
		MaxConcurrentCalls: settings.MaxConcurrentCalls,
		Logger:             state.logger,
	})
	if err != nil {
		return fmt.Errorf("daemon: create stdio MCP session pool: %w", err)
	}
	cleanup.add(pool.Close)
	state.mcpStdioPool = pool
	removed, err := pool.SweepOrphanHomes()
	if err != nil && state.logger != nil {
		state.logger.Warn("daemon: sweep orphaned stdio MCP homes", "error", err)
	}
	if removed > 0 && state.logger != nil {
		state.logger.Info("daemon: removed orphaned stdio MCP homes", "count", removed)
	}
	return nil
}
