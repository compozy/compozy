package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type selectionConfigResolver struct {
	workspace compozyconfig.Config
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

// selectionProfileNames maps profile ids to names the way the profile store does.
type selectionProfileNames map[string]string

func (p selectionProfileNames) List(context.Context) ([]profilepkg.WithCounts, error) {
	return nil, nil
}

func (p selectionProfileNames) ListIdentities(context.Context) ([]profilepkg.Profile, error) {
	return nil, nil
}

func (p selectionProfileNames) ProfileName(_ context.Context, id string) (string, error) {
	return p[id], nil
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
	newTools := func(t *testing.T, active compozyconfig.Config, profileFiles map[string]string) *daemonNativeTools {
		t.Helper()
		root := t.TempDir()
		homePaths := compozyconfig.HomePaths{
			ConfigFile:  filepath.Join(root, compozyconfig.ConfigName),
			ProfilesDir: filepath.Join(root, "profiles"),
		}
		// A pending global disable sits in the desired file; selection must never read it.
		if err := os.WriteFile(
			homePaths.ConfigFile,
			[]byte("[providers.gemini]\nenabled = false\n"),
			0o600,
		); err != nil {
			t.Fatalf("write global config: %v", err)
		}
		for name, content := range profileFiles {
			dir := filepath.Join(homePaths.ProfilesDir, name)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatalf("mkdir profile: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, compozyconfig.ConfigName), []byte(content), 0o600); err != nil {
				t.Fatalf("write profile config: %v", err)
			}
		}
		return &daemonNativeTools{deps: &daemonNativeToolsDeps{
			Config:            active,
			HomePaths:         homePaths,
			WorkspaceResolver: &selectionConfigResolver{workspace: withGemini(nil)},
			Profiles: selectionProfileNames{
				store.DefaultProfileID: "default", "profile-eng": "eng", "profile-plain": "plain",
			},
		}}
	}
	disable := "[providers.gemini]\nenabled = false\n"
	enable := "[providers.gemini]\nenabled = true\n"

	for _, tc := range []struct {
		name         string
		active       compozyconfig.Config
		profileFiles map[string]string
		scope        toolspkg.Scope
		wantDisabled bool
	}{
		{
			name: "Should apply a named profile's disable over an enabled global", active: withGemini(nil),
			profileFiles: map[string]string{"eng": disable}, scope: toolspkg.Scope{ProfileID: "profile-eng"},
			wantDisabled: true,
		},
		{
			name: "Should apply a named profile's re-enable over a global disable", active: withGemini(new(false)),
			profileFiles: map[string]string{"eng": enable}, scope: toolspkg.Scope{ProfileID: "profile-eng"},
		},
		{
			name: "Should apply the default profile's own disable", active: withGemini(nil),
			profileFiles: map[string]string{"default": disable}, scope: toolspkg.Scope{ProfileID: store.DefaultProfileID},
			wantDisabled: true,
		},
		{
			name: "Should apply the default profile's re-enable over a global disable", active: withGemini(new(false)),
			profileFiles: map[string]string{"default": enable}, scope: toolspkg.Scope{ProfileID: store.DefaultProfileID},
		},
		{
			name: "Should ignore a pending global disable for a profile without overrides", active: withGemini(nil),
			scope: toolspkg.Scope{ProfileID: "profile-plain"},
		},
		{
			name: "Should filter with the caller's workspace config", active: withGemini(new(false)),
			scope: toolspkg.Scope{WorkspaceID: "ws-lab"},
		},
		{
			name: "Should fall back to the active config without a scope", active: withGemini(new(false)),
			wantDisabled: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := newTools(t, tc.active, tc.profileFiles).selectionConfig(t.Context(), tc.scope)
			if err != nil {
				t.Fatalf("selectionConfig(%+v) error = %v", tc.scope, err)
			}
			if got := cfg.IsProviderDisabled("gemini"); got != tc.wantDisabled {
				t.Fatalf("selectionConfig(%+v) gemini disabled = %v, want %v", tc.scope, got, tc.wantDisabled)
			}
		})
	}

	t.Run(
		"Should return an error instead of the unscoped config when the profile config is unreadable",
		func(t *testing.T) {
			t.Parallel()
			tools := newTools(t, withGemini(nil), map[string]string{"eng": "[providers.gemini\nenabled = false\n"})
			cfg, err := tools.selectionConfig(t.Context(), toolspkg.Scope{ProfileID: "profile-eng"})
			if err == nil {
				t.Fatalf("selectionConfig(profile-eng) gemini disabled = %v, want the profile decode error",
					cfg.IsProviderDisabled("gemini"))
			}
		},
	)
}
