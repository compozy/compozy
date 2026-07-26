package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
)

// AutomationResourceStatusPayload reports total and enabled counts for one
// automation definition family.
type AutomationResourceStatusPayload struct {
	Total   int `json:"total"`
	Enabled int `json:"enabled"`
}

// AutomationSchedulerStatePayload exposes durable scheduler cursor diagnostics
// for one scheduled automation job.
type AutomationSchedulerStatePayload struct {
	JobID                     string                               `json:"job_id"`
	Registered                bool                                 `json:"registered"`
	NextRunAt                 *time.Time                           `json:"next_run_at,omitempty"`
	LastRunAt                 *time.Time                           `json:"last_run_at,omitempty"`
	LastScheduledAt           *time.Time                           `json:"last_scheduled_at,omitempty"`
	LastFireID                string                               `json:"last_fire_id,omitempty"`
	CatchUpPolicy             automationpkg.SchedulerCatchUpPolicy `json:"catch_up_policy,omitempty"`
	MisfireGraceSeconds       int                                  `json:"misfire_grace_seconds,omitempty"`
	ConsecutiveResumeFailures int                                  `json:"consecutive_resume_failures,omitempty"`
	LastMisfireAt             *time.Time                           `json:"last_misfire_at,omitempty"`
	MisfireCount              int                                  `json:"misfire_count,omitempty"`
	UpdatedAt                 *time.Time                           `json:"updated_at,omitempty"`
}

// AutomationHealthPayload describes additive automation health state inside
// the runtime status response.
type AutomationHealthPayload struct {
	Enabled          bool                              `json:"enabled"`
	Jobs             AutomationResourceStatusPayload   `json:"jobs"`
	Triggers         AutomationResourceStatusPayload   `json:"triggers"`
	SchedulerRunning bool                              `json:"scheduler_running"`
	NextFire         *time.Time                        `json:"next_fire,omitempty"`
	ScheduledJobs    []AutomationSchedulerStatePayload `json:"scheduled_jobs,omitempty"`
}

// JobPayload is the shared automation job response payload.
type JobPayload struct {
	ID          string                           `json:"id"`
	Scope       automationpkg.Scope              `json:"scope"`
	Name        string                           `json:"name"`
	TargetKind  automationpkg.TargetKind         `json:"target_kind"`
	AgentName   string                           `json:"agent_name"`
	WorkspaceID string                           `json:"workspace_id,omitempty"`
	Prompt      string                           `json:"prompt"`
	Schedule    *automationpkg.ScheduleSpec      `json:"schedule,omitempty"`
	Task        *automationpkg.JobTaskConfig     `json:"task,omitempty"`
	LoopTarget  *automationpkg.LoopTarget        `json:"loop_target,omitempty"`
	Enabled     bool                             `json:"enabled"`
	Retry       automationpkg.RetryConfig        `json:"retry"`
	FireLimit   automationpkg.FireLimitConfig    `json:"fire_limit"`
	Source      automationpkg.JobSource          `json:"source"`
	CreatedAt   time.Time                        `json:"created_at"`
	UpdatedAt   time.Time                        `json:"updated_at"`
	NextRun     *time.Time                       `json:"next_run,omitempty"`
	Scheduler   *AutomationSchedulerStatePayload `json:"scheduler,omitempty"`
}

// AutomationSuggestionPayload is a workspace-scoped, consent-first Job proposal.
type AutomationSuggestionPayload struct {
	ID          string                         `json:"id"`
	WorkspaceID string                         `json:"workspace_id"`
	Source      automationpkg.SuggestionSource `json:"source"`
	DedupKey    string                         `json:"dedup_key"`
	Status      automationpkg.SuggestionStatus `json:"status"`
	Payload     JobPayload                     `json:"payload"`
	CreatedAt   time.Time                      `json:"created_at"`
	ResolvedAt  *time.Time                     `json:"resolved_at,omitempty"`
}

// TriggerPayload is the shared automation trigger response payload.
type TriggerPayload struct {
	ID                   string                        `json:"id"`
	Scope                automationpkg.Scope           `json:"scope"`
	Name                 string                        `json:"name"`
	TargetKind           automationpkg.TargetKind      `json:"target_kind"`
	AgentName            string                        `json:"agent_name"`
	WorkspaceID          string                        `json:"workspace_id,omitempty"`
	Prompt               string                        `json:"prompt"`
	Event                string                        `json:"event"`
	Filter               map[string]string             `json:"filter,omitempty"`
	LoopTarget           *automationpkg.LoopTarget     `json:"loop_target,omitempty"`
	Enabled              bool                          `json:"enabled"`
	Retry                automationpkg.RetryConfig     `json:"retry"`
	FireLimit            automationpkg.FireLimitConfig `json:"fire_limit"`
	Source               automationpkg.JobSource       `json:"source"`
	WebhookID            string                        `json:"webhook_id,omitempty"`
	EndpointSlug         string                        `json:"endpoint_slug,omitempty"`
	WebhookSecretPresent bool                          `json:"webhook_secret_present"`
	WebhookSecretHash    string                        `json:"webhook_secret_hash,omitempty"`
	CreatedAt            time.Time                     `json:"created_at"`
	UpdatedAt            time.Time                     `json:"updated_at"`
}

// TriggerPayloadFromTrigger converts an internal automation trigger into the
// public redacted automation trigger payload.
func TriggerPayloadFromTrigger(trigger automationpkg.Trigger) TriggerPayload {
	webhookSecretPresent, webhookSecretHash := webhookSecretMetadataFromRef(trigger.WebhookSecretRef)
	return TriggerPayload{
		ID:                   trigger.ID,
		Scope:                trigger.Scope,
		Name:                 trigger.Name,
		TargetKind:           trigger.TargetKind,
		AgentName:            trigger.AgentName,
		WorkspaceID:          trigger.WorkspaceID,
		Prompt:               trigger.Prompt,
		Event:                trigger.Event,
		Filter:               cloneTriggerFilter(trigger.Filter),
		LoopTarget:           cloneLoopTarget(trigger.LoopTarget),
		Enabled:              trigger.Enabled,
		Retry:                trigger.Retry,
		FireLimit:            trigger.FireLimit,
		Source:               trigger.Source,
		WebhookID:            trigger.WebhookID,
		EndpointSlug:         trigger.EndpointSlug,
		WebhookSecretPresent: webhookSecretPresent,
		WebhookSecretHash:    webhookSecretHash,
		CreatedAt:            trigger.CreatedAt,
		UpdatedAt:            trigger.UpdatedAt,
	}
}

// TriggerPayloadsFromTriggers converts internal automation triggers into public
// redacted automation trigger payloads.
func TriggerPayloadsFromTriggers(triggers []automationpkg.Trigger) []TriggerPayload {
	payloads := make([]TriggerPayload, 0, len(triggers))
	for _, trigger := range triggers {
		payloads = append(payloads, TriggerPayloadFromTrigger(trigger))
	}
	return payloads
}

func webhookSecretMetadataFromRef(ref string) (bool, string) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return false, ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return true, "sha256:" + hex.EncodeToString(sum[:])
}

func cloneTriggerFilter(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}

// RunPayload is the shared automation run response payload.
type RunPayload struct {
	ID                   string                  `json:"id"`
	JobID                string                  `json:"job_id,omitempty"`
	TriggerID            string                  `json:"trigger_id,omitempty"`
	SessionID            string                  `json:"session_id,omitempty"`
	TaskID               string                  `json:"task_id,omitempty"`
	TaskRunID            string                  `json:"task_run_id,omitempty"`
	LoopRunID            string                  `json:"loop_run_id,omitempty"`
	FireID               string                  `json:"fire_id,omitempty"`
	Status               automationpkg.RunStatus `json:"status"`
	Attempt              int                     `json:"attempt"`
	NetworkParticipation *participation.Request  `json:"network_participation,omitempty"`
	ScheduledAt          *time.Time              `json:"scheduled_at,omitempty"`
	StartedAt            *time.Time              `json:"started_at,omitempty"`
	EndedAt              *time.Time              `json:"ended_at,omitempty"`
	Error                string                  `json:"error,omitempty"`
	DeliveryError        string                  `json:"delivery_error,omitempty"`
	DeliveryErrorAt      *time.Time              `json:"delivery_error_at,omitempty"`
	Metadata             map[string]any          `json:"metadata,omitempty"`
}

// WebhookDeliveryPayload is the shared webhook dispatch response payload.
type WebhookDeliveryPayload struct {
	Matched int          `json:"matched"`
	Runs    []RunPayload `json:"runs,omitempty"`
}

// CreateJobRequest is the shared automation job create payload.
type CreateJobRequest struct {
	Scope       automationpkg.Scope            `json:"scope"`
	Name        string                         `json:"name"`
	TargetKind  automationpkg.TargetKind       `json:"target_kind,omitempty"`
	AgentName   string                         `json:"agent_name"`
	WorkspaceID string                         `json:"workspace_id,omitempty"`
	Prompt      string                         `json:"prompt"`
	Schedule    automationpkg.ScheduleSpec     `json:"schedule"`
	Task        *automationpkg.JobTaskConfig   `json:"task,omitempty"`
	LoopTarget  *automationpkg.LoopTarget      `json:"loop_target,omitempty"`
	Enabled     *bool                          `json:"enabled,omitempty"`
	Retry       *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit   *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
}

// UpdateJobRequest is the shared automation job patch payload.
type UpdateJobRequest struct {
	Name       *string                        `json:"name,omitempty"`
	Prompt     *string                        `json:"prompt,omitempty"`
	Schedule   *automationpkg.ScheduleSpec    `json:"schedule,omitempty"`
	Task       *automationpkg.JobTaskConfig   `json:"task,omitempty"`
	LoopTarget *automationpkg.LoopTarget      `json:"loop_target,omitempty"`
	Enabled    *bool                          `json:"enabled,omitempty"`
	Retry      *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit  *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
}

// HasChanges reports whether the patch includes any mutable field.
func (r UpdateJobRequest) HasChanges() bool {
	return r.Name != nil ||
		r.Prompt != nil ||
		r.Schedule != nil ||
		r.Task != nil ||
		r.LoopTarget != nil ||
		r.Enabled != nil ||
		r.Retry != nil ||
		r.FireLimit != nil
}

// CreateTriggerRequest is the shared automation trigger create payload.
type CreateTriggerRequest struct {
	Scope              automationpkg.Scope            `json:"scope"`
	Name               string                         `json:"name"`
	TargetKind         automationpkg.TargetKind       `json:"target_kind,omitempty"`
	AgentName          string                         `json:"agent_name"`
	WorkspaceID        string                         `json:"workspace_id,omitempty"`
	Prompt             string                         `json:"prompt"`
	Event              string                         `json:"event"`
	Filter             map[string]string              `json:"filter,omitempty"`
	LoopTarget         *automationpkg.LoopTarget      `json:"loop_target,omitempty"`
	Enabled            *bool                          `json:"enabled,omitempty"`
	Retry              *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit          *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
	WebhookID          string                         `json:"webhook_id,omitempty"`
	EndpointSlug       string                         `json:"endpoint_slug,omitempty"`
	WebhookSecretValue string                         `json:"webhook_secret_value,omitempty"`
}

// UpdateTriggerRequest is the shared automation trigger patch payload.
type UpdateTriggerRequest struct {
	Name               *string                        `json:"name,omitempty"`
	Prompt             *string                        `json:"prompt,omitempty"`
	Event              *string                        `json:"event,omitempty"`
	Filter             map[string]string              `json:"filter,omitempty"`
	LoopTarget         *automationpkg.LoopTarget      `json:"loop_target,omitempty"`
	Enabled            *bool                          `json:"enabled,omitempty"`
	Retry              *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit          *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
	WebhookID          *string                        `json:"webhook_id,omitempty"`
	EndpointSlug       *string                        `json:"endpoint_slug,omitempty"`
	WebhookSecretValue *string                        `json:"webhook_secret_value,omitempty"`
}

// HasChanges reports whether the patch includes any mutable field.
func (r UpdateTriggerRequest) HasChanges() bool {
	return r.Name != nil ||
		r.Prompt != nil ||
		r.Event != nil ||
		r.Filter != nil ||
		r.LoopTarget != nil ||
		r.Enabled != nil ||
		r.Retry != nil ||
		r.FireLimit != nil ||
		r.WebhookID != nil ||
		r.EndpointSlug != nil ||
		r.WebhookSecretValue != nil
}

func cloneLoopTarget(source *automationpkg.LoopTarget) *automationpkg.LoopTarget {
	if source == nil {
		return nil
	}
	cloned := *source
	if len(source.Inputs) > 0 {
		cloned.Inputs = maps.Clone(source.Inputs)
	}
	if len(source.InputMapping) > 0 {
		cloned.InputMapping = maps.Clone(source.InputMapping)
	}
	cloned.NetworkParticipation = participation.CloneRequest(source.NetworkParticipation)
	return &cloned
}
