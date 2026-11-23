package remotehost

import (
	"errors"
	"io"
	"maps"
	"sync"
	"time"
)

const (
	DefaultStreamTicketTTL = 30 * time.Second
	// #nosec G101 -- this is a public protocol prefix, not credential material.
	streamTicketPrefix = "cpz_gwt_"
)

// DefaultStreamTicketLimit bounds live single-use tickets held by one daemon process.
const DefaultStreamTicketLimit = 256

// StreamTicket is a short-lived, single-use stream credential.
type StreamTicket struct {
	Ticket    string
	ExpiresAt time.Time
}

// StreamTicketClaim identifies the credential that minted a ticket.
type StreamTicketClaim struct {
	DeviceID string
	Epoch    uint64
}

type streamTicketEntry struct {
	StreamTicketClaim
	expires time.Time
}

// StreamTicketStore is a bounded, process-local set consumed atomically at stream connect.
type StreamTicketStore struct {
	mu      sync.Mutex
	entries map[string]streamTicketEntry
	max     int
	ttl     time.Duration
	now     func() time.Time
	entropy io.Reader
}

func NewStreamTicketStore(
	maxEntries int,
	ttl time.Duration,
	now func() time.Time,
	entropy io.Reader,
) (*StreamTicketStore, error) {
	if maxEntries <= 0 {
		return nil, errors.New("gateway: stream ticket limit must be positive")
	}
	if ttl <= 0 {
		return nil, errors.New("gateway: stream ticket TTL must be positive")
	}
	return &StreamTicketStore{
		entries: make(map[string]streamTicketEntry), max: maxEntries,
		ttl: ttl, now: now, entropy: entropy,
	}, nil
}

func (s *StreamTicketStore) Reconfigure(ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("gateway: stream ticket TTL must be positive")
	}
	s.mu.Lock()
	s.sweepExpired(s.now())
	s.ttl = ttl
	s.mu.Unlock()
	return nil
}

func (s *StreamTicketStore) Mint(deviceID string, epoch uint64) (StreamTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.sweepExpired(now)
	if len(s.entries) >= s.max {
		return StreamTicket{}, ErrStreamTicketLimit
	}
	ticket, err := GenerateOpaqueSecret(s.entropy, streamTicketPrefix)
	if err != nil {
		return StreamTicket{}, err
	}
	expiresAt := now.Add(s.ttl)
	s.entries[HashCredential(ticket)] = streamTicketEntry{
		StreamTicketClaim: StreamTicketClaim{DeviceID: deviceID, Epoch: epoch}, expires: expiresAt,
	}
	return StreamTicket{Ticket: ticket, ExpiresAt: expiresAt}, nil
}

func (s *StreamTicketStore) Consume(ticket string) (StreamTicketClaim, error) {
	if !ValidOpaqueSecret(ticket, streamTicketPrefix) {
		return StreamTicketClaim{}, ErrStreamTicketInvalid
	}
	digest := HashCredential(ticket)
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[digest]
	delete(s.entries, digest)
	if !ok || !s.now().Before(entry.expires) {
		return StreamTicketClaim{}, ErrStreamTicketInvalid
	}
	return entry.StreamTicketClaim, nil
}

func (s *StreamTicketStore) sweepExpired(now time.Time) {
	maps.DeleteFunc(s.entries, func(_ string, entry streamTicketEntry) bool {
		return !now.Before(entry.expires)
	})
}
