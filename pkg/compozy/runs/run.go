package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

var (
	// ErrIncompatibleSchemaVersion reports an event schema the reader cannot decode.
	ErrIncompatibleSchemaVersion = errors.New("runs: incompatible schema version")
	// ErrPartialEventLine reports a truncated final JSON line in events.jsonl.
	ErrPartialEventLine = errors.New("runs: partial final event line")
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

// RunSummary is the public metadata view for one persisted run.
type RunSummary struct {
	RunID         string
	Status        string
	Mode          string
	IDE           string
	Model         string
	WorkspaceRoot string
	StartedAt     time.Time
	EndedAt       *time.Time
	ArtifactsDir  string
}

// Run is a handle over one on-disk run.
type Run struct {
	summary    RunSummary
	eventsPath string
}

type runRecord struct {
	RunID         string    `json:"run_id"`
	Status        string    `json:"status"`
	Mode          string    `json:"mode"`
	IDE           string    `json:"ide"`
	Model         string    `json:"model"`
	WorkspaceRoot string    `json:"workspace_root"`
	ArtifactsDir  string    `json:"artifacts_dir"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Open loads one run's metadata and prepares replay access to its event log.
func Open(workspaceRoot, runID string) (*Run, error) {
	cleanRoot := filepath.Clean(strings.TrimSpace(workspaceRoot))
	if cleanRoot == "." {
		cleanRoot = ""
	}
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return nil, errors.New("open run: missing run id")
	}

	runDir := filepath.Join(cleanRoot, ".compozy", "runs", trimmedRunID)
	payload, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		return nil, fmt.Errorf("open run metadata: %w", err)
	}

	var record runRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, fmt.Errorf("decode run metadata: %w", err)
	}

	summary := normalizeRunSummary(cleanRoot, trimmedRunID, runDir, record)
	return &Run{
		summary:    summary,
		eventsPath: filepath.Join(runDir, "events.jsonl"),
	}, nil
}

// Summary returns the loaded run metadata.
func (r *Run) Summary() RunSummary {
	if r == nil {
		return RunSummary{}
	}
	return r.summary
}

// Replay yields all events from fromSeq to EOF, tolerating a truncated final line.
func (r *Run) Replay(fromSeq uint64) iter.Seq2[events.Event, error] {
	return func(yield func(events.Event, error) bool) {
		if r == nil {
			yield(events.Event{}, errors.New("replay run: nil run"))
			return
		}

		file, err := os.Open(r.eventsPath)
		if err != nil {
			yield(events.Event{}, fmt.Errorf("open run events: %w", err))
			return
		}
		defer func() {
			_ = file.Close()
		}()

		partialFinalLine, err := fileEndsWithNewline(file)
		if err != nil {
			yield(events.Event{}, fmt.Errorf("inspect run events: %w", err))
			return
		}

		scanner := newEventScanner(file)
		var (
			pendingDecodeErr error
			pendingLine      int
		)

		for lineNumber := 1; scanner.Scan(); lineNumber++ {
			if pendingDecodeErr != nil {
				yield(events.Event{}, pendingDecodeErr)
				return
			}

			line := bytesTrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}

			var ev events.Event
			if err := json.Unmarshal(line, &ev); err != nil {
				pendingDecodeErr = fmt.Errorf("decode run event line %d: %w", lineNumber, err)
				pendingLine = lineNumber
				continue
			}
			if err := validateSchemaVersion(ev.SchemaVersion); err != nil {
				yield(events.Event{}, err)
				return
			}
			if ev.Seq < fromSeq {
				continue
			}
			if !yield(ev, nil) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			yield(events.Event{}, fmt.Errorf("scan run events: %w", err))
			return
		}
		if pendingDecodeErr == nil {
			return
		}
		if partialFinalLine {
			yield(events.Event{}, fmt.Errorf("%w: line %d", ErrPartialEventLine, pendingLine))
			return
		}
		yield(events.Event{}, pendingDecodeErr)
	}
}

func normalizeRunSummary(workspaceRoot, runID, runDir string, record runRecord) RunSummary {
	summary := RunSummary{
		RunID:         runID,
		Status:        strings.TrimSpace(record.Status),
		Mode:          strings.TrimSpace(record.Mode),
		IDE:           strings.TrimSpace(record.IDE),
		Model:         strings.TrimSpace(record.Model),
		WorkspaceRoot: strings.TrimSpace(record.WorkspaceRoot),
		ArtifactsDir:  strings.TrimSpace(record.ArtifactsDir),
	}

	if strings.TrimSpace(record.RunID) != "" {
		summary.RunID = strings.TrimSpace(record.RunID)
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

func validateSchemaVersion(version string) error {
	major, _, ok := strings.Cut(strings.TrimSpace(version), ".")
	if !ok || major == "" {
		return &SchemaVersionError{Version: version}
	}
	expectedMajor, _, _ := strings.Cut(events.SchemaVersion, ".")
	if major != expectedMajor {
		return &SchemaVersionError{Version: version}
	}
	return nil
}

func isTerminalStatus(status string) bool {
	normalized := strings.TrimSpace(status)
	switch normalized {
	case "completed", "succeeded", "failed", "canceled":
		return true
	default:
		return strings.HasPrefix(normalized, "cancel") && strings.HasSuffix(normalized, "ed")
	}
}

func fileEndsWithNewline(file *os.File) (bool, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() == 0 {
		return false, nil
	}

	var tail [1]byte
	if _, err := file.ReadAt(tail[:], info.Size()-1); err != nil {
		return false, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	return tail[0] != '\n', nil
}
