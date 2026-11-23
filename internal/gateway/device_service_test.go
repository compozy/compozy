package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/workspaceaccess"
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

	t.Run("Should apply live pairing and stream-ticket limits to new artifacts", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		if err := service.ReconfigureLimits(1, 10*time.Minute, 45*time.Second); err != nil {
			t.Fatalf("ReconfigureLimits() error = %v", err)
		}
		pairing, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		if want := now.Add(10 * time.Minute); !pairing.ExpiresAt.Equal(want) {
			t.Fatalf("pairing expiry = %s, want %s", pairing.ExpiresAt, want)
		}
		if _, err := service.MintPairing(
			t.Context(), PairingRequest{Source: PairingSourceLocal},
		); !errors.Is(err, ErrPairingLimit) {
			t.Fatalf("MintPairing(over live limit) error = %v, want ErrPairingLimit", err)
		}
		issued, err := service.RedeemPairing(t.Context(), RedeemRequest{
			Artifact: pairing.Artifact, Name: "Live limits", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		})
		if err != nil {
			t.Fatalf("RedeemPairing() error = %v", err)
		}
		ticket, err := service.MintStreamTicket(t.Context(), issued.Device.ID)
		if err != nil {
			t.Fatalf("MintStreamTicket() error = %v", err)
		}
		if want := now.Add(45 * time.Second); !ticket.ExpiresAt.Equal(want) {
			t.Fatalf("stream ticket expiry = %s, want %s", ticket.ExpiresAt, want)
		}
	})

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
			devices[0].PairingOrigin != string(PairingSourceLocal) || !devices[0].LastSeenAt.Equal(now) ||
			devices[0].CreatedAt.IsZero() {
			t.Fatalf("device lifecycle payload = %#v / %#v", renamed, devices)
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

	// Invariant: removing an active device revokes it, closes its streams, appends the audit
	// event, and deletes the record; a failed append leaves the record in place.
	// Owning layer: gateway device service removal and its audit sink boundary.
	// Canonical suite: TestDeviceServiceLifecycle.
	t.Run("Should revoke close streams and delete an active device", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		store := newMemoryDeviceStore()
		registry := &recordingConnectionRegistry{store: store}
		events := &recordingDeviceEventSink{}
		service, err := NewDeviceService(
			store,
			WithDeviceClock(func() time.Time { return now }),
			WithDeviceEntropy(&incrementingReader{}),
			WithConnectionRegistry(registry),
			WithDeviceEvents(events),
		)
		if err != nil {
			t.Fatalf("NewDeviceService() error = %v", err)
		}
		issued, err := issueTestDevice(t.Context(), service, "Laptop", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		removed, err := service.RemoveDevice(t.Context(), issued.Device.ID)
		if err != nil {
			t.Fatalf("RemoveDevice() error = %v", err)
		}
		if !removed.Revoked || removed.Canceled != 2 || removed.Device.RevokeEpoch != 1 ||
			removed.Device.RevokedAt.IsZero() {
			t.Fatalf("RemoveDevice() result = %#v", removed)
		}
		if !registry.revokedBeforeCall {
			t.Fatal("live streams were canceled before the revoke was durable")
		}
		if _, exists := store.byID[issued.Device.ID]; exists {
			t.Fatal("device record survived removal")
		}
		recorded := events.recorded()
		if len(recorded) != 1 || recorded[0].Device.ID != issued.Device.ID ||
			recorded[0].Device.PairingOrigin != string(PairingSourceLocal) ||
			recorded[0].ActorKind != string(workspaceaccess.ActorHuman) ||
			recorded[0].RemovedAt.IsZero() {
			t.Fatalf("recorded removal event = %#v", recorded)
		}
	})

	t.Run("Should delete an already revoked device without a second revocation", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		store := newMemoryDeviceStore()
		registry := &recordingConnectionRegistry{store: store}
		events := &recordingDeviceEventSink{}
		service, err := NewDeviceService(
			store,
			WithDeviceClock(func() time.Time { return now }),
			WithDeviceEntropy(&incrementingReader{}),
			WithConnectionRegistry(registry),
			WithDeviceEvents(events),
		)
		if err != nil {
			t.Fatalf("NewDeviceService() error = %v", err)
		}
		issued, err := issueTestDevice(t.Context(), service, "Laptop", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		if _, err := service.RevokeDevice(t.Context(), issued.Device.ID); err != nil {
			t.Fatalf("RevokeDevice() error = %v", err)
		}
		removed, err := service.RemoveDevice(t.Context(), issued.Device.ID)
		if err != nil {
			t.Fatalf("RemoveDevice() error = %v", err)
		}
		if removed.Revoked || removed.Canceled != 0 || removed.Device.RevokeEpoch != 1 {
			t.Fatalf("RemoveDevice(revoked) result = %#v", removed)
		}
		if len(registry.canceled) != 1 {
			t.Fatalf("stream cancellations = %d, want only the earlier revoke", len(registry.canceled))
		}
	})

	t.Run("Should report not found when removing an unknown device", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		events := &recordingDeviceEventSink{}
		service := newDeviceServiceOnly(t, &now, WithDeviceEvents(events))
		if _, err := service.RemoveDevice(t.Context(), "dev_missing"); !errors.Is(err, ErrDeviceNotFound) {
			t.Fatalf("RemoveDevice(missing) error = %v, want ErrDeviceNotFound", err)
		}
		if len(events.recorded()) != 0 {
			t.Fatal("a missing device still recorded a removal event")
		}
	})

	t.Run("Should keep the record when the removal event cannot be appended", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		appendErr := errors.New("event ledger unavailable")
		events := &recordingDeviceEventSink{err: appendErr}
		store := newMemoryDeviceStore()
		service, err := NewDeviceService(
			store,
			WithDeviceClock(func() time.Time { return now }),
			WithDeviceEntropy(&incrementingReader{}),
			WithDeviceEvents(events),
		)
		if err != nil {
			t.Fatalf("NewDeviceService() error = %v", err)
		}
		issued, err := issueTestDevice(t.Context(), service, "Laptop", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		if _, err := service.RemoveDevice(t.Context(), issued.Device.ID); !errors.Is(err, appendErr) {
			t.Fatalf("RemoveDevice() error = %v, want the append failure", err)
		}
		if _, exists := store.byID[issued.Device.ID]; !exists {
			t.Fatal("failed removal deleted the device record")
		}
	})

	t.Run("Should refuse removal when no event sink is configured", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service, store := newDeviceServiceFixture(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "No audit", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		removalErr := func() error {
			_, err := service.RemoveDevice(t.Context(), issued.Device.ID)
			return err
		}()
		if removalErr == nil ||
			!strings.Contains(removalErr.Error(), "device removal requires a configured event sink") {
			t.Fatalf(
				"RemoveDevice() error = %v, want the missing event sink error",
				removalErr,
			)
		}
		if _, exists := store.byID[issued.Device.ID]; !exists {
			t.Fatal("removal without an event sink deleted the device record")
		}
	})

	t.Run("Should remove the device serving the current request", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now, WithDeviceEvents(&recordingDeviceEventSink{}))
		issued, err := issueTestDevice(t.Context(), service, "Self", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		mutationCtx, err := service.AcquireMutation(
			ContextWithDevice(t.Context(), issued.Device),
			issued.Device.ID,
			issued.Device.RevokeEpoch,
		)
		if err != nil {
			t.Fatalf("AcquireMutation() error = %v", err)
		}
		t.Cleanup(func() { remotehost.ReleaseMutation(mutationCtx) })
		removed, err := service.RemoveDevice(mutationCtx, issued.Device.ID)
		if err != nil {
			t.Fatalf("RemoveDevice(self) error = %v", err)
		}
		if removed.Device.ID != issued.Device.ID {
			t.Fatalf("removed self snapshot = %#v", removed.Device)
		}
		if _, err := service.Authenticate(t.Context(), issued.Credential); !errors.Is(
			err,
			ErrDeviceUnauthenticated,
		) {
			t.Fatalf("Authenticate(after self removal) error = %v", err)
		}
	})

	t.Run("Should fence removal by the acting device epoch", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		store := newMemoryDeviceStore()
		service, err := NewDeviceService(
			store,
			WithDeviceClock(func() time.Time { return now }),
			WithDeviceEntropy(&incrementingReader{}),
		)
		if err != nil {
			t.Fatalf("NewDeviceService() error = %v", err)
		}
		actor, err := issueTestDevice(t.Context(), service, "Actor", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice(actor) error = %v", err)
		}
		target, err := issueTestDevice(t.Context(), service, "Target", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice(target) error = %v", err)
		}
		if _, err := service.RevokeDevice(t.Context(), actor.Device.ID); err != nil {
			t.Fatalf("RevokeDevice(actor) error = %v", err)
		}
		staleCtx := ContextWithDevice(t.Context(), actor.Device)
		if _, err := service.RemoveDevice(staleCtx, target.Device.ID); !errors.Is(err, remotehost.ErrDeviceRevoked) {
			t.Fatalf("RemoveDevice(stale actor) error = %v, want remotehost.ErrDeviceRevoked", err)
		}
		if _, exists := store.byID[target.Device.ID]; !exists {
			t.Fatal("a revoked actor deleted another device record")
		}
	})

	t.Run("Should release idle mutation-gate state after repeated committed revocations", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)

		for index := range 8 {
			issued, err := issueTestDevice(
				t.Context(), service, fmt.Sprintf("Device %d", index), ActorKindOperatorDevice,
			)
			if err != nil {
				t.Fatalf("issueTestDevice(%d) error = %v", index, err)
			}
			if _, err := service.RevokeDevice(t.Context(), issued.Device.ID); err != nil {
				t.Fatalf("RevokeDevice(%d) error = %v", index, err)
			}
		}

		service.mutations.mu.Lock()
		tracked := len(service.mutations.devices)
		service.mutations.mu.Unlock()
		if tracked != 0 {
			t.Fatalf("tracked mutation-gate devices = %d, want 0 after committed revocations", tracked)
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
		); !errors.Is(err, remotehost.ErrDeviceRevoked) {
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
		t.Cleanup(func() { remotehost.ReleaseMutation(mutationCtx) })

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
		); !errors.Is(err, remotehost.ErrDeviceRevoked) {
			t.Fatalf("RevalidateForCommit(sealed) error = %v, want remotehost.ErrDeviceRevoked", err)
		}
		remotehost.ReleaseMutation(mutationCtx)
		outcome := <-revoked
		if outcome.err != nil || !outcome.result.Changed || outcome.result.Device.RevokeEpoch != 1 {
			t.Fatalf("RevokeDevice() = (%#v, %v)", outcome.result, outcome.err)
		}
	})

	t.Run("Should cancel epoch revalidation and release the mutation lease", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		store := &cancelAwareDeviceStore{
			memoryDeviceStore: newMemoryDeviceStore(),
			startCh:           make(chan struct{}),
			release:           make(chan struct{}),
		}
		service, err := NewDeviceService(
			store,
			WithDeviceClock(func() time.Time { return now }),
			WithDeviceEntropy(&incrementingReader{}),
		)
		if err != nil {
			t.Fatalf("NewDeviceService() error = %v", err)
		}
		issued, err := issueTestDevice(t.Context(), service, "Device", ActorKindOperatorDevice)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		outcome := make(chan error, 1)
		go func() {
			_, acquireErr := service.AcquireMutation(ctx, issued.Device.ID, issued.Device.RevokeEpoch)
			outcome <- acquireErr
		}()
		<-store.startCh
		cancel()

		select {
		case acquireErr := <-outcome:
			if !errors.Is(acquireErr, context.Canceled) {
				t.Fatalf("AcquireMutation() error = %v, want context.Canceled", acquireErr)
			}
		case <-time.After(200 * time.Millisecond):
			close(store.release)
			acquireErr := <-outcome
			t.Fatalf("AcquireMutation() ignored cancellation; eventual error = %v", acquireErr)
		}

		service.mutations.mu.Lock()
		tracked := len(service.mutations.devices)
		service.mutations.mu.Unlock()
		if tracked != 0 {
			t.Fatalf("tracked mutation-gate devices = %d, want canceled lease released", tracked)
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
		if _, err := service.ConsumeStreamTicket(t.Context(), ticket.Ticket); !errors.Is(err, remotehost.ErrStreamTicketInvalid) {
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

type cancelAwareDeviceStore struct {
	*memoryDeviceStore
	started sync.Once
	startCh chan struct{}
	release chan struct{}
}

func (s *cancelAwareDeviceStore) RevalidateDeviceEpoch(ctx context.Context, id string, epoch uint64) error {
	s.started.Do(func() { close(s.startCh) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return s.memoryDeviceStore.RevalidateDeviceEpoch(ctx, id, epoch)
	}
}

// Invariant: app origins have one strict canonical representation; owner: gateway validation.
