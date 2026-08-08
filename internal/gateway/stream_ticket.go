package gateway

import (
	"errors"
	"io"
	"sync"
	"time"
)

type streamTicketEntry struct {
	deviceID string
	epoch    uint64
	expires  time.Time
}

// streamTicketStore is a bounded, process-local set consumed atomically at stream connect.
type streamTicketStore struct {
	mu      sync.Mutex
	entries map[string]streamTicketEntry
	max     int
	ttl     time.Duration
	now     func() time.Time
	entropy io.Reader
}

func newStreamTicketStore(
	maxEntries int,
	ttl time.Duration,
	now func() time.Time,
	entropy io.Reader,
) (*streamTicketStore, error) {
	if maxEntries <= 0 {
		return nil, errors.New("gateway: stream ticket limit must be positive")
	}
	if ttl <= 0 {
		return nil, errors.New("gateway: stream ticket TTL must be positive")
	}
	return &streamTicketStore{
		entries: make(map[string]streamTicketEntry), max: maxEntries,
		ttl: ttl, now: now, entropy: entropy,
	}, nil
}

func (s *streamTicketStore) reconfigure(ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("gateway: stream ticket TTL must be positive")
	}
	s.mu.Lock()
	s.sweepExpired(s.now())
	s.ttl = ttl
	s.mu.Unlock()
	return nil
}

func (s *streamTicketStore) mint(device DeviceSession) (StreamTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.sweepExpired(now)
	if len(s.entries) >= s.max {
		return StreamTicket{}, ErrStreamTicketLimit
	}
	ticket, err := generateStreamTicket(s.entropy)
	if err != nil {
		return StreamTicket{}, err
	}
	expiresAt := now.Add(s.ttl)
	s.entries[hashCredential(ticket)] = streamTicketEntry{
		deviceID: device.ID, epoch: device.RevokeEpoch, expires: expiresAt,
	}
	return StreamTicket{Ticket: ticket, ExpiresAt: expiresAt}, nil
}

func (s *streamTicketStore) consume(ticket string) (streamTicketEntry, error) {
	if !validOpaqueSecret(ticket, streamTicketPrefix) {
		return streamTicketEntry{}, ErrStreamTicketInvalid
	}
	digest := hashCredential(ticket)
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[digest]
	delete(s.entries, digest)
	if !ok || !s.now().Before(entry.expires) {
		return streamTicketEntry{}, ErrStreamTicketInvalid
	}
	return entry, nil
}

func (s *streamTicketStore) sweepExpired(now time.Time) {
	for digest, entry := range s.entries {
		if !now.Before(entry.expires) {
			delete(s.entries, digest)
		}
	}
}
