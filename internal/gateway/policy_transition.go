package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (p *policy) Transition(ctx context.Context, req TransitionRequest) (Status, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.validateTransition(ctx, req); err != nil {
		p.setRefusal(err)
		status, statusErr := p.Status(ctx)
		return status, errors.Join(err, statusErr)
	}
	changed, err := p.store.Transition(ctx, req, p.now())
	if err != nil {
		return Status{}, fmt.Errorf("gateway: persist desired transition: %w", err)
	}
	if changed {
		if err := p.reconcileLocked(ctx); err != nil {
			p.setRefusal(err)
			status, statusErr := p.Status(ctx)
			status.Changed = true
			return status, errors.Join(err, statusErr)
		}
	}
	p.setRefusal(nil)
	status, err := p.Status(ctx)
	status.Changed = changed
	return status, err
}

func (p *policy) Reconcile(ctx context.Context) (Status, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.reconcileLocked(ctx); err != nil {
		p.setRefusal(err)
		status, statusErr := p.Status(ctx)
		return status, errors.Join(err, statusErr)
	}
	p.setRefusal(nil)
	return p.Status(ctx)
}

func (p *policy) SetEnabled(ctx context.Context, enabled bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !enabled {
		p.enabled.Store(false)
		_, disableErr := p.store.DisableAll(ctx)
		cleanupErr := p.reconciler.Reconcile(ctx, emptyExposurePlan())
		if disableErr != nil {
			disableErr = fmt.Errorf("gateway: disable durable exposure after ceiling change: %w", disableErr)
		}
		resultErr := errors.Join(disableErr, cleanupErr)
		if resultErr != nil {
			p.setRefusal(refusalError(
				"gateway disable did not complete",
				"repair the failing listener or provider, then retry disabling the gateway",
			))
			return resultErr
		}
		p.setRefusal(nil)
		return nil
	}
	previous := p.enabled.Load()
	p.enabled.Store(true)
	if err := p.reconcileLocked(ctx); err != nil {
		p.setRefusal(err)
		p.enabled.Store(previous)
		if !previous {
			_, disableErr := p.store.DisableAll(ctx)
			cleanupErr := p.reconciler.Reconcile(ctx, emptyExposurePlan())
			return errors.Join(err, disableErr, cleanupErr)
		}
		return err
	}
	p.setRefusal(nil)
	return nil
}

func (p *policy) reconcileLocked(ctx context.Context) error {
	if !p.enabled.Load() {
		if _, err := p.store.DisableAll(ctx); err != nil {
			return fmt.Errorf("gateway: enforce disabled ceiling: %w", err)
		}
		return p.reconciler.Reconcile(ctx, emptyExposurePlan())
	}
	plan, err := p.Plan(ctx)
	if err != nil {
		return err
	}
	if hasEnabledTier(plan) {
		active, authErr := p.auth.Active(ctx)
		if authErr != nil {
			return fmt.Errorf("gateway: check authentication readiness: %w", authErr)
		}
		if !active {
			cleanupErr := p.reconciler.Reconcile(ctx, disabledExposurePlan(plan))
			refusal := refusalError(
				"the gateway authentication model is inactive",
				"initialize pairing and device authentication before enabling remote reachability",
			)
			return errors.Join(refusal, cleanupErr)
		}
	}
	return p.reconciler.Reconcile(ctx, plan)
}

func emptyExposurePlan() ExposurePlan {
	return ExposurePlan{Tiers: []TierPlan{{Tier: TierPrivate}, {Tier: TierPublic}}}
}

func disabledExposurePlan(plan ExposurePlan) ExposurePlan {
	disabled := ExposurePlan{Tiers: make([]TierPlan, 0, len(plan.Tiers))}
	for _, tierPlan := range plan.Tiers {
		tierPlan.Enabled = false
		disabled.Tiers = append(disabled.Tiers, tierPlan)
	}
	return disabled
}

func (p *policy) validateTransition(ctx context.Context, req TransitionRequest) error {
	if err := p.validateTransitionRequest(req); err != nil {
		return err
	}
	if err := p.validateTransitionAuth(ctx, req); err != nil {
		return err
	}
	snapshot, err := p.store.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("gateway: load transition state: %w", err)
	}
	return validateTransitionTarget(req, snapshot)
}

func (p *policy) validateTransitionRequest(req TransitionRequest) error {
	if strings.TrimSpace(req.LegacyMode) != "" {
		return refusalError(
			"legacy exposure mode "+fmt.Sprintf("%q", req.LegacyMode)+" is not supported",
			"choose an explicit provider or surface transition for the private or public tier",
		)
	}
	if err := req.Tier.Validate(); err != nil {
		return refusalError(err.Error(), "choose private or public")
	}
	if err := req.Desired.Validate(); err != nil {
		return refusalError(err.Error(), "choose enabled or disabled")
	}
	if req.Desired == DesiredEnabled && !p.enabled.Load() {
		return refusalError(
			"gateway.enabled is false and blocks every remote transition",
			"set gateway.enabled=true, then retry the explicit transition",
		)
	}
	return nil
}

func (p *policy) validateTransitionAuth(ctx context.Context, req TransitionRequest) error {
	if req.Desired == DesiredEnabled {
		active, err := p.auth.Active(ctx)
		if err != nil {
			return fmt.Errorf("gateway: check authentication readiness: %w", err)
		}
		if !active {
			return refusalError(
				"the gateway authentication model is inactive",
				"initialize pairing and device authentication before enabling remote reachability",
			)
		}
	}
	return nil
}

func validateTransitionTarget(req TransitionRequest, snapshot Snapshot) error {
	switch req.Target {
	case TargetProvider:
		if strings.TrimSpace(req.Provider.Name) == "" {
			return refusalError("provider name is required", "choose the provider to transition")
		}
		if req.Desired == DesiredEnabled {
			return req.Provider.Validate()
		}
		return nil
	case TargetSurface:
		if err := req.Surface.Validate(); err != nil {
			return refusalError(err.Error(), "choose operator_ui or webhook_ingress")
		}
		if req.Surface == SurfaceWebhookIngress && req.Tier != TierPublic {
			return refusalError(
				"webhook ingress is available only on the public tier",
				"enable webhook_ingress on the public tier",
			)
		}
		if req.Desired == DesiredEnabled && !hasDesiredProvider(snapshot, req.Tier) {
			return refusalError(
				"no connectivity provider is enabled for the "+string(req.Tier)+" tier",
				"enable a trusted provider for that tier before enabling a surface",
			)
		}
		if req.Desired == DesiredEnabled && req.Surface == SurfaceOperatorUI && req.Tier == TierPublic {
			if len(snapshot.Devices) == 0 {
				return refusalError(
					"public operator UI requires at least one paired device",
					"pair a device through local or private access before enabling the public operator UI",
				)
			}
			if !req.Consent {
				return fmt.Errorf("%w: confirm public operator UI exposure before retrying", ErrConsentRequired)
			}
		}
		return nil
	default:
		return refusalError(
			"the requested exposure semantic is not recognized",
			"use a provider or surface transition; legacy exposure modes are not accepted",
		)
	}
}

func hasDesiredProvider(snapshot Snapshot, tier Tier) bool {
	for _, provider := range snapshot.Providers {
		if provider.Tier == tier && provider.Desired == DesiredEnabled {
			return true
		}
	}
	return false
}

func hasEnabledTier(plan ExposurePlan) bool {
	for _, tier := range plan.Tiers {
		if tier.Enabled {
			return true
		}
	}
	return false
}

func refusalError(cause string, fix string) error {
	return RefusalError{Cause: cause, Fix: fix}
}
