package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrIncompatibleSchemaVersion reports an event schema the reader cannot decode.
	ErrIncompatibleSchemaVersion = errors.New("runs: incompatible schema version")
	// ErrPartialEventLine reports a truncated final JSON line in events.jsonl.
	ErrPartialEventLine = errors.New("runs: partial final event line")
)

const (
	publicRunStatusRunning   = "running"
	publicRunStatusCompleted = "completed"
	publicRunStatusFailed    = "failed"
	publicRunStatusCancelled = "cancel" + "led"
)

// SchemaVersionError reports an unsupported event schema version.
type SchemaVersionError struct {
	Version string
}

// Error implements the error interface.
func (e *SchemaVersionError) Error() string {
	if e == nil {
		return ErrIncompatibleSchemaVersion.Error()
	}
	return fmt.Sprintf("%s %q", ErrIncompatibleSchemaVersion.Error(), e.Version)
}

// Unwrap exposes the sentinel error for errors.Is checks.
func (e *SchemaVersionError) Unwrap() error {
	return ErrIncompatibleSchemaVersion
}

// Run is a handle over one on-disk run.
type Run struct {
	summary RunSummary

	runDir      string
	runMetaPath string
	eventsPath  string
	resultPath  string
	jobsDir     string
	turnsDir    string
}

type runRecord struct {
	Version       int       `json:"version,omitempty"`
	RunID         string    `json:"run_id"`
	Status        string    `json:"status"`
	Mode          string    `json:"mode"`
	IDE           string    `json:"ide"`
	Model         string    `json:"model"`
	WorkspaceRoot string    `json:"workspace_root"`
	ArtifactsDir  string    `json:"artifacts_dir"`
	EventsPath    string    `json:"events_path,omitempty"`
	TurnsDir      string    `json:"turns_dir,omitempty"`
	ResultPath    string    `json:"result_path,omitempty"`
	JobsDir       string    `json:"jobs_dir,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type resultRecord struct {
	Status string `json:"status"`
}

type runPaths struct {
	runDir      string
	runMetaPath string
	eventsPath  string
	resultPath  string
	jobsDir     string
	turnsDir    string
}

type derivedRunState struct {
	status  string
	endedAt *time.Time
}

// Open loads one run and prepares replay access to its event log.
func Open(workspaceRoot, runID string) (*Run, error) {
	return loadRun(workspaceRoot, runID)
}

func loadRun(workspaceRoot, runID string) (*Run, error) {
	cleanRoot := cleanWorkspaceRoot(workspaceRoot)
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return nil, errors.New("open run: missing run id")
	}

	paths := defaultRunPaths(cleanRoot, trimmedRunID)
	payload, err := os.ReadFile(paths.runMetaPath)
	if err != nil {
		return nil, fmt.Errorf("open run %q metadata: %w", trimmedRunID, err)
	}

	var record runRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, fmt.Errorf("decode run %q metadata: %w", trimmedRunID, err)
	}

	paths = resolveRunPaths(cleanRoot, trimmedRunID, record)
	summary := normalizeRunSummary(cleanRoot, trimmedRunID, paths.runDir, record)
	derived, err := deriveRunState(paths)
	if err != nil {
		return nil, err
	}
	if summary.Status == "" {
		summary.Status = derived.status
	}
	if summary.Status == "" {
		summary.Status = defaultRunStatus()
	}
	if summary.EndedAt == nil {
		summary.EndedAt = derived.endedAt
	}

	return &Run{
		summary:     summary,
		runDir:      paths.runDir,
		runMetaPath: paths.runMetaPath,
		eventsPath:  paths.eventsPath,
		resultPath:  paths.resultPath,
		jobsDir:     paths.jobsDir,
		turnsDir:    paths.turnsDir,
	}, nil
}

// Summary returns the loaded run metadata.
func (r *Run) Summary() RunSummary {
	if r == nil {
		return RunSummary{}
	}
	return r.summary
}

func defaultRunPaths(workspaceRoot, runID string) runPaths {
	runDir := filepath.Join(runsDirForWorkspace(workspaceRoot), runID)
	return runPaths{
		runDir:      runDir,
		runMetaPath: filepath.Join(runDir, "run.json"),
		eventsPath:  filepath.Join(runDir, "events.jsonl"),
		resultPath:  filepath.Join(runDir, "result.json"),
		jobsDir:     filepath.Join(runDir, "jobs"),
		turnsDir:    filepath.Join(runDir, "turns"),
	}
}

func resolveRunPaths(workspaceRoot, runID string, record runRecord) runPaths {
	paths := defaultRunPaths(workspaceRoot, runID)
	paths.eventsPath = resolveRunArtifactPath(workspaceRoot, paths.eventsPath, record.EventsPath)
	paths.resultPath = resolveRunArtifactPath(workspaceRoot, paths.resultPath, record.ResultPath)
	paths.jobsDir = resolveRunArtifactPath(workspaceRoot, paths.jobsDir, record.JobsDir)
	paths.turnsDir = resolveRunArtifactPath(workspaceRoot, paths.turnsDir, record.TurnsDir)
	return paths
}

func resolveRunArtifactPath(workspaceRoot, fallback, candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	switch {
	case trimmed == "":
		return fallback
	case filepath.IsAbs(trimmed):
		return trimmed
	default:
		return filepath.Join(workspaceRoot, trimmed)
	}
}

func normalizeRunSummary(workspaceRoot, runID, runDir string, record runRecord) RunSummary {
	summary := RunSummary{
		RunID:         runID,
		Status:        normalizeStatus(record.Status),
		Mode:          strings.TrimSpace(record.Mode),
		IDE:           strings.TrimSpace(record.IDE),
		Model:         strings.TrimSpace(record.Model),
		WorkspaceRoot: strings.TrimSpace(record.WorkspaceRoot),
		ArtifactsDir:  strings.TrimSpace(record.ArtifactsDir),
	}

	if trimmed := strings.TrimSpace(record.RunID); trimmed != "" {
		summary.RunID = trimmed
	}
	if summary.WorkspaceRoot == "" {
		summary.WorkspaceRoot = workspaceRoot
	}
	if summary.ArtifactsDir == "" {
		summary.ArtifactsDir = runDir
	} else if !filepath.IsAbs(summary.ArtifactsDir) {
		summary.ArtifactsDir = filepath.Join(summary.WorkspaceRoot, summary.ArtifactsDir)
	}

	switch {
	case !record.CreatedAt.IsZero():
		summary.StartedAt = record.CreatedAt
	case !record.UpdatedAt.IsZero():
		summary.StartedAt = record.UpdatedAt
	}
	if isTerminalStatus(summary.Status) && !record.UpdatedAt.IsZero() {
		endedAt := record.UpdatedAt
		summary.EndedAt = &endedAt
	}
	return summary
}
