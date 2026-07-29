package workspaceaccess

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/testutil"
)

const (
	testWorkspaceA = "ws_0000000000000000"
	testWorkspaceB = "ws_0000000000000001"
)

func TestDefaultPolicyAuthorize(t *testing.T) {
	t.Parallel()

	t.Run("Should UT-001 allow operators before mode lookup", func(t *testing.T) {
		t.Parallel()
		modes := &modeSourceStub{mode: ModeDenyAll}
		consent := &consentCacheStub{}
		policy := newPolicyForTest(t, modes, consent, &auditEmitterSpy{}, slog.Default())

		decision, err := policy.Authorize(testutil.Context(t), testRequest(ActorHuman, testWorkspaceA, testWorkspaceB))
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		if !decision.Allowed || decision.Source != SourceOperator {
			t.Fatalf("Authorize() decision = %#v, want operator allow", decision)
		}
		if modes.calls != 0 || consent.calls != 0 {
			t.Fatalf("fast path dependency calls = modes:%d consent:%d, want zero", modes.calls, consent.calls)
		}
	})

	t.Run("Should UT-002 allow same-workspace agent sessions before mode lookup", func(t *testing.T) {
		t.Parallel()
		modes := &modeSourceStub{mode: ModeDenyAll}
		policy := newPolicyForTest(t, modes, &consentCacheStub{}, &auditEmitterSpy{}, slog.Default())

		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceA),
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		if !decision.Allowed || decision.Source != SourceSameWorkspace {
			t.Fatalf("Authorize() decision = %#v, want same-workspace allow", decision)
		}
		if modes.calls != 0 {
			t.Fatalf("mode source calls = %d, want zero", modes.calls)
		}
	})

	t.Run("Should UT-007 reject an empty target during structural validation", func(t *testing.T) {
		t.Parallel()
		modes := &modeSourceStub{mode: ModeApproveAll}
		policy := newPolicyForTest(t, modes, &consentCacheStub{}, &auditEmitterSpy{}, slog.Default())
		req := testRequest(ActorAgentSession, testWorkspaceA, "")

		decision, err := policy.Authorize(testutil.Context(t), req)
		if err == nil {
			t.Fatal("Authorize() error = nil, want non-nil")
		}
		assertDenied(t, decision, false)
		if modes.calls != 0 {
			t.Fatalf("mode source calls = %d, want zero", modes.calls)
		}
	})

	t.Run("Should UT-011 reject every nil constructor dependency", func(t *testing.T) {
		t.Parallel()
		valid := Deps{
			Modes:   &modeSourceStub{},
			Consent: &consentCacheStub{},
			Audit:   &auditEmitterSpy{},
			Log:     slog.Default(),
		}
		cases := []struct {
			name string
			deps Deps
		}{
			{name: "nil modes", deps: Deps{Consent: valid.Consent, Audit: valid.Audit, Log: valid.Log}},
			{name: "nil consent", deps: Deps{Modes: valid.Modes, Audit: valid.Audit, Log: valid.Log}},
			{name: "nil audit", deps: Deps{Modes: valid.Modes, Consent: valid.Consent, Log: valid.Log}},
			{name: "nil logger", deps: Deps{Modes: valid.Modes, Consent: valid.Consent, Audit: valid.Audit}},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				if policy, err := New(tc.deps); err == nil || policy != nil {
					t.Fatalf("New() = (%#v, %v), want nil policy and error", policy, err)
				}
			})
		}
	})

	t.Run("Should UT-012 audit allows and preserve them when emission fails", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		audit := &auditEmitterSpy{err: errors.New("append unavailable")}
		policy := newPolicyForTest(t, &modeSourceStub{}, &consentCacheStub{}, audit, testLogger(&logs))

		decision, err := policy.Authorize(testutil.Context(t), testRequest(ActorHuman, testWorkspaceA, testWorkspaceB))
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		if !decision.Allowed || decision.Source != SourceOperator {
			t.Fatalf("Authorize() decision = %#v, want operator allow", decision)
		}
		if len(audit.records) != 1 || !audit.records[0].Allowed {
			t.Fatalf("audit records = %#v, want one allow", audit.records)
		}
		if !strings.Contains(logs.String(), "workspace access audit emission failed") {
			t.Fatalf("logs = %q, want audit warning", logs.String())
		}
	})

	t.Run("Should UT-013 audit denials and preserve them when emission fails", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		audit := &auditEmitterSpy{err: errors.New("append unavailable")}
		policy := newPolicyForTest(t, &modeSourceStub{}, &consentCacheStub{}, audit, testLogger(&logs))

		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorExtension, testWorkspaceA, testWorkspaceB),
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		assertDenied(t, decision, false)
		if len(audit.records) != 1 || audit.records[0].Allowed {
			t.Fatalf("audit records = %#v, want one denial", audit.records)
		}
		if !strings.Contains(logs.String(), "workspace access audit emission failed") {
			t.Fatalf("logs = %q, want audit warning", logs.String())
		}
	})

	t.Run("Should UT-051 prevent non-agent kinds from inheriting approve-all", func(t *testing.T) {
		t.Parallel()
		modes := &modeSourceStub{mode: ModeApproveAll}
		consent := &consentCacheStub{}
		policy := newPolicyForTest(t, modes, consent, &auditEmitterSpy{}, slog.Default())
		for _, kind := range []ActorKind{ActorExtension, ActorAutomation, ActorNetworkPeer, ActorDaemon} {
			decision, err := policy.Authorize(testutil.Context(t), testRequest(kind, testWorkspaceA, testWorkspaceB))
			if err != nil {
				t.Fatalf("Authorize(%q) error = %v", kind, err)
			}
			assertDenied(t, decision, false)
		}
		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB),
		)
		if err != nil {
			t.Fatalf("Authorize(agent_session) error = %v", err)
		}
		if !decision.Allowed || decision.Source != SourcePermissionMode {
			t.Fatalf("Authorize(agent_session) = %#v, want permission-mode allow", decision)
		}
		if modes.calls != 1 || consent.calls != 0 {
			t.Fatalf("dependency calls = modes:%d consent:%d, want 1 and 0", modes.calls, consent.calls)
		}
	})

	t.Run("Should UT-053 reject non-canonical target references", func(t *testing.T) {
		t.Parallel()
		for _, target := range []string{
			"marketing",
			"../marketing",
			"/tmp/marketing",
			"ws_marketing",
			"01J00000000000000000000000",
		} {
			modes := &modeSourceStub{mode: ModeApproveAll}
			policy := newPolicyForTest(t, modes, &consentCacheStub{}, &auditEmitterSpy{}, slog.Default())
			decision, err := policy.Authorize(
				testutil.Context(t),
				testRequest(ActorAgentSession, testWorkspaceA, target),
			)
			if err == nil {
				t.Fatalf("Authorize(target=%q) error = nil, want non-nil", target)
			}
			assertDenied(t, decision, false)
			if modes.calls != 0 {
				t.Fatalf("Authorize(target=%q) mode calls = %d, want zero", target, modes.calls)
			}
		}
	})

	t.Run("Should UT-055 emit exactly one record for an error denial", func(t *testing.T) {
		t.Parallel()
		audit := &auditEmitterSpy{}
		policy := newPolicyForTest(t, &modeSourceStub{}, &consentCacheStub{}, audit, slog.Default())
		req := testRequest(ActorKind("unknown"), testWorkspaceA, testWorkspaceB)

		decision, err := policy.Authorize(testutil.Context(t), req)
		if err == nil {
			t.Fatal("Authorize() error = nil, want non-nil")
		}
		assertDenied(t, decision, false)
		if len(audit.records) != 1 || audit.records[0].Err == "" {
			t.Fatalf("audit records = %#v, want one error denial", audit.records)
		}
	})

	t.Run("Should UT-063 reject malformed actor and seam inputs before dependencies", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			req  Request
		}{
			{name: "unknown actor kind", req: testRequest(ActorKind("unknown"), testWorkspaceA, testWorkspaceB)},
			{name: "missing seam", req: requestWithoutSeam()},
			{name: "missing agent session id", req: requestWithoutSessionID()},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				modes := &modeSourceStub{mode: ModeApproveAll}
				consent := &consentCacheStub{consent: ConsentAllow, ok: true}
				policy := newPolicyForTest(t, modes, consent, &auditEmitterSpy{}, slog.Default())
				decision, err := policy.Authorize(testutil.Context(t), tc.req)
				if err == nil {
					t.Fatal("Authorize() error = nil, want non-nil")
				}
				assertDenied(t, decision, false)
				if modes.calls != 0 || consent.calls != 0 {
					t.Fatalf("dependency calls = modes:%d consent:%d, want zero", modes.calls, consent.calls)
				}
			})
		}
	})

	t.Run("Should UT-076 allow approve-all without consulting consent", func(t *testing.T) {
		t.Parallel()
		consent := &consentCacheStub{consent: ConsentReject, ok: true}
		policy := newPolicyForTest(
			t,
			&modeSourceStub{mode: ModeApproveAll},
			consent,
			&auditEmitterSpy{},
			slog.Default(),
		)
		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB),
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		if !decision.Allowed || decision.Source != SourcePermissionMode || decision.PromptEligible {
			t.Fatalf("Authorize() decision = %#v, want approve-all allow", decision)
		}
		if consent.calls != 0 {
			t.Fatalf("consent calls = %d, want zero", consent.calls)
		}
	})

	t.Run("Should UT-077 hard-deny deny-all without consulting consent", func(t *testing.T) {
		t.Parallel()
		consent := &consentCacheStub{consent: ConsentAllow, ok: true}
		policy := newPolicyForTest(t, &modeSourceStub{mode: ModeDenyAll}, consent, &auditEmitterSpy{}, slog.Default())
		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB),
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		assertDenied(t, decision, false)
		if consent.calls != 0 {
			t.Fatalf("consent calls = %d, want zero", consent.calls)
		}
	})

	t.Run("Should UT-078 make approve-reads misses prompt-eligible", func(t *testing.T) {
		t.Parallel()
		policy := newPolicyForTest(
			t,
			&modeSourceStub{mode: ModeApproveReads},
			&consentCacheStub{},
			&auditEmitterSpy{},
			slog.Default(),
		)
		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB),
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		assertDenied(t, decision, true)
	})

	t.Run("Should UT-079 allow cached session consent", func(t *testing.T) {
		t.Parallel()
		policy := newPolicyForTest(
			t,
			&modeSourceStub{mode: ModeApproveReads},
			&consentCacheStub{consent: ConsentAllow, ok: true},
			&auditEmitterSpy{},
			slog.Default(),
		)
		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB),
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		if !decision.Allowed || decision.Source != SourceSessionConsent || decision.PromptEligible {
			t.Fatalf("Authorize() decision = %#v, want session-consent allow", decision)
		}
	})

	t.Run("Should UT-080 hard-deny cached session rejection", func(t *testing.T) {
		t.Parallel()
		policy := newPolicyForTest(
			t,
			&modeSourceStub{mode: ModeApproveReads},
			&consentCacheStub{consent: ConsentReject, ok: true},
			&auditEmitterSpy{},
			slog.Default(),
		)
		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB),
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		assertDenied(t, decision, false)
	})

	t.Run("Should UT-081 fail closed on mode lookup and value errors", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			mode Mode
			err  error
		}{
			{name: "mode read failure", err: errors.New("read failed")},
			{name: "unknown session", err: errors.New("session not found")},
			{name: "unrecognized mode", mode: Mode("unrestricted")},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				audit := &auditEmitterSpy{}
				policy := newPolicyForTest(
					t,
					&modeSourceStub{mode: tc.mode, err: tc.err},
					&consentCacheStub{},
					audit,
					slog.Default(),
				)
				decision, err := policy.Authorize(
					testutil.Context(t),
					testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB),
				)
				if err == nil {
					t.Fatal("Authorize() error = nil, want non-nil")
				}
				assertDenied(t, decision, false)
				if len(audit.records) != 1 || audit.records[0].Err == "" || audit.records[0].Mode != tc.mode {
					t.Fatalf("audit records = %#v, want error and mode %q", audit.records, tc.mode)
				}
			})
		}
	})

	t.Run("Should UT-082 apply modes without same-workspace matching for workspace-less sessions", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name           string
			mode           Mode
			wantAllowed    bool
			wantPromptable bool
		}{
			{name: "approve-all", mode: ModeApproveAll, wantAllowed: true},
			{name: "approve-reads", mode: ModeApproveReads, wantPromptable: true},
			{name: "deny-all", mode: ModeDenyAll},
		}
		for _, tc := range cases {
			t.Run("Should apply "+tc.name, func(t *testing.T) {
				t.Parallel()
				policy := newPolicyForTest(
					t,
					&modeSourceStub{mode: tc.mode},
					&consentCacheStub{},
					&auditEmitterSpy{},
					slog.Default(),
				)
				req := testRequest(ActorAgentSession, "", testWorkspaceB)
				decision, err := policy.Authorize(testutil.Context(t), req)
				if err != nil {
					t.Fatalf("Authorize() error = %v", err)
				}
				if decision.Allowed != tc.wantAllowed || decision.PromptEligible != tc.wantPromptable {
					t.Fatalf(
						"Authorize() decision = %#v, want allowed=%t prompt=%t",
						decision,
						tc.wantAllowed,
						tc.wantPromptable,
					)
				}
			})
		}
	})
}

type modeSourceStub struct {
	mode  Mode
	err   error
	calls int
}

func (s *modeSourceStub) SessionPermissionMode(context.Context, string) (Mode, error) {
	s.calls++
	return s.mode, s.err
}

type consentCacheStub struct {
	consent Consent
	ok      bool
	calls   int
}

func (s *consentCacheStub) ConsentFor(context.Context, string) (Consent, bool) {
	s.calls++
	return s.consent, s.ok
}

func (*consentCacheStub) PutConsent(context.Context, string, Consent) {}

type auditEmitterSpy struct {
	records []AccessRecord
	err     error
}

func (s *auditEmitterSpy) EmitWorkspaceAccess(_ context.Context, record AccessRecord) error {
	s.records = append(s.records, record)
	return s.err
}

func newPolicyForTest(
	t *testing.T,
	modes ModeSource,
	consent SessionConsentCache,
	audit AuditEmitter,
	logger *slog.Logger,
) *DefaultPolicy {
	t.Helper()
	policy, err := New(Deps{Modes: modes, Consent: consent, Audit: audit, Log: logger})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return policy
}

func testRequest(kind ActorKind, home string, target string) Request {
	return Request{
		Actor: ActorRef{
			Kind:        kind,
			SessionID:   "sess-1",
			WorkspaceID: home,
			AgentName:   "coder",
			Operator:    kind == ActorHuman,
		},
		TargetWorkspaceID: target,
		Seam:              SeamTool,
	}
}

func requestWithoutSeam() Request {
	req := testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB)
	req.Seam = ""
	return req
}

func requestWithoutSessionID() Request {
	req := testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB)
	req.Actor.SessionID = ""
	return req
}

func assertDenied(t *testing.T, decision Decision, promptEligible bool) {
	t.Helper()
	if decision.Allowed || decision.Source != SourceDenied || decision.PromptEligible != promptEligible {
		t.Fatalf("decision = %#v, want denied with prompt=%t", decision, promptEligible)
	}
}

func testLogger(output *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
