package gateway

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPolicyAuthorityAndRefusals(t *testing.T) {
	t.Parallel()

	t.Run("Should persist desired state with one generation advance and no config intent [UT-010]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		status, err := policy.Transition(testContext(t), providerRequest(DesiredEnabled, 0))
		if err != nil {
			t.Fatalf("Transition() error = %v", err)
		}
		if !status.Changed {
			t.Fatal("Transition() Changed = false, want true")
		}
		snapshot, err := store.Snapshot(testContext(t))
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if got := snapshot.Providers[0].Generation; got != 1 {
			t.Fatalf("generation = %d, want 1", got)
		}
		if got := snapshot.Providers[0].Desired; got != DesiredEnabled {
			t.Fatalf("desired = %q, want %q", got, DesiredEnabled)
		}
	})

	t.Run("Should refuse every enable transition while the immutable ceiling is false [UT-011]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{}
		policy := newTestPolicy(t, store, newTestEffects(nil), false)
		requests := []TransitionRequest{
			providerRequest(DesiredEnabled, 0),
			{Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI, Desired: DesiredEnabled},
		}
		for _, req := range requests {
			_, err := policy.Transition(testContext(t), req)
			if !errors.Is(err, ErrExposureRefused) {
				t.Fatalf("Transition(%s) error = %v, want ErrExposureRefused", req.Target, err)
			}
			if !strings.Contains(err.Error(), "gateway.enabled") || !strings.Contains(err.Error(), "fix:") {
				t.Fatalf("Transition(%s) error = %q, want ceiling cause and fix", req.Target, err)
			}
		}
	})

	t.Run("Should disable every durable tier when the ceiling flips false [UT-011]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		if err := policy.SetEnabled(testContext(t), false); err != nil {
			t.Fatalf("SetEnabled(false) error = %v", err)
		}
		status, err := policy.Status(testContext(t))
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Enabled || status.Providers[0].Desired != DesiredDisabled ||
			status.Providers[0].Observed != ProviderDown || status.Surfaces[0].Desired != DesiredDisabled ||
			status.Surfaces[0].Observed != SurfaceOff || status.Tiers[0].Advertised {
			t.Fatalf("status after ceiling disable = %#v, want fully disabled", status)
		}
	})

	t.Run("Should refuse remote reachability while authentication is inactive [UT-012]", func(t *testing.T) {
		t.Parallel()
		policy, err := NewPolicy(&testStore{}, newTestEffects(nil), true)
		if err != nil {
			t.Fatalf("NewPolicy() error = %v", err)
		}
		_, err = policy.Transition(testContext(t), providerRequest(DesiredEnabled, 0))
		if !errors.Is(err, ErrExposureRefused) {
			t.Fatalf("Transition() error = %v, want ErrExposureRefused", err)
		}
		if !strings.Contains(err.Error(), "authentication model is inactive") ||
			!strings.Contains(err.Error(), "fix:") {
			t.Fatalf("Transition() error = %q, want auth cause and fix", err)
		}
	})

	t.Run("Should require a paired device before public operator UI exposure [UT-013]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: Snapshot{Providers: []ProviderActivation{{
			ProviderName: "connectivity-test", Tier: TierPublic,
			Desired: DesiredEnabled, Observed: ProviderDown, Generation: 1,
		}}}}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		_, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPublic, Surface: SurfaceOperatorUI,
			Desired: DesiredEnabled, Consent: true,
		})
		if !errors.Is(err, ErrExposureRefused) || !strings.Contains(err.Error(), "paired device") {
			t.Fatalf("Transition() error = %v, want paired-device refusal", err)
		}
	})

	t.Run("Should refuse webhook ingress outside the public tier", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: Snapshot{Providers: []ProviderActivation{{
			ProviderName: "connectivity-test", Tier: TierPrivate,
			Desired: DesiredEnabled, Observed: ProviderDown, Generation: 1,
		}}}}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)

		_, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceWebhookIngress,
			Desired: DesiredEnabled,
		})
		if !errors.Is(err, ErrExposureRefused) || !strings.Contains(err.Error(), "public tier") {
			t.Fatalf("Transition() error = %v, want public-tier refusal", err)
		}
		snapshot, snapshotErr := store.Snapshot(testContext(t))
		if snapshotErr != nil {
			t.Fatalf("Snapshot() error = %v", snapshotErr)
		}
		if len(snapshot.Surfaces) != 0 {
			t.Fatalf("snapshot surfaces = %#v, want invalid transition not persisted", snapshot.Surfaces)
		}
	})

	t.Run("Should reject removed legacy exposure semantics without interpretation [UT-014]", func(t *testing.T) {
		t.Parallel()
		policy := newTestPolicy(t, &testStore{}, newTestEffects(nil), true)
		_, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetProvider, Tier: TierPrivate, Desired: DesiredEnabled,
			LegacyMode: "public-ui",
		})
		if !errors.Is(err, ErrExposureRefused) || !strings.Contains(err.Error(), "legacy exposure mode") {
			t.Fatalf("Transition() error = %v, want legacy-semantics refusal", err)
		}
	})

	t.Run("Should surface an effect refusal in the transition status", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: Snapshot{
			Providers: []ProviderActivation{{
				ProviderName: "connectivity-test", Tier: TierPrivate,
				Desired: DesiredEnabled, Observed: ProviderDown, Generation: 1,
			}},
			Devices: []DeviceSession{{ID: "dev_test"}},
		}}
		effects := newTestEffects(nil)
		effects.failAt = "bind"
		effects.failErr = refusalError("the private listener is unavailable", "repair the listener and retry")
		policy := newTestPolicy(t, store, effects, true)

		status, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredEnabled,
		})
		if !errors.Is(err, ErrExposureRefused) {
			t.Fatalf("Transition() error = %v, want ErrExposureRefused", err)
		}
		if status.Refusal == nil || status.Refusal.Cause != "the private listener is unavailable" ||
			status.Refusal.Fix != "repair the listener and retry" {
			t.Fatalf("Transition() refusal = %#v", status.Refusal)
		}
	})
}

func TestPolicyGenerationFencingAndEffects(t *testing.T) {
	t.Parallel()

	t.Run("Should reject the losing concurrent transition on generation [UT-015]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		start := make(chan struct{})
		errs := make(chan error, 2)
		var ready sync.WaitGroup
		ready.Add(2)
		for range 2 {
			go func() {
				ready.Done()
				<-start
				_, err := policy.Transition(context.Background(), providerRequest(DesiredEnabled, 0))
				errs <- err
			}()
		}
		ready.Wait()
		close(start)
		var success, conflict int
		for range 2 {
			err := <-errs
			switch {
			case err == nil:
				success++
			case errors.Is(err, ErrGenerationConflict):
				conflict++
			default:
				t.Fatalf("Transition() error = %v", err)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatalf("success/conflict = %d/%d, want 1/1", success, conflict)
		}
	})

	t.Run("Should discard an effect completed for a stale generation [UT-016]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		log := &testCallLog{}
		effects := newTestEffects(log)
		effects.onCall = func(call string) {
			if call == "establish" {
				store.mutateSurfaceGeneration(SurfaceOperatorUI, TierPrivate)
			}
		}
		policy := newTestPolicy(t, store, effects, true)
		_, err := policy.Reconcile(testContext(t))
		if !errors.Is(err, ErrStaleGeneration) {
			t.Fatalf("Reconcile() error = %v, want ErrStaleGeneration", err)
		}
		if effects.isAdvertised(TierPrivate) {
			t.Fatal("private tier advertised after stale effect")
		}
		if calls := log.snapshot(); !slices.Contains(calls, "teardown") || !slices.Contains(calls, "unbind") {
			t.Fatalf("calls = %#v, want reverse compensation", calls)
		}
	})

	t.Run("Should refuse an enabled tier plan without a provider", func(t *testing.T) {
		t.Parallel()
		reconciler := NewReconciler(&testStore{}, newTestEffects(nil))
		err := reconciler.Reconcile(testContext(t), ExposurePlan{Tiers: []TierPlan{{
			Tier: TierPrivate, Enabled: true,
		}}})
		if !errors.Is(err, ErrExposureRefused) {
			t.Fatalf("Reconcile() error = %v, want ErrExposureRefused", err)
		}
	})

	t.Run("Should apply persist bind establish verify advertise in fixed order [UT-017]", func(t *testing.T) {
		t.Parallel()
		log := &testCallLog{}
		store := &testStore{
			log: log,
			snapshot: Snapshot{
				Providers: []ProviderActivation{{
					ProviderName: "connectivity-test", Tier: TierPrivate,
					Desired: DesiredEnabled, Observed: ProviderDown, Generation: 1,
				}},
				Devices: []DeviceSession{{ID: "dev_test"}},
			},
		}
		effects := newTestEffects(log)
		policy := newTestPolicy(t, store, effects, true)
		_, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredEnabled,
		})
		if err != nil {
			t.Fatalf("Transition() error = %v", err)
		}
		want := []string{"persist", "bind", "establish", "verify", "advertise"}
		if got := log.snapshot(); len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
			t.Fatalf("effect prefix = %#v, want %#v", got, want)
		}
		wantStates := []ProviderObservedState{ProviderEstablishing, ProviderUp}
		if got := effects.stateChanges(); !slices.Equal(got, wantStates) {
			t.Fatalf("provider state events = %#v, want %#v", got, wantStates)
		}
	})

	t.Run("Should compensate each failed effect without external reachability [UT-018]", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			failAt string
			want   []string
		}{
			{failAt: "persist", want: []string{"persist"}},
			{failAt: "bind", want: []string{"persist", "bind"}},
			{failAt: "establish", want: []string{"persist", "bind", "establish", "teardown", "unbind"}},
			{failAt: "verify", want: []string{"persist", "bind", "establish", "verify", "teardown", "unbind"}},
			{failAt: "advertise", want: []string{
				"persist", "bind", "establish", "verify", "advertise", "withdraw", "teardown", "unbind",
			}},
		}
		for _, tt := range cases {
			t.Run("Should compensate "+tt.failAt, func(t *testing.T) {
				t.Parallel()
				log := &testCallLog{}
				store := &testStore{
					log: log,
					snapshot: Snapshot{
						Providers: []ProviderActivation{{
							ProviderName: "connectivity-test", Tier: TierPrivate,
							Desired: DesiredEnabled, Observed: ProviderDown, Generation: 1,
						}},
						Devices: []DeviceSession{{ID: "dev_test"}},
					},
				}
				effects := newTestEffects(log)
				if tt.failAt == "persist" {
					store.transitionErr = errors.New("injected persist failure")
				} else {
					effects.failAt = tt.failAt
				}
				policy := newTestPolicy(t, store, effects, true)
				_, err := policy.Transition(testContext(t), TransitionRequest{
					Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
					Desired: DesiredEnabled,
				})
				if err == nil {
					t.Fatal("Transition() error = nil, want injected failure")
				}
				if got := log.snapshot(); len(got) < len(tt.want) || !slices.Equal(got[:len(tt.want)], tt.want) {
					t.Fatalf("call prefix = %#v, want %#v", got, tt.want)
				}
				if effects.isAdvertised(TierPrivate) {
					t.Fatal("private tier advertised after failed transition")
				}
			})
		}
	})

	t.Run("Should redact and bound a provider failure before persistence", func(t *testing.T) {
		t.Parallel()
		const secret = "sk-gateway-persisted-secret"
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		effects.failAt = "verify"
		effects.failErr = errors.New("api_key=" + secret + strings.Repeat("x", 4096))
		policy := newTestPolicy(t, store, effects, true)

		if _, err := policy.Reconcile(testContext(t)); !errors.Is(err, ErrProviderDegraded) {
			t.Fatalf("Reconcile() error = %v, want ErrProviderDegraded", err)
		}
		snapshot, err := store.Snapshot(testContext(t))
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		cause := snapshot.Providers[0].LastError
		if strings.Contains(cause, secret) {
			t.Fatalf("persisted LastError leaked secret: %q", cause)
		}
		if len(cause) > 2048 {
			t.Fatalf("persisted LastError length = %d, want <= 2048", len(cause))
		}
	})

	t.Run("Should degrade observed state when the provider state event cannot be stored", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		effects.stateErrAt = ProviderUp
		policy := newTestPolicy(t, store, effects, true)

		if _, err := policy.Reconcile(testContext(t)); err == nil {
			t.Fatal("Reconcile() error = nil, want provider state event failure")
		}
		snapshot, err := store.Snapshot(testContext(t))
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if snapshot.Providers[0].Observed != ProviderDegraded ||
			snapshot.Surfaces[0].Observed != SurfaceOff {
			t.Fatalf("observed state = %#v/%#v, want degraded/off", snapshot.Providers[0], snapshot.Surfaces[0])
		}
		if effects.isAdvertised(TierPrivate) {
			t.Fatal("provider remained advertised after state event failure")
		}
	})

	t.Run("Should restore the previous advertisement when listener replacement fails", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		policy := newTestPolicy(t, store, effects, true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile(initial) error = %v", err)
		}
		effects.failAt = "bind"
		if _, err := policy.Reconcile(testContext(t)); err == nil {
			t.Fatal("Reconcile(replacement) error = nil, want bind failure")
		}
		status, err := policy.Status(testContext(t))
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if !effects.isAdvertised(TierPrivate) || !status.Tiers[0].Advertised ||
			status.Tiers[0].ListenerAddress != "127.0.0.1:43210" {
			t.Fatalf("restored private tier = %#v, advertised=%t", status.Tiers[0], effects.isAdvertised(TierPrivate))
		}
	})

	t.Run("Should keep prior reachability when provider preflight refuses replacement [UT-061]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := &preflightTestEffects{testEffects: newTestEffects(nil)}
		reconciler := NewReconciler(store, effects)
		if err := reconciler.Reconcile(testContext(t), planSnapshot(store.snapshot)); err != nil {
			t.Fatalf("Reconcile(initial) error = %v", err)
		}
		effects.preflightErr = ErrDigestConfirmationRequired
		err := reconciler.Reconcile(testContext(t), planSnapshot(store.snapshot))
		if !errors.Is(err, ErrDigestConfirmationRequired) {
			t.Fatalf("Reconcile(replacement) error = %v, want ErrDigestConfirmationRequired", err)
		}
		if !effects.isAdvertised(TierPrivate) {
			t.Fatal("prior advertisement was withdrawn before failed authorization")
		}
		release, err := reconciler.Acquire(TierPrivate, SurfaceOperatorUI)
		if err != nil {
			t.Fatalf("Acquire() error = %v, want prior admission preserved", err)
		}
		release()
	})

	t.Run("Should withdraw, unbind, and automatically recover a degraded generation [UT-078]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		reconciler := NewReconciler(store, effects, WithProviderRecoveryBackoff(time.Millisecond, 5*time.Millisecond))
		t.Cleanup(func() {
			if err := reconciler.Close(testContext(t)); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		plan := planSnapshot(store.snapshot)
		if err := reconciler.Reconcile(testContext(t), plan); err != nil {
			t.Fatalf("Reconcile(initial) error = %v", err)
		}
		activation := *plan.Tiers[0].Provider
		if err := reconciler.handleProviderDegraded(
			testContext(t), activation, errors.New("token=secret-outage"),
		); err != nil {
			t.Fatalf("handleProviderDegraded() error = %v", err)
		}
		if effects.isAdvertised(TierPrivate) {
			t.Fatal("degraded provider remained advertised")
		}
		if _, err := reconciler.Acquire(TierPrivate, SurfaceOperatorUI); !errors.Is(err, ErrExposureRefused) {
			t.Fatalf("Acquire(degraded) error = %v, want ErrExposureRefused", err)
		}
		waitForProviderObserved(t, store, ProviderUp)
		if !effects.isAdvertised(TierPrivate) {
			t.Fatal("provider recovery did not restore advertisement")
		}
	})
}

func TestPolicyDisableSemantics(t *testing.T) {
	t.Parallel()

	t.Run("Should emit the provider down state after an operator disables it", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		policy := newTestPolicy(t, store, effects, true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		if _, err := policy.Transition(
			testContext(t),
			providerRequest(DesiredDisabled, 1),
		); err != nil {
			t.Fatalf("Transition(disable provider) error = %v", err)
		}
		want := []ProviderObservedState{ProviderEstablishing, ProviderUp, ProviderDown}
		if got := effects.stateChanges(); !slices.Equal(got, want) {
			t.Fatalf("provider state events = %#v, want %#v", got, want)
		}
	})

	t.Run("Should mark desired disabled and observed off immediately [UT-019]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		status, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredDisabled, ExpectedGeneration: 1,
		})
		if err != nil {
			t.Fatalf("Transition() error = %v", err)
		}
		if len(status.Surfaces) != 1 || status.Surfaces[0].Desired != DesiredDisabled ||
			status.Surfaces[0].Observed != SurfaceOff {
			t.Fatalf("surface status = %#v, want disabled/off", status.Surfaces)
		}
	})

	t.Run("Should let an admitted request finish and refuse the next after disable [UT-020]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		policy := newTestPolicy(t, store, effects, true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		release, err := policy.Acquire(TierPrivate, SurfaceOperatorUI)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		released := make(chan struct{})
		var releaseOnce sync.Once
		effects.onCall = func(call string) {
			if call != "withdraw" {
				return
			}
			releaseOnce.Do(func() {
				release()
				close(released)
			})
		}
		if _, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredDisabled, ExpectedGeneration: 1,
		}); err != nil {
			t.Fatalf("Transition(disable) error = %v", err)
		}
		select {
		case <-released:
		default:
			t.Fatal("admission release remained blocked during listener withdrawal")
		}
		if _, err := policy.Acquire(TierPrivate, SurfaceOperatorUI); !errors.Is(err, ErrExposureRefused) {
			t.Fatalf("Acquire() after disable error = %v, want ErrExposureRefused", err)
		}
	})

	t.Run("Should report a second disable as unchanged [UT-021]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		first, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredDisabled, ExpectedGeneration: 1,
		})
		if err != nil || !first.Changed {
			t.Fatalf("first disable = (%#v, %v), want changed", first, err)
		}
		second, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredDisabled, ExpectedGeneration: 2,
		})
		if err != nil {
			t.Fatalf("second disable error = %v", err)
		}
		if second.Changed {
			t.Fatal("second disable Changed = true, want false")
		}
	})

	t.Run("Should preserve possible reachability but close admission when withdraw fails", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		policy := newTestPolicy(t, store, effects, true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile(initial) error = %v", err)
		}
		effects.failAt = "withdraw"
		status, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredDisabled, ExpectedGeneration: 1,
		})
		if err == nil {
			t.Fatal("Transition(disable) error = nil, want withdraw failure")
		}
		if !effects.isAdvertised(TierPrivate) || !status.Tiers[0].Advertised {
			t.Fatalf(
				"status after failed withdraw = %#v, provider advertised=%t",
				status.Tiers[0],
				effects.isAdvertised(TierPrivate),
			)
		}
		if _, err := policy.Acquire(TierPrivate, SurfaceOperatorUI); !errors.Is(err, ErrExposureRefused) {
			t.Fatalf("Acquire() after desired disable error = %v, want ErrExposureRefused", err)
		}
		effects.failAt = ""
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile(retry) error = %v", err)
		}
		status, err = policy.Status(testContext(t))
		if err != nil {
			t.Fatalf("Status(after retry) error = %v", err)
		}
		if status.Tiers[0].Advertised || status.Tiers[0].ListenerAddress != "" {
			t.Fatalf("private tier after cleanup retry = %#v, want unreachable", status.Tiers[0])
		}
	})

	t.Run("Should retain a stable refusal after a ceiling-disable cleanup failure", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		policy := newTestPolicy(t, store, effects, true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
		effects.failAt = "withdraw"

		if err := policy.SetEnabled(testContext(t), false); err == nil {
			t.Fatal("SetEnabled(false) error = nil, want cleanup failure")
		}
		status, err := policy.Status(testContext(t))
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Refusal == nil || status.Refusal.Cause != "gateway disable did not complete" ||
			status.Refusal.Fix != "repair the failing listener or provider, then retry disabling the gateway" {
			t.Fatalf("Status().Refusal = %#v", status.Refusal)
		}
	})

	t.Run("Should retain the bound listener in status when unbind fails", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: enabledPrivateSnapshot()}
		effects := newTestEffects(nil)
		policy := newTestPolicy(t, store, effects, true)
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile(initial) error = %v", err)
		}
		effects.failAt = "unbind"
		status, err := policy.Transition(testContext(t), TransitionRequest{
			Target: TargetSurface, Tier: TierPrivate, Surface: SurfaceOperatorUI,
			Desired: DesiredDisabled, ExpectedGeneration: 1,
		})
		if err == nil {
			t.Fatal("Transition(disable) error = nil, want unbind failure")
		}
		if status.Tiers[0].Advertised || status.Tiers[0].ListenerAddress != "127.0.0.1:43210" {
			t.Fatalf("private tier after failed unbind = %#v, want withdrawn but still bound", status.Tiers[0])
		}
		effects.failAt = ""
		if _, err := policy.Reconcile(testContext(t)); err != nil {
			t.Fatalf("Reconcile(retry) error = %v", err)
		}
		status, err = policy.Status(testContext(t))
		if err != nil {
			t.Fatalf("Status(after retry) error = %v", err)
		}
		if status.Tiers[0].ListenerAddress != "" {
			t.Fatalf("private tier after unbind retry = %#v, want no listener", status.Tiers[0])
		}
	})
}

func newTestPolicy(t *testing.T, store Store, effects EffectDriver, enabled bool) Policy {
	t.Helper()
	policy, err := NewPolicy(
		store,
		effects,
		enabled,
		WithAuthGate(AuthGateFunc(activeAuthGate)),
		WithClock(func() time.Time { return time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC) }),
	)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	return policy
}

func providerRequest(desired DesiredState, expected uint64) TransitionRequest {
	return TransitionRequest{
		Target: TargetProvider, Tier: TierPrivate, Desired: desired,
		ExpectedGeneration: expected,
		Provider:           ProviderIdentity{Name: "connectivity-test", InstallSource: "bundled"},
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

type preflightTestEffects struct {
	*testEffects
	preflightErr error
}

func (e *preflightTestEffects) PreflightProvider(context.Context, ProviderActivation) error {
	return e.preflightErr
}

func waitForProviderObserved(t *testing.T, store *testStore, want ProviderObservedState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := store.Snapshot(testContext(t))
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if len(snapshot.Providers) == 1 && snapshot.Providers[0].Observed == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	snapshot, err := store.Snapshot(testContext(t))
	if err != nil {
		t.Fatalf("Snapshot(final) error = %v", err)
	}
	t.Fatalf("provider snapshot = %#v, want observed %q", snapshot.Providers, want)
}
