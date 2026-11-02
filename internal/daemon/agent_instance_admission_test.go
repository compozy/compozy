package daemon

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store/globaldb"
	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type instanceAdmissionInputs struct {
	definition instanceDefinition
	caller     agentinstance.Caller
	reachErr   error
	session    store.SessionInfo
	policy     store.SessionPermissionPolicy
}

func (i *instanceAdmissionInputs) AdoptionSession(
	context.Context,
	string,
) (store.SessionInfo, store.SessionPermissionPolicy, error) {
	return i.session, i.policy, nil
}

func TestInstanceAdoptAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, kind, agent, profile, workspace, parent, grant, want string
		operator, wider                                            bool
	}{
		{name: "Should adopt an eligible root", kind: "stays_on", agent: "coder", profile: "profile", workspace: "workspace", operator: true},
		{name: "Should reject agent adoption", want: "agent_instance_forbidden"},
		{name: "Should reject on demand adoption", kind: "on_demand", agent: "coder", profile: "profile", workspace: "workspace", operator: true, want: "agent_not_stays_on"},
		{name: "Should reject child adoption", parent: "parent", operator: true, want: "session_not_root"},
		{name: "Should reject cross agent adoption", agent: "other", operator: true, want: "session_scope_mismatch"},
		{name: "Should reject cross profile adoption", agent: "coder", profile: "other", operator: true, want: "session_scope_mismatch"},
		{name: "Should reject cross workspace adoption", agent: "coder", profile: "profile", workspace: "other", operator: true, want: "session_scope_mismatch"},
		{name: "Should reject wider permissions", kind: "stays_on", agent: "coder", profile: "profile", workspace: "workspace", operator: true, wider: true, want: "workspace_access_denied"},
		{name: "Should reject wider workspace grant", kind: "stays_on", agent: "coder", profile: "profile", workspace: "workspace", operator: true, grant: "unrestricted", want: "workspace_access_denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inputs := &instanceAdmissionInputs{
				definition: instanceDefinition{
					Kind:   tc.kind,
					Policy: store.SessionPermissionPolicy{Tools: []string{"read"}},
				},
				policy: store.SessionPermissionPolicy{Tools: []string{"read"}},
				session: store.SessionInfo{SessionType: "user", AgentName: tc.agent, ProfileID: tc.profile,
					WorkspaceID: tc.workspace, EffectiveWorkspaceAccess: tc.grant,
					Lineage: &store.SessionLineage{ParentSessionID: tc.parent}},
			}
			if tc.wider {
				inputs.policy.Tools = append(inputs.policy.Tools, "write")
			}
			admission := &instanceAdmission{definitions: inputs, sessions: inputs, reach: inputs}
			got, err := admission.ResolveAdopt(t.Context(), agentinstance.AdoptRequest{
				Agent: "coder", Profile: "profile", Workspace: "workspace",
				Actor: workspaceaccess.ActorRef{Operator: tc.operator}, SessionID: "candidate",
			})
			if tc.want != "" {
				if err == nil || err.Error() != tc.want {
					t.Fatalf("adoption error = %v, want %s", err, tc.want)
				}
				return
			}
			if err != nil || got.AgentName != "coder" || got.WorkspaceID != "workspace" {
				t.Fatalf("adoption: %#v, %v", got, err)
			}
		})
	}
}

func (i *instanceAdmissionInputs) InstanceDefinition(
	context.Context,
	string,
	string,
	string,
) (instanceDefinition, error) {
	return i.definition, nil
}
func (i *instanceAdmissionInputs) InstanceCaller(context.Context, string) (agentinstance.Caller, error) {
	return i.caller, nil
}

func (i *instanceAdmissionInputs) AuthorizeInstanceWorkspace(
	context.Context,
	workspaceaccess.ActorRef,
	string,
	string,
) error {
	return i.reachErr
}

func TestInstanceCreateAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, workspace, wantGrant string
		operator                   bool
	}{
		{name: "Should deny unrestricted inheritance to agent created global instances"},
		{name: "Should deny unrestricted inheritance to agent created workspace instances", workspace: "workspace"},
		{name: "Should retain unrestricted for operator created global instances", operator: true, wantGrant: workspaceaccess.Unrestricted},
		{name: "Should restrict operator created workspace instances", operator: true, workspace: "workspace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inputs := &instanceAdmissionInputs{
				definition: instanceDefinition{Kind: "stays_on", Grant: workspaceaccess.Unrestricted},
				caller: agentinstance.Caller{
					AgentName: "coder",
					ProfileID: "profile",
					Grant:     workspaceaccess.Unrestricted,
				},
			}
			admission := &instanceAdmission{definitions: inputs, callers: inputs, reach: inputs}
			got, err := admission.ResolveCreate(t.Context(), agentinstance.CreateRequest{
				Agent:     "coder",
				Profile:   "profile",
				Workspace: tc.workspace,
				Actor: workspaceaccess.ActorRef{
					Kind:      workspaceaccess.ActorAgentSession,
					SessionID: "caller",
					Operator:  tc.operator,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.GrantCeiling != tc.wantGrant {
				t.Fatalf("grant = %q, want %q", got.GrantCeiling, tc.wantGrant)
			}
		})
	}

	denied := errors.New("workspace_access_denied")
	for _, tc := range []struct {
		name, kind, agent, profile string
		depth                      int
		reachErr, wantErr          error
	}{
		{"Should admit the same agent", "stays_on", "coder", "profile", 0, nil, nil},
		{"Should reject on demand definitions", "on_demand", "coder", "profile", 0, nil, agentinstance.ErrNotStaysOn},
		{"Should reject another agent", "stays_on", "other", "profile", 0, nil, agentinstance.ErrForbidden},
		{"Should reject another profile", "stays_on", "coder", "other", 0, nil, agentinstance.ErrForbidden},
		{"Should reject depth one callers", "stays_on", "coder", "profile", 1, nil, agentinstance.ErrDepth},
		{"Should reject callers below depth one", "stays_on", "coder", "profile", 2, nil, agentinstance.ErrDepth},
		{"Should retain workspace denial", "stays_on", "coder", "profile", 0, denied, denied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			narrow := store.SessionPermissionPolicy{
				Tools: []string{"read"}, Skills: []string{"review"},
				MCPServers: []string{"docs"}, WorkspacePaths: []string{"/project"},
			}
			inputs := &instanceAdmissionInputs{
				definition: instanceDefinition{
					Kind:  tc.kind,
					Grant: "unrestricted",
					Policy: store.SessionPermissionPolicy{
						Tools: []string{"read", "write"}, Skills: []string{"review", "deploy"},
						MCPServers: []string{"docs", "admin"}, WorkspacePaths: []string{"/project", "/other"},
					},
				},
				caller: agentinstance.Caller{AgentName: tc.agent, ProfileID: tc.profile,
					InstanceDepth: tc.depth, PermissionPolicy: narrow},
				reachErr: tc.reachErr,
			}
			admission := &instanceAdmission{definitions: inputs, callers: inputs, reach: inputs}
			var boundary agentinstance.Admission = admission
			got, err := boundary.ResolveCreate(t.Context(), agentinstance.CreateRequest{
				Agent: "coder", Profile: "profile", Workspace: "workspace",
				Actor: workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: "caller"},
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("admission error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			var policy store.SessionPermissionPolicy
			if err := json.Unmarshal([]byte(got.PermissionCeiling), &policy); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(policy, narrow) || got.GrantCeiling != "" || got.Depth != 1 {
				t.Fatalf("admission widened authority: %#v, policy %#v", got, policy)
			}
		})
	}
}

type nativeInstanceTargetStore struct {
	agentinstance.InstanceStore
	target agentinstance.Instance
}

var _ agentinstance.InstanceStore = nativeInstanceTargetStore{}

func (s nativeInstanceTargetStore) Get(context.Context, string) (agentinstance.Instance, error) {
	return s.target, nil
}

func TestNativeInstanceAuthority(t *testing.T) {
	for _, tc := range []struct {
		id    toolspkg.ToolID
		input string
	}{
		{toolspkg.ToolIDAgentInstancePause, `{"id":"target"}`},
		{toolspkg.ToolIDAgentInstanceEnsureMain, `{"id":"target"}`},
		{toolspkg.ToolIDAgentInstanceChecks, `{"id":"target","enabled":false}`},
		{toolspkg.ToolIDAgentInstanceCheck, `{"id":"target","dry_run":true}`},
	} {
		t.Run("Should retain foreign instance denial without approval for "+tc.id.String(), func(t *testing.T) {
			t.Parallel()
			api := &core.AgentInstanceAPI{
				Store: nativeInstanceTargetStore{
					target: agentinstance.Instance{ID: "target", AgentName: "coder", ProfileID: "profile"},
				},
				Authority: agentinstance.Authority{
					Callers: &instanceAdmissionInputs{
						caller: agentinstance.Caller{InstanceID: "own", AgentName: "coder", ProfileID: "profile"},
					},
				},
			}
			inputs := nativeApproveAllPolicyInputs()
			inputs.SystemPermissionMode = toolspkg.PermissionModeApproveReads
			inputs.ApprovalAvailable = false
			registry := newDaemonNativeRegistry(
				t,
				&daemonNativeToolsDeps{
					AgentInstances: func() *core.AgentInstanceAPI { return api },
					Sessions: &testutil.StubSessionManager{
						StatusFn: func(context.Context, string) (*session.Info, error) {
							return &session.Info{ID: "caller", ProfileID: "profile", AgentName: "coder"}, nil
						},
					},
				},
				inputs,
			)
			_, err := registry.Call(
				t.Context(),
				toolspkg.Scope{SessionID: "caller", AgentName: "coder", ProfileID: "profile"},
				toolspkg.CallRequest{ToolID: tc.id, Input: []byte(tc.input)},
			)
			if !errors.Is(err, agentinstance.ErrForbidden) {
				t.Fatalf("foreign instance error=%v", err)
			}
		})
	}
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstanceArchive, toolspkg.ToolIDAgentInstanceCheck} {
		t.Run(
			"Should admit approve-all without a receipt and still enforce ownership for "+id.String(),
			func(t *testing.T) {
				t.Parallel()
				api := &core.AgentInstanceAPI{
					Store: nativeInstanceTargetStore{
						target: agentinstance.Instance{ID: "target", AgentName: "coder", ProfileID: "profile"},
					},
					Authority: agentinstance.Authority{
						Callers: &instanceAdmissionInputs{
							caller: agentinstance.Caller{InstanceID: "own", AgentName: "coder", ProfileID: "profile"},
						},
					},
				}
				native := &daemonNativeTools{deps: &daemonNativeToolsDeps{
					AgentInstances: func() *core.AgentInstanceAPI { return api },
					Sessions: &testutil.StubSessionManager{
						StatusFn: func(context.Context, string) (*session.Info, error) {
							return &session.Info{EffectivePermissions: "approve-all"}, nil
						},
					},
				}}
				_, err := native.agentInstanceCall(
					t.Context(),
					toolspkg.Scope{SessionID: "caller", AgentName: "coder", ProfileID: "profile"},
					toolspkg.CallRequest{ToolID: id, Input: []byte(`{"id":"target"}`)},
				)
				if !errors.Is(err, agentinstance.ErrForbidden) {
					t.Fatalf("ownership after approval admission: %v", err)
				}
			},
		)
	}

	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstanceCreate, toolspkg.ToolIDAgentInstanceArchive} {
		t.Run("Should require approval for "+id.String(), func(t *testing.T) {
			t.Parallel()
			native := &daemonNativeTools{
				deps: &daemonNativeToolsDeps{
					AgentInstances: func() *core.AgentInstanceAPI { return &core.AgentInstanceAPI{} },
				},
			}
			_, err := native.agentInstanceCall(
				t.Context(),
				toolspkg.Scope{SessionID: "caller", AgentName: "coder", ProfileID: "profile"},
				toolspkg.CallRequest{ToolID: id, Input: []byte(`{}`)},
			)
			if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
				t.Fatalf("approval: %v", err)
			}
		})
	}
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstancePause, toolspkg.ToolIDAgentInstanceResume, toolspkg.ToolIDAgentInstanceArchive, toolspkg.ToolIDAgentInstanceEnsureMain, toolspkg.ToolIDAgentInstanceReports, toolspkg.ToolIDAgentInstanceGet, toolspkg.ToolIDAgentInstanceAway, toolspkg.ToolIDAgentInstanceSeen, toolspkg.ToolIDAgentInstanceCheckIns} {
		t.Run("Should deny another instance through "+id.String(), func(t *testing.T) {
			t.Parallel()
			api := &core.AgentInstanceAPI{
				Store: nativeInstanceTargetStore{
					target: agentinstance.Instance{ID: "target", AgentName: "coder", ProfileID: "profile"},
				},
				Authority: agentinstance.Authority{
					Callers: &instanceAdmissionInputs{
						caller: agentinstance.Caller{InstanceID: "own", AgentName: "coder", ProfileID: "profile"},
					},
				},
			}
			native := &daemonNativeTools{
				deps: &daemonNativeToolsDeps{AgentInstances: func() *core.AgentInstanceAPI { return api }},
			}
			_, err := native.agentInstanceCall(
				t.Context(),
				toolspkg.Scope{SessionID: "caller", AgentName: "coder", ProfileID: "profile"},
				toolspkg.CallRequest{ToolID: id, Input: []byte(`{"id":"target"}`), ApprovalGranted: true},
			)
			if !errors.Is(err, agentinstance.ErrForbidden) {
				t.Fatalf("authority: %v", err)
			}
		})
	}
}

func TestInstanceWorkspaceReach(t *testing.T) {
	t.Run("Should preserve the workspace denial code from the real admission boundary", func(t *testing.T) {
		t.Parallel()
		policy := &recordingNativeWorkspaceAccessPolicy{decision: workspaceaccess.Decision{Allowed: false}}
		reach := instanceWorkspaceReach{policy: policy}
		err := reach.AuthorizeInstanceWorkspace(
			t.Context(),
			workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: "caller"},
			"target",
			"profile",
		)
		if !errors.Is(err, agentinstance.ErrWorkspaceAccessDenied) {
			t.Fatalf("workspace denial: %v", err)
		}
		if policy.calls != 1 || policy.last.Seam != workspaceaccess.SeamSpawn {
			t.Fatalf("authorization request: %#v", policy.last)
		}
	})
}

func TestNativeInstanceWorkspaceAuthority(t *testing.T) {
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstancePause, toolspkg.ToolIDAgentInstanceResume, toolspkg.ToolIDAgentInstanceArchive, toolspkg.ToolIDAgentInstanceEnsureMain, toolspkg.ToolIDAgentInstanceReports, toolspkg.ToolIDAgentInstanceGet, toolspkg.ToolIDAgentInstanceCheckIns} {
		t.Run("Should deny an unreachable owned instance through "+id.String(), func(t *testing.T) {
			t.Parallel()
			target := agentinstance.Instance{ID: "own", AgentName: "coder", ProfileID: "profile", WorkspaceID: "target"}
			api := &core.AgentInstanceAPI{
				Store: nativeInstanceTargetStore{target: target},
				Authority: agentinstance.Authority{
					Callers: &instanceAdmissionInputs{
						caller: agentinstance.Caller{InstanceID: "own", AgentName: "coder", ProfileID: "profile"},
					},
				},
			}
			policy := &recordingNativeWorkspaceAccessPolicy{decision: workspaceaccess.Decision{Allowed: false}}
			native := &daemonNativeTools{
				deps: &daemonNativeToolsDeps{
					AgentInstances:  func() *core.AgentInstanceAPI { return api },
					WorkspaceAccess: policy,
				},
			}
			_, err := native.agentInstanceCall(
				t.Context(),
				toolspkg.Scope{SessionID: "caller", AgentName: "coder", ProfileID: "profile"},
				toolspkg.CallRequest{ToolID: id, Input: []byte(`{"id":"own"}`), ApprovalGranted: true},
			)
			if !errors.Is(err, agentinstance.ErrWorkspaceAccessDenied) {
				t.Fatalf("workspace authority: %v", err)
			}
			if policy.calls != 1 || policy.last.Seam != workspaceaccess.SeamTool {
				t.Fatalf("policy request: %#v", policy.last)
			}
		})
	}
	t.Run("Should refuse native resume after a manual pause", func(t *testing.T) {
		t.Parallel()
		api := &core.AgentInstanceAPI{
			Store: nativeInstanceTargetStore{
				target: agentinstance.Instance{
					ID:          "own",
					AgentName:   "coder",
					ProfileID:   "profile",
					State:       "paused",
					PauseReason: "manual",
				},
			},
			Authority: agentinstance.Authority{
				Callers: &instanceAdmissionInputs{
					caller: agentinstance.Caller{InstanceID: "own", AgentName: "coder", ProfileID: "profile"},
				},
			},
		}
		native := &daemonNativeTools{
			deps: &daemonNativeToolsDeps{AgentInstances: func() *core.AgentInstanceAPI { return api }},
		}
		_, err := native.agentInstanceCall(
			t.Context(),
			toolspkg.Scope{SessionID: "caller", AgentName: "coder", ProfileID: "profile"},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDAgentInstanceResume, Input: []byte(`{"id":"own"}`)},
		)
		if !errors.Is(err, agentinstance.ErrForbidden) {
			t.Fatalf("manual resume: %v", err)
		}
	})
}

func TestInstanceMainReadModel(t *testing.T) {
	for _, tc := range []struct {
		name string
		info session.Info
		want string
	}{
		{"Should show an idle main", session.Info{State: session.StateActive}, "idle"},
		{"Should show a starting main as unknown", session.Info{State: session.StateStarting}, "unknown"},
		{"Should show a failed main", session.Info{State: session.StateActive, Failure: &store.SessionFailure{}}, "failed"},
		{"Should keep a canceled main idle after a clean stop", session.Info{State: session.StateStopped, Failure: &store.SessionFailure{Kind: store.FailureCanceled}}, "idle"},
		{"Should prioritize pending permission over failure", session.Info{State: session.StateActive, Failure: &store.SessionFailure{}, PendingPermissionCount: 1}, "waiting"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := contract.AgentInstancePayload{}
			projectInstanceMain(&payload, &tc.info)
			if payload.Activity != tc.want {
				t.Fatalf("activity=%q want %q", payload.Activity, tc.want)
			}
		})
	}

	t.Run("Should bound the latest activity to eighty characters on one line", func(t *testing.T) {
		t.Parallel()
		payload := contract.AgentInstancePayload{}
		info := &session.Info{
			Provider: "claude",
			State:    session.StateActive,
			Liveness: &store.SessionLivenessMeta{
				Activity: &store.SessionActivityMeta{
					TurnID:             "turn",
					LastActivityDetail: "  Working\n" + strings.Repeat("界", 90),
				},
			},
		}
		projectInstanceMain(&payload, info)
		if utf8.RuneCountInString(payload.Now) != 80 || strings.Contains(payload.Now, "\n") ||
			payload.Activity != "working" ||
			payload.Provider != "claude" {
			t.Fatalf("unexpected projection: %+v", payload)
		}
	})
	t.Run("Should expose a pending interaction as waiting", func(t *testing.T) {
		t.Parallel()
		payload := contract.AgentInstancePayload{}
		projectInstanceMain(&payload, &session.Info{State: session.StateActive, PendingClarifyCount: 1})
		if payload.Activity != "waiting" {
			t.Fatalf("activity = %q", payload.Activity)
		}
	})
}

type rotationEligibilityFixture struct {
	staticLoopSessionEventReader
	info     *session.Info
	monitors []monitor.Monitor
	ready    bool
}

func (f rotationEligibilityFixture) Status(context.Context, string) (*session.Info, error) {
	return f.info, nil
}
func (f rotationEligibilityFixture) List(string) []monitor.Monitor { return f.monitors }
func (f rotationEligibilityFixture) RotationAdmissionCount(context.Context, string) (int64, error) {
	return 7, nil
}
func (f rotationEligibilityFixture) RotationPersistenceReady(context.Context, string) (bool, error) {
	return f.ready, nil
}

func TestInstanceRotationEligibility(t *testing.T) {
	for _, tc := range []struct {
		name    string
		used    int64
		unknown bool
		compact bool
		monitor monitor.Status
		blocked bool
		want    bool
	}{
		{name: "Should rotate at the configured threshold", used: 80, want: true},
		{name: "Should wait below the threshold", used: 79},
		{name: "Should wait for known context usage", unknown: true},
		{name: "Should leave compaction to the provider", used: 90, compact: true},
		{name: "Should wait for active monitors", used: 90, monitor: monitor.StatusActive},
		{name: "Should wait for monitors retrying delivery", used: 90, monitor: monitor.StatusDelivering},
		{name: "Should ignore completed monitor history", used: 90, monitor: monitor.StatusFired, want: true},
		{name: "Should wait for persisted dependencies", used: 90, blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			size := int64(100)
			f := rotationEligibilityFixture{info: &session.Info{State: session.StateActive}, ready: !tc.blocked}
			if !tc.unknown {
				f.events = []store.SessionEvent{
					managedGoalContextEvent(
						t,
						1,
						acp.AgentEvent{
							Type:  acp.EventTypeUsage,
							Usage: &acp.TokenUsage{ContextUsed: &tc.used, ContextSize: &size},
						},
					),
				}
			}
			if tc.compact {
				f.info.AdvertisedCommands = []store.SessionAdvertisedCommand{{Name: "compact"}}
			}
			if tc.monitor != "" {
				f.monitors = []monitor.Monitor{{Status: tc.monitor}}
			}
			r := instanceMainRotator{sessions: f, store: f, monitors: f, threshold: 0.8, now: time.Now}
			got, err := r.Eligible(
				t.Context(),
				agentinstance.Instance{ID: "instance", MainSessionID: "main", State: "active"},
			)
			if err != nil || got.Eligible != tc.want || got.Publication.AdmissionCount != 7 {
				t.Fatalf("eligibility = %#v, %v", got, err)
			}
		})
	}
}

func (f rotationEligibilityFixture) SupersededSessions(context.Context, string) ([]string, error) {
	return nil, nil
}
func (f rotationEligibilityFixture) Stop(context.Context, string) error { return nil }

type rotationFlowFixture struct {
	rotationEligibilityFixture
	agentinstance.ReportStore
	startErr, publishErr, stopErr error
	published                     bool
	stopped                       []string
	reports                       []agentinstance.ReportInput
}

func (f *rotationFlowFixture) StartSuccessor(context.Context, agentinstance.Instance, string) (string, error) {
	return "candidate", f.startErr
}
func (f *rotationFlowFixture) PublishRotation(context.Context, agentinstance.RotationPublication) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.published = true
	return nil
}
func (f *rotationFlowFixture) PublishInstanceRotation(_ context.Context, _ string, publish func() error) error {
	return publish()
}
func (f *rotationFlowFixture) RetireSession(_ string, publish func() error) error { return publish() }
func (f *rotationFlowFixture) Stop(_ context.Context, id string) error {
	f.stopped = append(f.stopped, id)
	return f.stopErr
}
func (f *rotationFlowFixture) SupersededSessions(context.Context, string) ([]string, error) {
	if f.published {
		return []string{"main"}, nil
	}
	return nil, nil
}

func (f *rotationFlowFixture) AppendReport(
	_ context.Context,
	_ string,
	input agentinstance.ReportInput,
) (agentinstance.Report, error) {
	if err := input.Validate(); err != nil {
		return agentinstance.Report{}, err
	}
	f.reports = append(f.reports, input)
	return agentinstance.Report{}, nil
}

func TestInstanceRotationFlow(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		start, publish, stop bool
	}{
		{name: "Should publish a successor and stop only the predecessor"},
		{name: "Should stop a failed candidate and report without publication", start: true},
		{name: "Should stop a candidate when publication is refused", publish: true},
		{name: "Should retain publication when predecessor cleanup fails", stop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &rotationFlowFixture{}
			failure := errors.New("injected boundary failure")
			if tc.start {
				f.startErr = failure
			}
			if tc.publish {
				f.publishErr = failure
			}
			if tc.stop {
				f.stopErr = failure
			}
			r := instanceMainRotator{
				sessions: f,
				store:    f,
				monitors: f,
				starter:  f,
				reports:  f,
				now:      func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) },
			}
			var changes []agentinstance.Instance
			r.changed = func(instance agentinstance.Instance) {
				if !f.published || len(f.stopped) != 0 {
					t.Fatal("catalog change must follow publication and precede cleanup")
				}
				changes = append(changes, instance)
			}
			err := r.Rotate(
				t.Context(),
				agentinstance.Instance{ID: "instance", MainSessionID: "main"},
				agentinstance.RotationDecision{
					Eligible:    true,
					Publication: agentinstance.RotationPublication{InstanceID: "instance", PredecessorID: "main"},
				},
			)
			failed := tc.start || tc.publish || tc.stop
			if (err != nil) != failed {
				t.Fatalf("rotation error = %v", err)
			}
			wantPublished := !tc.start && !tc.publish
			if wantPublished {
				if len(changes) != 1 || changes[0].ID != "instance" || changes[0].MainSessionID != "candidate" {
					t.Fatalf("catalog changes = %#v", changes)
				}
			} else if len(changes) != 0 {
				t.Fatalf("failed publication emitted catalog changes: %#v", changes)
			}
			wantStopped := "candidate"
			if wantPublished {
				wantStopped = "main"
			}
			if f.published != wantPublished || len(f.stopped) != 1 || f.stopped[0] != wantStopped {
				t.Fatalf("published %v stopped %v", f.published, f.stopped)
			}
			if wantPublished && len(f.reports) != 0 {
				t.Fatal("post-publication cleanup generated a pre-publication failure report")
			}
			if !wantPublished &&
				(len(f.reports) != 1 || f.reports[0].Kind != "rotation_failed" || f.reports[0].SourceKey != "rotation:failed:main") {
				t.Fatalf("failure reports = %v", f.reports)
			}
		})
	}
}

func (f rotationEligibilityFixture) RotationWaitingSince(
	_ context.Context,
	_, _ string,
	now time.Time,
) (time.Time, error) {
	return now, nil
}
func (f rotationEligibilityFixture) ClearRotationWait(context.Context, string, string) error {
	return nil
}

func TestInstanceRotationDeferral(t *testing.T) {
	t.Run("Should preserve waiting across reopen and report only once after one hour", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "rotation.db")
		db, err := globaldb.OpenGlobalDB(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instance, _, err := db.Instances().
			AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.RegisterInstanceSession(
			t.Context(),
			new(store.SessionInfo{
				ID:            "main",
				ProfileID:     store.DefaultProfileID,
				AgentName:     "coder",
				State:         "active",
				RuntimeStatus: store.SessionRuntimeUnbound,
				SessionType:   "user",
			}),
			instance.ID,
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		instance, err = db.Instances().BindMain(t.Context(), instance.ID, "main")
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		r := instanceMainRotator{
			store:   db.Instances(),
			reports: db.InstanceReports(),
			now:     func() time.Time { return now },
		}
		if err := r.recordRotationDeferral(t.Context(), instance, false); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(t.Context()); err != nil {
			t.Fatal(err)
		}
		db, err = globaldb.OpenGlobalDB(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		r.store, r.reports = db.Instances(), db.InstanceReports()
		now = now.Add(time.Hour - time.Second)
		if err := r.recordRotationDeferral(t.Context(), instance, false); err != nil {
			t.Fatal(err)
		}
		reports, err := db.InstanceReports().
			ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 100})
		if err != nil || len(reports) != 0 {
			t.Fatalf("early reports = %v, %v", reports, err)
		}
		now = now.Add(time.Second)
		for range 2 {
			if err := r.recordRotationDeferral(t.Context(), instance, false); err != nil {
				t.Fatal(err)
			}
		}
		reports, err = db.InstanceReports().
			ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 100})
		if err != nil || len(reports) != 1 || reports[0].Kind != "rotation_deferred" {
			t.Fatalf("reports = %v, %v", reports, err)
		}
		if err := r.recordRotationDeferral(t.Context(), instance, true); err != nil {
			t.Fatal(err)
		}
		since, err := db.Instances().RotationWaitingSince(t.Context(), instance.ID, "main", now)
		if err != nil || !since.Equal(now) {
			t.Fatalf("new waiting interval = %v, %v", since, err)
		}
	})
}

func TestInstanceLateChildReentry(t *testing.T) {
	t.Run("Should resolve a stopped predecessor to the current instance main", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		instance, _, err := db.Instances().AdmitCreate(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder",
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.RegisterInstanceSession(t.Context(), new(store.SessionInfo{
			ID: "successor", ProfileID: store.DefaultProfileID, AgentName: "coder",
			State: "active", RuntimeStatus: store.SessionRuntimeUnbound, SessionType: "user",
		}), instance.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().BindMain(t.Context(), instance.ID, "successor"); err != nil {
			t.Fatal(err)
		}
		bridge := harnessReentryBridge{
			ctx:   t.Context(),
			store: db,
			sessions: &fakeSessionManager{infos: []*session.Info{
				{ID: "predecessor", InstanceID: instance.ID, State: session.StateStopped},
				{ID: "successor", InstanceID: instance.ID, State: session.StateActive, AgentName: "coder"},
			}},
		}
		metadata := detachedHarnessRunMetadata{}
		metadata.WakeTarget.SessionID = "predecessor"
		target := bridge.resolveWakeTargetSnapshot(metadata)
		if target.Missing || target.SessionID != "successor" || target.State != session.StateActive {
			t.Fatalf("wake target = %+v, want active successor", target)
		}
		item := harnessSyntheticWake{targetSessionID: "predecessor"}
		if !bridge.refreshWakeTarget(&item) || item.targetSessionID != "successor" {
			t.Fatalf("queued wake = %+v, want successor at dispatch", item)
		}
		if bridge.refreshWakeTarget(&item) {
			t.Fatal("unchanged pointer unexpectedly rerouted wake")
		}
	})
}

func TestNativeInstanceCheckIns(t *testing.T) {
	t.Run(
		"Should page persisted wake events through the native tool without leaking another instance",
		func(t *testing.T) {
			t.Parallel()
			db := openDaemonTestGlobalDB(t)
			var instances []agentinstance.Instance
			for _, agent := range []string{"coder", "reviewer"} {
				instance, _, err := db.Instances().AdmitCreate(t.Context(), agentinstance.Instance{
					AgentName: agent, ProfileID: store.DefaultProfileID,
					CreatedByKind: "human", PermissionCeiling: "{}",
				}, 10)
				if err != nil {
					t.Fatal(err)
				}
				instances = append(instances, instance)
			}
			for index, id := range []string{"event-a", "event-b", "event-other"} {
				instance := instances[0]
				if index == 2 {
					instance = instances[1]
				}
				_, err := db.DB().ExecContext(t.Context(), `INSERT INTO agent_heartbeat_wake_events
				(id,workspace_id,agent_name,instance_id,source,result,reason,created_at,expires_at)
				VALUES (?,'',?,?,'timer','skipped','quiet','2026-11-01T00:00:00Z','2026-11-02T00:00:00Z')`,
					id, instance.AgentName, instance.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			api := &core.AgentInstanceAPI{
				Store:     db.Instances(),
				Authority: agentinstance.Authority{Callers: db.Instances()},
			}
			native := &daemonNativeTools{
				deps: &daemonNativeToolsDeps{AgentInstances: func() *core.AgentInstanceAPI { return api }},
			}
			cursor := ""
			for _, want := range []string{"event-b", "event-a"} {
				input, err := json.Marshal(nativeInstanceInput{ID: instances[0].ID, Limit: 1, Cursor: cursor})
				if err != nil {
					t.Fatal(err)
				}
				result, err := native.agentInstanceCall(t.Context(), toolspkg.Scope{
					Operator: true, ProfileID: store.DefaultProfileID,
				}, toolspkg.CallRequest{ToolID: toolspkg.ToolIDAgentInstanceCheckIns, Input: input})
				if err != nil {
					t.Fatal(err)
				}
				var body contract.AgentInstanceCheckInsResponse
				if err := json.Unmarshal(result.Structured, &body); err != nil {
					t.Fatal(err)
				}
				if len(body.CheckIns) != 1 || body.CheckIns[0].ID != want || body.CheckIns[0].Reason != "quiet" ||
					body.CheckIns[0].Result != "skipped" {
					t.Fatalf("native check-ins = %+v, want %s", body, want)
				}
				cursor = body.NextCursor
			}
			if cursor != "" {
				t.Fatalf("terminal cursor = %q", cursor)
			}
		},
	)
}

func TestInstanceDefinitionPaths(t *testing.T) {
	t.Run("Should retain every resolved workspace directory in the instance ceiling", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		home := testHomePaths(t)
		root, additional := t.TempDir(), t.TempDir()
		now := time.Now().UTC()
		if err := db.InsertWorkspace(
			t.Context(),
			workspacepkg.Workspace{
				ID:             "ws-instance-paths",
				Name:           "instance-paths",
				RootDir:        root,
				AdditionalDirs: []string{additional},
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		); err != nil {
			t.Fatal(err)
		}
		resolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(home),
			workspacepkg.WithProfileConfigLoader(
				func(string, string) (compozyconfig.Config, error) { return compozyconfig.DefaultWithHome(home), nil },
			),
		)
		if err != nil {
			t.Fatal(err)
		}
		manager, err := session.NewManager(session.WithHomePaths(home))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		definitions := instanceDefinitions{
			workspaces: resolver,
			profiles:   nativeProfileReaderStub{},
			sessions:   manager,
			agents: stubSessionCommandAgentResolver(
				func(name string, _ *workspacepkg.ResolvedWorkspace) (compozyconfig.AgentDef, error) {
					return compozyconfig.AgentDef{
						Name:     name,
						Kind:     "stays_on",
						Provider: "claude",
						Prompt:   "Review work.",
					}, nil
				},
			),
		}
		definition, err := definitions.InstanceDefinition(
			t.Context(),
			store.DefaultProfileID,
			"ws-instance-paths",
			"coder",
		)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := resolver.ResolveForProfile(t.Context(), "ws-instance-paths", "default")
		if err != nil {
			t.Fatal(err)
		}
		want := store.NormalizeSessionPermissionPolicy(
			store.SessionPermissionPolicy{
				WorkspacePaths: append([]string{resolved.RootDir}, resolved.AdditionalDirs...),
			},
		)
		if len(want.WorkspacePaths) != 2 || !reflect.DeepEqual(definition.Policy.WorkspacePaths, want.WorkspacePaths) {
			t.Fatalf("definition paths = %v, want %v", definition.Policy.WorkspacePaths, want.WorkspacePaths)
		}
	})
}

type blockedInstanceReconciliation struct {
	agentinstance.InstanceStore
	agentinstance.Admission
	agentinstance.MainSessionStarter
	agentinstance.ReconciliationSessions
	started chan struct{}
	stopped chan struct{}
}

func (b *blockedInstanceReconciliation) ReconciliationPage(
	ctx context.Context,
	_ string,
	_ int,
) ([]agentinstance.Instance, error) {
	close(b.started)
	<-ctx.Done()
	close(b.stopped)
	return nil, ctx.Err()
}
func TestInstanceMaintenanceWorker(t *testing.T) {
	t.Run("Should isolate blocked reconciliation and join it on shutdown", func(t *testing.T) {
		t.Parallel()
		boundary := &blockedInstanceReconciliation{started: make(chan struct{}), stopped: make(chan struct{})}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(boundary),
			agentinstance.WithAdmission(boundary),
			agentinstance.WithMainSessions(boundary),
			agentinstance.WithActiveLimit(10),
		)
		if err != nil {
			t.Fatal(err)
		}
		state := &bootState{logger: slog.Default(), agentInstances: &core.AgentInstanceAPI{Service: service}}
		cleanup := &bootCleanup{}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if err := (&Daemon{}).bootInstanceMaintenance(ctx, state, cleanup, time.Hour); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			var errs []error
			state.runtimeWorkers.shutdown(context.Background(), &errs)
			if len(errs) > 0 {
				t.Error(errs)
			}
		})
		select {
		case <-boundary.started:
		case <-t.Context().Done():
			t.Fatal("maintenance did not start")
		}
		var errs []error
		state.runtimeWorkers.shutdown(t.Context(), &errs)
		if len(errs) > 0 {
			t.Fatal(errs)
		}
		select {
		case <-boundary.stopped:
		default:
			t.Fatal("shutdown did not join reconciliation")
		}
	})
}

func TestInstanceRotationRetry(t *testing.T) {
	t.Run("Should postpone failed rotation until the configured retry interval", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		used, size := int64(90), int64(100)
		f := &rotationFlowFixture{startErr: errors.New("start failed")}
		f.info = &session.Info{State: session.StateActive}
		f.ready = true
		f.events = []store.SessionEvent{
			managedGoalContextEvent(
				t,
				1,
				acp.AgentEvent{
					Type:  acp.EventTypeUsage,
					Usage: &acp.TokenUsage{ContextUsed: &used, ContextSize: &size},
				},
			),
		}
		r := instanceMainRotator{
			sessions:   f,
			store:      f,
			monitors:   f,
			starter:    f,
			reports:    f,
			now:        func() time.Time { return now },
			threshold:  0.8,
			retryDelay: time.Minute,
		}
		instance := agentinstance.Instance{ID: "instance", MainSessionID: "main", State: "active"}
		if err := r.Rotate(t.Context(), instance, agentinstance.RotationDecision{}); err == nil {
			t.Fatal("missing startup error")
		}
		decision, err := r.Eligible(t.Context(), instance)
		if err != nil || decision.Eligible {
			t.Fatalf("early retry = %#v, %v", decision, err)
		}
		now = now.Add(time.Minute)
		decision, err = r.Eligible(t.Context(), instance)
		if err != nil || !decision.Eligible {
			t.Fatalf("due retry = %#v, %v", decision, err)
		}
	})
}

func TestBootAgentInstances(t *testing.T) {
	t.Run("Should keep startup available when a paused instance project folder is missing", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		home := testHomePaths(t)
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		if err := db.InsertWorkspace(
			t.Context(),
			workspacepkg.Workspace{
				ID:        "missing",
				Name:      "missing",
				RootDir:   filepath.Join(t.TempDir(), "gone"),
				CreatedAt: now,
				UpdatedAt: now,
			},
		); err != nil {
			t.Fatal(err)
		}
		instance, _, err := db.Instances().
			AdmitCreate(t.Context(), agentinstance.Instance{AgentName: "coder", ProfileID: store.DefaultProfileID, WorkspaceID: "missing", CreatedByKind: "human", PermissionCeiling: "{}"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().
			ExecContext(t.Context(), "UPDATE agent_instances SET state='paused', pause_reason='manual' WHERE id=?", instance.ID); err != nil {
			t.Fatal(err)
		}
		resolver, err := workspacepkg.NewResolver(db, workspacepkg.WithHomePaths(home))
		if err != nil {
			t.Fatal(err)
		}
		profiles, err := profile.NewManager(profile.WithStore(db), profile.WithHomePaths(home))
		if err != nil {
			t.Fatal(err)
		}
		manager, err := session.NewManager(session.WithHomePaths(home))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		var logs bytes.Buffer
		state := &bootState{
			sessions:          manager,
			registry:          db,
			workspaceResolver: resolver,
			profiles:          profiles,
			cfg:               compozyconfig.DefaultWithHome(home),
			logger:            slog.New(slog.NewTextHandler(&logs, nil)),
		}
		if err := (&Daemon{now: func() time.Time { return now }}).bootAgentInstances(t.Context(), state); err != nil {
			t.Fatal(err)
		}
		if state.deps.AgentInstances == nil || !strings.Contains(logs.String(), instance.ID) {
			t.Fatalf("missing API or reconciliation diagnostic: %s", logs.String())
		}
		got, err := db.Instances().Get(t.Context(), instance.ID)
		if err != nil || got.State != "paused" || got.PauseReason != "manual" {
			t.Fatalf("instance = %+v, %v", got, err)
		}
	})
}
