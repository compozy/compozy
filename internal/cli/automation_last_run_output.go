package cli

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

const (
	lastRunLabel        = "Last Run"
	lastRunStatusKey    = "last_run_status"
	lastRunStartedAtKey = "last_run_started_at"
)

func formatAutomationLastRun(run *contract.AutomationLastRunPayload, now func() time.Time) string {
	if run == nil {
		return "—"
	}
	status := string(run.Status)
	if run.Status == automationpkg.RunCancelled && run.SkipReason != "" {
		status = "skipped"
	}
	if run.StartedAt == nil {
		return status
	}
	return status + " " + formatAge(now, *run.StartedAt) + " ago"
}

func automationLastRunStatus(run *contract.AutomationLastRunPayload) string {
	if run == nil {
		return ""
	}
	return string(run.Status)
}

func automationLastRunStartedAt(run *contract.AutomationLastRunPayload) string {
	if run == nil {
		return ""
	}
	return formatOptionalTime(run.StartedAt)
}
