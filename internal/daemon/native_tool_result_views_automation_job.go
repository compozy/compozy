package daemon

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

type compactJobRecord struct {
	ID          string                             `json:"id"`
	ProfileID   string                             `json:"profile_id"`
	ProfileName string                             `json:"profile_name"`
	Scope       automationpkg.Scope                `json:"scope"`
	Name        string                             `json:"name"`
	TargetKind  automationpkg.TargetKind           `json:"target_kind"`
	AgentName   string                             `json:"agent_name"`
	WorkspaceID string                             `json:"workspace_id,omitempty"`
	Schedule    *automationpkg.ScheduleSpec        `json:"schedule,omitzero"`
	Enabled     bool                               `json:"enabled"`
	Source      automationpkg.JobSource            `json:"source"`
	NextRun     *time.Time                         `json:"next_run,omitempty"`
	LastRun     *contract.AutomationLastRunPayload `json:"last_run,omitzero"`
	LoopTarget  *compactAutomationLoopTarget       `json:"loop_target,omitzero"`
	Task        *compactJobTask                    `json:"task,omitzero"`
	Scheduler   *compactJobScheduler               `json:"scheduler,omitzero"`
}

func compactJob(p contract.JobPayload) compactJobRecord {
	result := compactJobRecord{
		ID:          p.ID,
		ProfileID:   p.ProfileID,
		ProfileName: p.ProfileName,
		Scope:       p.Scope,
		Name:        p.Name,
		TargetKind:  p.TargetKind,
		AgentName:   p.AgentName,
		WorkspaceID: p.WorkspaceID,
		Schedule:    p.Schedule,
		Enabled:     p.Enabled,
		Source:      p.Source,
		NextRun:     p.NextRun,
		LastRun:     p.LastRun,
	}
	if p.LoopTarget != nil {
		value := compactAutomationTarget(*p.LoopTarget)
		result.LoopTarget = &value
	}
	if p.Task != nil {
		value := compactTask(*p.Task)
		result.Task = &value
	}
	if p.Scheduler != nil {
		value := compactScheduler(*p.Scheduler)
		result.Scheduler = &value
	}
	return result
}

type compactAutomationLoopTarget struct {
	WorkspaceID string `json:"workspace_id"`
	LoopName    string `json:"loop_name"`
}

func compactAutomationTarget(p automationpkg.LoopTarget) compactAutomationLoopTarget {
	return compactAutomationLoopTarget{
		WorkspaceID: p.WorkspaceID,
		LoopName:    p.LoopName,
	}
}

type compactJobTask struct {
	Title string `json:"title,omitempty"`
}

func compactTask(p automationpkg.JobTaskConfig) compactJobTask {
	return compactJobTask{
		Title: p.Title,
	}
}

type compactJobScheduler struct {
	Registered                bool       `json:"registered"`
	ConsecutiveResumeFailures int        `json:"consecutive_resume_failures,omitzero"`
	MisfireCount              int        `json:"misfire_count,omitzero"`
	LastMisfireAt             *time.Time `json:"last_misfire_at,omitempty"`
}

func compactScheduler(p contract.AutomationSchedulerStatePayload) compactJobScheduler {
	return compactJobScheduler{
		Registered:                p.Registered,
		ConsecutiveResumeFailures: p.ConsecutiveResumeFailures,
		MisfireCount:              p.MisfireCount,
		LastMisfireAt:             p.LastMisfireAt,
	}
}

var jobRecordViews = resultViews[contract.JobPayload]{
	Compact: func(p contract.JobPayload) any { return map[string]any{nativeAutomationToolsJobKey: compactJob(p)} },
	Full:    func(p contract.JobPayload) any { return map[string]any{nativeAutomationToolsJobKey: p} },
	Preview: func(p contract.JobPayload) string { return p.ID },
}

var jobListViews = resultViews[contract.JobsResponse]{
	Compact: func(p contract.JobsResponse) any {
		var rows []compactJobRecord
		if p.Jobs != nil {
			rows = make([]compactJobRecord, len(p.Jobs))
			for i, value := range p.Jobs {
				rows[i] = compactJob(value)
			}
		}
		return map[string]any{"jobs": rows, "page": p.Page}
	},
	Preview: func(p contract.JobsResponse) string {
		return fmt.Sprintf("%d of %d automation jobs", len(p.Jobs), p.Page.Total)
	},
}
