package gateway

import (
	"errors"
	"testing"
	"time"
)

func TestStreamTickets(t *testing.T) {
	t.Parallel()

	t.Run("Should require a freshly minted ticket for reconnect (UT-048)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Browser", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		ctx := ContextWithDevice(t.Context(), issued.Device)
		cached, err := service.MintStreamTicket(ctx, issued.Device.ID)
		if err != nil {
			t.Fatalf("MintStreamTicket(cached) error = %v", err)
		}
		// UT-047: tickets expire after 30 seconds and can only be consumed once.
		if !cached.ExpiresAt.Equal(now.Add(30 * time.Second)) {
			t.Fatalf("ticket expiry = %s", cached.ExpiresAt)
		}
		if _, err := service.ConsumeStreamTicket(t.Context(), cached.Ticket); err != nil {
			t.Fatalf("ConsumeStreamTicket(cached first) error = %v", err)
		}
		if _, err := service.ConsumeStreamTicket(t.Context(), cached.Ticket); !errors.Is(err, ErrStreamTicketInvalid) {
			t.Fatalf("ConsumeStreamTicket(cached reconnect) error = %v", err)
		}
		fresh, err := service.MintStreamTicket(ctx, issued.Device.ID)
		if err != nil {
			t.Fatalf("MintStreamTicket(fresh) error = %v", err)
		}
		if _, err := service.ConsumeStreamTicket(t.Context(), fresh.Ticket); err != nil {
			t.Fatalf("ConsumeStreamTicket(fresh) error = %v", err)
		}
	})

	t.Run("Should report ticket capacity without masking it as invalid authentication", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now, WithStreamTicketLimits(1, 30*time.Second))
		issued, err := issueTestDevice(t.Context(), service, "Browser", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		ctx := ContextWithDevice(t.Context(), issued.Device)
		if _, err := service.MintStreamTicket(ctx, issued.Device.ID); err != nil {
			t.Fatalf("MintStreamTicket(first) error = %v", err)
		}
		if _, err := service.MintStreamTicket(ctx, issued.Device.ID); !errors.Is(err, ErrStreamTicketLimit) {
			t.Fatalf("MintStreamTicket(over capacity) error = %v, want ErrStreamTicketLimit", err)
		}
	})
}
