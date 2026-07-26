package automation

import (
	"errors"

	"strings"

	"github.com/compozy/agh/internal/session"
)

func (d *Dispatcher) createOpts(req DispatchRequest) session.CreateOpts {
	opts := session.CreateOpts{
		AgentName: req.agentName(),
		Provider:  "",
		Name:      req.name(),
		Type:      session.SessionTypeSystem,
	}

	switch req.scope() {
	case AutomationScopeWorkspace:
		opts.Workspace = req.workspaceID()
	default:
		opts.WorkspacePath = d.globalWorkspacePath
	}

	return opts
}

func (d *Dispatcher) recordAutomationSessionTaskActor(
	sessionID string,
	workspaceID string,
	run *Run,
) error {
	if d == nil || d.taskActors == nil {
		return nil
	}
	actor, err := automationSessionTaskActorContext(sessionID, workspaceID, run)
	if err != nil {
		return err
	}
	return d.taskActors.RecordAutomationSessionTaskActor(strings.TrimSpace(sessionID), actor)
}

func (d *Dispatcher) deleteAutomationSessionTaskActor(sessionID string) {
	if d == nil || d.taskActors == nil {
		return
	}
	d.taskActors.DeleteAutomationSessionTaskActor(strings.TrimSpace(sessionID))
}

func (d *Dispatcher) tryAcquire() bool {
	select {
	case d.gate <- struct{}{}:
		return true
	default:
		return false
	}
}

func (d *Dispatcher) release() {
	select {
	case <-d.gate:
	default:
	}
}

func (r DispatchRequest) agentName() string {
	if r.Job != nil {
		return strings.TrimSpace(r.Job.AgentName)
	}
	return strings.TrimSpace(r.Trigger.AgentName)
}

func (r DispatchRequest) name() string {
	if r.Job != nil {
		return strings.TrimSpace(r.Job.Name)
	}
	return strings.TrimSpace(r.Trigger.Name)
}

func (r DispatchRequest) scope() Scope {
	if r.Job != nil {
		return r.Job.Scope
	}
	return r.Trigger.Scope
}

func (r DispatchRequest) workspaceID() string {
	if r.Job != nil {
		return strings.TrimSpace(r.Job.WorkspaceID)
	}
	return strings.TrimSpace(r.Trigger.WorkspaceID)
}

func (r DispatchRequest) envelopeData() map[string]any {
	if r.Envelope == nil {
		return nil
	}
	return r.Envelope.Data
}

func (r DispatchRequest) retryConfig() RetryConfig {
	if r.Job != nil {
		return r.Job.Retry
	}
	return r.Trigger.Retry
}

func (r DispatchRequest) fireLimitConfig() FireLimitConfig {
	if r.Job != nil {
		return r.Job.FireLimit
	}
	return r.Trigger.FireLimit
}

func (r DispatchRequest) prompt() (string, error) {
	source := strings.TrimSpace(r.Prompt)
	if source == "" {
		if r.Job != nil {
			source = strings.TrimSpace(r.Job.Prompt)
		} else {
			source = strings.TrimSpace(r.Trigger.Prompt)
		}
	}
	if source == "" {
		return "", errors.New("automation: dispatch prompt is required")
	}

	if r.Job != nil {
		return source, nil
	}

	return renderTriggerPrompt(source, r.Envelope)
}
