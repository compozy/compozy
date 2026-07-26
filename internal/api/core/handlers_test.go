package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestBaseHandlersSessionEndpoints(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	var createCalled atomic.Bool
	var attachCalls atomic.Int32
	var attachResult store.SessionAttach
	var repairSeen session.RepairOpts
	manager := testutil.StubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{testutil.NewSessionInfo("sess-a")}, nil
		},
		CreateAcceptedFn: func(_ context.Context, acceptedOpts session.CreateAcceptedOpts) (*session.Info, error) {
			createCalled.Store(true)
			opts := acceptedOpts.Session
			if opts.AgentName != "coder" ||
				opts.Provider != "fake" ||
				opts.Model != "fake-pro" ||
				opts.ReasoningEffort != "high" ||
				opts.Workspace != "alpha" ||
				opts.Type != session.SessionTypeUser {
				t.Fatalf("CreateAccepted opts = %#v", opts)
			}
			if acceptedOpts.InitialPrompt != "Investigate the failing build" {
				t.Fatalf("CreateAccepted initial prompt = %q", acceptedOpts.InitialPrompt)
			}
			created := testutil.NewSessionInfo("sess-created")
			created.AgentName = opts.AgentName
			created.Provider = opts.Provider
			created.State = session.StateStarting
			return created, nil
		},
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			if id == "missing" {
				return nil, session.ErrSessionNotFound
			}
			info := testutil.NewSessionInfo(id)
			info.CreatedAt = now
			info.UpdatedAt = now
			info.TranscriptEpoch = 1
			if attachResult.SessionID == id {
				info.AttachedTo = attachResult.AttachedTo
				info.AttachExpiresAt = &attachResult.AttachExpiresAt
				info.UpdatedAt = attachResult.AttachedAt
			}
			return info, nil
		},
		DeleteFn: func(_ context.Context, id string) error {
			if id != "sess-a" {
				t.Fatalf("Delete id = %q, want sess-a", id)
			}
			return nil
		},
		StopFn: func(_ context.Context, id string) error {
			if id != "sess-a" {
				t.Fatalf("Stop id = %q, want sess-a", id)
			}
			return nil
		},
		AttachSessionFn: func(_ context.Context, req store.SessionAttachRequest) (store.SessionAttach, error) {
			attachCalls.Add(1)
			if req.SessionID != "sess-a" {
				t.Fatalf("AttachSession() session id = %q, want sess-a", req.SessionID)
			}
			attachResult = store.SessionAttach{
				SessionID:       req.SessionID,
				AttachedTo:      req.AttachedTo,
				AttachedAt:      req.Now,
				AttachExpiresAt: req.Now.Add(req.TTL),
			}
			return attachResult, nil
		},
		RepairFn: func(_ context.Context, opts session.RepairOpts) (*session.RepairResult, error) {
			repairSeen = opts
			return &session.RepairResult{
				SessionID: opts.SessionID,
				Actions: []session.RepairAction{{
					Code:      session.RepairActionAppendTerminalError,
					TurnID:    "turn-1",
					Persisted: !opts.DryRun,
				}},
				Persisted: !opts.DryRun,
			}, nil
		},
		EventsFn: func(_ context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error) {
			if id != "sess-a" || query.Limit != 10 || query.AfterSequence != 5 {
				t.Fatalf("Events call = %q %#v", id, query)
			}
			return []store.SessionEvent{{
				ID:        "ev-1",
				SessionID: id,
				Sequence:  6,
				TurnID:    "turn-1",
				Type:      "agent_message",
				AgentName: "coder",
				Content:   `{"text":"hello"}`,
				Timestamp: now,
			}}, nil
		},
		HistoryFn: func(_ context.Context, id string, _ store.EventQuery) ([]store.TurnHistory, error) {
			return []store.TurnHistory{{
				TurnID: "turn-1",
				Events: []store.SessionEvent{{
					ID:        "ev-1",
					SessionID: id,
					Sequence:  1,
					TurnID:    "turn-1",
					Type:      "agent_message",
					AgentName: "coder",
					Content:   `{"text":"hello"}`,
					Timestamp: now,
				}},
			}}, nil
		},
		TranscriptPageFn: func(_ context.Context, id string, query transcript.PageQuery) (transcript.Page, error) {
			if id != "sess-a" || query.Limit != 2 || query.BeforeSequence != 9 {
				t.Fatalf("TranscriptPage() call = %q %#v, want bounded backward page", id, query)
			}
			return transcript.Page{Entries: []transcript.Entry{{
				StartSequence: 1,
				Sequence:      1,
				Message: transcript.UIMessage{
					ID:   "msg-1",
					Role: transcript.UIRoleUser,
					Parts: []transcript.UIMessagePart{{
						Type:  "text",
						Text:  "hello",
						State: "done",
					}},
				},
			}}, Generation: 4, MaxSequence: 12, HasOlder: true, NextBeforeSequence: 1}, nil
		},
	}

	fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)

	t.Run("Should list sessions", func(t *testing.T) {
		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list status = %d, want %d", listResp.Code, http.StatusOK)
		}
	})

	t.Run("Should create sessions", func(t *testing.T) {
		createResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{
				"agent_name":"coder",
				"provider":"fake",
				"model":"fake-pro",
				"reasoning_effort":"high",
				"workspace":"alpha",
				"prompt":"  Investigate the failing build  "
			}`),
		)
		if createResp.Code != http.StatusCreated || !createCalled.Load() {
			t.Fatalf("create status = %d, called=%v", createResp.Code, createCalled.Load())
		}
		var payload contract.SessionResponse
		if err := json.Unmarshal(createResp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(create response) error = %v", err)
		}
		if payload.Session.Provider != "fake" {
			t.Fatalf("created session provider = %q, want %q", payload.Session.Provider, "fake")
		}
		if payload.Session.State != session.StateStarting {
			t.Fatalf("created session state = %q, want %q", payload.Session.State, session.StateStarting)
		}
	})

	t.Run(
		"Should reject session creation without durable acceptance and never call synchronous create",
		func(t *testing.T) {
			fixture.Handlers.SessionAcceptance = nil
			createCalled.Store(false)
			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodPost,
				"/sessions",
				[]byte(`{"agent_name":"coder","provider":"fake","workspace":"alpha"}`),
			)
			fixture.Handlers.SessionAcceptance = manager
			if resp.Code != http.StatusServiceUnavailable {
				t.Fatalf(
					"create status = %d, want %d; body=%s",
					resp.Code,
					http.StatusServiceUnavailable,
					resp.Body.String(),
				)
			}
			if createCalled.Load() {
				t.Fatal("synchronous Create() called when durable acceptance was unavailable")
			}
		},
	)

	t.Run("Should get session details", func(t *testing.T) {
		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces/ws-workspace/sessions/sess-a", nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d", getResp.Code, http.StatusOK)
		}
	})

	t.Run("Should return not found for missing sessions", func(t *testing.T) {
		notFoundResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/missing",
			nil,
		)
		if notFoundResp.Code != http.StatusNotFound {
			t.Fatalf("get missing status = %d, want %d", notFoundResp.Code, http.StatusNotFound)
		}
	})

	t.Run("Should delete sessions", func(t *testing.T) {
		deleteResp := performRequest(
			t,
			fixture.Engine,
			http.MethodDelete,
			"/workspaces/ws-workspace/sessions/sess-a",
			nil,
		)
		if deleteResp.Code != http.StatusNoContent {
			t.Fatalf("delete status = %d, want %d", deleteResp.Code, http.StatusNoContent)
		}
		if got := deleteResp.Body.String(); got != "" {
			t.Fatalf("delete body = %q, want empty", got)
		}
	})

	t.Run("Should stop sessions", func(t *testing.T) {
		stopResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/sessions/sess-a/stop",
			nil,
		)
		if stopResp.Code != http.StatusNoContent {
			t.Fatalf("stop status = %d, want %d", stopResp.Code, http.StatusNoContent)
		}
		if got := stopResp.Body.String(); got != "" {
			t.Fatalf("stop body = %q, want empty", got)
		}
	})

	t.Run("Should attach sessions", func(t *testing.T) {
		attachResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/sessions/sess-a/attach",
			nil,
		)
		if attachResp.Code != http.StatusOK {
			t.Fatalf("attach status = %d, want %d", attachResp.Code, http.StatusOK)
		}
		var payload contract.SessionAttachResponse
		if err := json.Unmarshal(attachResp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(attach response) error = %v", err)
		}
		if payload.Attach.SessionID != "sess-a" {
			t.Fatalf("attach session id = %q, want %q", payload.Attach.SessionID, "sess-a")
		}
		if payload.Attach.AttachedAt.IsZero() {
			t.Fatal("attach attached_at = zero, want populated timestamp")
		}
		if !payload.Attach.AttachExpiresAt.After(payload.Attach.AttachedAt) {
			t.Fatalf(
				"attach expires_at = %v, attached_at = %v; want expires after attached",
				payload.Attach.AttachExpiresAt,
				payload.Attach.AttachedAt,
			)
		}
		if payload.Session.ID != "sess-a" {
			t.Fatalf("session id = %q, want %q", payload.Session.ID, "sess-a")
		}
		if payload.Session.AttachedTo != payload.Attach.AttachedTo {
			t.Fatalf(
				"session attached_to = %q, attach attached_to = %q; want match",
				payload.Session.AttachedTo,
				payload.Attach.AttachedTo,
			)
		}
		if payload.Session.AttachExpiresAt == nil ||
			!payload.Session.AttachExpiresAt.Equal(payload.Attach.AttachExpiresAt) {
			t.Fatalf(
				"session attach_expires_at = %#v, attach attach_expires_at = %v; want match",
				payload.Session.AttachExpiresAt,
				payload.Attach.AttachExpiresAt,
			)
		}
		if payload.Session.Attachable {
			t.Fatal("session attachable = true after successful attach, want false")
		}
		if !payload.Session.UpdatedAt.Equal(payload.Attach.AttachedAt) {
			t.Fatalf(
				"session updated_at = %v, want attach time %v",
				payload.Session.UpdatedAt,
				payload.Attach.AttachedAt,
			)
		}
	})

	t.Run("Should reject cross workspace attach before the Manager CAS", func(t *testing.T) {
		before := attachCalls.Load()
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-other/sessions/sess-a/attach",
			nil,
		)
		if resp.Code != http.StatusNotFound {
			t.Fatalf(
				"cross-workspace attach status = %d, want %d; body=%s",
				resp.Code,
				http.StatusNotFound,
				resp.Body.String(),
			)
		}
		if attachCalls.Load() != before {
			t.Fatalf("AttachSession() calls = %d after cross-workspace request, want %d", attachCalls.Load(), before)
		}
	})

	t.Run("Should repair sessions", func(t *testing.T) {
		repairResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/sessions/sess-a/repair?dry_run=true&force=true",
			nil,
		)
		if repairResp.Code != http.StatusOK {
			t.Fatalf("repair status = %d, want %d", repairResp.Code, http.StatusOK)
		}
		if repairSeen.SessionID != "sess-a" || !repairSeen.DryRun || !repairSeen.Force {
			t.Fatalf("repair opts = %#v, want sess-a dry-run force", repairSeen)
		}
		var payload contract.SessionRepairResponse
		if err := json.Unmarshal(repairResp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(repair response) error = %v", err)
		}
		if payload.Repair.SessionID != "sess-a" {
			t.Fatalf("repair session id = %q, want sess-a", payload.Repair.SessionID)
		}
		if payload.Repair.Persisted {
			t.Fatalf("repair persisted = %v, want false for dry-run", payload.Repair.Persisted)
		}
		if got, want := len(payload.Repair.Actions), 1; got != want {
			t.Fatalf("repair actions len = %d, want %d", got, want)
		}
		action := payload.Repair.Actions[0]
		if got, want := action.Code, session.RepairActionAppendTerminalError; got != want {
			t.Fatalf("repair action code = %q, want %q", got, want)
		}
		if got, want := action.TurnID, "turn-1"; got != want {
			t.Fatalf("repair action turn id = %q, want %q", got, want)
		}
		if action.Persisted {
			t.Fatalf("repair action persisted = %v, want false for dry-run", action.Persisted)
		}
	})

	t.Run("Should reject conflicting repair query aliases", func(t *testing.T) {
		repairResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/sessions/sess-a/repair?dry_run=true&dry-run=false",
			nil,
		)
		if repairResp.Code != http.StatusBadRequest {
			t.Fatalf("repair conflicting alias status = %d, want %d", repairResp.Code, http.StatusBadRequest)
		}
		if !strings.Contains(repairResp.Body.String(), "conflicting boolean query values for dry_run, dry-run") {
			t.Fatalf("repair conflicting alias body = %q, want conflict message", repairResp.Body.String())
		}
	})

	t.Run("Should return session events", func(t *testing.T) {
		eventsResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/events?limit=10&after_sequence=5",
			nil,
		)
		if eventsResp.Code != http.StatusOK {
			t.Fatalf("events status = %d, want %d", eventsResp.Code, http.StatusOK)
		}
	})

	t.Run("Should return session history", func(t *testing.T) {
		historyResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/history",
			nil,
		)
		if historyResp.Code != http.StatusOK {
			t.Fatalf("history status = %d, want %d", historyResp.Code, http.StatusOK)
		}
	})

	t.Run("Should return session transcript", func(t *testing.T) {
		transcriptResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/transcript?before_sequence=9&limit=2",
			nil,
		)
		if transcriptResp.Code != http.StatusOK {
			t.Fatalf("transcript status = %d, want %d", transcriptResp.Code, http.StatusOK)
		}
		var payload struct {
			Entries            []transcript.Entry `json:"entries"`
			Epoch              int64              `json:"epoch"`
			Generation         int64              `json:"generation"`
			MaxSequence        int64              `json:"max_sequence"`
			HasOlder           bool               `json:"has_older"`
			NextBeforeSequence int64              `json:"next_before_sequence"`
			Limit              int                `json:"limit"`
		}
		if err := json.Unmarshal(transcriptResp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(transcript response) error = %v", err)
		}
		if payload.Epoch != 1 || payload.Generation != 4 || payload.MaxSequence != 12 ||
			!payload.HasOlder || payload.NextBeforeSequence != 1 || payload.Limit != 2 ||
			len(payload.Entries) != 1 {
			t.Fatalf("transcript payload = %#v, want complete page envelope", payload)
		}
	})

	t.Run("Should reject forward cursors on the backward transcript page endpoint", func(t *testing.T) {
		transcriptResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/transcript?after_sequence=1",
			nil,
		)
		if transcriptResp.Code != http.StatusBadRequest {
			t.Fatalf("transcript status = %d, want %d", transcriptResp.Code, http.StatusBadRequest)
		}
		if !strings.Contains(transcriptResp.Body.String(), "after_sequence is not supported") {
			t.Fatalf("transcript body = %q, want hard-cut cursor error", transcriptResp.Body.String())
		}
	})

	t.Run("Should reject a transcript page outside the session workspace", func(t *testing.T) {
		transcriptResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-other/sessions/sess-a/transcript?before_sequence=9&limit=2",
			nil,
		)
		if transcriptResp.Code != http.StatusNotFound {
			t.Fatalf("foreign transcript status = %d, want %d", transcriptResp.Code, http.StatusNotFound)
		}
	})
}

func TestCreateSessionProviderAuthFailureReturnsDiagnostic(t *testing.T) {
	t.Parallel()

	t.Run("Should return diagnostic for provider auth failure", func(t *testing.T) {
		t.Parallel()

		item := diagnostics.NewItem(
			"provider.codex.auth",
			contract.CodeProviderCLIMissing,
			contract.CategoryProvider,
			"Provider auth status",
			"Provider CLI is not installed or not available on PATH.",
			contract.SeverityError,
			contract.FreshnessLive,
		)
		manager := testutil.StubSessionManager{
			CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
				return nil, acp.WrapFailure(
					store.FailureProviderAuth,
					"provider auth pre-start probe failed",
					diagnostics.NewStructuredError(item, errors.New("missing provider CLI")),
				)
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)

		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{"agent_name":"coder","provider":"codex","workspace":"alpha"}`),
		)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("create status = %d body = %s, want 422", response.Code, response.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(error payload) error = %v", err)
		}
		if payload.Diagnostic == nil {
			t.Fatal("payload.Diagnostic = nil, want provider diagnostic")
		}
		if got, want := payload.Diagnostic.Code, contract.CodeProviderCLIMissing; got != want {
			t.Fatalf("payload.Diagnostic.Code = %q, want %q", got, want)
		}
	})
}

func TestCreateSessionNegotiationFailuresReturnStableDiagnostics(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		code      string
		stage     string
		requested string
	}{
		{
			name:      "Should return model_unavailable",
			code:      acp.NegotiationCodeModelUnavailable,
			stage:     "model",
			requested: "missing-model",
		},
		{
			name:      "Should return reasoning_option_missing",
			code:      acp.NegotiationCodeReasoningOptionMissing,
			stage:     "reasoning effort",
			requested: "max",
		},
		{
			name:      "Should return reasoning_effort_unsupported",
			code:      acp.NegotiationCodeReasoningEffortUnsupported,
			stage:     "reasoning effort",
			requested: "max",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			negotiationErr := &acp.NegotiationError{
				Code:         tc.code,
				Stage:        tc.stage,
				Requested:    tc.requested,
				ValidChoices: []string{"high", "low"},
			}
			item := diagnostics.NewItem(
				"provider.negotiation."+tc.code,
				tc.code,
				contract.CategoryProvider,
				"Provider configuration is unavailable",
				negotiationErr.Error(),
				contract.SeverityError,
				contract.FreshnessLive,
				diagnostics.WithEvidence(map[string]any{
					"requested":     tc.requested,
					"valid_choices": []string{"high", "low"},
				}),
			)
			manager := testutil.StubSessionManager{
				CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
					return nil, diagnostics.NewStructuredError(item, negotiationErr)
				},
			}
			fixture := newHandlerFixture(
				t,
				manager,
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)

			response := performRequest(
				t,
				fixture.Engine,
				http.MethodPost,
				"/sessions",
				[]byte(
					`{"agent_name":"coder","provider":"codex","workspace":"alpha","model":"gpt-5.6-sol","reasoning_effort":"max"}`,
				),
			)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("create status = %d body = %s, want 422", response.Code, response.Body.String())
			}
			var payload contract.ErrorPayload
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal(error payload) error = %v", err)
			}
			if payload.Diagnostic == nil || payload.Diagnostic.Code != tc.code {
				t.Fatalf("payload.Diagnostic = %#v, want code %q", payload.Diagnostic, tc.code)
			}
			if payload.Diagnostic.Evidence["requested"] != tc.requested {
				t.Fatalf("diagnostic evidence = %#v, want requested %q", payload.Diagnostic.Evidence, tc.requested)
			}
		})
	}

	t.Run("Should return reasoning_effort_unsupported for an unknown effort value", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
				t.Fatal("session manager Create() called after request validation failed")
				return nil, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{"agent_name":"coder","provider":"codex","workspace":"alpha","reasoning_effort":"ultra"}`),
		)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("create status = %d body = %s, want 422", response.Code, response.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(error payload) error = %v", err)
		}
		if payload.Diagnostic == nil ||
			payload.Diagnostic.Code != contract.CodeReasoningEffortUnsupported {
			t.Fatalf(
				"payload.Diagnostic = %#v, want code %q",
				payload.Diagnostic,
				contract.CodeReasoningEffortUnsupported,
			)
		}
		if got, want := payload.Diagnostic.Evidence["requested"], "ultra"; got != want {
			t.Fatalf("diagnostic requested evidence = %#v, want %q", got, want)
		}
	})
}

func TestSessionRecapUsesSingleBoundedTranscriptRead(t *testing.T) {
	t.Parallel()

	t.Run("Should serve recap from one bounded transcript read", func(t *testing.T) {
		t.Parallel()

		occurredAt := time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC)
		marker, err := transcript.NewMarker(
			transcript.MarkerPromptInterrupted,
			"Prompt interrupted",
			occurredAt,
			map[string]any{"reason": "operator"},
		)
		if err != nil {
			t.Fatalf("transcript.NewMarker() error = %v", err)
		}
		openMarker, err := transcript.NewMarker(
			transcript.MarkerSessionUnhealthy,
			"Runtime health check failed.",
			occurredAt.Add(-500*time.Millisecond),
			map[string]any{"stall_reason": store.SessionStallReasonProcessUnhealthy},
		)
		if err != nil {
			t.Fatalf("transcript.NewMarker(open) error = %v", err)
		}
		recoveredMarker, err := transcript.NewMarker(
			transcript.MarkerSessionRecovered,
			"Runtime activity recovered.",
			occurredAt.Add(-250*time.Millisecond),
			map[string]any{"stall_reason": store.SessionStallReasonProcessUnhealthy},
		)
		if err != nil {
			t.Fatalf("transcript.NewMarker(recovered) error = %v", err)
		}
		providerMarker, err := transcript.NewMarker(
			transcript.MarkerProviderFailure,
			"Provider failed.",
			occurredAt.Add(250*time.Millisecond),
			map[string]any{"failure_kind": string(store.FailurePrompt)},
		)
		if err != nil {
			t.Fatalf("transcript.NewMarker(provider) error = %v", err)
		}
		preRecoveryProviderMarker, err := transcript.NewMarker(
			transcript.MarkerProviderFailure,
			"Provider auth failed before runtime recovery.",
			occurredAt.Add(-750*time.Millisecond),
			map[string]any{"failure_kind": string(store.FailureProviderAuth)},
		)
		if err != nil {
			t.Fatalf("transcript.NewMarker(preRecoveryProvider) error = %v", err)
		}
		markerEntry := func(id string, sequence int64, eventType string, marker transcript.Marker) transcript.Entry {
			t.Helper()
			normalized := marker.Normalize()
			return transcript.Entry{
				Message: transcript.UIMessage{
					ID:   id,
					Role: transcript.UIRoleSystem,
					Parts: []transcript.UIMessagePart{{
						Type:  "text",
						Text:  normalized.Summary,
						State: "done",
					}},
				},
				Sequence:  sequence,
				EventType: eventType,
				Marker:    &normalized,
			}
		}

		var transcriptPageCalls atomic.Int64
		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return testutil.NewSessionInfo("sess-a"), nil
			},
			TranscriptPageFn: func(
				_ context.Context,
				id string,
				query transcript.PageQuery,
			) (transcript.Page, error) {
				transcriptPageCalls.Add(1)
				if id != "sess-a" {
					t.Fatalf("TranscriptPage() id = %q, want sess-a", id)
				}
				if query.Limit != 500 {
					t.Fatalf("TranscriptPage() query = %#v, want single bounded recap read limit 500", query)
				}
				return transcript.Page{Entries: []transcript.Entry{
					markerEntry(
						"marker-provider-before-recovery",
						1,
						events.TranscriptMarkerCreated,
						preRecoveryProviderMarker,
					),
					markerEntry("marker-open", 2, events.TranscriptMarkerCreated, openMarker),
					markerEntry("marker-recovered", 3, events.TranscriptMarkerCreated, recoveredMarker),
					markerEntry("marker-provider", 4, events.TranscriptMarkerCreated, providerMarker),
					markerEntry("marker-redacted", 5, events.TranscriptMarkerRedacted, marker),
					{
						Message: transcript.UIMessage{
							ID:   "msg-1",
							Role: "assistant",
							Parts: []transcript.UIMessagePart{{
								Type:  "text",
								Text:  "hello",
								State: "done",
							}},
						},
						Sequence: 10,
					},
				}, Generation: 2, MaxSequence: 12}, nil
			},
			EventsFn: func(_ context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error) {
				t.Fatalf("Events() called with id=%q query=%#v; recap must use one transcript read", id, query)
				return nil, nil
			},
			InputQueueFn: func(_ context.Context, id string) (session.InputQueueSummary, error) {
				if id != "sess-a" {
					t.Fatalf("InputQueueSummary() id = %q, want sess-a", id)
				}
				return session.InputQueueSummary{QueueGeneration: 7, PendingInputs: 2}, nil
			},
		}
		observer := testutil.StubObserver{
			QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
				t.Fatalf("QueryEvents() called with query=%#v; recap markers must come from transcript entries", query)
				return nil, nil
			},
		}

		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/recap?limit=1",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("recap status = %d body=%s, want %d", response.Code, response.Body.String(), http.StatusOK)
		}

		var payload contract.SessionRecapResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(recap response) error = %v", err)
		}
		if got, want := len(payload.Recap.RecentMarkers), 5; got != want {
			t.Fatalf("len(recent_markers) = %d, want %d", got, want)
		}
		wantMarkerKinds := []string{
			transcript.MarkerPromptInterrupted,
			transcript.MarkerProviderFailure,
			transcript.MarkerSessionRecovered,
			transcript.MarkerSessionUnhealthy,
			transcript.MarkerProviderFailure,
		}
		for index, want := range wantMarkerKinds {
			if got := payload.Recap.RecentMarkers[index].Kind; got != want {
				t.Fatalf("recent_markers[%d].Kind = %q, want %q", index, got, want)
			}
		}
		if got, want := payload.Recap.PendingInputs, 2; got != want {
			t.Fatalf("pending_inputs = %d, want %d", got, want)
		}
		if got, want := payload.Recap.PendingMarkers, 2; got != want {
			t.Fatalf("pending_markers = %d, want %d", got, want)
		}
		if got, want := len(payload.Recap.RecentMessages), 1; got != want {
			t.Fatalf("len(recent_messages) = %d, want %d", got, want)
		}
		if got, want := payload.Recap.RecentMessages[0].Parts[0].Text, "hello"; got != want {
			t.Fatalf("recent_messages[0].parts[0].text = %q, want %q", got, want)
		}
		if got, want := payload.Recap.Snapshot.QueueGeneration, int64(7); got != want {
			t.Fatalf("snapshot.queue_generation = %d, want %d", got, want)
		}
		if got, want := payload.Recap.Snapshot.EventCursor, int64(12); got != want {
			t.Fatalf("snapshot.event_cursor = %d, want %d", got, want)
		}
		if got, want := payload.Recap.Snapshot.Consistency, "persisted_reads"; got != want {
			t.Fatalf("snapshot.consistency = %q, want %q", got, want)
		}
		if got, want := transcriptPageCalls.Load(), int64(1); got != want {
			t.Fatalf("TranscriptPage() calls = %d, want %d", got, want)
		}
	})
}

func TestSessionUsageEndpoint(t *testing.T) {
	t.Parallel()

	int64Ptr := func(v int64) *int64 { return &v }
	float64Ptr := func(v float64) *float64 { return &v }
	stringPtr := func(v string) *string { return &v }

	t.Run("Should return aggregated token usage summed across agent rows", func(t *testing.T) {
		t.Parallel()

		var usageCalls atomic.Int64
		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return testutil.NewSessionInfo("sess-a"), nil
			},
		}
		observer := testutil.StubObserver{
			QueryTokenStatsFn: func(_ context.Context, query store.TokenStatsQuery) ([]store.TokenStats, error) {
				usageCalls.Add(1)
				if query.SessionID != "sess-a" {
					t.Fatalf("QueryTokenStats() session id = %q, want sess-a", query.SessionID)
				}
				return []store.TokenStats{
					{
						SessionID:    "sess-a",
						AgentName:    "coder",
						InputTokens:  int64Ptr(100),
						OutputTokens: int64Ptr(40),
						TotalTokens:  int64Ptr(140),
						TotalCost:    float64Ptr(0.02),
						CostCurrency: stringPtr("USD"),
						CostStatus:   "estimated",
						CostSource:   "catalog_config",
						TurnCount:    2,
					},
					{
						SessionID:    "sess-a",
						AgentName:    "helper",
						InputTokens:  int64Ptr(10),
						OutputTokens: int64Ptr(5),
						TotalTokens:  int64Ptr(15),
						TotalCost:    float64Ptr(0.01),
						CostCurrency: stringPtr("USD"),
						CostStatus:   "estimated",
						CostSource:   "catalog_config",
						TurnCount:    1,
					},
				}, nil
			},
		}

		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/usage",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("usage status = %d body=%s, want %d", response.Code, response.Body.String(), http.StatusOK)
		}

		var payload contract.SessionUsageResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(usage response) error = %v", err)
		}
		if got := payload.Usage.InputTokens; got == nil || *got != 110 {
			t.Fatalf("usage.input_tokens = %v, want 110", got)
		}
		if got := payload.Usage.OutputTokens; got == nil || *got != 45 {
			t.Fatalf("usage.output_tokens = %v, want 45", got)
		}
		if got := payload.Usage.TotalTokens; got == nil || *got != 155 {
			t.Fatalf("usage.total_tokens = %v, want 155", got)
		}
		if got := payload.Usage.TotalCost; got == nil || *got < 0.0299 || *got > 0.0301 {
			t.Fatalf("usage.total_cost = %v, want ~0.03", got)
		}
		if got, want := payload.Usage.CostCurrency, "USD"; got != want {
			t.Fatalf("usage.cost_currency = %q, want %q", got, want)
		}
		if payload.Usage.CostStatus != "estimated" || payload.Usage.CostSource != "catalog_config" {
			t.Fatalf(
				"usage cost provenance = %q/%q, want estimated/catalog_config",
				payload.Usage.CostStatus,
				payload.Usage.CostSource,
			)
		}
		if got, want := payload.Usage.TurnCount, int64(3); got != want {
			t.Fatalf("usage.turn_count = %d, want %d", got, want)
		}
		if got, want := usageCalls.Load(), int64(1); got != want {
			t.Fatalf("QueryTokenStats() calls = %d, want %d", got, want)
		}
	})

	t.Run("Should omit aggregate cost when token stats use mixed currencies", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return testutil.NewSessionInfo("sess-a"), nil
			},
		}
		observer := testutil.StubObserver{
			QueryTokenStatsFn: func(_ context.Context, query store.TokenStatsQuery) ([]store.TokenStats, error) {
				if query.SessionID != "sess-a" {
					t.Fatalf("QueryTokenStats() session id = %q, want sess-a", query.SessionID)
				}
				return []store.TokenStats{
					{
						SessionID:    "sess-a",
						AgentName:    "coder",
						InputTokens:  int64Ptr(100),
						TotalTokens:  int64Ptr(100),
						TotalCost:    float64Ptr(0.02),
						CostCurrency: stringPtr("USD"),
						CostStatus:   "actual",
						CostSource:   "agent_reported",
						TurnCount:    2,
					},
					{
						SessionID:    "sess-a",
						AgentName:    "helper",
						OutputTokens: int64Ptr(25),
						TotalTokens:  int64Ptr(25),
						TotalCost:    float64Ptr(0.03),
						CostCurrency: stringPtr("EUR"),
						CostStatus:   "actual",
						CostSource:   "agent_reported",
						TurnCount:    1,
					},
				}, nil
			},
		}

		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/usage",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("usage status = %d body=%s, want %d", response.Code, response.Body.String(), http.StatusOK)
		}

		var payload contract.SessionUsageResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(usage response) error = %v", err)
		}
		if got := payload.Usage.TotalTokens; got == nil || *got != 125 {
			t.Fatalf("usage.total_tokens = %v, want 125", got)
		}
		if got := payload.Usage.TotalCost; got != nil {
			t.Fatalf("usage.total_cost = %v, want nil for mixed currencies", got)
		}
		if got := payload.Usage.CostCurrency; got != "" {
			t.Fatalf("usage.cost_currency = %q, want empty for mixed currencies", got)
		}
		if payload.Usage.CostStatus != "unknown" || payload.Usage.CostSource != "none" {
			t.Fatalf(
				"usage cost provenance = %q/%q, want fail-closed unknown/none",
				payload.Usage.CostStatus,
				payload.Usage.CostSource,
			)
		}
		if got, want := payload.Usage.TurnCount, int64(3); got != want {
			t.Fatalf("usage.turn_count = %d, want %d", got, want)
		}
	})

	t.Run("Should return included without fabricating an amount for native CLI usage", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return testutil.NewSessionInfo("sess-a"), nil
			},
		}
		observer := testutil.StubObserver{
			QueryTokenStatsFn: func(context.Context, store.TokenStatsQuery) ([]store.TokenStats, error) {
				return []store.TokenStats{{
					SessionID:   "sess-a",
					AgentName:   "coder",
					TotalTokens: int64Ptr(25),
					CostStatus:  "included",
					CostSource:  "none",
					TurnCount:   1,
				}}, nil
			},
		}

		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/usage",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("usage status = %d body=%s, want %d", response.Code, response.Body.String(), http.StatusOK)
		}

		var payload contract.SessionUsageResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(usage response) error = %v", err)
		}
		if payload.Usage.TotalCost != nil || payload.Usage.CostCurrency != "" ||
			payload.Usage.CostStatus != "included" || payload.Usage.CostSource != "none" {
			t.Fatalf("included usage = %#v, want included/none without amount", payload.Usage)
		}
	})

	t.Run("Should return an empty usage summary when no token stats exist", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return testutil.NewSessionInfo("sess-a"), nil
			},
		}
		observer := testutil.StubObserver{
			QueryTokenStatsFn: func(context.Context, store.TokenStatsQuery) ([]store.TokenStats, error) {
				return nil, nil
			},
		}

		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/usage",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("usage status = %d body=%s, want %d", response.Code, response.Body.String(), http.StatusOK)
		}

		var payload contract.SessionUsageResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(usage response) error = %v", err)
		}
		if payload.Usage.InputTokens != nil || payload.Usage.OutputTokens != nil ||
			payload.Usage.TotalTokens != nil || payload.Usage.TotalCost != nil {
			t.Fatalf("empty usage carried token/cost values: %#v", payload.Usage)
		}
		if got, want := payload.Usage.TurnCount, int64(0); got != want {
			t.Fatalf("usage.turn_count = %d, want %d", got, want)
		}
	})

	t.Run("Should return 404 for an unknown session", func(t *testing.T) {
		t.Parallel()

		var usageCalls atomic.Int64
		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return nil, session.ErrSessionNotFound
			},
		}
		observer := testutil.StubObserver{
			QueryTokenStatsFn: func(context.Context, store.TokenStatsQuery) ([]store.TokenStats, error) {
				usageCalls.Add(1)
				return nil, nil
			},
		}

		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/does-not-exist/usage",
			nil,
		)
		if response.Code != http.StatusNotFound {
			t.Fatalf("usage status = %d body=%s, want %d", response.Code, response.Body.String(), http.StatusNotFound)
		}
		if got := usageCalls.Load(); got != 0 {
			t.Fatalf("QueryTokenStats() calls = %d, want 0 for unknown session", got)
		}
	})
}

func TestLogsEndpointsRequireObserver(t *testing.T) {
	t.Parallel()

	t.Run("Should return service unavailable when observer is missing", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Observer = nil

		for _, path := range []string{
			"/logs?workspace_id=ws-workspace",
			"/logs/stream?workspace_id=ws-workspace",
		} {
			response := performRequest(t, fixture.Engine, http.MethodGet, path, nil)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf(
					"%s status = %d, want %d; body=%s",
					path,
					response.Code,
					http.StatusServiceUnavailable,
					response.Body.String(),
				)
			}
			if !strings.Contains(response.Body.String(), "observer is required") {
				t.Fatalf("%s body = %q, want observer error", path, response.Body.String())
			}
		}
	})
}

func TestBaseHandlersStreamingAndObserveEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should stream session events and expose observe endpoints", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		var sessionCalls atomic.Int32
		var observeCalls atomic.Int32
		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return testutil.NewSessionInfo(id), nil
			},
			EventsFn: func(_ context.Context, id string, _ store.EventQuery) ([]store.SessionEvent, error) {
				switch sessionCalls.Add(1) {
				case 1:
					return []store.SessionEvent{{
						ID:        "ev-1",
						SessionID: id,
						Sequence:  1,
						TurnID:    "turn-1",
						Type:      "agent_message",
						AgentName: "coder",
						Content:   `{"text":"hello"}`,
						Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
					}}, nil
				case 2:
					close(done)
					return []store.SessionEvent{{
						ID:        "ev-2",
						SessionID: id,
						Sequence:  2,
						TurnID:    "turn-1",
						Type:      "done",
						AgentName: "coder",
						Content:   `{"stop_reason":"end_turn"}`,
						Timestamp: time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
					}}, nil
				default:
					return nil, nil
				}
			},
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{testutil.NewSessionInfo("sess-a")}, nil
			},
		}
		observer := testutil.StubObserver{
			QueryEventsFn: func(_ context.Context, _ store.EventSummaryQuery) ([]store.EventSummary, error) {
				call := observeCalls.Add(1)
				ts := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
				switch call {
				case 1:
					return []store.EventSummary{
						{ID: "sum-1", SessionID: "sess-a", Type: "agent_message", AgentName: "coder", Timestamp: ts},
					}, nil
				case 2:
					return []store.EventSummary{
						{
							ID:        "sum-2",
							SessionID: "sess-a",
							Type:      "done",
							AgentName: "coder",
							Timestamp: ts.Add(time.Second),
						},
					}, nil
				default:
					return nil, nil
				}
			},
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", ActiveSessions: 1, Version: "dev"}, nil
			},
		}

		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.SetStreamDone(done)

		streamResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream?frames=raw",
			nil,
		)
		if streamResp.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d", streamResp.Code, http.StatusOK)
		}
		if records := testutil.ParseSSE(t, streamResp.Body.String()); len(records) < 2 {
			t.Fatalf("stream records = %d, want at least 2", len(records))
		}

		observeResp := performRequest(t, fixture.Engine, http.MethodGet, "/logs?workspace_id=ws-workspace", nil)
		if observeResp.Code != http.StatusOK {
			t.Fatalf("observe status = %d, want %d", observeResp.Code, http.StatusOK)
		}

		statusResp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if statusResp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", statusResp.Code, http.StatusOK)
		}

		doctorResp := performRequest(t, fixture.Engine, http.MethodGet, "/doctor", nil)
		if doctorResp.Code != http.StatusOK {
			t.Fatalf("doctor = %d, want %d", doctorResp.Code, http.StatusOK)
		}
	})

	t.Run("Should log stream lifecycle and catch-up metrics", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		close(done)
		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return testutil.NewSessionInfo(id), nil
			},
			EventsFn: func(_ context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error) {
				if id != "sess-a" {
					t.Fatalf("Events() id = %q, want sess-a", id)
				}
				if query.AfterSequence != 3 {
					t.Fatalf("Events() AfterSequence = %d, want 3", query.AfterSequence)
				}
				return []store.SessionEvent{{
					ID:        "event-4",
					SessionID: "sess-a",
					Sequence:  4,
					TurnID:    "turn-4",
					Type:      events.ACPAgentMessage,
					AgentName: "coder",
					Content:   `{"type":"agent_message","content":"hello"}`,
					Timestamp: time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC),
				}}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		var logs bytes.Buffer
		fixture.Handlers.Logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		fixture.Handlers.SetStreamDone(done)

		streamResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream?frames=raw&after_sequence=3",
			nil,
		)
		if streamResp.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d; body=%s", streamResp.Code, http.StatusOK, streamResp.Body.String())
		}
		records := testutil.ParseSSE(t, streamResp.Body.String())
		if got := len(records); got == 0 {
			t.Fatal("stream records = 0, want raw catch-up event")
		}
		logOutput := logs.String()
		for _, want := range []string{
			`"msg":"api: session stream opened"`,
			`"msg":"api: session stream closed"`,
			`"session_id":"sess-a"`,
			`"workspace_id":"ws-workspace"`,
			`"frame_mode":"raw"`,
			`"cursor":3`,
			`"catch_up_batch_size":1`,
			`"active_streams":1`,
		} {
			if !strings.Contains(logOutput, want) {
				t.Fatalf("stream logs missing %s in %s", want, logOutput)
			}
		}
	})

	t.Run("Should skip unused raw event read for transcript snapshot streams", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		close(done)
		var eventsCalls atomic.Int32
		emitted := make([]acp.AgentEvent, 0, 1)
		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				info := testutil.NewSessionInfo(id)
				info.TranscriptEpoch = 3
				return info, nil
			},
			EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
				eventsCalls.Add(1)
				return nil, nil
			},
			TranscriptPageFn: func(_ context.Context, id string, query transcript.PageQuery) (transcript.Page, error) {
				if id != "sess-a" || query.Limit <= 0 || query.BeforeSequence != 0 {
					t.Fatalf("TranscriptPage() call = %q %#v, want bounded tail read", id, query)
				}
				return transcript.Page{
					Entries: []transcript.Entry{{
						StartSequence: 7,
						Sequence:      7,
						Message: transcript.UIMessage{
							ID:   "msg-7",
							Role: transcript.UIRoleAssistant,
							Parts: []transcript.UIMessagePart{{
								Type:  "text",
								Text:  "snapshot",
								State: "done",
							}},
						},
					}},
					Generation:         4,
					MaxSequence:        7,
					HasOlder:           true,
					NextBeforeSequence: 7,
				}, nil
			},
		}
		observer := testutil.StubObserver{
			OnAgentEventFn: func(_ context.Context, sessionID string, event acp.AgentEvent) {
				if sessionID != "sess-a" {
					t.Fatalf("OnAgentEvent() session id = %q, want sess-a", sessionID)
				}
				emitted = append(emitted, event)
			},
		}
		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.SetStreamDone(done)

		streamResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream?frames=transcript",
			nil,
		)
		if streamResp.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d; body=%s", streamResp.Code, http.StatusOK, streamResp.Body.String())
		}
		if got := eventsCalls.Load(); got != 0 {
			t.Fatalf("Events() calls = %d, want 0 for transcript snapshot mode", got)
		}
		records := testutil.ParseSSE(t, streamResp.Body.String())
		if got := len(records); got == 0 {
			t.Fatal("stream records = 0, want snapshot")
		}
		if got := records[0].Event; got != contract.SessionStreamEventTranscriptSnapshot {
			t.Fatalf("first stream event = %q, want %q", got, contract.SessionStreamEventTranscriptSnapshot)
		}
		if got, want := len(emitted), 1; got != want {
			t.Fatalf("emitted events len = %d, want %d", got, want)
		}
		if got := emitted[0].Type; got != events.SessionStreamSnapshotServed {
			t.Fatalf("emitted event type = %q, want %q", got, events.SessionStreamSnapshotServed)
		}
		var eventPayload struct {
			WorkspaceID        string `json:"workspace_id"`
			SessionID          string `json:"session_id"`
			Epoch              int64  `json:"epoch"`
			Generation         int64  `json:"generation"`
			Reset              bool   `json:"reset"`
			Reason             string `json:"reason"`
			MaxSequence        int64  `json:"max_sequence"`
			HasOlder           bool   `json:"has_older"`
			NextBeforeSequence int64  `json:"next_before_sequence"`
		}
		if err := json.Unmarshal(emitted[0].Raw, &eventPayload); err != nil {
			t.Fatalf("json.Unmarshal(snapshot event raw) error = %v raw=%s", err, string(emitted[0].Raw))
		}
		if eventPayload.WorkspaceID != "ws-workspace" ||
			eventPayload.SessionID != "sess-a" ||
			eventPayload.Epoch != 3 ||
			eventPayload.Generation != 4 ||
			eventPayload.Reason != "" ||
			eventPayload.MaxSequence != 7 ||
			!eventPayload.HasOlder ||
			eventPayload.NextBeforeSequence != 7 ||
			eventPayload.Reset {
			t.Fatalf("snapshot served event payload = %#v", eventPayload)
		}
	})

	t.Run("Should drain materialized changes on a valid fenced reconnect", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		close(done)
		var eventsCalls atomic.Int32
		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				// Idle background session: still active, no in-flight turn.
				info := testutil.NewSessionInfo(id)
				info.TranscriptEpoch = 2
				return info, nil
			},
			EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
				// The stale cursor's incremental catch-up is empty for an idle session; snapshot
				// mode must not depend on it, so Events must never be read for the initial frame.
				eventsCalls.Add(1)
				return nil, nil
			},
			TranscriptPageFn: func(context.Context, string, transcript.PageQuery) (transcript.Page, error) {
				t.Fatal("TranscriptPage() should not be called for a valid reconnect")
				return transcript.Page{}, nil
			},
			TranscriptChangesFn: func(
				_ context.Context,
				id string,
				query transcript.ChangeQuery,
			) (transcript.ChangePage, error) {
				if id != "sess-a" || query.AfterSequence != 3 || query.Limit <= 0 {
					t.Fatalf("TranscriptChanges() call = %q %#v, want changes after 3", id, query)
				}
				return transcript.ChangePage{
					Entries: []transcript.Entry{{
						StartSequence: 2,
						Sequence:      6,
						Message: transcript.UIMessage{
							ID:   "repaired-entry",
							Role: transcript.UIRoleAssistant,
						},
					}},
					Generation:  4,
					MaxSequence: 6,
					NextAfter:   6,
				}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.SetStreamDone(done)

		streamResp := testutil.PerformRequestWithHeaders(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream?frames=transcript&epoch=2&generation=4",
			nil,
			map[string]string{"Last-Event-ID": "3"},
		)
		if streamResp.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d; body=%s", streamResp.Code, http.StatusOK, streamResp.Body.String())
		}
		records := testutil.ParseSSE(t, streamResp.Body.String())
		if len(records) == 0 {
			t.Fatal("stream records = 0, want a transcript delta on reconnect")
		}
		if got := records[0].Event; got != contract.SessionStreamEventTranscriptDelta {
			t.Fatalf("first stream event = %q, want %q", got, contract.SessionStreamEventTranscriptDelta)
		}
		var delta contract.TranscriptDeltaPayload
		testutil.DecodeSSEData(t, records[0], &delta)
		if len(delta.Entries) != 1 || delta.Entries[0].StartSequence != 2 {
			t.Fatalf("delta entries = %#v, want repaired historical entry", delta.Entries)
		}
		if delta.Epoch != 2 || delta.Generation != 4 || delta.Cursor != 6 || delta.MaxSequence != 6 {
			t.Fatalf("delta fence/cursor = %#v", delta)
		}
		if records[0].ID != "6" {
			t.Fatalf("delta frame id = %q, want \"6\"", records[0].ID)
		}
		if got := eventsCalls.Load(); got != 0 {
			t.Fatalf("Events() calls = %d, want 0 — snapshot must not depend on incremental catch-up", got)
		}
	})

	t.Run("Should reject negative transcript stream query cursors", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return testutil.NewSessionInfo(id), nil
			},
			EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
				t.Fatal("Events() should not be called for invalid cursor")
				return nil, nil
			},
			TranscriptPageFn: func(context.Context, string, transcript.PageQuery) (transcript.Page, error) {
				t.Fatal("TranscriptPage() should not be called for invalid cursor")
				return transcript.Page{}, nil
			},
			TranscriptChangesFn: func(context.Context, string, transcript.ChangeQuery) (transcript.ChangePage, error) {
				t.Fatal("TranscriptChanges() should not be called for invalid cursor")
				return transcript.ChangePage{}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)

		streamResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream?after_sequence=-1",
			nil,
		)
		if streamResp.Code != http.StatusBadRequest {
			t.Fatalf(
				"stream negative cursor status = %d, want %d; body=%s",
				streamResp.Code,
				http.StatusBadRequest,
				streamResp.Body.String(),
			)
		}
		if !strings.Contains(streamResp.Body.String(), "invalid event after sequence -1") {
			t.Fatalf("stream negative cursor body = %q, want invalid cursor message", streamResp.Body.String())
		}
	})

	t.Run("Should reject negative transcript stream last event ids", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return testutil.NewSessionInfo(id), nil
			},
			EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
				t.Fatal("Events() should not be called for invalid Last-Event-ID")
				return nil, nil
			},
			TranscriptPageFn: func(context.Context, string, transcript.PageQuery) (transcript.Page, error) {
				t.Fatal("TranscriptPage() should not be called for invalid Last-Event-ID")
				return transcript.Page{}, nil
			},
			TranscriptChangesFn: func(context.Context, string, transcript.ChangeQuery) (transcript.ChangePage, error) {
				t.Fatal("TranscriptChanges() should not be called for invalid Last-Event-ID")
				return transcript.ChangePage{}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)

		streamResp := testutil.PerformRequestWithHeaders(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream",
			nil,
			map[string]string{"Last-Event-ID": "-1"},
		)
		if streamResp.Code != http.StatusBadRequest {
			t.Fatalf(
				"stream negative Last-Event-ID status = %d, want %d; body=%s",
				streamResp.Code,
				http.StatusBadRequest,
				streamResp.Body.String(),
			)
		}
		if !strings.Contains(streamResp.Body.String(), "Last-Event-ID") {
			t.Fatalf("stream negative Last-Event-ID body = %q, want header message", streamResp.Body.String())
		}
	})

	t.Run("Should reject the removed transcript replay query", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		for _, query := range []string{"replay=snapshot", "replay="} {
			streamResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/sessions/sess-a/stream?frames=transcript&"+query,
				nil,
			)
			if streamResp.Code != http.StatusBadRequest {
				t.Fatalf("stream %s status = %d, want %d", query, streamResp.Code, http.StatusBadRequest)
			}
			if !strings.Contains(streamResp.Body.String(), "replay query is no longer supported") {
				t.Fatalf("stream %s body = %q, want removed-query error", query, streamResp.Body.String())
			}
		}
	})

	t.Run("Should reject empty transcript reconnect fences", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		for _, name := range []string{"epoch", "generation"} {
			streamResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/sessions/sess-a/stream?frames=transcript&"+name+"=",
				nil,
			)
			if streamResp.Code != http.StatusBadRequest {
				t.Fatalf("empty %s status = %d, want %d", name, streamResp.Code, http.StatusBadRequest)
			}
			if !strings.Contains(streamResp.Body.String(), "value is required when supplied") {
				t.Fatalf("empty %s body = %q, want required-value error", name, streamResp.Body.String())
			}
		}
	})

	t.Run("Should preserve raw stream initial replay limit and unbound polling", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		var calls atomic.Int32
		initialSeen := make(chan store.EventQuery, 1)
		pollSeen := make(chan store.EventQuery, 1)
		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return testutil.NewSessionInfo(id), nil
			},
			EventsFn: func(_ context.Context, _ string, query store.EventQuery) ([]store.SessionEvent, error) {
				switch calls.Add(1) {
				case 1:
					initialSeen <- query
					return []store.SessionEvent{{
						Sequence:  11,
						Type:      "agent_message",
						TurnID:    "turn-1",
						AgentName: "coder",
						Content:   "initial",
					}}, nil
				default:
					pollSeen <- query
					close(done)
					return nil, nil
				}
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.SetStreamDone(done)

		streamResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream?frames=raw&limit=2",
			nil,
		)
		if streamResp.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d; body=%s", streamResp.Code, http.StatusOK, streamResp.Body.String())
		}
		initialQuery := <-initialSeen
		if initialQuery.Limit != 2 {
			t.Fatalf("initial Events() limit = %d, want 2", initialQuery.Limit)
		}
		pollQuery := <-pollSeen
		if pollQuery.Limit != 0 {
			t.Fatalf("poll Events() limit = %d, want 0", pollQuery.Limit)
		}
		if pollQuery.AfterSequence != 11 {
			t.Fatalf("poll Events() after_sequence = %d, want 11", pollQuery.AfterSequence)
		}
	})
}

func TestBaseHandlersAgentEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should serve agent read endpoints and surface loader failures", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		testutil.WriteAgentDef(t, fixture.HomePaths, "coder")
		testutil.WriteAgentDef(t, fixture.HomePaths, "onboarding")

		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/coder", nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get agent status = %d, want %d", getResp.Code, http.StatusOK)
		}

		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list agents status = %d, want %d", listResp.Code, http.StatusOK)
		}
		var listed contract.AgentsResponse
		if err := json.Unmarshal(listResp.Body.Bytes(), &listed); err != nil {
			t.Fatalf("json.Unmarshal(list agents) error = %v", err)
		}
		if len(listed.Agents) != 2 || listed.Agents[0].Name != "coder" || listed.Agents[1].Name != "onboarding" {
			t.Fatalf("listed agents = %#v, want coder and onboarding", listed.Agents)
		}
		onboardingResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/onboarding", nil)
		if onboardingResp.Code != http.StatusOK {
			t.Fatalf("get onboarding status = %d, want %d", onboardingResp.Code, http.StatusOK)
		}

		fixture.Handlers.AgentLoader = func(string, aghconfig.HomePaths) (aghconfig.AgentDef, error) {
			return aghconfig.AgentDef{}, errors.New("boom")
		}
		missingResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/missing", nil)
		if missingResp.Code != http.StatusInternalServerError {
			t.Fatalf("missing agent status = %d, want %d", missingResp.Code, http.StatusInternalServerError)
		}
	})
}

func TestBaseHandlersCreateAgentEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("Should create a global AGENT.md definition", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		syncer := &recordingAgentDefinitionSync{}
		fixture.Handlers.AgentDefinitionSync = syncer
		body := mustJSON(t, contract.CreateAgentRequest{
			Scope: contract.AgentCreateScopeGlobal,
			Agent: contract.CreateAgentPayload{
				Name:            "pricing_strategist",
				Provider:        "claude",
				Model:           "claude-sonnet-5",
				ReasoningEffort: "max",
				Tools:           []string{"builtin__shell"},
				Permissions:     contract.SettingsPermissionModeApproveReads,
				CategoryPath:    []string{"Strategy"},
				Skills:          &contract.CreateAgentSkillsConfig{Disabled: []string{"legacy-skill"}},
				Prompt:          "You own pricing strategy.",
			},
		})

		resp := performRequest(t, fixture.Engine, http.MethodPost, "/agents", body)
		if resp.Code != http.StatusCreated {
			t.Fatalf(
				"create global agent status = %d, want %d; body=%s",
				resp.Code,
				http.StatusCreated,
				resp.Body.String(),
			)
		}
		var payload contract.AgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Agent.Name != "pricing_strategist" || payload.Agent.Provider != "claude" ||
			payload.Agent.ReasoningEffort != "max" {
			t.Fatalf("created agent payload = %#v, want pricing strategist", payload.Agent)
		}
		path := filepath.Join(
			fixture.HomePaths.AgentsDir,
			"pricing_strategist",
			aghconfig.AgentDefinitionFileName,
		)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat(created AGENT.md) error = %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("created AGENT.md mode = %v, want 0600", info.Mode().Perm())
		}
		loaded, err := aghconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(created AGENT.md) error = %v", err)
		}
		if loaded.Permissions != string(contract.SettingsPermissionModeApproveReads) ||
			loaded.ReasoningEffort != "max" ||
			len(loaded.Skills.Disabled) != 1 || loaded.Skills.Disabled[0] != "legacy-skill" {
			t.Fatalf("loaded agent = %#v, want permissions and disabled skill", loaded)
		}
		if syncer.calls != 1 {
			t.Fatalf("Sync() calls = %d, want 1", syncer.calls)
		}
	})

	t.Run("Should create a workspace AGENT.md definition", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if ref != "alpha" {
						t.Fatalf("Resolve() ref = %q, want alpha", ref)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      "ws-alpha",
							Name:    "alpha",
							RootDir: workspaceRoot,
						},
						WorkspaceID: "ws-alpha",
						Config: aghconfig.Config{
							Defaults: aghconfig.DefaultsConfig{Provider: "codex"},
						},
					}, nil
				},
			},
			nil,
			nil,
		)
		body := mustJSON(t, contract.CreateAgentRequest{
			Scope:     contract.AgentCreateScopeWorkspace,
			Workspace: "alpha",
			Agent: contract.CreateAgentPayload{
				Name:   "qa_operator",
				Prompt: "Stress test the workspace.",
			},
		})

		resp := performRequest(t, fixture.Engine, http.MethodPost, "/agents", body)
		if resp.Code != http.StatusCreated {
			t.Fatalf(
				"create workspace agent status = %d, want %d; body=%s",
				resp.Code,
				http.StatusCreated,
				resp.Body.String(),
			)
		}
		path := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"qa_operator",
			aghconfig.AgentDefinitionFileName,
		)
		loaded, err := aghconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(workspace AGENT.md) error = %v", err)
		}
		if loaded.Name != "qa_operator" || loaded.Provider != "" {
			t.Fatalf("loaded workspace agent = %#v, want inherited provider left unauthored", loaded)
		}
		var payload contract.AgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Agent.EffectiveRuntime == nil || payload.Agent.EffectiveRuntime.Provider != "codex" {
			t.Fatalf("created agent effective runtime = %#v, want codex", payload.Agent.EffectiveRuntime)
		}
	})

	t.Run("Should reject an exactly reserved name without creating a directory", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/agents",
			mustJSON(t, contract.CreateAgentRequest{
				Scope: contract.AgentCreateScopeGlobal,
				Agent: contract.CreateAgentPayload{
					Name: "coordinator", Provider: "codex", Prompt: "Do not persist.",
				},
			}),
		)
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("reserved create status = %d, want 422; body=%s", resp.Code, resp.Body.String())
		}
		var payload contract.ErrorPayload
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Diagnostic == nil || payload.Diagnostic.Code != contract.CodeAgentNameReserved {
			t.Fatalf("reserved create payload = %#v, want agent_name_reserved", payload)
		}
		path := filepath.Join(fixture.HomePaths.AgentsDir, "coordinator")
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(reserved agent directory) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should allow a name whose prefix matches a reserved identity", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.AgentDefinitionSync = &recordingAgentDefinitionSync{}
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/agents",
			mustJSON(t, contract.CreateAgentRequest{
				Scope: contract.AgentCreateScopeGlobal,
				Agent: contract.CreateAgentPayload{
					Name: "coordinator-helper", Provider: "codex", Prompt: "Persist exactly.",
				},
			}),
		)
		if resp.Code != http.StatusCreated {
			t.Fatalf("prefixed create status = %d, want 201; body=%s", resp.Code, resp.Body.String())
		}
		path := filepath.Join(
			fixture.HomePaths.AgentsDir,
			"coordinator-helper",
			aghconfig.AgentDefinitionFileName,
		)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("os.Stat(prefixed agent) error = %v", err)
		}
	})

	t.Run("Should reject an inherited runtime when the target scope has no provider default", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config = aghconfig.Config{}
		body := mustJSON(t, contract.CreateAgentRequest{
			Scope: contract.AgentCreateScopeGlobal,
			Agent: contract.CreateAgentPayload{
				Name:   "missing_runtime",
				Prompt: "Require a target default.",
			},
		})

		resp := performRequest(t, fixture.Engine, http.MethodPost, "/agents", body)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("create status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body)
		}
		if responseBody := resp.Body.String(); !strings.Contains(responseBody, "agent runtime cannot be resolved") ||
			!strings.Contains(responseBody, "provider is required") {
			t.Fatalf("create body = %s, want inherited-runtime cause", responseBody)
		}
	})

	t.Run("Should reject an update whose target runtime cannot resolve", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config = aghconfig.Config{}
		path := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		current, err := aghconfig.CreateAgentDefFile(path, aghconfig.AgentDefinitionDraft{
			Name: "coder", Prompt: "Before.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile() error = %v", err)
		}
		digest, err := aghconfig.AgentDefinitionDigest(current)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest() error = %v", err)
		}
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/agents/coder",
			mustJSON(t, contract.UpdateAgentRequest{
				Agent:          contract.CreateAgentPayload{Name: "coder", Prompt: "After."},
				ExpectedDigest: digest,
			}),
		)
		if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "provider is required") {
			t.Fatalf("update status = %d, body=%s; want unresolved runtime", resp.Code, resp.Body.String())
		}
	})

	t.Run("Should reject a duplicate whose target runtime cannot resolve", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config = aghconfig.Config{}
		path := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		if _, err := aghconfig.CreateAgentDefFile(path, aghconfig.AgentDefinitionDraft{
			Name: "coder", Prompt: "Source.",
		}, false); err != nil {
			t.Fatalf("CreateAgentDefFile() error = %v", err)
		}
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/agents/coder/duplicate",
			mustJSON(t, contract.DuplicateAgentRequest{Name: "reviewer"}),
		)
		if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "provider is required") {
			t.Fatalf("duplicate status = %d, body=%s; want unresolved runtime", resp.Code, resp.Body.String())
		}
	})

	t.Run("Should roll back the durable definition when catalog sync fails", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		syncErr := errors.New("sync unavailable")
		syncer := &recordingAgentDefinitionSync{errs: []error{syncErr, nil}}
		fixture.Handlers.AgentDefinitionSync = syncer
		path := filepath.Join(
			fixture.HomePaths.AgentsDir,
			"rollback_agent",
			aghconfig.AgentDefinitionFileName,
		)
		body := mustJSON(t, contract.CreateAgentRequest{
			Scope: contract.AgentCreateScopeGlobal,
			Agent: contract.CreateAgentPayload{
				Name:     "rollback_agent",
				Provider: "codex",
				Prompt:   "Rollback on sync failure.",
			},
		})

		resp := performRequest(t, fixture.Engine, http.MethodPost, "/agents", body)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("create status = %d, want %d; body=%s", resp.Code, http.StatusInternalServerError, resp.Body)
		}
		var payload contract.ErrorPayload
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if !strings.Contains(payload.Error, syncErr.Error()) {
			t.Fatalf("create error = %q, want sync cause", payload.Error)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(rolled back definition) error = %v, want os.ErrNotExist", err)
		}
		if syncer.calls != 2 {
			t.Fatalf("Sync() calls = %d, want initial sync plus reconciliation", syncer.calls)
		}
	})

	t.Run("Should reject duplicate AGENT.md definitions", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		body := mustJSON(t, contract.CreateAgentRequest{
			Scope: contract.AgentCreateScopeGlobal,
			Agent: contract.CreateAgentPayload{
				Name:     "duplicate",
				Provider: "codex",
				Prompt:   "First definition.",
			},
		})
		first := performRequest(t, fixture.Engine, http.MethodPost, "/agents", body)
		if first.Code != http.StatusCreated {
			t.Fatalf(
				"initial create status = %d, want %d; body=%s",
				first.Code,
				http.StatusCreated,
				first.Body.String(),
			)
		}
		second := performRequest(t, fixture.Engine, http.MethodPost, "/agents", body)
		if second.Code != http.StatusConflict {
			t.Fatalf(
				"duplicate create status = %d, want %d; body=%s",
				second.Code,
				http.StatusConflict,
				second.Body.String(),
			)
		}
	})

	t.Run("Should reject invalid simple AGENT.md fields", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			req  contract.CreateAgentRequest
		}{
			{
				name: "invalid permission",
				req: contract.CreateAgentRequest{
					Scope: contract.AgentCreateScopeGlobal,
					Agent: contract.CreateAgentPayload{
						Name:        "bad_permission",
						Provider:    "codex",
						Permissions: "maybe",
						Prompt:      "Prompt.",
					},
				},
			},
			{
				name: "invalid reasoning effort",
				req: contract.CreateAgentRequest{
					Scope: contract.AgentCreateScopeGlobal,
					Agent: contract.CreateAgentPayload{
						Name:            "bad_reasoning",
						Provider:        "codex",
						ReasoningEffort: "ultra",
						Prompt:          "Prompt.",
					},
				},
			},
			{
				name: "invalid tool",
				req: contract.CreateAgentRequest{
					Scope: contract.AgentCreateScopeGlobal,
					Agent: contract.CreateAgentPayload{
						Name:     "bad_tool",
						Provider: "codex",
						Tools:    []string{"shell"},
						Prompt:   "Prompt.",
					},
				},
			},
			{
				name: "invalid category",
				req: contract.CreateAgentRequest{
					Scope: contract.AgentCreateScopeGlobal,
					Agent: contract.CreateAgentPayload{
						Name:         "bad_category",
						Provider:     "codex",
						CategoryPath: []string{"Engineering/Platform"},
						Prompt:       "Prompt.",
					},
				},
			},
		}
		for _, tc := range tests {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				fixture := newHandlerFixture(
					t,
					testutil.StubSessionManager{},
					testutil.StubObserver{},
					testutil.StubWorkspaceService{},
					nil,
					nil,
				)
				resp := performRequest(t, fixture.Engine, http.MethodPost, "/agents", mustJSON(t, tc.req))
				if resp.Code != http.StatusBadRequest {
					t.Fatalf(
						"invalid create status = %d, want %d; body=%s",
						resp.Code,
						http.StatusBadRequest,
						resp.Body.String(),
					)
				}
			})
		}
	})

	t.Run("Should map unresolved workspaces through workspace errors", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				},
			},
			nil,
			nil,
		)
		body := mustJSON(t, contract.CreateAgentRequest{
			Scope:     contract.AgentCreateScopeWorkspace,
			Workspace: "missing",
			Agent: contract.CreateAgentPayload{
				Name:     "operator",
				Provider: "codex",
				Prompt:   "Operate.",
			},
		})
		resp := performRequest(t, fixture.Engine, http.MethodPost, "/agents", body)
		if resp.Code != http.StatusNotFound {
			t.Fatalf(
				"missing workspace status = %d, want %d; body=%s",
				resp.Code,
				http.StatusNotFound,
				resp.Body.String(),
			)
		}
	})
}

func TestBaseHandlersAgentDefinitionMutations(t *testing.T) {
	t.Parallel()

	t.Run("Should replace a global definition with CAS and a fresh digest", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		path := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		current, err := aghconfig.CreateAgentDefFile(path, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Before.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile() error = %v", err)
		}
		digest, err := aghconfig.AgentDefinitionDigest(current)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest() error = %v", err)
		}
		syncer := &recordingAgentDefinitionSync{}
		fixture.Handlers.AgentDefinitionSync = syncer
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/agents/coder",
			mustJSON(t, contract.UpdateAgentRequest{
				Agent: contract.CreateAgentPayload{
					Name: "coder", Provider: "codex", Model: "gpt-5", Prompt: "After.",
					Permissions: contract.SettingsPermissionModeApproveReads,
				},
				ExpectedDigest: digest,
			}),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("update status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.AgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Agent.Prompt != "After." || payload.Agent.Model != "gpt-5" ||
			payload.Agent.Origin != contract.AgentOriginGlobal || payload.Agent.DefinitionDigest == digest {
			t.Fatalf("updated payload = %#v", payload.Agent)
		}
		if payload.Agent.EffectiveRuntime == nil ||
			payload.Agent.EffectiveRuntime.Provider != "codex" ||
			payload.Agent.EffectiveRuntime.Model != "gpt-5" {
			t.Fatalf("updated effective runtime = %#v", payload.Agent.EffectiveRuntime)
		}
		if syncer.calls != 1 {
			t.Fatalf("Sync() calls = %d, want 1", syncer.calls)
		}
		loaded, err := aghconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(updated) error = %v", err)
		}
		if loaded.Prompt != "After." || loaded.Permissions != string(contract.SettingsPermissionModeApproveReads) {
			t.Fatalf("updated definition = %#v", loaded)
		}
	})

	t.Run("Should update a catalog-projected global definition at its effective source", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		path := filepath.Join(
			t.TempDir(),
			"extensions",
			"dev-cycle",
			"agents",
			"code_implementer",
			aghconfig.AgentDefinitionFileName,
		)
		current, err := aghconfig.CreateAgentDefFile(path, aghconfig.AgentDefinitionDraft{
			Name:         "code_implementer",
			Provider:     "codex",
			CategoryPath: []string{"Compozy"},
			Prompt:       "Before.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile() error = %v", err)
		}
		digest, err := aghconfig.AgentDefinitionDigest(current)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest() error = %v", err)
		}
		fixture.Handlers.AgentCatalog = stubAgentCatalog{
			get: map[string]aghconfig.AgentDef{"code_implementer": current},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/agents/code_implementer",
			mustJSON(t, contract.UpdateAgentRequest{
				Agent: contract.CreateAgentPayload{
					Name:         "code_implementer",
					Provider:     "codex",
					Model:        "gpt-5",
					CategoryPath: []string{"Compozy"},
					Prompt:       "After.",
				},
				ExpectedDigest: digest,
			}),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("update status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.AgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Agent.Model != "gpt-5" || payload.Agent.Prompt != "After." ||
			!slices.Equal(payload.Agent.CategoryPath, []string{"Compozy"}) {
			t.Fatalf("updated catalog agent = %#v", payload.Agent)
		}
		loaded, err := aghconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(effective source) error = %v", err)
		}
		if loaded.Model != "gpt-5" || loaded.Prompt != "After." {
			t.Fatalf("effective source definition = %#v", loaded)
		}
	})

	t.Run("Should preserve MCP sidecar ownership when updating the authored definition", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		path := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		if _, err := aghconfig.CreateAgentDefFile(path, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Before.",
		}, false); err != nil {
			t.Fatalf("CreateAgentDefFile() error = %v", err)
		}
		mcpPath := filepath.Join(filepath.Dir(path), aghconfig.MCPJSONName)
		if err := os.WriteFile(mcpPath, []byte(`{"mcpServers":{"github":{"command":"sidecar"}}}`), 0o600); err != nil {
			t.Fatalf("os.WriteFile(mcp.json) error = %v", err)
		}
		current, err := aghconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile() error = %v", err)
		}
		digest, err := aghconfig.AgentDefinitionDigest(current)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest() error = %v", err)
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/agents/coder",
			mustJSON(t, contract.UpdateAgentRequest{
				Agent:          contract.CreateAgentPayload{Name: "coder", Provider: "codex", Prompt: "After."},
				ExpectedDigest: digest,
			}),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("update status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(AGENT.md) error = %v", err)
		}
		authored, err := aghconfig.ParseAgentDef(contents)
		if err != nil {
			t.Fatalf("ParseAgentDef(AGENT.md) error = %v", err)
		}
		if len(authored.MCPServers) != 0 {
			t.Fatalf("authored MCP servers = %#v, want sidecar-only ownership", authored.MCPServers)
		}
		loaded, err := aghconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(updated) error = %v", err)
		}
		if len(loaded.MCPServers) != 1 || loaded.MCPServers[0].Name != "github" {
			t.Fatalf("runtime MCP servers = %#v, want merged github sidecar", loaded.MCPServers)
		}
	})

	t.Run("Should reject stale digest name mismatch invalid permission and unknown agent", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		path := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		current, err := aghconfig.CreateAgentDefFile(path, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Before.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile() error = %v", err)
		}
		digest, err := aghconfig.AgentDefinitionDigest(current)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest() error = %v", err)
		}
		tests := []struct {
			name       string
			route      string
			req        contract.UpdateAgentRequest
			wantStatus int
		}{
			{
				name: "stale digest", route: "/agents/coder", wantStatus: http.StatusConflict,
				req: contract.UpdateAgentRequest{
					Agent:          contract.CreateAgentPayload{Name: "coder", Provider: "codex", Prompt: "Changed."},
					ExpectedDigest: "stale",
				},
			},
			{
				name: "name mismatch", route: "/agents/coder", wantStatus: http.StatusBadRequest,
				req: contract.UpdateAgentRequest{
					Agent: contract.CreateAgentPayload{
						Name: "reviewer", Provider: "codex", Prompt: "Changed.",
					},
					ExpectedDigest: digest,
				},
			},
			{
				name: "invalid permission", route: "/agents/coder", wantStatus: http.StatusBadRequest,
				req: contract.UpdateAgentRequest{
					Agent: contract.CreateAgentPayload{
						Name: "coder", Provider: "codex", Prompt: "Changed.", Permissions: "maybe",
					},
					ExpectedDigest: digest,
				},
			},
			{
				name: "unknown agent", route: "/agents/missing", wantStatus: http.StatusNotFound,
				req: contract.UpdateAgentRequest{
					Agent:          contract.CreateAgentPayload{Name: "missing", Provider: "codex", Prompt: "Changed."},
					ExpectedDigest: digest,
				},
			},
		}
		for _, tc := range tests {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				resp := performRequest(t, fixture.Engine, http.MethodPut, tc.route, mustJSON(t, tc.req))
				if resp.Code != tc.wantStatus {
					t.Fatalf("update status = %d, want %d; body=%s", resp.Code, tc.wantStatus, resp.Body.String())
				}
			})
		}
	})

	t.Run("Should reject reserved update and duplicate targets without touching authored state", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		path := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		current, err := aghconfig.CreateAgentDefFile(path, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Original.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile() error = %v", err)
		}
		digest, err := aghconfig.AgentDefinitionDigest(current)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest() error = %v", err)
		}

		update := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/agents/coder",
			mustJSON(t, contract.UpdateAgentRequest{
				Agent: contract.CreateAgentPayload{
					Name: "dreaming-curator", Provider: "codex", Prompt: "Replacement.",
				},
				ExpectedDigest: digest,
			}),
		)
		if update.Code != http.StatusUnprocessableEntity {
			t.Fatalf("reserved update status = %d, want 422; body=%s", update.Code, update.Body.String())
		}
		var updatePayload contract.ErrorPayload
		decodeJSON(t, update.Body.Bytes(), &updatePayload)
		if updatePayload.Diagnostic == nil || updatePayload.Diagnostic.Code != contract.CodeAgentNameReserved {
			t.Fatalf("reserved update payload = %#v, want agent_name_reserved", updatePayload)
		}
		unchanged, err := aghconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(original) error = %v", err)
		}
		unchangedDigest, err := aghconfig.AgentDefinitionDigest(unchanged)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest(unchanged) error = %v", err)
		}
		if unchanged.Prompt != "Original." || unchangedDigest != digest {
			t.Fatalf(
				"reserved update changed agent/digest = %#v/%q, want original/%q",
				unchanged,
				unchangedDigest,
				digest,
			)
		}

		duplicate := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/agents/coder/duplicate",
			mustJSON(t, contract.DuplicateAgentRequest{Name: "coordinator"}),
		)
		if duplicate.Code != http.StatusUnprocessableEntity {
			t.Fatalf("reserved duplicate status = %d, want 422; body=%s", duplicate.Code, duplicate.Body.String())
		}
		var duplicatePayload contract.ErrorPayload
		decodeJSON(t, duplicate.Body.Bytes(), &duplicatePayload)
		if duplicatePayload.Diagnostic == nil || duplicatePayload.Diagnostic.Code != contract.CodeAgentNameReserved {
			t.Fatalf("reserved duplicate payload = %#v, want agent_name_reserved", duplicatePayload)
		}
		reservedPath := filepath.Join(fixture.HomePaths.AgentsDir, "coordinator")
		if _, err := os.Stat(reservedPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(reserved duplicate directory) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should delete only the effective workspace definition and disclose the global twin", func(t *testing.T) {
		t.Parallel()
		workspaceRoot := t.TempDir()
		workspacePath := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"coder",
			aghconfig.AgentDefinitionFileName,
		)
		workspaceAgent, err := aghconfig.CreateAgentDefFile(workspacePath, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Workspace.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(workspace) error = %v", err)
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "ws-registry", RootDir: workspaceRoot},
						WorkspaceID: "ws-identity",
						Agents:      []aghconfig.AgentDef{workspaceAgent},
					}, nil
				},
			},
			nil,
			nil,
		)
		globalPath := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		if _, err := aghconfig.CreateAgentDefFile(globalPath, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Global.",
		}, false); err != nil {
			t.Fatalf("CreateAgentDefFile(global) error = %v", err)
		}
		soulPurger := &recordingSoulHistoryPurger{}
		heartbeatPurger := &recordingHeartbeatHistoryPurger{}
		fixture.Handlers.SoulHistoryPurger = soulPurger
		fixture.Handlers.HeartbeatHistoryPurger = heartbeatPurger
		resp := performRequest(t, fixture.Engine, http.MethodDelete, "/agents/coder?workspace=alpha", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("delete status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.DeleteAgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Origin != contract.AgentOriginWorkspace || payload.UnshadowedOrigin != contract.AgentOriginGlobal {
			t.Fatalf("delete payload = %#v", payload)
		}
		if _, err := os.Stat(workspacePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(workspace definition) error = %v, want os.ErrNotExist", err)
		}
		if _, err := os.Stat(globalPath); err != nil {
			t.Fatalf("os.Stat(global twin) error = %v", err)
		}
		if soulPurger.workspaceID != "ws-registry" || soulPurger.name != "coder" ||
			soulPurger.sourcePath != workspacePath {
			t.Fatalf("soul purge = %#v", soulPurger)
		}
		if heartbeatPurger.workspaceID != "ws-registry" || heartbeatPurger.sourcePath != workspacePath {
			t.Fatalf("heartbeat purge = %#v", heartbeatPurger)
		}
		repeat := performRequest(t, fixture.Engine, http.MethodDelete, "/agents/coder?workspace=alpha", nil)
		if repeat.Code != http.StatusNotFound {
			t.Fatalf(
				"repeat delete status = %d, want %d; body=%s",
				repeat.Code,
				http.StatusNotFound,
				repeat.Body.String(),
			)
		}
	})

	t.Run("Should preserve the workspace definition when history purge fails", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		workspacePath := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"coder",
			aghconfig.AgentDefinitionFileName,
		)
		workspaceAgent, err := aghconfig.CreateAgentDefFile(workspacePath, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Workspace.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(workspace) error = %v", err)
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-alpha", RootDir: workspaceRoot},
						Agents:    []aghconfig.AgentDef{workspaceAgent},
					}, nil
				},
			},
			nil,
			nil,
		)
		purgeErr := errors.New("history unavailable")
		fixture.Handlers.SoulHistoryPurger = &recordingSoulHistoryPurger{err: purgeErr}
		fixture.Handlers.HeartbeatHistoryPurger = &recordingHeartbeatHistoryPurger{}
		syncer := &recordingAgentDefinitionSync{}
		fixture.Handlers.AgentDefinitionSync = syncer

		resp := performRequest(t, fixture.Engine, http.MethodDelete, "/agents/coder?workspace=alpha", nil)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("delete status = %d, want %d; body=%s", resp.Code, http.StatusInternalServerError, resp.Body)
		}
		if _, err := os.Stat(workspacePath); err != nil {
			t.Fatalf("os.Stat(preserved definition) error = %v", err)
		}
		if syncer.calls != 0 {
			t.Fatalf("Sync() calls = %d, want 0 before successful purge", syncer.calls)
		}
	})

	t.Run("Should leave unshadowed origin empty when no global twin exists", func(t *testing.T) {
		t.Parallel()
		workspaceRoot := t.TempDir()
		workspacePath := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"coder",
			aghconfig.AgentDefinitionFileName,
		)
		workspaceAgent, err := aghconfig.CreateAgentDefFile(workspacePath, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Workspace.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(workspace) error = %v", err)
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "ws-registry", RootDir: workspaceRoot},
						WorkspaceID: "ws-identity",
						Agents:      []aghconfig.AgentDef{workspaceAgent},
					}, nil
				},
			},
			nil,
			nil,
		)
		fixture.Handlers.SoulHistoryPurger = &recordingSoulHistoryPurger{}
		fixture.Handlers.HeartbeatHistoryPurger = &recordingHeartbeatHistoryPurger{}

		resp := performRequest(t, fixture.Engine, http.MethodDelete, "/agents/coder?workspace=alpha", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("delete status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.DeleteAgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Origin != contract.AgentOriginWorkspace || payload.UnshadowedOrigin != "" {
			t.Fatalf("delete payload = %#v, want workspace origin without unshadow disclosure", payload)
		}
	})

	t.Run("Should delete a workspace definition when the global twin is invalid", func(t *testing.T) {
		t.Parallel()
		workspaceRoot := t.TempDir()
		workspacePath := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"coder",
			aghconfig.AgentDefinitionFileName,
		)
		workspaceAgent, err := aghconfig.CreateAgentDefFile(workspacePath, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Workspace.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(workspace) error = %v", err)
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-registry", RootDir: workspaceRoot},
						Agents:    []aghconfig.AgentDef{workspaceAgent},
					}, nil
				},
			},
			nil,
			nil,
		)
		globalPath := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		if err := os.MkdirAll(filepath.Dir(globalPath), 0o700); err != nil {
			t.Fatalf("os.MkdirAll(global) error = %v", err)
		}
		if err := os.WriteFile(globalPath, []byte("not valid frontmatter\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(global) error = %v", err)
		}
		fixture.Handlers.SoulHistoryPurger = &recordingSoulHistoryPurger{}
		fixture.Handlers.HeartbeatHistoryPurger = &recordingHeartbeatHistoryPurger{}

		resp := performRequest(t, fixture.Engine, http.MethodDelete, "/agents/coder?workspace=alpha", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("delete status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.DeleteAgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.UnshadowedOrigin != "" {
			t.Fatalf("unshadowed origin = %q, want empty for invalid global twin", payload.UnshadowedOrigin)
		}
		if _, err := os.Stat(workspacePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(workspace definition) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run(
		"Should duplicate the server-side directory with overrides and reject an existing target",
		func(t *testing.T) {
			t.Parallel()
			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			sourcePath := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
			if _, err := aghconfig.CreateAgentDefFile(sourcePath, aghconfig.AgentDefinitionDraft{
				Name: "coder", Provider: "codex", Prompt: "Source.",
			}, false); err != nil {
				t.Fatalf("CreateAgentDefFile(source) error = %v", err)
			}
			if err := os.WriteFile(
				filepath.Join(filepath.Dir(sourcePath), "SOUL.md"),
				[]byte("soul\n"),
				0o600,
			); err != nil {
				t.Fatalf("WriteFile(SOUL.md) error = %v", err)
			}
			mcpBody := []byte(`{"mcpServers":{"github":{"command":"sidecar"}}}`)
			if err := os.WriteFile(
				filepath.Join(filepath.Dir(sourcePath), aghconfig.MCPJSONName),
				mcpBody,
				0o600,
			); err != nil {
				t.Fatalf("WriteFile(mcp.json) error = %v", err)
			}
			request := contract.DuplicateAgentRequest{
				Name: "reviewer",
				Overrides: &contract.DuplicateAgentOverrides{
					Provider:        "claude",
					Command:         "claude --print",
					Model:           "claude-opus",
					ReasoningEffort: contract.ReasoningEffort("high"),
					Tools:           []string{"agh__skill_view"},
					Toolsets:        []string{"agh__catalog"},
					DenyTools:       []string{"agh__delete_*"},
					Permissions:     contract.SettingsPermissionModeApproveReads,
					CategoryPath:    []string{"Engineering", "Review"},
					Skills:          &contract.CreateAgentSkillsConfig{Disabled: []string{"legacy"}},
					Prompt:          "Review.",
				},
			}
			resp := performRequest(t, fixture.Engine, http.MethodPost, "/agents/coder/duplicate", mustJSON(t, request))
			if resp.Code != http.StatusCreated {
				t.Fatalf("duplicate status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
			}
			var payload contract.AgentResponse
			decodeJSON(t, resp.Body.Bytes(), &payload)
			if payload.Agent.Name != "reviewer" || payload.Agent.Provider != "claude" ||
				payload.Agent.Command != "claude --print" || payload.Agent.Model != "claude-opus" ||
				payload.Agent.ReasoningEffort != contract.ReasoningEffort("high") ||
				!slices.Equal(payload.Agent.Tools, []string{"agh__skill_view"}) ||
				!slices.Equal(payload.Agent.Toolsets, []string{"agh__catalog"}) ||
				!slices.Equal(payload.Agent.DenyTools, []string{"agh__delete_*"}) ||
				payload.Agent.Permissions != string(contract.SettingsPermissionModeApproveReads) ||
				!slices.Equal(payload.Agent.CategoryPath, []string{"Engineering", "Review"}) ||
				payload.Agent.Skills == nil || !slices.Equal(payload.Agent.Skills.Disabled, []string{"legacy"}) ||
				payload.Agent.Prompt != "Review." || payload.Agent.Origin != contract.AgentOriginGlobal {
				t.Fatalf("duplicate payload = %#v", payload.Agent)
			}
			if payload.Agent.EffectiveRuntime == nil ||
				payload.Agent.EffectiveRuntime.Provider != "claude" ||
				payload.Agent.EffectiveRuntime.Model != "claude-opus" ||
				payload.Agent.EffectiveRuntime.ReasoningEffort != contract.ReasoningEffort("high") {
				t.Fatalf("duplicate effective runtime = %#v", payload.Agent.EffectiveRuntime)
			}
			targetDir := filepath.Join(fixture.HomePaths.AgentsDir, "reviewer")
			soulBody, err := os.ReadFile(filepath.Join(targetDir, "SOUL.md"))
			if err != nil {
				t.Fatalf("ReadFile(duplicate SOUL.md) error = %v", err)
			}
			if string(soulBody) != "soul\n" {
				t.Fatalf("duplicate SOUL.md = %q", soulBody)
			}
			duplicatedMCP, err := os.ReadFile(filepath.Join(targetDir, aghconfig.MCPJSONName))
			if err != nil {
				t.Fatalf("ReadFile(duplicate mcp.json) error = %v", err)
			}
			if !bytes.Equal(duplicatedMCP, mcpBody) {
				t.Fatalf("duplicate mcp.json = %q, want %q", duplicatedMCP, mcpBody)
			}
			definitionBody, err := os.ReadFile(filepath.Join(targetDir, aghconfig.AgentDefinitionFileName))
			if err != nil {
				t.Fatalf("ReadFile(duplicate AGENT.md) error = %v", err)
			}
			authored, err := aghconfig.ParseAgentDef(definitionBody)
			if err != nil {
				t.Fatalf("ParseAgentDef(duplicate AGENT.md) error = %v", err)
			}
			if len(authored.MCPServers) != 0 {
				t.Fatalf("duplicate authored MCP servers = %#v, want sidecar-only ownership", authored.MCPServers)
			}
			conflict := performRequest(
				t,
				fixture.Engine,
				http.MethodPost,
				"/agents/coder/duplicate",
				mustJSON(t, request),
			)
			if conflict.Code != http.StatusConflict {
				t.Fatalf(
					"duplicate conflict status = %d, want %d; body=%s",
					conflict.Code,
					http.StatusConflict,
					conflict.Body.String(),
				)
			}
		},
	)

	t.Run("Should default duplicate scope to the workspace source origin", func(t *testing.T) {
		t.Parallel()
		workspaceRoot := t.TempDir()
		sourcePath := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"coder",
			aghconfig.AgentDefinitionFileName,
		)
		source, err := aghconfig.CreateAgentDefFile(sourcePath, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Workspace source.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(source) error = %v", err)
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
						Agents:    []aghconfig.AgentDef{source},
					}, nil
				},
			},
			nil,
			nil,
		)
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/agents/coder/duplicate",
			mustJSON(t, contract.DuplicateAgentRequest{Name: "reviewer", Workspace: "alpha"}),
		)
		if resp.Code != http.StatusCreated {
			t.Fatalf("duplicate status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
		}
		var payload contract.AgentResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Agent.Origin != contract.AgentOriginWorkspace || payload.Agent.WorkspaceID != "ws-1" {
			t.Fatalf("duplicate payload = %#v, want ws-1 workspace origin", payload.Agent)
		}
		targetPath := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"reviewer",
			aghconfig.AgentDefinitionFileName,
		)
		if _, err := os.Stat(targetPath); err != nil {
			t.Fatalf("os.Stat(workspace duplicate) error = %v", err)
		}

		explicitResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/agents/coder/duplicate",
			mustJSON(t, contract.DuplicateAgentRequest{
				Name: "operator", Scope: contract.AgentCreateScopeWorkspace, Workspace: "alpha",
			}),
		)
		if explicitResp.Code != http.StatusCreated {
			t.Fatalf(
				"explicit duplicate status = %d, want %d; body=%s",
				explicitResp.Code,
				http.StatusCreated,
				explicitResp.Body.String(),
			)
		}
		var explicitPayload contract.AgentResponse
		decodeJSON(t, explicitResp.Body.Bytes(), &explicitPayload)
		if explicitPayload.Agent.Origin != contract.AgentOriginWorkspace ||
			explicitPayload.Agent.WorkspaceID != "ws-1" {
			t.Fatalf("explicit duplicate payload = %#v, want ws-1 workspace origin", explicitPayload.Agent)
		}
		explicitTargetPath := filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.AgentsDirName,
			"operator",
			aghconfig.AgentDefinitionFileName,
		)
		if _, err := os.Stat(explicitTargetPath); err != nil {
			t.Fatalf("os.Stat(explicit workspace duplicate) error = %v", err)
		}
	})

	t.Run("Should reject invalid duplicate scopes before writing", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		sourcePath := filepath.Join(fixture.HomePaths.AgentsDir, "coder", aghconfig.AgentDefinitionFileName)
		if _, err := aghconfig.CreateAgentDefFile(sourcePath, aghconfig.AgentDefinitionDraft{
			Name: "coder", Provider: "codex", Prompt: "Source.",
		}, false); err != nil {
			t.Fatalf("CreateAgentDefFile(source) error = %v", err)
		}
		requests := []contract.DuplicateAgentRequest{
			{Name: "workspace-copy", Scope: contract.AgentCreateScopeWorkspace},
			{Name: "invalid-copy", Scope: contract.AgentCreateScope("invalid")},
		}
		for _, request := range requests {
			t.Run("Should reject "+request.Name, func(t *testing.T) {
				t.Parallel()
				resp := performRequest(
					t,
					fixture.Engine,
					http.MethodPost,
					"/agents/coder/duplicate",
					mustJSON(t, request),
				)
				if resp.Code != http.StatusBadRequest {
					t.Fatalf(
						"duplicate status = %d, want %d; body=%s",
						resp.Code,
						http.StatusBadRequest,
						resp.Body.String(),
					)
				}
			})
		}
	})

	t.Run("Should reject unknown fields in mutation bodies", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		tests := []struct {
			name   string
			method string
			path   string
			body   []byte
		}{
			{
				name:   "update",
				method: http.MethodPut,
				path:   "/agents/coder",
				body: []byte(`{
					"agent":{"name":"coder","provider":"codex","prompt":"Changed."},
					"expected_digest":"digest",
					"unexpected":true
				}`),
			},
			{
				name:   "duplicate",
				method: http.MethodPost,
				path:   "/agents/coder/duplicate",
				body:   []byte(`{"name":"copy","unexpected":true}`),
			},
		}
		for _, tc := range tests {
			t.Run("Should strictly decode "+tc.name, func(t *testing.T) {
				t.Parallel()
				resp := performRequest(t, fixture.Engine, tc.method, tc.path, tc.body)
				if resp.Code != http.StatusBadRequest {
					t.Fatalf(
						"strict decode status = %d, want %d; body=%s",
						resp.Code,
						http.StatusBadRequest,
						resp.Body.String(),
					)
				}
			})
		}
	})
}

type recordingAgentDefinitionSync struct {
	calls int
	err   error
	errs  []error
}

func (s *recordingAgentDefinitionSync) Sync(context.Context) error {
	s.calls++
	if s.calls <= len(s.errs) {
		return s.errs[s.calls-1]
	}
	return s.err
}

type recordingSoulHistoryPurger struct {
	workspaceID string
	name        string
	sourcePath  string
	err         error
}

func (p *recordingSoulHistoryPurger) PurgeAgentHistory(
	_ context.Context,
	ref soul.WorkspaceRef,
	name string,
	sourcePath string,
) error {
	p.workspaceID = ref.WorkspaceID
	p.name = name
	p.sourcePath = sourcePath
	return p.err
}

type recordingHeartbeatHistoryPurger struct {
	workspaceID string
	name        string
	sourcePath  string
	err         error
}

func (p *recordingHeartbeatHistoryPurger) PurgeAgentHistory(
	_ context.Context,
	ref heartbeat.WorkspaceRef,
	name string,
	sourcePath string,
) error {
	p.workspaceID = ref.WorkspaceID
	p.name = name
	p.sourcePath = sourcePath
	return p.err
}

func TestBaseHandlersAgentCatalogEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should expose agent catalog lists, lookups, and error mappings", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.AgentCatalog = stubAgentCatalog{
			agents: []aghconfig.AgentDef{
				{Name: "zeta", Provider: "codex", Prompt: "Zeta prompt"},
				{
					Name:     "alpha",
					Provider: "codex",
					Skills:   aghconfig.AgentSkillsConfig{Disabled: []string{"legacy"}},
					Prompt:   "Alpha prompt",
				},
				{Name: "onboarding", Provider: "codex", Prompt: "Onboarding prompt"},
			},
			get: map[string]aghconfig.AgentDef{
				"alpha": {
					Name:     "alpha",
					Provider: "codex",
					Skills:   aghconfig.AgentSkillsConfig{Disabled: []string{"legacy"}},
					Prompt:   "Alpha prompt",
				},
				"onboarding": {
					Name:     "onboarding",
					Provider: "codex",
					Prompt:   "Onboarding prompt",
				},
			},
		}

		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list agent catalog status = %d, want %d", listResp.Code, http.StatusOK)
		}
		var listed contract.AgentsResponse
		if err := json.Unmarshal(listResp.Body.Bytes(), &listed); err != nil {
			t.Fatalf("json.Unmarshal(list agents) error = %v", err)
		}
		if len(listed.Agents) != 3 || listed.Agents[0].Name != "alpha" ||
			listed.Agents[1].Name != "onboarding" || listed.Agents[2].Name != "zeta" {
			t.Fatalf("listed agents = %#v, want alpha, onboarding, zeta", listed.Agents)
		}
		if listed.Agents[0].Origin != contract.AgentOriginGlobal || listed.Agents[0].DefinitionDigest == "" ||
			listed.Agents[0].Skills == nil || len(listed.Agents[0].Skills.Disabled) != 1 {
			t.Fatalf("listed alpha read shape = %#v", listed.Agents[0])
		}

		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/alpha", nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get agent catalog status = %d, want %d", getResp.Code, http.StatusOK)
		}
		onboardingResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/onboarding", nil)
		if onboardingResp.Code != http.StatusOK {
			t.Fatalf("get onboarding catalog status = %d, want %d", onboardingResp.Code, http.StatusOK)
		}

		fixture.Handlers.AgentCatalog = stubAgentCatalog{getErr: os.ErrNotExist}
		missingResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/missing", nil)
		if missingResp.Code != http.StatusNotFound {
			t.Fatalf("get missing catalog agent status = %d, want %d", missingResp.Code, http.StatusNotFound)
		}
		var missingPayload contract.ErrorPayload
		if err := json.Unmarshal(missingResp.Body.Bytes(), &missingPayload); err != nil {
			t.Fatalf("json.Unmarshal(missing catalog error) error = %v; body=%s", err, missingResp.Body.String())
		}
		if !strings.Contains(missingPayload.Error, "file does not exist") {
			t.Fatalf("missing catalog error = %q, want file-missing message", missingPayload.Error)
		}

		fixture.Handlers.AgentCatalog = stubAgentCatalog{listErr: os.ErrNotExist}
		missingListResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents", nil)
		if missingListResp.Code != http.StatusOK {
			t.Fatalf("list missing catalog status = %d, want %d", missingListResp.Code, http.StatusOK)
		}
		var missingList contract.AgentsResponse
		if err := json.Unmarshal(missingListResp.Body.Bytes(), &missingList); err != nil {
			t.Fatalf("json.Unmarshal(missing list agents) error = %v", err)
		}
		if len(missingList.Agents) != 0 {
			t.Fatalf("missing catalog agents = %#v, want empty list", missingList.Agents)
		}

		fixture.Handlers.AgentCatalog = stubAgentCatalog{listErr: errors.New("catalog unavailable")}
		errorResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents", nil)
		if errorResp.Code != http.StatusInternalServerError {
			t.Fatalf("list catalog error status = %d, want %d", errorResp.Code, http.StatusInternalServerError)
		}
	})
}

func TestBaseHandlersWorkspaceAgentEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should list and inspect workspace resolved agents", func(t *testing.T) {
		t.Parallel()

		const workspaceRef = "alpha"
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if ref != workspaceRef {
						t.Fatalf("Resolve() ref = %q, want %q", ref, workspaceRef)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-1", Name: workspaceRef},
						Agents: []aghconfig.AgentDef{
							{Name: "founder", Provider: "codex", Prompt: "Lead the startup."},
							{Name: "onboarding", Provider: "codex", Prompt: "Operator onboarding."},
							{Name: "qa", Provider: "codex", Prompt: "Stress test the release."},
						},
						AgentDiagnostics: []workspacepkg.AgentDiagnostic{{
							Name:      "broken",
							Path:      "/workspace/.agh/agents/broken/AGENT.md",
							ErrorKind: "frontmatter.missing",
							Message:   "config: missing YAML frontmatter",
						}},
					}, nil
				},
			},
			nil,
			nil,
		)
		fixture.Handlers.AgentCatalog = stubAgentCatalog{
			agents: []aghconfig.AgentDef{
				{Name: "extension-agent", Provider: "codex", Prompt: "Projected by extension."},
				{Name: "onboarding", Provider: "codex", Prompt: "Projected onboarding."},
			},
		}

		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents?workspace="+workspaceRef, nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf(
				"list workspace agents status = %d, want %d; body = %s",
				listResp.Code,
				http.StatusOK,
				listResp.Body.String(),
			)
		}
		var listed contract.AgentsResponse
		if err := json.Unmarshal(listResp.Body.Bytes(), &listed); err != nil {
			t.Fatalf("json.Unmarshal(list workspace agents) error = %v", err)
		}
		if got, want := len(listed.Agents), 5; got != want {
			t.Fatalf("len(workspace agents) = %d, want %d: %#v", got, want, listed.Agents)
		}
		if listed.Agents[0].Name != "broken" ||
			listed.Agents[1].Name != "extension-agent" ||
			listed.Agents[2].Name != "founder" ||
			listed.Agents[3].Name != "onboarding" ||
			listed.Agents[4].Name != "qa" {
			t.Fatalf(
				"workspace agent order = %#v, want broken, extension-agent, founder, onboarding, qa",
				listed.Agents,
			)
		}
		if len(listed.Agents[0].Diagnostics) != 1 ||
			listed.Agents[0].Diagnostics[0].ErrorKind != "frontmatter.missing" {
			t.Fatalf("workspace malformed agent diagnostics = %#v, want frontmatter.missing", listed.Agents[0])
		}
		if listed.Agents[0].Origin != contract.AgentOriginWorkspace || listed.Agents[0].WorkspaceID != "ws-1" ||
			listed.Agents[1].Origin != contract.AgentOriginGlobal || listed.Agents[1].WorkspaceID != "" ||
			listed.Agents[2].Origin != contract.AgentOriginWorkspace || listed.Agents[2].WorkspaceID != "ws-1" {
			t.Fatalf(
				"workspace agent origins = %#v, want workspace diagnostics/agents and global catalog additions",
				listed.Agents,
			)
		}

		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/founder?workspace="+workspaceRef, nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf(
				"get workspace agent status = %d, want %d; body = %s",
				getResp.Code,
				http.StatusOK,
				getResp.Body.String(),
			)
		}
		var got contract.AgentResponse
		if err := json.Unmarshal(getResp.Body.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(get workspace agent) error = %v", err)
		}
		if got.Agent.Name != "founder" || got.Agent.Provider != "codex" ||
			got.Agent.Origin != contract.AgentOriginWorkspace || got.Agent.WorkspaceID != "ws-1" {
			t.Fatalf("get workspace agent = %#v, want founder/codex", got.Agent)
		}
		onboardingResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/agents/onboarding?workspace="+workspaceRef,
			nil,
		)
		if onboardingResp.Code != http.StatusOK {
			t.Fatalf(
				"get workspace onboarding status = %d, want %d; body = %s",
				onboardingResp.Code,
				http.StatusOK,
				onboardingResp.Body.String(),
			)
		}

		missingResp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/missing?workspace="+workspaceRef, nil)
		if missingResp.Code != http.StatusNotFound {
			t.Fatalf(
				"get missing workspace agent status = %d, want %d; body = %s",
				missingResp.Code,
				http.StatusNotFound,
				missingResp.Body.String(),
			)
		}
		if !strings.Contains(missingResp.Body.String(), "not available in workspace") {
			t.Fatalf("get missing workspace agent body = %s, want not available message", missingResp.Body.String())
		}
	})

	t.Run("Should filter and page the fleet with exact backend session metrics", func(t *testing.T) {
		t.Parallel()

		const workspaceRef = "alpha"
		lastActivityAt := time.Date(2026, 7, 12, 9, 30, 0, 0, time.UTC)
		manager := testutil.StubSessionManager{
			MetricsByAgentFn: func(
				_ context.Context,
				workspaceID string,
			) (map[string]session.AgentSessionMetrics, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("AggregateSessionsByAgent() workspace = %q, want ws-1", workspaceID)
				}
				return map[string]session.AgentSessionMetrics{
					"alpha": {
						Total:          2,
						Active:         1,
						Failed:         1,
						RuntimeSeconds: 4_200,
						LastActivityAt: lastActivityAt,
					},
					"beta":  {Total: 1},
					"gamma": {Total: 3, Active: 2},
					"coordinator": {
						Total:  99,
						Active: 99,
					},
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if ref != workspaceRef {
						t.Fatalf("Resolve() ref = %q, want %q", ref, workspaceRef)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: "ws-1", Name: workspaceRef},
						Agents: []aghconfig.AgentDef{
							{Name: "alpha", Provider: "codex", CategoryPath: []string{"Release", "Backend"}},
							{Name: "beta", Provider: "codex", CategoryPath: []string{"Release", "Backend"}},
							{Name: "gamma", Provider: "codex", CategoryPath: []string{"Operations"}},
						},
					}, nil
				},
			},
			nil,
			nil,
		)

		firstResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/agents/catalog?workspace=alpha&q=a&status=active&limit=1",
			nil,
		)
		if firstResp.Code != http.StatusOK {
			t.Fatalf("first agent catalog status = %d; body=%s", firstResp.Code, firstResp.Body.String())
		}
		var first contract.AgentCatalogResponse
		decodeJSON(t, firstResp.Body.Bytes(), &first)
		if len(first.Agents) != 1 || first.Agents[0].Agent.Name != "alpha" ||
			first.Agents[0].Sessions == nil || first.Agents[0].Sessions.Active != 1 ||
			first.Agents[0].Sessions.Total != 2 || first.Agents[0].Sessions.Failed != 1 ||
			first.Agents[0].Sessions.RuntimeSeconds != 4_200 ||
			first.Agents[0].Sessions.LastActivityAt == nil ||
			!first.Agents[0].Sessions.LastActivityAt.Equal(lastActivityAt) {
			t.Fatalf("first agent catalog = %#v, want alpha with 1/2 sessions", first)
		}
		if first.Page.Total != 2 || !first.Page.HasMore || first.Page.NextCursor == "" ||
			!first.SessionsAvailable || first.Facets.Total != 3 || first.Facets.Active != 2 ||
			first.Facets.Idle != 1 || !slices.Equal(
			first.Facets.Categories,
			[]string{"Operations", "Release / Backend"},
		) {
			t.Fatalf("first agent catalog metadata = %#v", first)
		}

		exactResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/agents/catalog?workspace=alpha&name=beta&limit=1",
			nil,
		)
		var exact contract.AgentCatalogResponse
		decodeJSON(t, exactResp.Body.Bytes(), &exact)
		if exactResp.Code != http.StatusOK || len(exact.Agents) != 1 ||
			exact.Agents[0].Agent.Name != "beta" || exact.Page.Total != 1 || exact.Page.HasMore {
			t.Fatalf("exact-name agent catalog = status %d payload %#v", exactResp.Code, exact)
		}

		secondResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/agents/catalog?workspace=alpha&q=a&status=active&limit=1&cursor="+
				first.Page.NextCursor,
			nil,
		)
		var second contract.AgentCatalogResponse
		decodeJSON(t, secondResp.Body.Bytes(), &second)
		if secondResp.Code != http.StatusOK || len(second.Agents) != 1 ||
			second.Agents[0].Agent.Name != "gamma" || second.Page.HasMore {
			t.Fatalf("second agent catalog = status %d payload %#v", secondResp.Code, second)
		}

		mismatch := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/agents/catalog?workspace=alpha&status=idle&cursor="+first.Page.NextCursor,
			nil,
		)
		if mismatch.Code != http.StatusBadRequest {
			t.Fatalf("mismatched agent cursor status = %d, want %d", mismatch.Code, http.StatusBadRequest)
		}

		fixture.Handlers.Sessions = testutil.StubSessionManager{
			MetricsByAgentFn: func(context.Context, string) (map[string]session.AgentSessionMetrics, error) {
				return nil, errors.New("session catalog unavailable")
			},
		}
		partialResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/agents/catalog?workspace=alpha&status=active",
			nil,
		)
		var partial contract.AgentCatalogResponse
		decodeJSON(t, partialResp.Body.Bytes(), &partial)
		if partialResp.Code != http.StatusOK || partial.SessionsAvailable || len(partial.Agents) != 3 {
			t.Fatalf("partial agent catalog = status %d payload %#v", partialResp.Code, partial)
		}
		for _, item := range partial.Agents {
			if item.Sessions != nil {
				t.Fatalf("partial agent %q sessions = %#v, want nil", item.Agent.Name, item.Sessions)
			}
		}
		if partial.Facets.Active != 0 || partial.Facets.Idle != 0 {
			t.Fatalf("partial agent catalog facets = %#v, want no invented session status", partial.Facets)
		}
	})
}

type stubAgentCatalog struct {
	agents  []aghconfig.AgentDef
	get     map[string]aghconfig.AgentDef
	listErr error
	getErr  error
}

var _ core.AgentCatalog = stubAgentCatalog{}

func (s stubAgentCatalog) ListAgents(context.Context) ([]core.AgentCatalogEntry, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	entries := make([]core.AgentCatalogEntry, 0, len(s.agents))
	for _, agent := range s.agents {
		entries = append(entries, core.AgentCatalogEntry{
			Def:    aghconfig.CloneAgentDef(agent),
			Origin: contract.AgentOriginGlobal,
		})
	}
	return entries, nil
}

func (s stubAgentCatalog) GetAgent(_ context.Context, name string) (core.AgentCatalogEntry, error) {
	if s.getErr != nil {
		return core.AgentCatalogEntry{}, s.getErr
	}
	agent, ok := s.get[name]
	if !ok {
		return core.AgentCatalogEntry{}, os.ErrNotExist
	}
	return core.AgentCatalogEntry{
		Def:    aghconfig.CloneAgentDef(agent),
		Origin: contract.AgentOriginGlobal,
	}, nil
}

func TestDaemonStatusIncludesNetworkDiagnosticsWithoutCredentials(t *testing.T) {
	t.Parallel()

	t.Run("Should include network diagnostics in daemon status without leaking credentials", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{{ID: "sess-1"}}, nil
			},
		}
		observer := testutil.StubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", ActiveSessions: 1, Version: "dev"}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			StatusFn: func(context.Context) (*network.Status, error) {
				return &network.Status{
					Enabled:    true,
					Status:     network.StatusActive,
					LocalPeers: 1,
					Channels:   3,
				}, nil
			},
		}

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("daemon status = %d, want %d", resp.Code, http.StatusOK)
		}

		var payload struct {
			Daemon contract.DaemonStatusPayload `json:"daemon"`
		}
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Daemon.Network == nil {
			t.Fatal("daemon network payload = nil, want diagnostics")
		}
		if got, want := payload.Daemon.Network.LocalPeers, 1; got != want {
			t.Fatalf("daemon network Live participants = %d, want %d", got, want)
		}
		if got, want := payload.Daemon.Network.Channels, 3; got != want {
			t.Fatalf("daemon network channels = %d, want %d", got, want)
		}
		bodyLower := strings.ToLower(resp.Body.String())
		for _, forbidden := range []string{
			"token=",
			"claim_token",
			"agh_claim_",
			"authorization: bearer",
			"pkce_verifier",
			"oauth_code",
			"access_token",
			"refresh_token",
			"secret_binding",
		} {
			if strings.Contains(bodyLower, forbidden) {
				t.Fatalf("daemon status leaked credentials (%s): %s", forbidden, resp.Body.String())
			}
		}
	})

	t.Run("Should report enabled network as unavailable when status cannot be collected", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{{ID: "sess-1"}}, nil
			},
		}
		observer := testutil.StubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", ActiveSessions: 1, Version: "dev"}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = nil

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/doctor", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("doctor status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.DoctorPayload
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(doctor) error = %v", err)
		}
		for _, item := range payload.Items {
			if item.ID != "doctor.network.status" {
				continue
			}
			if item.Code != contract.CodeNetworkUnavailable || item.Severity != contract.SeverityWarn {
				t.Fatalf("network diagnostic = %#v, want unavailable warning", item)
			}
			return
		}
		t.Fatalf("doctor items = %#v, want network diagnostic", payload.Items)
	})

	t.Run("Should keep doctor status ok for informational diagnostics", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{{ID: "sess-1"}}, nil
			},
		}
		observer := testutil.StubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", ActiveSessions: 1, Version: "dev"}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = false

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/doctor", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("doctor status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.DoctorPayload
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(doctor) error = %v", err)
		}
		for _, item := range payload.Items {
			if item.ID == "doctor.network.status" &&
				item.Code == contract.CodeNetworkDisabled &&
				item.Severity == contract.SeverityInfo {
				return
			}
		}
		t.Fatalf("doctor items = %#v, want disabled network info diagnostic", payload.Items)
	})
}

func TestDaemonStatusProjectsSubprocessHealth(t *testing.T) {
	t.Run("Should project workspace-scoped redacted subprocess health", func(t *testing.T) {
		t.Parallel()

		manager := subprocessHealthSessionManager{
			StubSessionManager: testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{
						{ID: "sess-health-a", WorkspaceID: "ws-a", State: session.StateActive},
						{ID: "sess-health-b", WorkspaceID: "ws-b", State: session.StateActive},
					}, nil
				},
			},
			snapshots: []session.SubprocessHealthSnapshot{
				{
					SessionID:   "sess-health-a",
					WorkspaceID: "ws-a",
					AgentName:   "coder-a",
					Health: subprocess.HealthState{
						Healthy:             false,
						Message:             "api_key=super-secret probe failed",
						LastCheckedAt:       time.Date(2026, 7, 16, 10, 30, 0, 0, time.UTC),
						ConsecutiveFailures: 3,
					},
				},
				{
					SessionID:   "sess-health-b",
					WorkspaceID: "ws-b",
					AgentName:   "coder-b",
					Health: subprocess.HealthState{
						Healthy:       true,
						LastCheckedAt: time.Date(2026, 7, 16, 10, 30, 0, 0, time.UTC),
					},
				},
			},
		}
		observer := testutil.StubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", ActiveSessions: 2, Version: "dev"}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			manager.StubSessionManager,
			observer,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Sessions = manager

		statusResponse := performRequest(t, fixture.Engine, http.MethodGet, "/status?workspace_id=ws-a", nil)
		if statusResponse.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", statusResponse.Code, http.StatusOK, statusResponse.Body.String())
		}
		var statusPayload contract.StatusPayload
		decodeJSON(t, statusResponse.Body.Bytes(), &statusPayload)
		if statusPayload.SubprocessHealth.Status != "degraded" ||
			statusPayload.SubprocessHealth.Monitored != 1 ||
			statusPayload.SubprocessHealth.Unhealthy != 1 ||
			len(statusPayload.SubprocessHealth.Sessions) != 1 ||
			statusPayload.SubprocessHealth.Sessions[0].SessionID != "sess-health-a" {
			t.Fatalf(
				"subprocess health status = %#v, want workspace-scoped degradation",
				statusPayload.SubprocessHealth,
			)
		}
		if strings.Contains(statusPayload.SubprocessHealth.Sessions[0].Reason, "super-secret") {
			t.Fatalf("subprocess health reason leaked secret: %q", statusPayload.SubprocessHealth.Sessions[0].Reason)
		}

		doctorResponse := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/doctor?only=runtime.subprocess_health",
			nil,
		)
		if doctorResponse.Code != http.StatusOK {
			t.Fatalf("doctor = %d, want %d; body=%s", doctorResponse.Code, http.StatusOK, doctorResponse.Body.String())
		}
		var doctorPayload contract.DoctorPayload
		decodeJSON(t, doctorResponse.Body.Bytes(), &doctorPayload)
		if got, want := len(doctorPayload.Items), 1; got != want {
			t.Fatalf("doctor item count = %d, want %d", got, want)
		}
		if doctorPayload.Items[0].ID != "runtime.subprocess_health" ||
			doctorPayload.Items[0].Severity != contract.SeverityWarn ||
			strings.Contains(doctorPayload.Items[0].Message, "super-secret") {
			t.Fatalf("doctor subprocess item = %#v, want redacted degradation", doctorPayload.Items[0])
		}
	})
}

func TestDaemonStatusProjectsWorkspaceSkills(t *testing.T) {
	t.Run("Should project skills from the requested workspace", func(t *testing.T) {
		t.Parallel()

		var registryWorkspaceID string
		registry := &testutil.StubSkillsRegistry{
			ForWorkspaceFn: func(
				_ context.Context,
				resolved *workspacepkg.ResolvedWorkspace,
			) ([]*skills.Skill, error) {
				if resolved != nil {
					registryWorkspaceID = resolved.WorkspaceID
				}
				return []*skills.Skill{testSkill()}, nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(
				_ context.Context,
				ref string,
			) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-a" {
					t.Fatalf("Resolve() ref = %q, want ws-a", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      "ws-a",
						RootDir: "/workspaces/ws-a",
					},
					WorkspaceID: "ws-a",
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			workspaces,
			nil,
			nil,
		)
		fixture.Handlers.SkillsRegistry = registry

		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/status?workspace_id=ws-a",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusOK,
				response.Body.String(),
			)
		}
		if registryWorkspaceID != "ws-a" {
			t.Fatalf("skill registry workspace = %q, want ws-a", registryWorkspaceID)
		}

		var payload contract.StatusPayload
		decodeJSON(t, response.Body.Bytes(), &payload)
		if payload.Skills.DiscoveredCount != 1 {
			t.Fatalf("status.skills = %#v, want one workspace skill", payload.Skills)
		}
	})
}

type subprocessHealthSessionManager struct {
	testutil.StubSessionManager
	snapshots []session.SubprocessHealthSnapshot
}

func (m subprocessHealthSessionManager) SubprocessHealthSnapshots() []session.SubprocessHealthSnapshot {
	return append([]session.SubprocessHealthSnapshot(nil), m.snapshots...)
}

func TestDaemonDrainProjectsStatusAndDoctor(t *testing.T) {
	t.Parallel()

	manager := testutil.StubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{testutil.NewSessionInfo("sess-admitted")}, nil
		},
	}
	observer := testutil.StubObserver{
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{Status: "ok", ActiveSessions: 1, Version: "dev"}, nil
		},
	}
	fixture := newHandlerFixture(t, manager, observer, testutil.StubWorkspaceService{}, nil, nil)
	controller := &testDrainController{}
	fixture.Handlers.DrainController = controller

	for range 2 {
		response := performRequest(t, fixture.Engine, http.MethodPost, "/drain", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("drain status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.DrainStatusResponse
		decodeJSON(t, response.Body.Bytes(), &payload)
		if payload.State != contract.DrainStateDraining {
			t.Fatalf("drain state = %q, want %q", payload.State, contract.DrainStateDraining)
		}
	}

	statusResponse := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", statusResponse.Code, http.StatusOK, statusResponse.Body.String())
	}
	var statusPayload contract.StatusPayload
	decodeJSON(t, statusResponse.Body.Bytes(), &statusPayload)
	if statusPayload.Daemon.Status != string(contract.DrainStateDraining) {
		t.Fatalf("daemon status = %q, want %q", statusPayload.Daemon.Status, contract.DrainStateDraining)
	}

	doctorResponse := performRequest(t, fixture.Engine, http.MethodGet, "/doctor?only=daemon", nil)
	if doctorResponse.Code != http.StatusOK {
		t.Fatalf("doctor = %d, want %d; body=%s", doctorResponse.Code, http.StatusOK, doctorResponse.Body.String())
	}
	var doctorPayload contract.DoctorPayload
	decodeJSON(t, doctorResponse.Body.Bytes(), &doctorPayload)
	itemIndex := slices.IndexFunc(doctorPayload.Items, func(item contract.DiagnosticItem) bool {
		return item.Code == contract.CodeDaemonDraining
	})
	if itemIndex < 0 {
		t.Fatalf("doctor items = %#v, want daemon draining item", doctorPayload.Items)
	}
	item := doctorPayload.Items[itemIndex]
	if item.Code != contract.CodeDaemonDraining || item.Severity != contract.SeverityInfo {
		t.Fatalf("daemon diagnostic = %#v, want draining info", item)
	}

	for range 2 {
		response := performRequest(t, fixture.Engine, http.MethodPost, "/undrain", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("undrain status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.DrainStatusResponse
		decodeJSON(t, response.Body.Bytes(), &payload)
		if payload.State != contract.DrainStateActive {
			t.Fatalf("undrain state = %q, want %q", payload.State, contract.DrainStateActive)
		}
	}

	activeStatusResponse := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
	var activeStatus contract.StatusPayload
	decodeJSON(t, activeStatusResponse.Body.Bytes(), &activeStatus)
	if activeStatusResponse.Code != http.StatusOK || activeStatus.Daemon.Status != "running" {
		t.Fatalf("active daemon status = %d %#v, want running", activeStatusResponse.Code, activeStatus.Daemon)
	}
}

type testDrainController struct {
	draining atomic.Bool
}

func (c *testDrainController) Drain(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.draining.Store(true)
	return nil
}

func (c *testDrainController) Undrain(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.draining.Store(false)
	return nil
}

func (c *testDrainController) DrainState() contract.DrainState {
	if c.draining.Load() {
		return contract.DrainStateDraining
	}
	return contract.DrainStateActive
}

func TestLogsEndpointsRejectConflictingAliases(t *testing.T) {
	t.Parallel()

	t.Run("Should reject conflicting logs query aliases", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		for _, path := range []string{
			"/logs?after_seq=1&after_sequence=2",
			"/logs?workspace_id=ws-one&workspace=ws-two",
			"/logs?run=run-one&run_id=run-two",
		} {
			resp := performRequest(t, fixture.Engine, http.MethodGet, path, nil)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d, want %d; body=%s", path, resp.Code, http.StatusBadRequest, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), "conflicting query values") {
				t.Fatalf("%s body = %q, want conflicting query values message", path, resp.Body.String())
			}
		}
	})
}
