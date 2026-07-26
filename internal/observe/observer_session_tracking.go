package observe

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/session"
)

func (o *Observer) trackSession(id string, snapshot observedSession) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sessions[strings.TrimSpace(id)] = snapshot
}

func (o *Observer) untrackSession(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.sessions, strings.TrimSpace(id))
}

func (o *Observer) sessionSnapshot(id string) (observedSession, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	snapshot, ok := o.sessions[strings.TrimSpace(id)]
	return snapshot, ok
}

// OnSandboxLifecycleEvent receives optional sandbox lifecycle spans from session orchestration.
func (o *Observer) OnSandboxLifecycleEvent(_ context.Context, event session.SandboxLifecycleEvent) {
	if o == nil || o.logger == nil {
		return
	}
	o.logger.Debug(
		"observe: sandbox lifecycle",
		"name", event.Name,
		"span", event.Span,
		"session_id", event.SessionID,
		"workspace_id", event.WorkspaceID,
		"sandbox_id", event.SandboxID,
		"backend", event.Backend,
		"profile", event.Profile,
		"instance_id", event.InstanceID,
		"duration_ms", event.Duration.Milliseconds(),
		"error_kind", event.ErrorKind,
		"error", event.Error,
	)
}
