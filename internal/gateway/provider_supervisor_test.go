package gateway

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

func TestProviderSupervisorEnableSafety(t *testing.T) {
	t.Parallel()

	t.Run("Should establish verify and advertise in order [UT-060]", func(t *testing.T) {
		t.Parallel()
		events := &supervisorEventLog{}
		audit := &supervisorAuditRecorder{}
		source := healthySupervisorSource(events)
		supervisor := newSupervisorForTest(t, bundledTrustResolver(events), bundledIdentityStore(events),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}, events: events},
			&supervisorVerifier{timeout: time.Second, events: events}, WithProviderAuditSink(audit))
		activation := supervisorActivation("provider-a")
		reachability, err := supervisor.Establish(testutil.Context(t), activation, testProviderBound())
		if err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		if err := supervisor.Verify(testutil.Context(t), reachability); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if err := supervisor.Advertise(testutil.Context(t), reachability, []Surface{SurfaceOperatorUI}); err != nil {
			t.Fatalf("Advertise() error = %v", err)
		}
		want := []string{"trust", "identity", "resolve", "establish", "verify"}
		if got := events.snapshot(); !slices.Equal(got, want) {
			t.Fatalf("enable order = %#v, want %#v", got, want)
		}
		if got := audit.verifiedSnapshot(); len(got) != 1 || got[0] != reachability.Endpoints[0] {
			t.Fatalf("verified endpoint audit = %#v, want %#v", got, reachability.Endpoints)
		}
		supervisor.mu.Lock()
		advertised := supervisor.sessions[TierPrivate].cancel != nil
		supervisor.mu.Unlock()
		if !advertised {
			t.Fatal("provider monitor is not active after advertisement")
		}
	})

	t.Run("Should bound provider activation independently from endpoint proof [UT-166]", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
		source := healthySupervisorSource(nil)
		supervisor := newSupervisorForTest(
			t,
			bundledTrustResolver(nil),
			bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}},
			&supervisorVerifier{timeout: 10 * time.Second},
			WithProviderSupervisorClock(func() time.Time { return now }),
			WithProviderEstablishTimeout(time.Minute),
		)
		if _, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		); err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		request := source.lastEstablishRequest()
		if want := now.Add(time.Minute); !request.Deadline.Equal(want) {
			t.Fatalf("provider deadline = %s, want %s", request.Deadline, want)
		}
	})

	t.Run("Should refuse an ephemeral public endpoint before verification [IT-051]", func(t *testing.T) {
		t.Parallel()
		source := healthySupervisorSource(nil)
		source.reachability.Tier = TierPublic
		source.reachability.Endpoints[0].Stability = EndpointEphemeral
		verifier := &supervisorVerifier{timeout: time.Second}
		supervisor := newSupervisorForTest(t, bundledTrustResolver(nil), bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}}, verifier)
		activation := supervisorActivation("provider-a")
		activation.Tier = TierPublic
		if _, err := supervisor.Establish(
			testutil.Context(t), activation, testProviderBound(),
		); !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Establish() error = %v, want ErrEndpointUnverified", err)
		}
		if source.teardownCount() != 1 {
			t.Fatalf("provider teardowns = %d, want 1", source.teardownCount())
		}
		if verifier.callCount() != 0 {
			t.Fatalf("endpoint verification calls = %d, want 0", verifier.callCount())
		}
	})

	t.Run("Should preserve prior state when authorization is abandoned [UT-061]", func(t *testing.T) {
		t.Parallel()
		trust := bundledTrustResolver(nil)
		source := healthySupervisorSource(nil)
		supervisor := newSupervisorForTest(t, trust, bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}},
			&supervisorVerifier{timeout: time.Second})
		activation := supervisorActivation("provider-a")
		reachability, err := supervisor.Establish(testutil.Context(t), activation, testProviderBound())
		if err != nil {
			t.Fatalf("Establish(initial) error = %v", err)
		}
		if err := supervisor.Verify(testutil.Context(t), reachability); err != nil {
			t.Fatalf("Verify(initial) error = %v", err)
		}
		if err := supervisor.Advertise(testutil.Context(t), reachability, nil); err != nil {
			t.Fatalf("Advertise(initial) error = %v", err)
		}
		supervisor.mu.Lock()
		prior := supervisor.sessions[TierPrivate]
		supervisor.mu.Unlock()
		trust.setError("provider-a", context.Canceled)
		_, err = supervisor.Establish(testutil.Context(t), activation, testProviderBound())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Establish(abandoned) error = %v, want context.Canceled", err)
		}
		supervisor.mu.Lock()
		current := supervisor.sessions[TierPrivate]
		supervisor.mu.Unlock()
		if current != prior || current.cancel == nil {
			t.Fatal("abandoned authorization replaced the prior provider state")
		}
	})

	t.Run("Should refuse provider permission failure without partial exposure [UT-062]", func(t *testing.T) {
		t.Parallel()
		source := healthySupervisorSource(nil)
		source.establishErr = errors.New("provider quota denied")
		supervisor := newSupervisorForTest(t, bundledTrustResolver(nil), bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}},
			&supervisorVerifier{timeout: time.Second})
		if _, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		); err == nil {
			t.Fatal("Establish() error = nil, want provider refusal")
		}
		supervisor.mu.Lock()
		partial := supervisor.sessions[TierPrivate]
		supervisor.mu.Unlock()
		if partial != nil {
			t.Fatalf("partial provider session = %#v, want nil", partial)
		}
	})

	t.Run("Should never advertise an unverified endpoint [UT-071]", func(t *testing.T) {
		t.Parallel()
		verifier := &supervisorVerifier{timeout: time.Second, err: ErrEndpointUnverified}
		resolver := &supervisorSourceResolver{sources: map[string]ConnectivitySource{
			"provider-a": healthySupervisorSource(nil),
		}}
		supervisor := newSupervisorForTest(
			t,
			bundledTrustResolver(nil),
			bundledIdentityStore(nil),
			resolver,
			verifier,
		)
		reachability, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		)
		if err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		if err := supervisor.Verify(testutil.Context(t), reachability); !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
		}
		if err := supervisor.Advertise(testutil.Context(t), reachability, nil); !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Advertise() error = %v, want ErrEndpointUnverified", err)
		}
		supervisor.mu.Lock()
		monitor := supervisor.sessions[TierPrivate].cancel
		supervisor.mu.Unlock()
		if monitor != nil {
			t.Fatal("unverified provider started an advertisement monitor")
		}
	})

	t.Run("Should keep an unverified provider session available for a later proof", func(t *testing.T) {
		t.Parallel()
		source := healthySupervisorSource(nil)
		verifier := &supervisorVerifier{
			timeout: time.Second,
			errors:  []error{ErrEndpointUnverified, nil},
		}
		supervisor := newSupervisorForTest(
			t,
			bundledTrustResolver(nil),
			bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}},
			verifier,
		)
		activation := supervisorActivation("provider-a")
		bound := testProviderBound()
		reachability, err := supervisor.Establish(testutil.Context(t), activation, bound)
		if err != nil {
			t.Fatalf("Establish(initial) error = %v", err)
		}
		if err := supervisor.Verify(testutil.Context(t), reachability); !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify(initial) error = %v, want ErrEndpointUnverified", err)
		}
		supervisor.mu.Lock()
		session := supervisor.sessions[TierPrivate]
		supervisor.mu.Unlock()
		if session == nil {
			t.Fatal("unverified provider session was removed")
		}
		if _, ok := supervisor.challenges.Resolve(TierPrivate, session.path); !ok {
			t.Fatal("verification challenge was removed before provider teardown")
		}

		retried, err := supervisor.Establish(testutil.Context(t), activation, bound)
		if err != nil {
			t.Fatalf("Establish(retry) error = %v", err)
		}
		if source.establishCount() != 1 || source.teardownCount() != 0 {
			t.Fatalf(
				"provider lifecycle = establishes %d, teardowns %d; want 1, 0",
				source.establishCount(),
				source.teardownCount(),
			)
		}
		if err := supervisor.Verify(testutil.Context(t), retried); err != nil {
			t.Fatalf("Verify(retry) error = %v", err)
		}
		if err := supervisor.Advertise(testutil.Context(t), retried, nil); err != nil {
			t.Fatalf("Advertise() error = %v", err)
		}
	})
}

func TestProviderSupervisorTrustFreshness(t *testing.T) {
	t.Parallel()

	t.Run("Should rederive live trust before every enable [UT-072]", func(t *testing.T) {
		t.Parallel()
		trust := thirdPartyTrustResolver("digest-a", "digest-a")
		identity := thirdPartyIdentityStore("digest-a")
		resolver := &supervisorSourceResolver{sources: map[string]ConnectivitySource{
			"provider-a": healthySupervisorSource(nil),
		}}
		supervisor := newSupervisorForTest(t, trust, identity, resolver, &supervisorVerifier{timeout: time.Second})
		activation := supervisorActivation("provider-a")
		for attempt := range 2 {
			if _, err := supervisor.Establish(testutil.Context(t), activation, testProviderBound()); err != nil {
				t.Fatalf("Establish(attempt %d) error = %v", attempt, err)
			}
		}
		if got := trust.callCount("provider-a"); got != 2 {
			t.Fatalf("live trust calls = %d, want 2", got)
		}
	})

	t.Run("Should fail closed when the live digest changed [UT-073]", func(t *testing.T) {
		t.Parallel()
		resolver := &supervisorSourceResolver{sources: map[string]ConnectivitySource{
			"provider-a": healthySupervisorSource(nil),
		}}
		supervisor := newSupervisorForTest(t, thirdPartyTrustResolver("digest-new", "digest-new"),
			thirdPartyIdentityStore("digest-old"), resolver, &supervisorVerifier{timeout: time.Second})
		if _, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		); !errors.Is(err, ErrProviderTrustStale) {
			t.Fatalf("Establish() error = %v, want ErrProviderTrustStale", err)
		}
		if got := resolver.callCount(); got != 0 {
			t.Fatalf("provider process resolutions = %d, want 0", got)
		}
	})

	t.Run("Should require confirmation for a third-party provider [UT-074]", func(t *testing.T) {
		t.Parallel()
		trust := thirdPartyTrustResolver("digest-a", "")
		supervisor := newSupervisorForTest(t, trust, thirdPartyIdentityStore("digest-a"),
			&supervisorSourceResolver{}, &supervisorVerifier{timeout: time.Second})
		if _, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		); !errors.Is(err, ErrDigestConfirmationRequired) {
			t.Fatalf("Establish() error = %v, want ErrDigestConfirmationRequired", err)
		}
	})

	t.Run("Should require reconsent after declared control expands [UT-075]", func(t *testing.T) {
		t.Parallel()
		trust := thirdPartyTrustResolver("digest-expanded", "")
		supervisor := newSupervisorForTest(t, trust, thirdPartyIdentityStore("digest-original"),
			&supervisorSourceResolver{}, &supervisorVerifier{timeout: time.Second})
		if _, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		); !errors.Is(err, ErrDigestConfirmationRequired) {
			t.Fatalf("Establish() error = %v, want ErrDigestConfirmationRequired", err)
		}
	})

	t.Run("Should refuse and audit a provider acting outside its assigned tier [UT-076]", func(t *testing.T) {
		t.Parallel()
		trust := bundledTrustResolver(nil)
		trust.trust["provider-a"] = ProviderTrust{
			InstallSource: ProviderInstallSourceBundled,
			ChannelScopes: []string{ProviderChannelScope(TierPublic)},
		}
		resolver := &supervisorSourceResolver{sources: map[string]ConnectivitySource{
			"provider-a": healthySupervisorSource(nil),
		}}
		audit := &supervisorAuditRecorder{}
		supervisor := newSupervisorForTest(
			t,
			trust,
			bundledIdentityStore(nil),
			resolver,
			&supervisorVerifier{timeout: time.Second},
			WithProviderAuditSink(audit),
		)
		activation := supervisorActivation("provider-a")
		_, err := supervisor.Establish(testutil.Context(t), activation, testProviderBound())
		if !errors.Is(err, ErrExposureRefused) {
			t.Fatalf("Establish() error = %v, want ErrExposureRefused", err)
		}
		if got := resolver.callCount(); got != 0 {
			t.Fatalf("provider process resolutions = %d, want 0", got)
		}
		recorded := audit.snapshot()
		if len(recorded) != 1 || recorded[0].ProviderName != activation.ProviderName ||
			recorded[0].Tier != activation.Tier {
			t.Fatalf("audit activations = %#v, want one provider refusal", recorded)
		}
	})
}

func TestProviderSupervisorFailureLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should join degradation handling before signaling monitor shutdown", func(t *testing.T) {
		t.Parallel()
		source := healthySupervisorSource(nil)
		source.statusErr = errors.New("provider crashed")
		supervisor := newSupervisorForTest(t, bundledTrustResolver(nil), bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}},
			&supervisorVerifier{timeout: time.Second},
			WithProviderSupervisorIntervals(5*time.Millisecond, 20*time.Millisecond, time.Second))
		handlerStarted := make(chan struct{})
		releaseHandler := make(chan struct{})
		supervisor.SetProviderDegradationHandler(func(context.Context, ProviderActivation, error) error {
			close(handlerStarted)
			<-releaseHandler
			return nil
		})
		reachability, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		)
		if err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		if err := supervisor.Verify(testutil.Context(t), reachability); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if err := supervisor.Advertise(testutil.Context(t), reachability, nil); err != nil {
			t.Fatalf("Advertise() error = %v", err)
		}
		supervisor.mu.Lock()
		session := supervisor.sessions[TierPrivate]
		supervisor.mu.Unlock()
		select {
		case <-handlerStarted:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for degradation handler")
		}
		select {
		case <-session.done:
			t.Fatal("monitor signaled shutdown before degradation handling completed")
		default:
		}
		close(releaseHandler)
		select {
		case <-session.done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for joined monitor shutdown")
		}
	})

	t.Run("Should degrade and withdraw after a supervised outage [UT-078]", func(t *testing.T) {
		t.Parallel()
		source := healthySupervisorSource(nil)
		source.statusErr = errors.New("provider crashed")
		supervisor := newSupervisorForTest(t, bundledTrustResolver(nil), bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}},
			&supervisorVerifier{timeout: time.Second},
			WithProviderSupervisorIntervals(5*time.Millisecond, 20*time.Millisecond, 20*time.Millisecond))
		degraded := make(chan error, 1)
		supervisor.SetProviderDegradationHandler(func(_ context.Context, _ ProviderActivation, err error) error {
			degraded <- err
			return nil
		})
		reachability, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		)
		if err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		if err := supervisor.Verify(testutil.Context(t), reachability); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if err := supervisor.Advertise(testutil.Context(t), reachability, nil); err != nil {
			t.Fatalf("Advertise() error = %v", err)
		}
		select {
		case err := <-degraded:
			if !errors.Is(err, ErrProviderDegraded) {
				t.Fatalf("degradation error = %v, want ErrProviderDegraded", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for provider degradation")
		}
		supervisor.mu.Lock()
		active := supervisor.sessions[TierPrivate]
		supervisor.mu.Unlock()
		if active != nil || source.teardownCount() == 0 {
			t.Fatalf("outage state = session %#v, teardowns %d", active, source.teardownCount())
		}
	})

	t.Run("Should bound a hanging teardown [UT-079]", func(t *testing.T) {
		t.Parallel()
		source := healthySupervisorSource(nil)
		source.teardown = func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}
		supervisor := newSupervisorForTest(t, bundledTrustResolver(nil), bundledIdentityStore(nil),
			&supervisorSourceResolver{sources: map[string]ConnectivitySource{"provider-a": source}},
			&supervisorVerifier{timeout: time.Second},
			WithProviderSupervisorIntervals(time.Hour, 20*time.Millisecond, 20*time.Millisecond))
		activation := supervisorActivation("provider-a")
		if _, err := supervisor.Establish(testutil.Context(t), activation, testProviderBound()); err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		started := time.Now()
		err := supervisor.Teardown(testutil.Context(t), activation)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Teardown() error = %v, want deadline exceeded", err)
		}
		if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
			t.Fatalf("Teardown() elapsed = %s, want bounded", elapsed)
		}
	})

	t.Run("Should require explicit provider selection for one tier [UT-080]", func(t *testing.T) {
		t.Parallel()
		trust := bundledTrustResolver(nil)
		identity := bundledIdentityStore(nil)
		resolver := &supervisorSourceResolver{sources: map[string]ConnectivitySource{
			"provider-a": healthySupervisorSource(nil),
			"provider-b": healthySupervisorSource(nil),
		}}
		identity.identities["provider-b"] = ProviderIdentity{
			Name:          "provider-b",
			InstallSource: ProviderInstallSourceBundled,
		}
		trust.trust["provider-b"] = ProviderTrust{
			InstallSource: ProviderInstallSourceBundled,
			ChannelScopes: []string{ProviderChannelScope(TierPrivate), ProviderChannelScope(TierPublic)},
		}
		supervisor := newSupervisorForTest(t, trust, identity, resolver, &supervisorVerifier{timeout: time.Second})
		if _, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-a"), testProviderBound(),
		); err != nil {
			t.Fatalf("Establish(provider-a) error = %v", err)
		}
		if _, err := supervisor.Establish(
			testutil.Context(t), supervisorActivation("provider-b"), testProviderBound(),
		); !errors.Is(err, ErrTierProviderConflict) {
			t.Fatalf("Establish(provider-b) error = %v, want ErrTierProviderConflict", err)
		}
		supervisor.mu.Lock()
		selected := supervisor.sessions[TierPrivate]
		supervisor.mu.Unlock()
		if selected == nil || selected.activation.ProviderName != "provider-a" {
			t.Fatalf("selected provider = %#v, want provider-a", selected)
		}
	})
}

type supervisorEventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *supervisorEventLog) add(event string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
}

func (l *supervisorEventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

type supervisorTrustResolver struct {
	mu     sync.Mutex
	trust  map[string]ProviderTrust
	errors map[string]error
	calls  map[string]int
	events *supervisorEventLog
}

func (r *supervisorTrustResolver) ResolveProviderTrust(_ context.Context, name string) (ProviderTrust, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events.add("trust")
	r.calls[name]++
	return r.trust[name], r.errors[name]
}

func (r *supervisorTrustResolver) setError(name string, err error) {
	r.mu.Lock()
	r.errors[name] = err
	r.mu.Unlock()
}

func (r *supervisorTrustResolver) callCount(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[name]
}

type supervisorIdentityStore struct {
	identities map[string]ProviderIdentity
	events     *supervisorEventLog
}

func (s *supervisorIdentityStore) ProviderIdentity(_ context.Context, name string) (ProviderIdentity, error) {
	s.events.add("identity")
	identity, ok := s.identities[name]
	if !ok {
		return ProviderIdentity{}, ErrProviderNotFound
	}
	return identity, nil
}

type supervisorSourceResolver struct {
	mu      sync.Mutex
	sources map[string]ConnectivitySource
	calls   int
	events  *supervisorEventLog
}

func (r *supervisorSourceResolver) ResolveConnectivitySource(
	_ context.Context,
	name string,
) (ConnectivitySource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events.add("resolve")
	r.calls++
	source := r.sources[name]
	if source == nil {
		return nil, ErrProviderNotFound
	}
	return source, nil
}

func (r *supervisorSourceResolver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type supervisorSource struct {
	mu           sync.Mutex
	reachability Reachability
	establishErr error
	statusErr    error
	teardown     func(context.Context) error
	teardowns    int
	establishes  int
	lastRequest  EstablishRequest
	events       *supervisorEventLog
}

func (s *supervisorSource) Establish(_ context.Context, request EstablishRequest) (Reachability, error) {
	s.mu.Lock()
	s.establishes++
	s.lastRequest = request
	s.mu.Unlock()
	s.events.add("establish")
	return s.reachability, s.establishErr
}

func (s *supervisorSource) lastEstablishRequest() EstablishRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRequest
}

func (s *supervisorSource) establishCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.establishes
}

func (s *supervisorSource) Status(_ context.Context, _ Tier) (Reachability, error) {
	return s.reachability, s.statusErr
}

func (s *supervisorSource) Teardown(ctx context.Context, _ Tier, _ time.Time) error {
	s.mu.Lock()
	s.teardowns++
	teardown := s.teardown
	s.mu.Unlock()
	if teardown != nil {
		return teardown(ctx)
	}
	return nil
}

func (s *supervisorSource) teardownCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.teardowns
}

type supervisorVerifier struct {
	mu      sync.Mutex
	timeout time.Duration
	err     error
	errors  []error
	events  *supervisorEventLog
	calls   int
}

func (v *supervisorVerifier) Verify(context.Context, Tier, AdvertisedEndpoint, string, string) error {
	v.mu.Lock()
	v.calls++
	call := v.calls
	err := v.err
	if call <= len(v.errors) {
		err = v.errors[call-1]
	}
	v.mu.Unlock()
	v.events.add("verify")
	return err
}

func (v *supervisorVerifier) Timeout() time.Duration { return v.timeout }

func (v *supervisorVerifier) callCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.calls
}

type supervisorAuditRecorder struct {
	mu          sync.Mutex
	activations []ProviderActivation
	verified    []AdvertisedEndpoint
}

func (r *supervisorAuditRecorder) RecordEndpointVerified(
	_ context.Context,
	_ ProviderActivation,
	endpoint AdvertisedEndpoint,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verified = append(r.verified, endpoint)
	return nil
}

func (r *supervisorAuditRecorder) RecordProviderStateChanged(
	context.Context,
	ProviderActivation,
	ProviderObservedState,
	string,
) error {
	return nil
}

func (r *supervisorAuditRecorder) RecordProviderRefusal(
	_ context.Context,
	activation ProviderActivation,
	_ ProviderRefusalKind,
	_ error,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activations = append(r.activations, activation)
	return nil
}

func (r *supervisorAuditRecorder) snapshot() []ProviderActivation {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.activations)
}

func (r *supervisorAuditRecorder) verifiedSnapshot() []AdvertisedEndpoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.verified)
}

func newSupervisorForTest(
	t *testing.T,
	trust ProviderTrustResolver,
	identities ProviderIdentityStore,
	resolver ConnectivitySourceResolver,
	verifier EndpointVerification,
	options ...ProviderSupervisorOption,
) *ProviderSupervisor {
	t.Helper()
	options = append([]ProviderSupervisorOption{
		WithProviderSupervisorIntervals(time.Hour, 50*time.Millisecond, 50*time.Millisecond),
	}, options...)
	supervisor, err := NewProviderSupervisor(
		resolver, trust, identities, NewChallengeRegistry(), verifier, options...,
	)
	if err != nil {
		t.Fatalf("NewProviderSupervisor() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		for _, tier := range []Tier{TierPrivate, TierPublic} {
			supervisor.mu.Lock()
			session := supervisor.sessions[tier]
			supervisor.mu.Unlock()
			if session != nil {
				if err := supervisor.Teardown(
					ctx,
					session.activation,
				); err != nil &&
					!errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("Teardown(cleanup) error = %v", err)
				}
			}
		}
	})
	return supervisor
}

func healthySupervisorSource(events *supervisorEventLog) *supervisorSource {
	return &supervisorSource{events: events, reachability: Reachability{
		Tier: TierPrivate,
		Endpoints: []AdvertisedEndpoint{{
			URL: "https://gateway.example.test", Scheme: "https", Stability: EndpointStable,
		}},
		Health: HealthHealthy,
	}}
}

func bundledTrustResolver(events *supervisorEventLog) *supervisorTrustResolver {
	return &supervisorTrustResolver{
		trust: map[string]ProviderTrust{"provider-a": {
			InstallSource: ProviderInstallSourceBundled,
			ChannelScopes: []string{ProviderChannelScope(TierPrivate), ProviderChannelScope(TierPublic)},
		}},
		errors: make(map[string]error), calls: make(map[string]int), events: events,
	}
}

func thirdPartyTrustResolver(controlDigest string, confirmedDigest string) *supervisorTrustResolver {
	return &supervisorTrustResolver{
		trust: map[string]ProviderTrust{"provider-a": {
			InstallSource: "marketplace", ControlDigest: controlDigest, ConfirmedDigest: confirmedDigest,
			ChannelScopes: []string{ProviderChannelScope(TierPrivate), ProviderChannelScope(TierPublic)},
		}},
		errors: make(map[string]error), calls: make(map[string]int),
	}
}

func bundledIdentityStore(events *supervisorEventLog) *supervisorIdentityStore {
	return &supervisorIdentityStore{identities: map[string]ProviderIdentity{
		"provider-a": {Name: "provider-a", InstallSource: ProviderInstallSourceBundled},
	}, events: events}
}

func thirdPartyIdentityStore(digest string) *supervisorIdentityStore {
	return &supervisorIdentityStore{identities: map[string]ProviderIdentity{
		"provider-a": {Name: "provider-a", InstallSource: "marketplace", DigestConfirmed: digest},
	}}
}

func supervisorActivation(name string) ProviderActivation {
	return ProviderActivation{
		ProviderName: name, Tier: TierPrivate, Desired: DesiredEnabled, Generation: 1,
	}
}

func testProviderBound() netip.AddrPort {
	return netip.MustParseAddrPort("127.0.0.1:43210")
}
