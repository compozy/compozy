package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	monitorpkg "github.com/compozy/compozy/internal/monitor"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestWorkspaceAccessConsentLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should UT-084 clear only the stopped session consent through the lifecycle fanout", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		cache := newWorkspaceAccessConsentCache()
		fanout := newSessionLifecycleFanout(cache)
		cache.PutConsent(ctx, "sess-a", workspaceaccess.ConsentAllow)
		cache.PutConsent(ctx, "sess-b", workspaceaccess.ConsentReject)

		fanout.OnSessionStopped(ctx, &session.Session{ID: "sess-a", AgentName: "coder"})

		if consent, ok := cache.ConsentFor(ctx, "sess-a"); ok || consent != "" {
			t.Fatalf("ConsentFor(sess-a) = (%q, %t), want miss", consent, ok)
		}
		if consent, ok := cache.ConsentFor(ctx, "sess-b"); !ok || consent != workspaceaccess.ConsentReject {
			t.Fatalf("ConsentFor(sess-b) = (%q, %t), want reject hit", consent, ok)
		}
		fanout.OnSessionCreated(ctx, &session.Session{ID: "sess-c", AgentName: "coder"})
		if consent, ok := cache.ConsentFor(ctx, "sess-c"); ok || consent != "" {
			t.Fatalf("ConsentFor(new same-agent session) = (%q, %t), want miss", consent, ok)
		}
	})
}

func TestNativeSessionTargetAccess(t *testing.T) {
	t.Parallel()

	const (
		homeWorkspace    = "ws_00000000000000a1"
		foreignWorkspace = "ws_00000000000000b2"
		seededSecret     = "sk-proj-AbCdEf0123456789xyzQRS"
	)
	statusDescriptor := toolspkg.Descriptor{
		ID:      toolspkg.ToolIDSessionStatus,
		Backend: toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: "test"},
		InputSchema: json.RawMessage(
			`{"type":"object","properties":{"workspace":{"type":"string"},"session_id":{"type":"string"}}}`,
		),
		ReadOnly: true,
	}
	actorScope := toolspkg.Scope{SessionID: "sess-actor", WorkspaceID: homeWorkspace, AgentName: "coder"}
	sessionsFor := func(mode workspaceaccess.Mode) *apitest.StubSessionManager {
		return &apitest.StubSessionManager{StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			switch id {
			case "sess-actor", "sess-actor-next":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: homeWorkspace, ProfileID: "p-x",
					State: session.StateActive, EffectivePermissions: string(mode),
				}, nil
			case "sess-own":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: homeWorkspace, ProfileID: "p-x",
					State: session.StateActive, Name: "own " + seededSecret,
				}, nil
			case "sess-foreign":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: foreignWorkspace, ProfileID: "p-y",
					State: session.StateActive, Name: "deploy with " + seededSecret,
				}, nil
			case "sess-peer":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: homeWorkspace, ProfileID: "p-y",
					State: session.StateStopped,
				}, nil
			case "sess-sibling", "sess-sibling-next":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: foreignWorkspace, ProfileID: "p-x",
					State: session.StateActive,
				}, nil
			case "sess-user":
				return &session.Info{ID: id, AgentName: "coder", ProfileID: "p-y", State: session.StateStopped}, nil
			case "sess-user-actor":
				return &session.Info{
					ID: id, AgentName: "coder", ProfileID: "p-x",
					State: session.StateActive, EffectivePermissions: string(mode),
				}, nil
			default:
				return nil, session.ErrSessionNotFound
			}
		}}
	}
	newPolicy := func(t *testing.T, sessions *apitest.StubSessionManager) workspaceaccess.Policy {
		t.Helper()
		facts, err := newWorkspaceAccessModeSource(sessions)
		if err != nil {
			t.Fatalf("newWorkspaceAccessModeSource() error = %v", err)
		}
		policy, err := workspaceaccess.New(workspaceaccess.Deps{
			Facts:   facts,
			Consent: newWorkspaceAccessConsentCache(),
			Audit:   nativeSessionTargetAuditStub{},
			Log:     slog.New(slog.DiscardHandler),
		})
		if err != nil {
			t.Fatalf("workspaceaccess.New() error = %v", err)
		}
		return policy
	}
	t.Run("Should apply the policy matrix to id-only orchestration targets", func(t *testing.T) {
		t.Parallel()
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDSessionWait, toolspkg.ToolIDSessionStop, toolspkg.ToolIDSessionApprove,
			toolspkg.ToolIDSessionClarifyAnswer, toolspkg.ToolIDSessionPromptCancel, toolspkg.ToolIDSessionCompact,
			toolspkg.ToolIDMonitorCreate,
		} {
			t.Run("Should authorize "+string(id), func(t *testing.T) {
				t.Parallel()
				for _, tc := range []struct {
					name, target, hint string
					mode               workspaceaccess.Mode
					grant, operator    bool
				}{
					{name: "Should allow own scope", target: "sess-own", mode: workspaceaccess.ModeDenyAll},
					{name: "Should allow same-profile approve-all", target: "sess-sibling", mode: workspaceaccess.ModeApproveAll},
					{name: "Should require consent in the default mode", target: "sess-sibling", mode: workspaceaccess.ModeApproveReads, hint: "operator's approval"},
					{name: "Should refuse deny-all", target: "sess-sibling", mode: workspaceaccess.ModeDenyAll, hint: "deny-all"},
					{name: "Should require the grant for cross-profile writes", target: "sess-foreign", mode: workspaceaccess.ModeApproveAll, hint: "workspace_access: unrestricted"},
					{name: "Should allow unrestricted", target: "sess-foreign", mode: workspaceaccess.ModeDenyAll, grant: true},
					{name: "Should allow the operator", target: "sess-foreign", operator: true},
				} {
					t.Run(tc.name, func(t *testing.T) {
						t.Parallel()
						sessions := sessionsFor(tc.mode)
						status := sessions.StatusFn
						sessions.StatusFn = func(ctx context.Context, target string) (*session.Info, error) {
							info, err := status(ctx, target)
							if info != nil && target == "sess-actor" && tc.grant {
								info.EffectiveWorkspaceAccess = workspaceaccess.Unrestricted
							}
							return info, err
						}
						descriptor := statusDescriptor
						descriptor.ID, descriptor.ReadOnly = id, id == toolspkg.ToolIDSessionWait
						descriptor.InputSchema = json.RawMessage(
							`{"type":"object","properties":{"session_id":{"type":"string"}}}`,
						)
						scope := actorScope
						scope.Operator = tc.operator
						binder := newNativeWorkspaceInputBinder(
							nativeTestWorkspaceService(t),
							sessions,
							newPolicy(t, sessions),
							nil,
						)
						input := `{"session_id":"` + tc.target + `"}`
						if id == toolspkg.ToolIDMonitorCreate {
							input = `{"kind":"session","target_session_ids":["` + tc.target + `"]}`
						}
						err := binder.AuthorizeCallInput(t.Context(), scope, descriptor, toolspkg.CallRequest{
							ToolID: id, Input: json.RawMessage(input),
						})
						hint := tc.hint
						if (descriptor.ReadOnly || id == toolspkg.ToolIDMonitorCreate) &&
							hint == "workspace_access: unrestricted" {
							hint = ""
						}
						if hint == "" {
							if err != nil {
								t.Fatal(err)
							}
						} else if err == nil || !strings.Contains(err.Error(), hint) {
							t.Fatalf("authorization = %v, want %q", err, hint)
						}
					})
				}
			})
		}
	})
	t.Run("Should keep process operations local independently of permission mode", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		n := &daemonNativeTools{
			deps: &daemonNativeToolsDeps{Sessions: sessions, Workspaces: nativeTestWorkspaceService(t)},
		}
		scope := actorScope
		scope.ProfileID = "p-x"
		for _, id := range []toolspkg.ToolID{toolspkg.ToolIDSessionProcessesList, toolspkg.ToolIDSessionProcessesTerminate} {
			for _, target := range []string{"sess-own", "sess-sibling", "sess-peer"} {
				t.Run("Should enforce local process scope for "+string(id)+" targeting "+target, func(t *testing.T) {
					_, _, err := n.nativeOrchestrationTarget(t.Context(), scope, id, target, false)
					if target == "sess-own" {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), "workspace") ||
						strings.Contains(err.Error(), "approve-all") {
						t.Fatalf("local process target %s: %v", target, err)
					}
				})
			}
		}
	})
	t.Run("Should dispatch every id-only orchestration tool to its authorized foreign owner", func(t *testing.T) {
		t.Parallel()
		base := sessionsFor(workspaceaccess.ModeApproveAll)
		called := 0
		check := func(id string) {
			t.Helper()
			if id != "sess-sibling" {
				t.Fatalf("target = %q", id)
			}
			called++
		}
		base.ApproveFn = func(_ context.Context, id string, req acp.ApproveRequest) (session.ApprovalResult, error) {
			check(id)
			return session.ApprovalResult{Outcome: "applied", RequestID: req.RequestID}, nil
		}
		base.CancelPromptFn = func(_ context.Context, id string) (session.PromptCancelResult, error) {
			check(id)
			return session.PromptCancelResult{Outcome: session.PromptCancelOutcomeCanceled}, nil
		}
		base.RequestCompactionFn = func(_ context.Context, id string) (session.CompactionRequestResult, <-chan acp.AgentEvent, error) {
			check(id)
			return session.CompactionRequestResult{SessionID: id, PromptID: "compact", Command: "compact"}, nil, nil
		}
		manager := &nativeOrchestrationSessionManager{
			StubSessionManager: *base,
			requestStopFn:      func(_ context.Context, id string, _ session.StopCause, _ string) error { check(id); return nil },
			waitFn: func(_ context.Context, req session.WaitRequest) (session.WaitOutcome, error) {
				check(req.SessionID)
				return session.WaitOutcome{Outcome: session.WaitResultStateReached, State: session.BadgeIdle}, nil
			},
		}
		clarify := nativeOrchestrationClarifyBroker{answerFn: func(_ context.Context, scope toolspkg.Scope, _ string,
			_ toolspkg.ClarifyAnswerRequest) (toolspkg.ClarifyAnswerResult, error) {
			check(scope.SessionID)
			if scope.WorkspaceID != foreignWorkspace {
				t.Fatalf("clarify scope = %#v", scope)
			}
			return toolspkg.ClarifyAnswerResult{Outcome: "answered", RequestID: "question"}, nil
		}}
		registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(t, &daemonNativeToolsDeps{
			Sessions: manager, Workspaces: nativeTestWorkspaceService(t),
			Clarify: func() toolspkg.ClarifyBroker { return clarify },
		}, toolspkg.NewStaticPolicyInputResolver(nativeApproveAllPolicyInputs()), newPolicy(t, base))
		for _, call := range []struct {
			id    toolspkg.ToolID
			extra string
		}{
			{toolspkg.ToolIDSessionWait, ""}, {toolspkg.ToolIDSessionStop, `,"wait":false`},
			{toolspkg.ToolIDSessionApprove, `,"request_id":"permission","decision":"allow-once"`},
			{toolspkg.ToolIDSessionClarifyAnswer, `,"request_id":"question","choice":1`},
			{toolspkg.ToolIDSessionPromptCancel, ""}, {toolspkg.ToolIDSessionCompact, ""},
		} {
			t.Run("Should dispatch "+string(call.id)+" to its authorized owner", func(t *testing.T) {
				before := called
				result, err := registry.Call(t.Context(), actorScope, toolspkg.CallRequest{ToolID: call.id,
					Input: json.RawMessage(`{"session_id":"sess-sibling"` + call.extra + `}`)})
				if err != nil || called != before+1 || len(result.Structured) == 0 {
					t.Fatalf("Call(%s) = %s, %v; handler calls = %d", call.id, result.Structured, err, called-before)
				}
			})
		}
	})
	t.Run("Should authorize global reads while retaining grants for global definition writes", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		for _, id := range []toolspkg.ToolID{toolspkg.ToolIDTaskList, toolspkg.ToolIDAgentCreate, toolspkg.ToolIDExtensionsInstall} {
			t.Run("Should authorize the global scope for "+string(id), func(t *testing.T) {
				var descriptor toolspkg.Descriptor
				for _, item := range builtintools.NativeDescriptors() {
					if item.ID == id {
						descriptor = item
						break
					}
				}
				input, err := binder.BindCallInput(
					t.Context(),
					actorScope,
					descriptor,
					json.RawMessage(`{"scope":"global"}`),
				)
				if descriptor.ReadOnly {
					if err == nil {
						err = binder.AuthorizeCallInput(
							t.Context(),
							actorScope,
							descriptor,
							toolspkg.CallRequest{ToolID: id, Input: input},
						)
					}
					if err != nil {
						t.Fatalf("global read %s: %v", id, err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "workspace_access: unrestricted") {
					t.Fatalf("global write %s: %v", id, err)
				}
			})
		}
	})
	t.Run("Should register foreign session monitors and redact retained results after revocation", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		var revoked atomic.Bool
		status := sessions.StatusFn
		sessions.StatusFn = func(ctx context.Context, id string) (*session.Info, error) {
			info, err := status(ctx, id)
			if info != nil && id == actorScope.SessionID && revoked.Load() {
				info.EffectivePermissions = string(workspaceaccess.ModeDenyAll)
			}
			return info, err
		}
		policy := newPolicy(t, sessions)
		service, err := monitorpkg.NewService(t.Context(), nil, retryMonitorWaker{},
			monitorpkg.WithSessionTargets(monitorSessionTargets{sessions: func() any { return sessions }}))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.WithoutCancel(t.Context())); err != nil {
				t.Error(err)
			}
		})
		registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(t, &daemonNativeToolsDeps{
			Sessions: sessions, Workspaces: nativeTestWorkspaceService(t), WorkspaceAccess: policy,
			Monitors: func() *monitorpkg.Service { return service },
		}, toolspkg.NewStaticPolicyInputResolver(nativeApproveAllPolicyInputs()), policy)
		created, err := registry.Call(
			t.Context(),
			actorScope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMonitorCreate,
				Input: json.RawMessage(
					`{"kind":"session","description":"watch work","target_session_ids":["sess-foreign"]}`,
				),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Monitor monitorpkg.Monitor `json:"monitor"`
		}
		if err := json.Unmarshal(created.Structured, &payload); err != nil {
			t.Fatal(err)
		}
		mon := payload.Monitor
		// Retained snapshots must be filtered even if a revoked watch fired before the next list.
		mon.Targets = []monitorpkg.TargetStatus{
			{SessionID: "sess-foreign", Name: seededSecret, State: "idle", Matched: true},
		}
		n := &daemonNativeTools{deps: &daemonNativeToolsDeps{Sessions: sessions, WorkspaceAccess: policy}}
		visible, err := n.visibleMonitor(t.Context(), actorScope.SessionID, mon)
		if err != nil || strings.Contains(visible.Targets[0].Name, seededSecret) {
			t.Fatalf("visible = %#v, %v", visible, err)
		}
		revoked.Store(true)
		visible, err = n.visibleMonitor(t.Context(), actorScope.SessionID, mon)
		if err != nil || visible.Reason != "access_denied" || visible.Targets[0].Name != "" ||
			visible.Targets[0].Matched || visible.Targets[0].State != monitorpkg.TargetStateUnknown {
			t.Fatalf("revoked monitor = %#v, %v", visible, err)
		}
	})
	// Invariant: prompt-approved monitor reads retain only exact saved consent, with live revocation.
	// Owner: native authorization boundary; canonical suite: TestNativeSessionTargetAccess.
	t.Run("Should retain Always allow monitor grants and keep Allow once call scoped", func(t *testing.T) {
		t.Parallel()
		for _, choice := range []acpsdk.PermissionOptionId{workspaceAccessAllowAlwaysID, workspaceAccessAllowOnceID} {
			t.Run("Should apply "+string(choice)+" monitor consent", func(t *testing.T) {
				t.Parallel()
				sessions := sessionsFor(workspaceaccess.ModeAuto)
				policy := newPolicy(t, sessions)
				grants := &recordingApprovalGrantStore{}
				requester := selectedPermissionRequester(choice)
				binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, policy,
					newWorkspaceAccessPromptBridge(func() sessionPermissionRequester { return requester }, time.Second,
						newWorkspaceAccessConsentCache(), grants, slog.New(slog.DiscardHandler)))
				service, err := monitorpkg.NewService(t.Context(), nil, retryMonitorWaker{},
					monitorpkg.WithSessionTargets(monitorSessionTargets{sessions: func() any { return sessions }}))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := service.Shutdown(context.WithoutCancel(t.Context())); err != nil {
						t.Error(err)
					}
				})
				deps := &daemonNativeToolsDeps{
					Sessions:        sessions,
					Workspaces:      nativeTestWorkspaceService(t),
					WorkspaceAccess: policy,
					ApprovalGrants:  grants,
					Monitors:        func() *monitorpkg.Service { return service },
				}
				registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(t, deps,
					toolspkg.NewStaticPolicyInputResolver(nativeApproveAllPolicyInputs()), policy,
					toolspkg.WithCallInputBinder(binder), toolspkg.WithCallInputAuthorizer(binder))
				scope := actorScope
				scope.ProfileID = "p-x"
				if _, err := registry.Call(
					t.Context(),
					scope,
					toolspkg.CallRequest{
						ToolID: toolspkg.ToolIDMonitorCreate,
						Input: json.RawMessage(
							`{"kind":"session","description":"watch work","target_session_ids":["sess-sibling"]}`,
						),
					},
				); err != nil {
					t.Fatal(err)
				}
				if len(requester.requests) != 1 {
					t.Fatalf("prompts = %d, want one", len(requester.requests))
				}
				mon := service.List(scope.SessionID)[0]
				if len(mon.TargetScopes) != 1 || mon.TargetScopes[0].WorkspaceID != foreignWorkspace ||
					mon.TargetScopes[0].ProfileID != scope.ProfileID {
					t.Fatalf("captured scope = %+v", mon.TargetScopes)
				}
				mon.Targets = []monitorpkg.TargetStatus{
					{SessionID: "sess-sibling", Name: "deploy", State: "idle", Matched: true},
				}
				n := &daemonNativeTools{deps: deps}
				check := func(wantVisible bool) {
					t.Helper()
					visible, err := n.visibleMonitor(t.Context(), scope.SessionID, mon)
					if err != nil {
						t.Fatal(err)
					}
					if visible.Targets[0].Matched != wantVisible ||
						(visible.Targets[0].State == "idle") != wantVisible {
						t.Fatalf("visible monitor = %+v, want visible %t", visible, wantVisible)
					}
				}
				check(choice == workspaceAccessAllowAlwaysID)
				if choice != workspaceAccessAllowAlwaysID {
					return
				}
				// A different tool's grant, removed grant, lookup error, or new owner cannot authorize the watch.
				saved := grants.grants[0]
				grants.grants[0].ToolID = toolspkg.ToolIDSessionStatus
				check(false)
				grants.grants = nil
				check(false)
				grants.grants = []toolspkg.ApprovalGrant{saved}
				grants.lookupErr = errors.New("unavailable")
				check(false)
				grants.lookupErr = nil
				original := sessions.StatusFn
				moved := *sessions
				moved.StatusFn = func(ctx context.Context, id string) (*session.Info, error) {
					info, err := original(ctx, id)
					if id == "sess-sibling" && info != nil {
						info.ProfileID = "p-y"
					}
					return info, err
				}
				deps.Sessions = &moved
				check(false)
				deps.Sessions = sessions
				// Hard policy refusals dominate a saved read grant.
				deps.WorkspaceAccess = newPolicy(t, sessionsFor(workspaceaccess.ModeDenyAll))
				check(false)
				consent := newWorkspaceAccessConsentCache()
				consent.PutConsent(t.Context(), scope.SessionID, workspaceaccess.ConsentReject)
				facts, err := newWorkspaceAccessModeSource(sessions)
				if err != nil {
					t.Fatal(err)
				}
				deps.WorkspaceAccess, err = workspaceaccess.New(workspaceaccess.Deps{Facts: facts, Consent: consent,
					Audit: nativeSessionTargetAuditStub{}, Log: slog.New(slog.DiscardHandler)})
				if err != nil {
					t.Fatal(err)
				}
				check(false)
			})
		}
	})
	t.Run("Should preserve authorized global and all task catalog scopes through dispatch", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		status := sessions.StatusFn
		sessions.StatusFn = func(ctx context.Context, id string) (*session.Info, error) {
			info, err := status(ctx, id)
			if info != nil {
				info.ProfileID = store.DefaultProfileID
			}
			return info, err
		}
		policy := newPolicy(t, sessions)
		db := openDaemonTestGlobalDB(t)
		now := time.Now().UTC()
		if err := db.InsertWorkspace(t.Context(), workspacepkg.Workspace{
			ID: homeWorkspace, Name: "catalog", RootDir: t.TempDir(), CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		tasks, err := taskpkg.NewManager(taskpkg.WithStore(db), taskpkg.WithWorkspaceAccessPolicy(policy))
		if err != nil {
			t.Fatal(err)
		}
		operator, err := taskpkg.DeriveHumanActorContext("fixture", taskpkg.OriginKindCLI, "task.create")
		if err != nil {
			t.Fatal(err)
		}
		global, err := tasks.CreateTask(t.Context(), taskpkg.CreateTask{
			ProfileID: store.DefaultProfileID, Scope: taskpkg.ScopeGlobal, Title: "User work",
		}, operator)
		if err != nil {
			t.Fatal(err)
		}
		project, err := tasks.CreateTask(t.Context(), taskpkg.CreateTask{
			ProfileID: store.DefaultProfileID, Scope: taskpkg.ScopeWorkspace,
			WorkspaceID: homeWorkspace, Title: "Project work",
		}, operator)
		if err != nil {
			t.Fatal(err)
		}
		registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(t, &daemonNativeToolsDeps{
			Sessions: sessions, Tasks: tasks, Workspaces: nativeTestWorkspaceService(t),
		}, toolspkg.NewStaticPolicyInputResolver(nativeApproveAllPolicyInputs()), policy)
		for _, requested := range []string{"global", "all"} {
			t.Run("Should preserve task catalog scope "+requested, func(t *testing.T) {
				result, err := registry.Call(t.Context(), actorScope, toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDTaskList,
					Input:  json.RawMessage(`{"scope":"` + requested + `","include_drafts":true}`),
				})
				if err != nil {
					t.Fatal(err)
				}
				var payload contract.TasksResponse
				if err := json.Unmarshal(result.Structured, &payload); err != nil {
					t.Fatal(err)
				}
				want := 1
				if requested == "all" {
					want = 2
					requireNativeStructuredContains(t, result, []byte(project.ID))
				}
				if len(payload.Tasks) != want || payload.Page.Total != want {
					t.Fatalf("task catalog %s = %#v", requested, payload)
				}
				requireNativeStructuredContains(t, result, []byte(global.ID))
			})
		}
	})
	authorize := func(
		t *testing.T,
		binder *nativeWorkspaceInputBinder,
		input string,
	) (json.RawMessage, error) {
		t.Helper()
		bound, err := binder.BindCallInput(t.Context(), actorScope, statusDescriptor, json.RawMessage(input))
		if err != nil {
			return nil, err
		}
		return bound, binder.AuthorizeCallInput(t.Context(), actorScope, statusDescriptor, toolspkg.CallRequest{
			ToolID: statusDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
		})
	}

	t.Run("Should resolve an omitted workspace from a foreign session under approve-all", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		for input, wantWorkspace := range map[string]string{
			`{"session_id":"sess-foreign"}`: foreignWorkspace,
			`{"session_id":"sess-user"}`:    workspaceaccess.UserSelector,
		} {
			bound, err := authorize(t, binder, input)
			if err != nil {
				t.Fatalf("authorize(%s) error = %v, want approve-all reach", input, err)
			}
			if !strings.Contains(string(bound), `"workspace":"`+wantWorkspace+`"`) {
				t.Fatalf("bound input = %s, want the session's own workspace %q", bound, wantWorkspace)
			}
		}
	})

	t.Run("Should keep approve-all writes to another workspace of the same profile", func(t *testing.T) {
		t.Parallel()
		renameDescriptor := statusDescriptor
		renameDescriptor.ID, renameDescriptor.ReadOnly = toolspkg.ToolIDSessionRename, false
		write := func(t *testing.T, binder *nativeWorkspaceInputBinder, input string) error {
			t.Helper()
			bound, err := binder.BindCallInput(t.Context(), actorScope, renameDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), actorScope, renameDescriptor, toolspkg.CallRequest{
				ToolID: renameDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
			})
		}
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		if err := write(t, binder, `{"session_id":"sess-sibling"}`); err != nil {
			t.Fatalf("same-profile cross-workspace write error = %v, want the ADR-007 approve-all reach", err)
		}
		if err := write(t, binder, `{"session_id":"sess-missing"}`); err != nil {
			t.Fatalf("missing-session write error = %v, want the handler to report not found", err)
		}
		err := write(t, binder, `{"session_id":"sess-foreign"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if !strings.Contains(err.Error(), "workspace_access: unrestricted") {
			t.Fatalf("cross-profile write error = %v, want the grant named", err)
		}
		denySessions := sessionsFor(workspaceaccess.ModeDenyAll)
		denied := newNativeWorkspaceInputBinder(
			nativeTestWorkspaceService(t),
			denySessions,
			newPolicy(t, denySessions),
			nil,
		)
		foreignErr, missingErr := write(
			t,
			denied,
			`{"session_id":"sess-foreign"}`,
		), write(
			t,
			denied,
			`{"session_id":"sess-missing"}`,
		)
		if foreignErr == nil || missingErr == nil || foreignErr.Error() != missingErr.Error() {
			t.Fatalf("deny-all write denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
	})

	t.Run("Should treat another profile's session in the same project as a crossing for fork", func(t *testing.T) {
		t.Parallel()
		forkDescriptor := statusDescriptor
		forkDescriptor.ID, forkDescriptor.ReadOnly = toolspkg.ToolIDSessionFork, false
		fork := func(t *testing.T, binder *nativeWorkspaceInputBinder, input string) error {
			t.Helper()
			bound, err := binder.BindCallInput(t.Context(), actorScope, forkDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), actorScope, forkDescriptor, toolspkg.CallRequest{
				ToolID: forkDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
			})
		}
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		if err := fork(t, binder, `{"session_id":"sess-own"}`); err != nil {
			t.Fatalf("same-profile fork error = %v, want the caller's own work to fork", err)
		}
		// The derive manager checks only the workspace of a project source, so this boundary is
		// what keeps one profile from copying another profile's transcript in a shared project.
		err := fork(t, binder, `{"session_id":"sess-peer"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
	})

	t.Run("Should answer a denied caller the same for foreign and missing sessions", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeDenyAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		_, foreignErr := authorize(t, binder, `{"session_id":"sess-foreign"}`)
		_, missingErr := authorize(t, binder, `{"session_id":"sess-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		requireToolReason(t, missingErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if foreignErr.Error() != missingErr.Error() {
			t.Fatalf("denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
		if !strings.Contains(foreignErr.Error(), "deny-all") || strings.Contains(foreignErr.Error(), foreignWorkspace) {
			t.Fatalf("denial = %v, want the deny-all axis without the target workspace", foreignErr)
		}
		public := core.ToolErrorResponseForError(foreignErr, http.StatusForbidden, true).Error.Message
		if !strings.Contains(public, "deny-all") {
			t.Fatalf("public denial message = %q, want the axis guidance on every transport", public)
		}
		if _, err := authorize(t, binder, `{"session_id":"sess-own"}`); err != nil {
			t.Fatalf("authorize(own session) error = %v, want own workspace and profile unchanged", err)
		}
	})

	t.Run("Should deny @user probes from User scope like missing sessions", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeDenyAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		userScope := toolspkg.Scope{SessionID: "sess-user-actor", AgentName: "coder"}
		call := func(input string) error {
			bound, err := binder.BindCallInput(t.Context(), userScope, statusDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), userScope, statusDescriptor, toolspkg.CallRequest{
				ToolID: statusDescriptor.ID, SessionID: userScope.SessionID, Input: bound,
			})
		}
		foreignErr := call(`{"workspace":"@user","session_id":"sess-foreign"}`)
		missingErr := call(`{"workspace":"@user","session_id":"sess-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if missingErr == nil || foreignErr.Error() != missingErr.Error() {
			t.Fatalf("denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
	})

	t.Run("Should deny a foreign session whose project folder is gone like a missing one", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeDenyAll)
		workspaces := nativeTestWorkspaceService(t)
		resolve := workspaces.ResolveFn
		workspaces.ResolveFn = func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if ref == foreignWorkspace {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
			}
			return resolve(ctx, ref)
		}
		binder := newNativeWorkspaceInputBinder(workspaces, sessions, newPolicy(t, sessions), nil)
		historyDescriptor := statusDescriptor
		historyDescriptor.ID = toolspkg.ToolIDSessionHistory
		call := func(input string) error {
			bound, err := binder.BindCallInput(t.Context(), actorScope, historyDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), actorScope, historyDescriptor, toolspkg.CallRequest{
				ToolID: historyDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
			})
		}
		foreignErr, missingErr := call(`{"session_id":"sess-foreign"}`), call(`{"session_id":"sess-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if missingErr == nil || foreignErr.Error() != missingErr.Error() {
			t.Fatalf("denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
	})

	t.Run("Should prompt approve-reads identically for foreign and missing sessions", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveReads)
		requester := selectedPermissionRequester(workspaceAccessRejectOnceID)
		binder := newNativeWorkspaceInputBinder(
			nativeTestWorkspaceService(t),
			sessions,
			newPolicy(t, sessions),
			newWorkspaceAccessPromptBridge(
				func() sessionPermissionRequester { return requester },
				time.Second,
				newWorkspaceAccessConsentCache(),
				nil,
				nil,
			),
		)
		_, foreignErr := authorize(t, binder, `{"session_id":"sess-foreign"}`)
		_, missingErr := authorize(t, binder, `{"session_id":"sess-foreign-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if foreignErr.Error() != missingErr.Error() {
			t.Fatalf("rejections differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
		if len(requester.requests) != 2 {
			t.Fatalf("permission requests = %d, want one prompt per call", len(requester.requests))
		}
		for _, request := range requester.requests {
			title := *request.ToolCall.Title
			if !strings.HasPrefix(title, "Read session sess-foreign") || strings.Contains(title, foreignWorkspace) {
				t.Fatalf("prompt title = %q, want a session label without its owner", title)
			}
		}

		renameDescriptor := statusDescriptor
		renameDescriptor.ID = toolspkg.ToolIDSessionRename
		renameDescriptor.ReadOnly = false
		bound, err := binder.BindCallInput(
			t.Context(), actorScope, renameDescriptor, json.RawMessage(`{"session_id":"sess-sibling"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		renameErr := binder.AuthorizeCallInput(t.Context(), actorScope, renameDescriptor, toolspkg.CallRequest{
			ToolID: renameDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
		})
		requireToolReason(t, renameErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if len(requester.requests) != 3 {
			t.Fatalf("permission requests = %d, want the write prompted too", len(requester.requests))
		}
		if title := *requester.requests[2].ToolCall.Title; !strings.HasPrefix(title, "Change session sess-sibling") {
			t.Fatalf("write prompt title = %q, want the change named, not a read", title)
		}
	})

	t.Run("Should remember an operator Always allow for later reads of the same scope", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeAuto)
		grants := &recordingApprovalGrantStore{}
		requester := selectedPermissionRequester(workspaceAccessAllowAlwaysID)
		binder := newNativeWorkspaceInputBinder(
			nativeTestWorkspaceService(t),
			sessions,
			newPolicy(t, sessions),
			newWorkspaceAccessPromptBridge(
				func() sessionPermissionRequester { return requester },
				time.Second,
				newWorkspaceAccessConsentCache(),
				grants,
				slog.New(slog.DiscardHandler),
			),
		)
		scope := actorScope
		scope.ProfileID = "p-x"
		call := func(descriptor toolspkg.Descriptor, input string) error {
			bound, err := binder.BindCallInput(t.Context(), scope, descriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), scope, descriptor, toolspkg.CallRequest{
				ToolID: descriptor.ID, SessionID: scope.SessionID, Input: bound,
			})
		}
		requirePrompts := func(t *testing.T, want int) {
			t.Helper()
			if len(requester.requests) != want {
				t.Fatalf("permission requests = %d, want %d", len(requester.requests), want)
			}
		}

		if err := call(statusDescriptor, `{"session_id":"sess-sibling"}`); err != nil {
			t.Fatalf("first read error = %v, want the operator answer to admit it", err)
		}
		requirePrompts(t, 1)
		options := requester.requests[0].Options
		if len(options) < 2 || options[1].OptionId != workspaceAccessAllowAlwaysID ||
			options[1].Name != "Always allow" {
			t.Fatalf("read prompt options = %#v, want a remembered Always allow", options)
		}
		wantScope := "read:workspace:" + foreignWorkspace + "@p-x"
		if len(grants.grants) != 1 || grants.grants[0].AccessScope != wantScope ||
			grants.grants[0].AgentName != "coder" || grants.grants[0].ToolID != statusDescriptor.ID ||
			grants.grants[0].WorkspaceID != homeWorkspace || grants.grants[0].ProfileID != "p-x" {
			t.Fatalf("stored grants = %#v, want one %s grant for coder in its own workspace", grants.grants, wantScope)
		}

		toolInput, err := json.Marshal(map[string]string{"access_scope": wantScope})
		if err != nil {
			t.Fatal(err)
		}
		toolDigest, err := toolspkg.ApprovalInputDigest(toolInput, "")
		if err != nil || toolDigest == grants.grants[0].InputDigest {
			t.Fatalf("tool input digest = %q, %v, want a separate read-grant namespace", toolDigest, err)
		}

		// The same tool reading the same scope through a different input asks nobody.
		requester.response.Outcome = acpsdk.NewRequestPermissionOutcomeSelected(workspaceAccessRejectOnceID)
		if err := call(statusDescriptor, `{"session_id":"sess-sibling"}`); err != nil {
			t.Fatalf("repeat read error = %v, want the remembered grant", err)
		}
		if err := call(statusDescriptor, `{"workspace":"`+foreignWorkspace+`"}`); err != nil {
			t.Fatalf("workspace read error = %v, want the remembered grant", err)
		}
		requirePrompts(t, 1)

		// Recreating the bridge and rotating the actor session must not lose the grant.
		scope.SessionID = "sess-actor-next"
		binder.prompter = newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			newWorkspaceAccessConsentCache(),
			grants,
			slog.New(slog.DiscardHandler),
		)
		if err := call(statusDescriptor, `{"session_id":"sess-sibling-next"}`); err != nil {
			t.Fatalf("rotated session read error = %v, want the remembered grant", err)
		}
		requirePrompts(t, 1)

		// Another target scope and another tool stay outside the grant.
		err = call(statusDescriptor, `{"session_id":"sess-foreign"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		historyDescriptor := statusDescriptor
		historyDescriptor.ID = toolspkg.ToolIDSessionHistory
		err = call(historyDescriptor, `{"session_id":"sess-sibling"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		requirePrompts(t, 3)
		err = call(statusDescriptor, `{"workspace":"@user"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		requirePrompts(t, 4)

		// A write to the remembered scope still asks, and offers no durable answer.
		renameDescriptor := statusDescriptor
		renameDescriptor.ID, renameDescriptor.ReadOnly = toolspkg.ToolIDSessionRename, false
		err = call(renameDescriptor, `{"session_id":"sess-sibling"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		requirePrompts(t, 5)
		if option := requester.requests[4].Options[1]; option.OptionId != workspaceAccessAllowSessionID {
			t.Fatalf("write prompt allow option = %q, want the per-session answer", option.OptionId)
		}

		// Revocation restores the prompt.
		if err := grants.RevokeApprovalGrant(t.Context(), "p-x", homeWorkspace, grants.grants[0].ID); err != nil {
			t.Fatalf("RevokeApprovalGrant() error = %v", err)
		}
		err = call(statusDescriptor, `{"session_id":"sess-sibling"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		requirePrompts(t, 6)

		// A missing session asks the same question and leaves nothing behind.
		requester.response.Outcome = acpsdk.NewRequestPermissionOutcomeSelected(workspaceAccessAllowAlwaysID)
		if err := call(statusDescriptor, `{"session_id":"sess-missing"}`); err != nil {
			t.Fatalf("missing-session read error = %v, want the handler to report not found", err)
		}
		requirePrompts(t, 7)
		if requester.requests[6].Options[1].OptionId != workspaceAccessAllowAlwaysID {
			t.Fatalf("missing-session prompt options = %#v, want the found-session question",
				requester.requests[6].Options)
		}
		if len(grants.grants) != 0 {
			t.Fatalf("stored grants = %#v, want none for an unresolved target", grants.grants)
		}
	})

	t.Run("Should redact a seeded secret only when the read crosses the caller scope", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(
			t,
			&daemonNativeToolsDeps{
				Sessions: sessions, Workspaces: nativeTestWorkspaceService(t),
				Profiles: nativeProfileReaderStub{profiles: []profilepkg.WithCounts{
					{ID: "p-x", Name: "open-design", State: profilepkg.StateActive},
					{ID: "p-y", Name: "default", State: profilepkg.StateActive},
				}},
			},
			toolspkg.NewStaticPolicyInputResolver(nativeApproveAllPolicyInputs()),
			newPolicy(t, sessions),
		)
		read := func(sessionID string) string {
			result, err := registry.Call(t.Context(), toolspkg.Scope{SessionID: "sess-actor"}, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSessionStatus,
				Input:  json.RawMessage(`{"session_id":"` + sessionID + `"}`),
			})
			if err != nil {
				t.Fatalf("Registry.Call(session_status %s) error = %v", sessionID, err)
			}
			return string(result.Structured)
		}
		if foreign := read("sess-foreign"); strings.Contains(foreign, seededSecret) ||
			!strings.Contains(foreign, `"id":"sess-foreign"`) {
			t.Fatalf("foreign status = %s, want the session with its secret redacted", foreign)
		}
		if own := read("sess-own"); !strings.Contains(own, seededSecret) {
			t.Fatalf("own status = %s, want own-scope reads unchanged", own)
		}
	})
}

type nativeSessionTargetAuditStub struct{}

func (nativeSessionTargetAuditStub) EmitWorkspaceAccess(context.Context, workspaceaccess.AccessRecord) error {
	return nil
}
