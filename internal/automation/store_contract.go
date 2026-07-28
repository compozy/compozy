package automation

import (
	"context"
	"time"
)

// Store is the persistence surface consumed by the composed automation manager.
type Store interface {
	RunStore
	GetRun(ctx context.Context, id string) (Run, error)
	CreateJob(ctx context.Context, job Job) (Job, error)
	UpdateJob(ctx context.Context, job Job) (Job, error)
	DeleteJob(ctx context.Context, id string) error
	GetJob(ctx context.Context, id string) (Job, error)
	ListJobs(ctx context.Context, query JobListQuery) (JobListPage, error)
	ListJobDefinitions(ctx context.Context, source JobSource) ([]Job, error)
	CreateTrigger(ctx context.Context, trigger Trigger) (Trigger, error)
	UpdateTrigger(ctx context.Context, trigger Trigger) (Trigger, error)
	DeleteTrigger(ctx context.Context, id string) error
	GetTrigger(ctx context.Context, id string) (Trigger, error)
	ListTriggers(ctx context.Context, query TriggerListQuery) (TriggerListPage, error)
	ListTriggerDefinitions(ctx context.Context, source JobSource) ([]Trigger, error)
	ListRuns(ctx context.Context, query RunQuery) ([]Run, error)
	GetSchedulerState(ctx context.Context, jobID string) (SchedulerState, error)
	ListSchedulerStates(ctx context.Context) ([]SchedulerState, error)
	SaveSchedulerState(ctx context.Context, state SchedulerState) (SchedulerState, error)
	DeleteSchedulerState(ctx context.Context, jobID string) error
	ClaimScheduledRun(ctx context.Context, claim SchedulerClaim) (SchedulerClaimResult, error)
	RecordRunDeliveryError(ctx context.Context, runID string, runErr error) (Run, error)
	SetJobEnabledOverlay(ctx context.Context, overlay JobEnabledOverlay) (JobEnabledOverlay, error)
	GetJobEnabledOverlay(ctx context.Context, jobID string) (JobEnabledOverlay, error)
	ListJobEnabledOverlays(ctx context.Context) ([]JobEnabledOverlay, error)
	DeleteJobEnabledOverlay(ctx context.Context, jobID string) error
	SetTriggerEnabledOverlay(ctx context.Context, overlay TriggerEnabledOverlay) (TriggerEnabledOverlay, error)
	GetTriggerEnabledOverlay(ctx context.Context, triggerID string) (TriggerEnabledOverlay, error)
	ListTriggerEnabledOverlays(ctx context.Context) ([]TriggerEnabledOverlay, error)
	DeleteTriggerEnabledOverlay(ctx context.Context, triggerID string) error
	CreateSuggestion(ctx context.Context, suggestion Suggestion, pendingCap int) (Suggestion, error)
	GetSuggestion(ctx context.Context, workspaceID string, id string) (Suggestion, error)
	ListSuggestions(
		ctx context.Context,
		workspaceID string,
		status SuggestionStatus,
	) ([]Suggestion, error)
	ListAcceptedSuggestions(ctx context.Context) ([]Suggestion, error)
	ResolveSuggestion(
		ctx context.Context,
		workspaceID string,
		id string,
		to SuggestionStatus,
	) (Suggestion, error)
	RollbackSuggestionAcceptance(
		ctx context.Context,
		workspaceID string,
		id string,
		resolvedAt time.Time,
	) error
	RecordSuggestionTransition(ctx context.Context, suggestion Suggestion, jobID string) error
}
