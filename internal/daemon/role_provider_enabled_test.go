package daemon

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestRoleDisabledProviderRoutes(t *testing.T) {
	t.Parallel()

	disabledRoleConfig := func(t *testing.T, disabled ...string) compozyconfig.Config {
		t.Helper()
		cfg := roleResolverConfig()
		cfg.Roles.Coordinator.Enabled = true
		cfg.Roles.Coordinator.Provider = "codex"
		cfg.Roles.Coordinator.FallbackChain = []compozyconfig.RoleFallback{
			{Provider: "gemini"},
			{Provider: "claude"},
		}
		providers := compozyconfig.CloneProviderConfigs(cfg.Providers)
		for _, name := range disabled {
			provider := providers[name]
			provider.Enabled = new(false)
			providers[name] = provider
		}
		cfg.Providers = providers
		return cfg
	}

	t.Run("Should use the first enabled fallback when the primary provider is disabled", func(t *testing.T) {
		t.Parallel()
		cfg := disabledRoleConfig(t, "codex", "gemini")
		resolved, err := newRoleResolver(&cfg, nil, nil).Resolve(t.Context(), "", compozyconfig.RoleCoordinator)
		if err != nil {
			t.Fatalf("Resolve(coordinator) error = %v", err)
		}
		if !resolved.PrimaryDisabled || len(resolved.Fallbacks) != 1 || resolved.Fallbacks[0].Provider != "claude" {
			t.Fatalf("Resolve(coordinator) = primary_disabled %v fallbacks %#v, want only claude",
				resolved.PrimaryDisabled, resolved.Fallbacks)
		}
		var attempts []string
		result, err := resolved.invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			route roleAttemptRoute,
		) (string, bool, error) {
			attempts = append(attempts, route.Provider)
			return "accepted", true, nil
		})
		if err != nil || result != "accepted" {
			t.Fatalf("invokeRoleWithFallback() = %q, %v, want accepted", result, err)
		}
		if want := []string{"claude"}; !reflect.DeepEqual(attempts, want) {
			t.Fatalf("attempts = %#v, want %#v", attempts, want)
		}
	})

	t.Run("Should report provider_disabled when every role route is disabled", func(t *testing.T) {
		t.Parallel()
		cfg := disabledRoleConfig(t, "codex", "gemini", "claude")
		resolver := newRoleResolver(&cfg, nil, nil)
		_, err := resolver.Resolve(t.Context(), "", compozyconfig.RoleCoordinator)
		resolutionErr, ok := errors.AsType[*RoleResolutionError](err)
		if !ok || resolutionErr.Code != contract.CodeProviderDisabled ||
			!errors.Is(err, compozyconfig.ErrProviderDisabled) {
			t.Fatalf("Resolve(coordinator) error = %v, want provider_disabled role resolution error", err)
		}
		status, err := resolver.RoleStatus(t.Context(), "", string(compozyconfig.RoleCoordinator))
		if err != nil {
			t.Fatalf("RoleStatus(coordinator) error = %v", err)
		}
		if len(status.Diagnostics) != 1 || status.Diagnostics[0].Code != contract.CodeProviderDisabled {
			t.Fatalf("RoleStatus(coordinator).Diagnostics = %#v, want one provider_disabled", status.Diagnostics)
		}
	})

	t.Run("Should keep role status diagnostics empty while a fallback stays enabled", func(t *testing.T) {
		t.Parallel()
		cfg := disabledRoleConfig(t, "codex", "gemini")
		status, err := newRoleResolver(
			&cfg,
			nil,
			nil,
		).RoleStatus(t.Context(), "", string(compozyconfig.RoleCoordinator))
		if err != nil {
			t.Fatalf("RoleStatus(coordinator) error = %v", err)
		}
		if len(status.Diagnostics) != 0 {
			t.Fatalf("RoleStatus(coordinator).Diagnostics = %#v, want none", status.Diagnostics)
		}
	})
	t.Run("Should report provider_disabled for a primary inherited from the role agent", func(t *testing.T) {
		t.Parallel()
		cfg := disabledRoleConfig(t, "codex")
		cfg.Roles.Coordinator.Provider = ""
		cfg.Roles.Coordinator.FallbackChain = nil
		cfg.Roles.Coordinator.Agent = "my-curator"
		resolver := newRoleResolver(&cfg, nil, roleAgentResolverStub{
			agents: map[string]compozyconfig.AgentDef{
				"my-curator": {Name: "my-curator", Provider: "codex", Prompt: "curate"},
			},
		})
		status, err := resolver.RoleStatus(t.Context(), "", string(compozyconfig.RoleCoordinator))
		if err != nil {
			t.Fatalf("RoleStatus(coordinator) error = %v", err)
		}
		if len(status.Diagnostics) != 1 || status.Diagnostics[0].Code != contract.CodeProviderDisabled {
			t.Fatalf("RoleStatus(coordinator).Diagnostics = %#v, want one provider_disabled", status.Diagnostics)
		}
		_, err = resolver.Resolve(t.Context(), "", compozyconfig.RoleCoordinator)
		if !errors.Is(err, compozyconfig.ErrProviderDisabled) {
			t.Fatalf("Resolve(coordinator) error = %v, want the same role unavailable at invocation", err)
		}
	})
}
