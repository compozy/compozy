package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CursorStore persists confirmed notification delivery progress.
type CursorStore interface {
	GetCursor(ctx context.Context, key CursorKey) (Cursor, error)
	ListCursors(ctx context.Context, query CursorQuery) ([]Cursor, error)
	AdvanceCursor(ctx context.Context, update AdvanceCursor) (Cursor, error)
	ResetCursor(ctx context.Context, reset ResetCursor) (Cursor, error)
	RecordCursorError(ctx context.Context, report CursorError) (Cursor, error)
}

// CursorKey identifies one durable delivery cursor.
type CursorKey struct {
	ConsumerID string `json:"consumer_id"`
	StreamName string `json:"stream_name"`
	SubjectID  string `json:"subject_id"`
}

// Cursor stores the latest confirmed delivery position for one consumer.
type Cursor struct {
	Key             CursorKey `json:"key"`
	LastSequence    int64     `json:"last_sequence"`
	LastDeliveryID  string    `json:"last_delivery_id,omitempty"`
	LastDeliveredAt time.Time `json:"last_delivered_at"`
	LastError       string    `json:"last_error,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AdvanceCursor records a confirmed delivery position.
type AdvanceCursor struct {
	Key             CursorKey `json:"key"`
	LastSequence    int64     `json:"last_sequence"`
	LastDeliveredAt time.Time `json:"last_delivered_at"`
	DeliveryID      string    `json:"delivery_id,omitempty"`
	Now             time.Time `json:"now"`
}

// ResetCursor rewinds or repairs one cursor after an explicit recovery decision.
type ResetCursor struct {
	Key             CursorKey `json:"key"`
	LastSequence    int64     `json:"last_sequence"`
	LastDeliveryID  string    `json:"last_delivery_id,omitempty"`
	LastDeliveredAt time.Time `json:"last_delivered_at"`
	Reason          string    `json:"reason"`
	Now             time.Time `json:"now"`
}

// CursorError records a bounded diagnostic without advancing delivery progress.
type CursorError struct {
	Key       CursorKey `json:"key"`
	LastError string    `json:"last_error"`
	Now       time.Time `json:"now"`
}

// CursorQuery filters cursor diagnostics.
type CursorQuery struct {
	ConsumerID string `json:"consumer_id,omitempty"`
	StreamName string `json:"stream_name,omitempty"`
	SubjectID  string `json:"subject_id,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// Service validates cursor requests before delegating persistence to the store.
type Service struct {
	store CursorStore
}

// NewService creates a notification cursor service.
func NewService(store CursorStore) *Service {
	return &Service{store: store}
}

// Get returns one durable cursor.
func (s *Service) Get(ctx context.Context, key CursorKey) (Cursor, error) {
	if s == nil || s.store == nil {
		return Cursor{}, fmt.Errorf("%w: store is required", ErrInvalidCursor)
	}
	normalized, err := key.Normalize()
	if err != nil {
		return Cursor{}, err
	}
	return s.store.GetCursor(ctx, normalized)
}

// List returns durable cursors matching the query.
func (s *Service) List(ctx context.Context, query CursorQuery) ([]Cursor, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidCursor)
	}
	return s.store.ListCursors(ctx, query.Normalize())
}

// Advance records a monotonic confirmed delivery position.
func (s *Service) Advance(ctx context.Context, update AdvanceCursor) (Cursor, error) {
	if s == nil || s.store == nil {
		return Cursor{}, fmt.Errorf("%w: store is required", ErrInvalidCursor)
	}
	normalized, err := update.Normalize(time.Now())
	if err != nil {
		return Cursor{}, err
	}
	return s.store.AdvanceCursor(ctx, normalized)
}

// Reset repairs one cursor after an explicit recovery decision.
func (s *Service) Reset(ctx context.Context, reset ResetCursor) (Cursor, error) {
	if s == nil || s.store == nil {
		return Cursor{}, fmt.Errorf("%w: store is required", ErrInvalidCursor)
	}
	normalized, err := reset.Normalize(time.Now())
	if err != nil {
		return Cursor{}, err
	}
	return s.store.ResetCursor(ctx, normalized)
}

// RecordError stores a diagnostic without moving the confirmed delivery sequence.
func (s *Service) RecordError(ctx context.Context, report CursorError) (Cursor, error) {
	if s == nil || s.store == nil {
		return Cursor{}, fmt.Errorf("%w: store is required", ErrInvalidCursor)
	}
	normalized, err := report.Normalize(time.Now())
	if err != nil {
		return Cursor{}, err
	}
	return s.store.RecordCursorError(ctx, normalized)
}

// Normalize trims and validates cursor identity.
func (k CursorKey) Normalize() (CursorKey, error) {
	normalized := CursorKey{
		ConsumerID: strings.TrimSpace(k.ConsumerID),
		StreamName: strings.TrimSpace(k.StreamName),
		SubjectID:  strings.TrimSpace(k.SubjectID),
	}
	switch {
	case normalized.ConsumerID == "":
		return CursorKey{}, fmt.Errorf("%w: consumer id is required", ErrInvalidCursor)
	case normalized.StreamName == "":
		return CursorKey{}, fmt.Errorf("%w: stream name is required", ErrInvalidCursor)
	default:
		return normalized, nil
	}
}

// Normalize validates an advance request and fills missing timestamps.
func (a AdvanceCursor) Normalize(fallbackNow time.Time) (AdvanceCursor, error) {
	key, err := a.Key.Normalize()
	if err != nil {
		return AdvanceCursor{}, err
	}
	normalized := a
	normalized.Key = key
	normalized.DeliveryID = strings.TrimSpace(a.DeliveryID)
	if normalized.LastSequence <= 0 {
		return AdvanceCursor{}, fmt.Errorf("%w: last sequence must be greater than zero", ErrInvalidCursor)
	}
	if normalized.Now.IsZero() {
		normalized.Now = fallbackNow
	}
	if normalized.LastDeliveredAt.IsZero() {
		normalized.LastDeliveredAt = normalized.Now
	}
	normalized.Now = normalized.Now.UTC()
	normalized.LastDeliveredAt = normalized.LastDeliveredAt.UTC()
	return normalized, nil
}

// Normalize validates a reset request and fills missing timestamps.
func (r ResetCursor) Normalize(fallbackNow time.Time) (ResetCursor, error) {
	key, err := r.Key.Normalize()
	if err != nil {
		return ResetCursor{}, err
	}
	normalized := r
	normalized.Key = key
	normalized.LastDeliveryID = strings.TrimSpace(r.LastDeliveryID)
	normalized.Reason = strings.TrimSpace(r.Reason)
	if normalized.LastSequence < 0 {
		return ResetCursor{}, fmt.Errorf("%w: reset sequence must be zero or greater", ErrInvalidCursor)
	}
	if normalized.Reason == "" {
		return ResetCursor{}, ErrResetReasonRequired
	}
	if normalized.Now.IsZero() {
		normalized.Now = fallbackNow
	}
	normalized.Now = normalized.Now.UTC()
	if !normalized.LastDeliveredAt.IsZero() {
		normalized.LastDeliveredAt = normalized.LastDeliveredAt.UTC()
	}
	return normalized, nil
}

// Normalize validates an error report and fills missing timestamps.
func (e CursorError) Normalize(fallbackNow time.Time) (CursorError, error) {
	key, err := e.Key.Normalize()
	if err != nil {
		return CursorError{}, err
	}
	normalized := e
	normalized.Key = key
	normalized.LastError = strings.TrimSpace(e.LastError)
	if normalized.LastError == "" {
		return CursorError{}, fmt.Errorf("%w: last error is required", ErrInvalidCursor)
	}
	if normalized.Now.IsZero() {
		normalized.Now = fallbackNow
	}
	normalized.Now = normalized.Now.UTC()
	return normalized, nil
}

// Normalize trims query filters.
func (q CursorQuery) Normalize() CursorQuery {
	normalized := CursorQuery{
		ConsumerID: strings.TrimSpace(q.ConsumerID),
		StreamName: strings.TrimSpace(q.StreamName),
		SubjectID:  strings.TrimSpace(q.SubjectID),
		Limit:      q.Limit,
	}
	if normalized.Limit < 0 {
		normalized.Limit = 0
	}
	return normalized
}
