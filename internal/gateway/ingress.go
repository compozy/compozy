package gateway

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
	ingressEnablePath          = "/api/gateway/surfaces"
	ingressCompensationTimeout = 5 * time.Second
)

// IngressEventSink records durable binding lifecycle events.
type IngressEventSink interface {
	RecordIngressBound(context.Context, IngressMutationEvent) error
	RecordIngressUnbound(context.Context, IngressMutationEvent) error
}

type ingressManager struct {
	store  IngressStore
	policy Policy
	access workspaceaccess.Policy
	events IngressEventSink
	now    func() time.Time
	mu     sync.Mutex
}

func newIngressManager(
	store IngressStore,
	policy Policy,
	access workspaceaccess.Policy,
	events IngressEventSink,
	now func() time.Time,
) (*ingressManager, error) {
	if store == nil {
		return nil, errors.New("gateway: ingress store is required")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if policy == nil {
		return nil, errors.New("gateway: ingress policy is required")
	}
	return &ingressManager{
		store:  store,
		policy: policy,
		access: access,
		events: events,
		now:    now,
	}, nil
}

func (m *ingressManager) bind(ctx context.Context, req IngressBindRequest) (IngressBinding, error) {
	if ctx == nil {
		return IngressBinding{}, errors.New("gateway: bind ingress context is required")
	}
	if err := req.Subject.Validate(); err != nil {
		return IngressBinding{}, err
	}
	if !req.Confirmed {
		return IngressBinding{}, errors.New("gateway: ingress endpoint confirmation is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	subject, err := m.store.ResolveIngressSubject(ctx, req.Subject)
	if err != nil {
		return IngressBinding{}, err
	}
	if subject.TargetUnavailable {
		return IngressBinding{}, ErrIngressTargetUnavailable
	}
	if err := m.authorize(ctx, req.Caller, subject); err != nil {
		return IngressBinding{}, err
	}
	publicOn, err := m.publicIngressOn(ctx)
	if err != nil {
		return IngressBinding{}, err
	}
	if !publicOn {
		return IngressBinding{}, refusalError(
			"public ingress is not reachable",
			"enable the public webhook ingress surface and wait for a verified address",
		)
	}
	endpoint, err := m.store.GetIngressEndpoint(ctx, TierPublic)
	if err != nil || !endpoint.Reachable || strings.TrimSpace(endpoint.URL) == "" {
		if err != nil && !errors.Is(err, ErrEndpointUnverified) {
			return IngressBinding{}, err
		}
		return IngressBinding{}, refusalError(
			"public ingress is not reachable",
			"enable the public webhook ingress surface and wait for a verified address",
		)
	}
	previous, previousErr := m.store.GetIngressBinding(ctx, req.Subject)
	if previousErr != nil && !errors.Is(previousErr, ErrIngressSubjectNotFound) {
		return IngressBinding{}, previousErr
	}
	binding, changed, err := m.store.PutIngressBinding(
		ctx,
		subject,
		endpoint.Generation,
		m.now().UTC(),
	)
	if err != nil {
		return IngressBinding{}, err
	}
	binding.Changed = changed
	if changed && m.events != nil {
		if eventErr := m.events.RecordIngressBound(
			ctx,
			ingressMutationEvent(ctx, req.Caller, binding),
		); eventErr != nil {
			compensationCtx, cancelCompensation := ingressCompensationContext(ctx)
			defer cancelCompensation()
			compensationErr := m.restoreBinding(
				compensationCtx,
				subject,
				previous,
				previousErr == nil,
			)
			return IngressBinding{}, errors.Join(
				fmt.Errorf("gateway: record ingress bound event: %w", eventErr),
				compensationErr,
			)
		}
	}
	return binding, nil
}

func (m *ingressManager) unbind(
	ctx context.Context,
	req IngressUnbindRequest,
) (UnbindResult, error) {
	if ctx == nil {
		return UnbindResult{}, errors.New("gateway: unbind ingress context is required")
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
	if existingErr != nil && !errors.Is(existingErr, ErrIngressSubjectNotFound) {
		return UnbindResult{}, existingErr
	}
	changed, err := m.store.DeleteIngressBinding(ctx, subject)
	if err != nil {
		return UnbindResult{}, err
	}
	if changed && m.events != nil {
		if eventErr := m.events.RecordIngressUnbound(
			ctx,
			ingressMutationEvent(ctx, req.Caller, existing),
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
				fmt.Errorf("gateway: record ingress unbound event: %w", eventErr),
				compensationErr,
			)
		}
	}
	return UnbindResult{Subject: req.Subject, Changed: changed}, nil
}

func ingressCompensationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), ingressCompensationTimeout)
}

func (m *ingressManager) project(
	ctx context.Context,
	ref IngressSubjectRef,
) (IngressProjection, error) {
	subject, err := m.store.ResolveIngressSubject(ctx, ref)
	if err != nil {
		return IngressProjection{}, err
	}
	projection := IngressProjection{
		Subject: subject.IngressSubjectRef, Scope: subject.Scope,
		WorkspaceID: subject.WorkspaceID, Reachability: IngressReachabilityOff,
		EnablePath: ingressEnablePath,
	}
	if readWorkspaceID, scopedRead := ingressReadWorkspace(ctx); scopedRead &&
		subject.Scope == IngressScopeWorkspace && subject.WorkspaceID != readWorkspaceID {
		return IngressProjection{}, ErrIngressForbidden
	}
	if subject.TargetUnavailable {
		projection.Reachability = IngressReachabilityBroken
		return projection, nil
	}
	binding, bindingErr := m.store.GetIngressBinding(ctx, ref)
	if bindingErr == nil {
		projection.ConfirmedAt = binding.ConfirmedAt
		projection.ConfirmedEndpointGeneration = binding.EndpointGeneration
	} else if !errors.Is(bindingErr, ErrIngressSubjectNotFound) {
		return IngressProjection{}, bindingErr
	}
	publicOn, err := m.publicIngressOn(ctx)
	if err != nil {
		return IngressProjection{}, err
	}
	if !publicOn {
		return projection, nil
	}
	endpoint, err := m.store.GetIngressEndpoint(ctx, TierPublic)
	if err != nil {
		if errors.Is(err, ErrEndpointUnverified) {
			return projection, nil
		}
		return IngressProjection{}, err
	}
	projection.EndpointGeneration = endpoint.Generation
	if !endpoint.Reachable {
		return projection, nil
	}
	projection.URL, err = publicIngressURL(endpoint.URL, subject.Path)
	if err != nil {
		return IngressProjection{}, err
	}
	projection.Reachability = IngressReachabilityUnconfirmed
	if bindingErr != nil {
		if errors.Is(bindingErr, ErrIngressSubjectNotFound) {
			return projection, nil
		}
		return IngressProjection{}, bindingErr
	}
	if binding.EndpointGeneration == endpoint.Generation {
		projection.Reachability = IngressReachabilityLive
	} else {
		projection.Reachability = IngressReachabilityReconfirmation
	}
	return projection, nil
}

func (m *ingressManager) restoreBinding(
	ctx context.Context,
	subject IngressSubject,
	previous IngressBinding,
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

func ingressMutationEvent(
	ctx context.Context,
	caller *IngressCaller,
	binding IngressBinding,
) IngressMutationEvent {
	event := IngressMutationEvent{Binding: binding}
	if caller != nil {
		event.ActorKind = string(workspaceaccess.ActorAgentSession)
		event.ActorID = strings.TrimSpace(caller.SessionID)
		return event
	}
	if device, ok := DeviceFromContext(ctx); ok {
		event.ActorKind = string(device.ActorKind)
		event.ActorID = strings.TrimSpace(device.ID)
	}
	return event
}

func (m *ingressManager) publicIngressOn(ctx context.Context) (bool, error) {
	status, err := m.policy.Status(ctx)
	if err != nil {
		return false, err
	}
	for _, surface := range status.Surfaces {
		if surface.Surface == SurfaceWebhookIngress && surface.Tier == TierPublic &&
			surface.Desired == DesiredEnabled && surface.Observed == SurfaceOn {
			return true, nil
		}
	}
	return false, nil
}

func (m *ingressManager) authorize(
	ctx context.Context,
	caller *IngressCaller,
	subject IngressSubject,
) error {
	if caller == nil {
		return nil
	}
	if subject.Scope == IngressScopeGlobal ||
		strings.TrimSpace(caller.WorkspaceID) == subject.WorkspaceID {
		return nil
	}
	if m.access == nil {
		return ErrIngressForbidden
	}
	decision, err := m.access.Authorize(ctx, workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{
			Kind: workspaceaccess.ActorAgentSession, SessionID: caller.SessionID,
			WorkspaceID: caller.WorkspaceID, AgentName: caller.AgentName,
		},
		TargetWorkspaceID: subject.WorkspaceID,
		Seam:              workspaceaccess.SeamIdentity,
	})
	if err != nil {
		return errors.Join(ErrIngressForbidden, err)
	}
	if !decision.Allowed {
		return ErrIngressForbidden
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
