package tools

import (
	"context"
	"encoding/json"
	"strings"
)

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
	return e.evaluateCall(d, nil)
}

// evaluateCall keeps all policy restrictions and only refines the approval ceiling.
func (e *EffectivePolicyEvaluator) evaluateCall(d Descriptor, input json.RawMessage) (EffectiveToolDecision, error) {
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

	if e.inputs.TriageReadOnly &&
		(!isReadOnlyAutoApprovable(d) || d.Source.Kind != SourceBuiltin ||
			strings.HasPrefix(string(d.ID), "compozy__terminal_")) {
		denyDecision(&decision, ReasonPolicyDenied)
		return decision, nil
	}
	if e.inputs.ToolsDisabled {
		denyDecision(&decision, ReasonSourceDisabled)
		// The restrictions below still apply when tools are disabled: their reasons let callers tell a
		// plain disabled source apart from one that policy would also deny.
		e.applyRestrictions(d, &decision)
		decision.RegistryPolicyResult = policyResultDisabled
		return decision, nil
	}
	if e.applyRestrictions(d, &decision) {
		return decision, nil
	}

	sourceAllowed, requiresSourceApproval := e.evaluateSourcePolicy(d, &decision)
	if !sourceAllowed {
		return decision, nil
	}
	if requiresSourceApproval {
		requireApproval(&decision, e.inputs.ApprovalAvailable)
	}
	if e.permissionMode() != PermissionModeApproveReads || !isInstanceSelfServiceCall(d, input) {
		e.applyPermissionCeiling(d, &decision)
	}
	return decision, nil
}

// applyRestrictions applies visibility, deny-list, session, and agent restrictions in order and
// reports whether one denied the tool.
func (e *EffectivePolicyEvaluator) applyRestrictions(d Descriptor, decision *EffectiveToolDecision) bool {
	if !isSessionVisible(d.Visibility) {
		denyDecision(decision, ReasonVisibilityDenied)
		decision.RegistryPolicyResult = policyResultDenied
		return true
	}
	if e.matchesAny(e.inputs.DenyTools, d.ID) {
		denyDecision(decision, ReasonPolicyDenied)
		decision.RegistryPolicyResult = policyResultDenied
		return true
	}
	if e.matchesAny(e.inputs.Agent.DenyTools, d.ID) {
		denyDecision(decision, ReasonPolicyDenied)
		decision.AgentPolicyResult = policyResultDenied
		return true
	}
	if e.inputs.Session.Enforced {
		if _, ok := e.sessionAllowed[d.ID]; !ok {
			denyDecision(decision, ReasonSessionDenied)
			decision.SessionPolicyResult = policyResultDenied
			return true
		}
		decision.SessionPolicyResult = policyResultAllowed
	}
	if e.agentPolicyRestricts() {
		if _, ok := e.agentAllowed[d.ID]; !ok && !e.matchesAny(e.inputs.Agent.Tools, d.ID) {
			denyDecision(decision, ReasonPolicyDenied)
			decision.AgentPolicyResult = policyResultDenied
			return true
		}
		decision.AgentPolicyResult = policyResultAllowed
	}
	return false
}

// evaluateIndexedDescriptor isolates custom evaluators from the registry's immutable descriptors.
func evaluateIndexedDescriptor(
	ctx context.Context,
	scope Scope,
	evaluator PolicyEvaluator,
	descriptor Descriptor,
) (EffectiveToolDecision, error) {
	if indexed, ok := evaluator.(indexedPolicyEvaluator); ok {
		return indexed.evaluateIndexed(ctx, scope, descriptor)
	}
	return evaluator.Evaluate(ctx, scope, cloneDescriptor(descriptor))
}

// Ownership is still checked by the native instance authority before mutation.
func isInstanceSelfServiceCall(d Descriptor, input json.RawMessage) bool {
	if d.Source.Kind != SourceBuiltin || d.Destructive || d.OpenWorld || d.RequiresInteraction {
		return false
	}
	switch d.ID {
	case ToolIDAgentInstancePause, ToolIDAgentInstanceEnsureMain:
		return true
	case ToolIDAgentInstanceChecks, ToolIDAgentInstanceCheck:
		var options struct {
			Enabled *bool `json:"enabled"`
			DryRun  bool  `json:"dry_run"`
		}
		if json.Unmarshal(input, &options) != nil {
			return false
		}
		if d.ID == ToolIDAgentInstanceChecks {
			return options.Enabled != nil && !*options.Enabled
		}
		return options.DryRun
	default:
		return false
	}
}
