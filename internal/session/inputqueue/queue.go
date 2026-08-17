package inputqueue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
)

// Store is the persistence boundary for busy input.
type Store interface {
	store.SessionInputQueueStore
}

// IDGenerator returns a stable queue entry ID or the entropy failure that prevented allocation.
type IDGenerator func() (string, error)

// Config controls queue admission.
type Config struct {
	QueueCap     int
	MaxTextBytes int
}

// InputRequest carries the common fields for a non-admitted busy input.
type InputRequest struct {
	SessionID        string
	Text             string
	TargetTurnID     string
	Generation       int64
	Runtime          store.SessionInputRuntime
	SkillInvocations []commandpkg.Invocation
	Attachments      []store.SessionInputAttachment
}

// ErrInputTextRequired reports a queued input without text or attachments.
var ErrInputTextRequired = errors.New("inputqueue: text is required")

// Service owns validation and admission policy for session busy input.
type Service struct {
	store Store
	now   func() time.Time
	newID IDGenerator
	cfg   Config
}

type insertSpec struct {
	sessionID        string
	text             string
	mode             string
	delivery         string
	targetTurnID     string
	generation       int64
	runtime          store.SessionInputRuntime
	skillInvocations []commandpkg.Invocation
	attachments      []store.SessionInputAttachment
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
		newID: newQueueID,
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
		service.newID = newQueueID
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
	req InputRequest,
) (store.SessionInputQueueEntry, int, error) {
	insert, err := s.PrepareQueue(req)
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

// PrepareQueue validates queued input and allocates its durable insert without persisting it.
func (s *Service) PrepareQueue(req InputRequest) (store.SessionInputQueueInsert, error) {
	return s.newInsert(insertSpec{
		sessionID:        req.SessionID,
		text:             req.Text,
		mode:             store.SessionInputQueueModeQueue,
		delivery:         store.SessionInputDeliveryAfterTurn,
		generation:       req.Generation,
		runtime:          req.Runtime,
		skillInvocations: append([]commandpkg.Invocation(nil), req.SkillInvocations...),
		attachments:      append([]store.SessionInputAttachment(nil), req.Attachments...),
	})
}

// EnqueueAdmitted atomically persists one external command receipt and its queue entry.
func (s *Service) EnqueueAdmitted(
	ctx context.Context,
	admission store.SessionPromptAdmissionRequest,
	generation int64,
) (store.SessionPromptAdmission, store.SessionInputQueueEntry, int, bool, error) {
	admissionStore, insert, err := s.prepareAdmittedEntry(admission, insertSpec{
		mode:       store.SessionInputQueueModeQueue,
		delivery:   store.SessionInputDeliveryAfterTurn,
		generation: generation,
	})
	if err != nil {
		return store.SessionPromptAdmission{}, store.SessionInputQueueEntry{}, 0, false, err
	}
	receipt, entry, position, created, err := admissionStore.EnqueueAdmittedSessionInput(
		ctx,
		admission,
		insert,
	)
	if err != nil {
		if errors.Is(err, store.ErrSessionInputQueueFull) {
			return store.SessionPromptAdmission{}, store.SessionInputQueueEntry{}, 0, false,
				queueFullError(insert.SessionID, s.cfg.QueueCap, err)
		}
		return store.SessionPromptAdmission{}, store.SessionInputQueueEntry{}, 0, false, err
	}
	return receipt, entry, position, created, nil
}

// EnqueueInterrupt persists replacement input before the active turn is canceled.
func (s *Service) EnqueueInterrupt(
	ctx context.Context,
	req InputRequest,
) (store.SessionInputQueueEntry, int, error) {
	insert, err := s.newInsert(insertSpec{
		sessionID:        req.SessionID,
		text:             req.Text,
		mode:             store.SessionInputQueueModeInterrupt,
		delivery:         store.SessionInputDeliveryInterruptThenPrompt,
		targetTurnID:     req.TargetTurnID,
		runtime:          req.Runtime,
		skillInvocations: append([]commandpkg.Invocation(nil), req.SkillInvocations...),
		attachments:      append([]store.SessionInputAttachment(nil), req.Attachments...),
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	return s.store.EnqueueInterruptSessionInput(ctx, insert)
}

// EnqueueAdmittedInterrupt atomically persists a replacement command receipt and queue entry.
func (s *Service) EnqueueAdmittedInterrupt(
	ctx context.Context,
	admission store.SessionPromptAdmissionRequest,
	targetTurnID string,
) (store.SessionPromptAdmission, store.SessionInputQueueEntry, bool, error) {
	admissionStore, insert, err := s.prepareAdmittedEntry(admission, insertSpec{
		mode:         store.SessionInputQueueModeInterrupt,
		delivery:     store.SessionInputDeliveryInterruptThenPrompt,
		targetTurnID: targetTurnID,
	})
	if err != nil {
		return store.SessionPromptAdmission{}, store.SessionInputQueueEntry{}, false, err
	}
	receipt, entry, _, created, err := admissionStore.EnqueueAdmittedSessionInput(ctx, admission, insert)
	return receipt, entry, created, err
}

// StageSteer persists replacement steering guidance while a turn is active.
func (s *Service) StageSteer(
	ctx context.Context,
	req InputRequest,
) (store.SessionInputQueueEntry, error) {
	insert, err := s.newInsert(insertSpec{
		sessionID:        req.SessionID,
		text:             req.Text,
		mode:             store.SessionInputQueueModeSteer,
		delivery:         store.SessionInputDeliveryInterruptThenPrompt,
		targetTurnID:     req.TargetTurnID,
		generation:       req.Generation,
		runtime:          req.Runtime,
		skillInvocations: append([]commandpkg.Invocation(nil), req.SkillInvocations...),
		attachments:      append([]store.SessionInputAttachment(nil), req.Attachments...),
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return s.store.StageSessionSteer(ctx, insert)
}

// StageAdmittedSteer atomically persists one external receipt and its steering entry.
func (s *Service) StageAdmittedSteer(
	ctx context.Context,
	admission store.SessionPromptAdmissionRequest,
	targetTurnID string,
	generation int64,
) (store.SessionPromptAdmission, store.SessionInputQueueEntry, bool, error) {
	admissionStore, insert, err := s.prepareAdmittedEntry(admission, insertSpec{
		mode:         store.SessionInputQueueModeSteer,
		delivery:     store.SessionInputDeliveryInterruptThenPrompt,
		targetTurnID: targetTurnID,
		generation:   generation,
	})
	if err != nil {
		return store.SessionPromptAdmission{}, store.SessionInputQueueEntry{}, false, err
	}
	return admissionStore.StageAdmittedSessionSteer(ctx, admission, insert)
}

func (s *Service) prepareAdmittedEntry(
	admission store.SessionPromptAdmissionRequest,
	spec insertSpec,
) (store.SessionPromptAdmissionStore, store.SessionInputQueueInsert, error) {
	admissionStore, ok := s.store.(store.SessionPromptAdmissionStore)
	if !ok {
		return nil, store.SessionInputQueueInsert{}, errors.New(
			"inputqueue: prompt admission store is required",
		)
	}
	spec.sessionID = admission.SessionID
	spec.text = admission.AuthoredText
	spec.runtime = admission.Runtime
	spec.skillInvocations = append([]commandpkg.Invocation(nil), admission.SkillInvocations...)
	spec.attachments = append([]store.SessionInputAttachment(nil), admission.Attachments...)
	insert, err := s.newInsert(spec)
	if err != nil {
		return nil, store.SessionInputQueueInsert{}, err
	}
	return admissionStore, insert, nil
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

// List returns current pending operator inputs in dispatch order.
func (s *Service) List(
	ctx context.Context,
	sessionID string,
) ([]store.SessionInputQueueEntry, error) {
	target := strings.TrimSpace(sessionID)
	entries, err := s.store.ListPendingSessionInputs(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("inputqueue: list inputs for session %q: %w", target, err)
	}
	return entries, nil
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

// CurrentGeneration reads the current persisted generation for queue fencing.
func (s *Service) CurrentGeneration(ctx context.Context, sessionID string) (int64, error) {
	return s.store.CurrentSessionInputGeneration(ctx, sessionID)
}

func (s *Service) newInsert(spec insertSpec) (store.SessionInputQueueInsert, error) {
	target := strings.TrimSpace(spec.sessionID)
	message := strings.TrimSpace(spec.text)
	if target == "" {
		return store.SessionInputQueueInsert{}, errors.New("inputqueue: session id is required")
	}
	if spec.mode == store.SessionInputQueueModeSteer && len(spec.attachments) > 0 {
		return store.SessionInputQueueInsert{}, store.ErrSessionInputSteerTextOnly
	}
	if message == "" && (spec.mode == store.SessionInputQueueModeSteer || len(spec.attachments) == 0) {
		return store.SessionInputQueueInsert{}, ErrInputTextRequired
	}
	if len([]byte(message)) > s.cfg.MaxTextBytes {
		return store.SessionInputQueueInsert{}, fmt.Errorf(
			"inputqueue: text exceeds session.busy_input.max_text_bytes (%d)",
			s.cfg.MaxTextBytes,
		)
	}
	entryID, err := s.newID()
	if err != nil {
		return store.SessionInputQueueInsert{}, fmt.Errorf("inputqueue: generate entry id: %w", err)
	}
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return store.SessionInputQueueInsert{}, errors.New("inputqueue: id generator returned empty id")
	}
	return store.SessionInputQueueInsert{
		ID:                entryID,
		SessionID:         target,
		Mode:              spec.mode,
		Delivery:          spec.delivery,
		TargetTurnID:      strings.TrimSpace(spec.targetTurnID),
		Text:              message,
		Runtime:           spec.runtime,
		SkillInvocations:  append([]commandpkg.Invocation(nil), spec.skillInvocations...),
		Attachments:       append([]store.SessionInputAttachment(nil), spec.attachments...),
		SessionGeneration: spec.generation,
		QueueCap:          s.cfg.QueueCap,
		Now:               s.now(),
	}, nil
}

func newQueueID() (string, error) {
	suffix, err := randomSuffix(rand.Reader)
	if err != nil {
		return "", err
	}
	return "inq_" + suffix, nil
}

func randomSuffix(entropy io.Reader) (string, error) {
	if entropy == nil {
		return "", errors.New("inputqueue: id entropy source is required")
	}
	var buf [16]byte
	if _, err := io.ReadFull(entropy, buf[:]); err != nil {
		return "", fmt.Errorf("inputqueue: read id entropy: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func queueFullError(sessionID string, queueCap int, cause error) error {
	item := diagnostics.NewItem(diagnostics.ItemSpec{
		ID:       "session.input_queue.full",
		Code:     diagnosticcontract.CodeSessionQueueFull,
		Category: diagnosticcontract.CategorySession,
		Title:    "Session input queue full",
		Message: "The session input queue is at capacity. " +
			"Wait for the active turn to finish, cancel queued input, or retry with interrupt mode.",
		Severity:      diagnosticcontract.SeverityWarn,
		DataFreshness: diagnosticcontract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{
			"session_id": sessionID,
			"queue_cap":  queueCap,
		}),
	)
	return diagnostics.NewStructuredError(item, cause)
}
