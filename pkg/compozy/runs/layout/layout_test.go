package layout

import (
	"path/filepath"
	"testing"
)

// TestConstantsAreStable guards against silent renames. Changing any of these
// strings is a breaking change to the public pkg/compozy/runs API and to the
// internal writer in internal/core/model; if a constant must change, update
// this test deliberately and call out the rename in the PR description.
func TestConstantsAreStable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"RunMetaFileName", RunMetaFileName, "run.json"},
		{"EventsLogFileName", EventsLogFileName, "events.jsonl"},
		{"RunResultFileName", RunResultFileName, "result.json"},
		{"JobsDirName", JobsDirName, "jobs"},
		{"TurnsDirName", TurnsDirName, "turns"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestHelpersJoinUnderRunDir(t *testing.T) {
	t.Parallel()
	runDir := filepath.Join("ws", ".compozy", "runs", "run-1")
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"RunMetaPath", RunMetaPath(runDir), filepath.Join(runDir, RunMetaFileName)},
		{"EventsLogPath", EventsLogPath(runDir), filepath.Join(runDir, EventsLogFileName)},
		{"ResultPath", ResultPath(runDir), filepath.Join(runDir, RunResultFileName)},
		{"JobsDir", JobsDir(runDir), filepath.Join(runDir, JobsDirName)},
		{"TurnsDir", TurnsDir(runDir), filepath.Join(runDir, TurnsDirName)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}
