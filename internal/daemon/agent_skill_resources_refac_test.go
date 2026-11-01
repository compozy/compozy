package daemon

import (
	"context"
	"errors"
	"reflect"
	"testing"

	mcppkg "github.com/compozy/compozy/internal/mcp"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestResourceAgentCatalogLookupReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	t.Run("Should keep catalog records immutable after resolve mutation", func(t *testing.T) {
		t.Parallel()

		workspaceID := "ws-refac"
		catalog := newResourceCatalog(cloneAgentDef)
		catalog.Replace(1, []resources.Record[compozyconfig.AgentDef]{{
			ID:      "workspace:coder",
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspaceID},
			Version: 1,
			Spec: compozyconfig.AgentDef{
				Name:   "coder",
				Prompt: "workspace prompt",
				Tools:  []string{toolspkg.ToolIDToolInfo.String()},
			},
		}})
		resolved := &workspacepkg.ResolvedWorkspace{ID: workspaceID}
		dependency := agentCatalogDependency(catalog)

		first, err := dependency.ResolveAgent("coder", resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(first) error = %v", err)
		}
		first.Tools[0] = "corrupted"

		second, err := dependency.ResolveAgent("coder", resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(second) error = %v", err)
		}
		if got, want := second.Tools[0], toolspkg.ToolIDToolInfo.String(); got != want {
			t.Fatalf("ResolveAgent(second).Tools[0] = %q, want %q", got, want)
		}
		policyContext := &workspacepkg.ResolvedAgentConfig{Workspace: resolved.Workspace}
		policyAgent, err := dependency.ResolvePolicyAgent("coder", policyContext)
		if err != nil {
			t.Fatalf("ResolvePolicyAgent() error = %v", err)
		}
		policyAgent.Tools[0] = "corrupted"
		updated := catalog.Snapshot()
		if got, want := updated[0].Spec.Tools[0], toolspkg.ToolIDToolInfo.String(); got != want {
			t.Fatalf("catalog tool after policy result mutation = %q, want %q", got, want)
		}
		updated[0].Spec.Tools = []string{toolspkg.ToolIDSkillView.String()}
		catalog.Replace(2, updated)
		policyAgent, err = dependency.ResolvePolicyAgent("coder", policyContext)
		if err != nil {
			t.Fatalf("ResolvePolicyAgent(updated) error = %v", err)
		}
		if got, want := policyAgent.Tools[0], toolspkg.ToolIDSkillView.String(); got != want {
			t.Fatalf("ResolvePolicyAgent(updated).Tools[0] = %q, want %q", got, want)
		}
	})
}

func TestResourceAgentCatalogPolicyResolution(t *testing.T) {
	t.Parallel()

	catalog := newResourceCatalog(cloneAgentDef)
	catalog.Replace(1, []resources.Record[compozyconfig.AgentDef]{
		{ID: "user:coder:a", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			Spec: compozyconfig.AgentDef{Name: "coder", Prompt: "older user"}},
		{ID: "user:coder:z", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			Spec: compozyconfig.AgentDef{Name: "coder", Prompt: "user"}},
		{ID: "profile:coder", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: "pf-custom"},
			Spec: compozyconfig.AgentDef{Name: "coder", Prompt: "profile"}},
		{ID: "workspace:coder", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-a"},
			Spec: compozyconfig.AgentDef{Name: "coder", Prompt: "workspace"}},
		{ID: "workspace-profile:coder", Scope: resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspaceProfile, ID: "ws-a@pf:custom",
		}, Spec: compozyconfig.AgentDef{Name: "coder", Prompt: "workspace-profile"}},
	})
	dependency := agentCatalogDependency(catalog)
	for _, testCase := range []struct {
		name        string
		workspaceID string
		profileID   string
		profileName string
		wantPrompt  string
	}{
		{name: "Should preserve global record tie breaking", wantPrompt: "user"},
		{name: "Should preserve profile precedence", profileID: "pf-custom", profileName: "custom", wantPrompt: "profile"},
		{name: "Should preserve workspace precedence", workspaceID: "ws-a", wantPrompt: "workspace"},
		{name: "Should preserve workspace profile precedence", workspaceID: "ws-a", profileID: "pf-custom",
			profileName: "custom", wantPrompt: "workspace-profile"},
		{name: "Should exclude records from a different workspace", workspaceID: "ws-b", profileID: "pf-custom",
			profileName: "custom", wantPrompt: "profile"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			full := &workspacepkg.ResolvedWorkspace{
				ID:        testCase.workspaceID,
				ProfileID: testCase.profileID, ProfileName: testCase.profileName,
				Agents: []compozyconfig.AgentDef{{Name: "coder", Prompt: "snapshot"}},
			}
			policy := &workspacepkg.ResolvedAgentConfig{
				Workspace:   full.Workspace,
				ProfileID:   full.ProfileID,
				ProfileName: full.ProfileName,
				Agents:      full.Agents,
			}
			want, err := dependency.ResolveAgent(" coder ", full)
			if err != nil {
				t.Fatalf("ResolveAgent() error = %v", err)
			}
			got, err := dependency.ResolvePolicyAgent(" coder ", policy)
			if err != nil {
				t.Fatalf("ResolvePolicyAgent() error = %v", err)
			}
			if got.Prompt != testCase.wantPrompt || !reflect.DeepEqual(got, want) {
				t.Fatalf(
					"ResolvePolicyAgent() = %#v, want full resolution %#v with prompt %q",
					got,
					want,
					testCase.wantPrompt,
				)
			}
		})
	}

	t.Run("Should preserve snapshot and builtin fallback with exact error semantics", func(t *testing.T) {
		t.Parallel()

		full := &workspacepkg.ResolvedWorkspace{Agents: []compozyconfig.AgentDef{{
			Name: "fallback", Tools: []string{toolspkg.ToolIDToolInfo.String()},
		}}}
		policy := &workspacepkg.ResolvedAgentConfig{Agents: full.Agents}
		for _, resolver := range []*resourceAgentCatalog{nil, {}, dependency} {
			for _, name := range []string{"fallback", compozyconfig.BuiltinCoordinatorAgentName, "missing", " "} {
				for _, available := range []bool{false, true} {
					var fullContext *workspacepkg.ResolvedWorkspace
					var policyContext *workspacepkg.ResolvedAgentConfig
					if available {
						fullContext, policyContext = full, policy
					}
					want, wantErr := resolver.ResolveAgent(name, fullContext)
					got, gotErr := resolver.ResolvePolicyAgent(name, policyContext)
					if wantErr != nil {
						if gotErr == nil || gotErr.Error() != wantErr.Error() ||
							errors.Is(
								gotErr,
								workspacepkg.ErrAgentNotAvailable,
							) != errors.Is(
								wantErr,
								workspacepkg.ErrAgentNotAvailable,
							) {
							t.Fatalf(
								"ResolvePolicyAgent(%q, present=%t) error = %v, want %v",
								name,
								available,
								gotErr,
								wantErr,
							)
						}
					} else if gotErr != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf(
							"ResolvePolicyAgent(%q, present=%t) = %#v, %v; want %#v",
							name,
							available,
							got,
							gotErr,
							want,
						)
					}
				}
			}
		}
		got, err := resolvePolicyAgentFromWorkspaceSnapshot("fallback", policy)
		if err != nil {
			t.Fatalf("resolvePolicyAgentFromWorkspaceSnapshot() error = %v", err)
		}
		got.Tools[0] = "mutated"
		if policy.Agents[0].Tools[0] != toolspkg.ToolIDToolInfo.String() {
			t.Fatal("snapshot agent tools retained caller mutation")
		}
	})
}

func TestInstanceCatalogReconciliation(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "Should reconcile catalog mutations and invalidate tools"
		if fail {
			name = "Should retain catalog changes and invalidate tools when reconciliation fails"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := &bootState{
				agentCatalog:        newResourceCatalog(cloneAgentDef),
				toolProjectionEpoch: mcppkg.NewProjectionEpoch(),
				logger:              discardLogger(),
			}
			calls := 0
			bindInstanceCatalogReconciliation(t.Context(), state, func(ctx context.Context) error {
				if ctx != t.Context() {
					t.Fatal("reconciliation lost daemon context")
				}
				calls++
				if fail {
					return errors.New("reconciliation unavailable")
				}
				return nil
			})
			before := state.toolProjectionEpoch.Current()
			state.agentCatalog.Replace(
				1,
				[]resources.Record[compozyconfig.AgentDef]{
					{ID: "coder", Spec: compozyconfig.AgentDef{Name: "coder", Kind: "stays_on"}},
				},
			)
			if calls != 1 || state.toolProjectionEpoch.Current() <= before {
				t.Fatal("publication did not reconcile and invalidate")
			}
			before = state.toolProjectionEpoch.Current()
			state.agentCatalog.Update(
				func(records []resources.Record[compozyconfig.AgentDef]) []resources.Record[compozyconfig.AgentDef] {
					records[0].Spec.Kind = "on_demand"
					return records
				},
			)
			if calls != 2 || state.toolProjectionEpoch.Current() <= before ||
				state.agentCatalog.Snapshot()[0].Spec.Kind != "on_demand" {
				t.Fatal("update did not publish, reconcile and invalidate")
			}
			before = state.toolProjectionEpoch.Current()
			state.agentCatalog.Replace(3, nil)
			if calls != 3 || state.toolProjectionEpoch.Current() <= before || len(state.agentCatalog.Snapshot()) != 0 {
				t.Fatal("removal did not publish, reconcile and invalidate")
			}
		})
	}
}
