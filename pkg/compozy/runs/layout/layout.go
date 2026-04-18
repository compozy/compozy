// Package layout exports the compatibility on-disk layout for one persisted
// Compozy run.
//
// Internal writers still use these constants, and the public package continues
// to expose them for callers that need the compatibility artifact names, even
// though run reading itself is now daemon-backed.
package layout

import "path/filepath"

// File and directory names that live under one run directory.
const (
	// RunDBFileName is the basename of the per-run SQLite operational store.
	RunDBFileName = "run.db"
	// RunMetaFileName is the basename of the per-run metadata file written by
	// the journal and read by the public reader.
	RunMetaFileName = "run.json"
	// EventsLogFileName is the basename of the append-only event log inside
	// the run directory.
	EventsLogFileName = "events.jsonl"
	// RunResultFileName is the basename of the terminal run result document.
	RunResultFileName = "result.json"
	// JobsDirName is the basename of the subdirectory that holds per-job
	// artifacts (prompt, stdout, stderr).
	JobsDirName = "jobs"
	// TurnsDirName is the basename of the subdirectory that holds per-turn
	// transcript artifacts.
	TurnsDirName = "turns"
)

// RunDBPath returns the absolute path to the run database file inside runDir.
func RunDBPath(runDir string) string { return filepath.Join(runDir, RunDBFileName) }

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
