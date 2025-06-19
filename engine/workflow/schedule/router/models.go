package schedulerouter

import (
	"time"

	"github.com/compozy/compozy/engine/workflow"
)

// ScheduleInfoResponse represents the API response for schedule information
type ScheduleInfoResponse struct {
	WorkflowID    string             `json:"workflow_id"               example:"daily-report"`
	ScheduleID    string             `json:"schedule_id"               example:"schedule-my-project-daily-report"`
	Cron          string             `json:"cron"                      example:"0 0 9 * * 1-5"`
	Timezone      string             `json:"timezone"                  example:"America/New_York"`
	Enabled       bool               `json:"enabled"                   example:"true"`
	IsOverride    bool               `json:"is_override"               example:"false"`
	YAMLConfig    *workflow.Schedule `json:"yaml_config,omitempty"`
	NextRunTime   *time.Time         `json:"next_run_time,omitempty"   example:"2024-01-15T09:00:00-05:00"`
	LastRunTime   *time.Time         `json:"last_run_time,omitempty"   example:"2024-01-14T09:00:00-05:00"`
	LastRunStatus string             `json:"last_run_status,omitempty" example:"success"`
}

// ScheduleListResponse represents the API response for listing schedules
type ScheduleListResponse struct {
	Schedules []ScheduleInfoResponse `json:"schedules"`
	Total     int                    `json:"total"     example:"5"`
}

// UpdateScheduleRequest represents the API request for updating a schedule
type UpdateScheduleRequest struct {
	Enabled *bool   `json:"enabled" example:"false"`
	Cron    *string `json:"cron"    example:"0 0 */10 * * *"`
}
