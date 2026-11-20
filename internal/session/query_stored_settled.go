package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

// settledStoredStatus remembers stored sessions last read as stopped, keyed by the
// metadata file identity observed before that read. Metadata is replaced atomically,
// so any later write changes the identity and forces a full read again.
type settledStoredStatus struct {
	mu      sync.Mutex
	entries map[string]os.FileInfo
}

func (s *settledStoredStatus) matches(id string, current os.FileInfo) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	proven, ok := s.entries[id]
	return ok && os.SameFile(proven, current) && proven.Size() == current.Size() &&
		proven.ModTime().Equal(current.ModTime())
}

func (s *settledStoredStatus) remember(id string, observed os.FileInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]os.FileInfo)
	}
	s.entries[id] = observed
}

func (s *settledStoredStatus) forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
}

// StoredStatusUnlessSettled reads stored status like StoredStatus, except that it reports
// settled without reading metadata when the session is neither live nor pending and its
// metadata is the same file an earlier read found stopped. Repeated supervision passes use
// it to skip finished children instead of decoding their full metadata every time.
func (m *Manager) StoredStatusUnlessSettled(ctx context.Context, id string) (*Info, bool, error) {
	if ctx == nil {
		return nil, false, errors.New("session: status context is required")
	}
	target, err := normalizeStoredSessionID(id)
	if err != nil {
		return nil, false, err
	}
	if m.storedSessionLive(target) {
		m.settledStatus.forget(target)
		info, err := m.StoredStatus(ctx, target)
		return info, false, err
	}
	// Observe identity before reading so a concurrent replacement can only cause a re-read.
	observed, statErr := os.Stat(store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target)))
	if statErr == nil && m.settledStatus.matches(target, observed) {
		return nil, true, nil
	}
	info, err := m.StoredStatus(ctx, target)
	if err != nil || statErr != nil || info.State != StateStopped || m.storedSessionLive(target) {
		m.settledStatus.forget(target)
		return info, false, err
	}
	m.settledStatus.remember(target, observed)
	return info, false, nil
}

func (m *Manager) storedSessionLive(id string) bool {
	_, ok := m.Get(id)
	return ok || m.isPending(id)
}
