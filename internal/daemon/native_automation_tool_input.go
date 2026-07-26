package daemon

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	automationpkg "github.com/compozy/agh/internal/automation"
)

type automationJobIDInput struct {
	JobID string `json:"job_id"`
}

type automationTriggerIDInput struct {
	TriggerID string `json:"trigger_id"`
}

type automationRunIDInput struct {
	RunID string `json:"run_id"`
}

type automationJobCreateInput struct {
	Scope       automationpkg.Scope            `json:"scope"`
	Name        string                         `json:"name"`
	AgentName   string                         `json:"agent_name"`
	WorkspaceID string                         `json:"workspace_id,omitempty"`
	Prompt      string                         `json:"prompt"`
	Schedule    automationpkg.ScheduleSpec     `json:"schedule"`
	Task        *automationpkg.JobTaskConfig   `json:"task,omitempty"`
	Enabled     *bool                          `json:"enabled,omitempty"`
	Retry       *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit   *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
}

func (i automationJobCreateInput) request() contract.CreateJobRequest {
	return contract.CreateJobRequest{
		Scope:       i.Scope,
		Name:        i.Name,
		AgentName:   i.AgentName,
		WorkspaceID: i.WorkspaceID,
		Prompt:      i.Prompt,
		Schedule:    i.Schedule,
		Task:        i.Task,
		Enabled:     i.Enabled,
		Retry:       i.Retry,
		FireLimit:   i.FireLimit,
	}
}

type automationJobUpdateInput struct {
	JobID     string                         `json:"job_id"`
	Name      *string                        `json:"name,omitempty"`
	Prompt    *string                        `json:"prompt,omitempty"`
	Schedule  *automationpkg.ScheduleSpec    `json:"schedule,omitempty"`
	Task      *automationpkg.JobTaskConfig   `json:"task,omitempty"`
	Enabled   *bool                          `json:"enabled,omitempty"`
	Retry     *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
}

func (i automationJobUpdateInput) request() contract.UpdateJobRequest {
	return contract.UpdateJobRequest{
		Name:      i.Name,
		Prompt:    i.Prompt,
		Schedule:  i.Schedule,
		Task:      i.Task,
		Enabled:   i.Enabled,
		Retry:     i.Retry,
		FireLimit: i.FireLimit,
	}
}

type automationTriggerCreateInput struct {
	Scope              automationpkg.Scope            `json:"scope"`
	Name               string                         `json:"name"`
	AgentName          string                         `json:"agent_name"`
	WorkspaceID        string                         `json:"workspace_id,omitempty"`
	Prompt             string                         `json:"prompt"`
	Event              string                         `json:"event"`
	Filter             map[string]string              `json:"filter,omitempty"`
	Enabled            *bool                          `json:"enabled,omitempty"`
	Retry              *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit          *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
	WebhookID          string                         `json:"webhook_id,omitempty"`
	EndpointSlug       string                         `json:"endpoint_slug,omitempty"`
	WebhookSecretValue *string                        `json:"webhook_secret_value,omitempty"`
}

func (i automationTriggerCreateInput) request() contract.CreateTriggerRequest {
	return contract.CreateTriggerRequest{
		Scope:        i.Scope,
		Name:         i.Name,
		AgentName:    i.AgentName,
		WorkspaceID:  i.WorkspaceID,
		Prompt:       i.Prompt,
		Event:        i.Event,
		Filter:       i.Filter,
		Enabled:      i.Enabled,
		Retry:        i.Retry,
		FireLimit:    i.FireLimit,
		WebhookID:    i.WebhookID,
		EndpointSlug: i.EndpointSlug,
	}
}

func (i automationTriggerCreateInput) webhookSecretWrite() automationpkg.WebhookSecretWrite {
	write := automationpkg.WebhookSecretWrite{}
	if i.WebhookSecretValue != nil {
		value := strings.TrimSpace(*i.WebhookSecretValue)
		write.Value = &value
	}
	return write
}

type automationTriggerUpdateInput struct {
	TriggerID          string                         `json:"trigger_id"`
	Name               *string                        `json:"name,omitempty"`
	Prompt             *string                        `json:"prompt,omitempty"`
	Event              *string                        `json:"event,omitempty"`
	Filter             map[string]string              `json:"filter,omitempty"`
	Enabled            *bool                          `json:"enabled,omitempty"`
	Retry              *automationpkg.RetryConfig     `json:"retry,omitempty"`
	FireLimit          *automationpkg.FireLimitConfig `json:"fire_limit,omitempty"`
	WebhookID          *string                        `json:"webhook_id,omitempty"`
	EndpointSlug       *string                        `json:"endpoint_slug,omitempty"`
	WebhookSecretValue *string                        `json:"webhook_secret_value,omitempty"`
}

func (i automationTriggerUpdateInput) request() contract.UpdateTriggerRequest {
	return contract.UpdateTriggerRequest{
		Name:               i.Name,
		Prompt:             i.Prompt,
		Event:              i.Event,
		Filter:             i.Filter,
		Enabled:            i.Enabled,
		Retry:              i.Retry,
		FireLimit:          i.FireLimit,
		WebhookID:          i.WebhookID,
		EndpointSlug:       i.EndpointSlug,
		WebhookSecretValue: i.WebhookSecretValue,
	}
}

func (i automationTriggerUpdateInput) webhookSecretWrite() *automationpkg.WebhookSecretWrite {
	if i.WebhookSecretValue == nil {
		return nil
	}
	write := automationpkg.WebhookSecretWrite{}
	value := strings.TrimSpace(*i.WebhookSecretValue)
	write.Value = &value
	return &write
}
