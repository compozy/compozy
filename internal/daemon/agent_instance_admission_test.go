package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type instanceAdmissionInputs struct {
	definition instanceDefinition
	caller     agentinstance.Caller
	reachErr   error
	session    store.SessionInfo
	policy     store.SessionPermissionPolicy
}

func (i instanceAdmissionInputs) AdoptionSession(
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
			inputs := instanceAdmissionInputs{
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
				CreateRequest: agentinstance.CreateRequest{Agent: "coder", Profile: "profile", Workspace: "workspace",
					Actor: workspaceaccess.ActorRef{Operator: tc.operator}}, SessionID: "candidate",
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

func (i instanceAdmissionInputs) InstanceDefinition(
	context.Context,
	string,
	string,
	string,
) (instanceDefinition, error) {
	return i.definition, nil
}
func (i instanceAdmissionInputs) InstanceCaller(context.Context, string) (agentinstance.Caller, error) {
	return i.caller, nil
}

func (i instanceAdmissionInputs) AuthorizeInstanceWorkspace(
	context.Context,
	workspaceaccess.ActorRef,
	string,
	string,
) error {
	return i.reachErr
}

func TestInstanceCreateAdmission(t *testing.T) {
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
		{"Should retain workspace denial", "stays_on", "coder", "profile", 0, denied, denied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			narrow := store.SessionPermissionPolicy{
				Tools: []string{"read"}, Skills: []string{"review"},
				MCPServers: []string{"docs"}, WorkspacePaths: []string{"/project"},
			}
			inputs := instanceAdmissionInputs{
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
