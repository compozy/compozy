package tools

import (
	"context"
	"slices"
	"testing"
)

func TestEffectivePolicyEvaluator(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	externalRead := mcpDescriptor("mcp__github__search", "github", "search")
	builtinWrite := validDescriptor()
	builtinWrite.ID = "agh__task_update"
	builtinWrite.ReadOnly = false
	builtinWrite.Risk = RiskMutating

	t.Run("Should let explicit denies override allows toolsets trusted sources and approve all", func(t *testing.T) {
		t.Parallel()

		allowPattern, err := ParseToolPattern("mcp__github__*")
		if err != nil {
			t.Fatalf("ParseToolPattern(allow) error = %v", err)
		}
		denyPattern, err := ParseToolPattern("mcp__github__search")
		if err != nil {
			t.Fatalf("ParseToolPattern(deny) error = %v", err)
		}
		catalog, err := NewToolsetCatalog(Toolset{ID: "mcp__github_read", Tools: []string{"mcp__github__*"}})
		if err != nil {
			t.Fatalf("NewToolsetCatalog() error = %v", err)
		}
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			TrustedSources: []SourceGrant{
				{Kind: SourceMCP, Owner: "github"},
			},
			AllowTools:    []ToolPattern{allowPattern},
			AllowToolsets: []ToolsetID{"mcp__github_read"},
			DenyTools:     []ToolPattern{denyPattern},
		}, catalog, []ToolID{externalRead.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, externalRead)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if decision.Callable || decision.VisibleToSession {
			t.Fatalf("decision = %#v, want denied and hidden", decision)
		}
		if decision.RegistryPolicyResult != policyResultDenied {
			t.Fatalf("RegistryPolicyResult = %q, want %q", decision.RegistryPolicyResult, policyResultDenied)
		}
		requireDecisionReason(t, decision, ReasonPolicyDenied)
	})

	t.Run("Should not approve untrusted external read only tools with approve reads", func(t *testing.T) {
		t.Parallel()

		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveReads,
			ExternalDefault:      ExternalDefaultDisabled,
		}, ToolsetCatalog{}, []ToolID{externalRead.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, externalRead)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if decision.Callable {
			t.Fatalf("decision.Callable = true, want false: %#v", decision)
		}
		if decision.SourcePolicyResult != policyResultDenied {
			t.Fatalf("SourcePolicyResult = %q, want %q", decision.SourcePolicyResult, policyResultDenied)
		}
		requireDecisionReason(t, decision, ReasonSourceDisabled)
	})

	t.Run("Should allow trusted external read only sources under approve reads", func(t *testing.T) {
		t.Parallel()

		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveReads,
			ExternalDefault:      ExternalDefaultDisabled,
			TrustedSources:       []SourceGrant{{Kind: SourceMCP, Owner: "github"}},
		}, ToolsetCatalog{}, []ToolID{externalRead.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, externalRead)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if !decision.Callable || !decision.VisibleToSession {
			t.Fatalf("decision = %#v, want callable trusted read-only source", decision)
		}
		if decision.SourcePolicyResult != policyResultTrusted {
			t.Fatalf("SourcePolicyResult = %q, want %q", decision.SourcePolicyResult, policyResultTrusted)
		}
	})

	t.Run("Should apply ACP permission mode as ceiling above registry grants", func(t *testing.T) {
		t.Parallel()

		allowPattern, err := ParseToolPattern("agh__task_update")
		if err != nil {
			t.Fatalf("ParseToolPattern() error = %v", err)
		}
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveReads,
			AllowTools:           []ToolPattern{allowPattern},
		}, ToolsetCatalog{}, []ToolID{builtinWrite.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, builtinWrite)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if decision.Callable || !decision.ApprovalRequired {
			t.Fatalf("decision = %#v, want approval-required denial without approval channel", decision)
		}
		requireDecisionReason(t, decision, ReasonApprovalRequired)
		requireDecisionReason(t, decision, ReasonApprovalUnreachable)
	})

	t.Run("Should enforce session lineage concrete atoms independently", func(t *testing.T) {
		t.Parallel()

		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			Session: SessionToolPolicy{
				Enforced: true,
				Tools:    []ToolID{"agh__skill_view"},
			},
		}, ToolsetCatalog{}, []ToolID{builtinWrite.ID, "agh__skill_view"})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, builtinWrite)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if decision.Callable {
			t.Fatalf("decision.Callable = true, want session denial")
		}
		if decision.SessionPolicyResult != policyResultDenied {
			t.Fatalf("SessionPolicyResult = %q, want %q", decision.SessionPolicyResult, policyResultDenied)
		}
		requireDecisionReason(t, decision, ReasonSessionDenied)
	})

	t.Run("Should hide operator only descriptors from session projections", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		descriptor.ID = "agh__operator_debug"
		descriptor.Visibility = VisibilityOperator
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
		}, ToolsetCatalog{}, []ToolID{descriptor.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, descriptor)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if decision.Callable || decision.VisibleToSession {
			t.Fatalf("decision = %#v, want hidden from session", decision)
		}
		requireDecisionReason(t, decision, ReasonVisibilityDenied)
	})
}

func TestPolicyParsingValidationAndApprovalBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should default to ACP approve reads ceiling", func(t *testing.T) {
		t.Parallel()

		mutating := validDescriptor()
		mutating.ID = "agh__task_update"
		mutating.ReadOnly = false
		mutating.Risk = RiskMutating
		evaluator, err := NewEffectivePolicyEvaluator(DefaultPolicyInputs(), ToolsetCatalog{}, []ToolID{mutating.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(context.Background(), Scope{}, mutating)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if decision.SystemPermissionMode != string(PermissionModeApproveReads) {
			t.Fatalf("SystemPermissionMode = %q, want approve-reads", decision.SystemPermissionMode)
		}
		if decision.Callable || !decision.ApprovalRequired {
			t.Fatalf("decision = %#v, want default approve-reads ceiling to require approval", decision)
		}
	})

	t.Run("Should parse source grants and reject unsupported grant kinds", func(t *testing.T) {
		t.Parallel()

		grant, err := ParseSourceGrant("mcp:github")
		if err != nil {
			t.Fatalf("ParseSourceGrant() error = %v", err)
		}
		if !grant.Match(SourceRef{Kind: SourceMCP, Owner: "github"}) {
			t.Fatalf("SourceGrant.Match() = false, want true")
		}
		requireReason(t, SourceGrant{Kind: SourceBuiltin, Owner: "daemon"}.Validate("grant"), ReasonSourceDisabled)
		_, err = ParseSourceGrant("builtin:daemon")
		requireReason(t, err, ReasonSourceDisabled)
		_, err = ParseSourceGrant("mcp")
		requireReason(t, err, ReasonSourceDisabled)
	})

	t.Run("Should keep approval-required tools callable when approval channel exists", func(t *testing.T) {
		t.Parallel()

		mutating := validDescriptor()
		mutating.ID = "agh__task_update"
		mutating.ReadOnly = false
		mutating.Risk = RiskMutating
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeDenyAll,
			ApprovalAvailable:    true,
		}, ToolsetCatalog{}, []ToolID{mutating.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(context.Background(), Scope{}, mutating)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if !decision.Callable || !decision.ApprovalRequired {
			t.Fatalf("decision = %#v, want callable with approval required", decision)
		}
	})

	t.Run("Should reject invalid policy modes and external defaults", func(t *testing.T) {
		t.Parallel()

		_, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionMode("maybe"),
		}, ToolsetCatalog{}, nil)
		requireReason(t, err, ReasonPolicyDenied)
		_, err = NewEffectivePolicyEvaluator(PolicyInputs{
			ExternalDefault: ExternalDefault("maybe"),
		}, ToolsetCatalog{}, nil)
		requireReason(t, err, ReasonSourceDisabled)
	})
}

func requireDecisionReason(t *testing.T, decision EffectiveToolDecision, want ReasonCode) {
	t.Helper()

	if slices.Contains(decision.ReasonCodes, want) {
		return
	}
	t.Fatalf("decision reasons = %#v, want %q", decision.ReasonCodes, want)
}
