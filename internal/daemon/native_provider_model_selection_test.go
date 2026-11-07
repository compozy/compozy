package daemon

import (
	"context"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type selectionConfigResolver struct {
	workspace compozyconfig.Config
	profiles  map[string]compozyconfig.Config
}

func (r *selectionConfigResolver) Resolve(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{Config: r.workspace}, nil
}

func (r *selectionConfigResolver) ResolveOrRegister(
	ctx context.Context,
	ref string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.Resolve(ctx, ref)
}

func (r *selectionConfigResolver) ResolveUser(
	_ context.Context,
	profileID string,
) (workspacepkg.ResolvedWorkspace, error) {
	cfg, ok := r.profiles[profileID]
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return workspacepkg.ResolvedWorkspace{Config: cfg}, nil
}

func TestNativeProviderModelSelectionConfig(t *testing.T) {
	t.Parallel()

	withGemini := func(enabled *bool) compozyconfig.Config {
		cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		providers := compozyconfig.CloneProviderConfigs(cfg.Providers)
		gemini := providers["gemini"]
		gemini.Enabled = enabled
		providers["gemini"] = gemini
		cfg.Providers = providers
		return cfg
	}
	tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{
		Config: withGemini(new(false)),
		WorkspaceResolver: &selectionConfigResolver{
			workspace: withGemini(nil),
			profiles: map[string]compozyconfig.Config{
				"profile-eng":      withGemini(new(false)),
				"profile-override": withGemini(new(true)),
			},
		},
	}}

	for _, tc := range []struct {
		name         string
		scope        toolspkg.Scope
		wantDisabled bool
	}{
		{name: "Should filter with the caller's profile disable", scope: toolspkg.Scope{ProfileID: "profile-eng"}, wantDisabled: true},
		{
			name:  "Should honor a profile re-enable over a global disable",
			scope: toolspkg.Scope{ProfileID: "profile-override"},
		},
		{name: "Should filter with the caller's workspace config", scope: toolspkg.Scope{WorkspaceID: "ws-lab"}},
		{name: "Should fall back to the active config without a scope", scope: toolspkg.Scope{}, wantDisabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := tools.selectionConfig(t.Context(), tc.scope)
			if got := cfg.IsProviderDisabled("gemini"); got != tc.wantDisabled {
				t.Fatalf("selectionConfig(%+v) gemini disabled = %v, want %v", tc.scope, got, tc.wantDisabled)
			}
		})
	}
}
