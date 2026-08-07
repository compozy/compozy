package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestIngressProjectionAndBindings(t *testing.T) {
	t.Parallel()

	t.Run("Should UT-090 project webhook URLs for existing subjects", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-existing"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeWorkspace,
			WorkspaceID:       "workspace-a",
			Path:              "/api/webhooks/workspaces/workspace-a/deploy--wbh_existing",
		}

		projection, err := manager.project(testContext(t), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if got, want := projection.URL,
			"https://gateway.example.test/api/webhooks/workspaces/workspace-a/deploy--wbh_existing"; got != want {
			t.Fatalf("projection.URL = %q, want %q", got, want)
		}
		if projection.Reachability != IngressReachabilityUnconfirmed || projection.EndpointGeneration != 1 {
			t.Fatalf("projection = %#v, want unconfirmed generation 1", projection)
		}
	})

	t.Run("Should UT-091 report an honest off state and explicit empty bindings", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, false, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-off"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeGlobal,
			Path:              "/api/webhooks/global/deploy--wbh_off",
		}

		projection, err := manager.project(testContext(t), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if projection.URL != "" || projection.Reachability != IngressReachabilityOff ||
			projection.EnablePath != ingressEnablePath {
			t.Fatalf("projection = %#v, want off with enable path and no URL", projection)
		}
		bindings, err := store.ListIngressBindings(testContext(t))
		if err != nil {
			t.Fatalf("ListIngressBindings() error = %v", err)
		}
		if bindings == nil || len(bindings) != 0 {
			t.Fatalf("bindings = %#v, want explicit empty list", bindings)
		}
	})

	t.Run("Should UT-092 bind one bridge instance to the verified generation", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectBridgeInstance, ID: "bridge-a"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeWorkspace,
			WorkspaceID:       "workspace-a",
			Path:              "/api/bridge-callbacks/bridge-a",
		}

		binding, err := manager.bind(testContext(t), IngressBindRequest{Subject: ref, Confirmed: true})
		if err != nil {
			t.Fatalf("bind() error = %v", err)
		}
		if binding.Subject != ref || binding.EndpointGeneration != 1 || !binding.Changed {
			t.Fatalf("binding = %#v, want bridge-a at generation 1", binding)
		}
		stored, err := store.GetIngressBinding(testContext(t), ref)
		if err != nil {
			t.Fatalf("GetIngressBinding() error = %v", err)
		}
		if stored.Subject != ref || stored.EndpointGeneration != 1 {
			t.Fatalf("stored binding = %#v", stored)
		}
	})

	t.Run("Should UT-093 mark a bound bridge off without changing its subject", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, false, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectBridgeInstance, ID: "bridge-external"}
		subject := IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeWorkspace,
			WorkspaceID:       "workspace-a",
			Path:              "/api/bridge-callbacks/bridge-external",
		}
		store.subjects[ref] = subject
		store.bindings[ref] = IngressBinding{
			Subject: ref, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID, EndpointGeneration: 1,
		}

		projection, err := manager.project(testContext(t), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if projection.Reachability != IngressReachabilityOff || projection.URL != "" {
			t.Fatalf("projection = %#v, want off without a dead URL", projection)
		}
		if projection.ConfirmedEndpointGeneration != 1 {
			t.Fatalf("projection confirmed generation = %d, want 1 while off", projection.ConfirmedEndpointGeneration)
		}
		if got := store.subjects[ref]; got != subject {
			t.Fatalf("bridge subject changed = %#v, want %#v", got, subject)
		}
	})

	t.Run("Should surface an invalid local bridge target as broken and refuse confirmation", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectBridgeInstance, ID: "bridge-invalid-target"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeGlobal,
			Path:              "/api/bridge-callbacks/bridge-invalid-target",
			TargetUnavailable: true,
		}

		projection, err := manager.project(testContext(t), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if projection.Reachability != IngressReachabilityBroken || projection.URL != "" {
			t.Fatalf("projection = %#v, want broken without a callback URL", projection)
		}
		if _, err := manager.bind(testContext(t), IngressBindRequest{
			Subject: ref, Confirmed: true,
		}); !errors.Is(err, ErrIngressTargetUnavailable) {
			t.Fatalf("bind() error = %v, want ErrIngressTargetUnavailable", err)
		}
	})

	t.Run("Should deny a scoped projection from another workspace", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-foreign-read"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeWorkspace,
			WorkspaceID:       "workspace-b",
			Path:              "/api/webhooks/workspaces/workspace-b/foreign--wbh_read",
		}
		_, err := manager.project(WithIngressReadWorkspace(testContext(t), "workspace-a"), ref)
		if !errors.Is(err, ErrIngressForbidden) {
			t.Fatalf("project(foreign workspace) error = %v, want ErrIngressForbidden", err)
		}
	})

	t.Run("Should compensate binding mutations when the durable event write fails", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		manager.events = &ingressEventSinkStub{err: errors.New("injected event failure")}
		ref := IngressSubjectRef{Kind: IngressSubjectBridgeInstance, ID: "bridge-event-failure"}
		subject := IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeWorkspace,
			WorkspaceID:       "workspace-a",
			Path:              "/api/bridge-callbacks/bridge-event-failure",
		}
		store.subjects[ref] = subject
		if _, err := manager.bind(testContext(t), IngressBindRequest{Subject: ref, Confirmed: true}); err == nil {
			t.Fatal("bind(event failure) error = nil")
		}
		if _, exists := store.bindings[ref]; exists {
			t.Fatalf("binding remained after failed event: %#v", store.bindings[ref])
		}

		confirmedAt := time.Date(2026, 8, 6, 12, 45, 0, 0, time.UTC)
		store.bindings[ref] = IngressBinding{
			Subject: ref, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID,
			EndpointGeneration: 1, ConfirmedAt: confirmedAt,
		}
		if _, err := manager.unbind(testContext(t), IngressUnbindRequest{Subject: ref}); err == nil {
			t.Fatal("unbind(event failure) error = nil")
		}
		restored, exists := store.bindings[ref]
		if !exists || restored.EndpointGeneration != 1 || !restored.ConfirmedAt.Equal(confirmedAt) {
			t.Fatalf("restored binding = %#v, want original confirmation", restored)
		}
	})

	t.Run("Should attribute a remote binding event to the authenticated device", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		sink := &ingressEventSinkStub{}
		manager.events = sink
		ref := IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-device-actor"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeGlobal,
			Path:              "/api/webhooks/global/device--wbh_actor",
		}
		ctx := ContextWithDevice(testContext(t), DeviceSession{
			ID: "device-actor", ActorKind: ActorKindOperatorDevice,
		})
		if _, err := manager.bind(ctx, IngressBindRequest{Subject: ref, Confirmed: true}); err != nil {
			t.Fatalf("bind() error = %v", err)
		}
		if len(sink.bound) != 1 || sink.bound[0].ActorKind != string(ActorKindOperatorDevice) ||
			sink.bound[0].ActorID != "device-actor" {
			t.Fatalf("bound events = %#v, want authenticated device attribution", sink.bound)
		}
	})

	t.Run("Should UT-094 require trigger and bridge reconfirmation after the public address changes [IT-048]", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		refs := []IngressSubjectRef{
			{Kind: IngressSubjectWebhookTrigger, ID: "trigger-generation"},
			{Kind: IngressSubjectBridgeInstance, ID: "bridge-generation"},
		}
		paths := []string{
			"/api/webhooks/global/deploy--wbh_generation",
			"/api/bridge-callbacks/bridge-generation",
		}
		for index, ref := range refs {
			store.subjects[ref] = IngressSubject{
				IngressSubjectRef: ref,
				Scope:             IngressScopeGlobal,
				Path:              paths[index],
			}
			if _, err := manager.bind(testContext(t), IngressBindRequest{
				Subject: ref, Confirmed: true,
			}); err != nil {
				t.Fatalf("bind(%s) error = %v", ref.Kind, err)
			}
		}
		if _, changed, err := store.SyncIngressEndpoint(
			testContext(t), TierPublic, "https://gateway-two.example.test", time.Now().UTC(),
		); err != nil || !changed {
			t.Fatalf("SyncIngressEndpoint() = changed:%t error:%v, want changed", changed, err)
		}

		for _, ref := range refs {
			projection, err := manager.project(testContext(t), ref)
			if err != nil {
				t.Fatalf("project(%s) error = %v", ref.Kind, err)
			}
			if projection.Reachability != IngressReachabilityReconfirmation ||
				projection.EndpointGeneration != 2 || projection.ConfirmedEndpointGeneration != 1 ||
				!strings.HasPrefix(projection.URL, "https://gateway-two.example.test/") {
				t.Fatalf("projection(%s) = %#v, want generation 2 requiring reconfirmation", ref.Kind, projection)
			}
		}
	})

	t.Run("Should UT-095 persist only the server-resolved owner", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-owner"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeWorkspace,
			WorkspaceID:       "workspace-owner",
			Path:              "/api/webhooks/workspaces/workspace-owner/deploy--wbh_owner",
		}

		if _, err := manager.bind(testContext(t), IngressBindRequest{Subject: ref, Confirmed: true}); err != nil {
			t.Fatalf("bind() error = %v", err)
		}
		if store.lastPut.Scope != IngressScopeWorkspace || store.lastPut.WorkspaceID != "workspace-owner" {
			t.Fatalf("PutIngressBinding() subject = %#v, want server-resolved owner", store.lastPut)
		}
	})

	t.Run("Should UT-096 deny a caller from another workspace", func(t *testing.T) {
		t.Parallel()
		access := &ingressAccessPolicy{}
		store, manager := newIngressTestManager(t, true, access)
		ref := IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-foreign"}
		store.subjects[ref] = IngressSubject{
			IngressSubjectRef: ref,
			Scope:             IngressScopeWorkspace,
			WorkspaceID:       "workspace-owner",
			Path:              "/api/webhooks/workspaces/workspace-owner/deploy--wbh_foreign",
		}
		caller := &IngressCaller{SessionID: "session-agent", WorkspaceID: "workspace-other", AgentName: "coder"}

		_, err := manager.bind(testContext(t), IngressBindRequest{
			Subject: ref, Confirmed: true, Caller: caller,
		})
		if !errors.Is(err, ErrIngressForbidden) {
			t.Fatalf("bind(cross-workspace) error = %v, want ErrIngressForbidden", err)
		}
		if access.calls != 1 || len(store.bindings) != 0 {
			t.Fatalf("authorization calls = %d bindings = %#v, want one denial and no mutation", access.calls, store.bindings)
		}
	})

	t.Run("Should UT-097 sweep an orphan and treat it as absent", func(t *testing.T) {
		t.Parallel()
		store, _ := newIngressTestManager(t, true, nil)
		ref := IngressSubjectRef{Kind: IngressSubjectBridgeInstance, ID: "bridge-orphan"}
		store.bindings[ref] = IngressBinding{Subject: ref, EndpointGeneration: 1}

		if _, err := store.GetIngressBinding(testContext(t), ref); !errors.Is(err, ErrIngressSubjectNotFound) {
			t.Fatalf("GetIngressBinding(orphan) error = %v, want ErrIngressSubjectNotFound", err)
		}
		removed, err := store.SweepOrphanedIngressBindings(testContext(t))
		if err != nil {
			t.Fatalf("SweepOrphanedIngressBindings() error = %v", err)
		}
		if removed != 1 || len(store.bindings) != 0 {
			t.Fatalf("sweep removed = %d bindings = %#v, want one removal", removed, store.bindings)
		}
	})

	t.Run("Should filter binding reads to the validated agent workspace", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		for _, subject := range []IngressSubject{
			{
				IngressSubjectRef: IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-global"},
				Scope:             IngressScopeGlobal,
				Path:              "/api/webhooks/global/global--wbh_global",
			},
			{
				IngressSubjectRef: IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-a"},
				Scope:             IngressScopeWorkspace,
				WorkspaceID:       "workspace-a",
				Path:              "/api/webhooks/workspaces/workspace-a/a--wbh_a",
			},
			{
				IngressSubjectRef: IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-b"},
				Scope:             IngressScopeWorkspace,
				WorkspaceID:       "workspace-b",
				Path:              "/api/webhooks/workspaces/workspace-b/b--wbh_b",
			},
		} {
			store.subjects[subject.IngressSubjectRef] = subject
			store.bindings[subject.IngressSubjectRef] = IngressBinding{
				Subject: subject.IngressSubjectRef, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID,
				EndpointGeneration: 1,
			}
		}
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service, err := NewService(manager.policy, newDeviceServiceOnly(t, &now), func(service *Service) error {
			service.ingress = manager
			return nil
		})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		operatorStatus, err := service.Status(testContext(t))
		if err != nil {
			t.Fatalf("Status(operator) error = %v", err)
		}
		if got := len(operatorStatus.Bindings); got != 3 {
			t.Fatalf("operator binding count = %d, want 3", got)
		}
		agentStatus, err := service.Status(WithIngressReadWorkspace(testContext(t), "workspace-a"))
		if err != nil {
			t.Fatalf("Status(agent) error = %v", err)
		}
		if got := len(agentStatus.Bindings); got != 2 {
			t.Fatalf("agent binding count = %d, want global plus workspace-a", got)
		}
		for _, binding := range agentStatus.Bindings {
			if binding.Scope == IngressScopeWorkspace && binding.WorkspaceID != "workspace-a" {
				t.Fatalf("agent status leaked foreign binding %#v", binding)
			}
		}
	})
}

func TestIngressRateLimiterHighCardinalityCannotResetAnActiveBudget(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 15, 0, 0, 0, time.UTC)
	limiter := NewIngressRateLimiter(1, time.Minute, func() time.Time { return now })
	victimEndpoint := "/api/webhooks/global/valid--wbh_live"
	victimSource := "203.0.113.11"
	if allowed, _ := limiter.Allow(victimEndpoint, victimSource); !allowed {
		t.Fatal("victim's first request was denied")
	}
	if allowed, _ := limiter.Allow(victimEndpoint, victimSource); allowed {
		t.Fatal("victim's second request was admitted before churn")
	}
	for index := range maxTrackedIngressBudgets + 1 {
		endpoint := fmt.Sprintf("/api/webhooks/global/probe-%d", index)
		allowed, retryAfter := limiter.Allow(endpoint, "203.0.113.10")
		if !allowed && retryAfter <= 0 {
			t.Fatalf("probe %d denial has no retry guidance", index)
		}
	}
	if allowed, retryAfter := limiter.Allow(victimEndpoint, victimSource); allowed || retryAfter <= 0 {
		t.Fatalf("victim budget after churn = allowed:%t retry:%s, want denied with guidance", allowed, retryAfter)
	}
}

func TestIngressEndpointFailsClosedAcrossRestart(t *testing.T) {
	t.Parallel()

	store, _ := newIngressTestManager(t, true, nil)
	if !store.endpoint.Reachable || store.endpoint.Generation != 1 {
		t.Fatalf("initial endpoint = %#v, want reachable generation 1", store.endpoint)
	}
	reconciler := NewReconciler(store, newTestEffects(nil))
	err := reconciler.Reconcile(testContext(t), ExposurePlan{Tiers: []TierPlan{{
		Tier: TierPublic, Enabled: true,
	}}})
	if err == nil {
		t.Fatal("Reconcile(missing provider) error = nil")
	}
	if store.endpoint.Reachable {
		t.Fatalf("endpoint remained reachable after failed restart reconcile: %#v", store.endpoint)
	}
	restored, changed, err := store.SyncIngressEndpoint(
		testContext(t),
		TierPublic,
		"https://gateway.example.test",
		time.Date(2026, 8, 6, 15, 1, 0, 0, time.UTC),
	)
	if err != nil || changed || restored.Generation != 1 || !restored.Reachable {
		t.Fatalf("stable endpoint restore = (%#v, %t, %v), want reachable generation 1 unchanged [IT-051]", restored, changed, err)
	}
}

type ingressTestStore struct {
	*testStore
	mu       sync.Mutex
	subjects map[IngressSubjectRef]IngressSubject
	bindings map[IngressSubjectRef]IngressBinding
	endpoint IngressEndpointState
	lastPut  IngressSubject
}

func newIngressTestManager(
	t *testing.T,
	publicOn bool,
	access workspaceaccess.Policy,
) (*ingressTestStore, *ingressManager) {
	t.Helper()
	surface := SurfaceExposure{
		Surface:  SurfaceWebhookIngress,
		Tier:     TierPublic,
		Desired:  DesiredDisabled,
		Observed: SurfaceOff,
	}
	if publicOn {
		surface.Desired = DesiredEnabled
		surface.Observed = SurfaceOn
	}
	store := &ingressTestStore{
		testStore: &testStore{snapshot: Snapshot{Surfaces: []SurfaceExposure{surface}}},
		subjects:  make(map[IngressSubjectRef]IngressSubject),
		bindings:  make(map[IngressSubjectRef]IngressBinding),
		endpoint: IngressEndpointState{
			Tier: TierPublic, URL: "https://gateway.example.test", Generation: 1,
			Reachable: true, UpdatedAt: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		},
	}
	policy := newTestPolicy(t, store, newTestEffects(nil), true)
	manager, err := newIngressManager(
		store,
		policy,
		access,
		nil,
		func() time.Time { return time.Date(2026, 8, 6, 12, 30, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatalf("newIngressManager() error = %v", err)
	}
	return store, manager
}

func (s *ingressTestStore) ResolveIngressSubject(
	_ context.Context,
	ref IngressSubjectRef,
) (IngressSubject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subject, ok := s.subjects[ref]
	if !ok {
		return IngressSubject{}, ErrIngressSubjectNotFound
	}
	return subject, nil
}

func (s *ingressTestStore) GetIngressEndpoint(context.Context, Tier) (IngressEndpointState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.endpoint.Reachable || strings.TrimSpace(s.endpoint.URL) == "" {
		return IngressEndpointState{}, ErrEndpointUnverified
	}
	return s.endpoint, nil
}

func (s *ingressTestStore) SyncIngressEndpoint(
	_ context.Context,
	tier Tier,
	endpointURL string,
	now time.Time,
) (IngressEndpointState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.endpoint.URL != endpointURL
	if s.endpoint.Generation == 0 {
		s.endpoint.Generation = 1
	} else if changed {
		s.endpoint.Generation++
	}
	s.endpoint.Tier = tier
	s.endpoint.URL = endpointURL
	s.endpoint.Reachable = true
	s.endpoint.UpdatedAt = now
	return s.endpoint, changed, nil
}

func (s *ingressTestStore) MarkIngressEndpointUnavailable(context.Context, Tier, time.Time) error {
	s.mu.Lock()
	s.endpoint.Reachable = false
	s.mu.Unlock()
	return nil
}

func (s *ingressTestStore) GetIngressBinding(
	_ context.Context,
	ref IngressSubjectRef,
) (IngressBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subjects[ref]; !ok {
		return IngressBinding{}, ErrIngressSubjectNotFound
	}
	binding, ok := s.bindings[ref]
	if !ok {
		return IngressBinding{}, ErrIngressSubjectNotFound
	}
	return binding, nil
}

func (s *ingressTestStore) PutIngressBinding(
	_ context.Context,
	subject IngressSubject,
	generation uint64,
	confirmedAt time.Time,
) (IngressBinding, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPut = subject
	current, exists := s.bindings[subject.IngressSubjectRef]
	changed := !exists || current.EndpointGeneration != generation || current.Scope != subject.Scope ||
		current.WorkspaceID != subject.WorkspaceID
	binding := IngressBinding{
		Subject: subject.IngressSubjectRef, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID,
		EndpointGeneration: generation, ConfirmedAt: confirmedAt,
	}
	s.bindings[subject.IngressSubjectRef] = binding
	return binding, changed, nil
}

func (s *ingressTestStore) DeleteIngressBinding(_ context.Context, subject IngressSubject) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.bindings[subject.IngressSubjectRef]
	delete(s.bindings, subject.IngressSubjectRef)
	return exists, nil
}

func (s *ingressTestStore) ListIngressBindings(context.Context) ([]IngressBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bindings := make([]IngressBinding, 0, len(s.bindings))
	for ref, binding := range s.bindings {
		if _, ok := s.subjects[ref]; ok {
			bindings = append(bindings, binding)
		}
	}
	return bindings, nil
}

func (s *ingressTestStore) SweepOrphanedIngressBindings(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed int64
	for ref := range s.bindings {
		if _, ok := s.subjects[ref]; ok {
			continue
		}
		delete(s.bindings, ref)
		removed++
	}
	return removed, nil
}

type ingressAccessPolicy struct {
	calls int
}

type ingressEventSinkStub struct {
	err     error
	bound   []IngressMutationEvent
	unbound []IngressMutationEvent
}

func (s *ingressEventSinkStub) RecordIngressBound(_ context.Context, event IngressMutationEvent) error {
	s.bound = append(s.bound, event)
	return s.err
}

func (s *ingressEventSinkStub) RecordIngressUnbound(_ context.Context, event IngressMutationEvent) error {
	s.unbound = append(s.unbound, event)
	return s.err
}

func (p *ingressAccessPolicy) Authorize(
	context.Context,
	workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	p.calls++
	return workspaceaccess.Decision{Allowed: false, Source: workspaceaccess.SourceDenied}, nil
}
