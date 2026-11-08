package tools

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

func TestEffectivePolicyEvaluator(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	externalRead := mcpDescriptor("mcp__github__search", "github", "search")
	builtinWrite := validDescriptor()
	builtinWrite.ID = "compozy__task_update"
	builtinWrite.ReadOnly = false
	builtinWrite.Risk = RiskMutating

	t.Run("Should reject invalid descriptors supplied outside the registry index", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		evaluator, err := NewEffectivePolicyEvaluator(
			DefaultPolicyInputs(),
			ToolsetCatalog{},
			[]ToolID{descriptor.ID},
		)
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		descriptor.InputSchema = []byte("{")
		_, err = evaluator.Evaluate(t.Context(), Scope{}, descriptor)
		requireReason(t, err, ReasonSchemaInvalid)
	})

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

	t.Run("Should report policy denials beside the disabled source when tools are disabled", func(t *testing.T) {
		t.Parallel()

		denyPattern, err := ParseToolPattern("compozy__task_update")
		if err != nil {
			t.Fatalf("ParseToolPattern(deny) error = %v", err)
		}
		for _, tc := range []struct {
			name   string
			inputs PolicyInputs
			want   []ReasonCode
		}{
			{
				name:   "no other restriction",
				inputs: PolicyInputs{ToolsDisabled: true},
				want:   []ReasonCode{ReasonSourceDisabled},
			},
			{
				name:   "operator deny list",
				inputs: PolicyInputs{ToolsDisabled: true, DenyTools: []ToolPattern{denyPattern}},
				want:   []ReasonCode{ReasonSourceDisabled, ReasonPolicyDenied},
			},
			{
				name:   "agent deny list",
				inputs: PolicyInputs{ToolsDisabled: true, Agent: AgentToolPolicy{DenyTools: []ToolPattern{denyPattern}}},
				want:   []ReasonCode{ReasonSourceDisabled, ReasonPolicyDenied},
			},
			{
				name:   "enforced session allowlist",
				inputs: PolicyInputs{ToolsDisabled: true, Session: SessionToolPolicy{Enforced: true}},
				want:   []ReasonCode{ReasonSourceDisabled, ReasonSessionDenied},
			},
		} {
			t.Run("Should report "+tc.name, func(t *testing.T) {
				t.Parallel()

				evaluator, err := NewEffectivePolicyEvaluator(tc.inputs, ToolsetCatalog{}, []ToolID{builtinWrite.ID})
				if err != nil {
					t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
				}
				decision, err := evaluator.Evaluate(ctx, Scope{}, builtinWrite)
				if err != nil {
					t.Fatalf("Evaluate() error = %v", err)
				}
				if decision.Callable || decision.VisibleToSession {
					t.Fatalf("decision = %#v, want denied and hidden", decision)
				}
				if decision.RegistryPolicyResult != policyResultDisabled {
					t.Fatalf("RegistryPolicyResult = %q, want %q", decision.RegistryPolicyResult, policyResultDisabled)
				}
				if !slices.Equal(decision.ReasonCodes, tc.want) {
					t.Fatalf("ReasonCodes = %v, want %v", decision.ReasonCodes, tc.want)
				}
			})
		}
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

		allowPattern, err := ParseToolPattern("compozy__task_update")
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

	t.Run("Should not prompt for terminal input under approve reads", func(t *testing.T) {
		t.Parallel()

		terminalWrite := builtinWrite
		terminalWrite.ID = ToolIDTerminalWrite
		terminalWrite.Risk = RiskDestructive
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveReads,
			ApprovalAvailable:    true,
		}, ToolsetCatalog{}, []ToolID{terminalWrite.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, terminalWrite)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if !decision.Callable || decision.ApprovalRequired {
			t.Fatalf("decision = %#v, want direct terminal input", decision)
		}
	})

	t.Run("Should require approval for terminal input under deny all", func(t *testing.T) {
		t.Parallel()

		terminalWrite := builtinWrite
		terminalWrite.ID = ToolIDTerminalWrite
		terminalWrite.Risk = RiskDestructive
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeDenyAll,
			ApprovalAvailable:    true,
		}, ToolsetCatalog{}, []ToolID{terminalWrite.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, terminalWrite)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if !decision.Callable || !decision.ApprovalRequired {
			t.Fatalf("decision = %#v, want approval-gated terminal input", decision)
		}
		requireDecisionReason(t, decision, ReasonApprovalRequired)
	})

	// Invariant: extension runtime names do not replace logical names in session server grants.
	// Owner: effective tool policy; canonical suite: TestEffectivePolicyEvaluator.
	t.Run("Should match logical MCP grants while retaining runtime tool identity", func(t *testing.T) {
		t.Parallel()
		descriptor, err := mcpRegistryDescriptor(
			SourceRef{
				Kind:          SourceMCP,
				Owner:         "github.github",
				RawServerName: "github.github",
				ResourceID:    "extension-github",
			},
			MCPToolDescriptor{RawName: "search", ReadOnly: true, InputSchema: []byte(`{"type":"object"}`)},
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, grant := range []string{"github", "github.github", "other"} {
			evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				ExternalDefault:      ExternalDefaultEnabled,
				MCPResourceNames:     map[string]string{"extension-github": "github"},
				Session: SessionToolPolicy{
					Enforced:   true,
					Tools:      []ToolID{descriptor.ID},
					MCPServers: []string{grant},
				},
			}, ToolsetCatalog{}, []ToolID{descriptor.ID})
			if err != nil {
				t.Fatal(err)
			}
			decision, err := evaluator.Evaluate(ctx, Scope{}, descriptor)
			if err != nil || decision.Callable != (grant == "github") {
				t.Fatalf("grant %s for %s: %+v, %v", grant, descriptor.ID, decision, err)
			}
		}
	})

	// Invariant: a removed catalog resource cannot regain a server grant through its runtime name.
	// Owner: effective tool policy; canonical suite: TestEffectivePolicyEvaluator.
	t.Run("Should reject removed resource identities before runtime fallback", func(t *testing.T) {
		t.Parallel()
		for _, resourceID := range []string{"removed", ""} {
			descriptor := mcpDescriptor("mcp__github__search", "github", "search")
			descriptor.Source.ResourceID = resourceID
			evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				ExternalDefault:      ExternalDefaultEnabled,
				MCPResourceNames:     map[string]string{},
				Session: SessionToolPolicy{
					Enforced:   true,
					Tools:      []ToolID{descriptor.ID},
					MCPServers: []string{"github"},
				},
			}, ToolsetCatalog{}, []ToolID{descriptor.ID})
			if err != nil {
				t.Fatal(err)
			}
			decision, err := evaluator.Evaluate(ctx, Scope{}, descriptor)
			if err != nil || decision.Callable != (resourceID == "") {
				t.Fatalf("resource %q: %+v, %v", resourceID, decision, err)
			}
		}
	})

	t.Run("Should retain concrete MCP tool narrowing inside an allowed server", func(t *testing.T) {
		t.Parallel()
		for _, tool := range []string{"lookup", "delete"} {
			descriptor := validDescriptor()
			descriptor.ID = ToolID("mcp__docs__" + tool)
			descriptor.Source = SourceRef{Kind: SourceMCP, Owner: "docs", RawServerName: "docs", RawToolName: tool}
			evaluator, err := NewEffectivePolicyEvaluator(
				PolicyInputs{
					SystemPermissionMode: PermissionModeApproveAll,
					ExternalDefault:      ExternalDefaultEnabled,
					Session: SessionToolPolicy{
						Enforced:   true,
						Tools:      []ToolID{"mcp__docs__lookup"},
						MCPServers: []string{"docs"},
					},
				},
				ToolsetCatalog{},
				[]ToolID{descriptor.ID},
			)
			if err != nil {
				t.Fatal(err)
			}
			decision, err := evaluator.Evaluate(ctx, Scope{}, descriptor)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Callable != (tool == "lookup") {
				t.Fatalf("concrete narrowing for %s: %+v", tool, decision)
			}
		}
	})

	// Invariant: an explicit native-only override is exhaustive even when it inherits a server grant.
	// Owner: effective policy evaluator; canonical suite: TestEffectivePolicyEvaluator.
	t.Run("Should deny MCP tools under an explicit native-only tool override", func(t *testing.T) {
		t.Parallel()
		descriptor := validDescriptor()
		descriptor.ID = "mcp__docs__lookup"
		descriptor.Source = SourceRef{Kind: SourceMCP, Owner: "docs", RawServerName: "docs", RawToolName: "lookup"}
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll, ExternalDefault: ExternalDefaultEnabled,
			Session: SessionToolPolicy{Enforced: true, Tools: []ToolID{ToolIDSkillView}, MCPServers: []string{"docs"}},
		}, ToolsetCatalog{}, []ToolID{descriptor.ID, ToolIDSkillView})
		if err != nil {
			t.Fatal(err)
		}
		decision, err := evaluator.Evaluate(ctx, Scope{}, descriptor)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Callable {
			t.Fatalf("explicit tool ceiling widened: %+v", decision)
		}
	})

	t.Run("Should narrow MCP tools by the session server allowlist", func(t *testing.T) {
		t.Parallel()
		descriptor := validDescriptor()
		descriptor.ID = "mcp__docs__lookup"
		descriptor.Source = SourceRef{Kind: SourceMCP, Owner: "docs", RawServerName: "docs", RawToolName: "lookup"}
		for _, servers := range [][]string{nil, {"docs"}, {"other"}} {
			evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll, ExternalDefault: ExternalDefaultEnabled,
				Session: SessionToolPolicy{Enforced: true, MCPServers: servers, MCPByServer: true},
			}, ToolsetCatalog{}, []ToolID{descriptor.ID})
			if err != nil {
				t.Fatal(err)
			}
			decision, err := evaluator.Evaluate(ctx, Scope{}, descriptor)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Callable != (len(servers) > 0 && servers[0] == "docs") {
				t.Fatalf("servers %v: %+v", servers, decision)
			}
		}
	})

	t.Run("Should enforce session lineage concrete atoms independently", func(t *testing.T) {
		t.Parallel()

		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeApproveAll,
			Session: SessionToolPolicy{
				Enforced: true,
				Tools:    []ToolID{"compozy__skill_view"},
			},
		}, ToolsetCatalog{}, []ToolID{builtinWrite.ID, "compozy__skill_view"})
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
		descriptor.ID = "compozy__operator_debug"
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
		mutating.ID = "compozy__task_update"
		mutating.ReadOnly = false
		mutating.Risk = RiskMutating
		evaluator, err := NewEffectivePolicyEvaluator(DefaultPolicyInputs(), ToolsetCatalog{}, []ToolID{mutating.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(t.Context(), Scope{}, mutating)
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
		mutating.ID = "compozy__task_update"
		mutating.ReadOnly = false
		mutating.Risk = RiskMutating
		evaluator, err := NewEffectivePolicyEvaluator(PolicyInputs{
			SystemPermissionMode: PermissionModeDenyAll,
			ApprovalAvailable:    true,
		}, ToolsetCatalog{}, []ToolID{mutating.ID})
		if err != nil {
			t.Fatalf("NewEffectivePolicyEvaluator() error = %v", err)
		}
		decision, err := evaluator.Evaluate(t.Context(), Scope{}, mutating)
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

func TestAgentMemoryPermissionCeiling(t *testing.T) {
	for _, mode := range []PermissionMode{PermissionModeApproveReads, PermissionModeDenyAll} {
		t.Run("Should retain "+string(mode)+" authority for own memory writes", func(t *testing.T) {
			t.Parallel()
			for _, id := range []ToolID{ToolIDAgentMemoryAdd, ToolIDAgentMemoryReplace, ToolIDAgentMemoryRemove} {
				descriptor := validDescriptor()
				descriptor.ID = id
				descriptor.ReadOnly = false
				descriptor.Risk = RiskMutating
				inputs := DefaultPolicyInputs()
				inputs.SystemPermissionMode = mode
				evaluator, err := NewEffectivePolicyEvaluator(inputs, ToolsetCatalog{}, []ToolID{id})
				if err != nil {
					t.Fatal(err)
				}
				decision, err := evaluator.Evaluate(t.Context(), Scope{}, descriptor)
				if err != nil {
					t.Fatal(err)
				}
				if mode == PermissionModeApproveReads && (!decision.Callable || decision.ApprovalRequired) {
					t.Fatalf("unexpected approval: %+v", decision)
				}
				if mode == PermissionModeDenyAll && !decision.ApprovalRequired {
					t.Fatalf("deny-all bypassed: %+v", decision)
				}
			}
		})
	}
}

func TestTriageToolPolicy(t *testing.T) {
	t.Run("Should deny mutations external tools and terminal reads without approvals", func(t *testing.T) {
		t.Parallel()
		read := validDescriptor()
		write := read
		write.ID = "compozy__task_update"
		write.ReadOnly = false
		write.Risk = RiskMutating
		message := write
		message.ID = ToolIDSessionPrompt
		terminal := read
		terminal.ID = "compozy__terminal_read"
		external := mcpDescriptor("mcp__github__search", "github", "search")
		inputs := DefaultPolicyInputs()
		inputs.CheckIn = &CheckInPolicy{Disabled: true}
		inputs.SystemPermissionMode = PermissionModeApproveAll
		inputs.ApprovalAvailable = true
		evaluator, err := NewEffectivePolicyEvaluator(
			inputs,
			ToolsetCatalog{},
			[]ToolID{read.ID, write.ID, message.ID, terminal.ID, external.ID},
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, descriptor := range []Descriptor{write, message, terminal, external} {
			decision, err := evaluator.Evaluate(t.Context(), Scope{}, descriptor)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Callable || decision.ApprovalRequired {
				t.Fatalf("unsafe decision %s: %+v", descriptor.ID, decision)
			}
		}
		decision, err := evaluator.Evaluate(t.Context(), Scope{}, read)
		if err != nil {
			t.Fatal(err)
		}
		if !decision.Callable {
			t.Fatalf("read denied: %+v", decision)
		}
	})
}

func TestCoordinationPermissionLevels(t *testing.T) {
	t.Parallel()
	for _, mode := range []PermissionMode{PermissionModeApproveEdits, PermissionModeAuto} {
		t.Run("Should restrict coordination approvals at "+string(mode), func(t *testing.T) {
			t.Parallel()
			for _, lineage := range []bool{false, true} {
				for _, id := range []ToolID{ToolIDMonitorCreate, ToolIDMonitorCancel, ToolIDSubagentDelegate,
					ToolIDSubagentCancel, ToolIDSessionSpawn, ToolIDSessionPrompt, ToolIDGoalControl,
					ToolIDNotify, "compozy__agent_skill_propose", ToolIDSessionApprove,
					ToolIDSessionClarifyAnswer, ToolIDTerminalExec} {
					d := validDescriptor()
					d.ID = id
					d.ReadOnly = false
					d.Risk = RiskMutating
					e, err := NewEffectivePolicyEvaluator(PolicyInputs{
						SystemPermissionMode: mode, AlwaysOnLineage: lineage, ApprovalAvailable: true,
					}, ToolsetCatalog{}, []ToolID{id})
					if err != nil {
						t.Fatal(err)
					}
					got, err := e.Evaluate(t.Context(), Scope{}, d)
					if err != nil {
						t.Fatal(err)
					}
					wantApproval := id == ToolIDTerminalExec ||
						(!lineage && (id == ToolIDSessionApprove || id == ToolIDSessionClarifyAnswer))
					if !got.Callable || got.ApprovalRequired != wantApproval {
						t.Fatalf("mode=%s lineage=%v id=%s decision=%+v", mode, lineage, id, got)
					}
				}
			}
		})
		t.Run("Should decide builtin tools by declared risk at "+string(mode), func(t *testing.T) {
			t.Parallel()
			mutating := func(id ToolID) Descriptor {
				d := validDescriptor()
				d.ID, d.ReadOnly, d.Risk = id, false, RiskMutating
				return d
			}
			destructive := mutating("compozy__worktree_remove")
			destructive.Risk, destructive.Destructive = RiskDestructive, true
			openWorld := mutating("compozy__provider_usage_refresh")
			openWorld.OpenWorld = true
			external := mcpDescriptor("mcp__github__create_issue", "github", "create_issue")
			external.ReadOnly, external.Risk = false, RiskMutating
			cases := []struct {
				descriptor   Descriptor
				wantApproval bool
			}{
				{mutating("compozy__session_create"), false},
				{mutating("compozy__worktree_create"), false},
				{mutating("compozy__session_worktree_bind"), false},
				{mutating("compozy__session_rename"), false},
				{mutating("compozy__session_link"), false},
				{mutating("compozy__task_update"), false},
				{destructive, true},
				{openWorld, true},
				{external, true},
				{mutating(ToolIDTerminalWrite), true},
				{mutating(ToolIDConfigSet), true},
				{mutating("compozy__hooks_create"), true},
				{mutating("compozy__extensions_install"), true},
				{mutating(ToolIDAgentCreate), true},
				{mutating(ToolIDSessionReparent), true},
				{mutating(ToolIDSessionArchive), true},
				{mutating(ToolIDLoopApprove), true},
				{mutating("compozy__loop_create"), true},
				{mutating("compozy__loop_run"), true},
				{mutating("compozy__automation_jobs_create"), true},
				{mutating(ToolIDTaskExecutionProfileSet), true},
			}
			for _, tc := range cases {
				e, err := NewEffectivePolicyEvaluator(PolicyInputs{
					SystemPermissionMode: mode, AlwaysOnLineage: true, ApprovalAvailable: true,
					ExternalDefault: ExternalDefaultEnabled,
				}, ToolsetCatalog{}, []ToolID{tc.descriptor.ID})
				if err != nil {
					t.Fatal(err)
				}
				got, err := e.Evaluate(t.Context(), Scope{}, tc.descriptor)
				if err != nil {
					t.Fatal(err)
				}
				if !got.Callable || got.ApprovalRequired != tc.wantApproval {
					t.Fatalf("mode=%s id=%s decision=%+v", mode, tc.descriptor.ID, got)
				}
			}
		})
	}
}

func TestCheckInPolicy(t *testing.T) {
	t.Run("Should classify access and bound opt-in writes without approval [UT-001 UT-003 UT-004]", func(t *testing.T) {
		t.Parallel()
		read := validDescriptor()
		if AccessOf(read) != AccessRead {
			t.Fatal("read descriptor classified write")
		}
		for _, mutate := range []func(*Descriptor){func(d *Descriptor) { d.Destructive = true }, func(d *Descriptor) { d.OpenWorld = true }, func(d *Descriptor) { d.RequiresInteraction = true }, func(d *Descriptor) { d.Risk = RiskMutating }, func(d *Descriptor) { d.ReadOnly = false }} {
			d := read
			mutate(&d)
			if AccessOf(d) != AccessWrite {
				t.Fatalf("unsafe read: %+v", d)
			}
		}
		write := mcpDescriptor("mcp__linear__comment", "linear", "comment")
		write.ReadOnly = false
		write.OpenWorld = true
		write.Risk = RiskOpenWorld
		native := read
		native.ID = ToolIDSessionPrompt
		native.ReadOnly = false
		native.Risk = RiskMutating
		for _, tc := range []struct {
			name       string
			d          Descriptor
			selectors  []string
			unattended bool
			reason     ReasonCode
		}{
			{"Should allow a read", read, nil, false, ""},
			{"Should deny an unlisted write", write, nil, true, "check_in_write_not_allowed"},
			{"Should allow an exact write", write, []string{string(write.ID)}, true, ""},
			{"Should allow a server selector", write, []string{"mcp__linear__*"}, true, ""},
			{"Should reject a different server", write, []string{"mcp__linear_extra__*"}, true, "check_in_write_not_allowed"},
			{"Should refuse approvals", write, []string{string(write.ID)}, false, "check_in_write_needs_approval"},
			{"Should deny native coordination", native, []string{string(native.ID)}, true, "check_in_tool_not_writable"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				p := CheckInPolicy{
					AllowWrites:    tc.selectors,
					RunsUnattended: func(context.Context, Descriptor, json.RawMessage) bool { return tc.unattended },
				}
				if reason := p.denial(tc.d, nil); reason != tc.reason {
					t.Fatalf("reason=%s want=%s", reason, tc.reason)
				}
			})
		}
	})
}
