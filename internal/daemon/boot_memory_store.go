package daemon

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/memory"
)

func (d *Daemon) configureMemoryStore(state *bootState) error {
	state.globalMemoryDir = strings.TrimSpace(state.cfg.Memory.GlobalDir)
	if state.globalMemoryDir == "" {
		state.globalMemoryDir = d.homePaths.MemoryDir
	}
	state.memoryStore = memory.NewStore(
		state.globalMemoryDir,
		memory.WithCatalogDatabasePath(d.homePaths.DatabaseFile),
		memory.WithRecallSignalRecorderConfig(state.cfg.Memory.Recall.Signals),
		memory.WithFileLimits(state.cfg.Memory.File),
	)
	if err := state.memoryStore.EnsureDirs(); err != nil {
		return fmt.Errorf("daemon: ensure memory store directories: %w", err)
	}
	return nil
}
