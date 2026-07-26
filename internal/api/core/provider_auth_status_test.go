package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	settingspkg "github.com/compozy/agh/internal/settings"
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
					Scope:           settingspkg.ScopeGlobal,
					AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
					Providers: []settingspkg.ProviderItem{{
						Name: "codex",
						Settings: settingspkg.ProviderSettings{
							Command: "npx -y @agentclientprotocol/codex-acp@latest",
						},
						CommandAvailable: true,
						AuthStatus: settingspkg.ProviderAuthStatus{
							Mode:       aghconfig.ProviderAuthModeNativeCLI,
							EnvPolicy:  aghconfig.ProviderEnvPolicyFiltered,
							HomePolicy: aghconfig.ProviderHomePolicyIsolated,
							State:      "missing_cli",
							Message:    "Native CLI \"codex\" was not found on PATH.",
							LoginCmd:   "codex login",
							LoginEnv:   []string{"HOME=/tmp/agh/providers/codex"},
							NativeCLI: &settingspkg.ProviderNativeCLIStatus{
								Command: "codex",
								Present: false,
								Source:  "auth_login_command",
							},
						},
						SourceMetadata: settingspkg.SourceMetadata{
							EffectiveSource: settingspkg.SourceRef{
								Kind:  settingspkg.SourceKindBuiltinProvider,
								Scope: settingspkg.ScopeGlobal,
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
		if authStatus.NativeCLI == nil || authStatus.NativeCLI.Command != "codex" {
			t.Fatalf("AuthStatus.NativeCLI = %#v, want codex diagnostic", authStatus.NativeCLI)
		}
		if got, want := authStatus.NativeCLI.Present, false; got != want {
			t.Fatalf("AuthStatus.NativeCLI.Present = %t, want %t", got, want)
		}
		if got, want := authStatus.NativeCLI.Source, "auth_login_command"; got != want {
			t.Fatalf("AuthStatus.NativeCLI.Source = %q, want %q", got, want)
		}
		if got, want := strings.Join(authStatus.LoginEnv, " "), "HOME=/tmp/agh/providers/codex"; got != want {
			t.Fatalf("AuthStatus.LoginEnv = %q, want %q", got, want)
		}
	})
}
