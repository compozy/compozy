package gateway

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// AuthGate reports whether the gateway authentication model is ready.
type AuthGate interface {
	Active(context.Context) (bool, error)
}

// AuthGateFunc adapts a function to AuthGate.
type AuthGateFunc func(context.Context) (bool, error)

// Active implements AuthGate.
func (f AuthGateFunc) Active(ctx context.Context) (bool, error) { return f(ctx) }

// Policy is the gateway exposure authority used by the daemon composition root.
type Policy interface {
	Plan(context.Context) (ExposurePlan, error)
	Transition(context.Context, TransitionRequest) (Status, error)
	Reconcile(context.Context) (Status, error)
	SetEnabled(context.Context, bool) error
	Status(context.Context) (Status, error)
	Acquire(Tier, Surface) (func(), error)
	Close(context.Context) error
}

type policy struct {
	store      Store
	reconciler *Reconciler
	auth       AuthGate
	now        func() time.Time
	enabled    atomic.Bool
	mu         sync.Mutex
	refusalMu  sync.RWMutex
	refusal    *Refusal
}

// Option customizes Policy construction.
type Option func(*policy)

// WithAuthGate supplies the authentication readiness authority.
func WithAuthGate(gate AuthGate) Option {
	return func(p *policy) {
		if gate != nil {
			p.auth = gate
		}
	}
}

// WithClock supplies a deterministic clock.
func WithClock(now func() time.Time) Option {
	return func(p *policy) {
		if now != nil {
			p.now = now
		}
	}
}

// NewPolicy constructs the durable gateway exposure policy.
func NewPolicy(store Store, effects EffectDriver, enabled bool, options ...Option) (Policy, error) {
	if store == nil {
		return nil, errors.New("gateway: store is required")
	}
	p := &policy{
		store: store,
		auth: AuthGateFunc(func(context.Context) (bool, error) {
			return false, nil
		}),
		now: func() time.Time { return time.Now().UTC() },
	}
	p.enabled.Store(enabled)
	for _, option := range options {
		if option != nil {
			option(p)
		}
	}
	p.reconciler = NewReconciler(store, effects, WithReconcilerClock(p.now))
	return p, nil
}

func (p *policy) Plan(ctx context.Context) (ExposurePlan, error) {
	snapshot, err := p.store.Snapshot(ctx)
	if err != nil {
		return ExposurePlan{}, err
	}
	return planSnapshot(snapshot), nil
}

func (p *policy) Status(ctx context.Context) (Status, error) {
	snapshot, err := p.store.Snapshot(ctx)
	if err != nil {
		return Status{}, err
	}
	return projectStatus(snapshot, p.reconciler.Runtime(), p.enabled.Load(), p.currentRefusal()), nil
}

func (p *policy) Acquire(tier Tier, surface Surface) (func(), error) {
	return p.reconciler.Acquire(tier, surface)
}

func (p *policy) Close(ctx context.Context) error {
	return p.reconciler.Close(ctx)
}

func (p *policy) setRefusal(err error) {
	var refusal RefusalError
	p.refusalMu.Lock()
	defer p.refusalMu.Unlock()
	if errors.As(err, &refusal) {
		p.refusal = &Refusal{Cause: refusal.Cause, Fix: refusal.Fix}
		return
	}
	p.refusal = nil
}

func (p *policy) currentRefusal() *Refusal {
	p.refusalMu.RLock()
	defer p.refusalMu.RUnlock()
	if p.refusal == nil {
		return nil
	}
	refusalCopy := *p.refusal
	return &refusalCopy
}

func planSnapshot(snapshot Snapshot) ExposurePlan {
	plan := ExposurePlan{Tiers: []TierPlan{{Tier: TierPrivate}, {Tier: TierPublic}}}
	for i := range plan.Tiers {
		tierPlan := &plan.Tiers[i]
		for j := range snapshot.Providers {
			provider := snapshot.Providers[j]
			if provider.Tier == tierPlan.Tier && provider.Desired == DesiredEnabled {
				providerCopy := provider
				tierPlan.Provider = &providerCopy
				break
			}
		}
		for _, surface := range snapshot.Surfaces {
			if surface.Tier == tierPlan.Tier && surface.Desired == DesiredEnabled {
				tierPlan.Surfaces = append(tierPlan.Surfaces, surface)
			}
		}
		tierPlan.Enabled = tierPlan.Provider != nil && len(tierPlan.Surfaces) > 0
	}
	return plan
}
