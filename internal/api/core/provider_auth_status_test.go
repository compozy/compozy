package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func TestSettingsProviderAuthStatusPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should expose native CLI diagnostics on the HTTP providers collection", func(t *testing.T) {
		t.Parallel()

		service := &stubSettingsService{
			ListCollectionFn: func(
				_ context.Context,
				req settingspkg.CollectionRequest,
			) (settingspkg.CollectionEnvelope, error) {
				return settingspkg.CollectionEnvelope{
					Collection:      req.Collection,
					Scope:           settingspkg.ScopeUser,
					AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeUser},
					Providers: []settingspkg.ProviderItem{{
						Name: "codex",
						Settings: settingspkg.ProviderSettings{
							Command:      "npx -y @agentclientprotocol/codex-acp@latest",
							AuthLoginCmd: "codex login --token raw-login-secret",
						},
						CommandAvailable: true,
						AuthStatus: settingspkg.ProviderAuthStatus{
							Mode:       compozyconfig.ProviderAuthModeNativeCLI,
							EnvPolicy:  compozyconfig.ProviderEnvPolicyFiltered,
							HomePolicy: compozyconfig.ProviderHomePolicyIsolated,
							State:      "missing_cli",
							Message:    "Native CLI \"codex\" was not found on PATH.",
							Login: authproviders.ProviderLoginDescriptor{
								Configured:        true,
								Source:            "auth_login_command",
								Executable:        "codex",
								Presence:          authproviders.ProviderLoginPresenceMissing,
								RecommendedAction: authproviders.ProviderFailureActionInstallCLI,
							},
						},
						SourceMetadata: settingspkg.SourceMetadata{
							EffectiveSource: settingspkg.SourceRef{
								Kind:  settingspkg.SourceKindBuiltinProvider,
								Scope: settingspkg.ScopeUser,
							},
							AvailableTargets: []settingspkg.WriteTargetKind{settingspkg.WriteTargetGlobalConfig},
						},
					}},
				}, nil
			},
		}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/settings/providers",
			http.NoBody,
		)
		resp := httptest.NewRecorder()

		fixture.Engine.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf(
				"GET /api/settings/providers status = %d, want %d; body = %s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.SettingsProvidersResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if len(payload.Providers) != 1 {
			t.Fatalf("Providers = %#v, want one provider", payload.Providers)
		}
		authStatus := payload.Providers[0].AuthStatus
		if authStatus == nil {
			t.Fatal("AuthStatus = nil, want native CLI diagnostics")
			return
		}
		if got, want := authStatus.State, "missing_cli"; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if !authStatus.Login.Configured {
			t.Fatal("AuthStatus.Login.Configured = false, want true")
		}
		if got, want := authStatus.Login.Source, contract.ProviderLoginSourceAuthLoginCommand; got != want {
			t.Fatalf("AuthStatus.Login.Source = %q, want %q", got, want)
		}
		if got, want := authStatus.Login.Executable, "codex"; got != want {
			t.Fatalf("AuthStatus.Login.Executable = %q, want %q", got, want)
		}
		if got, want := authStatus.Login.Presence, contract.ProviderLoginPresenceMissing; got != want {
			t.Fatalf("AuthStatus.Login.Presence = %q, want %q", got, want)
		}
		if got, want := authStatus.Login.RecommendedAction, contract.ProviderRecommendedActionInstallCLI; got != want {
			t.Fatalf("AuthStatus.Login.RecommendedAction = %q, want %q", got, want)
		}
		for _, forbidden := range []string{
			"raw-login-secret",
			"codex login",
			`"login_command":`,
			`"login_env":`,
			`"native_cli":`,
		} {
			if strings.Contains(resp.Body.String(), forbidden) {
				t.Fatalf("response body leaked forbidden provider login field %q: %s", forbidden, resp.Body.String())
			}
		}
	})
}
