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
	session  [32]byte
	path     string
}

type streamTicketEntry struct {
	claim   StreamTicketClaim
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
	return newStreamTicketStore(maxEntries, ttl, now, entropy), nil
}

func newStreamTicketStore(
	maxEntries int,
	ttl time.Duration,
	now func() time.Time,
	entropy io.Reader,
) *StreamTicketStore {
	return &StreamTicketStore{
		entries: make(map[string]streamTicketEntry), max: maxEntries,
		ttl: ttl, now: now, entropy: entropy,
	}
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
	return s.mint(StreamTicketClaim{DeviceID: deviceID, Epoch: epoch})
}

func (s *StreamTicketStore) mint(claim StreamTicketClaim) (StreamTicket, error) {
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
		claim:   claim,
		expires: expiresAt,
	}
	return StreamTicket{Ticket: ticket, ExpiresAt: expiresAt}, nil
}

func (s *StreamTicketStore) Consume(ticket string) (StreamTicketClaim, error) {
	return s.consume(ticket, nil)
}

func (s *StreamTicketStore) consume(ticket string, accept func(StreamTicketClaim) bool) (StreamTicketClaim, error) {
	if !ValidOpaqueSecret(ticket, streamTicketPrefix) {
		return StreamTicketClaim{}, ErrStreamTicketInvalid
	}
	digest := HashCredential(ticket)
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[digest]
	if !ok || !s.now().Before(entry.expires) {
		delete(s.entries, digest)
		return StreamTicketClaim{}, ErrStreamTicketInvalid
	}
	if accept != nil && !accept(entry.claim) {
		return StreamTicketClaim{}, ErrStreamTicketInvalid
	}
	delete(s.entries, digest)
	return entry.claim, nil
}

func (s *StreamTicketStore) sweepExpired(now time.Time) {
	maps.DeleteFunc(s.entries, func(_ string, entry streamTicketEntry) bool {
		return !now.Before(entry.expires)
	})
}
