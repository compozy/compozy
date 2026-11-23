package remotehost

import (
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

func TestStreamTickets(t *testing.T) {
	t.Parallel()

	t.Run("Should require a freshly minted ticket for reconnect (UT-048)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		tickets, err := NewStreamTicketStore(DefaultStreamTicketLimit, DefaultStreamTicketTTL, func() time.Time { return now }, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		const deviceID, epoch = "dev_browser", 3
		cached, err := tickets.Mint(deviceID, epoch)
		if err != nil {
			t.Fatalf("Mint(cached) error = %v", err)
		}
		// UT-047: tickets expire after 30 seconds and can only be consumed once.
		if !cached.ExpiresAt.Equal(now.Add(30 * time.Second)) {
			t.Fatalf("ticket expiry = %s", cached.ExpiresAt)
		}
		if claim, err := tickets.Consume(cached.Ticket); err != nil || claim.DeviceID != deviceID || claim.Epoch != epoch {
			t.Fatalf("Consume(cached first) = (%#v, %v), want %s at epoch %d", claim, err, deviceID, epoch)
		}
		if _, err := tickets.Consume(cached.Ticket); !errors.Is(err, ErrStreamTicketInvalid) {
			t.Fatalf("Consume(cached reconnect) error = %v", err)
		}
		fresh, err := tickets.Mint(deviceID, epoch)
		if err != nil {
			t.Fatalf("Mint(fresh) error = %v", err)
		}
		if _, err := tickets.Consume(fresh.Ticket); err != nil {
			t.Fatalf("Consume(fresh) error = %v", err)
		}
	})

	t.Run("Should report ticket capacity without masking it as invalid authentication", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		tickets, err := NewStreamTicketStore(1, DefaultStreamTicketTTL, func() time.Time { return now }, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		const deviceID, epoch = "dev_browser", 3
		if _, err := tickets.Mint(deviceID, epoch); err != nil {
			t.Fatalf("Mint(first) error = %v", err)
		}
		if _, err := tickets.Mint(deviceID, epoch); !errors.Is(err, ErrStreamTicketLimit) {
			t.Fatalf("Mint(over capacity) error = %v, want ErrStreamTicketLimit", err)
		}
	})
}
