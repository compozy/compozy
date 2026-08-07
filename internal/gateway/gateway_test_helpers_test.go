package gateway

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"time"
)

type testCallLog struct {
	mu    sync.Mutex
	calls []string
}

func (l *testCallLog) add(call string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, call)
}

func (l *testCallLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.calls...)
}

type testStore struct {
	mu            sync.Mutex
	snapshot      Snapshot
	log           *testCallLog
	transitionErr error
}

func (s *testStore) Snapshot(context.Context) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneTestSnapshot(s.snapshot), nil
}

func (s *testStore) Transition(_ context.Context, req TransitionRequest, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.log != nil {
		s.log.add("persist")
	}
	if s.transitionErr != nil {
		return false, s.transitionErr
	}
	switch req.Target {
	case TargetProvider:
		for i := range s.snapshot.Providers {
			row := &s.snapshot.Providers[i]
			if row.ProviderName != req.Provider.Name || row.Tier != req.Tier {
				continue
			}
			if row.Generation != req.ExpectedGeneration {
				return false, ErrGenerationConflict
			}
			if row.Desired == req.Desired {
				return false, nil
			}
			row.Desired = req.Desired
			row.Generation++
			if req.Desired == DesiredDisabled {
				row.Observed = ProviderDown
				row.LastError = ""
			}
			return true, nil
		}
		if req.ExpectedGeneration != 0 {
			return false, ErrGenerationConflict
		}
		if req.Desired == DesiredDisabled {
			return false, nil
		}
		for _, row := range s.snapshot.Providers {
			if row.Tier == req.Tier && row.Desired == DesiredEnabled {
				return false, ErrTierProviderConflict
			}
		}
		s.snapshot.Providers = append(s.snapshot.Providers, ProviderActivation{
			ProviderName: req.Provider.Name, Tier: req.Tier,
			Desired: req.Desired, Observed: ProviderDown, Generation: 1,
		})
		return true, nil
	case TargetSurface:
		for i := range s.snapshot.Surfaces {
			row := &s.snapshot.Surfaces[i]
			if row.Surface != req.Surface || row.Tier != req.Tier {
				continue
			}
			if row.Generation != req.ExpectedGeneration {
				return false, ErrGenerationConflict
			}
			if row.Desired == req.Desired {
				return false, nil
			}
			row.Desired = req.Desired
			row.Generation++
			if req.Desired == DesiredDisabled {
				row.Observed = SurfaceOff
				row.ConsentedAt = time.Time{}
			} else if req.Consent {
				row.ConsentedAt = now
			}
			return true, nil
		}
		if req.ExpectedGeneration != 0 {
			return false, ErrGenerationConflict
		}
		if req.Desired == DesiredDisabled {
			return false, nil
		}
		exposure := SurfaceExposure{
			Surface: req.Surface, Tier: req.Tier, Desired: req.Desired,
			Observed: SurfaceOff, Generation: 1,
		}
		if req.Consent {
			exposure.ConsentedAt = now
		}
		s.snapshot.Surfaces = append(s.snapshot.Surfaces, exposure)
		return true, nil
	default:
		return false, errors.New("test store: unsupported target")
	}
}

func (s *testStore) DisableAll(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range s.snapshot.Providers {
		row := &s.snapshot.Providers[i]
		if row.Desired != DesiredDisabled || row.Observed != ProviderDown {
			row.Desired = DesiredDisabled
			row.Observed = ProviderDown
			row.Generation++
			row.LastError = ""
			changed = true
		}
	}
	for i := range s.snapshot.Surfaces {
		row := &s.snapshot.Surfaces[i]
		if row.Desired != DesiredDisabled || row.Observed != SurfaceOff || !row.ConsentedAt.IsZero() {
			row.Desired = DesiredDisabled
			row.Observed = SurfaceOff
			row.Generation++
			row.ConsentedAt = time.Time{}
			changed = true
		}
	}
	return changed, nil
}

func (s *testStore) SetObserved(
	_ context.Context,
	plan TierPlan,
	providerObserved ProviderObservedState,
	surfaceObserved SurfaceObservedState,
	at time.Time,
	cause string,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if plan.Provider == nil || !providerGenerationCurrent(s.snapshot, *plan.Provider) {
		return false, nil
	}
	for _, expected := range plan.Surfaces {
		if !surfaceGenerationCurrent(s.snapshot, expected) {
			return false, nil
		}
	}
	for i := range s.snapshot.Providers {
		row := &s.snapshot.Providers[i]
		if row.ProviderName == plan.Provider.ProviderName && row.Tier == plan.Provider.Tier {
			row.Observed = providerObserved
			row.LastHealthAt = at
			row.LastError = cause
		}
	}
	for _, expected := range plan.Surfaces {
		for i := range s.snapshot.Surfaces {
			row := &s.snapshot.Surfaces[i]
			if row.Surface == expected.Surface && row.Tier == expected.Tier {
				row.Observed = surfaceObserved
			}
		}
	}
	return true, nil
}

func (s *testStore) mutateSurfaceGeneration(surface Surface, tier Tier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.snapshot.Surfaces {
		row := &s.snapshot.Surfaces[i]
		if row.Surface == surface && row.Tier == tier {
			row.Generation++
		}
	}
}

type testEffects struct {
	mu         sync.Mutex
	log        *testCallLog
	failAt     string
	onCall     func(string)
	advertised map[Tier]bool
}

func newTestEffects(log *testCallLog) *testEffects {
	return &testEffects{log: log, advertised: map[Tier]bool{}}
}

func (e *testEffects) call(name string) error {
	if e.log != nil {
		e.log.add(name)
	}
	if e.onCall != nil {
		e.onCall(name)
	}
	if e.failAt == name {
		return errors.New("injected " + name + " failure")
	}
	return nil
}

func (e *testEffects) Bind(_ context.Context, _ Tier) (netip.AddrPort, error) {
	if err := e.call("bind"); err != nil {
		return netip.AddrPort{}, err
	}
	return netip.MustParseAddrPort("127.0.0.1:43210"), nil
}

func (e *testEffects) Unbind(context.Context, Tier) error { return e.call("unbind") }

func (e *testEffects) Establish(_ context.Context, activation ProviderActivation, _ netip.AddrPort) (
	Reachability,
	error,
) {
	if err := e.call("establish"); err != nil {
		return Reachability{}, err
	}
	return Reachability{
		Tier: activation.Tier, Addresses: []string{"https://gateway.example.test"}, Health: HealthHealthy,
	}, nil
}

func (e *testEffects) Teardown(context.Context, ProviderActivation) error {
	return e.call("teardown")
}

func (e *testEffects) Verify(context.Context, Reachability) error { return e.call("verify") }

func (e *testEffects) Advertise(_ context.Context, reachability Reachability, _ []Surface) error {
	if err := e.call("advertise"); err != nil {
		return err
	}
	e.mu.Lock()
	e.advertised[reachability.Tier] = true
	e.mu.Unlock()
	return nil
}

func (e *testEffects) Withdraw(_ context.Context, tier Tier) error {
	if err := e.call("withdraw"); err != nil {
		return err
	}
	e.mu.Lock()
	e.advertised[tier] = false
	e.mu.Unlock()
	return nil
}

func (e *testEffects) isAdvertised(tier Tier) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.advertised[tier]
}

func cloneTestSnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{
		Providers: slices.Clone(snapshot.Providers),
		Surfaces:  slices.Clone(snapshot.Surfaces),
		Devices:   slices.Clone(snapshot.Devices),
	}
}

func enabledPrivateSnapshot() Snapshot {
	return Snapshot{
		Providers: []ProviderActivation{{
			ProviderName: "connectivity-test", Tier: TierPrivate,
			Desired: DesiredEnabled, Observed: ProviderDown, Generation: 1,
		}},
		Surfaces: []SurfaceExposure{{
			Surface: SurfaceOperatorUI, Tier: TierPrivate,
			Desired: DesiredEnabled, Observed: SurfaceOff, Generation: 1,
		}},
		Devices: []DeviceSession{{ID: "dev_test", Name: "Test device"}},
	}
}

func activeAuthGate(context.Context) (bool, error) { return true, nil }
