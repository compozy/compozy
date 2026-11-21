package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// settledStoredStatus remembers stored sessions last read as stopped, keyed by the
// metadata file identity observed before that read. Metadata is replaced atomically,
// so any later write changes the identity and forces a full read again.
type settledStoredStatus struct {
	mu      sync.Mutex
	entries map[string]metaFileIdentity
}

// metaFileIdentity is captured at stat time. Size and modification time always take
// part; Unix also adds device and inode, which the atomic replace always changes.
type metaFileIdentity struct {
	size    int64
	modTime time.Time
	file    platformFileID
}

func newMetaFileIdentity(info os.FileInfo) metaFileIdentity {
	return metaFileIdentity{size: info.Size(), modTime: info.ModTime(), file: platformFileIDOf(info)}
}

func (i metaFileIdentity) equal(other metaFileIdentity) bool {
	return i.size == other.size && i.modTime.Equal(other.modTime) && i.file == other.file
}

func (s *settledStoredStatus) matches(id string, current metaFileIdentity) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	proven, ok := s.entries[id]
	return ok && proven.equal(current)
}

func (s *settledStoredStatus) remember(id string, observed metaFileIdentity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]metaFileIdentity)
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
// A settled answer also skips the owner proof and catalog decoration of StoredStatus:
// callers get no Info and no errors from those reads, only the stopped classification.
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
	var observed metaFileIdentity
	stat, statErr := os.Stat(store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target)))
	if statErr == nil {
		observed = newMetaFileIdentity(stat)
	}
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
