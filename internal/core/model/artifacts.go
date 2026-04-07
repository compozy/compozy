package model

import (
	"path/filepath"
	"strings"
)

type RunArtifacts struct {
	RunID       string
	RunDir      string
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
	safeRunID := sanitizeRunID(runID)
	runDir := filepath.Join(RunsBaseDirForWorkspace(workspaceRoot), safeRunID)
	return RunArtifacts{
		RunID:       safeRunID,
		RunDir:      runDir,
		RunMetaPath: filepath.Join(runDir, "run.json"),
		EventsPath:  filepath.Join(runDir, "events.jsonl"),
		TurnsDir:    filepath.Join(runDir, "turns"),
		JobsDir:     filepath.Join(runDir, "jobs"),
		ResultPath:  filepath.Join(runDir, "result.json"),
	}
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
		return "run"
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
