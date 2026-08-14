package worktree

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/diagnostics"
)

const catalogSubscriberBuffer = 64

type CatalogEventKind string

const (
	CatalogEventUpserted CatalogEventKind = "upserted"
	CatalogEventDeleted  CatalogEventKind = "deleted"
	CatalogEventFailed   CatalogEventKind = "failed"
)

type CatalogEvent struct {
	Kind        CatalogEventKind
	WorkspaceID string
	WorktreeID  string
	Error       string
}

type catalogBroadcaster struct {
	mu          sync.Mutex
	subscribers map[*catalogSubscriber]struct{}
}

type catalogSubscriber struct {
	ch        chan CatalogEvent
	closeOnce sync.Once
}

func newCatalogBroadcaster() *catalogBroadcaster {
	return &catalogBroadcaster{subscribers: make(map[*catalogSubscriber]struct{})}
}

func (b *catalogBroadcaster) subscribe(ctx context.Context) (<-chan CatalogEvent, func(), error) {
	if ctx == nil {
		return nil, nil, errors.New("worktree: catalog stream context is required")
	}
	subscriber := &catalogSubscriber{ch: make(chan CatalogEvent, catalogSubscriberBuffer)}
	b.mu.Lock()
	b.subscribers[subscriber] = struct{}{}
	b.mu.Unlock()
	cancel := sync.OnceFunc(func() {
		b.mu.Lock()
		delete(b.subscribers, subscriber)
		b.mu.Unlock()
		subscriber.close()
	})
	return subscriber.ch, cancel, nil
}

func (b *catalogBroadcaster) publish(event CatalogEvent) {
	if strings.TrimSpace(event.WorkspaceID) == "" || strings.TrimSpace(event.WorktreeID) == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for subscriber := range b.subscribers {
		select {
		case subscriber.ch <- event:
		default:
			delete(b.subscribers, subscriber)
			subscriber.close()
		}
	}
}

func (s *catalogSubscriber) close() {
	if s != nil {
		s.closeOnce.Do(func() { close(s.ch) })
	}
}

func (s *Service) SubscribeWorktreeCatalogEvents(
	ctx context.Context,
) (<-chan CatalogEvent, func(), error) {
	if s == nil {
		return nil, nil, errors.New("worktree: service is required")
	}
	s.catalogMu.Lock()
	if s.catalog == nil {
		s.catalog = newCatalogBroadcaster()
	}
	broadcaster := s.catalog
	s.catalogMu.Unlock()
	return broadcaster.subscribe(ctx)
}

func (s *Service) publishCatalogEvent(kind CatalogEventKind, item Worktree) {
	s.publishCatalogEventWithError(kind, item, "")
}

func (s *Service) publishCatalogFailure(item Worktree, cause error) {
	s.publishCatalogEventWithError(
		CatalogEventFailed,
		item,
		diagnostics.RedactAndBound(cause.Error(), setupDiagnosticLimit),
	)
}

func (s *Service) publishCatalogEventWithError(
	kind CatalogEventKind,
	item Worktree,
	errorMessage string,
) {
	if s == nil {
		return
	}
	s.catalogMu.Lock()
	broadcaster := s.catalog
	s.catalogMu.Unlock()
	if broadcaster != nil {
		broadcaster.publish(CatalogEvent{
			Kind: kind, WorkspaceID: item.WorkspaceID, WorktreeID: item.ID, Error: errorMessage,
		})
	}
}
