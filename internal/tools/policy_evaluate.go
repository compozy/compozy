package tools

import "context"

type indexedPolicyEvaluator interface {
	evaluateIndexed(context.Context, Scope, Descriptor) (EffectiveToolDecision, error)
}

// Evaluate defensively validates descriptors supplied outside the registry index.
func (e *EffectivePolicyEvaluator) Evaluate(
	ctx context.Context,
	scope Scope,
	d Descriptor,
) (EffectiveToolDecision, error) {
	if err := d.Validate(); err != nil {
		return EffectiveToolDecision{}, err
	}
	return e.evaluateIndexed(ctx, scope, d)
}

func (e *EffectivePolicyEvaluator) evaluateIndexed(
	_ context.Context,
	_ Scope,
	d Descriptor,
) (EffectiveToolDecision, error) {
	decision := EffectiveToolDecision{
		VisibleToOperator:    true,
		VisibleToSession:     true,
		Callable:             true,
		SystemPermissionMode: string(e.permissionMode()),
		SessionPolicyResult:  policyResultUnrestricted,
		AgentPolicyResult:    policyResultUnrestricted,
		RegistryPolicyResult: policyResultAllowed,
		SourcePolicyResult:   policyResultAllowed,
		AvailabilityResult:   policyResultAllowed,
		HookResult:           policyResultAllowed,
	}

	if e.inputs.ToolsDisabled {
		denyDecision(&decision, ReasonSourceDisabled)
		decision.RegistryPolicyResult = policyResultDisabled
		return decision, nil
	}
	if !isSessionVisible(d.Visibility) {
		denyDecision(&decision, ReasonVisibilityDenied)
		decision.RegistryPolicyResult = policyResultDenied
		return decision, nil
	}
	if e.matchesAny(e.inputs.DenyTools, d.ID) {
		denyDecision(&decision, ReasonPolicyDenied)
		decision.RegistryPolicyResult = policyResultDenied
		return decision, nil
	}
	if e.matchesAny(e.inputs.Agent.DenyTools, d.ID) {
		denyDecision(&decision, ReasonPolicyDenied)
		decision.AgentPolicyResult = policyResultDenied
		return decision, nil
	}
	if e.inputs.Session.Enforced {
		if _, ok := e.sessionAllowed[d.ID]; !ok {
			denyDecision(&decision, ReasonSessionDenied)
			decision.SessionPolicyResult = policyResultDenied
			return decision, nil
		}
		decision.SessionPolicyResult = policyResultAllowed
	}
	if e.agentPolicyRestricts() {
		if _, ok := e.agentAllowed[d.ID]; !ok && !e.matchesAny(e.inputs.Agent.Tools, d.ID) {
			denyDecision(&decision, ReasonPolicyDenied)
			decision.AgentPolicyResult = policyResultDenied
			return decision, nil
		}
		decision.AgentPolicyResult = policyResultAllowed
	}

	sourceAllowed, requiresSourceApproval := e.evaluateSourcePolicy(d, &decision)
	if !sourceAllowed {
		return decision, nil
	}
	if requiresSourceApproval {
		requireApproval(&decision, e.inputs.ApprovalAvailable)
	}
	e.applyPermissionCeiling(d, &decision)
	return decision, nil
}

func evaluateIndexedDescriptor(
	ctx context.Context,
	scope Scope,
	evaluator PolicyEvaluator,
	descriptor Descriptor,
) (EffectiveToolDecision, error) {
	if indexed, ok := evaluator.(indexedPolicyEvaluator); ok {
		return indexed.evaluateIndexed(ctx, scope, descriptor)
	}
	return evaluator.Evaluate(ctx, scope, descriptor)
}
