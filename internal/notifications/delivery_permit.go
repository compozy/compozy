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

type DeliveryPermitStore interface {
	AcquireDeliveryPermit(context.Context, DeliveryPermit) error
}

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
		return errorsPermitStoreUnavailable()
	}
	normalized, err := permit.Normalize(time.Now())
	if err != nil {
		return err
	}
	return store.AcquireDeliveryPermit(ctx, normalized)
}

func errorsPermitStoreUnavailable() error {
	return fmt.Errorf("%w: delivery permit store is required", ErrInvalidCursor)
}
