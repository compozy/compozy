package automation

import (
	"strings"

	"github.com/compozy/agh/internal/resources"
)

func currentResourceActor(source resources.ResourceSource, fallback resources.MutationActor) resources.MutationActor {
	actor := fallback
	if actor.Kind == "" {
		actor = defaultAutomationResourceActor()
	}
	actor.Source = source.Normalize()
	actor.ID = source.ID
	if strings.TrimSpace(actor.ID) == "" {
		actor.ID = "automation-resource"
	}
	return actor
}

func (m *Manager) projectedJobDefinitions() []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.projectedJobDefinitionsLocked()
}

func (m *Manager) projectedJobDefinitionsLocked() []Job {
	jobs := make([]Job, 0, len(m.projectedJobs))
	for _, job := range m.projectedJobs {
		jobs = append(jobs, cloneJob(job))
	}
	sortJobs(jobs)
	return jobs
}

func (m *Manager) projectedTriggerDefinitions() []Trigger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.projectedTriggerDefinitionsLocked()
}

func (m *Manager) projectedTriggerDefinitionsLocked() []Trigger {
	triggers := make([]Trigger, 0, len(m.projectedTriggers))
	for _, trigger := range m.projectedTriggers {
		triggers = append(triggers, cloneTrigger(trigger))
	}
	sortTriggers(triggers)
	return triggers
}

func (m *Manager) projectedJobDefinition(id string) (Job, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return Job{}, ErrJobNotFound
	}
	m.mu.RLock()
	job, ok := m.projectedJobs[trimmedID]
	m.mu.RUnlock()
	if !ok {
		return Job{}, ErrJobNotFound
	}
	return cloneJob(job), nil
}

func (m *Manager) projectedTriggerDefinition(id string) (Trigger, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return Trigger{}, ErrTriggerNotFound
	}
	m.mu.RLock()
	trigger, ok := m.projectedTriggers[trimmedID]
	m.mu.RUnlock()
	if !ok {
		return Trigger{}, ErrTriggerNotFound
	}
	return cloneTrigger(trigger), nil
}

func jobMapFromSlice(jobs []Job) map[string]Job {
	byID := make(map[string]Job, len(jobs))
	for _, job := range jobs {
		byID[job.ID] = cloneJob(job)
	}
	return byID
}

func triggerMapFromSlice(triggers []Trigger) map[string]Trigger {
	byID := make(map[string]Trigger, len(triggers))
	for _, trigger := range triggers {
		byID[trigger.ID] = cloneTrigger(trigger)
	}
	return byID
}

func cloneJobs(jobs []Job) []Job {
	if len(jobs) == 0 {
		return nil
	}
	cloned := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		cloned = append(cloned, cloneJob(job))
	}
	return cloned
}

func cloneTriggers(triggers []Trigger) []Trigger {
	if len(triggers) == 0 {
		return nil
	}
	cloned := make([]Trigger, 0, len(triggers))
	for _, trigger := range triggers {
		cloned = append(cloned, cloneTrigger(trigger))
	}
	return cloned
}
