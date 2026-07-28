package automation

import (
	"context"
	"errors"
	"strings"
)

const starterSuggestionCatalogVersion = "v1"

type suggestionCatalogEntry struct {
	key      string
	name     string
	prompt   string
	cronExpr string
}

var starterSuggestionCatalog = []suggestionCatalogEntry{
	{
		key:  "daily-workspace-briefing",
		name: "Daily workspace briefing",
		prompt: "Review recent workspace activity and prepare a concise briefing with priorities, " +
			"blockers, and next actions.",
		cronExpr: "0 8 * * *",
	},
	{
		key:      "weekday-standup-draft",
		name:     "Weekday standup draft",
		prompt:   "Review current workspace work and draft a standup update covering progress, plans, and blockers.",
		cronExpr: "0 9 * * 1-5",
	},
	{
		key:  "weekly-project-review",
		name: "Weekly project review",
		prompt: "Review the workspace projects and prepare a weekly summary of outcomes, risks, " +
			"decisions, and next priorities.",
		cronExpr: "0 16 * * 5",
	},
	{
		key:  "workspace-maintenance-scan",
		name: "Workspace maintenance scan",
		prompt: "Inspect the workspace for maintenance needs and report stale tasks, risky dependencies, " +
			"failing checks, and recommended cleanup.",
		cronExpr: "0 10 * * 1",
	},
}

func (m *Manager) ensureStarterSuggestions(
	ctx context.Context,
	workspaceID string,
	agentName string,
) error {
	for _, entry := range starterSuggestionCatalog {
		dedupKey := starterSuggestionDedupKey(entry.key)
		_, err := m.store.CreateSuggestion(ctx, Suggestion{
			ID:          stableConfigID("sugcat", workspaceID, dedupKey),
			WorkspaceID: workspaceID,
			Source:      SuggestionSourceCatalog,
			DedupKey:    dedupKey,
			Status:      SuggestionStatusPending,
			Payload: Job{
				ID:          suggestionJobID(workspaceID, dedupKey),
				Scope:       AutomationScopeWorkspace,
				Name:        entry.name,
				TargetKind:  TargetKindAgent,
				AgentName:   agentName,
				WorkspaceID: workspaceID,
				Prompt:      entry.prompt,
				Schedule: &ScheduleSpec{
					Mode:          ScheduleModeCron,
					Expr:          entry.cronExpr,
					CatchUpPolicy: SchedulerCatchUpPolicyRunOnce,
				},
				Enabled:   true,
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceDynamic,
			},
		}, m.config.Suggestions.PendingCap)
		if errors.Is(err, ErrSuggestionPendingCap) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func starterSuggestionDedupKey(key string) string {
	return "catalog:" + starterSuggestionCatalogVersion + ":" + strings.TrimSpace(key)
}

func suggestionJobID(workspaceID string, dedupKey string) string {
	return stableConfigID("jobsug", workspaceID, dedupKey)
}
