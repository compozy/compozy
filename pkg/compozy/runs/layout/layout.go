// Package layout exports the on-disk layout for one persisted Compozy run.
//
// Both the internal writer ([github.com/compozy/compozy/internal/core/model])
// and the public reader ([github.com/compozy/compozy/pkg/compozy/runs])
// depend on these constants so that renaming a run artifact is a single-place
// change visible to the type checker, not an agree-by-string contract.
package layout

import "path/filepath"

// File and directory names that live under one run directory.
const (
	RunMetaFileName   = "run.json"
	EventsLogFileName = "events.jsonl"
	RunResultFileName = "result.json"
	JobsDirName       = "jobs"
	TurnsDirName      = "turns"
)

// RunMetaPath returns the absolute path to the run metadata file inside runDir.
func RunMetaPath(runDir string) string { return filepath.Join(runDir, RunMetaFileName) }

// EventsLogPath returns the absolute path to the events log inside runDir.
func EventsLogPath(runDir string) string { return filepath.Join(runDir, EventsLogFileName) }

// ResultPath returns the absolute path to the result file inside runDir.
func ResultPath(runDir string) string { return filepath.Join(runDir, RunResultFileName) }

// JobsDir returns the absolute path to the jobs subdirectory inside runDir.
func JobsDir(runDir string) string { return filepath.Join(runDir, JobsDirName) }

// TurnsDir returns the absolute path to the turns subdirectory inside runDir.
func TurnsDir(runDir string) string { return filepath.Join(runDir, TurnsDirName) }
