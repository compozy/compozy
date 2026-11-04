package daemon

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// migrateDaemonAgentKinds runs the retired agent-kind migration over daemon-owned AGENT.md files:
// exactly $COMPOZY_HOME/agents and $COMPOZY_HOME/profiles/<profile>/agents for every profile
// directory. It runs after profile recovery and before agent discovery and never blocks boot.
// Workspace-owned roots migrate when the workspace resolver scans them; extension roots belong to
// their packages and are never rewritten.
func migrateDaemonAgentKinds(homePaths compozyconfig.HomePaths, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	for _, root := range daemonAgentKindRewriteRoots(homePaths, logger) {
		if _, _, err := compozyconfig.MigrateRetiredAgentKinds(root, logger); err != nil {
			logger.Warn("agent kind rewrite skipped", "reason", string(compozyconfig.AgentKindRewriteSkipIO),
				"path", root, "error", err)
		}
	}
}

func daemonAgentKindRewriteRoots(homePaths compozyconfig.HomePaths, logger *slog.Logger) []string {
	roots := []string{homePaths.AgentsDir}
	entries, err := os.ReadDir(homePaths.ProfilesDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("agent kind rewrite skipped", "reason", string(compozyconfig.AgentKindRewriteSkipIO),
				"path", homePaths.ProfilesDir, "error", err)
		}
		return roots
	}
	for _, entry := range entries {
		profileDir := filepath.Join(homePaths.ProfilesDir, entry.Name())
		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			logger.Warn("agent kind rewrite skipped", "reason", string(compozyconfig.AgentKindRewriteSkipSymlink),
				"path", profileDir)
		case entry.IsDir():
			roots = append(roots, filepath.Join(profileDir, compozyconfig.AgentsDirName))
		}
	}
	return roots
}
