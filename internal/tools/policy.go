package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	policyResultAllowed          = "allowed"
	policyResultDenied           = "denied"
	policyResultApprovalRequired = "approval_required"
	policyResultDisabled         = "disabled"
	policyResultTrusted          = "trusted"
	policyResultUnrestricted     = "unrestricted"
	policyResultUnavailable      = "unavailable"
	policyResultConflicted       = "conflicted"
)

// PermissionMode mirrors ACP's static approval modes without importing config.
type PermissionMode string

const (
	// PermissionModeDenyAll requires explicit approval for every tool call.
	PermissionModeDenyAll PermissionMode = "deny-all"
	// PermissionModeApproveReads auto-approves read-only tools.
	PermissionModeApproveReads PermissionMode = "approve-reads"
	// PermissionModeApproveAll auto-approves otherwise allowed tools.
	PermissionModeApproveAll PermissionMode = "approve-all"
)

// ExternalDefault controls default policy for external executable sources.
type ExternalDefault string

const (
	// ExternalDefaultDisabled denies external sources unless explicitly granted.
	ExternalDefaultDisabled ExternalDefault = "disabled"
	// ExternalDefaultAsk allows external sources but requires approval.
	ExternalDefaultAsk ExternalDefault = "ask"
	// ExternalDefaultEnabled allows external sources subject to the ACP ceiling.
	ExternalDefaultEnabled ExternalDefault = "enabled"
)

// SourceGrant grants policy to one descriptor source owner.
type SourceGrant struct {
	Kind  SourceKind `json:"kind"`
	Owner string     `json:"owner"`
}

// ParseSourceGrant parses kind:owner source policy entries.
func ParseSourceGrant(raw string) (SourceGrant, error) {
	trimmed := strings.TrimSpace(raw)
	kindText, owner, ok := strings.Cut(trimmed, ":")
	if trimmed == "" || !ok || owner == "" || strings.Contains(owner, ":") {
		return SourceGrant{}, NewValidationError(
			"source_grant",
			ReasonSourceDisabled,
			"source grant must use kind:owner",
		)
	}
	grant := SourceGrant{Kind: SourceKind(kindText), Owner: owner}
	if err := grant.Validate("source_grant"); err != nil {
		return SourceGrant{}, err
	}
	return grant, nil
}

// Validate ensures the grant can match a source deterministically.
func (g SourceGrant) Validate(field string) error {
	switch g.Kind {
	case SourceMCP, SourceExtension:
	default:
		return NewValidationError(field+".kind", ReasonSourceDisabled, "source grant must target mcp or extension")
	}
	if strings.TrimSpace(g.Owner) == "" {
		return NewValidationError(field+".owner", ReasonSourceDisabled, "source grant owner is required")
	}
	return nil
}

// Match reports whether the grant covers the descriptor source.
func (g SourceGrant) Match(source SourceRef) bool {
	return g.Kind == source.Kind && g.Owner == source.Owner
}

// AgentToolPolicy captures agent-local allow and deny grammar.
type AgentToolPolicy struct {
	Tools     []ToolPattern `json:"-"`
	Toolsets  []ToolsetID   `json:"toolsets,omitempty"`
	DenyTools []ToolPattern `json:"-"`
}

// SessionToolPolicy captures concrete resolved lineage atoms.
type SessionToolPolicy struct {
	Enforced bool     `json:"enforced"`
	Tools    []ToolID `json:"tools,omitempty"`
}

// PolicyInputs contains the config-neutral inputs for effective policy.
type PolicyInputs struct {
	ToolsDisabled        bool
	SystemPermissionMode PermissionMode
	ExternalDefault      ExternalDefault
	ApprovalAvailable    bool
	TrustedSources       []SourceGrant
	AllowSources         []SourceGrant
	AllowTools           []ToolPattern
	AllowToolsets        []ToolsetID
	DenyTools            []ToolPattern
	Agent                AgentToolPolicy
	Session              SessionToolPolicy
}

// DefaultPolicyInputs returns conservative registry defaults.
func DefaultPolicyInputs() PolicyInputs {
	return PolicyInputs{
		SystemPermissionMode: PermissionModeApproveReads,
		ExternalDefault:      ExternalDefaultDisabled,
	}
}

// EffectivePolicyEvaluator computes policy below the ACP permission ceiling.
type EffectivePolicyEvaluator struct {
	inputs          PolicyInputs
	registryAllowed map[ToolID]struct{}
	agentAllowed    map[ToolID]struct{}
	sessionAllowed  map[ToolID]struct{}
}

var _ PolicyEvaluator = (*EffectivePolicyEvaluator)(nil)

// NewEffectivePolicyEvaluator validates and prepares a policy evaluator.
func NewEffectivePolicyEvaluator(
	inputs PolicyInputs,
	toolsets ToolsetCatalog,
	universe []ToolID,
) (*EffectivePolicyEvaluator, error) {
	normalizedUniverse := normalizeToolUniverse(universe)
	if err := validatePolicyInputs(inputs); err != nil {
		return nil, err
	}
	registryAllowed, err := expandOptionalPolicyAtoms(
		toolsets,
		inputs.AllowTools,
		inputs.AllowToolsets,
		normalizedUniverse,
	)
	if err != nil {
		return nil, err
	}
	agentAllowed, err := expandOptionalPolicyAtoms(
		toolsets,
		inputs.Agent.Tools,
		inputs.Agent.Toolsets,
		normalizedUniverse,
	)
	if err != nil {
		return nil, err
	}
	sessionAllowed, err := normalizeSessionPolicyAtoms(inputs.Session.Tools)
	if err != nil {
		return nil, err
	}

	return &EffectivePolicyEvaluator{
		inputs:          inputs,
		registryAllowed: registryAllowed,
		agentAllowed:    agentAllowed,
		sessionAllowed:  sessionAllowed,
	}, nil
}

// Evaluate computes the effective decision for one descriptor.
func (e *EffectivePolicyEvaluator) Evaluate(_ context.Context, _ Scope, d Descriptor) (EffectiveToolDecision, error) {
	if err := d.Validate(); err != nil {
		return EffectiveToolDecision{}, err
	}
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

func validatePolicyInputs(inputs PolicyInputs) error {
	switch inputs.SystemPermissionMode {
	case "", PermissionModeDenyAll, PermissionModeApproveReads, PermissionModeApproveAll:
	default:
		return NewValidationError("system_permission_mode", ReasonPolicyDenied, "unsupported permission mode")
	}
	switch inputs.ExternalDefault {
	case "", ExternalDefaultDisabled, ExternalDefaultAsk, ExternalDefaultEnabled:
	default:
		return NewValidationError("external_default", ReasonSourceDisabled, "unsupported external default")
	}
	grants := [][]SourceGrant{inputs.TrustedSources, inputs.AllowSources}
	for groupIndex, group := range grants {
		for i, grant := range group {
			if err := grant.Validate(fmt.Sprintf("source_grants[%d][%d]", groupIndex, i)); err != nil {
				return err
			}
		}
	}
	for i, id := range append(append([]ToolsetID(nil), inputs.AllowToolsets...), inputs.Agent.Toolsets...) {
		if err := id.Validate(); err != nil {
			return wrapField(err, fmt.Sprintf("toolsets[%d]", i))
		}
	}
	return nil
}

func expandOptionalPolicyAtoms(
	toolsets ToolsetCatalog,
	patterns []ToolPattern,
	toolsetIDs []ToolsetID,
	universe []ToolID,
) (map[ToolID]struct{}, error) {
	expanded := make(map[ToolID]struct{})
	for _, pattern := range patterns {
		if id, ok := pattern.exactID(); ok {
			expanded[id] = struct{}{}
			continue
		}
		for _, id := range universe {
			if pattern.Match(id) {
				expanded[id] = struct{}{}
			}
		}
	}
	if len(toolsetIDs) == 0 {
		return expanded, nil
	}
	ids, err := toolsets.ExpandPatterns(nil, toolsetIDs, universe)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		expanded[id] = struct{}{}
	}
	return expanded, nil
}

func normalizeSessionPolicyAtoms(ids []ToolID) (map[ToolID]struct{}, error) {
	allowed := make(map[ToolID]struct{}, len(ids))
	for i, id := range ids {
		if err := id.Validate(); err != nil {
			return nil, wrapField(err, fmt.Sprintf("session.tools[%d]", i))
		}
		allowed[id] = struct{}{}
	}
	return allowed, nil
}

func (e *EffectivePolicyEvaluator) permissionMode() PermissionMode {
	if e.inputs.SystemPermissionMode == "" {
		return PermissionModeApproveReads
	}
	return e.inputs.SystemPermissionMode
}

func (e *EffectivePolicyEvaluator) externalDefault() ExternalDefault {
	if e.inputs.ExternalDefault == "" {
		return ExternalDefaultDisabled
	}
	return e.inputs.ExternalDefault
}

func (e *EffectivePolicyEvaluator) agentPolicyRestricts() bool {
	return len(e.inputs.Agent.Tools) > 0 || len(e.inputs.Agent.Toolsets) > 0
}

func (e *EffectivePolicyEvaluator) matchesAny(patterns []ToolPattern, id ToolID) bool {
	for _, pattern := range patterns {
		if pattern.Match(id) {
			return true
		}
	}
	return false
}

func (e *EffectivePolicyEvaluator) evaluateSourcePolicy(
	d Descriptor,
	decision *EffectiveToolDecision,
) (bool, bool) {
	if d.Source.Kind == SourceBuiltin {
		return true, false
	}
	if d.Source.Kind == SourceDynamic {
		denyDecision(decision, ReasonSourceDisabled)
		decision.SourcePolicyResult = policyResultDenied
		return false, false
	}
	if e.hasExplicitSourceGrant(d) || e.hasExplicitToolGrant(d.ID) {
		decision.SourcePolicyResult = policyResultAllowed
		return true, false
	}
	if e.isTrustedReadOnlySource(d) {
		decision.SourcePolicyResult = policyResultTrusted
		return true, false
	}

	switch e.externalDefault() {
	case ExternalDefaultEnabled:
		decision.SourcePolicyResult = policyResultAllowed
		return true, false
	case ExternalDefaultAsk:
		decision.SourcePolicyResult = policyResultApprovalRequired
		return true, true
	default:
		denyDecision(decision, ReasonSourceDisabled)
		decision.SourcePolicyResult = policyResultDenied
		return false, false
	}
}

func (e *EffectivePolicyEvaluator) hasExplicitSourceGrant(d Descriptor) bool {
	for _, grant := range e.inputs.AllowSources {
		if grant.Match(d.Source) {
			return true
		}
	}
	return false
}

func (e *EffectivePolicyEvaluator) hasExplicitToolGrant(id ToolID) bool {
	if _, ok := e.registryAllowed[id]; ok {
		return true
	}
	if _, ok := e.agentAllowed[id]; ok {
		return true
	}
	return false
}

func (e *EffectivePolicyEvaluator) isTrustedReadOnlySource(d Descriptor) bool {
	if !isReadOnlyAutoApprovable(d) {
		return false
	}
	for _, grant := range e.inputs.TrustedSources {
		if grant.Match(d.Source) {
			return true
		}
	}
	return false
}

func (e *EffectivePolicyEvaluator) applyPermissionCeiling(d Descriptor, decision *EffectiveToolDecision) {
	switch e.permissionMode() {
	case PermissionModeApproveAll:
		return
	case PermissionModeApproveReads:
		if isReadOnlyAutoApprovable(d) {
			return
		}
		requireApproval(decision, e.inputs.ApprovalAvailable)
	case PermissionModeDenyAll:
		requireApproval(decision, e.inputs.ApprovalAvailable)
	default:
		denyDecision(decision, ReasonPolicyDenied)
	}
}

func isReadOnlyAutoApprovable(d Descriptor) bool {
	return d.ReadOnly && !d.Destructive && !d.OpenWorld && !d.RequiresInteraction && d.Risk == RiskRead
}

func isSessionVisible(visibility Visibility) bool {
	return visibility == VisibilitySession || visibility == VisibilityModel
}

func denyDecision(decision *EffectiveToolDecision, reason ReasonCode) {
	decision.VisibleToSession = false
	decision.Callable = false
	appendDecisionReason(decision, reason)
}

func requireApproval(decision *EffectiveToolDecision, approvalAvailable bool) {
	decision.ApprovalRequired = true
	appendDecisionReason(decision, ReasonApprovalRequired)
	if approvalAvailable {
		return
	}
	decision.VisibleToSession = false
	decision.Callable = false
	appendDecisionReason(decision, ReasonApprovalUnreachable)
}

func applyAvailabilityDecision(decision EffectiveToolDecision, availability Availability) EffectiveToolDecision {
	if availability.Conflicted {
		decision.AvailabilityResult = policyResultConflicted
		decision.VisibleToSession = false
		decision.Callable = false
		appendDecisionReasons(&decision, availability.ReasonCodes)
		return decision
	}
	if availability.Executable {
		decision.AvailabilityResult = policyResultAllowed
		return decision
	}

	decision.AvailabilityResult = policyResultUnavailable
	decision.VisibleToSession = false
	decision.Callable = false
	if len(availability.ReasonCodes) > 0 {
		appendDecisionReasons(&decision, availability.ReasonCodes)
		return decision
	}
	switch {
	case !availability.Enabled:
		appendDecisionReason(&decision, ReasonSourceDisabled)
	case !availability.Available:
		appendDecisionReason(&decision, ReasonBackendUnhealthy)
	case !availability.Authorized:
		appendDecisionReason(&decision, ReasonPolicyDenied)
	default:
		appendDecisionReason(&decision, ReasonBackendNotExecutable)
	}
	return decision
}

func appendDecisionReasons(decision *EffectiveToolDecision, reasons []ReasonCode) {
	for _, reason := range reasons {
		appendDecisionReason(decision, reason)
	}
}

func appendDecisionReason(decision *EffectiveToolDecision, reason ReasonCode) {
	if reason == "" || slices.Contains(decision.ReasonCodes, reason) {
		return
	}
	decision.ReasonCodes = append(decision.ReasonCodes, reason)
}
