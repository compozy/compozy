package runs

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

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

// ListOptions filters the runs returned by List.
type ListOptions struct {
	Status []string
	Mode   []string
	Since  time.Time
	Until  time.Time
	Limit  int
}

// List enumerates runs under workspaceRoot's .compozy/runs directory.
func List(workspaceRoot string, opts ListOptions) ([]RunSummary, error) {
	cleanRoot := cleanWorkspaceRoot(workspaceRoot)
	runsDir := runsDirForWorkspace(cleanRoot)

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	summaries := make([]RunSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runID := entry.Name()
		runMetaPath := filepath.Join(runsDir, runID, "run.json")
		run, err := loadRun(cleanRoot, runID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
				slog.Warn(
					"skipping run without run.json",
					"component", "runs",
					"run_id", runID,
					"path", runMetaPath,
				)
				continue
			}
			return nil, err
		}
		summary := run.Summary()
		if !matchesListOptions(summary, opts) {
			continue
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].StartedAt.Equal(summaries[j].StartedAt) {
			return summaries[i].RunID > summaries[j].RunID
		}
		return summaries[i].StartedAt.After(summaries[j].StartedAt)
	})

	if opts.Limit > 0 && len(summaries) > opts.Limit {
		summaries = summaries[:opts.Limit]
	}
	return summaries, nil
}

func matchesListOptions(summary RunSummary, opts ListOptions) bool {
	if len(opts.Status) > 0 {
		if !slices.ContainsFunc(opts.Status, func(candidate string) bool {
			return normalizeStatus(candidate) == normalizeStatus(summary.Status)
		}) {
			return false
		}
	}
	if len(opts.Mode) > 0 {
		if !slices.ContainsFunc(opts.Mode, func(candidate string) bool {
			return strings.EqualFold(strings.TrimSpace(candidate), summary.Mode)
		}) {
			return false
		}
	}
	if !opts.Since.IsZero() && summary.StartedAt.Before(opts.Since) {
		return false
	}
	if !opts.Until.IsZero() && summary.StartedAt.After(opts.Until) {
		return false
	}
	return true
}

func cleanWorkspaceRoot(workspaceRoot string) string {
	trimmed := strings.TrimSpace(workspaceRoot)
	if trimmed == "" || trimmed == "." {
		return ""
	}
	return filepath.Clean(trimmed)
}

func runsDirForWorkspace(workspaceRoot string) string {
	return filepath.Join(cleanWorkspaceRoot(workspaceRoot), ".compozy", "runs")
}
