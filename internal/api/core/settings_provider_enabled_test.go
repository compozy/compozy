package core_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func TestSetSettingsProviderEnabled(t *testing.T) {
	t.Parallel()

	t.Run("Should toggle the provider and return the item with its live apply", func(t *testing.T) {
		t.Parallel()
		var gotName string
		var gotEnabled bool
		service := &stubSettingsService{
			SetProviderEnabledFn: func(
				_ context.Context,
				name string,
				enabled bool,
			) (settingspkg.ProviderItem, settingspkg.ApplyResult, error) {
				gotName, gotEnabled = name, enabled
				return settingspkg.ProviderItem{
					Name:    name,
					Enabled: enabled,
				}, settingspkg.ApplyResult{
					Applied: true,
				}, nil
			},
		}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		for _, enabled := range []bool{false, true} {
			resp := performRequest(t, fixture.Engine, http.MethodPatch,
				"/api/settings/providers/gemini/enabled", fmt.Appendf(nil, `{"enabled":%t}`, enabled))
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
			}
			var payload contract.SettingsProviderEnabledResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if gotName != "gemini" || gotEnabled != enabled || payload.Provider.Settings.Enabled != enabled ||
				!payload.Apply.Applied {
				t.Fatalf("toggle = %q/%v payload %#v, want gemini enabled=%v and applied",
					gotName, gotEnabled, payload, enabled)
			}
		}
	})

	for _, tc := range []struct {
		name      string
		body      string
		wantError string
	}{
		{name: "Should reject a missing enabled field", body: `{}`, wantError: "enabled is required"},
		{
			name:      "Should reject a non-boolean enabled value",
			body:      `{"enabled":"no"}`,
			wantError: "invalid enabled payload",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := &stubSettingsService{}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
			resp := performRequest(t, fixture.Engine, http.MethodPatch,
				"/api/settings/providers/gemini/enabled", []byte(tc.body))
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", resp.Code, resp.Body.String())
			}
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, resp, &payload)
			if !strings.Contains(payload.Error, tc.wantError) {
				t.Fatalf("error = %q, want it to contain %q", payload.Error, tc.wantError)
			}
		})
	}

	for _, tc := range []struct {
		name     string
		err      error
		wantCode int
		wantBody string
	}{
		{
			name:     "Should map an unknown provider to 404",
			err:      fmt.Errorf("%w: provider %q not found", settingspkg.ErrNotFound, "nope"),
			wantCode: http.StatusNotFound,
		},
		{
			name: "Should map a default provider rejection to 422 provider_default_disable_rejected",
			err: fmt.Errorf("%w: %w", settingspkg.ErrUnprocessable,
				&compozyconfig.ProviderDefaultDisableError{Provider: "codex"}),
			wantCode: http.StatusUnprocessableEntity,
			wantBody: "provider_default_disable_rejected",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := &stubSettingsService{
				SetProviderEnabledFn: func(
					context.Context,
					string,
					bool,
				) (settingspkg.ProviderItem, settingspkg.ApplyResult, error) {
					return settingspkg.ProviderItem{}, settingspkg.ApplyResult{}, tc.err
				},
			}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
			resp := performRequest(t, fixture.Engine, http.MethodPatch,
				"/api/settings/providers/codex/enabled", []byte(`{"enabled":false}`))
			if resp.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tc.wantCode, resp.Body.String())
			}
			if tc.wantBody != "" {
				var payload contract.ErrorPayload
				testutil.DecodeJSONResponse(t, resp, &payload)
				if payload.Code != tc.wantBody {
					t.Fatalf("error code = %q, want %q; body=%s", payload.Code, tc.wantBody, resp.Body.String())
				}
			}
		})
	}
}
