package runs

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// RunSummary is the public metadata view for one persisted run.
type RunSummary struct {
	RunID         string
	ParentRunID   string
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

// List enumerates daemon-managed runs for the supplied workspace root.
func List(workspaceRoot string, opts ListOptions) ([]RunSummary, error) {
	client, err := resolveRunsDaemonReader()
	if err != nil {
		return nil, err
	}
	summaries, err := client.ListRuns(context.Background(), cleanWorkspaceRoot(workspaceRoot), opts)
	if err != nil {
		return nil, err
	}
	filtered := make([]RunSummary, 0, len(summaries))
	for i := range summaries {
		if !matchesListOptions(summaries[i], opts) {
			continue
		}
		filtered = append(filtered, summaries[i])
	}
	sortRunSummaries(filtered)
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered, nil
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
