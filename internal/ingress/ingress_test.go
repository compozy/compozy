package ingress

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
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-existing"}
		store.subjects[ref] = Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-a",
			Path:        "/api/webhooks/workspaces/workspace-a/deploy--wbh_existing",
		}

		projection, err := manager.Project(t.Context(), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if got, want := projection.URL,
			"https://gateway.example.test/api/webhooks/workspaces/workspace-a/deploy--wbh_existing"; got != want {
			t.Fatalf("projection.URL = %q, want %q", got, want)
		}
		if projection.Reachability != ReachabilityUnconfirmed || projection.EndpointGeneration != 1 {
			t.Fatalf("projection = %#v, want unconfirmed generation 1", projection)
		}
	})

	t.Run("Should UT-091 report an honest off state and explicit empty bindings", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, false, nil)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-off"}
		store.subjects[ref] = Subject{
			SubjectRef: ref,
			Scope:      ScopeGlobal,
			Path:       "/api/webhooks/global/deploy--wbh_off",
		}

		projection, err := manager.Project(t.Context(), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if projection.URL != "" || projection.Reachability != ReachabilityOff ||
			projection.EnablePath != EnablePath {
			t.Fatalf("projection = %#v, want off with enable path and no URL", projection)
		}
		bindings, err := store.ListIngressBindings(t.Context())
		if err != nil {
			t.Fatalf("ListIngressBindings() error = %v", err)
		}
		if bindings == nil || len(bindings) != 0 {
			t.Fatalf("bindings = %#v, want explicit empty list", bindings)
		}
	})

	t.Run("Should UT-092 bind one webhook trigger to the verified generation", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-a"}
		store.subjects[ref] = Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-a",
			Path:        "/api/webhooks/workspaces/workspace-a/deploy--wbh_a",
		}

		binding, err := manager.Bind(t.Context(), BindRequest{Subject: ref, Confirmed: true})
		if err != nil {
			t.Fatalf("bind() error = %v", err)
		}
		if binding.Subject != ref || binding.EndpointGeneration != 1 || !binding.Changed {
			t.Fatalf("binding = %#v, want trigger-a at generation 1", binding)
		}
		stored, err := store.GetIngressBinding(t.Context(), ref)
		if err != nil {
			t.Fatalf("GetIngressBinding() error = %v", err)
		}
		if stored.Subject != ref || stored.EndpointGeneration != 1 {
			t.Fatalf("stored binding = %#v", stored)
		}
	})

	t.Run("Should UT-093 mark a bound webhook off without changing its subject", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, false, nil)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-external"}
		subject := Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-a",
			Path:        "/api/webhooks/workspaces/workspace-a/external--wbh_external",
		}
		store.subjects[ref] = subject
		store.bindings[ref] = Binding{
			Subject: ref, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID, EndpointGeneration: 1,
		}

		projection, err := manager.Project(t.Context(), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if projection.Reachability != ReachabilityOff || projection.URL != "" {
			t.Fatalf("projection = %#v, want off without a dead URL", projection)
		}
		if projection.ConfirmedEndpointGeneration != 1 {
			t.Fatalf("projection confirmed generation = %d, want 1 while off", projection.ConfirmedEndpointGeneration)
		}
		if got := store.subjects[ref]; got != subject {
			t.Fatalf("webhook subject changed = %#v, want %#v", got, subject)
		}
	})

	t.Run("Should surface an invalid local webhook target as broken and refuse confirmation", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-invalid-target"}
		store.subjects[ref] = Subject{
			SubjectRef:        ref,
			Scope:             ScopeGlobal,
			Path:              "/api/webhooks/global/invalid--wbh_invalid",
			TargetUnavailable: true,
		}

		projection, err := manager.Project(t.Context(), ref)
		if err != nil {
			t.Fatalf("project() error = %v", err)
		}
		if projection.Reachability != ReachabilityBroken || projection.URL != "" {
			t.Fatalf("projection = %#v, want broken without a callback URL", projection)
		}
		if _, err := manager.Bind(t.Context(), BindRequest{
			Subject: ref, Confirmed: true,
		}); !errors.Is(err, ErrTargetUnavailable) {
			t.Fatalf("bind() error = %v, want ErrTargetUnavailable", err)
		}
	})

	t.Run("Should deny a scoped projection from another workspace", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-foreign-read"}
		store.subjects[ref] = Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-b",
			Path:        "/api/webhooks/workspaces/workspace-b/foreign--wbh_read",
		}
		_, err := manager.Project(WithReadWorkspace(t.Context(), "workspace-a"), ref)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("project(foreign workspace) error = %v, want ErrForbidden", err)
		}
	})

	t.Run("Should compensate binding mutations when the durable event write fails", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		manager.events = &ingressEventSinkStub{err: errors.New("injected event failure")}
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-event-failure"}
		subject := Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-a",
			Path:        "/api/webhooks/workspaces/workspace-a/event-failure--wbh_failure",
		}
		store.subjects[ref] = subject
		if _, err := manager.Bind(t.Context(), BindRequest{Subject: ref, Confirmed: true}); err == nil {
			t.Fatal("bind(event failure) error = nil")
		}
		if _, exists := store.bindings[ref]; exists {
			t.Fatalf("binding remained after failed event: %#v", store.bindings[ref])
		}

		confirmedAt := time.Date(2026, 8, 6, 12, 45, 0, 0, time.UTC)
		store.bindings[ref] = Binding{
			Subject: ref, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID,
			EndpointGeneration: 1, ConfirmedAt: confirmedAt,
		}
		if _, err := manager.Unbind(t.Context(), UnbindRequest{Subject: ref}); err == nil {
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
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-device-actor"}
		store.subjects[ref] = Subject{
			SubjectRef: ref,
			Scope:      ScopeGlobal,
			Path:       "/api/webhooks/global/device--wbh_actor",
		}
		type deviceActorKey struct{}
		const actorKind = "operator_device"
		manager.remoteActor = func(ctx context.Context) (string, string, bool) {
			id, ok := ctx.Value(deviceActorKey{}).(string)
			return actorKind, id, ok
		}
		ctx := context.WithValue(t.Context(), deviceActorKey{}, "device-actor")
		if _, err := manager.Bind(ctx, BindRequest{Subject: ref, Confirmed: true}); err != nil {
			t.Fatalf("bind() error = %v", err)
		}
		if len(sink.bound) != 1 || sink.bound[0].ActorKind != actorKind ||
			sink.bound[0].ActorID != "device-actor" {
			t.Fatalf("bound events = %#v, want authenticated device attribution", sink.bound)
		}
	})

	t.Run("Should UT-140 attribute a local binding event to the canonical human actor", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		sink := &ingressEventSinkStub{}
		manager.events = sink
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-local-actor"}
		store.subjects[ref] = Subject{
			SubjectRef: ref,
			Scope:      ScopeGlobal,
			Path:       "/api/webhooks/global/local--wbh_actor",
		}
		if _, err := manager.Bind(t.Context(), BindRequest{Subject: ref, Confirmed: true}); err != nil {
			t.Fatalf("bind() error = %v", err)
		}
		if len(sink.bound) != 1 || sink.bound[0].ActorKind != string(workspaceaccess.ActorHuman) ||
			sink.bound[0].ActorID != "local-user" {
			t.Fatalf("bound events = %#v, want canonical local human attribution", sink.bound)
		}
	})

	t.Run(
		"Should UT-094 require subject reconfirmation after the public address changes [IT-048]",
		func(t *testing.T) {
			t.Parallel()
			store, manager := newIngressTestManager(t, true, nil)
			refs := []SubjectRef{
				{Kind: SubjectWebhookTrigger, ID: "trigger-generation"},
				{Kind: SubjectWebhookTrigger, ID: "trigger-generation-secondary"},
			}
			paths := []string{
				"/api/webhooks/global/deploy--wbh_generation",
				"/api/webhooks/global/secondary--wbh_generation_secondary",
			}
			for index, ref := range refs {
				store.subjects[ref] = Subject{
					SubjectRef: ref,
					Scope:      ScopeGlobal,
					Path:       paths[index],
				}
				if _, err := manager.Bind(t.Context(), BindRequest{
					Subject: ref, Confirmed: true,
				}); err != nil {
					t.Fatalf("bind(%s) error = %v", ref.Kind, err)
				}
			}
			if _, changed, err := store.SyncIngressEndpoint(
				t.Context(), "https://gateway-two.example.test", time.Now().UTC(),
			); err != nil || !changed {
				t.Fatalf("SyncIngressEndpoint() = changed:%t error:%v, want changed", changed, err)
			}

			for _, ref := range refs {
				projection, err := manager.Project(t.Context(), ref)
				if err != nil {
					t.Fatalf("project(%s) error = %v", ref.Kind, err)
				}
				if projection.Reachability != ReachabilityReconfirmation ||
					projection.EndpointGeneration != 2 || projection.ConfirmedEndpointGeneration != 1 ||
					!strings.HasPrefix(projection.URL, "https://gateway-two.example.test/") {
					t.Fatalf("projection(%s) = %#v, want generation 2 requiring reconfirmation", ref.Kind, projection)
				}
			}
		},
	)

	t.Run("Should UT-095 persist only the server-resolved owner", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-owner"}
		store.subjects[ref] = Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-owner",
			Path:        "/api/webhooks/workspaces/workspace-owner/deploy--wbh_owner",
		}

		if _, err := manager.Bind(t.Context(), BindRequest{Subject: ref, Confirmed: true}); err != nil {
			t.Fatalf("bind() error = %v", err)
		}
		if store.lastPut.Scope != ScopeWorkspace || store.lastPut.WorkspaceID != "workspace-owner" {
			t.Fatalf("PutIngressBinding() subject = %#v, want server-resolved owner", store.lastPut)
		}
	})

	t.Run("Should UT-096 deny a caller from another workspace", func(t *testing.T) {
		t.Parallel()
		access := &ingressAccessPolicy{}
		store, manager := newIngressTestManager(t, true, access)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-foreign"}
		store.subjects[ref] = Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-owner",
			Path:        "/api/webhooks/workspaces/workspace-owner/deploy--wbh_foreign",
		}
		caller := &Caller{SessionID: "session-agent", WorkspaceID: "workspace-other", AgentName: "coder"}

		_, err := manager.Bind(t.Context(), BindRequest{
			Subject: ref, Confirmed: true, Caller: caller,
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("bind(cross-workspace) error = %v, want ErrForbidden", err)
		}
		if access.calls != 1 || len(store.bindings) != 0 {
			t.Fatalf(
				"authorization calls = %d bindings = %#v, want one denial and no mutation",
				access.calls,
				store.bindings,
			)
		}
	})

	t.Run("Should authorize every workspace-scoped agent mutation through the canonical policy", func(t *testing.T) {
		t.Parallel()
		access := &ingressAccessPolicy{decision: workspaceaccess.Decision{
			Allowed: true,
			Source:  workspaceaccess.SourceSameWorkspace,
		}}
		store, manager := newIngressTestManager(t, true, access)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-same-workspace"}
		store.subjects[ref] = Subject{
			SubjectRef:  ref,
			Scope:       ScopeWorkspace,
			WorkspaceID: "workspace-owner",
			Path:        "/api/webhooks/workspaces/workspace-owner/deploy--wbh_same",
		}
		caller := &Caller{
			SessionID: "session-agent", WorkspaceID: "workspace-owner", AgentName: "coder",
		}

		if _, err := manager.Bind(t.Context(), BindRequest{
			Subject: ref, Confirmed: true, Caller: caller,
		}); err != nil {
			t.Fatalf("bind(same workspace) error = %v", err)
		}
		if access.calls != 1 {
			t.Fatalf("authorization calls = %d, want one", access.calls)
		}
	})

	t.Run("Should deny agent mutations of operator-global ingress subjects", func(t *testing.T) {
		t.Parallel()
		access := &ingressAccessPolicy{decision: workspaceaccess.Decision{
			Allowed: false,
			Source:  workspaceaccess.SourceDenied,
		}}
		store, manager := newIngressTestManager(t, true, access)
		ref := SubjectRef{Kind: SubjectWebhookTrigger, ID: "trigger-global-agent"}
		store.subjects[ref] = Subject{
			SubjectRef: ref,
			Scope:      ScopeGlobal,
			Path:       "/api/webhooks/global/deploy--wbh_global_agent",
		}
		caller := &Caller{
			SessionID: "session-agent", WorkspaceID: "workspace-owner", AgentName: "coder",
		}

		_, err := manager.Bind(t.Context(), BindRequest{
			Subject: ref, Confirmed: true, Caller: caller,
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("bind(global agent) error = %v, want ErrForbidden", err)
		}
		if access.calls != 1 || len(store.bindings) != 0 {
			t.Fatalf(
				"authorization calls = %d bindings = %#v, want global denial before mutation",
				access.calls,
				store.bindings,
			)
		}
	})

	t.Run("Should filter binding reads to the validated agent workspace", func(t *testing.T) {
		t.Parallel()
		store, manager := newIngressTestManager(t, true, nil)
		for _, subject := range []Subject{
			{
				Kind: SubjectWebhookTrigger, ID: "trigger-global",
				Scope: ScopeGlobal,
				Path:  "/api/webhooks/global/global--wbh_global",
			},
			{
				Kind: SubjectWebhookTrigger, ID: "trigger-a",
				Scope:       ScopeWorkspace,
				WorkspaceID: "workspace-a",
				Path:        "/api/webhooks/workspaces/workspace-a/a--wbh_a",
			},
			{
				Kind: SubjectWebhookTrigger, ID: "trigger-b",
				Scope:       ScopeWorkspace,
				WorkspaceID: "workspace-b",
				Path:        "/api/webhooks/workspaces/workspace-b/b--wbh_b",
			},
		} {
			store.subjects[subject.SubjectRef] = subject
			store.bindings[subject.SubjectRef] = Binding{
				Subject: subject.SubjectRef, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID,
				EndpointGeneration: 1,
			}
		}
		operatorBindings, err := manager.ListBindingProjections(t.Context())
		if err != nil {
			t.Fatalf("ListBindingProjections(operator) error = %v", err)
		}
		if got := len(operatorBindings); got != 3 {
			t.Fatalf("operator binding count = %d, want 3", got)
		}
		agentBindings, err := manager.ListBindingProjections(WithReadWorkspace(t.Context(), "workspace-a"))
		if err != nil {
			t.Fatalf("ListBindingProjections(agent) error = %v", err)
		}
		if got := len(agentBindings); got != 2 {
			t.Fatalf("agent binding count = %d, want global plus workspace-a", got)
		}
		for _, binding := range agentBindings {
			if binding.Scope == ScopeWorkspace && binding.WorkspaceID != "workspace-a" {
				t.Fatalf("agent status leaked foreign binding %#v", binding)
			}
		}
	})
}

func TestIngressRateLimiterHighCardinalityCannotResetAnActiveBudget(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve an active ingress budget during high-cardinality churn", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 8, 6, 15, 0, 0, 0, time.UTC)
		limiter := NewRateLimiter(1, time.Minute, func() time.Time { return now })
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
	})
}

type ingressTestStore struct {
	mu       sync.Mutex
	subjects map[SubjectRef]Subject
	bindings map[SubjectRef]Binding
	endpoint EndpointState
	lastPut  Subject
}

func newIngressTestManager(
	t *testing.T,
	publicOn bool,
	access workspaceaccess.Policy,
) (*ingressTestStore, *Manager) {
	t.Helper()
	store := &ingressTestStore{
		subjects: make(map[SubjectRef]Subject),
		bindings: make(map[SubjectRef]Binding),
		endpoint: EndpointState{
			URL: "https://gateway.example.test", Generation: 1,
			Reachable: true, UpdatedAt: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		},
	}
	manager, err := NewManager(
		store,
		access,
		nil,
		func() time.Time { return time.Date(2026, 8, 6, 12, 30, 0, 0, time.UTC) },
		nil,
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return store, manager
}

func (s *ingressTestStore) ResolveIngressSubject(
	_ context.Context,
	ref SubjectRef,
) (Subject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subject, ok := s.subjects[ref]
	if !ok {
		return Subject{}, ErrSubjectNotFound
	}
	return subject, nil
}

func (s *ingressTestStore) GetIngressEndpoint(context.Context) (EndpointState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.endpoint.Reachable || strings.TrimSpace(s.endpoint.URL) == "" {
		return EndpointState{}, ErrEndpointUnverified
	}
	return s.endpoint, nil
}

func (s *ingressTestStore) SyncIngressEndpoint(
	_ context.Context,
	endpointURL string,
	now time.Time,
) (EndpointState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.endpoint.URL != endpointURL
	if s.endpoint.Generation == 0 {
		s.endpoint.Generation = 1
	} else if changed {
		s.endpoint.Generation++
	}
	s.endpoint.URL = endpointURL
	s.endpoint.Reachable = true
	s.endpoint.UpdatedAt = now
	return s.endpoint, changed, nil
}

func (s *ingressTestStore) MarkIngressEndpointUnavailable(context.Context, time.Time) error {
	s.mu.Lock()
	s.endpoint.Reachable = false
	s.mu.Unlock()
	return nil
}

func (s *ingressTestStore) GetIngressBinding(
	_ context.Context,
	ref SubjectRef,
) (Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subjects[ref]; !ok {
		return Binding{}, ErrSubjectNotFound
	}
	binding, ok := s.bindings[ref]
	if !ok {
		return Binding{}, ErrSubjectNotFound
	}
	return binding, nil
}

func (s *ingressTestStore) PutIngressBinding(
	_ context.Context,
	subject Subject,
	generation uint64,
	confirmedAt time.Time,
) (Binding, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPut = subject
	current, exists := s.bindings[subject.SubjectRef]
	changed := !exists || current.EndpointGeneration != generation || current.Scope != subject.Scope ||
		current.WorkspaceID != subject.WorkspaceID
	binding := Binding{
		Subject: subject.SubjectRef, Scope: subject.Scope, WorkspaceID: subject.WorkspaceID,
		EndpointGeneration: generation, ConfirmedAt: confirmedAt,
	}
	s.bindings[subject.SubjectRef] = binding
	return binding, changed, nil
}

func (s *ingressTestStore) DeleteIngressBinding(_ context.Context, subject Subject) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.bindings[subject.SubjectRef]
	delete(s.bindings, subject.SubjectRef)
	return exists, nil
}

func (s *ingressTestStore) ListIngressBindings(context.Context) ([]Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bindings := make([]Binding, 0, len(s.bindings))
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
	calls    int
	decision workspaceaccess.Decision
	err      error
}

type ingressEventSinkStub struct {
	err     error
	bound   []MutationEvent
	unbound []MutationEvent
}

func (s *ingressEventSinkStub) RecordIngressBound(_ context.Context, event MutationEvent) error {
	s.bound = append(s.bound, event)
	return s.err
}

func (s *ingressEventSinkStub) RecordIngressUnbound(_ context.Context, event MutationEvent) error {
	s.unbound = append(s.unbound, event)
	return s.err
}

func (p *ingressAccessPolicy) Authorize(
	context.Context,
	workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	p.calls++
	decision := p.decision
	if decision.Source == "" {
		decision = workspaceaccess.Decision{Allowed: false, Source: workspaceaccess.SourceDenied}
	}
	return decision, p.err
}

func TestAdvertisedBase(t *testing.T) {
	t.Parallel()
	const path = "/api/webhooks/workspaces/ws-a/deploy--wbh_specific"
	for _, tc := range []struct {
		name, hostname, custom, want string
	}{
		{"Should prefer the active hosted route (UT-073)", "devbox-7f3k.compozy.run",
			"https://hooks.example.test", "https://devbox-7f3k.compozy.run"},
		{"Should advertise the configured HTTPS proxy (UT-073)", "",
			"https://hooks.example.test/", "https://hooks.example.test"},
		{"Should report unavailable without a public endpoint (UT-073)", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, err := AdvertisedBase(tc.hostname, tc.custom)
			if tc.want == "" {
				if base != "" || !errors.Is(err, ErrUnavailable) {
					t.Fatalf("advertised base = %q, %v", base, err)
				}
				return
			}
			endpoint, pathErr := publicIngressURL(base, path)
			if err != nil || pathErr != nil || base != tc.want || endpoint != tc.want+path {
				t.Fatalf("advertised endpoint = %q, %v", endpoint, errors.Join(err, pathErr))
			}
		})
	}
}
