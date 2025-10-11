package orchestrator

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
)

func readRunEvents(t *testing.T, projectRoot string) []map[string]any {
	t.Helper()
	storeDir := core.GetStoreDir(projectRoot)
	runDir := filepath.Join(storeDir, "llm_runs")
	entries, err := os.ReadDir(runDir)
	require.NoError(t, err)
	require.NotZero(t, len(entries), "no telemetry run recorded")

	events := make([]map[string]any, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(runDir, entry.Name())
		file, err := os.Open(path)
		require.NoError(t, err)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var evt map[string]any
			require.NoError(t, json.Unmarshal(line, &evt))
			events = append(events, evt)
		}
		require.NoError(t, scanner.Err())
		_ = file.Close()
	}
	return events
}

func findEventByStage(events []map[string]any, stage string) (map[string]any, bool) {
	for _, evt := range events {
		if s, ok := evt["stage"].(string); ok && s == stage {
			return evt, true
		}
	}
	return nil, false
}
