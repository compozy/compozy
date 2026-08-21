package notifications

import (
	"context"
	"fmt"
	"time"
)

// DeliveryPermit is the owner-active fence held from immediately before an
// external send through acknowledgement and cursor advancement.
type DeliveryPermit struct {
	Key        CursorKey `json:"key"`
	DeliveryID string    `json:"delivery_id"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// DeliveryPermitStore records the owner-active fence required before an external send.
type DeliveryPermitStore interface {
	// AcquireDeliveryPermit records an owner-active fence before an external send.
	AcquireDeliveryPermit(context.Context, DeliveryPermit) error
}

// DeliveryPermitReader enumerates durable sends that still require acknowledgement
// and cursor advancement. Callers must replay them with the same delivery ID.
type DeliveryPermitReader interface {
	ListDeliveryPermits(context.Context) ([]DeliveryPermit, error)
}

// Normalize validates the permit identity and fills an absent acquisition time.
func (p DeliveryPermit) Normalize(fallbackNow time.Time) (DeliveryPermit, error) {
	key, err := p.Key.Normalize()
	if err != nil {
		return DeliveryPermit{}, err
	}
	p.Key = key
	if err := validateNotificationIdentityUTF8(p.DeliveryID, "delivery id"); err != nil {
		return DeliveryPermit{}, err
	}
	if p.DeliveryID == "" {
		return DeliveryPermit{}, fmt.Errorf("%w: delivery id is required", ErrInvalidCursor)
	}
	if p.AcquiredAt.IsZero() {
		p.AcquiredAt = fallbackNow
	}
	p.AcquiredAt = p.AcquiredAt.UTC()
	return p, nil
}

// AcquireDeliveryPermit fences profile archival before an external send.
func (s *Service) AcquireDeliveryPermit(ctx context.Context, permit DeliveryPermit) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("%w: store is required", ErrInvalidCursor)
	}
	store, ok := s.store.(DeliveryPermitStore)
	if !ok {
		return fmt.Errorf("%w: delivery permit store is required", ErrInvalidCursor)
	}
	normalized, err := permit.Normalize(time.Now())
	if err != nil {
		return err
	}
	return store.AcquireDeliveryPermit(ctx, normalized)
}
