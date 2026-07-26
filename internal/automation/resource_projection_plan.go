package automation

import "github.com/compozy/agh/internal/resources"

type jobResourceProjectionPlan struct {
	revision   int64
	operations int
	jobs       []Job
	scheduler  *Scheduler
	unchanged  bool
}

func (p *jobResourceProjectionPlan) Kind() resources.ResourceKind {
	return JobResourceKind
}

func (p *jobResourceProjectionPlan) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

func (p *jobResourceProjectionPlan) OperationCount() int {
	if p == nil {
		return 0
	}
	return p.operations
}

type triggerResourceProjectionPlan struct {
	revision   int64
	operations int
	triggers   []Trigger
	engine     *TriggerEngine
	unchanged  bool
}

func (p *triggerResourceProjectionPlan) Kind() resources.ResourceKind {
	return TriggerResourceKind
}

func (p *triggerResourceProjectionPlan) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

func (p *triggerResourceProjectionPlan) OperationCount() int {
	if p == nil {
		return 0
	}
	return p.operations
}

func (m *Manager) jobResourceStateMatches(revision int64, jobs []Job) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.jobRevision != revision || len(m.projectedJobs) != len(jobs) {
		return false
	}
	for _, job := range jobs {
		current, ok := m.projectedJobs[job.ID]
		if !ok || !sameJobDefinition(current, job) {
			return false
		}
	}
	return true
}

func (m *Manager) triggerResourceStateMatches(revision int64, triggers []Trigger) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.triggerRevision != revision || len(m.projectedTriggers) != len(triggers) {
		return false
	}
	for _, trigger := range triggers {
		current, ok := m.projectedTriggers[trigger.ID]
		if !ok || !sameTriggerDefinition(current, trigger) {
			return false
		}
	}
	return true
}
