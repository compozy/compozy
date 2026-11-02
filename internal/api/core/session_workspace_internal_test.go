package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type workspaceGetterStub struct {
	get func(context.Context, string) (workspacepkg.Workspace, error)
}

func TestWorkspaceSkillPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the resolved frontmatter identity", func(t *testing.T) {
		t.Parallel()

		payloads := WorkspaceSkillPayloads([]workspacepkg.SkillPath{{
			Name:   "campaign-brief",
			Dir:    "/workspace/.compozy/skills/marketing/brief",
			Source: "workspace",
		}})
		if len(payloads) != 1 {
			t.Fatalf("len(WorkspaceSkillPayloads()) = %d, want 1", len(payloads))
		}
		if got, want := payloads[0].Name, "campaign-brief"; got != want {
			t.Fatalf("WorkspaceSkillPayloads()[0].Name = %q, want %q", got, want)
		}
	})
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

	t.Run("Should lookup workspace id", func(t *testing.T) {
		t.Parallel()

		if _, err := lookupWorkspaceID(t.Context(), "core-test", nil, "alpha"); err == nil {
			t.Fatal("lookupWorkspaceID(nil resolver) error = nil, want non-nil")
		}

		id, err := lookupWorkspaceID(t.Context(), "core-test", workspaceGetterStub{
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
	if got := statusForSessionError(compozyconfig.ErrProviderUnavailable); got != http.StatusBadRequest {
		t.Fatalf("statusForSessionError(provider unavailable) = %d, want %d", got, http.StatusBadRequest)
	}
	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
	}{

		{
			name:       "Should map a removed agent to not found",
			err:        skills.ErrAgentNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Should map unsupported prompt images to unprocessable entity",
			err:        session.ErrPromptImagesUnsupported,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "Should map unsupported prompt files to unprocessable entity",
			err:        session.ErrPromptFilesUnsupported,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "Should map an invalid agent definition to unprocessable entity",
			err:        skills.ErrAgentLocalInvalid,
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
	t.Run("Should map archive errors to conflict", func(t *testing.T) {
		t.Parallel()

		for _, archiveErr := range []error{
			session.ErrSessionArchived,
			session.ErrSessionArchiveRequiresStopped,
			store.ErrSessionArchived,
			store.ErrSessionIsInstanceMain,
		} {
			if got := statusForSessionError(archiveErr); got != http.StatusConflict {
				t.Fatalf("statusForSessionError(%v) = %d, want %d", archiveErr, got, http.StatusConflict)
			}
		}
	})
	t.Run("Should report a failed stop before archiving as a coded conflict", func(t *testing.T) {
		t.Parallel()

		// The stop cause alone would map to 400; the archive refusal owns the status.
		stopErr := fmt.Errorf("%w: sess-a: %w", session.ErrSessionArchiveStopFailed, session.ErrSessionNotActive)
		if got := statusForSessionError(stopErr); got != http.StatusConflict {
			t.Fatalf("statusForSessionError(%v) = %d, want %d", stopErr, got, http.StatusConflict)
		}
		if payload := ErrorPayloadForStatus(http.StatusConflict, stopErr, false); payload.Code !=
			"session_archive_stop_failed" {
			t.Fatalf("ErrorPayloadForStatus(%v) = %#v, want session_archive_stop_failed code", stopErr, payload)
		}
	})
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

	cfg := &compozyconfig.Config{
		Defaults: compozyconfig.DefaultsConfig{Provider: "codex"},
		Providers: map[string]compozyconfig.ProviderConfig{
			"alpha":  {Command: "alpha --acp"},
			"claude": {Command: "claude-overlay --acp"},
		},
	}

	expectedNames := make([]string, 0, len(compozyconfig.BuiltinProviders())+len(cfg.Providers))
	for name := range compozyconfig.BuiltinProviders() {
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
	liveApply := make(map[string]contract.ProviderRuntimeLiveApply, len(payloads))
	for _, payload := range payloads {
		seen[payload.Name] = true
		liveApply[payload.Name] = payload.RuntimeLiveApply
	}
	if liveApply["claude"] != contract.ProviderRuntimeLiveApplyNextStep ||
		liveApply["alpha"] != contract.ProviderRuntimeLiveApplyNextTurn ||
		liveApply["cursor"] != "" {
		t.Fatalf("runtime live apply = %#v, want claude next_step, alpha next_turn, cursor omitted", liveApply)
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

// Invariant: optional health retains direct linked-work board placement.
// Owner: API detail decoration; canonical session workspace helper suite.
func TestSessionOptionalHealthLinkedWork(t *testing.T) {
	t.Parallel()
	t.Run("Should retain linked review after health decoration", func(t *testing.T) {
		t.Parallel()
		h := &BaseHandlers{Sessions: healthLinkedWorkReader{}, SessionHealth: healthLinkedWorkReader{}}
		payload, err := h.sessionPayloadWithOptionalHealth(
			t.Context(),
			&session.Info{ID: "linked", State: session.StateStopped},
			true,
		)
		if err != nil || payload.BoardColumn != session.BoardInReview || payload.LinkedWork == nil {
			t.Fatalf("payload=%+v err=%v", payload, err)
		}
	})
}

func TestRespondLinkedWorkError(t *testing.T) {
	t.Parallel()
	// Invariant: only refused input is the client's fault; a daemon failure stays a 5xx the client can retry.
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"Should keep 400 for an invalid link", session.ErrLinkedWorkInvalid, http.StatusBadRequest},
		{"Should keep 400 for another repository", session.ErrLinkedWorkRepository, http.StatusBadRequest},
		{"Should keep 400 for a fork", session.ErrLinkedWorkForkUnsupported, http.StatusBadRequest},
		{"Should answer 500 for a storage failure", errors.New("database is locked"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			(&BaseHandlers{}).respondLinkedWorkError(ctx, tc.err)
			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.want)
			}
		})
	}
}

type healthLinkedWorkReader struct{ SessionManager }

func (healthLinkedWorkReader) InputQueueSummary(context.Context, string) (session.InputQueueSummary, error) {
	return session.InputQueueSummary{}, nil
}
func (healthLinkedWorkReader) GetSessionHealth(context.Context, string) (heartbeat.SessionHealth, error) {
	return heartbeat.SessionHealth{
		SessionID: "linked",
		State:     heartbeat.SessionHealthStateIdle,
		Health:    heartbeat.SessionHealthHealthy,
	}, nil
}

func (healthLinkedWorkReader) SessionLinkedWork(
	context.Context,
	[]*session.Info,
) (map[string]session.LinkedWorkProjection, error) {
	return map[string]session.LinkedWorkProjection{"linked": {DirectOpen: true}}, nil
}
func (healthLinkedWorkReader) LinkSessionWork(context.Context, string, store.SessionWorkLink, bool) error {
	return nil
}
func (healthLinkedWorkReader) UnlinkSessionWork(context.Context, string, string, string) error {
	return nil
}
func (healthLinkedWorkReader) RefreshSessionWorkLink(context.Context, string, string, string) error {
	return nil
}
