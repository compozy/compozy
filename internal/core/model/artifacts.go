package model

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/pkg/compozy/runs/layout"
)

const defaultRunID = "run"

type RunArtifacts struct {
	RunID       string
	RunDir      string
	RunDBPath   string
	RunMetaPath string
	EventsPath  string
	TurnsDir    string
	JobsDir     string
	ResultPath  string
}

type JobArtifacts struct {
	PromptPath string
	OutLogPath string
	ErrLogPath string
}

func NewRunArtifacts(workspaceRoot, runID string) RunArtifacts {
	return NewRunArtifactsForRunsDir(RunsBaseDirForWorkspace(workspaceRoot), runID)
}

func NewRunArtifactsForRunsDir(runsDir, runID string) RunArtifacts {
	safeRunID := sanitizeRunID(runID)
	runDir := filepath.Join(strings.TrimSpace(runsDir), safeRunID)
	return RunArtifacts{
		RunID:       safeRunID,
		RunDir:      runDir,
		RunDBPath:   layout.RunDBPath(runDir),
		RunMetaPath: layout.RunMetaPath(runDir),
		EventsPath:  layout.EventsLogPath(runDir),
		TurnsDir:    layout.TurnsDir(runDir),
		JobsDir:     layout.JobsDir(runDir),
		ResultPath:  layout.ResultPath(runDir),
	}
}

func ResolveHomeRunArtifacts(runID string) (RunArtifacts, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return RunArtifacts{}, fmt.Errorf("resolve home run artifacts: %w", err)
	}
	return NewRunArtifactsForRunsDir(homePaths.RunsDir, runID), nil
}

func ResolvePersistedRunArtifacts(workspaceRoot, runID string) (RunArtifacts, error) {
	workspaceArtifacts := NewRunArtifacts(workspaceRoot, runID)
	if _, err := os.Stat(workspaceArtifacts.RunMetaPath); err == nil {
		return workspaceArtifacts, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RunArtifacts{}, fmt.Errorf("stat workspace run metadata: %w", err)
	}

	homeArtifacts, err := ResolveHomeRunArtifacts(runID)
	if err != nil {
		return RunArtifacts{}, err
	}
	return homeArtifacts, nil
}

func sanitizeRunID(runID string) string {
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return "run"
	}
	normalized := strings.NewReplacer("/", "-", "\\", "-").Replace(trimmed)

	var builder strings.Builder
	builder.Grow(len(normalized))
	lastDash := false
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			builder.WriteRune(r)
			lastDash = false
		case r == '-':
			builder.WriteRune(r)
			lastDash = true
		default:
			if lastDash {
				continue
			}
			builder.WriteByte('-')
			lastDash = true
		}
	}

	safe := strings.Trim(builder.String(), "-")
	switch safe {
	case "", ".", "..":
		return defaultRunID
	default:
		return safe
	}
}

func (artifacts RunArtifacts) JobArtifacts(safeName string) JobArtifacts {
	sanitizedName := sanitizeJobArtifactName(safeName)
	return JobArtifacts{
		PromptPath: filepath.Join(artifacts.JobsDir, sanitizedName+".prompt.md"),
		OutLogPath: filepath.Join(artifacts.JobsDir, sanitizedName+".out.log"),
		ErrLogPath: filepath.Join(artifacts.JobsDir, sanitizedName+".err.log"),
	}
}

func sanitizeJobArtifactName(name string) string {
	safe := strings.TrimLeft(sanitizeRunID(name), ".-")
	if safe == "" {
		return "job"
	}
	return safe
}
