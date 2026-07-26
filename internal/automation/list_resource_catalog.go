package automation

import (
	"context"
	"sort"
	"strings"

	modelpkg "github.com/compozy/agh/internal/automation/model"
)

type resourceCatalogCandidate struct {
	id     string
	source JobSource
	name   string
}

func (m *Manager) listProjectedJobCatalog(ctx context.Context, query JobListQuery) (JobListPage, error) {
	query = modelpkg.JobListQueryWithDefaultLimit(query)
	if err := modelpkg.ValidateJobListQuery(query); err != nil {
		return JobListPage{}, err
	}
	cursor, hasCursor, err := modelpkg.JobListCursor(query)
	if err != nil {
		return JobListPage{}, err
	}
	overlays, err := m.jobEnabledOverrides(ctx)
	if err != nil {
		return JobListPage{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	candidates := make([]resourceCatalogCandidate, 0, len(m.projectedJobs))
	for _, job := range m.projectedJobs {
		effective := job
		if enabled, ok := overlays[job.ID]; ok && isOverlayManagedSource(job.Source) {
			effective.Enabled = enabled
		}
		if !modelpkg.JobMatchesListQuery(effective, query) {
			continue
		}
		candidates = append(candidates, resourceCatalogCandidate{
			id:     job.ID,
			source: job.Source,
			name:   job.Name,
		})
	}
	sortResourceCatalogCandidates(candidates)
	start := resourceCatalogPageStart(candidates, cursor, hasCursor)
	end := min(start+query.Limit, len(candidates))
	selected := candidates[start:end]
	jobs := make([]Job, 0, len(selected))
	for _, candidate := range selected {
		job := cloneJob(m.projectedJobs[candidate.id])
		if enabled, ok := overlays[job.ID]; ok && isOverlayManagedSource(job.Source) {
			job.Enabled = enabled
		}
		jobs = append(jobs, job)
	}
	page := JobListPage{
		Jobs:    jobs,
		Total:   len(candidates),
		Limit:   query.Limit,
		HasMore: end < len(candidates),
	}
	if page.HasMore {
		last := selected[len(selected)-1]
		page.NextCursor, err = modelpkg.EncodeJobListCursor(query, modelpkg.ListCursorPosition{
			Source: last.source,
			Name:   last.name,
			ID:     last.id,
		})
		if err != nil {
			return JobListPage{}, err
		}
	}
	return page, nil
}

func (m *Manager) listProjectedTriggerCatalog(
	ctx context.Context,
	query TriggerListQuery,
) (TriggerListPage, error) {
	query = modelpkg.TriggerListQueryWithDefaultLimit(query)
	if err := modelpkg.ValidateTriggerListQuery(query); err != nil {
		return TriggerListPage{}, err
	}
	cursor, hasCursor, err := modelpkg.TriggerListCursor(query)
	if err != nil {
		return TriggerListPage{}, err
	}
	overlays, err := m.triggerEnabledOverrides(ctx)
	if err != nil {
		return TriggerListPage{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	candidates := make([]resourceCatalogCandidate, 0, len(m.projectedTriggers))
	for _, trigger := range m.projectedTriggers {
		effective := trigger
		if enabled, ok := overlays[trigger.ID]; ok && isOverlayManagedSource(trigger.Source) {
			effective.Enabled = enabled
		}
		if !modelpkg.TriggerMatchesListQuery(effective, query) {
			continue
		}
		candidates = append(candidates, resourceCatalogCandidate{
			id:     trigger.ID,
			source: trigger.Source,
			name:   trigger.Name,
		})
	}
	sortResourceCatalogCandidates(candidates)
	start := resourceCatalogPageStart(candidates, cursor, hasCursor)
	end := min(start+query.Limit, len(candidates))
	selected := candidates[start:end]
	triggers := make([]Trigger, 0, len(selected))
	for _, candidate := range selected {
		trigger := cloneTrigger(m.projectedTriggers[candidate.id])
		if enabled, ok := overlays[trigger.ID]; ok && isOverlayManagedSource(trigger.Source) {
			trigger.Enabled = enabled
		}
		triggers = append(triggers, trigger)
	}
	page := TriggerListPage{
		Triggers: triggers,
		Total:    len(candidates),
		Limit:    query.Limit,
		HasMore:  end < len(candidates),
	}
	if page.HasMore {
		last := selected[len(selected)-1]
		page.NextCursor, err = modelpkg.EncodeTriggerListCursor(query, modelpkg.ListCursorPosition{
			Source: last.source,
			Name:   last.name,
			ID:     last.id,
		})
		if err != nil {
			return TriggerListPage{}, err
		}
	}
	return page, nil
}

func (m *Manager) jobEnabledOverrides(ctx context.Context) (map[string]bool, error) {
	overlays, err := m.store.ListJobEnabledOverlays(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(overlays))
	for _, overlay := range overlays {
		result[overlay.JobID] = overlay.EnabledOverride
	}
	return result, nil
}

func (m *Manager) triggerEnabledOverrides(ctx context.Context) (map[string]bool, error) {
	overlays, err := m.store.ListTriggerEnabledOverlays(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(overlays))
	for _, overlay := range overlays {
		result[overlay.TriggerID] = overlay.EnabledOverride
	}
	return result, nil
}

func sortResourceCatalogCandidates(candidates []resourceCatalogCandidate) {
	sort.Slice(candidates, func(left int, right int) bool {
		return compareResourceCatalogCandidate(candidates[left], candidates[right]) < 0
	})
}

func resourceCatalogPageStart(
	candidates []resourceCatalogCandidate,
	cursor modelpkg.ListCursorPosition,
	hasCursor bool,
) int {
	if !hasCursor {
		return 0
	}
	anchor := resourceCatalogCandidate{id: cursor.ID, source: cursor.Source, name: cursor.Name}
	return sort.Search(len(candidates), func(index int) bool {
		return compareResourceCatalogCandidate(candidates[index], anchor) > 0
	})
}

func compareResourceCatalogCandidate(left resourceCatalogCandidate, right resourceCatalogCandidate) int {
	if rank := modelpkg.ListSourceRank(left.source) - modelpkg.ListSourceRank(right.source); rank != 0 {
		return rank
	}
	if name := strings.Compare(left.name, right.name); name != 0 {
		return name
	}
	return strings.Compare(left.id, right.id)
}
