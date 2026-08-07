package gateway

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type recordingConnectionRegistry struct {
	mu                sync.Mutex
	canceled          []string
	store             *memoryDeviceStore
	revokedBeforeCall bool
}

func (r *recordingConnectionRegistry) Register(string, context.CancelFunc) uint64 { return 1 }
func (r *recordingConnectionRegistry) RegisterWaitable(string, context.CancelFunc, <-chan struct{}) uint64 {
	return 1
}
func (r *recordingConnectionRegistry) Deregister(string, uint64) {}
func (r *recordingConnectionRegistry) CancelDevice(_ context.Context, deviceID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.store != nil {
		record := r.store.byID[deviceID]
		r.revokedBeforeCall = !record.Session.RevokedAt.IsZero() && record.Session.RevokeEpoch > 0
	}
	r.canceled = append(r.canceled, deviceID)
	return 2, nil
}

func TestDeviceServiceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should list full device metadata and persist rename (UT-037)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Before", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		now = now.Add(time.Minute)
		if _, err := service.Authenticate(t.Context(), issued.Credential); err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
		renamed, err := service.RenameDevice(t.Context(), issued.Device.ID, "After")
		if err != nil {
			t.Fatalf("RenameDevice() error = %v", err)
		}
		devices, err := service.ListDevices(t.Context())
		if err != nil {
			t.Fatalf("ListDevices() error = %v", err)
		}
		if len(devices) != 1 || renamed.Name != "After" || devices[0].Name != "After" ||
			devices[0].PairingOrigin != string(PairingSourceLocal) || devices[0].LastSeenAt.IsZero() ||
			devices[0].CreatedAt.IsZero() {
			t.Fatalf("device lifecycle payload = %#v / %#v", renamed, devices)
		}
	})

	t.Run("Should order active inventory by latest activity (UT-038)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		first, err := issueTestDevice(t.Context(), service, "First", ActorKindCLIProfile)
		if err != nil {
			t.Fatalf("issue first device: %v", err)
		}
		now = now.Add(time.Minute)
		second, err := issueTestDevice(t.Context(), service, "Second", ActorKindCLIProfile)
		if err != nil {
			t.Fatalf("issue second device: %v", err)
		}
		now = now.Add(time.Minute)
		if _, err := service.Authenticate(t.Context(), first.Credential); err != nil {
			t.Fatalf("authenticate first device: %v", err)
		}
		devices, err := service.ListDevices(t.Context())
		if err != nil {
			t.Fatalf("ListDevices() error = %v", err)
		}
		if len(devices) != 2 || devices[0].ID != first.Device.ID || devices[1].ID != second.Device.ID {
			t.Fatalf("ordered devices = %#v", devices)
		}
	})

	t.Run("Should make revoke idempotent and distinct (UT-039)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Device", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		first, err := service.RevokeDevice(t.Context(), issued.Device.ID)
		if err != nil {
			t.Fatalf("RevokeDevice(first) error = %v", err)
		}
		second, err := service.RevokeDevice(t.Context(), issued.Device.ID)
		if err != nil {
			t.Fatalf("RevokeDevice(second) error = %v", err)
		}
		if !first.Changed || second.Changed || first.Device.RevokeEpoch != 1 || second.Device.RevokeEpoch != 1 {
			t.Fatalf("revoke results = %#v / %#v", first, second)
		}
	})

	t.Run("Should bump epoch before canceling every live connection (UT-040)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		store := newMemoryDeviceStore()
		registry := &recordingConnectionRegistry{store: store}
		service, err := NewDeviceService(
			store,
			WithDeviceClock(func() time.Time { return now }),
			WithDeviceEntropy(&incrementingReader{}),
			WithConnectionRegistry(registry),
		)
		if err != nil {
			t.Fatalf("NewDeviceService() error = %v", err)
		}
		issued, err := issueTestDevice(t.Context(), service, "Device", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		result, err := service.RevokeDevice(t.Context(), issued.Device.ID)
		if err != nil {
			t.Fatalf("RevokeDevice() error = %v", err)
		}
		if !registry.revokedBeforeCall || result.Canceled != 2 || result.Device.RevokeEpoch != 1 {
			t.Fatalf("revoke ordering = result:%#v registry:%#v", result, registry)
		}
	})

	t.Run("Should reject a stale epoch before a mutation commit (UT-042)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Device", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		if _, err := service.RevokeDevice(t.Context(), issued.Device.ID); err != nil {
			t.Fatalf("RevokeDevice() error = %v", err)
		}
		if err := service.RevalidateForCommit(
			t.Context(),
			issued.Device.ID,
			issued.Device.RevokeEpoch,
		); !errors.Is(err, ErrDeviceRevoked) {
			t.Fatalf("RevalidateForCommit(stale) error = %v", err)
		}
	})

	t.Run("Should seal and drain an in-flight mutation before durable revocation (UT-042)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Device", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		mutationCtx, err := service.AcquireMutation(
			t.Context(),
			issued.Device.ID,
			issued.Device.RevokeEpoch,
		)
		if err != nil {
			t.Fatalf("AcquireMutation() error = %v", err)
		}
		t.Cleanup(func() { ReleaseMutation(mutationCtx) })

		type revokeOutcome struct {
			result RevokeResult
			err    error
		}
		revoked := make(chan revokeOutcome, 1)
		go func() {
			result, revokeErr := service.RevokeDevice(t.Context(), issued.Device.ID)
			revoked <- revokeOutcome{result: result, err: revokeErr}
		}()
		select {
		case <-mutationCtx.Done():
		case <-time.After(time.Second):
			t.Fatal("mutation context was not canceled by revocation")
		}
		select {
		case outcome := <-revoked:
			t.Fatalf("RevokeDevice() returned before mutation release: %#v", outcome)
		default:
		}
		if err := service.RevalidateForCommit(
			context.WithoutCancel(mutationCtx),
			issued.Device.ID,
			issued.Device.RevokeEpoch,
		); !errors.Is(err, ErrDeviceRevoked) {
			t.Fatalf("RevalidateForCommit(sealed) error = %v, want ErrDeviceRevoked", err)
		}
		ReleaseMutation(mutationCtx)
		outcome := <-revoked
		if outcome.err != nil || !outcome.result.Changed || outcome.result.Device.RevokeEpoch != 1 {
			t.Fatalf("RevokeDevice() = (%#v, %v)", outcome.result, outcome.err)
		}
	})

	t.Run("Should permit self-revoke and reject the next call (UT-043)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Self", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		actorCtx := ContextWithDevice(t.Context(), issued.Device)
		if _, err := service.RevokeDevice(actorCtx, issued.Device.ID); err != nil {
			t.Fatalf("RevokeDevice(self) error = %v", err)
		}
		if _, err := service.Authenticate(t.Context(), issued.Credential); !errors.Is(err, ErrDeviceUnauthenticated) {
			t.Fatalf("Authenticate(after self revoke) error = %v", err)
		}
	})

	t.Run("Should allow local re-pairing after the last device is revoked (UT-044)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		first, err := issueTestDevice(t.Context(), service, "Only", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issue first device: %v", err)
		}
		if _, err := service.RevokeDevice(t.Context(), first.Device.ID); err != nil {
			t.Fatalf("revoke last device: %v", err)
		}
		second, err := issueTestDevice(t.Context(), service, "Replacement", ActorKindOperatorDevice)
		if err != nil || second.Device.ID == first.Device.ID {
			t.Fatalf("issue replacement = %#v, %v", second, err)
		}
	})

	t.Run("Should mask a revoked credential at every authentication entry point (UT-046)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Device", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		ctx := ContextWithDevice(t.Context(), issued.Device)
		ticket, err := service.MintStreamTicket(ctx, issued.Device.ID)
		if err != nil {
			t.Fatalf("MintStreamTicket() error = %v", err)
		}
		if _, err := service.RevokeDevice(t.Context(), issued.Device.ID); err != nil {
			t.Fatalf("RevokeDevice() error = %v", err)
		}
		if _, err := service.Authenticate(t.Context(), issued.Credential); !errors.Is(err, ErrDeviceUnauthenticated) {
			t.Fatalf("Authenticate(revoked) error = %v", err)
		}
		if _, err := service.ConsumeStreamTicket(t.Context(), ticket.Ticket); !errors.Is(err, ErrStreamTicketInvalid) {
			t.Fatalf("ConsumeStreamTicket(revoked) error = %v", err)
		}
	})

	t.Run("Should return deterministic not-found and valid empty inventory (UT-051, UT-052)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		devices, err := service.ListDevices(t.Context())
		if err != nil || devices == nil || len(devices) != 0 {
			t.Fatalf("ListDevices(empty) = %#v, %v", devices, err)
		}
		if _, err := service.RenameDevice(t.Context(), "dev_missing", "Missing"); !errors.Is(err, ErrDeviceNotFound) {
			t.Fatalf("RenameDevice(missing) error = %v", err)
		}
	})
}

func TestAuthFailureLimiter(t *testing.T) {
	t.Parallel()

	t.Run("Should use the system clock when no clock is supplied", func(t *testing.T) {
		t.Parallel()
		limiter := NewAuthFailureLimiter(1, time.Minute, nil)
		limiter.Failure("198.51.100.7", false)
		if limiter.Allow("198.51.100.7", false) {
			t.Fatal("Allow(remote over limit with default clock) = true")
		}
	})

	t.Run("Should rate-limit each remote source while exempting local access (UT-045)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		limiter := NewAuthFailureLimiter(2, time.Minute, func() time.Time { return now })
		limiter.Failure("198.51.100.8", false)
		limiter.Failure("198.51.100.8", false)
		if limiter.Allow("198.51.100.8", false) {
			t.Fatal("Allow(remote over limit) = true")
		}
		if !limiter.Allow("198.51.100.9", false) || !limiter.Allow("198.51.100.8", true) {
			t.Fatal("independent remote source or local exemption was denied")
		}
		now = now.Add(2 * time.Minute)
		if !limiter.Allow("198.51.100.8", false) {
			t.Fatal("Allow(remote after window) = false")
		}
	})

	t.Run("Should bound tracked remote sources under distributed probing", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		limiter := NewAuthFailureLimiter(2, time.Minute, func() time.Time { return now })
		for index := range maxTrackedAuthSources {
			limiter.Failure(fmt.Sprintf("source-%d", index), false)
		}
		if limiter.Allow("one-more-source", false) {
			t.Fatal("Allow(new source at tracking capacity) = true")
		}
		if len(limiter.failures) != maxTrackedAuthSources {
			t.Fatalf("tracked sources = %d, want bounded %d", len(limiter.failures), maxTrackedAuthSources)
		}
		now = now.Add(2 * time.Minute)
		if !limiter.Allow("one-more-source", false) {
			t.Fatal("Allow(new source after expiry pruning) = false")
		}
	})
}
