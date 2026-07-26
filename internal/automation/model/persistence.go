package model

import (
	"errors"
	"time"
)

var (
	// ErrJobNotFound reports that the requested automation job does not exist.
	ErrJobNotFound = errors.New("automation: job not found")
	// ErrTriggerNotFound reports that the requested automation trigger does not exist.
	ErrTriggerNotFound = errors.New("automation: trigger not found")
	// ErrRunNotFound reports that the requested automation run does not exist.
	ErrRunNotFound = errors.New("automation: run not found")
	// ErrRunAlreadyExists reports that an automation run identity has already been claimed.
	ErrRunAlreadyExists = errors.New("automation: run already exists")
	// ErrSchedulerStateNotFound reports that no durable scheduler cursor exists for a job.
	ErrSchedulerStateNotFound = errors.New("automation: scheduler state not found")
	// ErrScheduledFireAlreadyClaimed reports that a scheduled fire identity was already claimed.
	ErrScheduledFireAlreadyClaimed = errors.New("automation: scheduled fire already claimed")
	// ErrJobNameTaken reports a duplicate job name within the same automation scope.
	ErrJobNameTaken = errors.New("automation: job name already exists in scope")
	// ErrTriggerNameTaken reports a duplicate trigger name within the same automation scope.
	ErrTriggerNameTaken = errors.New("automation: trigger name already exists in scope")
	// ErrTriggerWebhookIDTaken reports a duplicate stable webhook identifier.
	ErrTriggerWebhookIDTaken = errors.New("automation: trigger webhook id already exists")
	// ErrOverlayRequiresConfigSource reports that enabled overlays only apply to config- or package-backed definitions.
	ErrOverlayRequiresConfigSource = errors.New("automation: enabled overlays require config or package source")
	// ErrJobOverlayNotFound reports that a job enabled overlay row does not exist.
	ErrJobOverlayNotFound = errors.New("automation: job enabled overlay not found")
	// ErrTriggerOverlayNotFound reports that a trigger enabled overlay row does not exist.
	ErrTriggerOverlayNotFound = errors.New("automation: trigger enabled overlay not found")
)

// JobListQuery filters persisted automation job listings.
type JobListQuery struct {
	Scope       Scope     `json:"scope,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	Source      JobSource `json:"source,omitempty"`
	LoopName    string    `json:"loop_name,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
	Search      string    `json:"q,omitempty"`
	Cursor      string    `json:"cursor,omitempty"`
	Limit       int       `json:"limit,omitempty"`
}

// TriggerListQuery filters persisted automation trigger listings.
type TriggerListQuery struct {
	Scope       Scope     `json:"scope,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	Event       string    `json:"event,omitempty"`
	Source      JobSource `json:"source,omitempty"`
	LoopName    string    `json:"loop_name,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
	Search      string    `json:"q,omitempty"`
	Cursor      string    `json:"cursor,omitempty"`
	Limit       int       `json:"limit,omitempty"`
}

// RunQuery filters automation run history and fire-limit window lookups.
type RunQuery struct {
	JobID     string    `json:"job_id,omitempty"`
	TriggerID string    `json:"trigger_id,omitempty"`
	Status    RunStatus `json:"status,omitempty"`
	ExcludeID string    `json:"exclude_id,omitempty"`
	Since     time.Time `json:"since"`
	Until     time.Time `json:"until"`
	Limit     int       `json:"limit,omitempty"`
}

// JobEnabledOverlay stores the runtime enabled override for a config-backed job.
type JobEnabledOverlay struct {
	JobID           string    `json:"job_id"`
	EnabledOverride bool      `json:"enabled_override"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TriggerEnabledOverlay stores the runtime enabled override for a config-backed trigger.
type TriggerEnabledOverlay struct {
	TriggerID       string    `json:"trigger_id"`
	EnabledOverride bool      `json:"enabled_override"`
	UpdatedAt       time.Time `json:"updated_at"`
}
