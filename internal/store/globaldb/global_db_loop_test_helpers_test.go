package globaldb

import (
	"path/filepath"
	"strings"
	"testing"

	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func openLoopTestGlobalDB(t *testing.T, workspaceIDs ...string) *GlobalDB {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	if len(workspaceIDs) == 0 {
		workspaceIDs = []string{"ws-1"}
	}
	seedLoopTestWorkspaces(t, globalDB, workspaceIDs...)
	return globalDB
}

func seedLoopTestWorkspaces(t *testing.T, globalDB *GlobalDB, workspaceIDs ...string) {
	t.Helper()

	seen := make(map[string]struct{}, len(workspaceIDs))
	for _, workspaceID := range workspaceIDs {
		trimmedID := strings.TrimSpace(workspaceID)
		if trimmedID == "" {
			continue
		}
		if _, ok := seen[trimmedID]; ok {
			continue
		}
		seen[trimmedID] = struct{}{}
		insertWorkspaceForGlobalTests(t, globalDB, aghworkspace.Workspace{
			ID:      trimmedID,
			RootDir: filepath.Join(t.TempDir(), trimmedID),
			Name:    trimmedID,
		})
	}
}
