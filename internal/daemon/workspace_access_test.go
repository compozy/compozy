package daemon

import (
	"testing"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestWorkspaceAccessConsentLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should UT-084 clear only the stopped session consent through the lifecycle fanout", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		cache := newWorkspaceAccessConsentCache()
		fanout := newSessionLifecycleFanout(cache)
		cache.PutConsent(ctx, "sess-a", workspaceaccess.ConsentAllow)
		cache.PutConsent(ctx, "sess-b", workspaceaccess.ConsentReject)

		fanout.OnSessionStopped(ctx, &session.Session{ID: "sess-a", AgentName: "coder"})

		if consent, ok := cache.ConsentFor(ctx, "sess-a"); ok || consent != "" {
			t.Fatalf("ConsentFor(sess-a) = (%q, %t), want miss", consent, ok)
		}
		if consent, ok := cache.ConsentFor(ctx, "sess-b"); !ok || consent != workspaceaccess.ConsentReject {
			t.Fatalf("ConsentFor(sess-b) = (%q, %t), want reject hit", consent, ok)
		}
		fanout.OnSessionCreated(ctx, &session.Session{ID: "sess-c", AgentName: "coder"})
		if consent, ok := cache.ConsentFor(ctx, "sess-c"); ok || consent != "" {
			t.Fatalf("ConsentFor(new same-agent session) = (%q, %t), want miss", consent, ok)
		}
	})
}
