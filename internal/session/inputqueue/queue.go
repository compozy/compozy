package inputqueue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
)

// Store is the persistence boundary for busy input.
type Store interface {
	store.SessionInputQueueStore
}

// IDGenerator returns stable queue entry ids.
type IDGenerator func() string

// Config controls queue admission.
type Config struct {
	QueueCap     int
	MaxTextBytes int
}

// Service owns validation and admission policy for session busy input.
type Service struct {
	store Store
	now   func() time.Time
	newID IDGenerator
	cfg   Config
}

// Option customizes a Service.
type Option func(*Service)

// WithClock injects a deterministic clock.
func WithClock(now func() time.Time) Option {
	return func(service *Service) {
		service.now = now
	}
}

// WithIDGenerator injects deterministic queue ids.
func WithIDGenerator(generator IDGenerator) Option {
	return func(service *Service) {
		service.newID = generator
	}
}

// New constructs a busy-input queue service.
func New(store Store, cfg Config, opts ...Option) (*Service, error) {
	if store == nil {
		return nil, errors.New("inputqueue: store is required")
	}
	service := &Service{
		store: store,
		cfg:   cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		newID: func() string {
			return "inq_" + randomSuffix()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	if service.now == nil {
		service.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if service.newID == nil {
		service.newID = func() string {
			return "inq_" + randomSuffix()
		}
	}
	if service.cfg.QueueCap <= 0 {
		service.cfg.QueueCap = 10
	}
	if service.cfg.MaxTextBytes <= 0 {
		service.cfg.MaxTextBytes = 64 << 10
	}
	return service, nil
}

// Enqueue appends operator input behind the active prompt turn.
func (s *Service) Enqueue(
	ctx context.Context,
	sessionID string,
	text string,
	generation int64,
) (store.SessionInputQueueEntry, int, error) {
	insert, err := s.newInsert(sessionID, text, store.SessionInputQueueModeQueue, generation)
	if err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	entry, position, err := s.store.EnqueueSessionInput(ctx, insert)
	if err != nil {
		if errors.Is(err, store.ErrSessionInputQueueFull) {
			return store.SessionInputQueueEntry{}, 0, queueFullError(insert.SessionID, s.cfg.QueueCap, err)
		}
		return store.SessionInputQueueEntry{}, 0, err
	}
	return entry, position, nil
}

// StageSteer stages replacement steering guidance while a turn is active.
func (s *Service) StageSteer(
	ctx context.Context,
	sessionID string,
	text string,
	generation int64,
) (store.SessionInputQueueEntry, error) {
	insert, err := s.newInsert(sessionID, text, store.SessionInputQueueModeSteer, generation)
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return s.store.StageSessionSteer(ctx, insert)
}

// ConsumeSteer atomically consumes the current staged steer entry, if any.
func (s *Service) ConsumeSteer(
	ctx context.Context,
	sessionID string,
) (store.SessionInputQueueEntry, bool, error) {
	return s.store.ConsumeSessionSteer(ctx, strings.TrimSpace(sessionID), s.now())
}

// ClaimNext leases the next input eligible for dispatch.
func (s *Service) ClaimNext(
	ctx context.Context,
	sessionID string,
) (store.SessionInputQueueEntry, bool, error) {
	return s.store.ClaimNextSessionInput(ctx, strings.TrimSpace(sessionID), s.now())
}

// PeekNext returns the next eligible input without mutating its queue state.
func (s *Service) PeekNext(
	ctx context.Context,
	sessionID string,
) (store.SessionInputQueueEntry, bool, error) {
	target := strings.TrimSpace(sessionID)
	entry, found, err := s.store.PeekNextSessionInput(ctx, target)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, fmt.Errorf(
			"inputqueue: peek next input for session %q: %w",
			target,
			err,
		)
	}
	return entry, found, nil
}

// Get returns one exact durable queue entry.
func (s *Service) Get(
	ctx context.Context,
	sessionID string,
	entryID string,
) (store.SessionInputQueueEntry, error) {
	target := strings.TrimSpace(sessionID)
	targetEntry := strings.TrimSpace(entryID)
	entry, err := s.store.GetSessionInputQueueEntry(
		ctx,
		target,
		targetEntry,
	)
	if err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf(
			"inputqueue: get input %q for session %q: %w",
			targetEntry,
			target,
			err,
		)
	}
	return entry, nil
}

// MarkSent records successful dispatch.
func (s *Service) MarkSent(ctx context.Context, sessionID string, entryID string) error {
	return s.store.MarkSessionInputSent(ctx, sessionID, entryID, s.now())
}

// Release returns a leased entry to queued state after a dispatch race.
func (s *Service) Release(ctx context.Context, sessionID string, entryID string) error {
	return s.store.ReleaseSessionInput(ctx, sessionID, entryID, s.now())
}

// MarkFailed records failed dispatch.
func (s *Service) MarkFailed(ctx context.Context, sessionID string, entryID string, summary string) error {
	return s.store.MarkSessionInputFailed(ctx, sessionID, entryID, summary, s.now())
}

// Cancel cancels one pending queue entry.
func (s *Service) Cancel(ctx context.Context, sessionID string, entryID string) (store.SessionInputQueueEntry, error) {
	return s.store.CancelSessionInput(ctx, sessionID, entryID, s.now())
}

// AdvanceGeneration fences older entries and returns the new generation.
func (s *Service) AdvanceGeneration(ctx context.Context, sessionID string) (int64, int, error) {
	generation, err := s.store.AdvanceSessionInputGeneration(ctx, sessionID, s.now())
	if err != nil {
		return 0, 0, err
	}
	canceled, err := s.store.CancelPendingSessionInputs(ctx, sessionID, generation, s.now())
	if err != nil {
		return 0, 0, err
	}
	return generation, canceled, nil
}

// CurrentGeneration reads the current persisted generation for queue fencing.
func (s *Service) CurrentGeneration(ctx context.Context, sessionID string) (int64, error) {
	return s.store.CurrentSessionInputGeneration(ctx, sessionID)
}

func (s *Service) newInsert(
	sessionID string,
	text string,
	mode string,
	generation int64,
) (store.SessionInputQueueInsert, error) {
	target := strings.TrimSpace(sessionID)
	message := strings.TrimSpace(text)
	if target == "" {
		return store.SessionInputQueueInsert{}, errors.New("inputqueue: session id is required")
	}
	if message == "" {
		return store.SessionInputQueueInsert{}, errors.New("inputqueue: text is required")
	}
	if len([]byte(message)) > s.cfg.MaxTextBytes {
		return store.SessionInputQueueInsert{}, fmt.Errorf(
			"inputqueue: text exceeds session.busy_input.max_text_bytes (%d)",
			s.cfg.MaxTextBytes,
		)
	}
	return store.SessionInputQueueInsert{
		ID:                strings.TrimSpace(s.newID()),
		SessionID:         target,
		Mode:              mode,
		Text:              message,
		SessionGeneration: generation,
		QueueCap:          s.cfg.QueueCap,
		Now:               s.now(),
	}, nil
}

func randomSuffix() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func queueFullError(sessionID string, queueCap int, cause error) error {
	item := diagnostics.NewItem(
		"session.input_queue.full",
		diagnosticcontract.CodeSessionQueueFull,
		diagnosticcontract.CategorySession,
		"Session input queue full",
		"The session input queue is at capacity. "+
			"Wait for the active turn to finish, cancel queued input, or retry with interrupt mode.",
		diagnosticcontract.SeverityWarn,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"session_id": sessionID,
			"queue_cap":  queueCap,
		}),
	)
	return diagnostics.NewStructuredError(item, cause)
}
