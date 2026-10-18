package observe

import (
	"strings"
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
