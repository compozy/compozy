package globaldb

import (
	"context"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

// eventSummaryPublications bridges a lifecycle commit and its immediate
// publication attempt without retaining a SQLite lock or inventing a second
// replay cursor. Readers wait only for attempts already active when they enter.
// A failed attempt remains durable in the outbox for the owned retry worker.
type eventSummaryPublications struct {
	mu      sync.Mutex
	pending map[chan struct{}]eventSummaryPublicationScope
}

type eventSummaryPublicationScope struct {
	profileID, workspaceID string
}

func (p *eventSummaryPublications) begin(profileID, workspaceID string) func() {
	done := make(chan struct{})
	p.mu.Lock()
	if p.pending == nil {
		p.pending = make(map[chan struct{}]eventSummaryPublicationScope)
	}
	p.pending[done] = eventSummaryPublicationScope{profileID: profileID, workspaceID: workspaceID}
	p.mu.Unlock()
	return func() {
		p.mu.Lock()
		delete(p.pending, done)
		close(done)
		p.mu.Unlock()
	}
}

func (p *eventSummaryPublications) wait(ctx context.Context, scope store.ReadScope, workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	p.mu.Lock()
	pending := make([]<-chan struct{}, 0, len(p.pending))
	for done, owner := range p.pending {
		if scope.Matches(owner.profileID) && (workspaceID == "" || workspaceID == owner.workspaceID) {
			pending = append(pending, done)
		}
	}
	p.mu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
