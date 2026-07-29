package daemon

import (
	"context"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type workspaceAccessConsentCache struct {
	mu        sync.RWMutex
	bySession map[string]workspaceaccess.Consent
}

var (
	_ workspaceaccess.SessionConsentCache = (*workspaceAccessConsentCache)(nil)
	_ sessionLifecycleObserver            = (*workspaceAccessConsentCache)(nil)
)

func newWorkspaceAccessConsentCache() *workspaceAccessConsentCache {
	return &workspaceAccessConsentCache{bySession: make(map[string]workspaceaccess.Consent)}
}

func (c *workspaceAccessConsentCache) ConsentFor(
	_ context.Context,
	sessionID string,
) (workspaceaccess.Consent, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	consent, ok := c.bySession[sessionID]
	return consent, ok
}

func (c *workspaceAccessConsentCache) PutConsent(
	_ context.Context,
	sessionID string,
	consent workspaceaccess.Consent,
) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bySession[sessionID] = consent
}

func (*workspaceAccessConsentCache) OnSessionCreated(context.Context, *session.Session) {}

func (c *workspaceAccessConsentCache) OnSessionStopped(_ context.Context, sess *session.Session) {
	if sess == nil {
		return
	}
	sessionID := strings.TrimSpace(sess.ID)
	if sessionID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.bySession, sessionID)
}
