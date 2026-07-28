package runs

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// RunSummary is the public metadata view for one persisted run.
type RunSummary struct {
	RunID            string
	ParentRunID      string
	Status           string
	Mode             string
	IDE              string
	Model            string
	Speed            kinds.Speed
	SpeedResolutions map[int]kinds.SpeedResolution
	WorkspaceRoot    string
	StartedAt        time.Time
	EndedAt          *time.Time
	ArtifactsDir     string
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
	ctx := context.Background()
	cleanRoot := cleanWorkspaceRoot(workspaceRoot)
	summaries, err := client.ListRuns(ctx, cleanRoot, opts)
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
	for i := range filtered {
		hydrated, hydrateErr := hydrateRunSummaryDetails(ctx, client, cleanRoot, filtered[i])
		if hydrateErr != nil {
			return nil, hydrateErr
		}
		filtered[i] = hydrated
	}
	return filtered, nil
}

func hydrateRunSummaryDetails(
	ctx context.Context,
	client daemonRunReader,
	workspaceRoot string,
	summary RunSummary,
) (RunSummary, error) {
	detailed, err := client.OpenRun(ctx, workspaceRoot, summary.RunID)
	if err != nil {
		return RunSummary{}, fmt.Errorf("hydrate run summary %q: %w", summary.RunID, err)
	}

	summary.IDE = firstNonEmpty(detailed.IDE, summary.IDE)
	summary.Model = firstNonEmpty(detailed.Model, summary.Model)
	if detailed.Speed != "" {
		summary.Speed = detailed.Speed
	}
	if detailed.SpeedResolutions != nil {
		summary.SpeedResolutions = cloneSpeedResolutions(detailed.SpeedResolutions)
	}
	return summary, nil
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
