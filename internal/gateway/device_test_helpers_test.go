package gateway

import (
	"context"
	"errors"
	"io"
	"sort"
	"sync"
	"testing"
	"time"
)

type incrementingReader struct {
	mu   sync.Mutex
	next byte
}

func (r *incrementingReader) Read(target []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range target {
		target[index] = r.next
		r.next++
	}
	return len(target), nil
}

var _ io.Reader = (*incrementingReader)(nil)

type memoryDeviceStore struct {
	mu     sync.Mutex
	byID   map[string]StoredDeviceSession
	byHash map[string]string
}

func newMemoryDeviceStore() *memoryDeviceStore {
	return &memoryDeviceStore{
		byID: make(map[string]StoredDeviceSession), byHash: make(map[string]string),
	}
}

func (s *memoryDeviceStore) CreateDevice(_ context.Context, record StoredDeviceSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byHash[record.TokenHash]; exists {
		return errors.New("duplicate token hash")
	}
	s.byID[record.Session.ID] = record
	s.byHash[record.TokenHash] = record.Session.ID
	return nil
}

func (s *memoryDeviceStore) FindDeviceByTokenHash(
	_ context.Context,
	hash string,
	now time.Time,
) (StoredDeviceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byHash[hash]
	if !ok {
		return StoredDeviceSession{}, ErrDeviceNotFound
	}
	record := s.byID[id]
	if !record.Session.RevokedAt.IsZero() {
		return StoredDeviceSession{}, ErrDeviceNotFound
	}
	record.Session.LastSeenAt = now
	s.byID[id] = record
	return record, nil
}

func (s *memoryDeviceStore) ListDevices(context.Context) ([]DeviceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	devices := make([]DeviceSession, 0, len(s.byID))
	for _, record := range s.byID {
		devices = append(devices, record.Session)
	}
	sort.Slice(devices, func(left, right int) bool {
		leftActive := devices[left].RevokedAt.IsZero()
		rightActive := devices[right].RevokedAt.IsZero()
		if leftActive != rightActive {
			return leftActive
		}
		leftSeen := devices[left].LastSeenAt
		rightSeen := devices[right].LastSeenAt
		if leftSeen.IsZero() != rightSeen.IsZero() {
			return !leftSeen.IsZero()
		}
		if !leftSeen.Equal(rightSeen) {
			return leftSeen.After(rightSeen)
		}
		return devices[left].CreatedAt.After(devices[right].CreatedAt)
	})
	return devices, nil
}

func (s *memoryDeviceStore) GetDevice(_ context.Context, id string) (DeviceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.byID[id]
	if !ok {
		return DeviceSession{}, ErrDeviceNotFound
	}
	return record.Session, nil
}

func (s *memoryDeviceStore) RenameDevice(_ context.Context, id, name string) (DeviceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.renameLocked(id, name)
}

func (s *memoryDeviceStore) RenameDeviceForActor(
	_ context.Context,
	actorID string,
	epoch uint64,
	id string,
	name string,
) (DeviceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.revalidateLocked(actorID, epoch); err != nil {
		return DeviceSession{}, err
	}
	return s.renameLocked(id, name)
}

func (s *memoryDeviceStore) RevokeDevice(
	_ context.Context,
	id string,
	now time.Time,
) (DeviceSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revokeLocked(id, now)
}

func (s *memoryDeviceStore) RevokeDeviceForActor(
	_ context.Context,
	actorID string,
	epoch uint64,
	id string,
	now time.Time,
) (DeviceSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.revalidateLocked(actorID, epoch); err != nil {
		return DeviceSession{}, false, err
	}
	return s.revokeLocked(id, now)
}

func (s *memoryDeviceStore) RevalidateDeviceEpoch(_ context.Context, id string, epoch uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revalidateLocked(id, epoch)
}

func (s *memoryDeviceStore) renameLocked(id, name string) (DeviceSession, error) {
	record, ok := s.byID[id]
	if !ok {
		return DeviceSession{}, ErrDeviceNotFound
	}
	record.Session.Name = name
	s.byID[id] = record
	return record.Session, nil
}

func (s *memoryDeviceStore) revokeLocked(id string, now time.Time) (DeviceSession, bool, error) {
	record, ok := s.byID[id]
	if !ok {
		return DeviceSession{}, false, ErrDeviceNotFound
	}
	if !record.Session.RevokedAt.IsZero() {
		return record.Session, false, nil
	}
	record.Session.RevokedAt = now
	record.Session.RevokeEpoch++
	s.byID[id] = record
	return record.Session, true, nil
}

func (s *memoryDeviceStore) revalidateLocked(id string, epoch uint64) error {
	record, ok := s.byID[id]
	if !ok || !record.Session.RevokedAt.IsZero() || record.Session.RevokeEpoch != epoch {
		return ErrDeviceRevoked
	}
	return nil
}

func newDeviceServiceFixture(
	t *testing.T,
	now *time.Time,
	options ...DeviceServiceOption,
) (*DeviceService, *memoryDeviceStore) {
	t.Helper()
	store := newMemoryDeviceStore()
	base := []DeviceServiceOption{
		WithDeviceClock(func() time.Time { return *now }),
		WithDeviceEntropy(&incrementingReader{}),
		WithPairingLimits(2, time.Minute),
		WithStreamTicketLimits(8, 30*time.Second),
	}
	service, err := NewDeviceService(store, append(base, options...)...)
	if err != nil {
		t.Fatalf("NewDeviceService() error = %v", err)
	}
	return service, store
}

func newDeviceServiceOnly(t *testing.T, now *time.Time, options ...DeviceServiceOption) *DeviceService {
	t.Helper()
	service, store := newDeviceServiceFixture(t, now, options...)
	if store == nil {
		t.Fatal("newDeviceServiceFixture() store = nil")
	}
	return service
}

func issueTestDevice(
	ctx context.Context,
	service *DeviceService,
	name string,
	kind ActorKind,
) (IssuedCredential, error) {
	artifact, err := service.MintPairing(ctx, PairingRequest{Source: PairingSourceLocal})
	if err != nil {
		return IssuedCredential{}, err
	}
	return service.RedeemPairing(ctx, RedeemRequest{
		Artifact: artifact.Artifact, Name: name, Kind: kind,
		Source: PairingSourcePrivate,
	})
}
