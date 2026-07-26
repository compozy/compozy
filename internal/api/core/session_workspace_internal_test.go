package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/admission"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type workspaceGetterStub struct {
	get func(context.Context, string) (workspacepkg.Workspace, error)
}

func (s workspaceGetterStub) Get(ctx context.Context, ref string) (workspacepkg.Workspace, error) {
	return s.get(ctx, ref)
}

func TestSessionWorkspaceHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should validate create session request", func(t *testing.T) {
		t.Parallel()

		if err := validateCreateSessionRequest("core-test", "", ""); err == nil {
			t.Fatal("validateCreateSessionRequest() error = nil, want non-nil")
		}
		if err := validateCreateSessionRequest("core-test", "alpha", "/workspace"); err == nil {
			t.Fatal("validateCreateSessionRequest(mutually exclusive) error = nil, want non-nil")
		}
		if err := validateCreateSessionRequest("core-test", "", "relative"); err == nil {
			t.Fatal("validateCreateSessionRequest(relative path) error = nil, want non-nil")
		}
		if err := validateCreateSessionRequest("core-test", "alpha", ""); err != nil {
			t.Fatalf("validateCreateSessionRequest(workspace ref) error = %v", err)
		}
	})

	t.Run("Should validate create session runtime overrides", func(t *testing.T) {
		t.Parallel()

		if err := validateCreateSessionRuntimeOverrides("core-test", "", "gpt-5.4", ""); !errors.Is(
			err,
			session.ErrInvalidRuntimeOverride,
		) {
			t.Fatalf("validateCreateSessionRuntimeOverrides(model) error = %v, want ErrInvalidRuntimeOverride", err)
		}
		if err := validateCreateSessionRuntimeOverrides("core-test", "", "", "high"); !errors.Is(
			err,
			session.ErrInvalidRuntimeOverride,
		) {
			t.Fatalf(
				"validateCreateSessionRuntimeOverrides(reasoning provider) error = %v, want ErrInvalidRuntimeOverride",
				err,
			)
		}
		err := validateCreateSessionRuntimeOverrides(
			"core-test",
			"codex",
			"",
			"ultra",
		)
		if !errors.Is(err, session.ErrInvalidRuntimeOverride) {
			t.Fatalf(
				"validateCreateSessionRuntimeOverrides(reasoning enum) error = %v, want ErrInvalidRuntimeOverride",
				err,
			)
		}
		item, ok := diagnostics.ItemFromError(err)
		if !ok || item.Code != diagnosticcontract.CodeReasoningEffortUnsupported {
			t.Fatalf("reasoning enum diagnostic = %#v, want reasoning_effort_unsupported; err=%v", item, err)
		}
		if got, want := statusForSessionError(err), http.StatusUnprocessableEntity; got != want {
			t.Fatalf("statusForSessionError(reasoning enum) = %d, want %d", got, want)
		}
		if err := validateCreateSessionRuntimeOverrides("core-test", "codex", "gpt-5.4", "high"); err != nil {
			t.Fatalf("validateCreateSessionRuntimeOverrides(valid) error = %v", err)
		}
	})

	t.Run("Should lookup workspace id", func(t *testing.T) {
		t.Parallel()

		if _, err := lookupWorkspaceID(context.Background(), "core-test", nil, "alpha"); err == nil {
			t.Fatal("lookupWorkspaceID(nil resolver) error = nil, want non-nil")
		}

		id, err := lookupWorkspaceID(context.Background(), "core-test", workspaceGetterStub{
			get: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				if ref != "alpha" {
					t.Fatalf("Get ref = %q, want alpha", ref)
				}
				return workspacepkg.Workspace{ID: "ws_alpha"}, nil
			},
		}, "alpha")
		if err != nil {
			t.Fatalf("lookupWorkspaceID() error = %v", err)
		}
		if id != "ws_alpha" {
			t.Fatalf("lookupWorkspaceID() = %q, want ws_alpha", id)
		}
	})

	t.Run("Should filter and trim helpers", func(t *testing.T) {
		t.Parallel()

		filtered := filterSessionInfosByWorkspaceIDInternal([]*session.Info{
			{ID: "sess-1", WorkspaceID: "ws_alpha"},
			nil,
			{ID: "sess-2", WorkspaceID: "ws_beta"},
		}, "ws_alpha")
		if len(filtered) != 1 || filtered[0].ID != "sess-1" {
			t.Fatalf("filterSessionInfosByWorkspaceIDInternal() = %#v", filtered)
		}

		trimmed := trimStringSliceInternal([]string{" one ", "", " two "})
		if len(trimmed) != 3 || trimmed[0] != "one" || trimmed[2] != "two" {
			t.Fatalf("trimStringSliceInternal() = %#v", trimmed)
		}
	})

	t.Run("Should path validators", func(t *testing.T) {
		t.Parallel()

		if err := validateAbsolutePathInternal("core-test", "path", ""); err == nil {
			t.Fatal("validateAbsolutePathInternal(empty) error = nil, want non-nil")
		}
		if err := validateAbsolutePathInternal("core-test", "path", "relative"); err == nil {
			t.Fatal("validateAbsolutePathInternal(relative) error = nil, want non-nil")
		}
		if err := validateAbsolutePathInternal("core-test", "path", "/workspace"); err != nil {
			t.Fatalf("validateAbsolutePathInternal(abs) error = %v", err)
		}

		if err := validateAbsolutePathsInternal("core-test", "paths", []string{"/workspace", "relative"}); err == nil {
			t.Fatal("validateAbsolutePathsInternal(relative entry) error = nil, want non-nil")
		}
		if err := validateAbsolutePathsInternal("core-test", "paths", []string{" /workspace ", ""}); err != nil {
			t.Fatalf("validateAbsolutePathsInternal(valid) error = %v", err)
		}
	})
}

func TestSessionWorkspaceStatusMappings(t *testing.T) {
	t.Parallel()

	if got := statusForWorkspaceError(workspacepkg.ErrWorkspaceNotFound); got != http.StatusNotFound {
		t.Fatalf("statusForWorkspaceError(not found) = %d, want %d", got, http.StatusNotFound)
	}
	if got := statusForWorkspaceError(workspacepkg.ErrWorkspaceRootMissing); got != http.StatusGone {
		t.Fatalf("statusForWorkspaceError(root missing) = %d, want %d", got, http.StatusGone)
	}
	if got := statusForWorkspaceError(workspacepkg.ErrWorkspaceHasSessions); got != http.StatusConflict {
		t.Fatalf("statusForWorkspaceError(has sessions) = %d, want %d", got, http.StatusConflict)
	}
	if got := statusForWorkspaceError(workspacepkg.ErrWorkspaceHasActiveSessions); got != http.StatusConflict {
		t.Fatalf("statusForWorkspaceError(has active sessions) = %d, want %d", got, http.StatusConflict)
	}
	if got := statusForWorkspaceError(
		workspacepkg.ErrWorkspaceResolverUnavailable,
	); got != http.StatusServiceUnavailable {
		t.Fatalf("statusForWorkspaceError(resolver unavailable) = %d, want %d", got, http.StatusServiceUnavailable)
	}
	if got := statusForWorkspaceError(context.Canceled); got != statusClientClosedRequest {
		t.Fatalf("statusForWorkspaceError(context canceled) = %d, want %d", got, statusClientClosedRequest)
	}
	if got := statusForWorkspaceError(errors.New("boom")); got != http.StatusInternalServerError {
		t.Fatalf("statusForWorkspaceError(default) = %d, want %d", got, http.StatusInternalServerError)
	}

	if got := statusForSessionError(session.ErrSessionNotFound); got != http.StatusNotFound {
		t.Fatalf("statusForSessionError(session missing) = %d, want %d", got, http.StatusNotFound)
	}
	if got := statusForSessionError(admission.ErrDraining); got != http.StatusServiceUnavailable {
		t.Fatalf("statusForSessionError(draining) = %d, want %d", got, http.StatusServiceUnavailable)
	}
	if got := statusForSessionError(os.ErrNotExist); got != http.StatusNotFound {
		t.Fatalf("statusForSessionError(os not exist) = %d, want %d", got, http.StatusNotFound)
	}
	if got := statusForSessionError(workspacepkg.ErrWorkspaceRootMissing); got != http.StatusGone {
		t.Fatalf("statusForSessionError(root missing) = %d, want %d", got, http.StatusGone)
	}
	if got := statusForSessionError(aghconfig.ErrProviderUnavailable); got != http.StatusBadRequest {
		t.Fatalf("statusForSessionError(provider unavailable) = %d, want %d", got, http.StatusBadRequest)
	}
	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "Should map invalid participation strategy to bad request",
			err:        participation.ErrStrategyInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Should map conflicting participation fields to bad request",
			err:        participation.ErrStrategyChannelConflict,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Should map an unknown participation channel to not found",
			err:        participation.ErrChannelUnknown,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Should map administratively disabled Live participation to conflict",
			err:        participation.ErrUnavailable,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "Should map unsupported Live participation to unprocessable entity",
			err:        participation.ErrLiveUnsupported,
			wantStatus: http.StatusUnprocessableEntity,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := statusForSessionError(fmt.Errorf("wrapped: %w", tc.err)); got != tc.wantStatus {
				t.Fatalf("statusForSessionError(%v) = %d, want %d", tc.err, got, tc.wantStatus)
			}
		})
	}
	t.Run("Should map negotiation errors to unprocessable entity", func(t *testing.T) {
		t.Parallel()

		if got := statusForSessionError(fmt.Errorf("wrapped: %w", &acp.NegotiationError{
			Code:      acp.NegotiationCodeModelUnavailable,
			Stage:     "model",
			Requested: "missing-model",
		})); got != http.StatusUnprocessableEntity {
			t.Fatalf("statusForSessionError(negotiation) = %d, want %d", got, http.StatusUnprocessableEntity)
		}
	})
	if got := statusForSessionError(session.ErrInvalidRuntimeOverride); got != http.StatusBadRequest {
		t.Fatalf("statusForSessionError(invalid runtime override) = %d, want %d", got, http.StatusBadRequest)
	}
	if got := statusForSessionError(session.ErrSessionNotActive); got != http.StatusBadRequest {
		t.Fatalf("statusForSessionError(not active) = %d, want %d", got, http.StatusBadRequest)
	}
	if got := statusForSessionError(session.ErrPendingPermissionConflict); got != http.StatusConflict {
		t.Fatalf("statusForSessionError(conflict) = %d, want %d", got, http.StatusConflict)
	}
	if got := statusForSessionError(transcript.ErrProjectionIncompatible); got != http.StatusServiceUnavailable {
		t.Fatalf("statusForSessionError(projection incompatible) = %d, want %d", got, http.StatusServiceUnavailable)
	}
	if got := statusForSessionError(transcript.ErrProjectionCorrupt); got != http.StatusInternalServerError {
		t.Fatalf("statusForSessionError(projection corrupt) = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := statusForSessionError(context.Canceled); got != statusClientClosedRequest {
		t.Fatalf("statusForSessionError(context canceled) = %d, want %d", got, statusClientClosedRequest)
	}
	if got := statusForSessionError(errors.New("boom")); got != http.StatusInternalServerError {
		t.Fatalf("statusForSessionError(default) = %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestSessionProviderOptionPayloads(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty input returns empty payload",
			input:    nil,
			expected: nil,
		},
		{
			name:     "single input is trimmed and preserved",
			input:    []string{"  codex  "},
			expected: []string{"codex"},
		},
		{
			name:     "duplicates and blanks are removed before sorting",
			input:    []string{"", "claude", " codex ", "claude", "  "},
			expected: []string{"claude", "codex"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payloads := sessionProviderOptionPayloads(tc.input)
			if got, want := len(payloads), len(tc.expected); got != want {
				t.Fatalf("len(payloads) = %d, want %d (%#v)", got, want, payloads)
			}
			for i, want := range tc.expected {
				if got := payloads[i].Name; got != want {
					t.Fatalf("payloads[%d].Name = %q, want %q (%#v)", i, got, want, payloads)
				}
			}
		})
	}
}

func TestSessionProviderOptionPayloadsFromConfig(t *testing.T) {
	t.Parallel()

	cfg := &aghconfig.Config{
		Defaults: aghconfig.DefaultsConfig{Provider: "codex"},
		Providers: map[string]aghconfig.ProviderConfig{
			"alpha":  {Command: "alpha --acp"},
			"claude": {Command: "claude-overlay --acp"},
		},
	}

	expectedNames := make([]string, 0, len(aghconfig.BuiltinProviders())+len(cfg.Providers))
	for name := range aghconfig.BuiltinProviders() {
		if _, err := cfg.ResolveProvider(name); err == nil {
			expectedNames = append(expectedNames, name)
		}
	}
	for name := range cfg.Providers {
		if _, err := cfg.ResolveProvider(name); err == nil {
			expectedNames = append(expectedNames, name)
		}
	}

	payloads := SessionProviderOptionPayloadsFromConfig(cfg)
	expected := sessionProviderOptionPayloads(expectedNames)
	if got, want := len(payloads), len(expected); got != want {
		t.Fatalf("len(payloads) = %d, want %d (%#v)", got, want, payloads)
	}
	if got, want := payloads[0].Name, "codex"; got != want {
		t.Fatalf("payloads[0].Name = %q, want default provider %q first (%#v)", got, want, payloads)
	}
	seen := make(map[string]bool, len(payloads))
	for _, payload := range payloads {
		seen[payload.Name] = true
	}
	for _, want := range expected {
		if !seen[want.Name] {
			t.Fatalf("payloads missing provider %q (%#v)", want.Name, payloads)
		}
	}
	remainder := make([]string, 0, len(expected)-1)
	for _, want := range expected {
		if want.Name == "codex" {
			continue
		}
		remainder = append(remainder, want.Name)
	}
	for i, want := range remainder {
		payload := payloads[i+1]
		if payload.Name != want {
			t.Fatalf("payloads[%d].Name = %q, want %q after default (%#v)", i+1, payload.Name, want, payloads)
		}
	}
	for _, payload := range payloads {
		if payload.AuthMode == "" {
			t.Fatalf("provider %q AuthMode = empty, want advertised auth mode", payload.Name)
		}
		if payload.EnvPolicy == "" {
			t.Fatalf("provider %q EnvPolicy = empty, want advertised env policy", payload.Name)
		}
		if payload.HomePolicy == "" {
			t.Fatalf("provider %q HomePolicy = empty, want advertised home policy", payload.Name)
		}
	}
}
