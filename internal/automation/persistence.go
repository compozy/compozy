package automation

import modelpkg "github.com/compozy/agh/internal/automation/model"

var (
	// ErrJobNotFound reports that the requested automation job does not exist.
	ErrJobNotFound = modelpkg.ErrJobNotFound
	// ErrTriggerNotFound reports that the requested automation trigger does not exist.
	ErrTriggerNotFound = modelpkg.ErrTriggerNotFound
	// ErrRunNotFound reports that the requested automation run does not exist.
	ErrRunNotFound = modelpkg.ErrRunNotFound
	// ErrRunAlreadyExists reports that an automation run identity has already been claimed.
	ErrRunAlreadyExists = modelpkg.ErrRunAlreadyExists
	// ErrSchedulerStateNotFound reports that no durable scheduler cursor exists for a job.
	ErrSchedulerStateNotFound = modelpkg.ErrSchedulerStateNotFound
	// ErrScheduledFireAlreadyClaimed reports that a scheduled fire identity was already claimed.
	ErrScheduledFireAlreadyClaimed = modelpkg.ErrScheduledFireAlreadyClaimed
	// ErrJobNameTaken reports a duplicate job name within the same automation scope.
	ErrJobNameTaken = modelpkg.ErrJobNameTaken
	// ErrTriggerNameTaken reports a duplicate trigger name within the same automation scope.
	ErrTriggerNameTaken = modelpkg.ErrTriggerNameTaken
	// ErrTriggerWebhookIDTaken reports a duplicate stable webhook identifier.
	ErrTriggerWebhookIDTaken = modelpkg.ErrTriggerWebhookIDTaken
	// ErrOverlayRequiresConfigSource reports that enabled overlays only apply to TOML-backed definitions.
	ErrOverlayRequiresConfigSource = modelpkg.ErrOverlayRequiresConfigSource
	// ErrJobOverlayNotFound reports that a job enabled overlay row does not exist.
	ErrJobOverlayNotFound = modelpkg.ErrJobOverlayNotFound
	// ErrTriggerOverlayNotFound reports that a trigger enabled overlay row does not exist.
	ErrTriggerOverlayNotFound = modelpkg.ErrTriggerOverlayNotFound
	// ErrListCursorInvalid reports a malformed or query-mismatched automation list cursor.
	ErrListCursorInvalid = modelpkg.ErrListCursorInvalid
)

const (
	// DefaultListLimit bounds automation list responses when callers omit a limit.
	DefaultListLimit = modelpkg.DefaultListLimit
	// MaxListLimit bounds one automation list response.
	MaxListLimit = modelpkg.MaxListLimit
)

// JobListQuery filters persisted automation job listings.
type JobListQuery = modelpkg.JobListQuery

// TriggerListQuery filters persisted automation trigger listings.
type TriggerListQuery = modelpkg.TriggerListQuery

// JobListPage is one stable page of automation jobs.
type JobListPage = modelpkg.JobListPage

// TriggerListPage is one stable page of automation triggers.
type TriggerListPage = modelpkg.TriggerListPage

// ValidateJobListQuery validates job list filters and cursor binding.
func ValidateJobListQuery(query JobListQuery) error {
	return modelpkg.ValidateJobListQuery(query)
}

// ValidateTriggerListQuery validates trigger list filters and cursor binding.
func ValidateTriggerListQuery(query TriggerListQuery) error {
	return modelpkg.ValidateTriggerListQuery(query)
}

// BuildJobListPage applies canonical job filters, ordering, and cursor pagination.
func BuildJobListPage(jobs []Job, query JobListQuery) (JobListPage, error) {
	return modelpkg.BuildJobListPage(jobs, query)
}

// BuildTriggerListPage applies canonical trigger filters, ordering, and cursor pagination.
func BuildTriggerListPage(triggers []Trigger, query TriggerListQuery) (TriggerListPage, error) {
	return modelpkg.BuildTriggerListPage(triggers, query)
}

// SortJobsForList orders jobs by source precedence, name, and id.
func SortJobsForList(jobs []Job) {
	modelpkg.SortJobsForList(jobs)
}

// SortTriggersForList orders triggers by source precedence, name, and id.
func SortTriggersForList(triggers []Trigger) {
	modelpkg.SortTriggersForList(triggers)
}

// RunQuery filters automation run history and fire-limit window lookups.
type RunQuery = modelpkg.RunQuery

// JobEnabledOverlay stores the runtime enabled override for a config-backed job.
type JobEnabledOverlay = modelpkg.JobEnabledOverlay

// TriggerEnabledOverlay stores the runtime enabled override for a config-backed trigger.
type TriggerEnabledOverlay = modelpkg.TriggerEnabledOverlay
