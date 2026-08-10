package desktoprelease

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var migrationStreams = map[string]string{
	schemaStreamGlobal:    "internal/store/globaldb/schema/migrations",
	schemaStreamMemory:    "internal/memory/schema/migrations",
	schemaStreamSession:   "internal/store/sessiondb/schema/migrations",
	schemaStreamWorkspace: "internal/store/workspacedb/schema/migrations",
}

func DiscoverSchemaHeads(ctx context.Context, repoRoot string) (map[string]uint64, error) {
	heads := make(map[string]uint64, len(migrationStreams))
	for stream, relativeDir := range migrationStreams {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("desktop release: discover schema heads: %w", err)
		}
		entries, err := os.ReadDir(filepath.Join(repoRoot, relativeDir))
		if err != nil {
			return nil, fmt.Errorf("desktop release: read %s migrations: %w", stream, err)
		}
		var head uint64
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			prefix, _, found := strings.Cut(entry.Name(), "_")
			if !found {
				continue
			}
			version, parseErr := strconv.ParseUint(prefix, 10, 64)
			if parseErr != nil {
				continue
			}
			if version > head {
				head = version
			}
		}
		if head == 0 {
			return nil, fmt.Errorf("desktop release: %s migration stream has no versioned SQL files", stream)
		}
		heads[stream] = head
	}
	return heads, nil
}
