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

	t.Run("Should UT-002 allow same workspace and profile after facts lookup", func(t *testing.T) {
		t.Parallel()
		modes := &modeSourceStub{mode: ModeDenyAll}
		audit := &auditEmitterSpy{}
		policy := newPolicyForTest(t, modes, &consentCacheStub{}, audit, slog.Default())

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
		if modes.calls != 1 {
			t.Fatalf("facts source calls = %d, want one", modes.calls)
		}
		if len(audit.records) != 0 {
			t.Fatalf("audit records = %#v, want none: same-scope access crosses no boundary", audit.records)
		}
	})

	t.Run("Should UT-007 reject an empty target during structural validation", func(t *testing.T) {
		t.Parallel()
		modes := &modeSourceStub{mode: ModeApproveAll}
		policy := newPolicyForTest(t, modes, &consentCacheStub{}, &auditEmitterSpy{}, slog.Default())
		req := testRequest(ActorAgentSession, testWorkspaceA, "")

		decision, err := policy.Authorize(testutil.Context(t), req)
		assertErrorIsContains(t, err, ErrInvalidRequest, `target workspace id "" is not canonical`)
		assertDenied(t, decision, false)
		if modes.calls != 0 {
			t.Fatalf("mode source calls = %d, want zero", modes.calls)
		}
	})

	t.Run("Should UT-011 reject every nil constructor dependency", func(t *testing.T) {
		t.Parallel()
		valid := Deps{
			Facts:   &modeSourceStub{},
			Consent: &consentCacheStub{},
			Audit:   &auditEmitterSpy{},
			Log:     slog.Default(),
		}
		cases := []struct {
			name    string
			deps    Deps
			wantErr string
		}{
			{
				name: "nil modes", deps: Deps{Consent: valid.Consent, Audit: valid.Audit, Log: valid.Log},
				wantErr: "session facts source is required",
			},
			{
				name: "nil consent", deps: Deps{Facts: valid.Facts, Audit: valid.Audit, Log: valid.Log},
				wantErr: "session consent cache is required",
			},
			{
				name: "nil audit", deps: Deps{Facts: valid.Facts, Consent: valid.Consent, Log: valid.Log},
				wantErr: "audit emitter is required",
			},
			{
				name: "nil logger", deps: Deps{Facts: valid.Facts, Consent: valid.Consent, Audit: valid.Audit},
				wantErr: "logger is required",
			},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				policy, err := New(tc.deps)
				if policy != nil {
					t.Fatalf("New() policy = %#v, want nil", policy)
				}
				assertErrorContains(t, err, tc.wantErr)
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

		normalizedAudit := &auditEmitterSpy{}
		normalizedPolicy := newPolicyForTest(
			t,
			&modeSourceStub{mode: ModeApproveAll},
			&consentCacheStub{},
			normalizedAudit,
			slog.Default(),
		)
		normalizedReq := testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB)
		normalizedReq.Actor.SessionID = "  sess-a  "
		normalizedReq.Actor.WorkspaceID = "  " + testWorkspaceA + "  "
		normalizedReq.Actor.AgentName = "  coder  "
		normalizedReq.Target.WorkspaceID = "  " + testWorkspaceB + "  "
		if _, err := normalizedPolicy.Authorize(testutil.Context(t), normalizedReq); err != nil {
			t.Fatalf("Authorize(normalized audit) error = %v", err)
		}
		if len(normalizedAudit.records) != 1 || normalizedAudit.records[0].Actor.SessionID != "sess-a" ||
			normalizedAudit.records[0].Actor.WorkspaceID != testWorkspaceA ||
			normalizedAudit.records[0].Actor.AgentName != "coder" ||
			normalizedAudit.records[0].TargetID != testWorkspaceB {
			t.Fatalf("normalized audit record = %#v, want canonical decision inputs", normalizedAudit.records)
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
		for _, kind := range []ActorKind{ActorExtension, ActorAutomation, ActorDaemon} {
			t.Run("Should deny "+string(kind), func(t *testing.T) {
				t.Parallel()
				modes := &modeSourceStub{mode: ModeApproveAll}
				consent := &consentCacheStub{}
				policy := newPolicyForTest(t, modes, consent, &auditEmitterSpy{}, slog.Default())
				decision, err := policy.Authorize(
					testutil.Context(t),
					testRequest(kind, testWorkspaceA, testWorkspaceB),
				)
				if err != nil {
					t.Fatalf("Authorize(%q) error = %v", kind, err)
				}
				assertDenied(t, decision, false)
				if modes.calls != 0 || consent.calls != 0 {
					t.Fatalf("dependency calls = modes:%d consent:%d, want zero", modes.calls, consent.calls)
				}
			})
		}
		modes := &modeSourceStub{mode: ModeApproveAll}
		consent := &consentCacheStub{}
		policy := newPolicyForTest(t, modes, consent, &auditEmitterSpy{}, slog.Default())
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
		} {
			t.Run("Should reject target "+target, func(t *testing.T) {
				t.Parallel()
				modes := &modeSourceStub{mode: ModeApproveAll}
				policy := newPolicyForTest(t, modes, &consentCacheStub{}, &auditEmitterSpy{}, slog.Default())
				decision, err := policy.Authorize(
					testutil.Context(t),
					testRequest(ActorAgentSession, testWorkspaceA, target),
				)
				assertErrorIsContains(t, err, ErrInvalidRequest, "is not canonical")
				assertDenied(t, decision, false)
				if modes.calls != 0 {
					t.Fatalf("Authorize(target=%q) mode calls = %d, want zero", target, modes.calls)
				}
			})
		}

		policy := newPolicyForTest(
			t,
			&modeSourceStub{mode: ModeApproveAll},
			&consentCacheStub{},
			&auditEmitterSpy{},
			slog.Default(),
		)
		decision, err := policy.Authorize(
			testutil.Context(t),
			testRequest(ActorAgentSession, testWorkspaceA, "01J00000000000000000000001"),
		)
		if err != nil || !decision.Allowed {
			t.Fatalf("Authorize(canonical ULID) = %#v, %v, want allowed", decision, err)
		}
	})

	t.Run("Should UT-055 emit exactly one record for an error denial", func(t *testing.T) {
		t.Parallel()
		audit := &auditEmitterSpy{}
		policy := newPolicyForTest(t, &modeSourceStub{}, &consentCacheStub{}, audit, slog.Default())
		req := testRequest(ActorKind("unknown"), testWorkspaceA, testWorkspaceB)

		decision, err := policy.Authorize(testutil.Context(t), req)
		assertErrorIsContains(t, err, ErrInvalidRequest, "unknown actor kind")
		assertDenied(t, decision, false)
		if len(audit.records) != 1 || audit.records[0].Err == "" {
			t.Fatalf("audit records = %#v, want one error denial", audit.records)
		}
	})

	t.Run("Should UT-063 reject malformed actor and seam inputs before dependencies", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name    string
			req     Request
			wantErr string
		}{
			{
				name: "unknown actor kind", req: testRequest(ActorKind("unknown"), testWorkspaceA, testWorkspaceB),
				wantErr: "unknown actor kind",
			},
			{name: "missing seam", req: requestWithoutSeam(), wantErr: "unknown seam"},
			{name: "missing agent session id", req: requestWithoutSessionID(), wantErr: "agent session id is required"},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				modes := &modeSourceStub{mode: ModeApproveAll}
				consent := &consentCacheStub{consent: ConsentAllow, ok: true}
				policy := newPolicyForTest(t, modes, consent, &auditEmitterSpy{}, slog.Default())
				decision, err := policy.Authorize(testutil.Context(t), tc.req)
				assertErrorIsContains(t, err, ErrInvalidRequest, tc.wantErr)
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
			name    string
			mode    Mode
			err     error
			wantErr string
		}{
			{name: "mode read failure", err: errors.New("read failed"), wantErr: "read failed"},
			{name: "unknown session", err: errors.New("session not found"), wantErr: "session not found"},
			{
				name:    "unrecognized mode",
				mode:    Mode("unrestricted"),
				wantErr: `unrecognized permission mode "unrestricted"`,
			},
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
				assertErrorContains(t, err, tc.wantErr)
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

func (s *modeSourceStub) SessionFacts(context.Context, string) (SessionFacts, error) {
	s.calls++
	return SessionFacts{Mode: s.mode, ProfileID: "profile-default", WorkspaceID: testWorkspaceA}, s.err
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
	modes SessionFactsSource,
	consent SessionConsentCache,
	audit AuditEmitter,
	logger *slog.Logger,
) *DefaultPolicy {
	t.Helper()
	policy, err := New(Deps{Facts: modes, Consent: consent, Audit: audit, Log: logger})
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
		Target: TargetRef{Kind: TargetWorkspace, WorkspaceID: target, ProfileID: "profile-default"},
		Seam:   SeamTool,
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

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}

func assertErrorIsContains(t *testing.T, err error, target error, want string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want errors.Is(%v)", err, target)
	}
	assertErrorContains(t, err, want)
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

func TestUserScopeAccessPolicy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                         string
		actorWorkspace, actorProfile string
		target                       TargetRef
		mode                         Mode
		allowed                      bool
	}{
		{"Should deny ungranted aggregate access under approve-all", testWorkspaceA, "p1", TargetRef{Kind: TargetAggregate, ProfileID: "p1", Surface: "gateway-ingress"}, ModeApproveAll, false},
		{"Should allow User in its own profile even under deny-all", "", "p1", ScopeTarget("", "p1"), ModeDenyAll, true},
		{"Should deny project to User under deny-all", testWorkspaceA, "p1", ScopeTarget("", "p1"), ModeDenyAll, false},
		{"Should allow project to User under approve-all", testWorkspaceA, "p1", ScopeTarget("", "p1"), ModeApproveAll, true},
		{"Should deny User to project under approve-reads without consent", "", "p1", ScopeTarget(testWorkspaceA, "p1"), ModeApproveReads, false},
		{"Should allow User to project under approve-all", "", "p1", ScopeTarget(testWorkspaceA, "p1"), ModeApproveAll, true},
		{"Should deny same workspace in another profile even under approve-all", testWorkspaceA, "p1", ScopeTarget(testWorkspaceA, "p2"), ModeApproveAll, false},
		{"Should deny User in another profile even under approve-all", "", "p1", ScopeTarget("", "p2"), ModeApproveAll, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := factsSourceFunc(func(context.Context, string) (SessionFacts, error) {
				return SessionFacts{Mode: tc.mode, WorkspaceID: tc.actorWorkspace, ProfileID: tc.actorProfile}, nil
			})
			policy := newPolicyForTest(t, facts, &consentCacheStub{}, &auditEmitterSpy{}, slog.Default())
			req := testRequest(ActorAgentSession, tc.actorWorkspace, testWorkspaceA)
			req.Target = tc.target
			decision, err := policy.Authorize(t.Context(), req)
			if err != nil || decision.Allowed != tc.allowed {
				t.Fatalf("Authorize = %#v, %v; want allowed %v", decision, err, tc.allowed)
			}
		})
	}
	t.Run("Should reject all-profile mutations and incomplete typed targets", func(t *testing.T) {
		t.Parallel()
		policy := newPolicyForTest(t, &modeSourceStub{}, &consentCacheStub{}, &auditEmitterSpy{}, slog.Default())
		for _, target := range []TargetRef{{Kind: TargetUser}, {Kind: TargetUser, AllProfiles: true}, {Kind: TargetUser, ProfileID: "p1", WorkspaceID: testWorkspaceA}, {Kind: TargetAggregate, ProfileID: "p1"}} {
			req := testRequest(ActorHuman, "", testWorkspaceA)
			req.Target = target
			if _, err := policy.Authorize(t.Context(), req); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("target %#v error = %v", target, err)
			}
		}
	})
}

type factsSourceFunc func(context.Context, string) (SessionFacts, error)

func (f factsSourceFunc) SessionFacts(ctx context.Context, id string) (SessionFacts, error) {
	return f(ctx, id)
}

func TestUnrestrictedGrant(t *testing.T) {
	t.Parallel()
	for _, target := range []TargetRef{ScopeTarget(testWorkspaceB, "p1"), ScopeTarget("", "p1"), ScopeTarget(testWorkspaceA, "p2"), {Kind: TargetAggregate, Surface: "session-catalog", AllProfiles: true}} {
		t.Run("Should authorize and audit granted "+string(target.Kind)+target.ProfileID, func(t *testing.T) {
			t.Parallel()
			audit := &auditEmitterSpy{}
			facts := factsSourceFunc(func(context.Context, string) (SessionFacts, error) {
				return SessionFacts{
					Mode:            ModeDenyAll,
					WorkspaceAccess: Unrestricted,
					WorkspaceID:     testWorkspaceA,
					ProfileID:       "p1",
				}, nil
			})
			policy := newPolicyForTest(t, facts, &consentCacheStub{}, audit, slog.Default())
			req := testRequest(ActorAgentSession, testWorkspaceA, testWorkspaceB)
			req.Target, req.ReadOnly = target, true
			decision, err := policy.Authorize(t.Context(), req)
			if err != nil || !decision.Allowed || decision.Source != SourceAgentGrant || decision.PromptEligible {
				t.Fatalf("Authorize = %#v, %v", decision, err)
			}
			if len(audit.records) != 1 || audit.records[0].ActorProfileID != "p1" || audit.records[0].Target != target {
				t.Fatalf("audit = %#v", audit.records)
			}
		})
	}
	t.Run("Should cap every non-operator creation at its persisted ceiling", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			origin           Origin
			definition, want string
		}{
			{Origin{Kind: ActorHuman}, Unrestricted, Unrestricted},
			{Origin{Kind: ActorAgentSession}, Unrestricted, ""},
			{Origin{Kind: ActorAgentSession, Ceiling: Unrestricted}, "", ""},
			{Origin{Kind: ActorAutomation, Ceiling: Unrestricted}, Unrestricted, Unrestricted},
			{Origin{}, Unrestricted, ""},
		} {
			if got := tc.origin.Effective(tc.definition); got != tc.want {
				t.Fatalf("Effective(%#v,%q)=%q want %q", tc.origin, tc.definition, got, tc.want)
			}
		}
	})
}
