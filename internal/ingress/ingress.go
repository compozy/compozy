package ingress

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	// EnablePath is the operator API path used to enable public ingress.
	EnablePath                 = "/api/remote"
	ingressCompensationTimeout = 5 * time.Second
)

// EventSink records durable binding lifecycle events.
type EventSink interface {
	RecordIngressBound(context.Context, MutationEvent) error
	RecordIngressUnbound(context.Context, MutationEvent) error
}

type Manager struct {
	store       Store
	access      workspaceaccess.Policy
	events      EventSink
	now         func() time.Time
	remoteActor func(context.Context) (kind, id string, ok bool)
	mu          sync.Mutex
}

func NewManager(
	store Store,
	access workspaceaccess.Policy,
	events EventSink,
	now func() time.Time,
	remoteActor func(context.Context) (kind, id string, ok bool),
) (*Manager, error) {
	if store == nil {
		return nil, errors.New("ingress: store is required")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Manager{
		store:       store,
		access:      access,
		events:      events,
		now:         now,
		remoteActor: remoteActor,
	}, nil
}

func (m *Manager) Bind(ctx context.Context, req BindRequest) (Binding, error) {
	if ctx == nil {
		return Binding{}, errors.New("ingress: bind ingress context is required")
	}
	if err := req.Subject.Validate(); err != nil {
		return Binding{}, err
	}
	if !req.Confirmed {
		return Binding{}, errors.New("ingress: endpoint confirmation is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	subject, err := m.store.ResolveIngressSubject(ctx, req.Subject)
	if err != nil {
		return Binding{}, err
	}
	if subject.TargetUnavailable {
		return Binding{}, ErrTargetUnavailable
	}
	if err := m.authorize(ctx, req.Caller, subject); err != nil {
		return Binding{}, err
	}
	endpoint, err := m.store.GetIngressEndpoint(ctx)
	if err != nil || !endpoint.Reachable || strings.TrimSpace(endpoint.URL) == "" {
		if err != nil && !errors.Is(err, ErrEndpointUnverified) {
			return Binding{}, err
		}
		return Binding{}, ErrUnreachable
	}
	previous, previousErr := m.store.GetIngressBinding(ctx, req.Subject)
	if previousErr != nil && !errors.Is(previousErr, ErrSubjectNotFound) {
		return Binding{}, previousErr
	}
	binding, changed, err := m.store.PutIngressBinding(
		ctx,
		subject,
		endpoint.Generation,
		m.now().UTC(),
	)
	if err != nil {
		return Binding{}, err
	}
	binding.Changed = changed
	if changed && m.events != nil {
		if eventErr := m.events.RecordIngressBound(
			ctx,
			m.mutationEvent(ctx, req.Caller, binding),
		); eventErr != nil {
			compensationCtx, cancelCompensation := ingressCompensationContext(ctx)
			defer cancelCompensation()
			compensationErr := m.restoreBinding(
				compensationCtx,
				subject,
				previous,
				previousErr == nil,
			)
			return Binding{}, errors.Join(
				fmt.Errorf("ingress: record ingress bound event: %w", eventErr),
				compensationErr,
			)
		}
	}
	return binding, nil
}

func (m *Manager) Unbind(
	ctx context.Context,
	req UnbindRequest,
) (UnbindResult, error) {
	if ctx == nil {
		return UnbindResult{}, errors.New("ingress: unbind ingress context is required")
	}
	if err := req.Subject.Validate(); err != nil {
		return UnbindResult{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	subject, err := m.store.ResolveIngressSubject(ctx, req.Subject)
	if err != nil {
		return UnbindResult{}, err
	}
	if err := m.authorize(ctx, req.Caller, subject); err != nil {
		return UnbindResult{}, err
	}
	existing, existingErr := m.store.GetIngressBinding(ctx, req.Subject)
	if existingErr != nil && !errors.Is(existingErr, ErrSubjectNotFound) {
		return UnbindResult{}, existingErr
	}
	changed, err := m.store.DeleteIngressBinding(ctx, subject)
	if err != nil {
		return UnbindResult{}, err
	}
	if changed && m.events != nil {
		if eventErr := m.events.RecordIngressUnbound(
			ctx,
			m.mutationEvent(ctx, req.Caller, existing),
		); eventErr != nil {
			compensationCtx, cancelCompensation := ingressCompensationContext(ctx)
			defer cancelCompensation()
			_, _, compensationErr := m.store.PutIngressBinding(
				compensationCtx,
				subject,
				existing.EndpointGeneration,
				existing.ConfirmedAt,
			)
			return UnbindResult{}, errors.Join(
				fmt.Errorf("ingress: record ingress unbound event: %w", eventErr),
				compensationErr,
			)
		}
	}
	return UnbindResult{Subject: req.Subject, Changed: changed}, nil
}

func ingressCompensationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), ingressCompensationTimeout)
}

func (m *Manager) Project(
	ctx context.Context,
	ref SubjectRef,
) (Projection, error) {
	subject, err := m.store.ResolveIngressSubject(ctx, ref)
	if err != nil {
		return Projection{}, err
	}
	projection := Projection{
		Subject: subject.SubjectRef, Scope: subject.Scope,
		WorkspaceID: subject.WorkspaceID, Reachability: ReachabilityOff,
		EnablePath: EnablePath,
	}
	if readWorkspaceID, scopedRead := ingressReadWorkspace(ctx); scopedRead &&
		subject.Scope == ScopeWorkspace && subject.WorkspaceID != readWorkspaceID {
		return Projection{}, ErrForbidden
	}
	if subject.TargetUnavailable {
		projection.Reachability = ReachabilityBroken
		return projection, nil
	}
	binding, bindingErr := m.store.GetIngressBinding(ctx, ref)
	if bindingErr == nil {
		projection.ConfirmedAt = binding.ConfirmedAt
		projection.ConfirmedEndpointGeneration = binding.EndpointGeneration
	} else if !errors.Is(bindingErr, ErrSubjectNotFound) {
		return Projection{}, bindingErr
	}
	endpoint, err := m.store.GetIngressEndpoint(ctx)
	if err != nil {
		if errors.Is(err, ErrEndpointUnverified) {
			return projection, nil
		}
		return Projection{}, err
	}
	projection.EndpointGeneration = endpoint.Generation
	if !endpoint.Reachable {
		return projection, nil
	}
	projection.URL, err = publicIngressURL(endpoint.URL, subject.Path)
	if err != nil {
		return Projection{}, err
	}
	projection.Reachability = ReachabilityUnconfirmed
	if bindingErr != nil {
		if errors.Is(bindingErr, ErrSubjectNotFound) {
			return projection, nil
		}
		return Projection{}, bindingErr
	}
	if binding.EndpointGeneration == endpoint.Generation {
		projection.Reachability = ReachabilityLive
	} else {
		projection.Reachability = ReachabilityReconfirmation
	}
	return projection, nil
}

func (m *Manager) restoreBinding(
	ctx context.Context,
	subject Subject,
	previous Binding,
	hadPrevious bool,
) error {
	if !hadPrevious {
		_, err := m.store.DeleteIngressBinding(ctx, subject)
		return err
	}
	_, _, err := m.store.PutIngressBinding(
		ctx,
		subject,
		previous.EndpointGeneration,
		previous.ConfirmedAt,
	)
	return err
}

func (m *Manager) mutationEvent(
	ctx context.Context,
	caller *Caller,
	binding Binding,
) MutationEvent {
	event := MutationEvent{Binding: binding}
	if caller != nil {
		event.ActorKind = string(workspaceaccess.ActorAgentSession)
		event.ActorID = strings.TrimSpace(caller.SessionID)
		return event
	}
	if m.remoteActor != nil {
		if kind, id, ok := m.remoteActor(ctx); ok {
			event.ActorKind, event.ActorID = kind, strings.TrimSpace(id)
			return event
		}
	}
	event.ActorKind = string(workspaceaccess.ActorHuman)
	event.ActorID = "local-user"
	return event
}

func (m *Manager) authorize(
	ctx context.Context,
	caller *Caller,
	subject Subject,
) error {
	if caller == nil {
		return nil
	}
	if m.access == nil {
		return ErrForbidden
	}
	target := workspaceaccess.ScopeTarget(subject.WorkspaceID, subject.ProfileID)
	if subject.Scope == ScopeGlobal {
		target = workspaceaccess.TargetRef{
			Kind:      workspaceaccess.TargetAggregate,
			ProfileID: subject.ProfileID,
			Surface:   "webhook-ingress",
		}
	}
	decision, err := m.access.Authorize(ctx, workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{
			Kind: workspaceaccess.ActorAgentSession, SessionID: caller.SessionID,
			WorkspaceID: caller.WorkspaceID, AgentName: caller.AgentName,
		},
		Target: target,
		Seam:   workspaceaccess.SeamIdentity,
	})
	if err != nil {
		return errors.Join(ErrForbidden, err)
	}
	if !decision.Allowed {
		return ErrForbidden
	}
	return nil
}

func publicIngressURL(baseURL, subjectPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed == nil || !parsed.IsAbs() || parsed.Hostname() == "" {
		return "", ErrEndpointUnverified
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = path.Join(parsed.Path, subjectPath)
	return parsed.String(), nil
}

func (m *Manager) ListBindingProjections(ctx context.Context) ([]Projection, error) {
	bindings, err := m.store.ListIngressBindings(ctx)
	if err != nil {
		return nil, err
	}
	projections := make([]Projection, 0, len(bindings))
	readWorkspaceID, scopedRead := ingressReadWorkspace(ctx)
	for _, binding := range bindings {
		if scopedRead && binding.Scope == ScopeWorkspace && binding.WorkspaceID != readWorkspaceID {
			continue
		}
		projection, projectErr := m.Project(ctx, binding.Subject)
		if errors.Is(projectErr, ErrSubjectNotFound) {
			continue
		}
		if projectErr != nil {
			return nil, projectErr
		}
		projections = append(projections, projection)
	}
	return projections, nil
}
