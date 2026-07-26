package globaldb

import (
	"errors"
	"fmt"
	"strings"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/store"
)

func (g *AutomationRepo) normalizeJobForCreate(job automation.Job) (automation.Job, error) {
	normalized := normalizeAutomationJob(job)
	if normalized.Source == "" {
		normalized.Source = automation.JobSourceDynamic
	}
	if strings.TrimSpace(normalized.ID) == "" {
		normalized.ID = store.NewID("job")
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	if err := normalized.Validate("job"); err != nil {
		return automation.Job{}, err
	}
	return normalized, nil
}

func (g *AutomationRepo) normalizeJobForUpdate(job automation.Job) (automation.Job, error) {
	normalized := normalizeAutomationJob(job)
	if strings.TrimSpace(normalized.ID) == "" {
		return automation.Job{}, errors.New("store: automation job id is required")
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}
	if err := normalized.Validate("job"); err != nil {
		return automation.Job{}, err
	}
	return normalized, nil
}

func (g *AutomationRepo) normalizeTriggerForCreate(trigger automation.Trigger) (automation.Trigger, error) {
	normalized := normalizeAutomationTrigger(trigger)
	if normalized.Source == "" {
		normalized.Source = automation.JobSourceDynamic
	}
	if strings.TrimSpace(normalized.ID) == "" {
		normalized.ID = store.NewID("trg")
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	if err := normalized.Validate("trigger"); err != nil {
		return automation.Trigger{}, err
	}
	return normalized, nil
}

func (g *AutomationRepo) normalizeTriggerForUpdate(trigger automation.Trigger) (automation.Trigger, error) {
	normalized := normalizeAutomationTrigger(trigger)
	if strings.TrimSpace(normalized.ID) == "" {
		return automation.Trigger{}, errors.New("store: automation trigger id is required")
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}
	if err := normalized.Validate("trigger"); err != nil {
		return automation.Trigger{}, err
	}
	return normalized, nil
}

func (g *AutomationRepo) normalizeRunForCreate(run automation.Run) (automation.Run, error) {
	normalized := normalizeAutomationRun(run)
	if strings.TrimSpace(normalized.ID) == "" {
		normalized.ID = store.NewID("run")
	}
	if normalized.Attempt == 0 {
		normalized.Attempt = 1
	}
	if err := validateAutomationRunRecord(normalized); err != nil {
		return automation.Run{}, err
	}
	return normalized, nil
}

func (g *AutomationRepo) normalizeRunForUpdate(run automation.Run) (automation.Run, error) {
	normalized := normalizeAutomationRun(run)
	if strings.TrimSpace(normalized.ID) == "" {
		return automation.Run{}, errors.New("store: automation run id is required")
	}
	if normalized.Attempt <= 0 {
		return automation.Run{}, fmt.Errorf("store: automation run %q attempt must be positive", normalized.ID)
	}
	if err := validateAutomationRunRecord(normalized); err != nil {
		return automation.Run{}, err
	}
	return normalized, nil
}
