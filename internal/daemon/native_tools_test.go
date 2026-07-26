package daemon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	devcycle "github.com/compozy/agh/extensions/dev-cycle"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apitest "github.com/compozy/agh/internal/api/testutil"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	hookspkg "github.com/compozy/agh/internal/hooks"
	mcppkg "github.com/compozy/agh/internal/mcp"
	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
	builtintools "github.com/compozy/agh/internal/tools/builtin"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	skillbundled "github.com/compozy/agh/skills"
)

const (
	nativeNetworkTestWorkspaceID         = "ws-native-network"
	nativeNetworkTestWorkspaceIdentityID = "01NATIVEWORKSPACEIDENTITY"
)

type nativeSessionPageHealthManager struct {
	apitest.StubSessionManager
	healthByID  map[string]heartbeat.SessionHealth
	healthCalls int
	healthInfos []*session.Info
}

type nativeClarifyBrokerStub struct{}

func (nativeClarifyBrokerStub) Ask(
	context.Context,
	toolspkg.Scope,
	toolspkg.ClarifyQuestion,
) (toolspkg.ClarifyAnswer, error) {
	choice := 0
	return toolspkg.ClarifyAnswer{Choice: &choice}, nil
}

func (nativeClarifyBrokerStub) Pending(
	context.Context,
	toolspkg.Scope,
) ([]toolspkg.ClarifyPending, error) {
	return nil, nil
}

func (nativeClarifyBrokerStub) Answer(
	context.Context,
	toolspkg.Scope,
	string,
	toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswer, error) {
	return toolspkg.ClarifyAnswer{}, errors.New("native clarify broker stub: unexpected Answer call")
}

func (m *nativeSessionPageHealthManager) SessionHealthForPage(
	_ context.Context,
	infos []*session.Info,
) (map[string]heartbeat.SessionHealth, error) {
	m.healthCalls++
	m.healthInfos = append([]*session.Info(nil), infos...)
	return m.healthByID, nil
}

func nativeNetworkTestWorkspaceService(t *testing.T) apitest.StubWorkspaceService {
	return nativeNetworkTestWorkspaceServiceWithIdentity(t, "")
}

func nativeNetworkTestWorkspaceServiceWithIdentity(
	t *testing.T,
	identityID string,
) apitest.StubWorkspaceService {
	t.Helper()

	root := t.TempDir()
	resolve := func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
		if err := ctx.Err(); err != nil {
			return workspacepkg.ResolvedWorkspace{}, err
		}
		workspaceID := strings.TrimSpace(ref)
		if workspaceID == "" {
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
		}
		resolvedIdentityID := strings.TrimSpace(identityID)
		if resolvedIdentityID == "" {
			resolvedIdentityID = workspaceID
		}
		return workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      workspaceID,
				RootDir: root,
				Name:    workspaceID,
			},
			WorkspaceID: resolvedIdentityID,
		}, nil
	}
	return apitest.StubWorkspaceService{
		GetFn: func(ctx context.Context, ref string) (workspacepkg.Workspace, error) {
			resolved, err := resolve(ctx, ref)
			return resolved.Workspace, err
		},
		ResolveFn: resolve,
	}
}

func nativeNetworkTestSessionManager(workspaceID string) apitest.StubSessionManager {
	return apitest.StubSessionManager{
		StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			sessionID := strings.TrimSpace(id)
			if sessionID == "" {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{
				ID:                   sessionID,
				WorkspaceID:          workspaceID,
				NetworkParticipation: daemonTestLiveParticipation(workspaceID, "builders"),
			}, nil
		},
	}
}

func TestDaemonNativeTools(t *testing.T) {
	t.Parallel()

	t.Run("Should execute window manager tools through scoped revisioned state", func(t *testing.T) {
		t.Parallel()

		manager, err := windowmanager.NewService(
			windowmanager.NewMemoryRepository(),
			windowmanager.NewMemoryWorkspaceResolver("workspace-a", "workspace-b"),
			nil,
			windowmanager.DefaultConfig(),
		)
		if err != nil {
			t.Fatalf("windowmanager.NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Close(); err != nil {
				t.Errorf("Manager.Close() error = %v", err)
			}
		})

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			WindowManager: manager,
			Workspaces:    nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{
			WorkspaceID: "workspace-a",
			SessionID:   "session-a",
			AgentName:   "agent-a",
		}

		createResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDDesktopCreate,
			Input: json.RawMessage(
				`{"expected_revision":0,"desktop_id":"desktop-build","name":"Build"}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(desktop_create) error = %v", err)
		}
		var created windowManagerCommandResult
		if err := json.Unmarshal(createResult.Structured, &created); err != nil {
			t.Fatalf("json.Unmarshal(desktop_create) error = %v", err)
		}
		if created.WorkspaceID != "workspace-a" || created.Revision != 1 || !created.Applied {
			t.Fatalf("desktop_create result = %#v, want applied workspace-a revision 1", created)
		}
		got := created.Changes.DesktopIDs
		want := []windowmanager.DesktopID{"desktop-build"}
		if !slices.Equal(got, want) {
			t.Fatalf("desktop_create changed desktops = %v, want %v", got, want)
		}
		if len(created.Diagnostics) != 0 {
			t.Fatalf("desktop_create diagnostics = %v, want none", created.Diagnostics)
		}

		listResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDDesktopList,
			Input:  json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(desktop_list) error = %v", err)
		}
		var listed windowManagerDesktopsResult
		if err := json.Unmarshal(listResult.Structured, &listed); err != nil {
			t.Fatalf("json.Unmarshal(desktop_list) error = %v", err)
		}
		if listed.WorkspaceID != "workspace-a" || listed.Revision != 1 || len(listed.Desktops) != 2 {
			t.Fatalf("desktop_list result = %#v, want two desktops at workspace-a revision 1", listed)
		}

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDLayoutArrange,
			Input:  json.RawMessage(`{"expected_revision":1,"resource_id":"missing"}`),
		})
		requireToolReason(t, err, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)

		previewResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDLayoutPreview,
			Input: json.RawMessage(
				`{"expected_revision":1,"command_id":"desktop.update","payload":{"desktop_id":"desktop-build","name":"Renamed"}}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(layout_preview) error = %v", err)
		}
		var previewed windowManagerPreviewResult
		if err := json.Unmarshal(previewResult.Structured, &previewed); err != nil {
			t.Fatalf("json.Unmarshal(layout_preview) error = %v", err)
		}
		if !previewed.Changed || previewed.Revision != 2 ||
			!slices.Equal(previewed.Changes.DesktopIDs, []windowmanager.DesktopID{"desktop-build"}) {
			t.Fatalf("layout_preview result = %#v, want changed revision 2 for desktop-build", previewed)
		}
		persisted, err := manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Manager.Snapshot() error = %v", err)
		}
		if persisted.Revision != 1 || persisted.Desktops[1].Name != "Build" {
			t.Fatalf("persisted snapshot = %#v, want unmodified revision 1", persisted)
		}

		openResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWindowOpen,
			Input: json.RawMessage(
				`{"expected_revision":1,"window_id":"window-tasks","app":"tasks",` +
					`"desktop_id":"desktop-build","route":{"pathname":"/tasks","search":{}},` +
					`"floating_rect":{"x":0.1,"y":0.1,"width":0.5,"height":0.5}}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(window_open) error = %v", err)
		}
		var opened windowManagerCommandResult
		if err := json.Unmarshal(openResult.Structured, &opened); err != nil {
			t.Fatalf("json.Unmarshal(window_open) error = %v", err)
		}
		if !opened.Applied || opened.Revision != 2 {
			t.Fatalf("window_open result = %#v, want applied revision 2", opened)
		}

		navigateResult, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWindowNavigate,
			Input: json.RawMessage(
				`{"expected_revision":2,"window_id":"window-tasks",` +
					`"route":{"pathname":"/tasks/detail","search":{"task_id":"task-123"}}}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(window_navigate) error = %v", err)
		}
		var navigated windowManagerCommandResult
		if err := json.Unmarshal(navigateResult.Structured, &navigated); err != nil {
			t.Fatalf("json.Unmarshal(window_navigate) error = %v", err)
		}
		if !navigated.Applied || navigated.Revision != 3 {
			t.Fatalf("window_navigate result = %#v, want applied revision 3", navigated)
		}
		persisted, err = manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Manager.Snapshot() after navigate error = %v", err)
		}
		route := persisted.Windows["window-tasks"].Route
		if route.Pathname != "/tasks/detail" || string(route.Search["task_id"]) != `"task-123"` {
			t.Fatalf("persisted route = %#v, want task detail", route)
		}

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWindowNavigate,
			Input: json.RawMessage(
				`{"expected_revision":3,"window_id":"window-tasks",` +
					`"route":{"pathname":"/tasks","search":[]}}`,
			),
		})
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDDesktopUpdate,
			Input: json.RawMessage(
				`{"expected_revision":0,"desktop_id":"desktop-build","name":"Stale"}`,
			),
		})
		requireToolReason(t, err, toolspkg.ErrToolConflict, toolspkg.ReasonConflictedID)

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDDesktopList,
			Input:  json.RawMessage(`{"workspace_id":"workspace-b"}`),
		})
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)

		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDDesktopList,
			Input:  json.RawMessage(`{"unknown":true}`),
		})
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
	})

	t.Run("Should reject layout apply fields absent from its descriptor", func(t *testing.T) {
		t.Parallel()
		var input windowManagerLayoutApplyInput
		err := decodeWindowManagerJSON(
			toolspkg.ToolIDLayoutApply,
			json.RawMessage(`{"expected_revision":0,"document":{},"rebase":{}}`),
			&input,
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
	})

	t.Run("Should reject conflicting native window group move fields", func(t *testing.T) {
		t.Parallel()
		native := &daemonNativeTools{}
		cases := []struct {
			name    string
			input   string
			message string
		}{
			{
				name: "Should require placement outside group mode",
				input: `{"expected_revision":0,"window_id":"window-a",` +
					`"destination_desktop_id":"desktop-b"}`,
				message: "placement is required unless move_group is true",
			},
			{
				name: "Should reject target in group mode",
				input: `{"expected_revision":0,"window_id":"window-a",` +
					`"destination_desktop_id":"desktop-b","move_group":true,` +
					`"target_window_id":"window-b"}`,
				message: "move_group cannot be combined with target_window_id",
			},
			{
				name: "Should reject placement in group mode",
				input: `{"expected_revision":0,"window_id":"window-a",` +
					`"destination_desktop_id":"desktop-b","move_group":true,"placement":"right"}`,
				message: "move_group cannot be combined with placement",
			},
			{
				name: "Should reject floating rectangle in group mode",
				input: `{"expected_revision":0,"window_id":"window-a",` +
					`"destination_desktop_id":"desktop-b","move_group":true,` +
					`"floating_rect":{"x":0,"y":0,"width":0.5,"height":0.5}}`,
				message: "move_group cannot be combined with floating_rect",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := native.windowMove(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDWindowMove,
					Input:  json.RawMessage(tc.input),
				})
				requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
				if !strings.Contains(err.Error(), tc.message) {
					t.Fatalf("windowMove() error = %v, want message containing %q", err, tc.message)
				}
			})
		}
	})

	t.Run("Should require approval before destructive window manager commands", func(t *testing.T) {
		t.Parallel()

		manager, err := windowmanager.NewService(
			windowmanager.NewMemoryRepository(),
			windowmanager.NewMemoryWorkspaceResolver("workspace-a"),
			nil,
			windowmanager.DefaultConfig(),
		)
		if err != nil {
			t.Fatalf("windowmanager.NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Close(); err != nil {
				t.Errorf("Manager.Close() error = %v", err)
			}
		})

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			WindowManager: manager,
			Workspaces:    nativeNetworkTestWorkspaceService(t),
		}, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    false,
		})
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "workspace-a", AgentName: "agent-a"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDDesktopDelete,
				Input: json.RawMessage(
					`{"expected_revision":0,"desktop_id":"desktop-default"}`,
				),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf("Registry.Call(desktop_delete approve-reads) error = %v, want ErrToolApprovalRequired", err)
		}
		snapshot, err := manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Manager.Snapshot() error = %v", err)
		}
		if snapshot.Revision != 0 || len(snapshot.Desktops) != 1 {
			t.Fatalf("snapshot after blocked delete = %#v, want untouched initial state", snapshot)
		}
	})

	t.Run("Should page retained tool results only within the caller workspace", func(t *testing.T) {
		t.Parallel()

		store := openDaemonTestToolArtifactStore(t)
		ref, err := store.Put(t.Context(), "workspace-a", []byte("abcdefgh"))
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			ToolArtifacts: store,
		}, nativeApproveAllPolicyInputs())
		result, err := registry.Call(t.Context(), toolspkg.Scope{WorkspaceID: "workspace-a"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolArtifactRead,
			Input: json.RawMessage(fmt.Sprintf(
				`{"artifact_uri":%q,"offset":2,"limit":3}`,
				ref.URI,
			)),
		})
		if err != nil {
			t.Fatalf("Call(tool artifact read) error = %v", err)
		}
		var page toolspkg.ToolArtifactPage
		if err := json.Unmarshal(result.Structured, &page); err != nil {
			t.Fatalf("json.Unmarshal(page) error = %v", err)
		}
		if page.Offset != 2 || page.Bytes != 3 || page.NextOffset != 5 || page.EOF {
			t.Fatalf("page = %#v, want bounded middle page", page)
		}
		decoded, err := base64.StdEncoding.DecodeString(page.DataBase64)
		if err != nil {
			t.Fatalf("DecodeString() error = %v", err)
		}
		if got, want := string(decoded), "cde"; got != want {
			t.Fatalf("page bytes = %q, want %q", got, want)
		}

		_, err = registry.Call(t.Context(), toolspkg.Scope{WorkspaceID: "workspace-b"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolArtifactRead,
			Input:  json.RawMessage(fmt.Sprintf(`{"artifact_uri":%q}`, ref.URI)),
		})
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("Call(foreign workspace) error = %v, want indistinguishable not found", err)
		}

		_, err = registry.Call(t.Context(), toolspkg.Scope{WorkspaceID: "workspace-a"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolArtifactRead,
			Input:  json.RawMessage(fmt.Sprintf(`{"artifact_uri":%q,"offset":9}`, ref.URI)),
		})
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeInvalidInput {
			t.Fatalf("Call(offset beyond end) error = %v, want invalid input", err)
		}
	})

	t.Run("Should project clarification after its boot-scoped broker becomes available", func(t *testing.T) {
		t.Parallel()

		var broker toolspkg.ClarifyBroker
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Clarify: func() toolspkg.ClarifyBroker { return broker },
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "session-clarify", WorkspaceID: "ws-clarify", AgentName: "general"}

		views, err := registry.List(t.Context(), scope)
		if err != nil {
			t.Fatalf("List(before clarify broker) error = %v", err)
		}
		if slices.ContainsFunc(views, func(view toolspkg.ToolView) bool {
			return view.Descriptor.ID == toolspkg.ToolIDClarify
		}) {
			t.Fatal("List(before clarify broker) exposed agh__clarify")
		}

		broker = nativeClarifyBrokerStub{}
		views, err = registry.List(t.Context(), scope)
		if err != nil {
			t.Fatalf("List(after clarify broker) error = %v", err)
		}
		if !slices.ContainsFunc(views, func(view toolspkg.ToolView) bool {
			return view.Descriptor.ID == toolspkg.ToolIDClarify
		}) {
			t.Fatal("List(after clarify broker) missing agh__clarify")
		}

		result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDClarify,
			Input:  json.RawMessage(`{"question":"Deploy?","choices":["Staging","Production"]}`),
		})
		if err != nil {
			t.Fatalf("Call(agh__clarify) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"choice":0`))
	})

	t.Run("Should redact every canonical secret shape from network results", func(t *testing.T) {
		t.Parallel()

		value := map[string]string{
			"claim":  "agh_claim_NETWORKSECRET",
			"openai": "sk-network-secret",
			"slack":  "xoxb-network-secret",
		}
		result, err := structuredNetworkResult(
			value,
			"Bearer network-secret sk-network-secret xoxb-network-secret agh_claim_NETWORKSECRET",
		)
		if err != nil {
			t.Fatalf("structuredNetworkResult() error = %v", err)
		}
		for _, secret := range []string{
			"network-secret",
			"sk-network-secret",
			"xoxb-network-secret",
			"agh_claim_NETWORKSECRET",
		} {
			if strings.Contains(string(result.Structured), secret) || strings.Contains(result.Preview, secret) ||
				len(result.Content) != 1 || strings.Contains(result.Content[0].Text, secret) {
				t.Fatalf("structuredNetworkResult() leaked %q: %#v", secret, result)
			}
		}
	})

	t.Run("Should reject coordination tools for a Local caller with not_participating", func(t *testing.T) {
		t.Parallel()

		sessions := apitest.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return &session.Info{
					ID:                   "sess-local",
					WorkspaceID:          nativeNetworkTestWorkspaceID,
					NetworkParticipation: participation.LocalSpec(),
				}, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:  apitest.StubNetworkService{},
			Sessions: sessions,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-local", WorkspaceID: nativeNetworkTestWorkspaceID},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDNetworkStatus},
		)
		requireToolReason(
			t,
			err,
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonNetworkNotParticipating,
		)
	})

	t.Run("Should dispatch skill catalog tools through the real skill registry", func(t *testing.T) {
		t.Parallel()

		skillRegistry := newLoadedNativeSkillRegistry(t)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Skills: skillRegistry,
		}, nativeApproveAllPolicyInputs())

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDSkillList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(skill_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"agh"`))

		searchResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSkillSearch,
				Input:  json.RawMessage(`{"query":"network"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(skill_search) error = %v", err)
		}
		requireNativeStructuredContains(t, searchResult, []byte(`"agh"`))

		viewResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSkillView,
				Input:  json.RawMessage(`{"name":"agh","file":"references/memory.md"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(skill_view) error = %v", err)
		}
		skill, ok := skillRegistry.Get("agh")
		if !ok {
			t.Fatal("Registry.Get(agh) found = false, want true")
		}
		expectedContent, err := skillRegistry.LoadResource(t.Context(), skill, "references/memory.md")
		if err != nil {
			t.Fatalf("Registry.LoadResource(memory) error = %v", err)
		}
		var viewPayload struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(viewResult.Structured, &viewPayload); err != nil {
			t.Fatalf("json.Unmarshal(skill_view) error = %v", err)
		}
		if viewPayload.Content != expectedContent || len(viewResult.Content) != 1 ||
			viewResult.Content[0].Text != expectedContent {
			t.Fatalf("skill_view did not return the resolved bundled resource")
		}
	})

	t.Run("Should expose bootstrap diagnostics and exclude non-MVP lifecycle tools", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Skills:  newLoadedNativeSkillRegistry(t),
			Network: &nativeNetworkStub{},
			Tasks:   &nativeTaskManager{},
		}, nativeApproveAllPolicyInputs())

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDToolList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(tool_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"agh__task_child_create"`))
		requireNativeStructuredContains(t, listResult, []byte(`"agh__bundles_list"`))
		requireNativeStructuredContains(t, listResult, []byte(`"agh__resources_list"`))
		requireNativeStructuredContains(t, listResult, []byte(`"agh__mcp_status"`))
		requireNativeStructuredContains(t, listResult, []byte(`"agh__mcp_auth_status"`))
		requireNativeStructuredExcludes(t, listResult, []byte(`"agh__task_claim"`))
		requireNativeStructuredExcludes(t, listResult, []byte(`"agh__skill_install"`))
		requireNativeStructuredExcludes(t, listResult, []byte(`"agh__mcp_auth_login"`))
		requireNativeStructuredExcludes(t, listResult, []byte(`"agh__mcp_auth_logout"`))

		searchResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDToolSearch,
				Input:  json.RawMessage(`{"query":"child"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(tool_search) error = %v", err)
		}
		requireNativeStructuredContains(t, searchResult, []byte(`"agh__task_child_create"`))

		infoResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDToolInfo,
				Input:  json.RawMessage(`{"tool_id":"agh__network_send"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(tool_info) error = %v", err)
		}
		requireNativeStructuredContains(t, infoResult, []byte(`"open_world"`))

		_, err = registry.Get(t.Context(), toolspkg.Scope{Operator: true}, "agh__task_claim")
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("Registry.Get(excluded task claim) error = %v, want ErrToolNotFound", err)
		}
	})

	t.Run("Should dispatch bundle and resource native tools through daemon services", func(t *testing.T) {
		t.Parallel()

		bundleService := &nativeBundleServiceStub{
			catalog: []bundlepkg.CatalogEntry{{ExtensionName: "ext-bundle"}},
			activations: []bundlepkg.ActivationPreview{{
				Activation: bundlepkg.Activation{
					ID:            "act-1",
					ExtensionName: "ext-bundle",
					BundleName:    "starter",
					ProfileName:   "default",
					Scope:         bundlepkg.ScopeGlobal,
				},
			}},
		}
		resourceService := &nativeResourceServiceStub{
			records: []resources.RawRecord{{
				Kind:     resources.ResourceKind("tool.mcp_server"),
				ID:       "mcp.github",
				SpecJSON: json.RawMessage(`{"name":"github"}`),
			}},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundleService },
			Resources:     resourceService,
		}, nativeApproveAllPolicyInputs())

		bundleResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDBundlesList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(bundles_list) error = %v", err)
		}
		requireNativeStructuredContains(t, bundleResult, []byte(`"act-1"`))

		resourceResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDResourcesSnapshot},
		)
		if err != nil {
			t.Fatalf("Registry.Call(resources_snapshot) error = %v", err)
		}
		requireNativeStructuredContains(t, resourceResult, []byte(`"mcp.github"`))
	})

	t.Run("Should resolve bundle service after native registry boot", func(t *testing.T) {
		t.Parallel()

		var bundleService core.BundleService
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundleService },
		}, nativeApproveAllPolicyInputs())
		operatorScope := toolspkg.Scope{Operator: true}

		views, err := registry.OperatorProjection(t.Context(), operatorScope)
		if err != nil {
			t.Fatalf("OperatorProjection() error = %v", err)
		}
		requireNativeToolUnavailableReason(t, views, toolspkg.ToolIDBundlesList)

		bundleService = &nativeBundleServiceStub{
			catalog: []bundlepkg.CatalogEntry{{ExtensionName: "ext-bundle"}},
		}
		result, err := registry.Call(
			t.Context(),
			operatorScope,
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDBundlesList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(bundles_list) after late service error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"ext-bundle"`))
	})

	t.Run("Should preserve the reserved agent code from bundle activation", func(t *testing.T) {
		t.Parallel()

		bundleService := &nativeBundleServiceStub{
			activateErr: fmt.Errorf("bundle agent: %w", aghconfig.ErrAgentNameReserved),
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundleService },
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesActivate,
				Input: json.RawMessage(
					`{"extension_name":"ext-bundle","bundle_name":"starter","profile_name":"default","scope":"global"}`,
				),
			},
		)
		if !errors.Is(err, aghconfig.ErrAgentNameReserved) {
			t.Fatalf("Registry.Call(bundles_activate reserved) error = %v, want ErrAgentNameReserved", err)
		}
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeAgentNameReserved {
			t.Fatalf("Registry.Call(bundles_activate reserved) error = %#v, want agent_name_reserved", err)
		}
	})

	t.Run("Should bind bundle and resource native tools to the caller workspace", func(t *testing.T) {
		t.Parallel()

		bundleService := &nativeBundleServiceStub{
			catalog: []bundlepkg.CatalogEntry{{ExtensionName: "ext-bundle"}},
			activations: []bundlepkg.ActivationPreview{
				{
					Activation: bundlepkg.Activation{
						ID:            "act-ws-1",
						ExtensionName: "ext-bundle",
						BundleName:    "starter",
						ProfileName:   "default",
						Scope:         bundlepkg.ScopeWorkspace,
						WorkspaceID:   "ws-1",
					},
				},
				{
					Activation: bundlepkg.Activation{
						ID:            "act-ws-2",
						ExtensionName: "ext-bundle",
						BundleName:    "starter",
						ProfileName:   "default",
						Scope:         bundlepkg.ScopeWorkspace,
						WorkspaceID:   "ws-2",
					},
				},
			},
		}
		resourceService := &nativeResourceServiceStub{
			records: []resources.RawRecord{
				{
					Kind:     resources.ResourceKind("tool.mcp_server"),
					ID:       "mcp.ws-1",
					Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
					SpecJSON: json.RawMessage(`{"name":"github"}`),
				},
				{
					Kind:     resources.ResourceKind("tool.mcp_server"),
					ID:       "mcp.ws-2",
					Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-2"},
					SpecJSON: json.RawMessage(`{"name":"linear"}`),
				},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundleService },
			Resources:     resourceService,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "coder"}

		listResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDBundlesList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(bundles_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"act-ws-1"`))
		requireNativeStructuredExcludes(t, listResult, []byte(`"act-ws-2"`))

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesActivate,
				Input: json.RawMessage(
					`{"extension_name":"ext-bundle","bundle_name":"starter","profile_name":"default"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(bundles_activate scoped default) error = %v", err)
		}
		if bundleService.activateCalls != 1 ||
			bundleService.lastActivate.Scope != bundlepkg.ScopeWorkspace ||
			bundleService.lastActivate.Workspace != "ws-1" {
			t.Fatalf(
				"Activate request = %#v after %d calls, want workspace ws-1",
				bundleService.lastActivate,
				bundleService.activateCalls,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesActivate,
				Input: json.RawMessage(
					`{"extension_name":"ext-bundle","bundle_name":"starter","profile_name":"default","bind_primary_channel_as_default":true}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
		if bundleService.activateCalls != 1 {
			t.Fatalf(
				"Activate calls = %d, want removed default binding rejected before service",
				bundleService.activateCalls,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesActivate,
				Input: json.RawMessage(
					`{"extension_name":"ext-bundle","bundle_name":"starter","profile_name":"default","confirm_network_requirement":true}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonPolicyDenied)
		if bundleService.activateCalls != 1 {
			t.Fatalf(
				"Activate calls = %d, want non-operator confirmation rejected before service",
				bundleService.activateCalls,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesActivate,
				Input: json.RawMessage(
					`{"extension_name":"ext-bundle","bundle_name":"starter","profile_name":"default","scope":"workspace","workspace":"ws-2"}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
		if bundleService.activateCalls != 1 {
			t.Fatalf(
				"Activate calls = %d, want cross-workspace request rejected before service",
				bundleService.activateCalls,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesInfo,
				Input:  json.RawMessage(`{"id":"act-ws-2"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesDeactivate,
				Input:  json.RawMessage(`{"id":"act-ws-2"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
		if bundleService.deactivateCalls != 0 {
			t.Fatalf(
				"Deactivate calls = %d, want cross-workspace request rejected before mutation",
				bundleService.deactivateCalls,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDResourcesList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(resources_list scoped default) error = %v", err)
		}
		if resourceService.listCalls != 1 ||
			resourceService.lastFilter.Scope == nil ||
			resourceService.lastFilter.Scope.Kind != resources.ResourceScopeKindWorkspace ||
			resourceService.lastFilter.Scope.ID != "ws-1" {
			t.Fatalf(
				"Resource filter = %#v after %d calls, want workspace ws-1",
				resourceService.lastFilter,
				resourceService.listCalls,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDResourcesSnapshot,
				Input:  json.RawMessage(`{"scope_kind":"workspace","scope_id":"ws-2"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
		if resourceService.listCalls != 1 {
			t.Fatalf(
				"Resource list calls = %d, want cross-workspace filter rejected before service",
				resourceService.listCalls,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDResourcesInfo,
				Input:  json.RawMessage(`{"kind":"tool.mcp_server","id":"mcp.ws-2"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
	})

	t.Run("Should default operator bundle activation to the scoped workspace", func(t *testing.T) {
		t.Parallel()

		bundleService := &nativeBundleServiceStub{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			BundleService: func() core.BundleService { return bundleService },
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true, WorkspaceID: "ws-operator"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBundlesActivate,
				Input: json.RawMessage(
					"{\"extension_name\":\"ext-bundle\",\"bundle_name\":\"starter\",\"profile_name\":\"default\",\"confirm_network_requirement\":true}",
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(bundles_activate operator scoped default) error = %v", err)
		}
		if bundleService.activateCalls != 1 ||
			bundleService.lastActivate.Scope != bundlepkg.ScopeWorkspace ||
			bundleService.lastActivate.Workspace != "ws-operator" ||
			!bundleService.lastActivate.ConfirmNetworkRequirement {
			t.Fatalf(
				"Activate request = %#v after %d calls, want workspace ws-operator",
				bundleService.lastActivate,
				bundleService.activateCalls,
			)
		}
	})

	t.Run("Should bind shared native workspace resolution to the caller workspace", func(t *testing.T) {
		t.Parallel()

		adapter := &daemonNativeTools{
			deps: &daemonNativeToolsDeps{
				Workspaces: nativeNetworkTestWorkspaceService(t),
			},
		}
		scope := toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "coder"}

		resolved, err := adapter.nativeResolvedWorkspace(
			t.Context(),
			toolspkg.ToolIDNetworkPeers,
			"",
			scope,
		)
		if err != nil {
			t.Fatalf("nativeResolvedWorkspace(scoped default) error = %v", err)
		}
		if resolved.WorkspaceID != "ws-1" {
			t.Fatalf("Resolved workspace id = %q, want ws-1", resolved.WorkspaceID)
		}

		_, err = adapter.nativeResolvedWorkspace(
			t.Context(),
			toolspkg.ToolIDNetworkPeers,
			"ws-2",
			scope,
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)

		operatorResolved, err := adapter.nativeResolvedWorkspace(
			t.Context(),
			toolspkg.ToolIDNetworkPeers,
			"ws-2",
			toolspkg.Scope{Operator: true},
		)
		if err != nil {
			t.Fatalf("nativeResolvedWorkspace(operator) error = %v", err)
		}
		if operatorResolved.WorkspaceID != "ws-2" {
			t.Fatalf("Operator resolved workspace id = %q, want ws-2", operatorResolved.WorkspaceID)
		}

		operatorDefaultResolved, err := adapter.nativeResolvedWorkspace(
			t.Context(),
			toolspkg.ToolIDNetworkPeers,
			"",
			toolspkg.Scope{Operator: true, WorkspaceID: "ws-1"},
		)
		if err != nil {
			t.Fatalf("nativeResolvedWorkspace(operator default) error = %v", err)
		}
		if operatorDefaultResolved.WorkspaceID != "ws-1" {
			t.Fatalf(
				"Operator default resolved workspace id = %q, want ws-1",
				operatorDefaultResolved.WorkspaceID,
			)
		}

		filter, err := hookCatalogFilter(toolspkg.ToolIDHooksList, hooksListInput{}, scope)
		if err != nil {
			t.Fatalf("hookCatalogFilter(scoped default) error = %v", err)
		}
		if filter.WorkspaceID != "ws-1" {
			t.Fatalf("Hook filter workspace id = %q, want ws-1", filter.WorkspaceID)
		}

		_, err = hookCatalogFilter(
			toolspkg.ToolIDHooksList,
			hooksListInput{WorkspaceID: "ws-2"},
			scope,
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)

		operatorFilter, err := hookCatalogFilter(
			toolspkg.ToolIDHooksList,
			hooksListInput{WorkspaceID: "ws-2"},
			toolspkg.Scope{Operator: true},
		)
		if err != nil {
			t.Fatalf("hookCatalogFilter(operator) error = %v", err)
		}
		if operatorFilter.WorkspaceID != "ws-2" {
			t.Fatalf("Operator hook filter workspace id = %q, want ws-2", operatorFilter.WorkspaceID)
		}

		observer := &nativeObserverStub{}
		registry := newDaemonNativeRegistry(
			t,
			&daemonNativeToolsDeps{Observer: observer},
			nativeApproveAllPolicyInputs(),
		)
		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksList,
				Input:  json.RawMessage(`{"workspace_id":"ws-2"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksInfo,
				Input:  json.RawMessage(`{"workspace_id":"ws-2","name":"hook-a"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
		if observer.catalogCall != 0 {
			t.Fatalf("QueryHookCatalog calls = %d, want 0", observer.catalogCall)
		}
	})

	t.Run("Should reject foreign workspace inputs for scoped session and skill native tools", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: apitest.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{{ID: "sess-1", WorkspaceID: "ws-1"}}, nil
				},
			},
			Skills: newLoadedNativeSkillRegistry(t),
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "coder"}

		cases := []struct {
			id    toolspkg.ToolID
			input json.RawMessage
		}{
			{toolspkg.ToolIDSessionList, json.RawMessage("{\"workspace\":\"ws-2\"}")},
			{toolspkg.ToolIDSkillList, json.RawMessage("{\"workspace_id\":\"ws-2\"}")},
			{toolspkg.ToolIDSkillSearch, json.RawMessage("{\"query\":\"agh\",\"workspace_id\":\"ws-2\"}")},
			{toolspkg.ToolIDSkillView, json.RawMessage("{\"name\":\"agh\",\"workspace_id\":\"ws-2\"}")},
		}
		for _, tc := range cases {
			t.Run(tc.id.String(), func(t *testing.T) {
				t.Parallel()

				_, err := registry.Call(
					t.Context(),
					scope,
					toolspkg.CallRequest{ToolID: tc.id, Input: tc.input},
				)
				requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
			})
		}
	})

	t.Run("Should keep unavailable read surfaces operator-only with deterministic reasons", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{}, nativeApproveAllPolicyInputs())

		operatorViews, err := registry.List(t.Context(), toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(operator) error = %v", err)
		}
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDNetworkStatus)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDNetworkThreads)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDNetworkDirectResolve)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDNetworkWork)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDSessionList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDSessionHealth)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDAgentHeartbeatStatus)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDAgentHeartbeatWake)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDWorkspaceDescribe)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDProviderModelsList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDProviderModelsCurate)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDMemoryList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDListLogs)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDBridgesList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDAutomationJobsList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDExtensionsList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDBundlesList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDResourcesList)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDMCPStatus)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDMCPAuthStatus)

		sessionViews, err := registry.List(t.Context(), toolspkg.Scope{SessionID: "sess-1"})
		if err != nil {
			t.Fatalf("Registry.List(session) error = %v", err)
		}
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDNetworkStatus,
			toolspkg.ToolIDNetworkThreads,
			toolspkg.ToolIDNetworkDirectResolve,
			toolspkg.ToolIDNetworkWork,
			toolspkg.ToolIDSessionList,
			toolspkg.ToolIDSessionHealth,
			toolspkg.ToolIDAgentHeartbeatStatus,
			toolspkg.ToolIDAgentHeartbeatWake,
			toolspkg.ToolIDWorkspaceDescribe,
			toolspkg.ToolIDProviderModelsList,
			toolspkg.ToolIDProviderModelsCurate,
			toolspkg.ToolIDMemoryList,
			toolspkg.ToolIDListLogs,
			toolspkg.ToolIDBridgesList,
			toolspkg.ToolIDAutomationJobsList,
			toolspkg.ToolIDExtensionsList,
			toolspkg.ToolIDMCPAuthStatus,
		} {
			if nativeToolViewByID(sessionViews, id) != nil {
				t.Fatalf("session projection leaked unavailable tool %s", id)
			}
		}
	})

	t.Run("Should mark bridge catalog tools unavailable without the bounded observer capability", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name     string
			observer core.Observer
		}{
			{name: "nil observer"},
			{
				name: "observer missing bounded bridge catalog methods",
				observer: struct{ core.Observer }{
					Observer: &nativeObserverStub{},
				},
			},
			{
				name: "bounded observer missing bridge source",
				observer: &nativeObserverStub{
					bridgeCatalogReadyErr: errors.New("bridge source is missing"),
				},
			},
		} {
			t.Run("Should report dependency missing for "+test.name, func(t *testing.T) {
				t.Parallel()

				deps := &daemonNativeToolsDeps{
					Bridges:  apitest.StubBridgeService{},
					Observer: test.observer,
				}
				registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
				views, err := registry.List(t.Context(), toolspkg.Scope{Operator: true})
				if err != nil {
					t.Fatalf("Registry.List(operator) error = %v", err)
				}
				for _, id := range []toolspkg.ToolID{
					toolspkg.ToolIDBridgesList,
					toolspkg.ToolIDBridgesStatus,
				} {
					requireNativeToolUnavailableReason(t, views, id)
					_, callErr := registry.Call(
						t.Context(),
						toolspkg.Scope{Operator: true},
						toolspkg.CallRequest{ToolID: id},
					)
					requireToolReason(
						t,
						callErr,
						toolspkg.ErrToolUnavailable,
						toolspkg.ReasonDependencyMissing,
					)
				}

				_, directErr := (&daemonNativeTools{deps: deps}).bridgesList(
					t.Context(),
					toolspkg.Scope{Operator: true},
					toolspkg.CallRequest{ToolID: toolspkg.ToolIDBridgesList},
				)
				requireToolReason(
					t,
					directErr,
					toolspkg.ErrToolUnavailable,
					toolspkg.ReasonDependencyMissing,
				)
			})
		}
	})

	t.Run("Should mark workspace describe unavailable without hiding lighter workspace reads", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: apitest.StubWorkspaceService{},
		}, nativeApproveAllPolicyInputs())

		operatorViews, err := registry.List(t.Context(), toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(operator) error = %v", err)
		}
		requireNativeToolAvailable(t, operatorViews, toolspkg.ToolIDWorkspaceList)
		requireNativeToolAvailable(t, operatorViews, toolspkg.ToolIDWorkspaceInfo)
		requireNativeToolUnavailableReason(t, operatorViews, toolspkg.ToolIDWorkspaceDescribe)

		sessionViews, err := registry.List(t.Context(), toolspkg.Scope{SessionID: "sess-1"})
		if err != nil {
			t.Fatalf("Registry.List(session) error = %v", err)
		}
		requireNativeViewContains(t, sessionViews, toolspkg.ToolIDWorkspaceList)
		requireNativeViewContains(t, sessionViews, toolspkg.ToolIDWorkspaceInfo)
		requireNativeViewExcludes(t, sessionViews, toolspkg.ToolIDWorkspaceDescribe)
	})

	t.Run("Should reject schema-invalid task input before service calls", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskCreate,
				Input:  json.RawMessage(`{"scope":"global","title":"root","parent_task_id":"not-allowed"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(task_create invalid input) error = %v, want ErrToolInvalidInput", err)
		}
		if tasks.createCalls != 0 {
			t.Fatalf("CreateTask calls = %d, want 0", tasks.createCalls)
		}
	})

	t.Run("Should reject schema-invalid input for every native built-in before service calls", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		networkService := &nativeNetworkStub{}
		memoryStore := memorypkg.NewStore(
			filepath.Join(t.TempDir(), "schema-memory"),
			memorypkg.WithCatalogDatabasePath(filepath.Join(t.TempDir(), store.GlobalDatabaseName)),
		)
		openDaemonMemoryCatalog(t, memoryStore)
		catalog := &nativeModelCatalogService{}
		settingsService := &nativeProviderModelSettingsService{}
		extractor := &nativeMemoryExtractorService{}
		providers := &nativeMemoryProviderService{}
		ledger := &nativeMemorySessionLedgerService{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Skills:       newLoadedNativeSkillRegistry(t),
			Network:      networkService,
			NetworkStore: apitest.StubNetworkStore{},
			Tasks:        tasks,
			Bridges:      apitest.StubBridgeService{},
			Automation:   apitest.StubAutomationManager{},
			ModelCatalog: catalog,
			Settings: func() core.SettingsService {
				return settingsService
			},
			MemoryStore:         memoryStore,
			MemoryExtractor:     extractor,
			MemoryProviders:     providers,
			MemorySessionLedger: ledger,
			MCPAuth: func() toolspkg.MCPAuthStatusProvider {
				return &nativeMCPAuthStatusProvider{}
			},
		}, nativeApproveAllPolicyInputs())

		cases := []struct {
			id    toolspkg.ToolID
			input json.RawMessage
		}{
			{toolspkg.ToolIDToolList, json.RawMessage(`{"limit":"bad"}`)},
			{toolspkg.ToolIDToolSearch, json.RawMessage(`{"query":7}`)},
			{toolspkg.ToolIDToolInfo, json.RawMessage(`{"tool_id":7}`)},
			{toolspkg.ToolIDSkillList, json.RawMessage(`{"limit":"bad"}`)},
			{toolspkg.ToolIDSkillSearch, json.RawMessage(`{"query":7}`)},
			{toolspkg.ToolIDSkillView, json.RawMessage(`{"name":7}`)},
			{toolspkg.ToolIDNetworkPeers, json.RawMessage(`{"channel":7}`)},
			{toolspkg.ToolIDNetworkSend, json.RawMessage(`{"channel":"default","kind":"say","body":"bad"}`)},
			{
				toolspkg.ToolIDNetworkSend,
				json.RawMessage(`{"channel":"default","kind":"say","surface":"direct","body":{}}`),
			},
			{
				toolspkg.ToolIDNetworkSend,
				json.RawMessage(`{"channel":"default","kind":"say","body":{},"interaction_id":"old"}`),
			},
			{toolspkg.ToolIDNetworkThreads, json.RawMessage(`{"channel":"builders","interaction_id":"old"}`)},
			{toolspkg.ToolIDNetworkThreadMessages, json.RawMessage(`{"channel":"builders","limit":"bad"}`)},
			{toolspkg.ToolIDNetworkDirects, json.RawMessage(`{"channel":"builders","limit":"bad"}`)},
			{toolspkg.ToolIDNetworkDirectResolve, json.RawMessage(`{"channel":"builders","peer_id":7}`)},
			{toolspkg.ToolIDNetworkDirectMessages, json.RawMessage(`{"channel":"builders","limit":"bad"}`)},
			{toolspkg.ToolIDNetworkWork, json.RawMessage(`{"work_id":7}`)},
			{toolspkg.ToolIDTaskList, json.RawMessage(`{"limit":"bad"}`)},
			{toolspkg.ToolIDTaskRead, json.RawMessage(`{"task_id":7}`)},
			{toolspkg.ToolIDTaskCreate, json.RawMessage(`{"scope":"global","title":"root","parent_task_id":"nope"}`)},
			{toolspkg.ToolIDTaskChildCreate, json.RawMessage(`{"parent_task_id":"parent","scope":"global","title":7}`)},
			{toolspkg.ToolIDTaskUpdate, json.RawMessage(`{"task_id":"task","clear_owner":"no"}`)},
			{toolspkg.ToolIDTaskCancel, json.RawMessage(`{"task_id":7}`)},
			{toolspkg.ToolIDTaskRunList, json.RawMessage(`{"task_id":"task","limit":"bad"}`)},
			{toolspkg.ToolIDTaskRunReviewRequest, json.RawMessage(`{"task_id":"task","run_id":7}`)},
			{toolspkg.ToolIDTaskRunReviewList, json.RawMessage(`{"limit":"bad"}`)},
			{toolspkg.ToolIDTaskRunReviewShow, json.RawMessage(`{"review_id":7}`)},
			{toolspkg.ToolIDTaskExecutionProfileGet, json.RawMessage(`{"task_id":7}`)},
			{
				toolspkg.ToolIDTaskExecutionProfileSet,
				json.RawMessage(`{"task_id":"task","profile":{"created_at":"bad"}}`),
			},
			{toolspkg.ToolIDTaskExecutionProfileDelete, json.RawMessage(`{"task_id":7}`)},
			{toolspkg.ToolIDTaskNotificationSubscribe, json.RawMessage("{\"task_id\":7}")},
			{toolspkg.ToolIDTaskNotificationList, json.RawMessage("{\"task_id\":\"task\",\"limit\":\"bad\"}")},
			{toolspkg.ToolIDTaskNotificationShow, json.RawMessage("{\"task_id\":\"task\",\"subscription_id\":7}")},
			{toolspkg.ToolIDTaskNotificationDelete, json.RawMessage("{\"task_id\":\"task\",\"subscription_id\":7}")},
			{toolspkg.ToolIDAutomationJobsList, json.RawMessage(`{"limit":"bad"}`)},
			{toolspkg.ToolIDAutomationJobsGet, json.RawMessage(`{"job_id":7}`)},
			{toolspkg.ToolIDMCPAuthStatus, json.RawMessage(`{"server_name":7}`)},
			{
				toolspkg.ToolIDAutomationJobsCreate,
				json.RawMessage(
					`{"scope":"global","name":"daily","agent_name":"codex","prompt":"run","schedule":"bad"}`,
				),
			},
			{
				toolspkg.ToolIDAutomationTriggersCreate,
				json.RawMessage(
					`{"scope":"global","name":"event","agent_name":"codex","prompt":"run","event":"session.created","filter":"bad"}`,
				),
			},
			{toolspkg.ToolIDAutomationRunsList, json.RawMessage(`{"limit":"bad"}`)},
			{toolspkg.ToolIDAutomationRunsGet, json.RawMessage(`{"run_id":7}`)},
			{toolspkg.ToolIDProviderModelsList, json.RawMessage("{\"provider_id\":7}")},
			{toolspkg.ToolIDProviderModelsRefresh, json.RawMessage("{\"source_id\":7}")},
			{toolspkg.ToolIDProviderModelsStatus, json.RawMessage("{\"provider_id\":7}")},
			{toolspkg.ToolIDProviderModelsCurate, json.RawMessage("{\"provider_id\":7,\"model_id\":true}")},
			{toolspkg.ToolIDMemoryHealth, json.RawMessage("{\"workspace_id\":7}")},
			{toolspkg.ToolIDMemoryScopeShow, json.RawMessage("{\"scope\":7}")},
			{toolspkg.ToolIDMemoryAdminHistory, json.RawMessage("{\"limit\":\"bad\"}")},
			{toolspkg.ToolIDMemoryReindex, json.RawMessage("{\"include_system\":\"bad\"}")},
			{toolspkg.ToolIDMemoryPromote, json.RawMessage("{\"filename\":7}")},
			{toolspkg.ToolIDMemoryReset, json.RawMessage("{\"confirm\":\"bad\"}")},
			{toolspkg.ToolIDMemoryReload, json.RawMessage("{\"scope\":7}")},
			{toolspkg.ToolIDMemoryDecisionsList, json.RawMessage("{\"limit\":\"bad\"}")},
			{toolspkg.ToolIDMemoryDecisionsShow, json.RawMessage("{\"decision_id\":7}")},
			{toolspkg.ToolIDMemoryDecisionsRevert, json.RawMessage("{\"decision_id\":7}")},
			{toolspkg.ToolIDMemoryRecallTrace, json.RawMessage("{\"session_id\":\"sess\",\"turn_seq\":\"bad\"}")},
			{toolspkg.ToolIDMemoryDreamList, json.RawMessage("{\"limit\":\"bad\"}")},
			{toolspkg.ToolIDMemoryDreamShow, json.RawMessage("{\"dream_id\":7}")},
			{toolspkg.ToolIDMemoryDreamTrigger, json.RawMessage("{\"force\":\"bad\"}")},
			{toolspkg.ToolIDMemoryDreamRetry, json.RawMessage("{\"failure_id\":7}")},
			{toolspkg.ToolIDMemoryDailyList, json.RawMessage("{\"limit\":\"bad\"}")},
			{toolspkg.ToolIDMemoryExtractorRetry, json.RawMessage("{\"failure_id\":7}")},
			{toolspkg.ToolIDMemoryProviderList, json.RawMessage("{\"workspace_id\":7}")},
			{toolspkg.ToolIDMemoryProviderGet, json.RawMessage("{\"name\":7}")},
			{toolspkg.ToolIDMemoryProviderSelect, json.RawMessage("{\"name\":7}")},
			{toolspkg.ToolIDMemoryProviderEnable, json.RawMessage("{\"name\":7}")},
			{toolspkg.ToolIDMemoryProviderDisable, json.RawMessage("{\"name\":7}")},
			{toolspkg.ToolIDMemorySessionLedger, json.RawMessage("{\"session_id\":7}")},
			{toolspkg.ToolIDMemorySessionReplay, json.RawMessage("{\"session_id\":7}")},
			{toolspkg.ToolIDMemorySessionsPrune, json.RawMessage("{\"older_than_hours\":\"bad\"}")},
		}

		for _, tc := range cases {
			t.Run(tc.id.String(), func(t *testing.T) {
				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{Operator: true},
					toolspkg.CallRequest{ToolID: tc.id, Input: tc.input},
				)
				if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
					t.Fatalf("Registry.Call(%s) error = %v, want ErrToolInvalidInput", tc.id, err)
				}
			})
		}
		if got := tasks.totalCalls(); got != 0 {
			t.Fatalf("task manager calls = %d, want 0", got)
		}
		if got := networkService.totalCalls(); got != 0 {
			t.Fatalf("network calls = %d, want 0", got)
		}
		if got := catalog.totalCalls(); got != 0 {
			t.Fatalf("model catalog calls = %d, want 0", got)
		}
		if got := settingsService.calls; got != 0 {
			t.Fatalf("provider model curation calls = %d, want 0", got)
		}
		if got := extractor.totalCalls(); got != 0 {
			t.Fatalf("memory extractor calls = %d, want 0", got)
		}
		if got := providers.totalCalls(); got != 0 {
			t.Fatalf("memory provider calls = %d, want 0", got)
		}
		if got := ledger.totalCalls(); got != 0 {
			t.Fatalf("memory session ledger calls = %d, want 0", got)
		}
	})

	t.Run("Should read provider model catalog tools through the model catalog service boundary", func(t *testing.T) {
		t.Parallel()

		available := true
		now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
		catalog := &nativeModelCatalogService{
			models: []modelcatalog.Model{{
				ProviderID:        "codex",
				ModelID:           "gpt-5.4",
				DisplayName:       "GPT-5.4",
				Available:         &available,
				AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
				RefreshedAt:       now,
				Sources: []modelcatalog.SourceRef{{
					SourceID:    modelcatalog.SourceIDConfig,
					SourceKind:  modelcatalog.SourceKindConfig,
					Priority:    modelcatalog.PriorityConfig,
					RefreshedAt: now,
				}},
			}},
			statuses: []modelcatalog.SourceStatus{{
				SourceID:     modelcatalog.SourceIDConfig,
				SourceKind:   modelcatalog.SourceKindConfig,
				ProviderID:   "codex",
				Priority:     modelcatalog.PriorityConfig,
				LastRefresh:  now,
				LastSuccess:  now,
				RefreshState: modelcatalog.RefreshStateSucceeded,
				RowCount:     1,
			}},
		}
		settingsService := &nativeProviderModelSettingsService{
			result: settingspkg.ProviderModelCurationResult{
				Model: modelcatalog.Model{
					ProviderID:             "codex",
					ModelID:                "gpt-5.6-sol",
					Hidden:                 true,
					DefaultReasoningEffort: new(modelcatalog.ReasoningEffortMax),
				},
				Apply: settingspkg.ApplyResult{
					Applied: true,
					Record: settingspkg.ApplyRecord{
						ID:         "cfgapp-native-curate",
						Lifecycle:  lifecycle.Live,
						Generation: 3,
					},
				},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			ModelCatalog: catalog,
			Settings: func() core.SettingsService {
				return settingsService
			},
		}, nativeApproveAllPolicyInputs())

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsList,
				Input: json.RawMessage(
					`{"provider_id":"codex","source_id":"config","view":"all","include_stale":true}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(provider_models_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"provider_id":"codex"`))
		requireNativeStructuredContains(t, listResult, []byte(`"model_id":"gpt-5.4"`))
		if catalog.listCalls != 1 ||
			catalog.lastList.ProviderID != "codex" ||
			catalog.lastList.SourceID != modelcatalog.SourceIDConfig ||
			catalog.lastList.View != modelcatalog.CatalogViewAll ||
			catalog.lastList.Refresh ||
			!catalog.lastList.IncludeStale {
			t.Fatalf("ListModels options = %#v after %d calls", catalog.lastList, catalog.listCalls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsList,
				Input:  json.RawMessage(`{"refresh":true}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(provider_models_list refresh) error = %v, want ErrToolInvalidInput", err)
		}
		if catalog.listCalls != 1 {
			t.Fatalf("ListModels calls = %d, want unchanged after refresh input", catalog.listCalls)
		}

		refreshResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsRefresh,
				Input: json.RawMessage(
					`{"provider_id":"codex","source_id":"config","force":true,"request_id":"req-1"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(provider_models_refresh) error = %v", err)
		}
		requireNativeStructuredContains(t, refreshResult, []byte(`"source_id":"config"`))
		if catalog.refreshCalls != 1 ||
			catalog.lastRefresh.ProviderID != "codex" ||
			catalog.lastRefresh.SourceID != modelcatalog.SourceIDConfig ||
			!catalog.lastRefresh.Force ||
			catalog.lastRefresh.RequestID != "req-1" {
			t.Fatalf("Refresh options = %#v after %d calls", catalog.lastRefresh, catalog.refreshCalls)
		}

		statusResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsStatus,
				Input:  json.RawMessage(`{"provider_id":"codex"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(provider_models_status) error = %v", err)
		}
		requireNativeStructuredContains(t, statusResult, []byte(`"refresh_state":"succeeded"`))
		if catalog.statusCalls != 1 || catalog.lastStatusProviderID != "codex" {
			t.Fatalf("ListSourceStatus provider = %q after %d calls", catalog.lastStatusProviderID, catalog.statusCalls)
		}

		curateResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsCurate,
				Input: json.RawMessage(
					`{"provider_id":"codex","model_id":"gpt-5.6-sol","hidden":true,"default_effort":"max"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(provider_models_curate) error = %v", err)
		}
		requireNativeStructuredContains(t, curateResult, []byte(`"model_id":"gpt-5.6-sol"`))
		requireNativeStructuredContains(t, curateResult, []byte(`"hidden":true`))
		if settingsService.calls != 1 ||
			settingsService.lastRequest.ProviderID != "codex" ||
			settingsService.lastRequest.ModelID != "gpt-5.6-sol" ||
			settingsService.lastRequest.Hidden == nil ||
			!*settingsService.lastRequest.Hidden ||
			settingsService.lastRequest.DefaultReasoningEffort == nil ||
			*settingsService.lastRequest.DefaultReasoningEffort != "max" {
			t.Fatalf("curation request = %#v after %d calls", settingsService.lastRequest, settingsService.calls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsList,
				Input:  json.RawMessage(`{"provider_id":"Bad Provider"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(provider_models_list invalid provider) error = %v, want ErrToolInvalidInput", err)
		}
		if catalog.listCalls != 1 {
			t.Fatalf("ListModels calls = %d, want unchanged after invalid provider", catalog.listCalls)
		}
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsList,
				Input:  json.RawMessage(`{"view":"recent"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(provider_models_list invalid view) error = %v, want ErrToolInvalidInput", err)
		}
		if catalog.listCalls != 1 {
			t.Fatalf("ListModels calls = %d, want unchanged after invalid view", catalog.listCalls)
		}
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsRefresh,
				Input:  json.RawMessage(`{"source_id":"bad source"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(provider_models_refresh invalid source) error = %v, want ErrToolInvalidInput", err)
		}
		if catalog.refreshCalls != 1 {
			t.Fatalf("Refresh calls = %d, want unchanged after invalid source", catalog.refreshCalls)
		}
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsStatus,
				Input:  json.RawMessage(`{"provider_id":"Bad Provider"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(provider_models_status invalid provider) error = %v, want ErrToolInvalidInput", err)
		}
		if catalog.statusCalls != 1 {
			t.Fatalf("Status calls = %d, want unchanged after invalid provider", catalog.statusCalls)
		}
	})

	t.Run("Should map provider model service failures to native tool errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			id   toolspkg.ToolID
			mut  func(*nativeModelCatalogService)
			want toolspkg.ErrorCode
		}{
			{
				name: "list backend failure",
				id:   toolspkg.ToolIDProviderModelsList,
				mut: func(catalog *nativeModelCatalogService) {
					catalog.listErr = errors.New("catalog read failed")
				},
				want: toolspkg.ErrorCodeBackendFailed,
			},
			{
				name: "refresh unavailable",
				id:   toolspkg.ToolIDProviderModelsRefresh,
				mut: func(catalog *nativeModelCatalogService) {
					catalog.refreshErr = modelcatalog.ErrAllSourcesFailed
				},
				want: toolspkg.ErrorCodeUnavailable,
			},
			{
				name: "status backend failure",
				id:   toolspkg.ToolIDProviderModelsStatus,
				mut: func(catalog *nativeModelCatalogService) {
					catalog.statusErr = errors.New("status read failed")
				},
				want: toolspkg.ErrorCodeBackendFailed,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				catalog := &nativeModelCatalogService{}
				tc.mut(catalog)
				registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
					ModelCatalog: catalog,
				}, nativeApproveAllPolicyInputs())
				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{},
					toolspkg.CallRequest{ToolID: tc.id},
				)
				requireToolCode(t, err, tc.want)
			})
		}
	})

	t.Run("Should preserve model_not_found from provider model curation", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("provider model codex/missing was not found")
		item := diagnostics.NewItem(
			"provider.models.model_not_found",
			diagnosticcontract.CodeModelNotFound,
			diagnosticcontract.CategoryProvider,
			"Provider model not found",
			cause.Error(),
			diagnosticcontract.SeverityError,
			diagnosticcontract.FreshnessLive,
		)
		settingsService := &nativeProviderModelSettingsService{
			err: diagnostics.NewStructuredError(item, cause),
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			ModelCatalog: &nativeModelCatalogService{},
			Settings: func() core.SettingsService {
				return settingsService
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsCurate,
				Input:  json.RawMessage(`{"provider_id":"codex","model_id":"missing"}`),
			},
		)
		requireToolCode(t, err, toolspkg.ErrorCodeModelNotFound)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(provider_models_curate missing) error = %v, want ErrToolInvalidInput", err)
		}
	})

	t.Run("Should require approval for mutating tools under approve-reads policy", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		catalog := &nativeModelCatalogService{}
		settingsService := &nativeProviderModelSettingsService{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks:        tasks,
			ModelCatalog: catalog,
			Settings: func() core.SettingsService {
				return settingsService
			},
		}, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    false,
		})

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskCreate,
				Input:  json.RawMessage(`{"scope":"global","title":"root"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf("Registry.Call(task_create approve-reads) error = %v, want ErrToolApprovalRequired", err)
		}
		if tasks.createCalls != 0 {
			t.Fatalf("CreateTask calls = %d, want 0", tasks.createCalls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsRefresh,
				Input:  json.RawMessage(`{"provider_id":"codex"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf(
				"Registry.Call(provider_models_refresh approve-reads) error = %v, want ErrToolApprovalRequired",
				err,
			)
		}
		if catalog.refreshCalls != 0 {
			t.Fatalf("Refresh calls = %d, want 0", catalog.refreshCalls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDProviderModelsCurate,
				Input:  json.RawMessage(`{"provider_id":"codex","model_id":"gpt-5.6-sol"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf(
				"Registry.Call(provider_models_curate approve-reads) error = %v, want ErrToolApprovalRequired",
				err,
			)
		}
		if settingsService.calls != 0 {
			t.Fatalf("ApplyProviderModelCuration calls = %d, want 0", settingsService.calls)
		}
	})

	t.Run("Should mutate allowed config paths and reject guarded config paths", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		globalTarget, err := aghconfig.ResolveConfigWriteTarget(homePaths, "", aghconfig.WriteScopeGlobal)
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
		}
		if _, err := aghconfig.EditConfigOverlay(
			homePaths,
			"",
			globalTarget,
			func(editor *aghconfig.OverlayEditor) error {
				return editor.SetTable([]string{"providers", "codex"}, map[string]any{"command": "codex-acp"})
			},
		); err != nil {
			t.Fatalf("EditConfigOverlay(provider fixture) error = %v", err)
		}
		settingsService := &nativeConfigSettingsService{
			result: settingspkg.ApplyResult{
				Applied:    true,
				NextAction: lifecycle.NextActionNone,
				Record: settingspkg.ApplyRecord{
					ID:         "cfgapp-native-marketplace",
					ActiveHash: "sha256:active-marketplace",
					Generation: 7,
					Lifecycle:  lifecycle.Live,
					Status:     lifecycle.StatusApplied,
				},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths: homePaths,
			Settings: func() core.SettingsService {
				return settingsService
			},
		}, nativeApproveAllPolicyInputs())

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  json.RawMessage(`{"path":"defaults.agent","value":"planner"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_set allowed) error = %v", err)
		}
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if cfg.Defaults.Agent != "planner" {
			t.Fatalf("Defaults.Agent = %q, want planner", cfg.Defaults.Agent)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  json.RawMessage(`{"path":"roles.dream.model","value":"claude-haiku-4-5"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_set dream role model) error = %v", err)
		}
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after dream role model) error = %v", err)
		}
		if cfg.Roles.Dream.Model != "claude-haiku-4-5" {
			t.Fatalf("Roles.Dream.Model = %q, want claude-haiku-4-5", cfg.Roles.Dream.Model)
		}
		dreamRoleResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigGet,
				Input:  json.RawMessage(`{"path":"roles.dream.model"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_get dream role model) error = %v", err)
		}
		requireNativeStructuredContains(t, dreamRoleResult, []byte(`"claude-haiku-4-5"`))

		rolesTableInput := json.RawMessage(
			`{"path":"roles","value":{"auto_title":{"enabled":true,"fallback_chain":[{"provider":"codex","model":"gpt-5-mini","reasoning_effort":"medium"}]}}}`,
		)
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  rolesTableInput,
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_set roles table) error = %v; cause = %v", err, errors.Unwrap(err))
		}
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after roles table) error = %v", err)
		}
		if got := cfg.Roles.AutoTitle.FallbackChain; len(got) != 1 ||
			got[0].Provider != "codex" || got[0].Model != "gpt-5-mini" {
			t.Fatalf("Roles.AutoTitle.FallbackChain = %#v, want structured fallback", got)
		}
		coordinatorEnabledResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigGet,
				Input:  json.RawMessage(`{"path":"roles.coordinator.enabled"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_get coordinator enabled) error = %v", err)
		}
		requireNativeStructuredContains(
			t,
			coordinatorEnabledResult,
			[]byte(`"path":"roles.coordinator.enabled"`),
		)

		goalResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  json.RawMessage(`{"path":"goals.context_nudge_ratio","value":0.0}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_set Goal ratio) error = %v", err)
		}
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after Goal ratio) error = %v", err)
		}
		if cfg.Goals.ContextNudgeRatio != 0 {
			t.Fatalf("Goals.ContextNudgeRatio = %v, want explicit zero", cfg.Goals.ContextNudgeRatio)
		}
		requireNativeStructuredContains(t, goalResult, []byte(`"restart-required"`))

		capacityResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input: json.RawMessage(
					`{"path":"task.orchestration.max_active_runs_per_workspace","value":8}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_set task workspace capacity) error = %v", err)
		}
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after task workspace capacity) error = %v", err)
		}
		if cfg.Task.Orchestration.MaxActiveRunsPerWorkspace != 8 {
			t.Fatalf(
				"Task.Orchestration.MaxActiveRunsPerWorkspace = %d, want 8",
				cfg.Task.Orchestration.MaxActiveRunsPerWorkspace,
			)
		}
		requireNativeStructuredContains(t, capacityResult, []byte(`"restart-required"`))
		requireNativeStructuredContains(t, capacityResult, []byte(`"next_action":"restart-daemon"`))

		marketplaceResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  json.RawMessage(`{"path":"marketplace.catalog.timeout","value":"3s"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_set Marketplace timeout) error = %v", err)
		}
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after Marketplace timeout) error = %v", err)
		}
		if cfg.Marketplace.Catalog.Timeout != "3s" {
			t.Fatalf("Marketplace.Catalog.Timeout = %q, want 3s", cfg.Marketplace.Catalog.Timeout)
		}
		requireNativeStructuredContains(t, marketplaceResult, []byte(`"applied":true`))
		requireNativeStructuredContains(t, marketplaceResult, []byte(`"apply_record_id":"cfgapp-native-marketplace"`))
		requireNativeStructuredContains(t, marketplaceResult, []byte(`"active_generation":7`))
		if settingsService.reloadCalls != 3 {
			t.Fatalf("Settings.Reload calls = %d, want 3", settingsService.reloadCalls)
		}

		workspaceRoot := t.TempDir()
		for _, toolID := range []toolspkg.ToolID{toolspkg.ToolIDConfigSet, toolspkg.ToolIDConfigUnset} {
			input := map[string]any{
				"path":           "marketplace.catalog.timeout",
				"scope":          "workspace",
				"workspace_root": workspaceRoot,
			}
			if toolID == toolspkg.ToolIDConfigSet {
				input["value"] = "4s"
			}
			encoded, marshalErr := json.Marshal(input)
			if marshalErr != nil {
				t.Fatalf("json.Marshal(%s input) error = %v", toolID, marshalErr)
			}
			_, err = registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{ToolID: toolID, Input: encoded},
			)
			requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonConfigScopeNotAllowed)
		}
		workspaceConfig := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName)
		if _, statErr := os.Stat(workspaceConfig); !os.IsNotExist(statErr) {
			t.Fatalf("workspace config stat error = %v, want no file", statErr)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  json.RawMessage(`{"path":"marketplace.catalog.timeout","value":"0s"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(config_set invalid Marketplace timeout) error = %v, want ErrToolInvalidInput", err)
		}
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after invalid Marketplace timeout) error = %v", err)
		}
		if cfg.Marketplace.Catalog.Timeout != "3s" {
			t.Fatalf("Marketplace.Catalog.Timeout after invalid write = %q, want 3s", cfg.Marketplace.Catalog.Timeout)
		}

		unsetResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigUnset,
				Input:  json.RawMessage(`{"path":"marketplace.catalog.timeout"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_unset Marketplace timeout) error = %v", err)
		}
		requireNativeStructuredContains(t, unsetResult, []byte(`"applied":true`))
		requireNativeStructuredContains(t, unsetResult, []byte(`"apply_record_id":"cfgapp-native-marketplace"`))
		if settingsService.reloadCalls != 4 {
			t.Fatalf("Settings.Reload calls = %d, want 4", settingsService.reloadCalls)
		}

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigGet,
				Input:  json.RawMessage(`{"path":"defaults.agent"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_get) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"planner"`))

		cases := []struct {
			path   string
			reason toolspkg.ReasonCode
		}{
			{path: "daemon.socket", reason: toolspkg.ReasonConfigTrustRootForbidden},
			{path: "http.port", reason: toolspkg.ReasonConfigTrustRootForbidden},
			{path: "providers.claude.credential_slots[0].secret_ref", reason: toolspkg.ReasonConfigSecretPathForbidden},
			{path: "mcp_servers[0].env.TOKEN", reason: toolspkg.ReasonConfigSecretPathForbidden},
			{path: "sandboxes.default.runtime_root", reason: toolspkg.ReasonConfigTrustRootForbidden},
			{path: "marketplace.catalog.base_url", reason: toolspkg.ReasonConfigTrustRootForbidden},
			{path: "memory.dream.agent", reason: toolspkg.ReasonConfigPathForbidden},
			{path: "memory.dream.enabled", reason: toolspkg.ReasonConfigPathForbidden},
			{path: "memory.extractor.model", reason: toolspkg.ReasonConfigPathForbidden},
			{path: "memory.extractor.enabled", reason: toolspkg.ReasonConfigPathForbidden},
			{path: "memory.controller.llm.model", reason: toolspkg.ReasonConfigPathForbidden},
			{path: "session.auto_title_enabled", reason: toolspkg.ReasonConfigPathForbidden},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.path, func(t *testing.T) {
				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{},
					toolspkg.CallRequest{
						ToolID: toolspkg.ToolIDConfigSet,
						Input: json.RawMessage(
							fmt.Sprintf(`{"path":%q,"value":"blocked"}`, tc.path),
						),
					},
				)
				requireToolReason(t, err, toolspkg.ErrToolDenied, tc.reason)
			})
		}
	})

	t.Run("Should report live config reconciliation failures from the settings apply record", func(t *testing.T) {
		t.Parallel()

		settingsService := &nativeConfigSettingsService{
			result: settingspkg.ApplyResult{
				Applied:    false,
				NextAction: lifecycle.NextActionRetry,
				Record: settingspkg.ApplyRecord{
					ID:        "cfgapp-native-marketplace-failed",
					Lifecycle: lifecycle.Live,
					Status:    lifecycle.StatusFailed,
				},
				PartialFailures: []settingspkg.ApplyFailure{{
					Subsystem: "marketplace",
					Diagnostic: diagnostics.NewItem(
						"config.apply.marketplace_sync_failed",
						diagnosticcontract.CodeConfigPartialFailure,
						diagnosticcontract.CategoryConfig,
						"Marketplace sync failed",
						"test reconciliation failure",
						diagnosticcontract.SeverityError,
						diagnosticcontract.FreshnessLive,
					),
				}},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths: testHomePaths(t),
			Settings: func() core.SettingsService {
				return settingsService
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  json.RawMessage(`{"path":"marketplace.catalog.ttl","value":"10m"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config_set Marketplace TTL) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"applied":false`))
		requireNativeStructuredContains(t, result, []byte(`"apply_record_id":"cfgapp-native-marketplace-failed"`))
		requireNativeStructuredContains(t, result, []byte(`"next_action":"retry"`))
		requireNativeStructuredContains(t, result, []byte(`"subsystem":"marketplace"`))
		if settingsService.reloadCalls != 1 {
			t.Fatalf("Settings.Reload calls = %d, want 1", settingsService.reloadCalls)
		}
	})

	t.Run("Should require approval before config writes reach persistence", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths: homePaths,
		}, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    false,
		})

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDConfigSet,
				Input:  json.RawMessage(`{"path":"defaults.agent","value":"planner"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf("Registry.Call(config_set approve-reads) error = %v, want ErrToolApprovalRequired", err)
		}
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if cfg.Defaults.Agent == "planner" {
			t.Fatal("Defaults.Agent was mutated before approval")
		}
	})

	t.Run("Should read hook introspection tools through observer without leaking secret fields", func(t *testing.T) {
		t.Parallel()

		stableWorkspaceID := "ws-stable"
		registryWorkspaceID := "ws-registry"
		workspaceRoot := t.TempDir()
		workspaces := apitest.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != stableWorkspaceID && ref != registryWorkspaceID {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      registryWorkspaceID,
						RootDir: workspaceRoot,
						Name:    "hooks",
					},
					WorkspaceID: stableWorkspaceID,
				}, nil
			},
		}
		observer := &nativeObserverStub{
			catalog: []hookspkg.CatalogEntry{{
				Order:        1,
				Name:         "config-tool",
				Event:        hookspkg.HookToolPreCall,
				Source:       hookspkg.HookSourceConfig,
				Mode:         hookspkg.HookModeSync,
				Required:     true,
				Priority:     500,
				Timeout:      time.Second,
				ExecutorKind: hookspkg.HookExecutorSubprocess,
				Matcher:      hookspkg.HookMatcher{ToolID: "agh__task_read"},
				Metadata: map[string]string{
					"access_token": "secret-value",
					"visible":      "ok",
				},
			}},
			runs: []hookspkg.HookRunRecord{{
				HookName:      "config-tool",
				Event:         hookspkg.HookToolPreCall,
				Source:        hookspkg.HookSourceConfig,
				Mode:          hookspkg.HookModeSync,
				Duration:      2 * time.Millisecond,
				Outcome:       hookspkg.HookRunOutcomeApplied,
				DispatchDepth: 1,
				PatchApplied:  json.RawMessage(`{"password":"secret-value","visible":"ok"}`),
				Required:      true,
				RecordedAt:    time.Unix(100, 0).UTC(),
			}},
			events: []hookspkg.EventDescriptor{{
				Event:         hookspkg.HookToolPreCall,
				Family:        hookspkg.HookEventFamilyTool,
				SyncEligible:  true,
				PayloadSchema: "ToolPreCallPayload",
				PatchSchema:   "ToolCallPatch",
			}},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Observer:   observer,
			Sessions:   nativeNetworkTestSessionManager(registryWorkspaceID),
			Workspaces: workspaces,
		}, nativeApproveAllPolicyInputs())

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDHooksList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"config-tool"`))
		requireNativeStructuredContains(t, listResult, []byte(`"visible":"ok"`))
		requireNativeStructuredExcludes(t, listResult, []byte(`secret-value`))

		infoResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksInfo,
				Input:  json.RawMessage(`{"name":"config-tool"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_info) error = %v", err)
		}
		requireNativeStructuredContains(t, infoResult, []byte(`"config-tool"`))
		requireNativeStructuredExcludes(t, infoResult, []byte(`secret-value`))

		eventsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksEvents,
				Input:  json.RawMessage(`{"family":"tool"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_events) error = %v", err)
		}
		requireNativeStructuredContains(t, eventsResult, []byte(`"ToolPreCallPayload"`))

		runsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksRuns,
				Input: json.RawMessage(
					`{"workspace_id":"ws-stable","session_id":"sess-hooks","event":"tool.pre_call","outcome":"applied","last":1}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_runs) error = %v", err)
		}
		requireNativeStructuredContains(t, runsResult, []byte(`"config-tool"`))
		requireNativeStructuredContains(t, runsResult, []byte(`"visible":"ok"`))
		requireNativeStructuredExcludes(t, runsResult, []byte(`secret-value`))
		if observer.catalogCall != 2 {
			t.Fatalf("QueryHookCatalog calls = %d, want 2", observer.catalogCall)
		}
		if observer.hookRunCalls != 1 || observer.lastHookRunQuery.SessionID != "sess-hooks" {
			t.Fatalf("QueryHookRuns query = %#v after %d calls", observer.lastHookRunQuery, observer.hookRunCalls)
		}

		foreignObserver := &nativeObserverStub{runs: observer.runs}
		foreignRegistry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Observer: foreignObserver,
			Sessions: apitest.StubSessionManager{
				StatusFn: func(_ context.Context, id string) (*session.Info, error) {
					return &session.Info{ID: strings.TrimSpace(id), WorkspaceID: "ws-other"}, nil
				},
			},
			Workspaces: workspaces,
		}, nativeApproveAllPolicyInputs())
		_, err = foreignRegistry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksRuns,
				Input:  json.RawMessage(`{"workspace_id":"ws-stable","session_id":"sess-hooks"}`),
			},
		)
		if err == nil {
			t.Fatal("Registry.Call(hooks_runs foreign session) error = nil, want non-nil")
		}
		if foreignObserver.hookRunCalls != 0 {
			t.Fatalf("foreign QueryHookRuns calls = %d, want 0", foreignObserver.hookRunCalls)
		}
	})

	t.Run("Should manage config backed hooks through restart-required lifecycle", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		observer := &nativeObserverStub{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths: homePaths,
			Observer:  observer,
		}, nativeApproveAllPolicyInputs())

		createResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksCreate,
				Input: json.RawMessage(
					`{"name":"tool-audit","event":"tool.pre_call","command":"/bin/echo","args":["audit"]}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_create) error = %v", err)
		}
		requireNativeStructuredContains(t, createResult, []byte(`"applied":false`))
		requireNativeStructuredContains(t, createResult, []byte(`"lifecycle":"restart-required"`))
		requireNativeStructuredContains(t, createResult, []byte(`"next_action":"restart-daemon"`))
		target, err := aghconfig.ResolveConfigWriteTarget(homePaths, "", aghconfig.WriteScopeGlobal)
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
		}
		decls, err := aghconfig.OverlayHookDeclarations(target)
		if err != nil {
			t.Fatalf("OverlayHookDeclarations() error = %v", err)
		}
		if len(decls) != 1 || decls[0].Name != "tool-audit" || decls[0].Command != "/bin/echo" {
			t.Fatalf("OverlayHookDeclarations() = %#v, want created hook", decls)
		}

		disableResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksDisable,
				Input:  json.RawMessage(`{"name":"tool-audit"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_disable) error = %v", err)
		}
		requireNativeStructuredContains(t, disableResult, []byte(`"applied":false`))
		requireNativeStructuredContains(t, disableResult, []byte(`"lifecycle":"restart-required"`))
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		active, err := aghconfig.HookDeclarations(cfg.Hooks, nil)
		if err != nil {
			t.Fatalf("HookDeclarations(disabled) error = %v", err)
		}
		if len(active) != 0 {
			t.Fatalf("HookDeclarations(disabled) len = %d, want 0", len(active))
		}

		enableResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksEnable,
				Input:  json.RawMessage(`{"name":"tool-audit"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_enable) error = %v", err)
		}
		requireNativeStructuredContains(t, enableResult, []byte(`"applied":false`))
		requireNativeStructuredContains(t, enableResult, []byte(`"next_action":"restart-daemon"`))
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(enabled) error = %v", err)
		}
		active, err = aghconfig.HookDeclarations(cfg.Hooks, nil)
		if err != nil {
			t.Fatalf("HookDeclarations(enabled) error = %v", err)
		}
		if len(active) != 1 || active[0].Name != "tool-audit" {
			t.Fatalf("HookDeclarations(enabled) = %#v, want active tool-audit", active)
		}

		updateResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksUpdate,
				Input:  json.RawMessage(`{"name":"tool-audit","command":"/usr/bin/env","args":["true"]}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_update) error = %v", err)
		}
		requireNativeStructuredContains(t, updateResult, []byte(`"applied":false`))
		requireNativeStructuredContains(t, updateResult, []byte(`"lifecycle":"restart-required"`))
		decls, err = aghconfig.OverlayHookDeclarations(target)
		if err != nil {
			t.Fatalf("OverlayHookDeclarations(updated) error = %v", err)
		}
		if len(decls) != 1 || decls[0].Command != "/usr/bin/env" || len(decls[0].Args) != 1 {
			t.Fatalf("OverlayHookDeclarations(updated) = %#v, want updated command", decls)
		}

		deleteResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksDelete,
				Input:  json.RawMessage(`{"name":"tool-audit"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(hooks_delete) error = %v", err)
		}
		requireNativeStructuredContains(t, deleteResult, []byte(`"applied":false`))
		requireNativeStructuredContains(t, deleteResult, []byte(`"next_action":"restart-daemon"`))
		decls, err = aghconfig.OverlayHookDeclarations(target)
		if err != nil {
			t.Fatalf("OverlayHookDeclarations(deleted) error = %v", err)
		}
		if len(decls) != 0 {
			t.Fatalf("OverlayHookDeclarations(deleted) = %#v, want empty", decls)
		}
	})

	t.Run("Should reject immutable hook sources and secret hook executor inputs", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		bindings := &nativeHookBindingsStub{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths: homePaths,
			Observer: &nativeObserverStub{
				catalog: []hookspkg.CatalogEntry{{
					Name:   "native-session",
					Event:  hookspkg.HookSessionPostCreate,
					Source: hookspkg.HookSourceNative,
					Mode:   hookspkg.HookModeAsync,
				}, {
					Name:   "skill-session",
					Event:  hookspkg.HookSessionPostCreate,
					Source: hookspkg.HookSourceSkill,
					Mode:   hookspkg.HookModeAsync,
				}},
			},
			HookBindings: bindings,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksUpdate,
				Input:  json.RawMessage(`{"name":"native-session","command":"/bin/echo"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonHookSourceImmutable)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksDelete,
				Input:  json.RawMessage(`{"name":"skill-session"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonHookSourceImmutable)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksCreate,
				Input: json.RawMessage(
					`{"name":"secret-hook","event":"tool.pre_call","command":"/bin/echo","env":{"API_KEY":"secret"}}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonHookSecretInputForbidden)
		if bindings.syncCalls != 0 {
			t.Fatalf("HookBindings.Sync calls = %d, want 0 after denied hooks", bindings.syncCalls)
		}
	})

	t.Run("Should require approval before hook mutations reach config writer", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		bindings := &nativeHookBindingsStub{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths:    homePaths,
			Observer:     &nativeObserverStub{},
			HookBindings: bindings,
		}, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    false,
		})

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDHooksCreate,
				Input:  json.RawMessage(`{"name":"blocked","event":"tool.pre_call","command":"/bin/echo"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf("Registry.Call(hooks_create approve-reads) error = %v, want ErrToolApprovalRequired", err)
		}
		if bindings.syncCalls != 0 {
			t.Fatalf("HookBindings.Sync calls = %d, want 0 before approval", bindings.syncCalls)
		}
	})

	t.Run("Should route bounded task tools through task service boundaries", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{
			listSummaries: []taskpkg.Summary{{
				ID:     "task-listed",
				Title:  "Listed task",
				Status: taskpkg.TaskStatusPending,
				Scope:  taskpkg.ScopeWorkspace,
			}},
			getView: func() *taskpkg.View {
				rawClaimToken := "agh_claim_READSECRET"
				blockedReasons := []taskpkg.BlockedReason{{
					Source:  taskpkg.BlockedSourceBlock,
					Kind:    taskpkg.BlockKindNeedsInput,
					Reason:  "waiting on " + rawClaimToken,
					BlockID: "block-read",
				}}
				return &taskpkg.View{
					Summary: taskpkg.Summary{
						ID:             "task-read",
						Title:          "Read task",
						Status:         taskpkg.TaskStatusBlocked,
						BlockedReasons: &blockedReasons,
					},
					Task: taskpkg.Task{
						ID:     "task-read",
						Title:  "Read task",
						Status: taskpkg.TaskStatusBlocked,
					},
				}
			}(),
			runs: []taskpkg.Run{{
				ID:     "run-listed",
				TaskID: "task-run",
				Status: taskpkg.TaskRunStatusQueued,
			}},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-actor", WorkspaceID: "ws-1"}

		_, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskList,
				Input:  json.RawMessage(`{"scope":"workspace","status":"pending","limit":3}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_list) error = %v", err)
		}
		if tasks.listCalls != 1 ||
			tasks.lastQuery.WorkspaceID != "ws-1" ||
			tasks.lastQuery.Status != taskpkg.TaskStatusPending {
			t.Fatalf("ListTasks calls/query = %d/%#v, want workspace pending query", tasks.listCalls, tasks.lastQuery)
		}

		readResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRead,
				Input:  json.RawMessage(`{"task_id":"task-read"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_read) error = %v", err)
		}
		if tasks.getCalls != 1 || tasks.lastGetID != "task-read" {
			t.Fatalf("GetTask calls/id = %d/%q, want task-read", tasks.getCalls, tasks.lastGetID)
		}
		requireNativeStructuredContains(t, readResult, []byte(`agh_claim_[REDACTED]`))
		requireNativeStructuredExcludes(t, readResult, []byte(`agh_claim_READSECRET`))

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskCreate,
				Input:  json.RawMessage(`{"scope":"global","title":"root task"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_create) error = %v", err)
		}
		if tasks.createCalls != 1 || tasks.lastCreateSpec.WorkspaceID != "" {
			t.Fatalf(
				"CreateTask calls/spec = %d/%#v, want global task without caller workspace fallback",
				tasks.createCalls,
				tasks.lastCreateSpec,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskUpdate,
				Input: json.RawMessage(
					`{"task_id":"task-update","title":"Updated task","clear_owner":true,` +
						`"network_participation":{"mode":"local"}}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_update) error = %v", err)
		}
		if tasks.updateCalls != 1 ||
			tasks.lastUpdateID != "task-update" ||
			tasks.lastPatch.Title == nil ||
			*tasks.lastPatch.Title != "Updated task" ||
			!tasks.lastPatch.ClearOwner ||
			tasks.lastPatch.NetworkParticipation == nil ||
			tasks.lastPatch.NetworkParticipation.Mode == nil ||
			*tasks.lastPatch.NetworkParticipation.Mode != participation.ModeLocal {
			t.Fatalf(
				"UpdateTask calls/patch = %d/%q/%#v, want title patch",
				tasks.updateCalls,
				tasks.lastUpdateID,
				tasks.lastPatch,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskCancel,
				Input:  json.RawMessage(`{"task_id":"task-cancel","reason":"operator canceled"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_cancel) error = %v", err)
		}
		if tasks.cancelCalls != 1 ||
			tasks.lastCancelID != "task-cancel" ||
			tasks.lastCancel.Reason != "operator canceled" {
			t.Fatalf(
				"CancelTask calls/request = %d/%q/%#v, want cancellation request",
				tasks.cancelCalls,
				tasks.lastCancelID,
				tasks.lastCancel,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunList,
				Input: json.RawMessage(
					`{"task_id":"task-run","status":"queued",` +
						`"participation_channel":"builders","limit":2}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_run_list) error = %v", err)
		}
		if tasks.runListCalls != 1 ||
			tasks.lastRunListTaskID != "task-run" ||
			tasks.lastRunQuery.Status != taskpkg.TaskRunStatusQueued ||
			tasks.lastRunQuery.ParticipationChannel != "builders" ||
			tasks.lastRunQuery.Limit != 2 {
			t.Fatalf(
				"ListTaskRuns calls/query = %d/%q/%#v, want queued run query",
				tasks.runListCalls,
				tasks.lastRunListTaskID,
				tasks.lastRunQuery,
			)
		}
	})

	t.Run("Should persist typed participation from native task create", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{Tasks: tasks}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-task-create", WorkspaceID: "ws-1"}

		_, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskCreate,
				Input: json.RawMessage(
					`{"scope":"workspace","title":"Coordinated task",` +
						`"network_participation":{"mode":"local"}}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_create participation) error = %v", err)
		}
		if tasks.createCalls != 1 || tasks.profileGetCalls != 0 || tasks.profileSetCalls != 0 {
			t.Fatalf(
				"task/profile calls = %d/%d/%d, want 1/0/0",
				tasks.createCalls,
				tasks.profileGetCalls,
				tasks.profileSetCalls,
			)
		}
		request := tasks.lastCreateSpec.NetworkParticipation
		if request == nil || request.Mode == nil || *request.Mode != participation.ModeLocal {
			t.Fatalf("stored network participation = %#v, want local request", request)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskCreate,
				Input:  json.RawMessage(`{"scope":"workspace","title":"Legacy","network_channel":"legacy"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(task_create legacy channel) error = %v, want ErrToolInvalidInput", err)
		}
		if tasks.createCalls != 1 {
			t.Fatalf("CreateTask calls = %d, want no write for legacy field", tasks.createCalls)
		}
	})

	t.Run("Should route task run review request list and show through task service authority", func(t *testing.T) {
		t.Parallel()

		review := taskpkg.RunReview{
			ReviewID:    "review-native",
			TaskID:      "task-native",
			RunID:       "run-native",
			Policy:      taskpkg.ReviewPolicyAlways,
			ReviewRound: 2,
			Attempt:     1,
			Status:      taskpkg.RunReviewStatusRequested,
			Reason:      "final check",
			RequestedAt: time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		tasks := &nativeTaskManager{
			requestReviewResult:  review,
			requestReviewCreated: true,
			getReview:            review,
			listReviews:          []taskpkg.RunReview{review},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-review-ops", WorkspaceID: "ws-1", AgentName: "planner"}

		requestResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunReviewRequest,
				Input: json.RawMessage(
					`{"task_id":"task-native","run_id":"run-native","policy":"always",` +
						`"review_round":2,"attempt":1,"reason":"final check"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_run_review_request) error = %v", err)
		}
		requireNativeStructuredContains(t, requestResult, []byte(`"created":true`))
		requireNativeStructuredContains(t, requestResult, []byte(`"review_id":"review-native"`))
		if tasks.requestReviewCalls != 1 ||
			tasks.lastRequestReview.TaskID != "task-native" ||
			tasks.lastRequestReview.RunID != "run-native" ||
			tasks.lastRequestReview.Policy != taskpkg.ReviewPolicyAlways ||
			tasks.lastRequestReview.ReviewRound != 2 {
			t.Fatalf(
				"RequestRunReview calls/request = %d/%#v, want normalized request",
				tasks.requestReviewCalls,
				tasks.lastRequestReview,
			)
		}

		listResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunReviewList,
				Input:  json.RawMessage(`{"task_id":"task-native","status":"requested","limit":5}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_run_review_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"reviews":[`))
		requireNativeStructuredContains(t, listResult, []byte(`"review_id":"review-native"`))
		if tasks.listReviewCalls != 1 ||
			tasks.lastListReviewQuery.TaskID != "task-native" ||
			tasks.lastListReviewQuery.Status != taskpkg.RunReviewStatusRequested ||
			tasks.lastListReviewQuery.Limit != 5 {
			t.Fatalf(
				"ListRunReviews calls/query = %d/%#v, want filtered query",
				tasks.listReviewCalls,
				tasks.lastListReviewQuery,
			)
		}

		showResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunReviewShow,
				Input:  json.RawMessage(`{"review_id":"review-native"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_run_review_show) error = %v", err)
		}
		requireNativeStructuredContains(t, showResult, []byte(`"review_id":"review-native"`))
		if tasks.getReviewCalls != 1 || tasks.lastGetReviewID != "review-native" {
			t.Fatalf(
				"GetRunReview calls/id = %d/%q, want review-native",
				tasks.getReviewCalls,
				tasks.lastGetReviewID,
			)
		}
	})

	t.Run("Should reject malformed task run review request before review writes", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-review-ops"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunReviewRequest,
				Input:  json.RawMessage(`{"task_id":"task-native","run_id":"run-native","policy":"none"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(task_run_review_request invalid) error = %v, want ErrToolInvalidInput", err)
		}
		if tasks.requestReviewCalls != 0 {
			t.Fatalf("RequestRunReview calls = %d, want 0 for malformed input", tasks.requestReviewCalls)
		}
	})

	t.Run("Should route task execution profile native tools through task service authority", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{
			executionProfile: taskpkg.ExecutionProfile{
				TaskID: "task-profile",
				Worker: taskpkg.WorkerProfile{
					Mode:              taskpkg.WorkerModeSelect,
					AgentName:         "worker-a",
					Provider:          "codex",
					Model:             "gpt-5.4",
					AllowedAgentNames: []string{"worker-a"},
				},
				Review: taskpkg.ReviewProfile{
					AgentName:            "reviewer-a",
					RequiredCapabilities: []string{"review"},
				},
				Sandbox: taskpkg.SandboxPolicy{
					Mode:       taskpkg.SandboxModeRef,
					SandboxRef: "daytona",
				},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-profile", WorkspaceID: "ws-1", AgentName: "planner"}

		readResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskExecutionProfileGet,
				Input:  json.RawMessage(`{"task_id":"task-profile"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_execution_profile_get) error = %v", err)
		}
		requireNativeStructuredContains(t, readResult, []byte(`"task_id":"task-profile"`))
		requireNativeStructuredContains(t, readResult, []byte(`"agent_name":"worker-a"`))
		if tasks.profileGetCalls != 1 || tasks.lastProfileTaskID != "task-profile" {
			t.Fatalf(
				"GetExecutionProfile calls/task = %d/%q, want task-profile",
				tasks.profileGetCalls,
				tasks.lastProfileTaskID,
			)
		}

		setResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskExecutionProfileSet,
				Input: json.RawMessage(
					`{"task_id":"task-profile","profile":{` +
						`"worker":{"mode":"select","agent_name":"worker-b","required_capabilities":["build"]},` +
						`"review":{"agent_name":"reviewer-b","allowed_channel_ids":["reviews"]},` +
						`"participants":{"allowed_agent_names":["worker-b"],"required_capabilities":["build"]},` +
						`"sandbox":{"mode":"none"},"runtime":{"mode":"evidence"},` +
						`"network_participation":{"mode":"local"}}}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_execution_profile_set) error = %v", err)
		}
		requireNativeStructuredContains(t, setResult, []byte(`"agent_name":"worker-b"`))
		requireNativeStructuredContains(t, setResult, []byte(`"mode":"none"`))
		if tasks.profileSetCalls != 1 ||
			tasks.lastSetProfile.TaskID != "task-profile" ||
			tasks.lastSetProfile.Worker.AgentName != "worker-b" ||
			tasks.lastSetProfile.Participants.RequiredCapabilities[0] != "build" ||
			tasks.lastSetProfile.Runtime.Mode != taskpkg.RuntimeModeEvidence ||
			tasks.lastSetProfile.NetworkParticipation == nil ||
			tasks.lastSetProfile.NetworkParticipation.Mode == nil ||
			*tasks.lastSetProfile.NetworkParticipation.Mode != participation.ModeLocal {
			t.Fatalf(
				"SetExecutionProfile calls/profile = %d/%#v, want profile update",
				tasks.profileSetCalls,
				tasks.lastSetProfile,
			)
		}

		deleteResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskExecutionProfileDelete,
				Input:  json.RawMessage(`{"task_id":"task-profile"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_execution_profile_delete) error = %v", err)
		}
		requireNativeStructuredContains(t, deleteResult, []byte(`"deleted":true`))
		if tasks.profileDeleteCalls != 1 || tasks.lastDeleteProfileTaskID != "task-profile" {
			t.Fatalf(
				"DeleteExecutionProfile calls/task = %d/%q, want task-profile",
				tasks.profileDeleteCalls,
				tasks.lastDeleteProfileTaskID,
			)
		}
	})

	t.Run("Should reject malformed task execution profile before profile writes", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-profile", WorkspaceID: "ws-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskExecutionProfileSet,
				Input:  json.RawMessage(`{"task_id":"task-profile","profile":{"created_at":"bad"}}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(task_execution_profile_set invalid) error = %v, want ErrToolInvalidInput", err)
		}
		if tasks.profileSetCalls != 0 {
			t.Fatalf("SetExecutionProfile calls = %d, want 0 for malformed input", tasks.profileSetCalls)
		}
	})

	t.Run("Should reject conflicting nested task execution profile ids before profile writes", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-profile", WorkspaceID: "ws-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskExecutionProfileSet,
				Input: json.RawMessage(
					`{"task_id":"task-profile","profile":{"task_id":"other-task","worker":{"mode":"select"}}}`,
				),
			},
		)
		if err == nil {
			t.Fatal("Registry.Call(task_execution_profile_set conflicting ids) error = nil, want invalid input")
		}
		if got, ok := toolspkg.ReasonOf(err); !ok || got != toolspkg.ReasonSchemaInvalid {
			t.Fatalf("ReasonOf(error) = %q/%v, want %q", got, ok, toolspkg.ReasonSchemaInvalid)
		}
		if !strings.Contains(err.Error(), `profile.task_id must match task_id "task-profile"`) {
			t.Fatalf("error = %q, want conflicting task_id detail", err)
		}
		if tasks.profileSetCalls != 0 {
			t.Fatalf("SetExecutionProfile calls = %d, want 0 for conflicting ids", tasks.profileSetCalls)
		}
	})

	t.Run("Should route autonomy tools through session-bound lease lookup", func(t *testing.T) {
		t.Parallel()

		rawToken := "agh_claim_NATIVEAUTONOMY123"
		hash, err := taskpkg.ClaimTokenHash(rawToken)
		if err != nil {
			t.Fatalf("ClaimTokenHash() error = %v", err)
		}
		tasks := &nativeTaskManager{
			claimResult: &taskpkg.ClaimResult{
				Task: &taskpkg.Task{
					ID:          "task-1",
					Title:       "Autonomy task",
					Status:      taskpkg.TaskStatusInProgress,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
				},
				Run: taskpkg.Run{
					ID:             "run-1",
					TaskID:         "task-1",
					Status:         taskpkg.TaskRunStatusClaimed,
					SessionID:      "sess-agent",
					ClaimTokenHash: hash,
					RunNetworkState: &taskpkg.RunNetworkState{
						NetworkSpec: daemonTestLiveParticipation("ws-1", "builders"),
					},
					LeaseUntil: time.Now().UTC().Add(time.Minute),
				},
				ClaimToken: rawToken,
			},
			lookupHandle: taskpkg.AutonomyLeaseHandle{
				RunID:          "run-1",
				TaskID:         "task-1",
				WorkspaceID:    "ws-1",
				SessionID:      "sess-agent",
				Status:         taskpkg.TaskRunStatusClaimed,
				ClaimToken:     rawToken,
				ClaimTokenHash: hash,
				LeaseUntil:     time.Now().UTC().Add(time.Minute),
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-agent", WorkspaceID: "ws-1", AgentName: "coder"}

		claimResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunClaimNext,
				Input:  json.RawMessage(`{"run_id":"run-1","lease_seconds":60}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_run_claim_next) error = %v", err)
		}
		requireNativeStructuredContains(t, claimResult, []byte(`"claimed":true`))
		requireNativeStructuredContains(t, claimResult, []byte(`"claim_token_hash"`))
		requireNativeStructuredExcludes(t, claimResult, []byte(rawToken))
		requireNativeStructuredExcludes(t, claimResult, []byte(`"claim_token"`))
		if tasks.claimNextCalls != 1 ||
			tasks.lastClaimCriteria.RunID != "run-1" ||
			tasks.lastClaimCriteria.ClaimerSessionID != "sess-agent" ||
			tasks.lastClaimCriteria.WorkspaceID != "ws-1" ||
			tasks.lastClaimActor.Actor.Ref != "sess-agent" {
			t.Fatalf(
				"claim next calls/criteria/actor = %d/%#v/%#v, want caller session/workspace",
				tasks.claimNextCalls,
				tasks.lastClaimCriteria,
				tasks.lastClaimActor,
			)
		}

		heartbeatResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunHeartbeat,
				Input:  json.RawMessage(`{"run_id":"run-1","lease_seconds":30}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_run_heartbeat) error = %v", err)
		}
		requireNativeStructuredExcludes(t, heartbeatResult, []byte(rawToken))
		requireNativeStructuredExcludes(t, heartbeatResult, []byte(`"claim_token"`))
		if tasks.lookupCalls != 1 ||
			tasks.lastLookupSessionID != "sess-agent" ||
			tasks.lastLookupRunID != "run-1" ||
			tasks.heartbeatCalls != 1 ||
			tasks.lastHeartbeat.ClaimToken != rawToken ||
			tasks.lastHeartbeat.LeaseDuration != 30*time.Second {
			t.Fatalf(
				"heartbeat lookup/call = %d/%q/%q/%d/%#v, want session lookup and internal token writer",
				tasks.lookupCalls,
				tasks.lastLookupSessionID,
				tasks.lastLookupRunID,
				tasks.heartbeatCalls,
				tasks.lastHeartbeat,
			)
		}

		for _, tt := range []struct {
			name   string
			toolID toolspkg.ToolID
			input  json.RawMessage
		}{
			{
				name:   "Should complete with internal lease token",
				toolID: toolspkg.ToolIDTaskRunComplete,
				input:  json.RawMessage(`{"run_id":"run-1","result":{"ok":true},"created_task_ids":["task-child"]}`),
			},
			{
				name:   "Should fail with internal lease token",
				toolID: toolspkg.ToolIDTaskRunFail,
				input:  json.RawMessage(`{"run_id":"run-1","error":"boom","metadata":{"code":"E_TASK"}}`),
			},
			{
				name:   "Should release with internal lease token",
				toolID: toolspkg.ToolIDTaskRunRelease,
				input:  json.RawMessage(`{"run_id":"run-1","reason":"handoff"}`),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				result, err := registry.Call(
					t.Context(),
					scope,
					toolspkg.CallRequest{ToolID: tt.toolID, Input: tt.input},
				)
				if err != nil {
					t.Fatalf("Registry.Call(%s) error = %v", tt.toolID, err)
				}
				requireNativeStructuredExcludes(t, result, []byte(rawToken))
				requireNativeStructuredExcludes(t, result, []byte(`"claim_token"`))
			})
		}
		if tasks.lastCompletion.ClaimToken != rawToken ||
			tasks.lastFailure.ClaimToken != rawToken ||
			tasks.lastRelease.ClaimToken != rawToken {
			t.Fatalf(
				"terminal/release tokens = %q/%q/%q, want internal token",
				tasks.lastCompletion.ClaimToken,
				tasks.lastFailure.ClaimToken,
				tasks.lastRelease.ClaimToken,
			)
		}
		if !slices.Equal(tasks.lastCompletion.CreatedTaskIDs, []string{"task-child"}) {
			t.Fatalf("completion created task ids = %#v, want task-child", tasks.lastCompletion.CreatedTaskIDs)
		}
	})

	t.Run("Should project workspace capacity as a typed autonomy conflict", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{claimErr: taskpkg.ErrWorkspaceActiveRunCapReached}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-agent", WorkspaceID: "ws-1", AgentName: "coder"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunClaimNext,
				Input:  json.RawMessage(`{}`),
			},
		)
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) ||
			toolErr.Code != toolspkg.ErrorCodeConflict ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonAutonomyWorkspaceCapacity) ||
			!errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached) ||
			!errors.Is(err, toolspkg.ErrToolConflict) {
			t.Fatalf("Registry.Call(task_run_claim_next) error = %#v, want workspace capacity conflict", err)
		}
		if tasks.claimNextCalls != 1 {
			t.Fatalf("ClaimNextRun() calls = %d, want 1", tasks.claimNextCalls)
		}
	})

	t.Run("Should gate task block native tools by leased task and operator scope", func(t *testing.T) {
		t.Parallel()

		rawToken := "agh_claim_TASKBLOCK123"
		hash, err := taskpkg.ClaimTokenHash(rawToken)
		if err != nil {
			t.Fatalf("ClaimTokenHash() error = %v", err)
		}
		tasks := &nativeTaskManager{
			lookupHandle: taskpkg.AutonomyLeaseHandle{
				RunID:          "run-1",
				TaskID:         "task-1",
				WorkspaceID:    "ws-1",
				SessionID:      "sess-agent",
				Status:         taskpkg.TaskRunStatusClaimed,
				ClaimToken:     rawToken,
				ClaimTokenHash: hash,
				LeaseUntil:     time.Now().UTC().Add(time.Minute),
			},
			blockResult: taskpkg.TaskBlock{
				ID:      "block-native",
				TaskID:  "task-1",
				Kind:    taskpkg.BlockKindNeedsInput,
				Reason:  "creator input " + rawToken,
				Details: json.RawMessage(`{"note":"` + rawToken + `"}`),
			},
			clearBlockResult: taskpkg.TaskBlock{
				ID:        "block-native",
				TaskID:    "task-1",
				Kind:      taskpkg.BlockKindNeedsInput,
				Reason:    "creator input " + rawToken,
				ClearNote: "done " + rawToken,
			},
			blockList: []taskpkg.TaskBlock{
				{
					ID:        "block-native",
					TaskID:    "task-1",
					Kind:      taskpkg.BlockKindNeedsInput,
					Reason:    "creator input " + rawToken,
					ClearNote: "done " + rawToken,
				},
			},
			recoverTask: &taskpkg.Task{
				ID:          "task-1",
				Title:       "Recovered " + rawToken,
				Description: "description " + rawToken,
				Status:      taskpkg.TaskStatusReady,
				Metadata:    json.RawMessage(`{"note":"` + rawToken + `"}`),
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-agent", WorkspaceID: "ws-1", AgentName: "coder"}

		blockResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskBlock,
				Input: json.RawMessage(
					`{"task_id":"task-1","kind":"needs_input","reason":"creator input","run_id":"run-1"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_block) error = %v", err)
		}
		requireNativeStructuredContains(t, blockResult, []byte(`"block-native"`))
		requireNativeStructuredContains(t, blockResult, []byte(`agh_claim_[REDACTED]`))
		requireNativeStructuredExcludes(t, blockResult, []byte(rawToken))
		if tasks.blockCalls != 1 ||
			tasks.lookupCalls != 1 ||
			tasks.lastLookupSessionID != "sess-agent" ||
			tasks.lastLookupRunID != "run-1" ||
			tasks.lastBlockRequest.TaskID != "task-1" ||
			tasks.lastBlockRequest.RunID != "run-1" ||
			tasks.lastBlockRequest.ClaimToken != rawToken {
			t.Fatalf(
				"task block calls/lookup/request = %d/%d/%q/%q/%#v, want leased task block",
				tasks.blockCalls,
				tasks.lookupCalls,
				tasks.lastLookupSessionID,
				tasks.lastLookupRunID,
				tasks.lastBlockRequest,
			)
		}

		unblockResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskUnblock,
				Input: json.RawMessage(
					`{"task_id":"task-1","block_id":"block-native","run_id":"run-1","note":"done"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_unblock) error = %v", err)
		}
		requireNativeStructuredContains(t, unblockResult, []byte(`"block-native"`))
		requireNativeStructuredContains(t, unblockResult, []byte(`agh_claim_[REDACTED]`))
		requireNativeStructuredExcludes(t, unblockResult, []byte(rawToken))
		if tasks.clearBlockCalls != 1 ||
			tasks.lookupCalls != 2 ||
			tasks.lastClearBlockTaskID != "task-1" ||
			tasks.lastClearBlockID != "block-native" ||
			tasks.lastClearBlockNote != "done" {
			t.Fatalf(
				"task unblock calls/lookup/task/block/note = %d/%d/%q/%q/%q, want leased unblock",
				tasks.clearBlockCalls,
				tasks.lookupCalls,
				tasks.lastClearBlockTaskID,
				tasks.lastClearBlockID,
				tasks.lastClearBlockNote,
			)
		}

		blocksResult, err := registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskBlocks,
				Input:  json.RawMessage(`{"task_id":"task-1","include_cleared":true}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_blocks) error = %v", err)
		}
		requireNativeStructuredContains(t, blocksResult, []byte(`"block-native"`))
		requireNativeStructuredContains(t, blocksResult, []byte(`agh_claim_[REDACTED]`))
		requireNativeStructuredExcludes(t, blocksResult, []byte(rawToken))
		if tasks.listBlockCalls != 1 ||
			tasks.lastListBlockTaskID != "task-1" ||
			!tasks.lastListIncludeCleared {
			t.Fatalf(
				"task blocks calls/task/include = %d/%q/%v, want cleared block list",
				tasks.listBlockCalls,
				tasks.lastListBlockTaskID,
				tasks.lastListIncludeCleared,
			)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskBlock,
				Input: json.RawMessage(
					`{"task_id":"task-other","kind":"needs_input","reason":"foreign","run_id":"run-1"}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonAutonomyForeignRun)
		if tasks.blockCalls != 1 {
			t.Fatalf("BlockTask calls = %d, want 1 after foreign task denial", tasks.blockCalls)
		}

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRecover,
				Input:  json.RawMessage(`{"task_id":"task-1","note":"reviewed"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonSessionDenied)
		if tasks.recoverCalls != 0 {
			t.Fatalf("RecoverTask calls = %d, want 0 for session-scoped recover", tasks.recoverCalls)
		}

		recoverResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRecover,
				Input:  json.RawMessage(`{"task_id":"task-1","note":"operator reviewed"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_recover operator) error = %v", err)
		}
		requireNativeStructuredContains(t, recoverResult, []byte(`"task-1"`))
		requireNativeStructuredContains(t, recoverResult, []byte(`agh_claim_[REDACTED]`))
		requireNativeStructuredExcludes(t, recoverResult, []byte(rawToken))
		if tasks.recoverCalls != 1 ||
			tasks.lastRecoverTaskID != "task-1" ||
			tasks.lastRecoverNote != "operator reviewed" {
			t.Fatalf(
				"RecoverTask calls/task/note = %d/%q/%q, want operator recover",
				tasks.recoverCalls,
				tasks.lastRecoverTaskID,
				tasks.lastRecoverNote,
			)
		}
	})

	t.Run("Should map stale autonomy writer rejection after session lookup", func(t *testing.T) {
		t.Parallel()

		rawToken := "agh_claim_STALEWRITER123"
		hash, err := taskpkg.ClaimTokenHash(rawToken)
		if err != nil {
			t.Fatalf("ClaimTokenHash() error = %v", err)
		}
		tasks := &nativeTaskManager{
			lookupHandle: taskpkg.AutonomyLeaseHandle{
				RunID:          "run-1",
				TaskID:         "task-1",
				WorkspaceID:    "ws-1",
				SessionID:      "sess-agent",
				Status:         taskpkg.TaskRunStatusClaimed,
				ClaimToken:     rawToken,
				ClaimTokenHash: hash,
				LeaseUntil:     time.Now().UTC().Add(time.Minute),
			},
			heartbeatErr: fmt.Errorf("%w: writer rejected stale lease %s", taskpkg.ErrLeaseExpired, rawToken),
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-agent", WorkspaceID: "ws-1", AgentName: "coder"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunHeartbeat,
				Input:  json.RawMessage(`{"run_id":"run-1","lease_seconds":30}`),
			},
		)
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) ||
			toolErr.Code != toolspkg.ErrorCodeConflict ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonAutonomyLeaseExpired) ||
			!errors.Is(err, taskpkg.ErrLeaseExpired) ||
			!errors.Is(err, toolspkg.ErrToolConflict) {
			t.Fatalf("Registry.Call(task_run_heartbeat) error = %#v, want autonomy lease conflict", err)
		}
		if bytes.Contains([]byte(err.Error()), []byte(rawToken)) {
			t.Fatalf("Registry.Call(task_run_heartbeat) error leaked raw token: %v", err)
		}
		if tasks.lookupCalls != 1 || tasks.heartbeatCalls != 1 || tasks.lastHeartbeat.ClaimToken != rawToken {
			t.Fatalf(
				"lookup/heartbeat = %d/%d/%#v, want lookup then token-fenced writer",
				tasks.lookupCalls,
				tasks.heartbeatCalls,
				tasks.lastHeartbeat,
			)
		}
	})

	t.Run("Should route reviewer-bound submit run review through task service authority", func(t *testing.T) {
		t.Parallel()

		confidence := 0.82
		review := taskpkg.RunReview{
			ReviewID:          "review-1",
			TaskID:            "task-1",
			RunID:             "run-1",
			Policy:            taskpkg.ReviewPolicyAlways,
			ReviewRound:       1,
			Attempt:           1,
			Status:            taskpkg.RunReviewStatusInReview,
			MissingWork:       json.RawMessage(`[]`),
			ReviewerSessionID: "sess-reviewer",
			ReviewerAgentName: "reviewer",
			RequestedAt:       time.Now().UTC(),
			StartedAt:         time.Now().UTC(),
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}
		tasks := &nativeTaskManager{
			reviewBinding: taskpkg.RunReviewBinding{
				Review:            review,
				SessionID:         "sess-reviewer",
				ReviewerAgentName: "reviewer",
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Skills: newLoadedNativeSkillRegistry(t),
			Tasks:  tasks,
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-reviewer", WorkspaceID: "ws-1", AgentName: "reviewer"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunReviewSubmit,
				Input: json.RawMessage(
					`{"review_id":"review-1","run_id":"run-1","outcome":"rejected","confidence":0.82,` +
						`"reason":"missing verification","missing_work":["run final verify"],` +
						`"next_round_guidance":"Run the full gate and report evidence.",` +
						`"review_text":"The implementation is close.","delivery_id":"delivery-1"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(submit_run_review) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"review_id":"review-1"`))
		requireNativeStructuredContains(t, result, []byte(`"outcome":"rejected"`))
		requireNativeStructuredExcludes(t, result, []byte(`"claim_token"`))
		if tasks.lookupReviewCalls != 2 ||
			tasks.lastReviewSessionID != "sess-reviewer" ||
			tasks.lookupCalls != 0 ||
			tasks.recordReviewCalls != 1 {
			t.Fatalf(
				"review lookup/lease lookup/record = %d/%q/%d/%d, want bound review lookup and no lease lookup",
				tasks.lookupReviewCalls,
				tasks.lastReviewSessionID,
				tasks.lookupCalls,
				tasks.recordReviewCalls,
			)
		}
		if tasks.lastRecordReview.ReviewID != "review-1" ||
			tasks.lastRecordReview.RunID != "run-1" ||
			tasks.lastRecordReview.Verdict.Outcome != taskpkg.RunReviewOutcomeRejected ||
			tasks.lastRecordReview.Verdict.Confidence == nil ||
			*tasks.lastRecordReview.Verdict.Confidence != confidence ||
			tasks.lastRecordReview.Verdict.DeliveryID != "delivery-1" {
			t.Fatalf("RecordRunReview request = %#v, want normalized rejected verdict", tasks.lastRecordReview)
		}
		if !bytes.Equal(tasks.lastRecordReview.Verdict.MissingWork, []byte(`["run final verify"]`)) {
			t.Fatalf(
				"missing_work = %s, want [\"run final verify\"]",
				tasks.lastRecordReview.Verdict.MissingWork,
			)
		}
	})

	t.Run("Should hide submit run review from sessions without an active review binding", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Skills: newLoadedNativeSkillRegistry(t),
			Tasks:  tasks,
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-unbound", WorkspaceID: "ws-1", AgentName: "reviewer"}

		views, err := registry.List(t.Context(), scope)
		if err != nil {
			t.Fatalf("Registry.List(unbound reviewer) error = %v", err)
		}
		requireNativeViewExcludes(t, views, toolspkg.ToolIDTaskRunReviewSubmit)

		_, err = registry.Call(
			t.Context(),
			scope,
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunReviewSubmit,
				Input: json.RawMessage(
					`{"review_id":"review-1","run_id":"run-1","outcome":"approved","confidence":1,` +
						`"reason":"ok","missing_work":[],"next_round_guidance":"",` +
						`"delivery_id":"delivery-1"}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolUnavailable, toolspkg.ReasonSessionDenied)
		if tasks.recordReviewCalls != 0 {
			t.Fatalf("RecordRunReview calls = %d, want 0 for unbound session", tasks.recordReviewCalls)
		}
	})

	t.Run("Should map unbound review lookups to denied and redact backend failures", func(t *testing.T) {
		t.Parallel()

		bindingErr := nativeReviewToolError(toolspkg.ToolIDTaskRunReviewSubmit, taskpkg.ErrRunReviewNotFound)
		requireToolReason(t, bindingErr, toolspkg.ErrToolDenied, toolspkg.ReasonSessionDenied)

		rawErr := errors.New("backend leaked agh_claim_secret-123")
		wrapped := nativeReviewToolError(toolspkg.ToolIDTaskRunReviewSubmit, rawErr)
		if !errors.Is(wrapped, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("wrapped error = %v, want %v", wrapped, toolspkg.ErrToolBackendFailed)
		}
		if strings.Contains(wrapped.Error(), "agh_claim_secret-123") {
			t.Fatalf("wrapped error = %q, want redacted claim token", wrapped.Error())
		}
		if !strings.Contains(wrapped.Error(), "agh_claim_[REDACTED]") {
			t.Fatalf("wrapped error = %q, want redacted token marker", wrapped.Error())
		}
	})

	t.Run("Should reject schema-invalid submit run review input before verdict writes", func(t *testing.T) {
		t.Parallel()

		review := taskpkg.RunReview{
			ReviewID:          "review-1",
			TaskID:            "task-1",
			RunID:             "run-1",
			Policy:            taskpkg.ReviewPolicyAlways,
			ReviewRound:       1,
			Attempt:           1,
			Status:            taskpkg.RunReviewStatusInReview,
			MissingWork:       json.RawMessage(`[]`),
			ReviewerSessionID: "sess-reviewer",
			RequestedAt:       time.Now().UTC(),
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}
		tasks := &nativeTaskManager{
			reviewBinding: taskpkg.RunReviewBinding{Review: review, SessionID: "sess-reviewer"},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Skills: newLoadedNativeSkillRegistry(t),
			Tasks:  tasks,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-reviewer", WorkspaceID: "ws-1", AgentName: "reviewer"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskRunReviewSubmit,
				Input: json.RawMessage(
					`{"review_id":"review-1","run_id":"run-1","outcome":"approved",` +
						`"confidence":"bad","reason":"ok","missing_work":[],` +
						`"next_round_guidance":"","delivery_id":"delivery-1"}`,
				),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(submit_run_review invalid input) error = %v, want ErrToolInvalidInput", err)
		}
		if tasks.recordReviewCalls != 0 {
			t.Fatalf("RecordRunReview calls = %d, want 0 for schema-invalid input", tasks.recordReviewCalls)
		}
	})

	t.Run("Should route child creation through task child-lineage service boundary", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{
			childErr: fmt.Errorf("%w: child parent task id is required", taskpkg.ErrValidation),
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskChildCreate,
				Input: json.RawMessage(
					`{"parent_task_id":"parent-1","scope":"workspace","title":"child",` +
						`"network_participation":{"mode":"local"}}`,
				),
			},
		)
		if !errors.Is(err, taskpkg.ErrValidation) || !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("Registry.Call(task_child_create) error = %v, want wrapped task validation", err)
		}
		if tasks.createCalls != 0 {
			t.Fatalf("CreateTask calls = %d, want 0", tasks.createCalls)
		}
		if tasks.childCreateCalls != 1 {
			t.Fatalf("CreateChildTask calls = %d, want 1", tasks.childCreateCalls)
		}
		if tasks.childParentID != "parent-1" {
			t.Fatalf("child parent id = %q, want parent-1", tasks.childParentID)
		}
		if tasks.childSpec.WorkspaceID != "ws-1" {
			t.Fatalf("child workspace_id = %q, want caller workspace fallback", tasks.childSpec.WorkspaceID)
		}
		if tasks.childSpec.NetworkParticipation == nil ||
			tasks.childSpec.NetworkParticipation.Mode == nil ||
			*tasks.childSpec.NetworkParticipation.Mode != participation.ModeLocal {
			t.Fatalf("child participation = %#v, want local request", tasks.childSpec.NetworkParticipation)
		}
	})

	t.Run("Should roll back promoted task when thread origin persistence fails", func(t *testing.T) {
		t.Parallel()

		tasks := &nativeTaskManager{}
		originErr := errors.New("origin persistence failed")
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks: tasks,
			NetworkStore: apitest.StubNetworkStore{
				ListConversationMessagesFn: func(
					_ context.Context,
					ref store.NetworkConversationRef,
					query store.NetworkConversationMessageQuery,
				) ([]store.NetworkConversationMessage, error) {
					if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "builders" ||
						ref.ThreadID != "thread_promote" || query.Limit != 200 {
						t.Fatalf("ListConversationMessages() ref=%#v query=%#v", ref, query)
					}
					return []store.NetworkConversationMessage{{
						MessageID: "msg-origin",
						PeerFrom:  "reviewer.sess-a",
						Text:      "promote this",
					}}, nil
				},
				GetThreadFn: func(
					_ context.Context,
					ref store.NetworkChannelRef,
					threadID string,
				) (store.NetworkThreadSummary, error) {
					if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "builders" ||
						threadID != "thread_promote" {
						t.Fatalf("GetThread() ref=%#v threadID=%q", ref, threadID)
					}
					return store.NetworkThreadSummary{
						WorkspaceID: nativeNetworkTestWorkspaceID,
						Channel:     "builders",
						ThreadID:    "thread_promote",
						Title:       "Promoted task",
					}, nil
				},
				PutNetworkTaskThreadOriginFn: func(context.Context, store.NetworkTaskThreadOrigin) error {
					return originErr
				},
			},
			Sessions:   nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskPromoteFromThread,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","thread_id":"thread_promote","origin_message_id":"msg-origin"}`,
				),
			},
		)
		if !errors.Is(err, originErr) {
			t.Fatalf("Registry.Call(task_promote_from_thread) error = %v, want origin error", err)
		}
		if tasks.createCalls != 1 {
			t.Fatalf("CreateTask calls = %d, want 1", tasks.createCalls)
		}
		if tasks.deleteCalls != 1 || tasks.lastDeleteID != "task-created" {
			t.Fatalf(
				"DeleteTask calls/id = %d/%q, want rollback of task-created",
				tasks.deleteCalls,
				tasks.lastDeleteID,
			)
		}
	})

	t.Run("Should keep scanning native thread promotion source ids after digest limit", func(t *testing.T) {
		t.Parallel()

		digest, sourceIDs, found := nativeNetworkThreadPromotionDigest([]store.NetworkConversationMessage{
			{
				MessageID: "msg-large",
				PeerFrom:  "coordinator.sess-a",
				Text:      strings.Repeat("large preview ", 420),
			},
			{
				MessageID: "msg-target",
				PeerFrom:  "reviewer.sess-b",
				Text:      "target message",
			},
		}, "msg-target")
		if !found {
			t.Fatal("nativeNetworkThreadPromotionDigest() found = false, want true")
		}
		if !slices.Contains(sourceIDs, "msg-target") {
			t.Fatalf("sourceIDs = %#v, want target id after digest limit", sourceIDs)
		}
		if len(digest) > 4000 {
			t.Fatalf("len(digest) = %d, want <= 4000", len(digest))
		}
	})

	t.Run("Should reject native fan-out designations before enqueue metadata is used", func(t *testing.T) {
		t.Parallel()

		_, err := prepareNativeFanOutDesignations(taskFanOutRunsInput{
			IdempotencyKey: "fanout-key",
			Designations: []contract.TaskFanOutRunDesignationRequest{
				{Brief: "Valid lane"},
				{Brief: "   "},
			},
		}, aghconfig.DefaultTaskDesignatedRunMax)
		if err == nil || !strings.Contains(err.Error(), "designations[1].brief") {
			t.Fatalf("prepareNativeFanOutDesignations() error = %v, want second designation validation", err)
		}

		_, err = prepareNativeFanOutDesignations(taskFanOutRunsInput{
			Designations: []contract.TaskFanOutRunDesignationRequest{{Brief: "Missing idempotency"}},
		}, aghconfig.DefaultTaskDesignatedRunMax)
		if err == nil || !strings.Contains(err.Error(), "idempotency_key") {
			t.Fatalf("prepareNativeFanOutDesignations() error = %v, want idempotency validation", err)
		}
	})

	t.Run("Should list network peers through the existing network service boundary", func(t *testing.T) {
		t.Parallel()

		networkService := &nativeNetworkStub{
			peers: []network.PeerInfo{{PeerID: "peer-1"}},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:    networkService,
			Sessions:   nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkPeers,
				Input:  json.RawMessage(`{"workspace_id":"ws-native-network","channel":"default"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_peers) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"peer-1"`))
		if networkService.peersCalls != 1 ||
			networkService.peersWorkspaceID != nativeNetworkTestWorkspaceID ||
			networkService.peersChannel != "default" {
			t.Fatalf(
				"ListPeers calls/workspace/channel = %d/%q/%q, want native workspace/default channel",
				networkService.peersCalls,
				networkService.peersWorkspaceID,
				networkService.peersChannel,
			)
		}
	})

	t.Run("Should read network inspection tools through the existing network service boundary", func(t *testing.T) {
		t.Parallel()

		networkService := &nativeNetworkStub{
			status:   &network.Status{Enabled: true, Status: network.StatusActive, LocalPeers: 2},
			channels: []network.ChannelInfo{{Channel: "builders", PeerCount: 2}},
			inbox: []network.Envelope{{
				ID:      "msg-1",
				Kind:    network.KindSay,
				Channel: "builders",
				From:    "peer-1",
				Body:    json.RawMessage(`{"text":"hello"}`),
			}},
		}
		usageStore := &nativeNetworkUsageStub{
			report: store.NetworkUsageReport{
				Total:      store.NetworkUsageSummary{WakeCount: 3, ActualWakeCount: 2},
				NextCursor: "cursor-next",
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:      networkService,
			NetworkUsage: usageStore,
			Sessions:     nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
			Workspaces:   nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		statusResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDNetworkStatus},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_status) error = %v", err)
		}
		requireNativeStructuredContains(t, statusResult, []byte(`"local_peers":2`))

		usageResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkUsage,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","owner_kind":"task_run","owner_id":"run-1","channel":"builders","limit":25}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_usage) error = %v", err)
		}
		requireNativeStructuredContains(t, usageResult, []byte(`"next_cursor":"cursor-next"`))
		if usageStore.query.WorkspaceID != nativeNetworkTestWorkspaceID ||
			usageStore.query.Owner == nil || usageStore.query.Owner.Kind != participation.OwnerKindTaskRun ||
			usageStore.query.Owner.ID != "run-1" || usageStore.query.Channel != "builders" ||
			usageStore.query.Limit != 25 {
			t.Fatalf("network usage query = %#v", usageStore.query)
		}

		channelsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkChannels,
				Input:  json.RawMessage(`{"workspace_id":"ws-native-network"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_channels) error = %v", err)
		}
		requireNativeStructuredContains(t, channelsResult, []byte(`"channel":"builders"`))

		inboxResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkInbox,
				Input:  json.RawMessage(`{"workspace_id":"ws-native-network"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_inbox) error = %v", err)
		}
		requireNativeStructuredContains(t, inboxResult, []byte(`"msg-1"`))
		if networkService.statusCalls != 1 ||
			networkService.channelsCalls != 1 ||
			networkService.inboxCalls != 1 ||
			networkService.inboxSessionID != "sess-1" {
			t.Fatalf("network inspection calls = %#v", networkService)
		}
	})

	t.Run("Should send network messages through the existing network service boundary", func(t *testing.T) {
		t.Parallel()

		networkService := &nativeNetworkStub{
			sendErr: fmt.Errorf("%w: session=sess-missing", network.ErrLocalPeerNotFound),
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:  networkService,
			Sessions: nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
			Workspaces: nativeNetworkTestWorkspaceServiceWithIdentity(
				t,
				nativeNetworkTestWorkspaceIdentityID,
			),
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-scope"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkSend,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-missing","channel":"default","surface":"thread","thread_id":"thread_native_send","kind":"say","body":{"text":"hello"}}`,
				),
			},
		)
		if !errors.Is(err, network.ErrLocalPeerNotFound) || !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("Registry.Call(network_send) error = %v, want wrapped network error", err)
		}
		if networkService.sendCalls != 1 {
			t.Fatalf("Network.Send calls = %d, want 1", networkService.sendCalls)
		}
		if networkService.lastSend.SessionID != "sess-scope" {
			t.Fatalf("SendRequest.SessionID = %q, want scoped session", networkService.lastSend.SessionID)
		}
		if networkService.lastSend.WorkspaceID != nativeNetworkTestWorkspaceID {
			t.Fatalf(
				"SendRequest.WorkspaceID = %q, want registry workspace %q",
				networkService.lastSend.WorkspaceID,
				nativeNetworkTestWorkspaceID,
			)
		}
		if networkService.lastSend.Surface == nil || *networkService.lastSend.Surface != network.SurfaceThread {
			t.Fatalf("SendRequest.Surface = %v, want thread", networkService.lastSend.Surface)
		}
		if got, want := string(networkService.lastSend.Body), `{"text":"hello"}`; got != want {
			t.Fatalf("SendRequest.Body = %s, want %s", got, want)
		}
	})

	t.Run("Should dispatch native network thread direct and work tools through the store boundary", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		sessionID := "sess-local"
		remoteSessionID := "sess-reviewer"
		directID, sessionA, sessionB, err := store.NetworkDirectRoomIdentity(
			nativeNetworkTestWorkspaceID,
			"builders",
			sessionID,
			remoteSessionID,
		)
		if err != nil {
			t.Fatalf("DirectRoomIdentity() error = %v", err)
		}
		resolvedDirects := make(map[string]store.NetworkDirectRoomSummary)
		resolveCalls := 0
		storeStub := apitest.StubNetworkStore{
			ListThreadsFn: func(
				_ context.Context,
				ref store.NetworkChannelRef,
				query store.NetworkThreadQuery,
			) (store.NetworkThreadPage, error) {
				if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "builders" || query.Limit != 2 ||
					query.After != "thread_root" {
					t.Fatalf("ListThreads ref/query = %#v/%#v, want requested filters", ref, query)
				}
				return store.NetworkThreadPage{Threads: []store.NetworkThreadSummary{{
					WorkspaceID:        ref.WorkspaceID,
					Channel:            ref.Channel,
					ThreadID:           "thread_launch",
					RootMessageID:      "msg_thread_root",
					Title:              "Launch",
					OpenedByPeerID:     "coder.sess-abc",
					OpenedSessionID:    sessionID,
					OpenedAt:           now,
					LastActivityAt:     now,
					MessageCount:       2,
					ParticipantCount:   2,
					OpenWorkCount:      1,
					LastMessagePreview: "ready",
				}}, Total: 5, Limit: 2, HasMore: true, NextCursor: "opaque-thread-next"}, nil
			},
			ListDirectRoomsFn: func(
				_ context.Context,
				ref store.NetworkChannelRef,
				query store.NetworkDirectRoomQuery,
			) (store.NetworkDirectRoomPage, error) {
				if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "builders" ||
					query.SessionID != remoteSessionID ||
					query.Limit != 3 {
					t.Fatalf("ListDirectRooms ref/query = %#v/%#v, want requested filters", ref, query)
				}
				return store.NetworkDirectRoomPage{Directs: []store.NetworkDirectRoomSummary{{
					WorkspaceID:        ref.WorkspaceID,
					Channel:            ref.Channel,
					DirectID:           directID,
					SessionA:           sessionA,
					SessionB:           sessionB,
					OpenedAt:           now,
					LastActivityAt:     now,
					MessageCount:       1,
					OpenWorkCount:      1,
					LastMessagePreview: "handoff",
				}}, Total: 4, Limit: 3, HasMore: true, NextCursor: "opaque-direct-next"}, nil
			},
			ResolveDirectRoomFn: func(
				_ context.Context,
				entry store.NetworkDirectRoomEntry,
			) (store.NetworkDirectRoomSummary, error) {
				resolveCalls++
				if entry.WorkspaceID != nativeNetworkTestWorkspaceID || entry.Channel != "builders" ||
					entry.DirectID != directID || entry.SessionA != sessionA ||
					entry.SessionB != sessionB {
					t.Fatalf("ResolveDirectRoom entry = %#v, want deterministic direct room", entry)
				}
				if summary, ok := resolvedDirects[entry.DirectID]; ok {
					return summary, nil
				}
				summary := store.NetworkDirectRoomSummary{
					WorkspaceID:        entry.WorkspaceID,
					Channel:            entry.Channel,
					DirectID:           entry.DirectID,
					SessionA:           entry.SessionA,
					SessionB:           entry.SessionB,
					OpenedAt:           entry.OpenedAt,
					LastActivityAt:     entry.LastActivityAt,
					MessageCount:       0,
					OpenWorkCount:      0,
					LastMessagePreview: "created",
				}
				resolvedDirects[entry.DirectID] = summary
				return summary, nil
			},
			ListConversationMessagesFn: func(
				_ context.Context,
				ref store.NetworkConversationRef,
				query store.NetworkConversationMessageQuery,
			) ([]store.NetworkConversationMessage, error) {
				switch ref.Surface {
				case store.NetworkSurfaceThread:
					if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "builders" ||
						ref.ThreadID != "thread_launch" ||
						query.Kind != store.NetworkKindSay || query.WorkID != "work_launch" {
						t.Fatalf("ListConversationMessages thread ref/query = %#v/%#v", ref, query)
					}
					return []store.NetworkConversationMessage{{
						MessageID:   "msg_thread_launch",
						Channel:     ref.Channel,
						Surface:     ref.Surface,
						ThreadID:    ref.ThreadID,
						Direction:   "sent",
						PeerFrom:    "coder.sess-abc",
						Kind:        store.NetworkKindSay,
						WorkID:      "work_launch",
						Text:        "secret agh_claim_RESULT123",
						PreviewText: "secret agh_claim_RESULT123",
						Body:        json.RawMessage(`{"text":"secret agh_claim_BODY123"}`),
						Timestamp:   now,
					}}, nil
				case store.NetworkSurfaceDirect:
					if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "builders" ||
						ref.DirectID != directID || query.Limit != 5 {
						t.Fatalf("ListConversationMessages direct ref/query = %#v/%#v", ref, query)
					}
					return []store.NetworkConversationMessage{{
						MessageID:   "msg_direct_launch",
						Channel:     ref.Channel,
						Surface:     ref.Surface,
						DirectID:    ref.DirectID,
						Direction:   "received",
						PeerFrom:    "reviewer.sess-xyz",
						Kind:        store.NetworkKindTrace,
						WorkID:      "work_direct",
						PreviewText: "handoff",
						Body:        json.RawMessage(`{"state":"needs_input"}`),
						Timestamp:   now,
					}}, nil
				default:
					t.Fatalf("ListConversationMessages ref = %#v, want thread or direct", ref)
					return nil, nil
				}
			},
			GetWorkFn: func(_ context.Context, workspaceID string, workID string) (store.NetworkWorkEntry, error) {
				if workspaceID != nativeNetworkTestWorkspaceID || workID != "work_launch" {
					t.Fatalf(
						"GetWork workspaceID/workID = %q/%q, want native workspace/work_launch",
						workspaceID,
						workID,
					)
				}
				return store.NetworkWorkEntry{
					WorkID:            workID,
					WorkspaceID:       workspaceID,
					Channel:           "builders",
					Surface:           store.NetworkSurfaceThread,
					ThreadID:          "thread_launch",
					OpenedBySessionID: sessionID,
					TargetSessionID:   remoteSessionID,
					State:             "needs_input",
					OpenedAt:          now,
					LastActivityAt:    now,
				}, nil
			},
		}
		networkService := &nativeNetworkStub{
			peers: []network.PeerInfo{
				{SessionID: &sessionID, PeerID: "coder.sess-abc", Channel: "builders", Local: true},
				{SessionID: &remoteSessionID, PeerID: "reviewer.sess-xyz", Channel: "builders", Local: false},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:      networkService,
			NetworkStore: storeStub,
			Sessions:     nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
			Workspaces:   nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		threadsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkThreads,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","limit":2,"after":"thread_root"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_threads) error = %v", err)
		}
		requireNativeStructuredContains(t, threadsResult, []byte(`"thread_launch"`))
		requireNativeStructuredContains(t, threadsResult, []byte(`"total":5`))
		requireNativeStructuredContains(t, threadsResult, []byte(`"limit":2`))
		requireNativeStructuredContains(t, threadsResult, []byte(`"opaque-thread-next"`))

		threadMessagesResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkThreadMessages,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","thread_id":"thread_launch","kind":"say","work_id":"work_launch","limit":5}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_thread_messages) error = %v", err)
		}
		requireNativeStructuredContains(t, threadMessagesResult, []byte(`"msg_thread_launch"`))
		requireNativeStructuredContains(t, threadMessagesResult, []byte(`"limit":5`))
		requireNativeStructuredExcludes(t, threadMessagesResult, []byte(`agh_claim_RESULT123`))
		requireNativeStructuredExcludes(t, threadMessagesResult, []byte(`agh_claim_BODY123`))

		directsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkDirects,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","session_id":"sess-reviewer","limit":3}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_directs) error = %v", err)
		}
		requireNativeStructuredContains(t, directsResult, []byte(directID))
		requireNativeStructuredContains(t, directsResult, []byte(`"total":4`))
		requireNativeStructuredContains(t, directsResult, []byte(`"limit":3`))
		requireNativeStructuredContains(t, directsResult, []byte(`"opaque-direct-next"`))

		for i := range 2 {
			resolveResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{SessionID: sessionID},
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDNetworkDirectResolve,
					Input: json.RawMessage(
						`{"workspace_id":"ws-native-network","channel":"builders","peer_id":"reviewer.sess-xyz"}`,
					),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(network_direct_resolve #%d) error = %v", i+1, err)
			}
			requireNativeStructuredContains(t, resolveResult, []byte(directID))
		}
		if resolveCalls != 2 || len(resolvedDirects) != 1 {
			t.Fatalf(
				"direct resolve calls/map = %d/%d, want idempotent same direct room",
				resolveCalls,
				len(resolvedDirects),
			)
		}

		directMessagesResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkDirectMessages,
				Input: json.RawMessage(
					fmt.Sprintf(
						`{"workspace_id":"ws-native-network","channel":"builders","direct_id":%q,"kind":"trace","work_id":"work_direct","limit":4}`,
						directID,
					),
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_direct_messages) error = %v", err)
		}
		requireNativeStructuredContains(t, directMessagesResult, []byte(`"msg_direct_launch"`))
		requireNativeStructuredContains(t, directMessagesResult, []byte(`"limit":4`))

		workResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkWork,
				Input:  json.RawMessage(`{"workspace_id":"ws-native-network","work_id":"work_launch"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(network_work) error = %v", err)
		}
		requireNativeStructuredContains(t, workResult, []byte(`"state":"needs_input"`))
	})

	t.Run(
		"Should reject native network send with the same conversation validation as HTTP payloads",
		func(t *testing.T) {
			t.Parallel()

			networkService := &nativeNetworkStub{}
			registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
				Network:    networkService,
				Sessions:   nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
				Workspaces: nativeNetworkTestWorkspaceService(t),
			}, nativeApproveAllPolicyInputs())
			invalidPayloads := []json.RawMessage{
				json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-scope","channel":"","surface":"thread","thread_id":"thread_bad","kind":"say","body":{"text":"blank channel"}}`,
				),
				json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-scope","channel":"default","surface":"thread","kind":"say","body":{"text":"missing thread"}}`,
				),
				json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-scope","channel":"default","surface":"direct","kind":"say","body":{"text":"missing direct"}}`,
				),
				json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-scope","channel":"default","surface":"thread","thread_id":"thread_bad","direct_id":"direct_99401d24bee62651d189e5a561785466","kind":"say","body":{"text":"both"}}`,
				),
				json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-scope","channel":"default","surface":"thread","thread_id":"thread_bad","kind":"receipt","body":{"status":"accepted"}}`,
				),
			}
			for _, input := range invalidPayloads {
				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{SessionID: "sess-scope"},
					toolspkg.CallRequest{ToolID: toolspkg.ToolIDNetworkSend, Input: input},
				)
				if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
					t.Fatalf("Registry.Call(network_send %s) error = %v, want ErrToolInvalidInput", input, err)
				}
			}
			if networkService.sendCalls != 0 {
				t.Fatalf("Network.Send calls = %d, want 0", networkService.sendCalls)
			}
		},
	)

	t.Run("Should reject native network read inputs through validation helpers", func(t *testing.T) {
		t.Parallel()

		sessionID := "sess-local"
		cases := []struct {
			name  string
			scope toolspkg.Scope
			id    toolspkg.ToolID
			input json.RawMessage
		}{
			{
				name:  "Should reject blank thread list channel",
				id:    toolspkg.ToolIDNetworkThreads,
				input: json.RawMessage(`{"workspace_id":"ws-native-network","channel":""}`),
			},
			{
				name:  "Should reject negative thread list limit",
				id:    toolspkg.ToolIDNetworkThreads,
				input: json.RawMessage(`{"workspace_id":"ws-native-network","channel":"builders","limit":-1}`),
			},
			{
				name:  "Should reject invalid thread message container",
				id:    toolspkg.ToolIDNetworkThreadMessages,
				input: json.RawMessage(`{"workspace_id":"ws-native-network","channel":"builders","thread_id":"bad"}`),
			},
			{
				name: "Should reject conflicting thread message cursors",
				id:   toolspkg.ToolIDNetworkThreadMessages,
				input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","thread_id":"thread_launch","before":"msg_later","after":"msg_earlier"}`,
				),
			},
			{
				name:  "Should reject negative direct list limit",
				id:    toolspkg.ToolIDNetworkDirects,
				input: json.RawMessage(`{"workspace_id":"ws-native-network","channel":"builders","limit":-1}`),
			},
			{
				name:  "Should reject invalid direct message container",
				id:    toolspkg.ToolIDNetworkDirectMessages,
				input: json.RawMessage(`{"workspace_id":"ws-native-network","channel":"builders","direct_id":"bad"}`),
			},
			{
				name: "Should require direct resolve caller session",
				id:   toolspkg.ToolIDNetworkDirectResolve,
				input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","peer_id":"coder.sess-abc"}`,
				),
			},
			{
				name:  "Should reject invalid direct resolve peer id",
				scope: toolspkg.Scope{SessionID: sessionID},
				id:    toolspkg.ToolIDNetworkDirectResolve,
				input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","peer_id":"bad peer"}`,
				),
			},
			{
				name:  "Should reject same-peer direct resolve",
				scope: toolspkg.Scope{SessionID: sessionID},
				id:    toolspkg.ToolIDNetworkDirectResolve,
				input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","peer_id":"coder.sess-abc"}`,
				),
			},
			{
				name:  "Should reject invalid work lookup id",
				id:    toolspkg.ToolIDNetworkWork,
				input: json.RawMessage(`{"workspace_id":"ws-native-network","work_id":"bad/path"}`),
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				networkService := &nativeNetworkStub{
					peers: []network.PeerInfo{{
						SessionID: &sessionID,
						PeerID:    "coder.sess-abc",
						Channel:   "builders",
						Local:     true,
					}},
				}
				registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
					Network:      networkService,
					NetworkStore: apitest.StubNetworkStore{},
					Sessions:     nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
					Workspaces:   nativeNetworkTestWorkspaceService(t),
				}, nativeApproveAllPolicyInputs())
				_, err := registry.Call(t.Context(), tc.scope, toolspkg.CallRequest{ToolID: tc.id, Input: tc.input})
				requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
			})
		}
	})

	t.Run("Should surface unresolved native direct peer lookup as a backend network error", func(t *testing.T) {
		t.Parallel()

		sessionID := "sess-local"
		networkService := &nativeNetworkStub{
			peers: []network.PeerInfo{{
				SessionID: &sessionID,
				PeerID:    "coder.sess-abc",
				Channel:   "builders",
				Local:     true,
			}},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:      networkService,
			NetworkStore: apitest.StubNetworkStore{},
			Sessions:     nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
			Workspaces:   nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: sessionID},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkDirectResolve,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","peer_id":"reviewer.sess-xyz"}`,
				),
			},
		)
		if !errors.Is(err, network.ErrTargetPeerNotFound) || !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf(
				"Registry.Call(network_direct_resolve missing peer) error = %v, want wrapped target peer error",
				err,
			)
		}
	})

	t.Run("Should surface native direct list store errors as backend failures", func(t *testing.T) {
		t.Parallel()

		storeErr := errors.New("direct list failed")
		remoteSessionID := "sess-reviewer"
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network: &nativeNetworkStub{peers: []network.PeerInfo{{
				SessionID: &remoteSessionID,
				PeerID:    "reviewer.sess-xyz",
				Channel:   "builders",
			}}},
			Workspaces: nativeNetworkTestWorkspaceService(t),
			NetworkStore: apitest.StubNetworkStore{
				ListDirectRoomsFn: func(
					context.Context,
					store.NetworkChannelRef,
					store.NetworkDirectRoomQuery,
				) (store.NetworkDirectRoomPage, error) {
					return store.NetworkDirectRoomPage{}, storeErr
				},
			},
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkDirects,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","channel":"builders","session_id":"sess-reviewer"}`,
				),
			},
		)
		if !errors.Is(err, storeErr) || !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("Registry.Call(network_directs store error) error = %v, want wrapped store backend error", err)
		}
	})

	t.Run("Should reject raw claim token fields before network send", func(t *testing.T) {
		t.Parallel()

		networkService := &nativeNetworkStub{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:    networkService,
			Sessions:   nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		const rawToken = "agh_claim_NATIVE_SECURITY_123"
		tests := []struct {
			name       string
			input      json.RawMessage
			wantReason toolspkg.ReasonCode
			secret     string
		}{
			{
				name: "raw claim token",
				input: json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-scope","channel":"default","surface":"thread","thread_id":"thread_claim_token","kind":"say","body":{"claim_token":"` + rawToken + `"}}`,
				),
				wantReason: toolspkg.ReasonNetworkRawTokenRejected,
				secret:     rawToken,
			},
			{
				name: "caller supplied verified-format identity",
				input: json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-scope","channel":"default","surface":"thread","thread_id":"thread_identity","kind":"say","from":"alice@39f713d0a644253f04529421b9f51b9b","body":{"text":"spoof"}}`,
				),
				wantReason: toolspkg.ReasonSchemaInvalid,
			},
		}
		for _, test := range tests {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{SessionID: "sess-scope"},
					toolspkg.CallRequest{ToolID: toolspkg.ToolIDNetworkSend, Input: test.input},
				)
				var toolErr *toolspkg.ToolError
				if !errors.As(err, &toolErr) ||
					toolErr.Code != toolspkg.ErrorCodeInvalidInput ||
					!slices.Contains(toolErr.ReasonCodes, test.wantReason) {
					t.Fatalf("Registry.Call(network_send) error = %#v, want reason %q", err, test.wantReason)
				}
				if test.secret != "" && strings.Contains(err.Error(), test.secret) {
					t.Fatalf("native validation error leaked raw credential: %v", err)
				}
			})
		}
		if networkService.sendCalls != 0 {
			t.Fatalf("Network.Send calls = %d, want 0", networkService.sendCalls)
		}
	})

	t.Run("Should reject raw claim token fields through hosted MCP", func(t *testing.T) {
		t.Parallel()

		networkService := &nativeNetworkStub{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:    networkService,
			Sessions:   nativeNetworkTestSessionManager("ws-1"),
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		counter := byte(1)
		service, err := mcppkg.NewHostedService(mcppkg.HostedConfig{
			Enabled:        true,
			BindNonceTTL:   time.Minute,
			ExpectedBinary: executable,
			Registry: func() toolspkg.Registry {
				return registry
			},
			Now: func() time.Time {
				return time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
			},
			NonceReader: func(dst []byte) error {
				for i := range dst {
					dst[i] = counter
					counter++
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewHostedService() error = %v", err)
		}
		launch, err := service.Launch(t.Context(), mcppkg.HostedLaunchRequest{
			SessionID:   "sess-scope",
			WorkspaceID: "ws-1",
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		peer := mcppkg.PeerInfo{
			PID:            os.Getpid(),
			UID:            os.Getuid(),
			GID:            os.Getgid(),
			ExecutablePath: executable,
			Supported:      true,
		}
		bind, err := service.Bind(
			t.Context(),
			mcppkg.HostedBindRequest{SessionID: "sess-scope", Nonce: launch.Args[len(launch.Args)-1]},
			peer,
		)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}

		_, err = service.Call(t.Context(), mcppkg.HostedCallRequest{
			BindID:   bind.BindID,
			ToolName: toolspkg.ToolIDNetworkSend.String(),
			Input: json.RawMessage(
				`{"workspace_id":"ws-1","session_id":"sess-scope","channel":"default","surface":"thread","thread_id":"thread_claim_token","kind":"say","body":{"claim_token":"agh_claim_HOSTED123"}}`,
			),
		}, peer)
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) ||
			toolErr.Code != toolspkg.ErrorCodeInvalidInput ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonNetworkRawTokenRejected) {
			t.Fatalf("HostedService.Call(network_send) error = %#v, want network_raw_token_rejected", err)
		}
		if networkService.sendCalls != 0 {
			t.Fatalf("Network.Send calls = %d, want 0", networkService.sendCalls)
		}
	})

	t.Run("Should expose hosted MCP network schemas identical to native descriptors", func(t *testing.T) {
		t.Parallel()

		networkService := &nativeNetworkStub{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:      networkService,
			NetworkStore: apitest.StubNetworkStore{},
			Sessions:     nativeNetworkTestSessionManager("ws-1"),
			Workspaces:   nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		counter := byte(42)
		service, err := mcppkg.NewHostedService(mcppkg.HostedConfig{
			Enabled:        true,
			BindNonceTTL:   time.Minute,
			ExpectedBinary: executable,
			Registry: func() toolspkg.Registry {
				return registry
			},
			Now: func() time.Time {
				return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
			},
			NonceReader: func(dst []byte) error {
				for i := range dst {
					dst[i] = counter
					counter++
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewHostedService() error = %v", err)
		}
		launch, err := service.Launch(t.Context(), mcppkg.HostedLaunchRequest{
			SessionID:   "sess-schema",
			WorkspaceID: "ws-1",
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		peer := mcppkg.PeerInfo{
			PID:            os.Getpid(),
			UID:            os.Getuid(),
			GID:            os.Getgid(),
			ExecutablePath: executable,
			Supported:      true,
		}
		bind, err := service.Bind(
			t.Context(),
			mcppkg.HostedBindRequest{SessionID: "sess-schema", Nonce: launch.Args[len(launch.Args)-1]},
			peer,
		)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		nativeDescriptors := nativeDescriptorMap(builtintools.NativeDescriptors())
		hostedViews := make(map[toolspkg.ToolID]toolspkg.ToolView, len(bind.Tools))
		for _, view := range bind.Tools {
			hostedViews[view.Descriptor.ID] = view
		}
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDNetworkSend,
			toolspkg.ToolIDNetworkThreads,
			toolspkg.ToolIDNetworkThreadMessages,
			toolspkg.ToolIDNetworkDirects,
			toolspkg.ToolIDNetworkDirectResolve,
			toolspkg.ToolIDNetworkDirectMessages,
			toolspkg.ToolIDNetworkWork,
		} {
			hosted, ok := hostedViews[id]
			if !ok {
				t.Fatalf("hosted MCP projection missing network tool %s", id)
			}
			native := nativeDescriptors[id]
			if !bytes.Equal(hosted.Descriptor.InputSchema, native.InputSchema) {
				t.Fatalf(
					"%s hosted input schema = %s, want native schema %s",
					id,
					hosted.Descriptor.InputSchema,
					native.InputSchema,
				)
			}
		}
	})

	t.Run("Should read session tools through the existing session manager boundary", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		stableWorkspaceID := "ws-stable"
		registryWorkspaceID := "ws-1"
		foreignStableWorkspaceID := "ws-foreign-stable"
		var seenListQuery session.ListQuery
		info := &session.Info{
			ID:          "sess-1",
			AgentName:   "coder",
			WorkspaceID: registryWorkspaceID,
			State:       session.StateActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		workspaces := apitest.StubWorkspaceService{
			GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				switch ref {
				case stableWorkspaceID, registryWorkspaceID:
					return workspacepkg.Workspace{ID: registryWorkspaceID}, nil
				case foreignStableWorkspaceID, "ws-other":
					return workspacepkg.Workspace{ID: "ws-other"}, nil
				default:
					return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
				}
			},
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				switch ref {
				case stableWorkspaceID, registryWorkspaceID:
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: registryWorkspaceID},
						WorkspaceID: stableWorkspaceID,
					}, nil
				case foreignStableWorkspaceID, "ws-other":
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "ws-other"},
						WorkspaceID: foreignStableWorkspaceID,
					}, nil
				default:
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
			},
		}
		manager := &nativeSessionPageHealthManager{
			StubSessionManager: apitest.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{info}, nil
				},
				ListPageFn: func(_ context.Context, query session.ListQuery) (session.ListPage, error) {
					seenListQuery = query
					return session.ListPage{
						Sessions:   []*session.Info{info},
						NextCursor: "cursor-next",
						HasMore:    true,
						Total:      3,
						Limit:      2,
					}, nil
				},
				StatusFn: func(_ context.Context, id string) (*session.Info, error) {
					if id != "sess-1" {
						return nil, session.ErrSessionNotFound
					}
					return info, nil
				},
				EventsFn: func(_ context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error) {
					if id != "sess-1" || query.Limit != 1 {
						t.Fatalf("Events query = %q %#v", id, query)
					}
					return []store.SessionEvent{{
						ID:        "event-1",
						SessionID: id,
						Sequence:  1,
						TurnID:    "turn-1",
						Type:      "agent_message",
						AgentName: "coder",
						Content:   `{"text":"hello"}`,
						Timestamp: now,
					}}, nil
				},
				HistoryFn: func(_ context.Context, id string, query store.EventQuery) ([]store.TurnHistory, error) {
					if id != "sess-1" || query.Limit != 1 {
						t.Fatalf("History query = %q %#v", id, query)
					}
					return []store.TurnHistory{{
						TurnID: "turn-1",
						Events: []store.SessionEvent{{
							ID:        "event-1",
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
			},
			healthByID: map[string]heartbeat.SessionHealth{
				"sess-1": {
					SessionID:   "sess-1",
					WorkspaceID: registryWorkspaceID,
					AgentName:   "coder",
					State:       heartbeat.SessionHealthStateIdle,
					Health:      heartbeat.SessionHealthHealthy,
					Attachable:  true,
					UpdatedAt:   now,
				},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: workspaces,
			Sessions:   manager,
		}, nativeApproveAllPolicyInputs())

		for _, tc := range []struct {
			id    toolspkg.ToolID
			input json.RawMessage
			want  []byte
		}{
			{toolspkg.ToolIDSessionList, nil, []byte(`"sess-1"`)},
			{toolspkg.ToolIDSessionStatus, json.RawMessage(`{"workspace_id":"ws-stable","session_id":"sess-1"}`), []byte(`"session"`)},
			{toolspkg.ToolIDSessionEvents, json.RawMessage(`{"workspace_id":"ws-stable","session_id":"sess-1","limit":1}`), []byte(`"event-1"`)},
			{toolspkg.ToolIDSessionHistory, json.RawMessage(`{"workspace_id":"ws-stable","session_id":"sess-1","limit":1}`), []byte(`"turn-1"`)},
			{toolspkg.ToolIDSessionDescribe, json.RawMessage(`{"workspace_id":"ws-stable","session_id":"sess-1","limit":1}`), []byte(`"history"`)},
		} {
			t.Run(tc.id.String(), func(t *testing.T) {
				result, err := registry.Call(
					t.Context(),
					toolspkg.Scope{},
					toolspkg.CallRequest{ToolID: tc.id, Input: tc.input},
				)
				if err != nil {
					t.Fatalf("Registry.Call(%s) error = %v", tc.id, err)
				}
				requireNativeStructuredContains(t, result, tc.want)
			})
		}

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSessionList,
				Input: json.RawMessage(
					`{"workspace":"ws-stable","state":"active","type":"user","agent":"coder","q":"review",` +
						`"resumable":true,"include_health":true,"sort":"last_activity",` +
						`"cursor":"cursor-prev","limit":2}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(session_list page) error = %v; cause=%v", err, errors.Unwrap(err))
		}
		if seenListQuery.WorkspaceID == "" || seenListQuery.State != "active" ||
			seenListQuery.SessionType != session.SessionTypeUser ||
			seenListQuery.AgentName != "coder" || seenListQuery.Search != "review" ||
			!seenListQuery.Resumable || seenListQuery.Sort != "last_activity" ||
			seenListQuery.Cursor != "cursor-prev" || seenListQuery.Limit != 2 {
			t.Fatalf("session list query = %#v, want full paged filter parity", seenListQuery)
		}
		var listResponse contract.SessionCatalogResponse
		if err := json.Unmarshal(listResult.Structured, &listResponse); err != nil {
			t.Fatalf("json.Unmarshal(session_list result) error = %v", err)
		}
		if len(listResponse.Sessions) != 1 || listResponse.Page.NextCursor != "cursor-next" ||
			!listResponse.Page.HasMore || listResponse.Page.Total != 3 || listResponse.Page.Limit != 2 {
			t.Fatalf("session_list response = %#v, want truthful page", listResponse)
		}
		if manager.healthCalls != 1 || len(manager.healthInfos) != 1 || manager.healthInfos[0] != info {
			t.Fatalf(
				"SessionHealthForPage() calls/infos = %d/%#v, want one returned-page batch",
				manager.healthCalls,
				manager.healthInfos,
			)
		}
		if health := listResponse.Sessions[0].Health; health == nil ||
			health.State != contract.SessionHealthStateIdle || health.Health != contract.SessionHealthHealthy {
			t.Fatalf("session_list health = %#v, want nested page health", health)
		}

		registryWithoutHealth := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: workspaces,
			Sessions:   manager.StubSessionManager,
		}, nativeApproveAllPolicyInputs())
		if _, defaultErr := registryWithoutHealth.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDSessionList},
		); defaultErr != nil {
			t.Fatalf("Registry.Call(session_list without page health) error = %v", defaultErr)
		}
		_, missingHealthErr := registryWithoutHealth.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSessionList,
				Input:  json.RawMessage(`{"include_health":true}`),
			},
		)
		if !errors.Is(missingHealthErr, toolspkg.ErrToolUnavailable) {
			t.Fatalf(
				"Registry.Call(session_list include_health without capability) error = %v, want unavailable",
				missingHealthErr,
			)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSessionStatus,
				Input:  json.RawMessage(`{"workspace_id":"ws-foreign-stable","session_id":"sess-1"}`),
			},
		)
		if err == nil {
			t.Fatal("Registry.Call(session_status foreign workspace) error = nil, want ownership rejection")
		}
	})

	t.Run("Should expose authored context tools only through managed services", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 29, 14, 30, 0, 0, time.UTC)
		health := heartbeat.SessionHealth{
			SessionID:           "sess-heartbeat",
			WorkspaceID:         "ws-1",
			AgentName:           "coder",
			State:               heartbeat.SessionHealthStateIdle,
			Health:              heartbeat.SessionHealthHealthy,
			Attachable:          true,
			EligibleForWake:     true,
			IneligibilityReason: "",
			UpdatedAt:           now,
		}
		workspace := apitest.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-1" {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      "ws-1",
						RootDir: "/workspace/agh",
						Name:    "agh",
					},
					WorkspaceID: "ws-1",
					Config: aghconfig.Config{
						Agents: aghconfig.AgentsConfig{Heartbeat: aghconfig.DefaultHeartbeatConfig()},
					},
					Agents: []aghconfig.AgentDef{{
						Name:       "coder",
						SourcePath: "/workspace/agh/.agh/agents/coder/AGENT.md",
					}},
				}, nil
			},
		}
		status := &nativeHeartbeatStatusStub{
			result: heartbeat.StatusResult{
				AgentName:    "coder",
				Enabled:      true,
				Present:      true,
				Active:       true,
				Valid:        true,
				Digest:       "hb-digest",
				ConfigDigest: "cfg-digest",
				SnapshotID:   "hbs-1",
				Summary:      "check in",
				WakeState: &heartbeat.WakeState{
					WorkspaceID:      "ws-1",
					AgentName:        "coder",
					SessionID:        "sess-heartbeat",
					PolicySnapshotID: "hbs-1",
					LastResult:       heartbeat.WakeResultSkipped,
					LastReason:       heartbeat.WakeReasonQuietWindow,
					UpdatedAt:        now,
				},
				SessionHealth: &health,
			},
		}
		wake := &nativeHeartbeatWakeStub{
			result: heartbeat.WakeDecision{
				WakeEventID:      "hwe-tool",
				Result:           heartbeat.WakeResultSkipped,
				Reason:           heartbeat.WakeReasonSessionUnhealthy,
				PolicySnapshotID: "hbs-1",
				PolicyDigest:     "hb-digest",
				ConfigDigest:     "cfg-digest",
			},
		}
		wakeEvents := nativeHeartbeatWakeEventStub{events: []heartbeat.WakeEvent{{
			ID:               "hwe-history",
			WorkspaceID:      "ws-1",
			AgentName:        "coder",
			SessionID:        "sess-heartbeat",
			PolicySnapshotID: "hbs-1",
			Source:           heartbeat.WakeSourceManual,
			Result:           heartbeat.WakeResultSkipped,
			Reason:           heartbeat.WakeReasonQuietWindow,
			CreatedAt:        now,
			ExpiresAt:        now.Add(time.Hour),
		}}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces:        nativeNetworkTestWorkspaceService(t),
			WorkspaceResolver: workspace,
			Sessions:          nativeNetworkTestSessionManager("ws-1"),
			SessionHealth:     nativeSessionHealthStub{health: health},
			HeartbeatStatus:   status,
			HeartbeatWake:     wake,
			WakeEvents:        wakeEvents,
		}, nativeApproveAllPolicyInputs())

		healthResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSessionHealth,
				Input:  json.RawMessage(`{"workspace_id":"ws-1","session_id":"sess-heartbeat"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(session_health) error = %v", err)
		}
		requireNativeStructuredContains(t, healthResult, []byte(`"eligible_for_wake":true`))

		statusResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentHeartbeatStatus,
				Input: json.RawMessage(
					`{"workspace_id":"ws-1","agent_name":"coder","session_id":"sess-heartbeat","include_session_health":true,"include_recent_wake_events":true}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(agent_heartbeat_status) error = %v", err)
		}
		requireNativeStructuredContains(t, statusResult, []byte(`"snapshot_id":"hbs-1"`))
		requireNativeStructuredContains(t, statusResult, []byte(`"wake_events"`))

		wakeResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentHeartbeatWake,
				Input: json.RawMessage(
					`{"workspace_id":"ws-1","agent_name":"coder","session_id":"sess-heartbeat","dry_run":true}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(agent_heartbeat_wake) error = %v", err)
		}
		requireNativeStructuredContains(t, wakeResult, []byte(`"wake_event_id":"hwe-tool"`))
		if status.last.Target.AgentPath != "/workspace/agh/.agh/agents/coder/AGENT.md" {
			t.Fatalf("heartbeat status target path = %q, want managed agent path", status.last.Target.AgentPath)
		}
		if wake.last.SessionID != "sess-heartbeat" ||
			wake.last.WorkspaceID != "ws-1" ||
			wake.last.AgentName != "coder" ||
			wake.last.Source != heartbeat.WakeSourceManual {
			t.Fatalf("heartbeat wake request = %#v, want managed manual wake target", wake.last)
		}
		if status.calls != 1 {
			t.Fatalf("heartbeat status calls = %d, want 1 after owned session status", status.calls)
		}
		if wake.calls != 1 {
			t.Fatalf("heartbeat wake calls = %d, want 1 after owned session wake", wake.calls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentHeartbeatStatus,
				Input: json.RawMessage(
					[]byte(
						"{\"workspace_id\":\"ws-foreign\",\"agent_name\":\"coder\",\"session_id\":\"sess-heartbeat\",\"include_session_health\":true}",
					),
				),
			},
		)
		requireToolCode(t, err, toolspkg.ErrorCodeBackendFailed)
		if status.calls != 1 {
			t.Fatalf("heartbeat status calls = %d, want unchanged after foreign workspace rejection", status.calls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentHeartbeatWake,
				Input: json.RawMessage(
					[]byte(
						"{\"workspace_id\":\"ws-foreign\",\"agent_name\":\"coder\",\"session_id\":\"sess-heartbeat\",\"dry_run\":true}",
					),
				),
			},
		)
		requireToolCode(t, err, toolspkg.ErrorCodeBackendFailed)
		if wake.calls != 1 {
			t.Fatalf("heartbeat wake calls = %d, want unchanged after foreign workspace rejection", wake.calls)
		}
	})

	t.Run("Should read workspace tools through the existing workspace service boundary", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		workspace := workspacepkg.Workspace{
			ID:           "ws-1",
			RootDir:      "/workspace/agh",
			Name:         "agh",
			DefaultAgent: "coder",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		manager := apitest.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{{ID: "sess-1", AgentName: "coder", WorkspaceID: "ws-1"}}, nil
			},
		}
		workspaces := apitest.StubWorkspaceService{
			ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
				return []workspacepkg.Workspace{workspace}, nil
			},
			GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				if ref != "ws-1" {
					return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspace, nil
			},
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-1" {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspace,
					WorkspaceID: "ws-1",
					Agents:      []aghconfig.AgentDef{{Name: "coder", Provider: "codex"}},
					Skills:      []workspacepkg.SkillPath{{Dir: "/workspace/agh/skills/review", Source: "workspace"}},
				}, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions:   manager,
			Workspaces: workspaces,
		}, nativeApproveAllPolicyInputs())

		for _, tc := range []struct {
			id    toolspkg.ToolID
			input json.RawMessage
			want  []byte
		}{
			{toolspkg.ToolIDWorkspaceList, nil, []byte(`"workspaces"`)},
			{toolspkg.ToolIDWorkspaceInfo, json.RawMessage(`{"workspace":"ws-1"}`), []byte(`"workspace"`)},
			{toolspkg.ToolIDWorkspaceDescribe, json.RawMessage(`{"workspace":"ws-1"}`), []byte(`"skills"`)},
		} {
			t.Run(tc.id.String(), func(t *testing.T) {
				result, err := registry.Call(
					t.Context(),
					toolspkg.Scope{},
					toolspkg.CallRequest{ToolID: tc.id, Input: tc.input},
				)
				if err != nil {
					t.Fatalf("Registry.Call(%s) error = %v", tc.id, err)
				}
				requireNativeStructuredContains(t, result, tc.want)
			})
		}
	})

	t.Run("Should read memory tools through the current memory store with redaction", func(t *testing.T) {
		t.Parallel()

		rawClaim := "agh_claim_secret123"
		globalDir := filepath.Join(t.TempDir(), "global-memory")
		catalogPath := filepath.Join(t.TempDir(), "memory.db")
		memoryStore := memorypkg.NewStore(globalDir, memorypkg.WithCatalogDatabasePath(catalogPath))
		openDaemonMemoryCatalog(t, memoryStore)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		stableWorkspaceID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		if err := memoryStore.Write(
			memcontract.ScopeGlobal,
			"global.md",
			nativeMemoryDocument(
				"Global "+rawClaim,
				"Global description "+rawClaim,
				memcontract.TypeUser,
				"global memory body "+rawClaim,
			),
		); err != nil {
			t.Fatalf("Write(global memory) error = %v", err)
		}
		if err := memoryStore.ForWorkspace(workspaceRoot).Write(
			memcontract.ScopeWorkspace,
			"workspace.md",
			nativeMemoryDocument(
				"Workspace "+rawClaim,
				"Workspace description "+rawClaim,
				memcontract.TypeProject,
				"workspace memory body "+rawClaim,
			),
		); err != nil {
			t.Fatalf("Write(workspace memory) error = %v", err)
		}
		workspaces := apitest.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-1" && ref != stableWorkspaceID {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
					WorkspaceID: stableWorkspaceID,
				}, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MemoryStore: memoryStore,
			Workspaces:  workspaces,
		}, nativeApproveAllPolicyInputs())

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryList,
				Input:  json.RawMessage(`{"scope":"workspace","workspace":"` + stableWorkspaceID + `"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"workspace.md"`))
		requireNativeStructuredExcludes(t, listResult, []byte(rawClaim))

		globalListResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryList,
				Input:  json.RawMessage(`{"scope":"global"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_list global) error = %v", err)
		}
		requireNativeStructuredContains(t, globalListResult, []byte(`"global.md"`))
		requireNativeStructuredExcludes(t, globalListResult, []byte(`"workspace.md"`))
		requireNativeStructuredExcludes(t, globalListResult, []byte(rawClaim))

		combinedListResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryList,
				Input:  json.RawMessage(`{"workspace":"` + stableWorkspaceID + `","sort":"name","limit":1}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_list combined) error = %v", err)
		}
		var firstPage contract.MemoryListResponse
		if err := json.Unmarshal(combinedListResult.Structured, &firstPage); err != nil {
			t.Fatalf("json.Unmarshal(memory_list combined) error = %v", err)
		}
		if len(firstPage.Memories) != 1 || firstPage.Memories[0].Filename != "global.md" {
			t.Fatalf("memory_list combined first page = %#v, want global.md", firstPage.Memories)
		}
		if firstPage.Page.Total != 2 || firstPage.Page.Limit != 1 || !firstPage.Page.HasMore ||
			firstPage.Page.NextCursor == "" {
			t.Fatalf("memory_list combined first page metadata = %#v", firstPage.Page)
		}
		requireNativeStructuredContains(t, combinedListResult, []byte(`"global.md"`))
		requireNativeStructuredExcludes(t, combinedListResult, []byte(`"workspace.md"`))
		requireNativeStructuredExcludes(t, combinedListResult, []byte(rawClaim))

		secondInput, err := json.Marshal(map[string]any{
			"workspace": stableWorkspaceID,
			"sort":      "name",
			"limit":     1,
			"cursor":    firstPage.Page.NextCursor,
		})
		if err != nil {
			t.Fatalf("json.Marshal(memory_list second page) error = %v", err)
		}
		secondListResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDMemoryList, Input: secondInput},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_list second page) error = %v", err)
		}
		var secondPage contract.MemoryListResponse
		if err := json.Unmarshal(secondListResult.Structured, &secondPage); err != nil {
			t.Fatalf("json.Unmarshal(memory_list second page) error = %v", err)
		}
		if len(secondPage.Memories) != 1 || secondPage.Memories[0].Filename != "workspace.md" {
			t.Fatalf("memory_list combined second page = %#v, want workspace.md", secondPage.Memories)
		}
		if secondPage.Memories[0].WorkspaceID != stableWorkspaceID {
			t.Fatalf(
				"memory_list combined second page workspace_id = %q, want %q",
				secondPage.Memories[0].WorkspaceID,
				stableWorkspaceID,
			)
		}
		if secondPage.Page.Total != 2 || secondPage.Page.Limit != 1 || secondPage.Page.HasMore {
			t.Fatalf("memory_list combined second page metadata = %#v", secondPage.Page)
		}
		requireNativeStructuredExcludes(t, secondListResult, []byte(rawClaim))

		readResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryShow,
				Input:  json.RawMessage(`{"filename":"global.md","scope":"global"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_show) error = %v", err)
		}
		requireNativeStructuredContains(t, readResult, []byte(`agh_claim_[REDACTED]`))
		requireNativeStructuredExcludes(t, readResult, []byte(rawClaim))

		workspaceReadResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryShow,
				Input: json.RawMessage(
					`{"filename":"workspace.md","scope":"workspace","workspace":"` + stableWorkspaceID + `"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_show workspace) error = %v", err)
		}
		requireNativeStructuredContains(t, workspaceReadResult, []byte(`"workspace.md"`))
		requireNativeStructuredExcludes(t, workspaceReadResult, []byte(rawClaim))

		searchResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemorySearch,
				Input:  json.RawMessage(`{"query":"workspace memory body","workspace":"` + stableWorkspaceID + `"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_search) error = %v", err)
		}
		requireNativeStructuredContains(t, searchResult, []byte(`workspace memory body`))
		requireNativeStructuredContains(
			t,
			searchResult,
			[]byte(`workspace::`+stableWorkspaceID+`::workspace.md::chunk:0001`),
		)
		requireNativeStructuredExcludes(t, searchResult, []byte(rawClaim))
		if err := memoryStore.ForWorkspace(workspaceRoot).Write(
			memcontract.ScopeWorkspace,
			"batch.md",
			nativeMemoryDocument(
				"Batch",
				"Atomic batch fixture",
				memcontract.TypeProject,
				"batch source body",
			),
		); err != nil {
			t.Fatalf("Write(batch memory) error = %v", err)
		}

		proposeResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryPropose,
				Input: json.RawMessage(
					`{"filename":"tool.md","type":"user","content":"Tool memory proposals use the controller path."}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_propose) error = %v", err)
		}
		requireNativeStructuredContains(t, proposeResult, []byte(`"decision"`))
		requireNativeStructuredContains(t, proposeResult, []byte(`"applied":true`))

		batchResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryPropose,
				Input: json.RawMessage(
					`{"scope":"workspace","workspace":"` + stableWorkspaceID +
						`","filename":"batch.md","operations":[` +
						`{"action":"replace","old_text":"batch source body","content":"workspace batch memory body"},` +
						`{"action":"add","content":"Batch-added workspace fact."}]}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_propose batch) error = %v", err)
		}
		requireNativeStructuredContains(t, batchResult, []byte(`"operations"`))
		requireNativeStructuredContains(t, batchResult, []byte(`"action":"replace"`))
		requireNativeStructuredContains(t, batchResult, []byte(`"action":"add"`))
		requireNativeStructuredContains(t, batchResult, []byte(`"applied":true`))
		workspaceBatchBytes, err := memoryStore.ForWorkspace(workspaceRoot).Read(
			memcontract.ScopeWorkspace,
			"batch.md",
		)
		if err != nil {
			t.Fatalf("Store.Read(workspace batch) error = %v", err)
		}
		if !bytes.Contains(workspaceBatchBytes, []byte("workspace batch memory body")) ||
			!bytes.Contains(workspaceBatchBytes, []byte("Batch-added workspace fact.")) {
			t.Fatalf("workspace batch bytes = %q, want both committed operations", workspaceBatchBytes)
		}
		if bytes.Contains(workspaceBatchBytes, []byte("batch source body")) {
			t.Fatalf("workspace batch bytes = %q, want replaced source text", workspaceBatchBytes)
		}

		noteResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryNote,
				Input: json.RawMessage(
					`{"content":"Remember to check release notes before deploys.","tags":["ad-hoc"]}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_note) error = %v", err)
		}
		requireNativeStructuredContains(t, noteResult, []byte(`"decision"`))

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryShow,
				Input:  json.RawMessage(`{"filename":"missing.md","scope":"global"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("Registry.Call(memory_show missing) error = %v, want ErrToolNotFound", err)
		}
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryPropose,
				Input:  json.RawMessage(`{"operation":"merge"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(memory_propose invalid op) error = %v, want ErrToolInvalidInput", err)
		}
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryPropose,
				Input: json.RawMessage(
					`{"filename":"mixed.md","content":"single","operations":[{"action":"add","content":"batch"}]}`,
				),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(memory_propose mixed shape) error = %v, want ErrToolInvalidInput", err)
		}
	})

	t.Run("Should reach an oldest matching native memory beyond the prompt scan cap", func(t *testing.T) {
		t.Parallel()

		globalDir := filepath.Join(t.TempDir(), "global-memory")
		memoryStore := memorypkg.NewStore(globalDir)
		if err := memoryStore.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		seedNativeMemoryCatalogDocuments(t, globalDir, 205)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MemoryStore: memoryStore,
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryList,
				Input:  json.RawMessage(`{"scope":"global","type":"reference","limit":1}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_list) error = %v", err)
		}
		var payload contract.MemoryListResponse
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("json.Unmarshal(memory_list) error = %v", err)
		}
		if payload.Page.Total != 1 || len(payload.Memories) != 1 ||
			payload.Memories[0].Filename != "memory-000.md" {
			t.Fatalf("memory_list payload = %#v, want oldest matching memory", payload)
		}
	})

	t.Run("Should bind native agent memory lists to the caller identity", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		identity, err := workspacepkg.EnsureIdentity(t.Context(), workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		memoryStore := memorypkg.NewStore(
			filepath.Join(baseDir, "global-memory"),
			memorypkg.WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		)
		openDaemonMemoryCatalog(t, memoryStore)
		base := memoryStore.ForWorkspace(workspaceRoot)
		for _, agentName := range []string{"reviewer-a", "reviewer-b"} {
			agentStore := base.ForAgent(identity.WorkspaceID, agentName, memcontract.AgentTierWorkspace)
			if err := agentStore.EnsureDirs(); err != nil {
				t.Fatalf("Store.EnsureDirs(%s) error = %v", agentName, err)
			}
			if err := agentStore.Write(
				memcontract.ScopeAgent,
				agentName+".md",
				nativeMemoryDocument(agentName, "agent memory", memcontract.TypeFeedback, "body"),
			); err != nil {
				t.Fatalf("Store.Write(%s) error = %v", agentName, err)
			}
		}
		workspaces := apitest.StubWorkspaceService{
			ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: identity.WorkspaceID, RootDir: workspaceRoot},
					WorkspaceID: identity.WorkspaceID,
				}, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MemoryStore: memoryStore,
			Workspaces:  workspaces,
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: identity.WorkspaceID, AgentName: "reviewer-a"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryList,
				Input:  json.RawMessage(`{"agent_tier":"workspace"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_list agent) error = %v", err)
		}
		var payload contract.MemoryListResponse
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("json.Unmarshal(memory_list agent) error = %v", err)
		}
		if payload.Page.Total != 1 || len(payload.Memories) != 1 {
			t.Fatalf("memory_list agent payload = %#v, want one reviewer-a memory", payload)
		}
		got := payload.Memories[0]
		if got.Filename != "reviewer-a.md" || got.WorkspaceID != identity.WorkspaceID ||
			got.AgentName != "reviewer-a" || got.AgentTier != memcontract.AgentTierWorkspace {
			t.Fatalf("memory_list agent item = %#v, want caller-bound reviewer-a identity", got)
		}
	})

	t.Run("Should dispatch Memory admin tools through operational Memory v2 services", func(t *testing.T) {
		t.Parallel()

		globalDir := filepath.Join(t.TempDir(), "memory")
		catalogPath := filepath.Join(t.TempDir(), "memory.db")
		memoryStore := memorypkg.NewStore(globalDir, memorypkg.WithCatalogDatabasePath(catalogPath))
		openDaemonMemoryCatalog(t, memoryStore)
		for idx := range 205 {
			filename := fmt.Sprintf("ops-%03d.md", idx)
			if err := memoryStore.Write(
				memcontract.ScopeGlobal,
				filename,
				nativeMemoryDocument(
					fmt.Sprintf("Ops %03d", idx),
					"Operational memory",
					memcontract.TypeUser,
					"memory admin health",
				),
			); err != nil {
				t.Fatalf("Write(%q) error = %v", filename, err)
			}
		}
		cfg := aghconfig.Config{}
		cfg.Memory.Enabled = true
		cfg.Memory.GlobalDir = globalDir
		cfg.Memory.Dream.CheckInterval = time.Hour
		extractor := &nativeMemoryExtractorService{
			status: contract.MemoryExtractorStatusPayload{
				Status:                 contract.MemoryExtractorStateIdle,
				QueuedSessions:         2,
				ActiveProviderSessions: 1,
				SkippedTurns:           5,
				BackpressuredSessions:  6,
			},
		}
		providers := &nativeMemoryProviderService{
			provider: contract.MemoryProviderPayload{
				Name:   "builtin",
				Status: contract.MemoryProviderStateActive,
				Active: true,
				Tools:  []string{toolspkg.ToolIDMemoryPropose.String()},
			},
		}
		ledger := &nativeMemorySessionLedgerService{
			response: contract.MemorySessionLedgerResponse{
				Meta: contract.MemorySessionLedgerMetaPayload{
					Version:   1,
					SessionID: "sess-memory",
					Path:      filepath.Join(t.TempDir(), "sess-memory.jsonl"),
					Checksum:  "sha256:test",
					CreatedAt: time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
				},
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Config:              cfg,
			MemoryStore:         memoryStore,
			MemoryExtractor:     extractor,
			MemoryProviders:     providers,
			MemorySessionLedger: ledger,
			Sessions:            nativeNetworkTestSessionManager("ws-1"),
			Workspaces:          nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		healthResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDMemoryHealth},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_health) error = %v", err)
		}
		var healthPayload contract.MemoryHealthPayload
		if err := json.Unmarshal(healthResult.Structured, &healthPayload); err != nil {
			t.Fatalf("json.Unmarshal(memory_health) error = %v", err)
		}
		if !healthPayload.Enabled || healthPayload.GlobalFiles != 205 {
			t.Fatalf("memory_health payload = %#v, want enabled with 205 global files", healthPayload)
		}

		extractorResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDMemoryExtractorStatus},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_extractor_status) error = %v", err)
		}
		requireNativeStructuredContains(t, extractorResult, []byte(`"status":"idle"`))
		requireNativeStructuredContains(t, extractorResult, []byte(`"active_provider_sessions":1`))
		requireNativeStructuredContains(t, extractorResult, []byte(`"skipped_turns":5`))
		requireNativeStructuredContains(t, extractorResult, []byte(`"backpressured_sessions":6`))
		if extractor.statusCalls != 1 {
			t.Fatalf("extractor Status calls = %d, want 1", extractor.statusCalls)
		}

		providerResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryProviderList,
				Input:  json.RawMessage(`{"workspace_id":"ws-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_provider_list) error = %v", err)
		}
		requireNativeStructuredContains(t, providerResult, []byte(`"name":"builtin"`))
		if providers.listCalls != 1 || providers.lastWorkspaceID != "ws-1" {
			t.Fatalf("provider list workspace = %q after %d calls", providers.lastWorkspaceID, providers.listCalls)
		}

		ledgerResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemorySessionLedger,
				Input:  json.RawMessage(`{"workspace_id":"ws-1","session_id":"sess-memory"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(memory_session_ledger) error = %v", err)
		}
		requireNativeStructuredContains(t, ledgerResult, []byte(`"session_id":"sess-memory"`))
		if ledger.getCalls != 1 || ledger.lastSessionID != "sess-memory" {
			t.Fatalf("session ledger id = %q after %d calls", ledger.lastSessionID, ledger.getCalls)
		}

		foreignLedger := &nativeMemorySessionLedgerService{}
		foreignRegistry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MemorySessionLedger: foreignLedger,
			Sessions: apitest.StubSessionManager{
				StatusFn: func(_ context.Context, id string) (*session.Info, error) {
					return &session.Info{ID: strings.TrimSpace(id), WorkspaceID: "ws-other"}, nil
				},
			},
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())
		for _, tc := range []struct {
			name  string
			id    toolspkg.ToolID
			input json.RawMessage
		}{
			{
				name:  "session ledger",
				id:    toolspkg.ToolIDMemorySessionLedger,
				input: json.RawMessage(`{"workspace_id":"ws-1","session_id":"sess-memory"}`),
			},
			{
				name:  "session replay",
				id:    toolspkg.ToolIDMemorySessionReplay,
				input: json.RawMessage(`{"workspace_id":"ws-1","session_id":"sess-memory"}`),
			},
		} {
			_, err := foreignRegistry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{ToolID: tc.id, Input: tc.input},
			)
			if err == nil {
				t.Fatalf("Registry.Call(%s foreign session) error = nil, want non-nil", tc.name)
			}
		}
		if foreignLedger.totalCalls() != 0 {
			t.Fatalf("foreign memory ledger calls = %d, want 0", foreignLedger.totalCalls())
		}

		_, err = registry.Get(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.ToolIDMemoryAdminHistory)
		if err != nil {
			t.Fatalf("Registry.Get(memory_admin_history) error = %v", err)
		}
		_, err = registry.Get(t.Context(), toolspkg.Scope{Operator: true}, "agh__memory_history")
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("Registry.Get(memory_history legacy) error = %v, want ErrToolNotFound", err)
		}
	})

	t.Run("Should cover Memory admin native groups and destructive reset guards", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			id      toolspkg.ToolID
			scope   toolspkg.Scope
			input   func(nativeMemoryAdminFixture) json.RawMessage
			want    []byte
			wantFor func(nativeMemoryAdminFixture) []byte
			wantErr toolspkg.ErrorCode
			assert  func(*testing.T, nativeMemoryAdminFixture)
		}{
			{name: "health", id: toolspkg.ToolIDMemoryHealth, want: []byte(`"enabled":true`)},
			{
				name:  "scope show",
				id:    toolspkg.ToolIDMemoryScopeShow,
				input: staticNativeInput(`{"scope":"global"}`),
				want:  []byte(`"scope":"global"`),
			},
			{name: "history", id: toolspkg.ToolIDMemoryAdminHistory, want: []byte(`"operations"`)},
			{name: "reindex", id: toolspkg.ToolIDMemoryReindex, want: []byte(`"indexed_files"`)},
			{
				name: "promote",
				id:   toolspkg.ToolIDMemoryPromote,
				input: staticNativeInput(
					`{"filename":"ops.md","from":{"scope":"global"},"to":{"scope":"global"},` +
						`"idempotency_key":"promote-test","dry_run":true}`,
				),
				want: []byte(`"decision"`),
				assert: func(t *testing.T, fixture nativeMemoryAdminFixture) {
					t.Helper()
					records, err := fixture.memoryStore.ListDecisionRecords(t.Context(), memorypkg.DecisionListQuery{})
					if err != nil {
						t.Fatalf("ListDecisionRecords() error = %v", err)
					}
					if len(records) != 2 {
						t.Fatalf("decision record count = %d, want existing fixture decisions only", len(records))
					}
				},
			},
			{
				name:  "reset unconfirmed guard",
				id:    toolspkg.ToolIDMemoryReset,
				input: staticNativeInput(`{"derived_only":true,"confirm":false}`),
				want:  []byte(`"deleted_rows":0`),
			},
			{
				name:  "reset derived",
				id:    toolspkg.ToolIDMemoryReset,
				input: staticNativeInput(`{"derived_only":true,"confirm":true}`),
				want:  []byte(`"derived_only":true`),
			},
			{name: "reload", id: toolspkg.ToolIDMemoryReload, want: []byte(`"generation"`)},
			{
				name: "decisions list filters by target filename before limit",
				id:   toolspkg.ToolIDMemoryDecisionsList,
				input: func(fixture nativeMemoryAdminFixture) json.RawMessage {
					return json.RawMessage(fmt.Sprintf(
						`{"filename":%q,"limit":1}`,
						fixture.decision.TargetFilename,
					))
				},
				wantFor: func(fixture nativeMemoryAdminFixture) []byte {
					return fmt.Appendf(nil, `"id":%q`, fixture.decision.ID)
				},
			},
			{
				name: "decisions show",
				id:   toolspkg.ToolIDMemoryDecisionsShow,
				input: func(fixture nativeMemoryAdminFixture) json.RawMessage {
					return json.RawMessage(fmt.Sprintf(`{"decision_id":%q}`, fixture.decision.ID))
				},
				want: []byte(`"decision"`),
			},
			{
				name:  "decisions show denies out-of-scope agent decision",
				id:    toolspkg.ToolIDMemoryDecisionsShow,
				scope: toolspkg.Scope{SessionID: "sess-reviewer", AgentName: "reviewer"},
				input: func(fixture nativeMemoryAdminFixture) json.RawMessage {
					return json.RawMessage(fmt.Sprintf(`{"decision_id":%q}`, fixture.agentDecision.ID))
				},
				wantErr: toolspkg.ErrorCodeDenied,
			},
			{
				name: "decisions revert dry run",
				id:   toolspkg.ToolIDMemoryDecisionsRevert,
				input: func(fixture nativeMemoryAdminFixture) json.RawMessage {
					return json.RawMessage(fmt.Sprintf(
						`{"decision_id":%q,"reason":"verify native revert","dry_run":true}`,
						fixture.decision.ID,
					))
				},
				want: []byte(`"dry_run":true`),
			},
			{
				name:  "decisions revert dry run denies out-of-scope agent decision",
				id:    toolspkg.ToolIDMemoryDecisionsRevert,
				scope: toolspkg.Scope{SessionID: "sess-reviewer", AgentName: "reviewer"},
				input: func(fixture nativeMemoryAdminFixture) json.RawMessage {
					return json.RawMessage(fmt.Sprintf(
						`{"decision_id":%q,"reason":"verify native revert","dry_run":true}`,
						fixture.agentDecision.ID,
					))
				},
				wantErr: toolspkg.ErrorCodeDenied,
			},
			{
				name:    "recall trace not materialized",
				id:      toolspkg.ToolIDMemoryRecallTrace,
				input:   staticNativeInput(`{"session_id":"sess-memory","turn_seq":1}`),
				wantErr: toolspkg.ErrorCodeNotFound,
			},
			{name: "dream status", id: toolspkg.ToolIDMemoryDreamStatus, want: []byte(`"dreams":[]`)},
			{name: "dream list", id: toolspkg.ToolIDMemoryDreamList, want: []byte(`"dreams"`)},
			{
				name:    "dream show missing",
				id:      toolspkg.ToolIDMemoryDreamShow,
				input:   staticNativeInput(`{"dream_id":"dream-missing"}`),
				wantErr: toolspkg.ErrorCodeNotFound,
			},
			{
				name:  "dream trigger",
				id:    toolspkg.ToolIDMemoryDreamTrigger,
				input: staticNativeInput(`{"scope":"workspace","workspace_id":"ws-1","force":true}`),
				want:  []byte(`"triggered":true`),
				assert: func(t *testing.T, fixture nativeMemoryAdminFixture) {
					t.Helper()
					if fixture.dream.triggerCalls != 1 || fixture.dream.lastWorkspace != "ws-1" {
						t.Fatalf("dream trigger = %#v, want one ws-1 call", fixture.dream)
					}
				},
			},
			{
				name:  "dream retry",
				id:    toolspkg.ToolIDMemoryDreamRetry,
				input: staticNativeInput(`{"failure_id":"dream-failure","force":true}`),
				want:  []byte(`"retried":true`),
				assert: func(t *testing.T, fixture nativeMemoryAdminFixture) {
					t.Helper()
					if fixture.dream.triggerCalls != 1 || fixture.dream.lastWorkspace != "dream-failure" {
						t.Fatalf("dream retry trigger = %#v, want one dream-failure call", fixture.dream)
					}
				},
			},
			{
				name:    "dream retry rejects missing target",
				id:      toolspkg.ToolIDMemoryDreamRetry,
				input:   staticNativeInput(`{"force":true}`),
				wantErr: toolspkg.ErrorCodeInvalidInput,
			},
			{
				name:    "dream retry rejects conflicting targets",
				id:      toolspkg.ToolIDMemoryDreamRetry,
				input:   staticNativeInput(`{"failure_id":"dream-failure","dream_id":"dream-run","force":true}`),
				wantErr: toolspkg.ErrorCodeInvalidInput,
			},
			{name: "daily list", id: toolspkg.ToolIDMemoryDailyList, want: []byte(`"logs"`)},
			{name: "extractor status", id: toolspkg.ToolIDMemoryExtractorStatus, want: []byte(`"status":"idle"`)},
			{name: "extractor failures", id: toolspkg.ToolIDMemoryExtractorFailures, want: []byte(`"failure-native"`)},
			{
				name:  "extractor retry",
				id:    toolspkg.ToolIDMemoryExtractorRetry,
				input: staticNativeInput(`{"failure_id":"failure-native"}`),
				want:  []byte(`"retried":1`),
				assert: func(t *testing.T, fixture nativeMemoryAdminFixture) {
					t.Helper()
					if fixture.extractor.lastRetry.FailureID != "failure-native" {
						t.Fatalf("extractor retry = %#v, want failure-native", fixture.extractor.lastRetry)
					}
				},
			},
			{name: "extractor drain", id: toolspkg.ToolIDMemoryExtractorDrain, want: []byte(`"remaining":0`)},
			{
				name:  "provider list",
				id:    toolspkg.ToolIDMemoryProviderList,
				input: staticNativeInput(`{"workspace_id":"ws-1"}`),
				want:  []byte(`"name":"builtin"`),
			},
			{
				name:  "provider get",
				id:    toolspkg.ToolIDMemoryProviderGet,
				input: staticNativeInput(`{"workspace_id":"ws-1","name":"builtin"}`),
				want:  []byte(`"name":"builtin"`),
			},
			{
				name:  "provider select",
				id:    toolspkg.ToolIDMemoryProviderSelect,
				input: staticNativeInput(`{"workspace_id":"ws-1","name":"builtin"}`),
				want:  []byte(`"active":true`),
			},
			{
				name:  "provider enable",
				id:    toolspkg.ToolIDMemoryProviderEnable,
				input: staticNativeInput(`{"workspace_id":"ws-1","name":"builtin","reason":"test"}`),
				want:  []byte(`"changed":true`),
			},
			{
				name:  "provider disable",
				id:    toolspkg.ToolIDMemoryProviderDisable,
				input: staticNativeInput(`{"workspace_id":"ws-1","name":"builtin","reason":"test"}`),
				want:  []byte(`"changed":true`),
			},
			{
				name:  "session ledger",
				id:    toolspkg.ToolIDMemorySessionLedger,
				input: staticNativeInput(`{"workspace_id":"ws-1","session_id":"sess-memory"}`),
				want:  []byte(`"session_id":"sess-memory"`),
			},
			{
				name: "session replay",
				id:   toolspkg.ToolIDMemorySessionReplay,
				input: staticNativeInput(
					`{"workspace_id":"ws-1","session_id":"sess-memory","include_tool_events":true,"include_memory":true}`,
				),
				want: []byte(`"session_id":"sess-memory"`),
			},
			{
				name:  "sessions prune",
				id:    toolspkg.ToolIDMemorySessionsPrune,
				input: staticNativeInput(`{"older_than_hours":24,"dry_run":true}`),
				want:  []byte(`"dry_run":true`),
			},
			{name: "sessions repair", id: toolspkg.ToolIDMemorySessionsRepair, want: []byte(`"repaired_ledgers":1`)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				fixture := newNativeMemoryAdminFixture(t)
				var input json.RawMessage
				if tc.input != nil {
					input = tc.input(fixture)
				}
				result, err := fixture.registry.Call(
					t.Context(),
					tc.scope,
					toolspkg.CallRequest{ToolID: tc.id, Input: input},
				)
				if tc.wantErr != "" {
					requireToolCode(t, err, tc.wantErr)
					return
				}
				if err != nil {
					t.Fatalf("Registry.Call(%s) error = %v", tc.id, err)
				}
				want := tc.want
				if tc.wantFor != nil {
					want = tc.wantFor(fixture)
				}
				requireNativeStructuredContains(t, result, want)
				if tc.assert != nil {
					tc.assert(t, fixture)
				}
			})
		}
	})

	t.Run(
		"Should manage task notification subscriptions through native task and bridge boundaries",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC)
			tasks := &nativeTaskManager{
				getView: &taskpkg.View{Task: taskpkg.Task{
					ID:          "task-1",
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
					Title:       "Notify bridge",
					Status:      taskpkg.TaskStatusInProgress,
				}},
			}
			var (
				putSubscription bridgepkg.BridgeTaskSubscription
				listQuery       bridgepkg.BridgeTaskSubscriptionQuery
				deleteID        string
				deleted         bool
			)
			bridges := apitest.StubBridgeService{
				GetInstanceFn: func(_ context.Context, id string) (*bridgepkg.BridgeInstance, error) {
					if id != "bridge-1" {
						t.Fatalf("GetInstance id = %q, want bridge-1", id)
					}
					return &bridgepkg.BridgeInstance{
						ID:          "bridge-1",
						Scope:       bridgepkg.ScopeWorkspace,
						WorkspaceID: "ws-1",
					}, nil
				},
				PutTaskSubscriptionFn: func(_ context.Context, subscription bridgepkg.BridgeTaskSubscription) error {
					putSubscription = subscription
					return nil
				},
				GetTaskSubscriptionFn: func(_ context.Context, id string) (bridgepkg.BridgeTaskSubscription, error) {
					if id != putSubscription.SubscriptionID {
						t.Fatalf("GetBridgeTaskSubscription id = %q, want %q", id, putSubscription.SubscriptionID)
					}
					if deleted {
						return bridgepkg.BridgeTaskSubscription{}, bridgepkg.ErrBridgeTaskSubscriptionNotFound
					}
					stored := putSubscription
					if stored.UpdatedAt.IsZero() {
						stored.UpdatedAt = now
					}
					return stored, nil
				},
				ListTaskSubscriptionsFn: func(
					_ context.Context,
					query bridgepkg.BridgeTaskSubscriptionQuery,
				) ([]bridgepkg.BridgeTaskSubscription, error) {
					listQuery = query
					return []bridgepkg.BridgeTaskSubscription{putSubscription}, nil
				},
				DeleteTaskSubscriptionFn: func(_ context.Context, id string) error {
					deleteID = id
					deleted = true
					return nil
				},
				GetCursorFn: func(_ context.Context, key notifications.CursorKey) (notifications.Cursor, error) {
					if key.ConsumerID != "bridge_task_subscription:sub-native" ||
						key.StreamName != "task_events" ||
						key.SubjectID != "task-1" {
						t.Fatalf("GetCursor key = %#v, want native subscription cursor", key)
					}
					return notifications.Cursor{
						Key:             key,
						LastSequence:    11,
						LastDeliveryID:  "delivery-11",
						LastDeliveredAt: now,
						UpdatedAt:       now,
					}, nil
				},
			}
			registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
				Tasks:   tasks,
				Bridges: bridges,
			}, nativeApproveAllPolicyInputs())

			subscribeResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDTaskNotificationSubscribe,
					Input: json.RawMessage(
						`{"task_id":"task-1","subscription_id":"sub-native","bridge_instance_id":"bridge-1",` +
							`"scope":"workspace","workspace_id":"ws-1","peer_id":"peer-1","thread_id":"thread-1",` +
							`"delivery_mode":"reply"}`,
					),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(task_notification_subscribe) error = %v", err)
			}
			requireNativeStructuredContains(t, subscribeResult, []byte(`"subscription_id":"sub-native"`))
			requireNativeStructuredContains(t, subscribeResult, []byte(`"last_sequence":11`))
			if putSubscription.SubscriptionID != "sub-native" ||
				putSubscription.TaskID != "task-1" ||
				putSubscription.BridgeInstanceID != "bridge-1" ||
				putSubscription.Scope != bridgepkg.ScopeWorkspace ||
				putSubscription.WorkspaceID != "ws-1" ||
				putSubscription.DeliveryMode != bridgepkg.DeliveryModeReply {
				t.Fatalf("put subscription = %#v", putSubscription)
			}

			listResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDTaskNotificationList,
					Input: json.RawMessage(
						`{"task_id":"task-1","bridge_instance_id":"bridge-1","scope":"workspace","workspace_id":"ws-1","limit":3}`,
					),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(task_notification_list) error = %v", err)
			}
			requireNativeStructuredContains(t, listResult, []byte(`"subscription_id":"sub-native"`))
			if listQuery.TaskID != "task-1" ||
				listQuery.BridgeInstanceID != "bridge-1" ||
				listQuery.Scope != bridgepkg.ScopeWorkspace ||
				listQuery.WorkspaceID != "ws-1" ||
				listQuery.Limit != 3 {
				t.Fatalf("list query = %#v", listQuery)
			}

			showResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDTaskNotificationShow,
					Input:  json.RawMessage(`{"task_id":"task-1","subscription_id":"sub-native"}`),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(task_notification_show) error = %v", err)
			}
			requireNativeStructuredContains(t, showResult, []byte(`"last_delivery_id":"delivery-11"`))

			deleteResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDTaskNotificationDelete,
					Input:  json.RawMessage(`{"task_id":"task-1","subscription_id":"sub-native"}`),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(task_notification_delete) error = %v", err)
			}
			requireNativeStructuredContains(t, deleteResult, []byte(`"deleted":true`))
			if deleteID != "sub-native" {
				t.Fatalf("delete id = %q, want sub-native", deleteID)
			}
		},
	)

	t.Run("Should keep task notification subscribe successful when cursor enrichment fails", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 12, 11, 30, 0, 0, time.UTC)
		tasks := &nativeTaskManager{
			getView: &taskpkg.View{Task: taskpkg.Task{
				ID:          "task-1",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-1",
				Title:       "Notify bridge",
				Status:      taskpkg.TaskStatusInProgress,
			}},
		}
		var putSubscription bridgepkg.BridgeTaskSubscription
		bridges := apitest.StubBridgeService{
			GetInstanceFn: func(_ context.Context, id string) (*bridgepkg.BridgeInstance, error) {
				if id != "bridge-1" {
					t.Fatalf("GetInstance id = %q, want bridge-1", id)
				}
				return &bridgepkg.BridgeInstance{
					ID:          "bridge-1",
					Scope:       bridgepkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
				}, nil
			},
			PutTaskSubscriptionFn: func(_ context.Context, subscription bridgepkg.BridgeTaskSubscription) error {
				putSubscription = subscription
				return nil
			},
			GetTaskSubscriptionFn: func(_ context.Context, id string) (bridgepkg.BridgeTaskSubscription, error) {
				if id != putSubscription.SubscriptionID {
					t.Fatalf("GetBridgeTaskSubscription id = %q, want %q", id, putSubscription.SubscriptionID)
				}
				stored := putSubscription
				stored.UpdatedAt = now
				return stored, nil
			},
			GetCursorFn: func(context.Context, notifications.CursorKey) (notifications.Cursor, error) {
				return notifications.Cursor{}, errors.New("cursor backend unavailable")
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Tasks:   tasks,
			Bridges: bridges,
		}, nativeApproveAllPolicyInputs())

		subscribeResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDTaskNotificationSubscribe,
				Input: json.RawMessage(
					`{"task_id":"task-1","subscription_id":"sub-native","bridge_instance_id":"bridge-1",` +
						`"scope":"workspace","workspace_id":"ws-1","peer_id":"peer-1","thread_id":"thread-1",` +
						`"delivery_mode":"reply"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(task_notification_subscribe) error = %v", err)
		}
		requireNativeStructuredContains(t, subscribeResult, []byte(`"subscription_id":"sub-native"`))
		requireNativeStructuredContains(
			t,
			subscribeResult,
			[]byte(`"consumer_id":"bridge_task_subscription:sub-native"`),
		)
		if putSubscription.SubscriptionID != "sub-native" {
			t.Fatalf("put subscription id = %q, want sub-native", putSubscription.SubscriptionID)
		}
	})

	t.Run("Should reject task notification invalid input and bridge service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			id      toolspkg.ToolID
			input   json.RawMessage
			bridges apitest.StubBridgeService
			want    toolspkg.ErrorCode
		}{
			{
				name: "subscribe missing delivery target",
				id:   toolspkg.ToolIDTaskNotificationSubscribe,
				input: json.RawMessage(
					`{"task_id":"task-1","subscription_id":"sub-1","bridge_instance_id":"bridge-1",` +
						`"scope":"workspace","workspace_id":"ws-1","delivery_mode":"reply"}`,
				),
				want: toolspkg.ErrorCodeInvalidInput,
			},
			{
				name: "subscribe scope mismatch",
				id:   toolspkg.ToolIDTaskNotificationSubscribe,
				input: json.RawMessage(
					`{"task_id":"task-1","subscription_id":"sub-1","bridge_instance_id":"bridge-1",` +
						`"scope":"global","peer_id":"peer-1","delivery_mode":"reply"}`,
				),
				want: toolspkg.ErrorCodeInvalidInput,
			},
			{
				name: "subscribe missing bridge instance",
				id:   toolspkg.ToolIDTaskNotificationSubscribe,
				input: json.RawMessage(
					`{"task_id":"task-1","subscription_id":"sub-1","bridge_instance_id":"missing",` +
						`"scope":"workspace","workspace_id":"ws-1","peer_id":"peer-1","delivery_mode":"reply"}`,
				),
				bridges: apitest.StubBridgeService{},
				want:    toolspkg.ErrorCodeNotFound,
			},
			{
				name:  "list invalid scope",
				id:    toolspkg.ToolIDTaskNotificationList,
				input: json.RawMessage(`{"task_id":"task-1","scope":"invalid"}`),
				want:  toolspkg.ErrorCodeInvalidInput,
			},
			{
				name:  "list backend failure",
				id:    toolspkg.ToolIDTaskNotificationList,
				input: json.RawMessage(`{"task_id":"task-1"}`),
				bridges: apitest.StubBridgeService{
					ListTaskSubscriptionsFn: func(
						context.Context,
						bridgepkg.BridgeTaskSubscriptionQuery,
					) ([]bridgepkg.BridgeTaskSubscription, error) {
						return nil, errors.New("list subscriptions failed")
					},
				},
				want: toolspkg.ErrorCodeBackendFailed,
			},
			{
				name:  "show missing subscription id",
				id:    toolspkg.ToolIDTaskNotificationShow,
				input: json.RawMessage(`{"task_id":"task-1"}`),
				want:  toolspkg.ErrorCodeInvalidInput,
			},
			{
				name:  "show missing subscription",
				id:    toolspkg.ToolIDTaskNotificationShow,
				input: json.RawMessage(`{"task_id":"task-1","subscription_id":"missing"}`),
				bridges: apitest.StubBridgeService{
					GetTaskSubscriptionFn: func(context.Context, string) (bridgepkg.BridgeTaskSubscription, error) {
						return bridgepkg.BridgeTaskSubscription{}, bridgepkg.ErrBridgeTaskSubscriptionNotFound
					},
				},
				want: toolspkg.ErrorCodeNotFound,
			},
			{
				name:  "delete missing subscription id",
				id:    toolspkg.ToolIDTaskNotificationDelete,
				input: json.RawMessage(`{"task_id":"task-1"}`),
				want:  toolspkg.ErrorCodeInvalidInput,
			},
			{
				name:  "delete missing subscription",
				id:    toolspkg.ToolIDTaskNotificationDelete,
				input: json.RawMessage(`{"task_id":"task-1","subscription_id":"missing"}`),
				bridges: apitest.StubBridgeService{
					GetTaskSubscriptionFn: func(context.Context, string) (bridgepkg.BridgeTaskSubscription, error) {
						return bridgepkg.BridgeTaskSubscription{}, bridgepkg.ErrBridgeTaskSubscriptionNotFound
					},
				},
				want: toolspkg.ErrorCodeNotFound,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				bridges := tc.bridges
				tasks := &nativeTaskManager{
					getView: &taskpkg.View{Task: taskpkg.Task{
						ID:          "task-1",
						Scope:       taskpkg.ScopeWorkspace,
						WorkspaceID: "ws-1",
						Title:       "Notify bridge",
						Status:      taskpkg.TaskStatusInProgress,
					}},
				}
				registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
					Tasks:   tasks,
					Bridges: bridges,
				}, nativeApproveAllPolicyInputs())

				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{},
					toolspkg.CallRequest{ToolID: tc.id, Input: tc.input},
				)
				requireToolCode(t, err, tc.want)
			})
		}
	})

	t.Run("Should deny subagent memory writes and mark root tool writes", func(t *testing.T) {
		t.Parallel()

		globalDir := filepath.Join(t.TempDir(), "global-memory")
		memoryStore := memorypkg.NewStore(
			globalDir,
			memorypkg.WithCatalogDatabasePath(filepath.Join(t.TempDir(), "agh.db")),
		)
		openDaemonMemoryCatalog(t, memoryStore)
		recorder := &nativeMemoryToolWriteRecorder{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			MemoryStore:      memoryStore,
			MemoryToolWrites: recorder,
		}, nativeApproveAllPolicyInputs())

		rootResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-root", ActorKind: " AGENT_ROOT "},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryPropose,
				Input: json.RawMessage(
					`{"filename":"root_tool.md","type":"user","content":"Root memory writes are allowed."}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(root memory_propose) error = %v", err)
		}
		requireNativeStructuredContains(t, rootResult, []byte(`"applied":true`))
		if recorder.sessionID != "sess-root" || recorder.calls != 1 {
			t.Fatalf("tool write recorder = %#v, want sess-root once", recorder)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-child", ActorKind: " Agent_SubAgent "},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryPropose,
				Input: json.RawMessage(
					`{"filename":"child_tool.md","type":"user","content":"Subagent memory writes are denied."}`,
				),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolDenied) {
			t.Fatalf("Registry.Call(subagent memory_propose) error = %v, want ErrToolDenied", err)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-child", ActorKind: "AGENT_SUBAGENT"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDMemoryNote,
				Input:  json.RawMessage(`{"content":"Subagent notes are denied."}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolDenied) {
			t.Fatalf("Registry.Call(subagent memory_note) error = %v, want ErrToolDenied", err)
		}

		events, err := memoryStore.ListMemoryEventSummaries(
			t.Context(),
			nil,
			store.EventSummaryQuery{Type: "memory.write.rejected"},
		)
		if err != nil {
			t.Fatalf("ListMemoryEventSummaries(write rejected) error = %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("write rejected events = %#v, want two denied writes", events)
		}
	})

	t.Run("Should read observe tools through the observer without leaking event secrets", func(t *testing.T) {
		t.Parallel()

		rawClaim := "agh_claim_observe123"
		now := time.Date(2026, 4, 29, 15, 0, 0, 0, time.UTC)
		observer := &nativeObserverStub{
			eventSummaries: []store.EventSummary{
				{
					ID:          "evt-1",
					WorkspaceID: "ws-native-network",
					SessionID:   "sess-1",
					Type:        "agent_message",
					AgentName:   "coder",
					Summary:     "deploy completed " + rawClaim,
					Timestamp:   now,
				},
				{
					ID:          "evt-2",
					WorkspaceID: "ws-native-network",
					SessionID:   "sess-2",
					Type:        "agent_message",
					AgentName:   "reviewer",
					Summary:     "review completed",
					Timestamp:   now.Add(time.Second),
				},
			},
			health: observe.Health{
				Status:         "ok",
				ActiveSessions: 1,
				ActiveAgents:   1,
				Retention: observe.RetentionHealth{
					LastSweepError: "sweep failed " + rawClaim,
				},
				Version: "test",
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Observer:   observer,
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		eventsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDListLogs,
				Input:  json.RawMessage(`{"workspace_id":"ws-native-network","limit":1}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(logs) error = %v", err)
		}
		requireNativeStructuredContains(t, eventsResult, []byte(`"evt-1"`))
		requireNativeStructuredExcludes(t, eventsResult, []byte(rawClaim))

		filteredEventsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDListLogs,
				Input: json.RawMessage(
					`{"workspace_id":"ws-native-network","session_id":"sess-2","since":"2026-04-29T15:00:00Z"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(logs filtered) error = %v", err)
		}
		requireNativeStructuredContains(t, filteredEventsResult, []byte(`"evt-2"`))
		requireNativeStructuredExcludes(t, filteredEventsResult, []byte(`"evt-1"`))

		searchResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDObserveSearch,
				Input:  json.RawMessage(`{"workspace_id":"ws-native-network","query":"deploy","limit":10}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(observe_search) error = %v", err)
		}
		requireNativeStructuredContains(t, searchResult, []byte(`"evt-1"`))
		requireNativeStructuredExcludes(t, searchResult, []byte(`"evt-2"`))
		requireNativeStructuredExcludes(t, searchResult, []byte(rawClaim))

		metricsResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDObserveMetrics},
		)
		if err != nil {
			t.Fatalf("Registry.Call(observe_metrics) error = %v", err)
		}
		requireNativeStructuredContains(t, metricsResult, []byte(`"active_sessions":1`))
		requireNativeStructuredContains(t, metricsResult, []byte(`agh_claim_[REDACTED]`))
		requireNativeStructuredExcludes(t, metricsResult, []byte(rawClaim))

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDListLogs,
				Input:  json.RawMessage(`{"since":"not-a-date"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(logs invalid since) error = %v, want ErrToolInvalidInput", err)
		}
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDObserveSearch,
				Input:  json.RawMessage(`{"workspace_id":"ws-native-network","query":""}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) {
			t.Fatalf("Registry.Call(observe_search empty query) error = %v, want ErrToolInvalidInput", err)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-native-network", AgentName: "coder"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDListLogs,
				Input:  json.RawMessage(`{"workspace_id":"ws-other"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-native-network", AgentName: "coder"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDObserveSearch,
				Input:  json.RawMessage(`{"workspace_id":"ws-other","query":"deploy"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
	})

	t.Run(
		"Should read bridge tools through existing projections without leaking credential material",
		func(t *testing.T) {
			t.Parallel()

			rawClaim := "agh_claim_bridge123"
			now := time.Date(2026, 4, 29, 16, 0, 0, 0, time.UTC)
			degradation := &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: "refresh failed " + rawClaim,
			}
			instance := bridgepkg.BridgeInstance{
				ID:             "bridge-1",
				Scope:          bridgepkg.ScopeGlobal,
				Platform:       "slack",
				ExtensionName:  "slack-ext",
				DisplayName:    "Slack",
				Source:         bridgepkg.BridgeInstanceSourceDynamic,
				Enabled:        true,
				Status:         bridgepkg.BridgeStatusReady,
				DMPolicy:       bridgepkg.BridgeDMPolicyOpen,
				RoutingPolicy:  bridgepkg.RoutingPolicy{IncludePeer: true},
				ProviderConfig: json.RawMessage(`{"bot_token":"secret-value"}`),
				Degradation:    degradation,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			observer := &nativeObserverStub{
				bridgeHealth: []observe.BridgeInstanceHealth{{
					BridgeInstanceID: "bridge-1",
					Status:           bridgepkg.BridgeStatusReady,
					RouteCount:       2,
					LastError:        "provider returned " + rawClaim,
				}},
			}
			bridges := apitest.StubBridgeService{
				ListInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
					return []bridgepkg.BridgeInstance{instance}, nil
				},
				GetInstanceFn: func(_ context.Context, id string) (*bridgepkg.BridgeInstance, error) {
					if id != "bridge-1" {
						return nil, bridgepkg.ErrBridgeInstanceNotFound
					}
					next := instance
					return &next, nil
				},
			}
			registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
				Bridges:  bridges,
				Observer: observer,
			}, nativeApproveAllPolicyInputs())

			listResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{ToolID: toolspkg.ToolIDBridgesList},
			)
			if err != nil {
				t.Fatalf("Registry.Call(bridges_list) error = %v", err)
			}
			requireNativeStructuredContains(t, listResult, []byte(`"bridge-1"`))
			requireNativeStructuredContains(t, listResult, []byte(`"route_count":2`))
			requireNativeStructuredContains(t, listResult, []byte(`agh_claim_[REDACTED]`))
			requireNativeStructuredExcludes(t, listResult, []byte(`bot_token`))
			requireNativeStructuredExcludes(t, listResult, []byte(`secret-value`))
			requireNativeStructuredExcludes(t, listResult, []byte(rawClaim))

			statusResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDBridgesStatus,
					Input:  json.RawMessage(`{"bridge_id":"bridge-1"}`),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(bridges_status) error = %v", err)
			}
			requireNativeStructuredContains(t, statusResult, []byte(`"bridge-1"`))
			requireNativeStructuredExcludes(t, statusResult, []byte(`bot_token`))
			requireNativeStructuredExcludes(t, statusResult, []byte(`secret-value`))
			requireNativeStructuredExcludes(t, statusResult, []byte(rawClaim))

			aggregateStatusResult, err := registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{ToolID: toolspkg.ToolIDBridgesStatus},
			)
			if err != nil {
				t.Fatalf("Registry.Call(bridges_status aggregate) error = %v", err)
			}
			requireNativeStructuredContains(t, aggregateStatusResult, []byte(`"status_counts":{"ready":1}`))
			requireNativeStructuredExcludes(t, aggregateStatusResult, []byte(`bot_token`))
			requireNativeStructuredExcludes(t, aggregateStatusResult, []byte(rawClaim))

			_, err = registry.Call(
				t.Context(),
				toolspkg.Scope{},
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDBridgesStatus,
					Input:  json.RawMessage(`{"bridge_id":"missing"}`),
				},
			)
			if !errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound) {
				t.Fatalf("Registry.Call(bridges_status missing) error = %v, want ErrBridgeInstanceNotFound", err)
			}
		},
	)

	t.Run("Should page bridge catalogs within the caller workspace authority", func(t *testing.T) {
		t.Parallel()

		instances := make([]bridgepkg.BridgeInstance, 0, 58)
		for index := range 55 {
			instances = append(instances, bridgepkg.BridgeInstance{
				ID:          fmt.Sprintf("bridge-prefix-%03d", index),
				Scope:       bridgepkg.ScopeGlobal,
				Platform:    "telegram",
				DisplayName: fmt.Sprintf("Prefix %03d", index),
				Enabled:     true,
				Status:      bridgepkg.BridgeStatusReady,
			})
		}
		instances = append(
			instances,
			bridgepkg.BridgeInstance{
				ID: "bridge-needle-global", Scope: bridgepkg.ScopeGlobal, Platform: "slack",
				DisplayName: "Needle A", Enabled: true, Status: bridgepkg.BridgeStatusReady,
			},
			bridgepkg.BridgeInstance{
				ID: "bridge-needle-own", Scope: bridgepkg.ScopeWorkspace, WorkspaceID: "ws-1",
				Platform: "slack", DisplayName: "Needle B", Enabled: true, Status: bridgepkg.BridgeStatusReady,
			},
			bridgepkg.BridgeInstance{
				ID: "bridge-needle-other", Scope: bridgepkg.ScopeWorkspace, WorkspaceID: "ws-2",
				Platform: "slack", DisplayName: "Needle C", Enabled: true, Status: bridgepkg.BridgeStatusReady,
			},
		)
		instancesByID := make(map[string]bridgepkg.BridgeInstance, len(instances))
		records := make([]bridgepkg.BridgeCatalogRecord, 0, len(instances))
		for _, instance := range instances {
			instancesByID[instance.ID] = instance
			records = append(records, bridgepkg.BridgeCatalogRecordFromInstance(instance))
		}
		catalogCalls := 0
		hydrationCalls := 0
		var hydratedIDs []string
		observer := &nativeObserverStub{bridgeHealth: []observe.BridgeInstanceHealth{
			{BridgeInstanceID: "bridge-needle-global", Status: bridgepkg.BridgeStatusReady, RouteCount: 1},
			{BridgeInstanceID: "bridge-needle-own", Status: bridgepkg.BridgeStatusReady, RouteCount: 2},
			{BridgeInstanceID: "bridge-needle-other", Status: bridgepkg.BridgeStatusReady, RouteCount: 3},
		}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Bridges: apitest.StubBridgeService{
				ListInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
					t.Fatal("ListInstances() must not hydrate the full native bridge catalog")
					return nil, nil
				},
				ListCatalogRecordsFn: func(
					_ context.Context,
					query bridgepkg.BridgeCatalogQuery,
				) ([]bridgepkg.BridgeCatalogRecord, error) {
					catalogCalls++
					if query.Scope != "all" || query.WorkspaceID != "ws-1" || query.Platform != "slack" {
						t.Fatalf("ListCatalogRecords() query = %#v, want caller visibility and platform", query)
					}
					return records, nil
				},
				ListInstancesByIDsFn: func(
					_ context.Context,
					ids []string,
				) ([]bridgepkg.BridgeInstance, error) {
					hydrationCalls++
					hydratedIDs = append([]string(nil), ids...)
					result := make([]bridgepkg.BridgeInstance, 0, len(ids))
					for _, id := range ids {
						result = append(result, instancesByID[id])
					}
					return result, nil
				},
			},
			Observer: observer,
		}, nativeApproveAllPolicyInputs())

		first, err := registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBridgesList,
				Input:  json.RawMessage(`{"q":"needle","platform":"SLACK","sort":"name","limit":1}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(first bridges_list page) error = %v", err)
		}
		var firstPage nativeBridgeCatalogResponse
		if err := json.Unmarshal(first.Structured, &firstPage); err != nil {
			t.Fatalf("json.Unmarshal(first bridges_list page) error = %v", err)
		}
		if got, want := firstPage.Page.Total, 2; got != want {
			t.Fatalf("first page total = %d, want %d", got, want)
		}
		if len(firstPage.Bridges) != 1 || firstPage.Bridges[0].ID != "bridge-needle-global" ||
			!firstPage.Page.HasMore || firstPage.Page.NextCursor == "" {
			t.Fatalf("first page = %#v, want global-first continuation", firstPage)
		}
		if len(firstPage.BridgeHealth) != 1 || firstPage.BridgeHealth["bridge-needle-global"].RouteCount != 1 {
			t.Fatalf("first page health = %#v, want returned id only", firstPage.BridgeHealth)
		}
		if catalogCalls != 1 || hydrationCalls != 1 || observer.bridgeEffectiveCalls != 1 ||
			observer.bridgeHealthForCalls != 1 ||
			!slices.Equal(hydratedIDs, []string{"bridge-needle-global"}) ||
			!slices.Equal(observer.bridgeHealthForIDs, hydratedIDs) {
			t.Fatalf(
				"first page calls = catalog:%d hydration:%d effective:%d health:%d hydrated:%#v health_ids:%#v",
				catalogCalls,
				hydrationCalls,
				observer.bridgeEffectiveCalls,
				observer.bridgeHealthForCalls,
				hydratedIDs,
				observer.bridgeHealthForIDs,
			)
		}

		observer.bridgeHealthForIDs = nil
		secondInput, err := json.Marshal(bridgeCatalogInput{
			Search:   "needle",
			Platform: "slack",
			Sort:     "name",
			Cursor:   firstPage.Page.NextCursor,
			Limit:    1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(second bridges_list input) error = %v", err)
		}
		second, err := registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDBridgesList, Input: secondInput},
		)
		if err != nil {
			t.Fatalf("Registry.Call(second bridges_list page) error = %v", err)
		}
		var secondPage nativeBridgeCatalogResponse
		if err := json.Unmarshal(second.Structured, &secondPage); err != nil {
			t.Fatalf("json.Unmarshal(second bridges_list page) error = %v", err)
		}
		if len(secondPage.Bridges) != 1 || secondPage.Bridges[0].ID != "bridge-needle-own" ||
			secondPage.Page.HasMore || secondPage.Page.Total != 2 {
			t.Fatalf("second page = %#v, want own workspace continuation", secondPage)
		}
		if !slices.Equal(observer.bridgeHealthForIDs, []string{"bridge-needle-own"}) {
			t.Fatalf("second page health ids = %#v, want own id only", observer.bridgeHealthForIDs)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDBridgesList,
				Input:  json.RawMessage(`{"workspace_id":"ws-2"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
	})
}

func openDaemonTestToolArtifactStore(t *testing.T) *toolspkg.FilesystemToolArtifactStore {
	t.Helper()
	store, err := toolspkg.OpenFilesystemToolArtifactStore(
		t.Context(),
		filepath.Join(t.TempDir(), "tool-artifacts"),
		toolspkg.ToolArtifactRetention{MaxCount: 10, MaxBytes: 1 << 20, MaxAge: time.Hour},
	)
	if err != nil {
		t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
	}
	return store
}

func TestDaemonBootToolRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should wire the native registry during daemon boot", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		skillsRegistry := newLoadedNativeSkillRegistry(t)
		state := &bootState{
			cfg:            cfg,
			registry:       &recordingRegistry{},
			skillsRegistry: skillsRegistry,
			deps: RuntimeDeps{
				SkillsRegistry: skillsRegistry,
				Network:        &nativeNetworkStub{},
				Tasks:          &nativeTaskManager{},
			},
		}
		daemon := &Daemon{homePaths: homePaths}

		cleanup := &bootCleanup{}
		if err := daemon.bootToolRegistry(t.Context(), state, cleanup); err != nil {
			t.Fatalf("bootToolRegistry() error = %v", err)
		}
		t.Cleanup(func() {
			var cleanupErr error
			cleanup.run(t.Context(), &cleanupErr)
			if cleanupErr != nil {
				t.Errorf("boot cleanup error = %v", cleanupErr)
			}
		})
		if state.toolRegistry == nil || state.deps.ToolRegistry == nil {
			t.Fatalf(
				"tool registry wiring = state:%#v deps:%#v, want both populated",
				state.toolRegistry,
				state.deps.ToolRegistry,
			)
		}
		view, err := state.deps.ToolRegistry.Get(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.ToolIDTaskCancel)
		if err != nil {
			t.Fatalf("ToolRegistry.Get(task_cancel) error = %v", err)
		}
		if !view.Descriptor.Destructive || view.Descriptor.ReadOnly {
			t.Fatalf("task_cancel risk flags = %#v, want destructive mutating tool", view.Descriptor)
		}
		_, err = state.deps.ToolRegistry.Get(t.Context(), toolspkg.Scope{Operator: true}, "agh__skill_remove")
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("ToolRegistry.Get(skill_remove) error = %v, want ErrToolNotFound", err)
		}
	})
}

func TestDaemonNativeRuntimePolicyResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve full default projection and scoped runtime policy inputs", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		sessions := &nativeToolPolicySessionStub{
			info: &session.Info{
				ID:        "sess-1",
				AgentName: "coder",
				State:     session.StateActive,
			},
		}
		agents := &nativeToolPolicyAgentResolverStub{
			agent: aghconfig.AgentDef{
				Name:     "coder",
				Provider: "opencode",
				Prompt:   "Use the available tools to help the user.",
			},
		}
		resolver, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
			Config:            &cfg,
			Sessions:          sessions,
			AgentResolver:     agents,
			ApprovalAvailable: true,
		})
		if err != nil {
			t.Fatalf("newNativeToolPolicyResolver() error = %v", err)
		}
		registry := newDaemonNativeRegistryWithPolicyResolver(t, &daemonNativeToolsDeps{
			Skills: newLoadedNativeSkillRegistry(t),
			Tasks:  &nativeTaskManager{},
		}, resolver)
		scope := toolspkg.Scope{SessionID: "sess-1"}

		views, err := registry.SessionProjection(ctx, scope)
		if err != nil {
			t.Fatalf("SessionProjection(full default) error = %v", err)
		}
		requireNativeViewContains(t, views, toolspkg.ToolIDToolList)
		requireNativeViewContains(t, views, toolspkg.ToolIDToolInfo)
		requireNativeViewContains(t, views, toolspkg.ToolIDSkillView)
		requireNativeViewContains(t, views, toolspkg.ToolIDTaskRead)

		_, err = registry.Call(ctx, scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolInfo,
			Input:  json.RawMessage(`{"tool_id":"agh__tool_list"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(tool_info full default) error = %v", err)
		}

		agents.agent.Toolsets = []string{toolspkg.ToolsetIDTasks.String()}
		views, err = registry.SessionProjection(ctx, scope)
		if err != nil {
			t.Fatalf("SessionProjection(agent narrowed to tasks) error = %v", err)
		}
		requireNativeViewContains(t, views, toolspkg.ToolIDTaskRead)
		requireNativeViewExcludes(t, views, toolspkg.ToolIDToolInfo)
		_, err = registry.Call(ctx, scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolInfo,
			Input:  json.RawMessage(`{"tool_id":"agh__tool_list"}`),
		})
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonPolicyDenied)

		agents.agent.Toolsets = nil
		sessions.info.Lineage = &store.SessionLineage{
			ParentSessionID: "parent-1",
			RootSessionID:   "root-1",
			SpawnDepth:      1,
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools: []string{toolspkg.ToolIDToolInfo.String()},
			},
		}
		views, err = registry.SessionProjection(ctx, scope)
		if err != nil {
			t.Fatalf("SessionProjection(session lineage) error = %v", err)
		}
		requireNativeViewContains(t, views, toolspkg.ToolIDToolInfo)
		requireNativeViewExcludes(t, views, toolspkg.ToolIDToolList)
		_, err = registry.Call(ctx, scope, toolspkg.CallRequest{ToolID: toolspkg.ToolIDToolList})
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonSessionDenied)
	})

	t.Run("Should trust bundled extension read only tools in default runtime policy", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		deps, extensionRegistry, _, _ := newNativeExtensionToolDeps(t)
		cfg := testConfig(t, deps.HomePaths)
		if err := devcycle.EnsureManagedInstall(deps.HomePaths, extensionRegistry); err != nil {
			t.Fatalf("EnsureManagedInstall(dev-cycle) error = %v", err)
		}
		runtime := &nativeBundledDevCycleToolRuntime{
			devCycleLoopSchemaRuntime: newDevCycleLoopSchemaRuntime(t, extensionRegistry),
		}
		provider, err := extensionpkg.NewExtensionToolProvider(
			extensionRegistry,
			func() extensionpkg.ExtensionToolRuntime {
				return runtime
			},
		)
		if err != nil {
			t.Fatalf("NewExtensionToolProvider() error = %v", err)
		}
		resolver, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
			Config:            &cfg,
			ExtensionRegistry: extensionRegistry,
			ApprovalAvailable: true,
		})
		if err != nil {
			t.Fatalf("newNativeToolPolicyResolver() error = %v", err)
		}
		toolsets, err := builtintools.ToolsetCatalog()
		if err != nil {
			t.Fatalf("ToolsetCatalog() error = %v", err)
		}
		registry, err := toolspkg.NewRegistry(
			toolspkg.WithProviders(provider),
			toolspkg.WithPolicyInputResolver(resolver, toolsets),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		const importTasksToolID toolspkg.ToolID = "ext__dev_cycle__import_tasks"

		views, err := registry.SessionProjection(ctx, toolspkg.Scope{})
		if err != nil {
			t.Fatalf("SessionProjection(extension tools) error = %v", err)
		}
		requireNativeViewContains(t, views, importTasksToolID)

		result, err := registry.Call(ctx, toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: importTasksToolID,
			Input:  json.RawMessage(`{"pattern":"*.md"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(import_tasks) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"count":0`))
		if len(runtime.calls) != 1 {
			t.Fatalf("runtime calls = %d, want 1", len(runtime.calls))
		}
		if got := runtime.calls[0].extensionID; got != devcycle.Name {
			t.Fatalf("runtime extension id = %q, want %q", got, devcycle.Name)
		}
		if got := runtime.calls[0].request.ToolID; got != importTasksToolID {
			t.Fatalf("runtime tool id = %q, want %q", got, importTasksToolID)
		}

		inputs, err := resolver.Resolve(ctx, toolspkg.Scope{})
		if err != nil {
			t.Fatalf("Resolve(bundled enabled) error = %v", err)
		}
		bundledGrant := toolspkg.SourceGrant{Kind: toolspkg.SourceExtension, Owner: devcycle.Name}
		if !sourceGrantExists(inputs.TrustedSources, bundledGrant) {
			t.Fatalf("enabled trusted sources = %#v, want %#v", inputs.TrustedSources, bundledGrant)
		}
		if err := extensionRegistry.Disable(devcycle.Name); err != nil {
			t.Fatalf("Disable(dev-cycle) error = %v", err)
		}
		inputs, err = resolver.Resolve(ctx, toolspkg.Scope{})
		if err != nil {
			t.Fatalf("Resolve(bundled disabled) error = %v", err)
		}
		if sourceGrantExists(inputs.TrustedSources, bundledGrant) {
			t.Fatalf("disabled trusted sources = %#v, want bundled grant removed", inputs.TrustedSources)
		}
		if err := extensionRegistry.Enable(devcycle.Name); err != nil {
			t.Fatalf("Enable(dev-cycle) error = %v", err)
		}
		inputs, err = resolver.Resolve(ctx, toolspkg.Scope{})
		if err != nil {
			t.Fatalf("Resolve(bundled re-enabled) error = %v", err)
		}
		if !sourceGrantExists(inputs.TrustedSources, bundledGrant) {
			t.Fatalf("re-enabled trusted sources = %#v, want %#v", inputs.TrustedSources, bundledGrant)
		}
	})

	t.Run("Should return diagnostic status for tools denied by explicit agent policy", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		sessions := &nativeToolPolicySessionStub{
			info: &session.Info{
				ID:        "sess-catalog",
				AgentName: "catalog-only",
				State:     session.StateActive,
			},
		}
		agents := &nativeToolPolicyAgentResolverStub{
			agent: aghconfig.AgentDef{
				Name:     "catalog-only",
				Provider: "opencode",
				Prompt:   "Inspect the tool catalog.",
				Toolsets: []string{toolspkg.ToolsetIDCatalog.String()},
			},
		}
		resolver, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
			Config:            &cfg,
			Sessions:          sessions,
			AgentResolver:     agents,
			ApprovalAvailable: true,
		})
		if err != nil {
			t.Fatalf("newNativeToolPolicyResolver() error = %v", err)
		}
		registry := newDaemonNativeRegistryWithPolicyResolver(t, &daemonNativeToolsDeps{
			Skills:  newLoadedNativeSkillRegistry(t),
			Network: &nativeNetworkStub{},
		}, resolver)
		scope := toolspkg.Scope{SessionID: "sess-catalog"}

		views, err := registry.SessionProjection(ctx, scope)
		if err != nil {
			t.Fatalf("SessionProjection(catalog-only) error = %v", err)
		}
		requireNativeViewContains(t, views, toolspkg.ToolIDToolSearch)
		requireNativeViewContains(t, views, toolspkg.ToolIDToolInfo)
		requireNativeViewExcludes(t, views, toolspkg.ToolIDNetworkChannelCreate)

		searchResult, err := registry.Call(ctx, scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolSearch,
			Input:  json.RawMessage(`{"query":"network channel create"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(tool_search diagnostic) error = %v", err)
		}
		requireNativeStructuredContains(t, searchResult, []byte(`"agh__network_channel_create"`))
		requireNativeStructuredContains(t, searchResult, []byte(`"callable":false`))
		requireNativeStructuredContains(t, searchResult, []byte(`"policy_denied"`))

		infoResult, err := registry.Call(ctx, scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDToolInfo,
			Input:  json.RawMessage(`{"tool_id":"agh__network_channel_create"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(tool_info diagnostic) error = %v", err)
		}
		requireNativeStructuredContains(t, infoResult, []byte(`"agh__network_channel_create"`))
		requireNativeStructuredContains(t, infoResult, []byte(`"callable":false`))
		requireNativeStructuredContains(t, infoResult, []byte(`"policy_denied"`))
	})

	t.Run(
		"Should bind coordinator-safe native tools to the caller session workspace and channel policy",
		func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			homePaths := testHomePaths(t)
			cfg := testConfig(t, homePaths)
			root := t.TempDir()

			statusCalls := make([]string, 0, 8)
			eventCalls := make([]string, 0, 1)
			historyCalls := make([]string, 0, 1)
			sessions := apitest.StubSessionManager{
				StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					statusCalls = append(statusCalls, id)
					switch strings.TrimSpace(id) {
					case "sess-coord":
						return &session.Info{
							ID:                   "sess-coord",
							AgentName:            "coordinator",
							Type:                 session.SessionTypeCoordinator,
							State:                session.StateActive,
							WorkspaceID:          "ws-coord",
							NetworkParticipation: daemonTestLiveParticipation("ws-coord", "ch-run-1"),
							Lineage: &store.SessionLineage{
								ParentSessionID: "sess-root",
								RootSessionID:   "sess-root",
								SpawnDepth:      1,
								PermissionPolicy: store.SessionPermissionPolicy{
									Tools: []string{
										toolspkg.ToolIDNetworkChannels.String(),
										toolspkg.ToolIDNetworkInbox.String(),
										toolspkg.ToolIDNetworkSend.String(),
										toolspkg.ToolIDSessionDescribe.String(),
									},
									NetworkChannels: []string{"ch-run-1"},
								},
							},
						}, nil
					case "sess-foreign":
						return &session.Info{
							ID:          "sess-foreign",
							AgentName:   "worker",
							State:       session.StateActive,
							WorkspaceID: "ws-foreign",
						}, nil
					default:
						return nil, session.ErrSessionNotFound
					}
				},
				EventsFn: func(ctx context.Context, id string, _ store.EventQuery) ([]store.SessionEvent, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					eventCalls = append(eventCalls, id)
					return nil, nil
				},
				HistoryFn: func(ctx context.Context, id string, _ store.EventQuery) ([]store.TurnHistory, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					historyCalls = append(historyCalls, id)
					return nil, nil
				},
			}
			workspaces := apitest.StubWorkspaceService{
				ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if err := ctx.Err(); err != nil {
						return workspacepkg.ResolvedWorkspace{}, err
					}
					workspaceID := strings.TrimSpace(ref)
					if workspaceID == "" {
						return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      workspaceID,
							RootDir: root,
							Name:    workspaceID,
						},
						WorkspaceID: workspaceID,
						Config:      cfg,
					}, nil
				},
			}
			resolver, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
				Config:            &cfg,
				Sessions:          sessions,
				WorkspaceResolver: workspaces,
				ApprovalAvailable: true,
			})
			if err != nil {
				t.Fatalf("newNativeToolPolicyResolver() error = %v", err)
			}
			networkService := &nativeNetworkStub{
				channels: []network.ChannelInfo{
					{WorkspaceID: "ws-coord", Channel: "ch-run-1", PeerCount: 1},
					{WorkspaceID: "ws-coord", Channel: "ch-run-2", PeerCount: 1},
				},
				inbox: []network.Envelope{
					{
						ID:      "msg-allowed",
						Kind:    network.KindSay,
						Channel: "ch-run-1",
						From:    "peer-1",
						Body:    json.RawMessage(`{"text":"allowed"}`),
					},
					{
						ID:      "msg-blocked",
						Kind:    network.KindSay,
						Channel: "ch-run-2",
						From:    "peer-2",
						Body:    json.RawMessage(`{"text":"blocked"}`),
					},
				},
			}
			registry := newDaemonNativeRegistryWithPolicyResolver(t, &daemonNativeToolsDeps{
				Network:    networkService,
				Sessions:   sessions,
				Workspaces: workspaces,
			}, resolver)
			scope := toolspkg.Scope{SessionID: "sess-coord"}

			channelsResult, err := registry.Call(ctx, scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkChannels,
				Input:  json.RawMessage(`{"workspace_id":"ws-foreign"}`),
			})
			if err != nil {
				t.Fatalf("Registry.Call(network_channels) error = %v", err)
			}
			requireNativeStructuredContains(t, channelsResult, []byte(`"channel":"ch-run-1"`))
			requireNativeStructuredExcludes(t, channelsResult, []byte(`"channel":"ch-run-2"`))
			if got := networkService.channelsWorkspaceID; got != "ws-coord" {
				t.Fatalf("Network.ListChannels workspace_id = %q, want ws-coord", got)
			}

			inboxResult, err := registry.Call(ctx, scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkInbox,
				Input:  json.RawMessage(`{"workspace_id":"ws-foreign","session_id":"sess-foreign"}`),
			})
			if err != nil {
				t.Fatalf("Registry.Call(network_inbox) error = %v", err)
			}
			requireNativeStructuredContains(t, inboxResult, []byte(`"msg-allowed"`))
			requireNativeStructuredExcludes(t, inboxResult, []byte(`"msg-blocked"`))
			if got := networkService.inboxSessionID; got != "sess-coord" {
				t.Fatalf("Network.Inbox session_id = %q, want sess-coord", got)
			}

			describeResult, err := registry.Call(ctx, scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSessionDescribe,
				Input:  json.RawMessage(`{"workspace_id":"ws-foreign","session_id":"sess-foreign"}`),
			})
			if err != nil {
				t.Fatalf("Registry.Call(session_describe) error = %v", err)
			}
			requireNativeStructuredContains(t, describeResult, []byte(`"id":"sess-coord"`))
			if len(eventCalls) != 1 || eventCalls[0] != "sess-coord" {
				t.Fatalf("Sessions.Events calls = %#v, want only sess-coord", eventCalls)
			}
			if len(historyCalls) != 1 || historyCalls[0] != "sess-coord" {
				t.Fatalf("Sessions.History calls = %#v, want only sess-coord", historyCalls)
			}

			_, err = registry.Call(ctx, scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkSend,
				Input: json.RawMessage(
					`{"workspace_id":"ws-foreign","session_id":"sess-foreign","channel":"ch-run-1","surface":"thread","thread_id":"thread_coord","kind":"say","body":{"text":"hello"}}`,
				),
			})
			if err != nil {
				t.Fatalf("Registry.Call(network_send allowed) error = %v", err)
			}
			if got := networkService.lastSend.SessionID; got != "sess-coord" {
				t.Fatalf("Network.Send session_id = %q, want sess-coord", got)
			}
			if got := networkService.lastSend.WorkspaceID; got != "ws-coord" {
				t.Fatalf("Network.Send workspace_id = %q, want ws-coord", got)
			}

			_, err = registry.Call(ctx, scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDNetworkSend,
				Input: json.RawMessage(
					`{"workspace_id":"ws-foreign","session_id":"sess-foreign","channel":"ch-run-2","surface":"thread","thread_id":"thread_coord","kind":"say","body":{"text":"blocked"}}`,
				),
			})
			requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonSessionDenied)
			if got := networkService.sendCalls; got != 1 {
				t.Fatalf("Network.Send calls = %d, want 1", got)
			}
			if slices.Contains(statusCalls, "sess-foreign") {
				t.Fatalf("Sessions.Status calls = %#v, want caller session only", statusCalls)
			}
		},
	)

	t.Run("Should keep Memory v2 write tools root-only unless lineage grants them", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		sessions := &nativeToolPolicySessionStub{
			info: &session.Info{
				ID:        "sess-root",
				AgentName: "coder",
				State:     session.StateActive,
			},
		}
		agents := &nativeToolPolicyAgentResolverStub{
			agent: aghconfig.AgentDef{
				Name:        "coder",
				Provider:    "opencode",
				Prompt:      "Use memory tools deliberately.",
				Permissions: string(aghconfig.PermissionModeApproveAll),
				Toolsets:    []string{toolspkg.ToolsetIDMemory.String()},
			},
		}
		resolver, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
			Config:            &cfg,
			Sessions:          sessions,
			AgentResolver:     agents,
			ApprovalAvailable: true,
		})
		if err != nil {
			t.Fatalf("newNativeToolPolicyResolver() error = %v", err)
		}
		memoryStore := memorypkg.NewStore(filepath.Join(t.TempDir(), "memory"))
		registry := newDaemonNativeRegistryWithPolicyResolver(t, &daemonNativeToolsDeps{
			MemoryStore: memoryStore,
		}, resolver)
		rootScope := toolspkg.Scope{SessionID: "sess-root"}

		rootViews, err := registry.SessionProjection(ctx, rootScope)
		if err != nil {
			t.Fatalf("SessionProjection(root memory) error = %v", err)
		}
		requireNativeViewContains(t, rootViews, toolspkg.ToolIDMemoryShow)
		requireNativeViewContains(t, rootViews, toolspkg.ToolIDMemoryPropose)
		requireNativeViewContains(t, rootViews, toolspkg.ToolIDMemoryNote)

		sessions.info.ID = "sess-child"
		sessions.info.Lineage = &store.SessionLineage{
			ParentSessionID: "sess-root",
			RootSessionID:   "sess-root",
			SpawnDepth:      1,
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools: []string{
					toolspkg.ToolIDMemoryList.String(),
					toolspkg.ToolIDMemoryShow.String(),
					toolspkg.ToolIDMemorySearch.String(),
				},
			},
		}
		childScope := toolspkg.Scope{SessionID: "sess-child"}
		childViews, err := registry.SessionProjection(ctx, childScope)
		if err != nil {
			t.Fatalf("SessionProjection(child memory) error = %v", err)
		}
		requireNativeViewContains(t, childViews, toolspkg.ToolIDMemoryShow)
		requireNativeViewExcludes(t, childViews, toolspkg.ToolIDMemoryPropose)
		requireNativeViewExcludes(t, childViews, toolspkg.ToolIDMemoryNote)
	})
}

type nativeBundledDevCycleToolRuntime struct {
	*devCycleLoopSchemaRuntime
	calls []nativeBundledDevCycleToolCall
}

type nativeBundledDevCycleToolCall struct {
	extensionID string
	request     toolspkg.ExtensionToolCallRequest
}

func (r *nativeBundledDevCycleToolRuntime) CallTool(
	ctx context.Context,
	extensionID string,
	req toolspkg.ExtensionToolCallRequest,
) (toolspkg.ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return toolspkg.ToolResult{}, err
	}
	r.calls = append(r.calls, nativeBundledDevCycleToolCall{
		extensionID: extensionID,
		request:     req,
	})
	structured := json.RawMessage(`{"tasks":[],"count":0}`)
	return toolspkg.ToolResult{
		Structured: structured,
		Preview:    string(structured),
		Bytes:      int64(len(structured)),
	}, nil
}

func newDaemonNativeRegistry(
	t *testing.T,
	deps *daemonNativeToolsDeps,
	policyInputs toolspkg.PolicyInputs,
) *toolspkg.RuntimeRegistry {
	t.Helper()

	return newDaemonNativeRegistryWithPolicyResolver(
		t,
		deps,
		toolspkg.NewStaticPolicyInputResolver(policyInputs),
	)
}

func newDaemonNativeRegistryWithPolicyResolver(
	t *testing.T,
	deps *daemonNativeToolsDeps,
	resolver toolspkg.PolicyInputResolver,
) *toolspkg.RuntimeRegistry {
	t.Helper()

	if deps == nil {
		deps = &daemonNativeToolsDeps{}
	}
	var registry *toolspkg.RuntimeRegistry
	deps.Registry = func() toolspkg.Registry {
		return registry
	}
	provider, err := newDaemonNativeProvider(deps)
	if err != nil {
		t.Fatalf("newDaemonNativeProvider() error = %v", err)
	}
	toolsets, err := builtintools.ToolsetCatalog()
	if err != nil {
		t.Fatalf("builtin.ToolsetCatalog() error = %v", err)
	}
	registry, err = toolspkg.NewRegistry(
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputResolver(resolver, toolsets),
		toolspkg.WithDefaultMaxResultBytes(aghconfig.DefaultToolsMaxResultBytes),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

type nativeToolPolicySessionStub struct {
	info *session.Info
	err  error
}

func (s *nativeToolPolicySessionStub) Status(context.Context, string) (*session.Info, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.info, nil
}

type nativeToolPolicyAgentResolverStub struct {
	agent aghconfig.AgentDef
	err   error
}

func (r *nativeToolPolicyAgentResolverStub) ResolveAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	if r.err != nil {
		return aghconfig.AgentDef{}, r.err
	}
	if name != r.agent.Name {
		return aghconfig.AgentDef{}, fmt.Errorf("%w: %s", workspacepkg.ErrAgentNotAvailable, name)
	}
	return r.agent, nil
}

func nativeApproveAllPolicyInputs() toolspkg.PolicyInputs {
	return toolspkg.PolicyInputs{
		SystemPermissionMode: toolspkg.PermissionModeApproveAll,
		ApprovalAvailable:    true,
	}
}

type nativeMemoryAdminFixture struct {
	registry      *toolspkg.RuntimeRegistry
	memoryStore   *memorypkg.Store
	decision      memcontract.Decision
	agentDecision memcontract.Decision
	dream         *nativeDreamTriggerService
	extractor     *nativeMemoryExtractorService
	providers     *nativeMemoryProviderService
	ledger        *nativeMemorySessionLedgerService
}

func staticNativeInput(input string) func(nativeMemoryAdminFixture) json.RawMessage {
	return func(nativeMemoryAdminFixture) json.RawMessage {
		return json.RawMessage(input)
	}
}

func newNativeMemoryAdminFixture(t *testing.T) nativeMemoryAdminFixture {
	t.Helper()

	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	globalDir := filepath.Join(t.TempDir(), "memory")
	memoryStore := memorypkg.NewStore(
		globalDir,
		memorypkg.WithCatalogDatabasePath(filepath.Join(t.TempDir(), store.GlobalDatabaseName)),
	)
	openDaemonMemoryCatalog(t, memoryStore)
	if err := memoryStore.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	if err := memoryStore.Write(
		memcontract.ScopeGlobal,
		"ops.md",
		nativeMemoryDocument("Ops", "Operational memory", memcontract.TypeUser, "memory admin health"),
	); err != nil {
		t.Fatalf("Write(global memory) error = %v", err)
	}
	decision, err := memoryStore.ProposeCandidate(t.Context(), memcontract.Candidate{
		Scope:   memcontract.ScopeGlobal,
		Origin:  memcontract.OriginTool,
		Content: "Native Memory admin decisions stay inspectable.",
		Frontmatter: memcontract.Header{
			Name:  "Native admin decision",
			Type:  memcontract.TypeUser,
			Scope: memcontract.ScopeGlobal,
		},
		SubmittedAt: now,
	})
	if err != nil {
		t.Fatalf("ProposeCandidate() error = %v", err)
	}
	agentStore := memoryStore.ForAgent("", "coder", memcontract.AgentTierGlobal)
	if err := agentStore.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs(agent memory) error = %v", err)
	}
	agentDecision, err := agentStore.ProposeCandidate(t.Context(), memcontract.Candidate{
		Scope:     memcontract.ScopeAgent,
		AgentName: "coder",
		AgentTier: memcontract.AgentTierGlobal,
		Origin:    memcontract.OriginTool,
		Content:   "Native Memory admin agent decisions stay scoped.",
		Frontmatter: memcontract.Header{
			Name:      "Native admin agent decision",
			Type:      memcontract.TypeFeedback,
			Scope:     memcontract.ScopeAgent,
			AgentName: "coder",
			AgentTier: memcontract.AgentTierGlobal,
		},
		SubmittedAt: now,
	})
	if err != nil {
		t.Fatalf("ProposeCandidate(agent) error = %v", err)
	}

	cfg := aghconfig.Config{}
	cfg.Memory.Enabled = true
	cfg.Memory.GlobalDir = globalDir
	cfg.Roles.Dream.Agent = aghconfig.BuiltinDreamingCuratorAgentName
	cfg.Memory.Dream.CheckInterval = time.Hour
	dream := &nativeDreamTriggerService{enabled: true, triggered: true, reason: "queued", last: now}
	extractor := &nativeMemoryExtractorService{
		status: contract.MemoryExtractorStatusPayload{
			Status:         contract.MemoryExtractorStateIdle,
			QueuedSessions: 2,
			FailureCount:   1,
		},
		failures: []contract.MemoryExtractorFailurePayload{{
			ID:        "failure-native",
			SessionID: "sess-memory",
			Reason:    "decode failed",
			Path:      filepath.Join(t.TempDir(), "failure.json"),
			CreatedAt: now,
		}},
		retry: contract.MemoryExtractorRetryResponse{Retried: 1},
		drain: contract.MemoryExtractorDrainResponse{DrainedAt: now, Remaining: 0},
	}
	providers := &nativeMemoryProviderService{
		provider: contract.MemoryProviderPayload{
			Name:   "builtin",
			Status: contract.MemoryProviderStateActive,
			Active: true,
			Tools:  []string{toolspkg.ToolIDMemoryPropose.String()},
		},
	}
	ledger := &nativeMemorySessionLedgerService{
		response: contract.MemorySessionLedgerResponse{
			Meta: contract.MemorySessionLedgerMetaPayload{
				Version:   1,
				SessionID: "sess-memory",
				Path:      filepath.Join(t.TempDir(), "sess-memory.jsonl"),
				Checksum:  "sha256:test",
				CreatedAt: now,
			},
		},
		replay: contract.MemorySessionReplayResponse{
			Events: []contract.MemorySessionLedgerEntryPayload{{
				Sequence:  1,
				EventType: "message.created",
				EmittedAt: now,
			}},
		},
		prune:  contract.MemorySessionsPruneResponse{PrunedSessions: 2, PrunedEvents: 3, DryRun: true},
		repair: contract.MemorySessionsRepairResponse{RepairedLedgers: 1, CompletedAt: now},
	}
	registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
		Config:              cfg,
		MemoryStore:         memoryStore,
		DreamTrigger:        dream,
		MemoryExtractor:     extractor,
		MemoryProviders:     providers,
		MemorySessionLedger: ledger,
		Sessions:            nativeNetworkTestSessionManager("ws-1"),
		Workspaces:          nativeNetworkTestWorkspaceService(t),
	}, nativeApproveAllPolicyInputs())
	return nativeMemoryAdminFixture{
		registry:      registry,
		memoryStore:   memoryStore,
		decision:      decision,
		agentDecision: agentDecision,
		dream:         dream,
		extractor:     extractor,
		providers:     providers,
		ledger:        ledger,
	}
}

func openDaemonMemoryCatalog(t *testing.T, store *memorypkg.Store) {
	t.Helper()
	ctx := testutil.Context(t)
	if err := store.OpenCatalog(ctx); err != nil {
		t.Fatalf("MemoryStore.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(ctx); err != nil {
			t.Errorf("MemoryStore.CloseCatalog() error = %v", err)
		}
	})
}

type nativeDreamTriggerService struct {
	enabled       bool
	triggered     bool
	reason        string
	last          time.Time
	err           error
	triggerCalls  int
	lastWorkspace string
}

func (s *nativeDreamTriggerService) Trigger(_ context.Context, workspace string) (bool, string, error) {
	s.triggerCalls++
	s.lastWorkspace = workspace
	if s.err != nil {
		return false, "", s.err
	}
	return s.triggered, s.reason, nil
}

func (s *nativeDreamTriggerService) LastConsolidatedAt() (time.Time, error) {
	if s.err != nil {
		return time.Time{}, s.err
	}
	return s.last, nil
}

func (s *nativeDreamTriggerService) Enabled() bool {
	return s.enabled
}

type nativeModelCatalogService struct {
	models               []modelcatalog.Model
	statuses             []modelcatalog.SourceStatus
	listCalls            int
	refreshCalls         int
	statusCalls          int
	lastList             modelcatalog.ListOptions
	lastRefresh          modelcatalog.RefreshOptions
	lastStatusProviderID string
	listErr              error
	refreshErr           error
	statusErr            error
}

type nativeProviderModelSettingsService struct {
	core.SettingsService
	result      settingspkg.ProviderModelCurationResult
	err         error
	calls       int
	lastRequest settingspkg.ProviderModelCurationRequest
}

type nativeConfigSettingsService struct {
	core.SettingsService
	result      settingspkg.ApplyResult
	err         error
	reloadCalls int
}

func (s *nativeConfigSettingsService) Reload(
	_ context.Context,
) (settingspkg.ApplyResult, error) {
	s.reloadCalls++
	if s.err != nil {
		return settingspkg.ApplyResult{}, s.err
	}
	return s.result, nil
}

func (s *nativeProviderModelSettingsService) ApplyProviderModelCuration(
	_ context.Context,
	req settingspkg.ProviderModelCurationRequest,
) (settingspkg.ProviderModelCurationResult, error) {
	s.calls++
	s.lastRequest = req
	if s.err != nil {
		return settingspkg.ProviderModelCurationResult{}, s.err
	}
	return s.result, nil
}

func (s *nativeModelCatalogService) ListModels(
	_ context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	s.listCalls++
	s.lastList = opts
	if s.listErr != nil {
		return nil, s.listErr
	}
	return append([]modelcatalog.Model(nil), s.models...), nil
}

func (s *nativeModelCatalogService) Refresh(
	_ context.Context,
	opts modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	s.refreshCalls++
	s.lastRefresh = opts
	if s.refreshErr != nil {
		return append([]modelcatalog.SourceStatus(nil), s.statuses...), s.refreshErr
	}
	return append([]modelcatalog.SourceStatus(nil), s.statuses...), nil
}

func (s *nativeModelCatalogService) ListSourceStatus(
	_ context.Context,
	providerID string,
) ([]modelcatalog.SourceStatus, error) {
	s.statusCalls++
	s.lastStatusProviderID = providerID
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	return append([]modelcatalog.SourceStatus(nil), s.statuses...), nil
}

func (s *nativeModelCatalogService) totalCalls() int {
	return s.listCalls + s.refreshCalls + s.statusCalls
}

type nativeMemoryExtractorService struct {
	status       contract.MemoryExtractorStatusPayload
	failures     []contract.MemoryExtractorFailurePayload
	retry        contract.MemoryExtractorRetryResponse
	drain        contract.MemoryExtractorDrainResponse
	err          error
	statusCalls  int
	failureCalls int
	retryCalls   int
	drainCalls   int
	lastRetry    contract.MemoryExtractorRetryRequest
}

func (s *nativeMemoryExtractorService) Status(
	context.Context,
) (contract.MemoryExtractorStatusPayload, error) {
	s.statusCalls++
	if s.err != nil {
		return contract.MemoryExtractorStatusPayload{}, s.err
	}
	return s.status, nil
}

func (s *nativeMemoryExtractorService) ListFailures(
	context.Context,
) ([]contract.MemoryExtractorFailurePayload, error) {
	s.failureCalls++
	if s.err != nil {
		return nil, s.err
	}
	return append([]contract.MemoryExtractorFailurePayload(nil), s.failures...), nil
}

func (s *nativeMemoryExtractorService) Retry(
	_ context.Context,
	req contract.MemoryExtractorRetryRequest,
) (contract.MemoryExtractorRetryResponse, error) {
	s.retryCalls++
	s.lastRetry = req
	if s.err != nil {
		return contract.MemoryExtractorRetryResponse{}, s.err
	}
	return s.retry, nil
}

func (s *nativeMemoryExtractorService) Drain(
	context.Context,
) (contract.MemoryExtractorDrainResponse, error) {
	s.drainCalls++
	if s.err != nil {
		return contract.MemoryExtractorDrainResponse{}, s.err
	}
	if !s.drain.DrainedAt.IsZero() {
		return s.drain, nil
	}
	return contract.MemoryExtractorDrainResponse{DrainedAt: time.Now().UTC()}, nil
}

func (s *nativeMemoryExtractorService) totalCalls() int {
	return s.statusCalls + s.failureCalls + s.retryCalls + s.drainCalls
}

type nativeMemoryProviderService struct {
	provider        contract.MemoryProviderPayload
	err             error
	listCalls       int
	getCalls        int
	selectCalls     int
	enableCalls     int
	disableCalls    int
	lastWorkspaceID string
	lastName        string
	lastReason      string
}

func (s *nativeMemoryProviderService) List(
	_ context.Context,
	workspaceID string,
) ([]contract.MemoryProviderPayload, error) {
	s.listCalls++
	s.lastWorkspaceID = workspaceID
	if s.err != nil {
		return nil, s.err
	}
	return []contract.MemoryProviderPayload{s.provider}, nil
}

func (s *nativeMemoryProviderService) Get(
	_ context.Context,
	workspaceID string,
	name string,
) (contract.MemoryProviderPayload, error) {
	s.getCalls++
	s.lastWorkspaceID = workspaceID
	s.lastName = name
	if s.err != nil {
		return contract.MemoryProviderPayload{}, s.err
	}
	return s.provider, nil
}

func (s *nativeMemoryProviderService) Select(
	_ context.Context,
	workspaceID string,
	name string,
) (contract.MemoryProviderPayload, error) {
	s.selectCalls++
	s.lastWorkspaceID = workspaceID
	s.lastName = name
	if s.err != nil {
		return contract.MemoryProviderPayload{}, s.err
	}
	selected := s.provider
	selected.Name = name
	selected.Active = true
	selected.Status = contract.MemoryProviderStateActive
	return selected, nil
}

func (s *nativeMemoryProviderService) Enable(
	_ context.Context,
	workspaceID string,
	name string,
	reason string,
) (contract.MemoryProviderLifecycleResponse, error) {
	s.enableCalls++
	s.lastWorkspaceID = workspaceID
	s.lastName = name
	s.lastReason = reason
	if s.err != nil {
		return contract.MemoryProviderLifecycleResponse{}, s.err
	}
	enabled := s.provider
	enabled.Name = name
	return contract.MemoryProviderLifecycleResponse{Provider: enabled, Changed: true}, nil
}

func (s *nativeMemoryProviderService) Disable(
	_ context.Context,
	workspaceID string,
	name string,
	reason string,
) (contract.MemoryProviderLifecycleResponse, error) {
	s.disableCalls++
	s.lastWorkspaceID = workspaceID
	s.lastName = name
	s.lastReason = reason
	if s.err != nil {
		return contract.MemoryProviderLifecycleResponse{}, s.err
	}
	disabled := s.provider
	disabled.Name = name
	disabled.Active = false
	disabled.Status = contract.MemoryProviderStateStandby
	return contract.MemoryProviderLifecycleResponse{Provider: disabled, Changed: true}, nil
}

func (s *nativeMemoryProviderService) totalCalls() int {
	return s.listCalls + s.getCalls + s.selectCalls + s.enableCalls + s.disableCalls
}

type nativeMemorySessionLedgerService struct {
	response      contract.MemorySessionLedgerResponse
	replay        contract.MemorySessionReplayResponse
	prune         contract.MemorySessionsPruneResponse
	repair        contract.MemorySessionsRepairResponse
	err           error
	getCalls      int
	replayCalls   int
	pruneCalls    int
	repairCalls   int
	lastSessionID string
	lastReplay    contract.MemorySessionReplayRequest
	lastPrune     contract.MemorySessionsPruneRequest
}

func (s *nativeMemorySessionLedgerService) Get(
	_ context.Context,
	sessionID string,
) (contract.MemorySessionLedgerResponse, error) {
	s.getCalls++
	s.lastSessionID = sessionID
	if s.err != nil {
		return contract.MemorySessionLedgerResponse{}, s.err
	}
	response := s.response
	response.Meta.SessionID = sessionID
	return response, nil
}

func (s *nativeMemorySessionLedgerService) Replay(
	_ context.Context,
	sessionID string,
	req contract.MemorySessionReplayRequest,
) (contract.MemorySessionReplayResponse, error) {
	s.replayCalls++
	s.lastSessionID = sessionID
	s.lastReplay = req
	if s.err != nil {
		return contract.MemorySessionReplayResponse{}, s.err
	}
	response := s.replay
	response.SessionID = sessionID
	return response, nil
}

func (s *nativeMemorySessionLedgerService) Prune(
	_ context.Context,
	req contract.MemorySessionsPruneRequest,
) (contract.MemorySessionsPruneResponse, error) {
	s.pruneCalls++
	s.lastPrune = req
	if s.err != nil {
		return contract.MemorySessionsPruneResponse{}, s.err
	}
	return s.prune, nil
}

func (s *nativeMemorySessionLedgerService) Repair(
	context.Context,
) (contract.MemorySessionsRepairResponse, error) {
	s.repairCalls++
	if s.err != nil {
		return contract.MemorySessionsRepairResponse{}, s.err
	}
	if !s.repair.CompletedAt.IsZero() {
		return s.repair, nil
	}
	return contract.MemorySessionsRepairResponse{CompletedAt: time.Now().UTC()}, nil
}

func (s *nativeMemorySessionLedgerService) totalCalls() int {
	return s.getCalls + s.replayCalls + s.pruneCalls + s.repairCalls
}

type nativeMemoryToolWriteRecorder struct {
	sessionID string
	turnSeq   int64
	calls     int
}

func (r *nativeMemoryToolWriteRecorder) RecordToolWrite(sessionID string, turnSeq int64) {
	r.sessionID = sessionID
	r.turnSeq = turnSeq
	r.calls++
}

func newLoadedNativeSkillRegistry(t *testing.T) *skills.Registry {
	t.Helper()

	registry := skills.NewRegistry(skills.RegistryConfig{BundledFS: skillbundled.FS()})
	if err := registry.LoadAll(t.Context()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	return registry
}

func TestNativeSkillsForSessionAgentUsesLiveAuthoredDefinition(t *testing.T) {
	t.Run("Should resolve authored agents by name and preserve package-owned session snapshots", func(t *testing.T) {
		t.Parallel()

		registry := &recordingNativeSessionSkillRegistry{
			Registry: newLoadedNativeSkillRegistry(t),
		}
		manager := &nativeSessionAgentManager{
			agent: aghconfig.AgentDef{
				Name:       "reviewer",
				SourcePath: "/tmp/.agh/agents/reviewer/AGENT.md",
			},
		}
		native := &daemonNativeTools{deps: &daemonNativeToolsDeps{
			Skills:   registry,
			Sessions: manager,
		}}
		scope := toolspkg.Scope{SessionID: "sess-authored", AgentName: "reviewer"}
		if _, err := native.skillsFor(t.Context(), scope, toolspkg.ToolIDSkillList, ""); err != nil {
			t.Fatalf("skillsFor(authored) error = %v", err)
		}

		manager.agent.SourcePath = ""
		if _, err := native.skillsFor(t.Context(), scope, toolspkg.ToolIDSkillList, ""); err != nil {
			t.Fatalf("skillsFor(packaged) error = %v", err)
		}
		if got, want := registry.resolvedNames, []string{"reviewer"}; !slices.Equal(got, want) {
			t.Fatalf("resolved agent names = %#v, want %#v", got, want)
		}
		if got, want := registry.concreteNames, []string{"reviewer"}; !slices.Equal(got, want) {
			t.Fatalf("concrete agent names = %#v, want %#v", got, want)
		}
	})
}

type nativeSessionAgentManager struct {
	apitest.StubSessionManager
	agent aghconfig.AgentDef
}

func (m *nativeSessionAgentManager) SessionAgentDefinition(string) (aghconfig.AgentDef, bool) {
	return aghconfig.CloneAgentDef(m.agent), true
}

type recordingNativeSessionSkillRegistry struct {
	*skills.Registry
	resolvedNames []string
	concreteNames []string
}

func (r *recordingNativeSessionSkillRegistry) ForAgentSession(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
	agentName string,
	_ string,
) ([]*skills.Skill, error) {
	r.resolvedNames = append(r.resolvedNames, agentName)
	return nil, nil
}

func (r *recordingNativeSessionSkillRegistry) ForAgentDefSession(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
	agent aghconfig.AgentDef,
	_ string,
) ([]*skills.Skill, error) {
	r.concreteNames = append(r.concreteNames, agent.Name)
	return nil, nil
}

func requireNativeStructuredContains(t *testing.T, result toolspkg.ToolResult, needle []byte) {
	t.Helper()

	if !bytes.Contains(result.Structured, needle) {
		t.Fatalf("structured result = %s, want to contain %s", result.Structured, needle)
	}
}

func requireNativeStructuredExcludes(t *testing.T, result toolspkg.ToolResult, needle []byte) {
	t.Helper()

	if bytes.Contains(result.Structured, needle) {
		t.Fatalf("structured result = %s, want to exclude %s", result.Structured, needle)
	}
}

func nativeToolViewByID(views []toolspkg.ToolView, id toolspkg.ToolID) *toolspkg.ToolView {
	for i := range views {
		if views[i].Descriptor.ID == id {
			return &views[i]
		}
	}
	return nil
}

func nativeDescriptorMap(descriptors []toolspkg.Descriptor) map[toolspkg.ToolID]toolspkg.Descriptor {
	values := make(map[toolspkg.ToolID]toolspkg.Descriptor, len(descriptors))
	for _, descriptor := range descriptors {
		values[descriptor.ID] = descriptor
	}
	return values
}

func requireNativeToolAvailable(t *testing.T, views []toolspkg.ToolView, id toolspkg.ToolID) {
	t.Helper()

	view := nativeToolViewByID(views, id)
	if view == nil {
		t.Fatalf("projection does not contain %q: %#v", id, views)
	}
	if !view.Availability.Available {
		t.Fatalf("%s availability = %#v, want available", id, view.Availability)
	}
}

func requireNativeToolUnavailableReason(t *testing.T, views []toolspkg.ToolView, id toolspkg.ToolID) {
	t.Helper()

	view := nativeToolViewByID(views, id)
	if view == nil {
		t.Fatalf("projection does not contain %q: %#v", id, views)
	}
	if view.Availability.Available {
		t.Fatalf("%s availability = %#v, want unavailable", id, view.Availability)
	}
	if len(view.Availability.ReasonCodes) == 0 ||
		!slices.Contains(view.Availability.ReasonCodes, toolspkg.ReasonDependencyMissing) {
		t.Fatalf(
			"%s availability reasons = %#v, want dependency_missing",
			id,
			view.Availability.ReasonCodes,
		)
	}
}

func requireNativeViewContains(t *testing.T, views []toolspkg.ToolView, id toolspkg.ToolID) {
	t.Helper()

	for i := range views {
		if views[i].Descriptor.ID == id {
			return
		}
	}
	t.Fatalf("projection does not contain %q: %#v", id, views)
}

func requireNativeViewExcludes(t *testing.T, views []toolspkg.ToolView, id toolspkg.ToolID) {
	t.Helper()

	for i := range views {
		if views[i].Descriptor.ID == id {
			t.Fatalf("projection contains %q: %#v", id, views)
		}
	}
}

func nativeMemoryDocument(name string, description string, typ memcontract.Type, body string) []byte {
	return fmt.Appendf(nil,
		"---\nname: %s\ndescription: %s\ntype: %s\n---\n\n%s",
		name,
		description,
		typ,
		body,
	)
}

func seedNativeMemoryCatalogDocuments(t *testing.T, dir string, count int) {
	t.Helper()

	baseTime := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	for index := range count {
		typ := memcontract.TypeProject
		if index == 0 {
			typ = memcontract.TypeReference
		}
		filename := fmt.Sprintf("memory-%03d.md", index)
		content := nativeMemoryDocument(fmt.Sprintf("Memory %03d", index), "desc", typ, "body")
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", filename, err)
		}
		modTime := baseTime.Add(time.Duration(index) * time.Minute)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("Chtimes(%s) error = %v", filename, err)
		}
	}
}

func requireToolReason(t *testing.T, err error, target error, reason toolspkg.ReasonCode) {
	t.Helper()

	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want %v", err, target)
	}
	got, ok := toolspkg.ReasonOf(err)
	if !ok || got != reason {
		t.Fatalf("ReasonOf(error) = %q/%v, want %q", got, ok, reason)
	}
}

func requireToolCode(t *testing.T, err error, want toolspkg.ErrorCode) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want tool code %s", want)
	}
	var toolErr *toolspkg.ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("error = %T %[1]v, want *tools.ToolError", err)
	}
	if toolErr.Code != want {
		t.Fatalf("tool error code = %s, want %s; error=%v", toolErr.Code, want, err)
	}
}

type nativeHookBindingsStub struct {
	syncCalls int
	err       error
}

func (b *nativeHookBindingsStub) Sync(context.Context) error {
	b.syncCalls++
	return b.err
}

type nativeBundleServiceStub struct {
	catalog         []bundlepkg.CatalogEntry
	activations     []bundlepkg.ActivationPreview
	network         bundlepkg.NetworkSettings
	activateCalls   int
	lastActivate    bundlepkg.ActivateRequest
	activateErr     error
	deactivateCalls int
}

func (s *nativeBundleServiceStub) Catalog(context.Context) ([]bundlepkg.CatalogEntry, error) {
	return append([]bundlepkg.CatalogEntry(nil), s.catalog...), nil
}

func (s *nativeBundleServiceStub) PreviewActivation(
	_ context.Context,
	_ bundlepkg.ActivateRequest,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, errors.New("native bundle stub: preview should not be called")
}

func (s *nativeBundleServiceStub) Activate(
	_ context.Context,
	req bundlepkg.ActivateRequest,
) (bundlepkg.ActivationPreview, error) {
	s.activateCalls++
	s.lastActivate = req
	if s.activateErr != nil {
		return bundlepkg.ActivationPreview{}, s.activateErr
	}
	return bundlepkg.ActivationPreview{
		Activation: bundlepkg.Activation{
			ID:            "act-created",
			ExtensionName: req.ExtensionName,
			BundleName:    req.BundleName,
			ProfileName:   req.ProfileName,
			Scope:         req.Scope,
			WorkspaceID:   req.Workspace,
		},
	}, nil
}

func (s *nativeBundleServiceStub) ListActivations(context.Context) ([]bundlepkg.ActivationPreview, error) {
	return append([]bundlepkg.ActivationPreview(nil), s.activations...), nil
}

func (s *nativeBundleServiceStub) GetActivation(
	_ context.Context,
	id string,
) (bundlepkg.ActivationPreview, error) {
	for _, item := range s.activations {
		if item.Activation.ID == id {
			return item, nil
		}
	}
	return bundlepkg.ActivationPreview{}, bundlepkg.ErrActivationNotFound
}

func (s *nativeBundleServiceStub) UpdateActivation(
	_ context.Context,
	_ bundlepkg.UpdateActivationRequest,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, errors.New("native bundle stub: update should not be called")
}

func (s *nativeBundleServiceStub) Deactivate(context.Context, string) error {
	s.deactivateCalls++
	return nil
}

func (s *nativeBundleServiceStub) NetworkSettings(context.Context) (bundlepkg.NetworkSettings, error) {
	return s.network, nil
}

func (s *nativeBundleServiceStub) ConfirmNetworkRequirement(
	_ context.Context,
	activationID string,
	_ int64,
) (bundlepkg.ActivationPreview, error) {
	for _, item := range s.activations {
		if item.Activation.ID == activationID {
			return item, nil
		}
	}
	return bundlepkg.ActivationPreview{}, bundlepkg.ErrActivationNotFound
}

type nativeResourceServiceStub struct {
	records    []resources.RawRecord
	listCalls  int
	lastFilter resources.ResourceFilter
}

func (s *nativeResourceServiceStub) List(
	_ context.Context,
	filter resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	s.listCalls++
	s.lastFilter = filter
	results := make([]resources.RawRecord, 0, len(s.records))
	for _, record := range s.records {
		if filter.Kind != "" && record.Kind != filter.Kind {
			continue
		}
		if filter.Scope != nil && record.Scope.Normalize() != filter.Scope.Normalize() {
			continue
		}
		results = append(results, record)
	}
	return results, nil
}

func (s *nativeResourceServiceStub) Get(
	_ context.Context,
	kind resources.ResourceKind,
	id string,
) (resources.RawRecord, error) {
	for _, record := range s.records {
		if record.Kind == kind && record.ID == id {
			return record, nil
		}
	}
	return resources.RawRecord{}, resources.ErrNotFound
}

func (s *nativeResourceServiceStub) Put(
	_ context.Context,
	_ resources.RawDraft,
) (resources.RawRecord, error) {
	return resources.RawRecord{}, errors.New("native resource stub: put should not be called")
}

func (s *nativeResourceServiceStub) Delete(
	_ context.Context,
	_ resources.ResourceKind,
	_ string,
	_ int64,
) error {
	return errors.New("native resource stub: delete should not be called")
}

type nativeObserverStub struct {
	catalog               []hookspkg.CatalogEntry
	catalogCall           int
	runs                  []hookspkg.HookRunRecord
	hookRunCalls          int
	lastHookRunQuery      store.HookRunQuery
	events                []hookspkg.EventDescriptor
	eventSummaries        []store.EventSummary
	bridgeHealth          []observe.BridgeInstanceHealth
	bridgeHealthCalls     int
	bridgeEffectiveCalls  int
	bridgeHealthForCalls  int
	bridgeHealthForIDs    []string
	bridgeCatalogReadyErr error
	health                observe.Health
	eventQueryCalls       int
	lastEventQuery        store.EventSummaryQuery
}

func (o *nativeObserverStub) CheckBridgeCatalogReady() error {
	return o.bridgeCatalogReadyErr
}

func (o *nativeObserverStub) QueryEvents(
	_ context.Context,
	query store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	o.eventQueryCalls++
	o.lastEventQuery = query
	results := make([]store.EventSummary, 0, len(o.eventSummaries))
	for _, event := range o.eventSummaries {
		if query.WorkspaceID != "" && event.WorkspaceID != query.WorkspaceID {
			continue
		}
		if query.SessionID != "" && event.SessionID != query.SessionID {
			continue
		}
		if query.AgentName != "" && event.AgentName != query.AgentName {
			continue
		}
		if query.Type != "" && event.Type != query.Type {
			continue
		}
		if !query.Since.IsZero() && event.Timestamp.Before(query.Since) {
			continue
		}
		results = append(results, event)
	}
	if query.Limit > 0 && query.Limit < len(results) {
		return results[:query.Limit], nil
	}
	return results, nil
}

func (o *nativeObserverStub) QueryHookCatalog(
	_ context.Context,
	_ hookspkg.CatalogFilter,
) ([]hookspkg.CatalogEntry, error) {
	o.catalogCall++
	return append([]hookspkg.CatalogEntry(nil), o.catalog...), nil
}

func (o *nativeObserverStub) QueryHookRuns(
	_ context.Context,
	query store.HookRunQuery,
) ([]hookspkg.HookRunRecord, error) {
	o.hookRunCalls++
	o.lastHookRunQuery = query
	return append([]hookspkg.HookRunRecord(nil), o.runs...), nil
}

func (o *nativeObserverStub) QueryHookEvents(
	context.Context,
	hookspkg.EventFilter,
) ([]hookspkg.EventDescriptor, error) {
	if len(o.events) > 0 {
		return append([]hookspkg.EventDescriptor(nil), o.events...), nil
	}
	return hookspkg.AllEventDescriptors(), nil
}

func (o *nativeObserverStub) QueryTokenStats(
	context.Context,
	store.TokenStatsQuery,
) ([]store.TokenStats, error) {
	return nil, nil
}

func (o *nativeObserverStub) QueryBridgeHealth(context.Context) ([]observe.BridgeInstanceHealth, error) {
	o.bridgeHealthCalls++
	return append([]observe.BridgeInstanceHealth(nil), o.bridgeHealth...), nil
}

func (o *nativeObserverStub) QueryBridgeEffectiveStatuses(
	_ context.Context,
	records []bridgepkg.BridgeCatalogRecord,
) (map[string]bridgepkg.BridgeStatus, error) {
	o.bridgeEffectiveCalls++
	statuses := make(map[string]bridgepkg.BridgeStatus, len(records))
	for _, record := range records {
		statuses[record.ID] = record.Status
	}
	for _, item := range o.bridgeHealth {
		statuses[item.BridgeInstanceID] = item.Status
	}
	return statuses, nil
}

func (o *nativeObserverStub) QueryBridgeHealthFor(
	_ context.Context,
	instances []bridgepkg.BridgeInstance,
) ([]observe.BridgeInstanceHealth, error) {
	o.bridgeHealthForCalls++
	requested := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		requested[instance.ID] = struct{}{}
		o.bridgeHealthForIDs = append(o.bridgeHealthForIDs, instance.ID)
	}
	result := make([]observe.BridgeInstanceHealth, 0, len(instances))
	for _, item := range o.bridgeHealth {
		if _, ok := requested[item.BridgeInstanceID]; ok {
			result = append(result, item)
		}
	}
	return result, nil
}

func (o *nativeObserverStub) Health(context.Context) (observe.Health, error) {
	return o.health, nil
}

func (o *nativeObserverStub) QueryTaskDashboard(
	context.Context,
	observe.TaskDashboardQuery,
) (observe.TaskDashboardView, error) {
	return observe.TaskDashboardView{}, nil
}

func (o *nativeObserverStub) QueryTaskInbox(
	context.Context,
	observe.TaskInboxQuery,
	taskpkg.ActorIdentity,
) (observe.TaskInboxView, error) {
	return observe.TaskInboxView{}, nil
}

func (o *nativeObserverStub) QueryObserveOverview(
	context.Context,
	observe.OverviewQuery,
) (observe.OverviewView, error) {
	return observe.OverviewView{}, nil
}

type nativeNetworkStub struct {
	sendErr             error
	sendCalls           int
	lastSend            network.SendRequest
	peers               []network.PeerInfo
	peersCalls          int
	peersWorkspaceID    string
	peersChannel        string
	status              *network.Status
	statusCalls         int
	channels            []network.ChannelInfo
	channelsCalls       int
	channelsWorkspaceID string
	inbox               []network.Envelope
	inboxCalls          int
	inboxSessionID      string
}

type nativeNetworkUsageStub struct {
	report store.NetworkUsageReport
	query  store.NetworkUsageQuery
}

func (s *nativeNetworkUsageStub) GetNetworkUsage(
	_ context.Context,
	query store.NetworkUsageQuery,
) (store.NetworkUsageReport, error) {
	s.query = query
	return s.report, nil
}

func (n *nativeNetworkStub) Send(_ context.Context, req network.SendRequest) (string, error) {
	n.sendCalls++
	n.lastSend = req
	if n.sendErr != nil {
		return "", n.sendErr
	}
	return "msg-1", nil
}

func (n *nativeNetworkStub) ListPeers(
	_ context.Context,
	workspaceID string,
	channel string,
) ([]network.PeerInfo, error) {
	n.peersCalls++
	n.peersWorkspaceID = workspaceID
	n.peersChannel = channel
	return append([]network.PeerInfo(nil), n.peers...), nil
}

func (n *nativeNetworkStub) totalCalls() int {
	return n.sendCalls + n.peersCalls + n.statusCalls + n.channelsCalls + n.inboxCalls
}

func (n *nativeNetworkStub) ListChannels(_ context.Context, workspaceID string) ([]network.ChannelInfo, error) {
	n.channelsCalls++
	n.channelsWorkspaceID = workspaceID
	return append([]network.ChannelInfo(nil), n.channels...), nil
}

func (n *nativeNetworkStub) Status(context.Context) (*network.Status, error) {
	n.statusCalls++
	if n.status != nil {
		status := *n.status
		return &status, nil
	}
	return &network.Status{Enabled: true, Status: network.StatusReady}, nil
}

func (n *nativeNetworkStub) Inbox(_ context.Context, sessionID string) ([]network.Envelope, error) {
	n.inboxCalls++
	n.inboxSessionID = sessionID
	return append([]network.Envelope(nil), n.inbox...), nil
}

func (n *nativeNetworkStub) WaitInbox(context.Context, string, string) ([]network.Envelope, error) {
	return nil, nil
}

type nativeSessionHealthStub struct {
	health heartbeat.SessionHealth
	err    error
}

func (s nativeSessionHealthStub) GetSessionHealth(
	context.Context,
	string,
) (heartbeat.SessionHealth, error) {
	if s.err != nil {
		return heartbeat.SessionHealth{}, s.err
	}
	return s.health, nil
}

type nativeHeartbeatStatusStub struct {
	result heartbeat.StatusResult
	err    error
	calls  int
	last   heartbeat.StatusRequest
}

func (s *nativeHeartbeatStatusStub) Inspect(
	context.Context,
	heartbeat.InspectRequest,
) (heartbeat.InspectResult, error) {
	return heartbeat.InspectResult{}, nil
}

func (s *nativeHeartbeatStatusStub) Status(
	_ context.Context,
	req heartbeat.StatusRequest,
) (heartbeat.StatusResult, error) {
	s.calls++
	s.last = req
	if s.err != nil {
		return heartbeat.StatusResult{}, s.err
	}
	return s.result, nil
}

type nativeHeartbeatWakeStub struct {
	result heartbeat.WakeDecision
	err    error
	calls  int
	last   heartbeat.WakeRequest
}

func (s *nativeHeartbeatWakeStub) Wake(
	_ context.Context,
	req heartbeat.WakeRequest,
) (heartbeat.WakeDecision, error) {
	s.calls++
	s.last = req
	if s.err != nil {
		return heartbeat.WakeDecision{}, s.err
	}
	return s.result, nil
}

type nativeHeartbeatWakeEventStub struct {
	events []heartbeat.WakeEvent
	err    error
}

func (s nativeHeartbeatWakeEventStub) ListHeartbeatWakeEvents(
	context.Context,
	heartbeat.WakeEventListQuery,
) ([]heartbeat.WakeEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]heartbeat.WakeEvent(nil), s.events...), nil
}

var errUnexpectedNativeTaskCall = errors.New("unexpected native task manager call")

type nativeTaskManager struct {
	unsupportedNativeTaskManager
	createCalls             int
	lastCreateSpec          taskpkg.CreateTask
	deleteCalls             int
	lastDeleteID            string
	deleteErr               error
	childCreateCalls        int
	childParentID           string
	childSpec               taskpkg.CreateTask
	childErr                error
	listCalls               int
	lastQuery               taskpkg.Query
	listSummaries           []taskpkg.Summary
	getCalls                int
	lastGetID               string
	getView                 *taskpkg.View
	updateCalls             int
	lastUpdateID            string
	lastPatch               taskpkg.Patch
	updateTask              *taskpkg.Task
	cancelCalls             int
	lastCancelID            string
	lastCancel              taskpkg.CancelTask
	cancelTask              *taskpkg.Task
	blockCalls              int
	lastBlockRequest        taskpkg.BlockRequest
	blockResult             taskpkg.TaskBlock
	blockErr                error
	clearBlockCalls         int
	lastClearBlockTaskID    string
	lastClearBlockID        string
	lastClearBlockNote      string
	clearBlockResult        taskpkg.TaskBlock
	clearBlockErr           error
	listBlockCalls          int
	lastListBlockTaskID     string
	lastListIncludeCleared  bool
	blockList               []taskpkg.TaskBlock
	listBlockErr            error
	recoverCalls            int
	lastRecoverTaskID       string
	lastRecoverNote         string
	recoverTask             *taskpkg.Task
	recoverErr              error
	runListCalls            int
	lastRunListTaskID       string
	lastRunQuery            taskpkg.RunQuery
	runs                    []taskpkg.Run
	claimNextCalls          int
	lastClaimCriteria       taskpkg.ClaimCriteria
	lastClaimActor          taskpkg.ActorContext
	claimResult             *taskpkg.ClaimResult
	claimErr                error
	lookupCalls             int
	lastLookupSessionID     string
	lastLookupRunID         string
	lookupHandle            taskpkg.AutonomyLeaseHandle
	lookupErr               error
	heartbeatCalls          int
	lastHeartbeat           taskpkg.LeaseHeartbeat
	heartbeatErr            error
	completeCalls           int
	lastCompletion          taskpkg.LeaseCompletion
	completeErr             error
	failCalls               int
	lastFailure             taskpkg.LeaseFailure
	failErr                 error
	releaseCalls            int
	lastRelease             taskpkg.LeaseRelease
	releaseErr              error
	lookupReviewCalls       int
	lastReviewSessionID     string
	reviewBinding           taskpkg.RunReviewBinding
	lookupReviewErr         error
	requestReviewCalls      int
	lastRequestReview       taskpkg.RunReviewRequest
	requestReviewResult     taskpkg.RunReview
	requestReviewCreated    bool
	requestReviewErr        error
	getReviewCalls          int
	lastGetReviewID         string
	getReview               taskpkg.RunReview
	getReviewErr            error
	listReviewCalls         int
	lastListReviewQuery     taskpkg.RunReviewQuery
	listReviews             []taskpkg.RunReview
	listReviewErr           error
	recordReviewCalls       int
	lastRecordReview        taskpkg.RecordRunReviewRequest
	recordReviewResult      taskpkg.RunReviewResult
	recordReviewErr         error
	profileGetCalls         int
	profileSetCalls         int
	profileDeleteCalls      int
	lastProfileTaskID       string
	lastSetProfile          taskpkg.ExecutionProfile
	lastDeleteProfileTaskID string
	executionProfile        taskpkg.ExecutionProfile
	profileErr              error
}

func (m *nativeTaskManager) CreateTask(
	_ context.Context,
	spec taskpkg.CreateTask,
	_ taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	m.createCalls++
	m.lastCreateSpec = spec
	return &taskpkg.Task{
		ID:          firstNonEmpty(spec.ID, "task-created"),
		Scope:       spec.Scope,
		WorkspaceID: spec.WorkspaceID,
		Title:       spec.Title,
		Status:      taskpkg.TaskStatusPending,
	}, nil
}

func (m *nativeTaskManager) DeleteTask(
	_ context.Context,
	id string,
	_ taskpkg.ActorContext,
) error {
	m.deleteCalls++
	m.lastDeleteID = id
	return m.deleteErr
}

func (m *nativeTaskManager) UpdateTask(
	_ context.Context,
	id string,
	patch taskpkg.Patch,
	_ taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	m.updateCalls++
	m.lastUpdateID = id
	m.lastPatch = patch
	if m.updateTask != nil {
		return m.updateTask, nil
	}
	return &taskpkg.Task{ID: id, Title: stringValue(patch.Title), Status: taskpkg.TaskStatusPending}, nil
}

func (m *nativeTaskManager) CancelTask(
	_ context.Context,
	id string,
	req taskpkg.CancelTask,
	_ taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	m.cancelCalls++
	m.lastCancelID = id
	m.lastCancel = req
	if m.cancelTask != nil {
		return m.cancelTask, nil
	}
	return &taskpkg.Task{ID: id, Title: "Canceled task", Status: taskpkg.TaskStatusCanceled}, nil
}

func (m *nativeTaskManager) BlockTask(
	_ context.Context,
	req taskpkg.BlockRequest,
	_ taskpkg.ActorContext,
) (taskpkg.TaskBlock, error) {
	m.blockCalls++
	m.lastBlockRequest = req
	if m.blockErr != nil {
		return taskpkg.TaskBlock{}, m.blockErr
	}
	if m.blockResult.ID != "" {
		return m.blockResult, nil
	}
	return taskpkg.TaskBlock{
		ID:     "block-native",
		TaskID: req.TaskID,
		Kind:   req.Kind,
		Reason: req.Reason,
	}, nil
}

func (m *nativeTaskManager) ClearTaskBlock(
	_ context.Context,
	taskID string,
	blockID string,
	note string,
	_ taskpkg.ActorContext,
) (taskpkg.TaskBlock, error) {
	m.clearBlockCalls++
	m.lastClearBlockTaskID = taskID
	m.lastClearBlockID = blockID
	m.lastClearBlockNote = note
	if m.clearBlockErr != nil {
		return taskpkg.TaskBlock{}, m.clearBlockErr
	}
	if m.clearBlockResult.ID != "" {
		return m.clearBlockResult, nil
	}
	return taskpkg.TaskBlock{
		ID:        blockID,
		TaskID:    taskID,
		ClearNote: note,
	}, nil
}

func (m *nativeTaskManager) RecoverTask(
	_ context.Context,
	id string,
	note string,
	_ taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	m.recoverCalls++
	m.lastRecoverTaskID = id
	m.lastRecoverNote = note
	if m.recoverErr != nil {
		return nil, m.recoverErr
	}
	if m.recoverTask != nil {
		return m.recoverTask, nil
	}
	return &taskpkg.Task{ID: id, Status: taskpkg.TaskStatusReady}, nil
}

func (m *nativeTaskManager) ExpireTaskBlocks(
	context.Context,
	time.Time,
	taskpkg.ActorContext,
) (taskpkg.ExpireTaskBlocksResult, error) {
	return taskpkg.ExpireTaskBlocksResult{}, nil
}

func (m *nativeTaskManager) ListTaskBlocks(
	_ context.Context,
	taskID string,
	includeCleared bool,
	_ taskpkg.ActorContext,
) ([]taskpkg.TaskBlock, error) {
	m.listBlockCalls++
	m.lastListBlockTaskID = taskID
	m.lastListIncludeCleared = includeCleared
	if m.listBlockErr != nil {
		return nil, m.listBlockErr
	}
	return append([]taskpkg.TaskBlock(nil), m.blockList...), nil
}

func (m *nativeTaskManager) GetTask(
	_ context.Context,
	id string,
	_ taskpkg.ActorContext,
) (*taskpkg.View, error) {
	m.getCalls++
	m.lastGetID = id
	if m.getView != nil {
		return m.getView, nil
	}
	return &taskpkg.View{
		Summary: taskpkg.Summary{ID: id, Title: "Read task", Status: taskpkg.TaskStatusPending},
		Task:    taskpkg.Task{ID: id, Title: "Read task", Status: taskpkg.TaskStatusPending},
	}, nil
}

func (m *nativeTaskManager) ListTaskRuns(
	_ context.Context,
	taskID string,
	query taskpkg.RunQuery,
	_ taskpkg.ActorContext,
) ([]taskpkg.Run, error) {
	m.runListCalls++
	m.lastRunListTaskID = taskID
	m.lastRunQuery = query
	return append([]taskpkg.Run(nil), m.runs...), nil
}

func (m *nativeTaskManager) ListTasks(
	_ context.Context,
	query taskpkg.Query,
	_ taskpkg.ActorContext,
) ([]taskpkg.Summary, error) {
	m.listCalls++
	m.lastQuery = query
	return append([]taskpkg.Summary(nil), m.listSummaries...), nil
}

func (m *nativeTaskManager) ListTaskCatalog(
	_ context.Context,
	query taskpkg.CatalogQuery,
	_ taskpkg.ActorContext,
) (taskpkg.CatalogPage, error) {
	m.listCalls++
	m.lastQuery = taskpkg.Query{
		Scope:       taskpkg.Scope(query.Scope),
		WorkspaceID: query.WorkspaceID,
		Status:      query.Status,
		Priority:    query.Priority,
		OwnerKind:   query.OwnerKind,
		OwnerRef:    query.OwnerRef,
		Search:      query.Search,
		Limit:       query.Limit,
	}
	return taskpkg.CatalogPage{
		Tasks: append([]taskpkg.Summary(nil), m.listSummaries...),
		Total: len(m.listSummaries),
		Limit: query.Limit,
	}, nil
}

func (m *nativeTaskManager) GetExecutionProfile(
	_ context.Context,
	taskID string,
	_ taskpkg.ActorContext,
) (taskpkg.ExecutionProfile, error) {
	m.profileGetCalls++
	m.lastProfileTaskID = taskID
	if m.profileErr != nil {
		return taskpkg.ExecutionProfile{}, m.profileErr
	}
	if m.executionProfile.TaskID != "" {
		return m.executionProfile, nil
	}
	return taskpkg.ExecutionProfile{TaskID: taskID}, nil
}

func (m *nativeTaskManager) SetExecutionProfile(
	_ context.Context,
	taskID string,
	profile *taskpkg.ExecutionProfile,
	_ taskpkg.ActorContext,
) (taskpkg.ExecutionProfile, error) {
	m.profileSetCalls++
	if profile != nil {
		m.lastSetProfile = *profile
	}
	if m.profileErr != nil {
		return taskpkg.ExecutionProfile{}, m.profileErr
	}
	if m.lastSetProfile.TaskID == "" {
		m.lastSetProfile.TaskID = taskID
	}
	m.executionProfile = m.lastSetProfile
	return m.lastSetProfile, nil
}

func (m *nativeTaskManager) DeleteExecutionProfile(
	_ context.Context,
	taskID string,
	_ taskpkg.ActorContext,
) error {
	m.profileDeleteCalls++
	m.lastDeleteProfileTaskID = taskID
	if m.profileErr != nil {
		return m.profileErr
	}
	m.executionProfile = taskpkg.ExecutionProfile{}
	return nil
}

func (m *nativeTaskManager) ClaimNextRun(
	_ context.Context,
	criteria taskpkg.ClaimCriteria,
	actor taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	m.claimNextCalls++
	m.lastClaimCriteria = criteria
	m.lastClaimActor = actor
	if m.claimErr != nil {
		return nil, m.claimErr
	}
	if m.claimResult != nil {
		result := *m.claimResult
		return &result, nil
	}
	return nil, taskpkg.ErrNoClaimableRun
}

func (m *nativeTaskManager) LookupActiveRunForSession(
	_ context.Context,
	sessionID string,
	runID string,
) (taskpkg.AutonomyLeaseHandle, error) {
	m.lookupCalls++
	m.lastLookupSessionID = sessionID
	m.lastLookupRunID = runID
	if m.lookupErr != nil {
		return taskpkg.AutonomyLeaseHandle{}, m.lookupErr
	}
	return m.lookupHandle, nil
}

func (m *nativeTaskManager) HeartbeatRunLease(
	_ context.Context,
	heartbeat taskpkg.LeaseHeartbeat,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	m.heartbeatCalls++
	m.lastHeartbeat = heartbeat
	if m.heartbeatErr != nil {
		return nil, m.heartbeatErr
	}
	run := nativeLeaseRun(heartbeat.RunID, taskpkg.TaskRunStatusClaimed, m.lookupHandle)
	run.LeaseUntil = time.Now().UTC().Add(heartbeat.LeaseDuration)
	return &run, nil
}

func (m *nativeTaskManager) CompleteRunLease(
	_ context.Context,
	completion taskpkg.LeaseCompletion,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	m.completeCalls++
	m.lastCompletion = completion
	if m.completeErr != nil {
		return nil, m.completeErr
	}
	run := nativeLeaseRun(completion.RunID, taskpkg.TaskRunStatusCompleted, m.lookupHandle)
	return &run, nil
}

func (m *nativeTaskManager) FailRunLease(
	_ context.Context,
	failure taskpkg.LeaseFailure,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	m.failCalls++
	m.lastFailure = failure
	if m.failErr != nil {
		return nil, m.failErr
	}
	run := nativeLeaseRun(failure.RunID, taskpkg.TaskRunStatusFailed, m.lookupHandle)
	return &run, nil
}

func (m *nativeTaskManager) ReleaseRunLease(
	_ context.Context,
	release taskpkg.LeaseRelease,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	m.releaseCalls++
	m.lastRelease = release
	if m.releaseErr != nil {
		return nil, m.releaseErr
	}
	run := nativeLeaseRun(release.RunID, taskpkg.TaskRunStatusQueued, m.lookupHandle)
	return &run, nil
}

func (m *nativeTaskManager) LookupRunReviewForSession(
	_ context.Context,
	sessionID string,
	_ taskpkg.ActorContext,
) (taskpkg.RunReviewBinding, error) {
	m.lookupReviewCalls++
	m.lastReviewSessionID = sessionID
	if m.lookupReviewErr != nil {
		return taskpkg.RunReviewBinding{}, m.lookupReviewErr
	}
	if m.reviewBinding.Review.ReviewID == "" {
		return taskpkg.RunReviewBinding{}, taskpkg.ErrRunReviewNotFound
	}
	return m.reviewBinding, nil
}

func (m *nativeTaskManager) RequestRunReview(
	_ context.Context,
	req taskpkg.RunReviewRequest,
	_ taskpkg.ActorContext,
) (taskpkg.RunReview, bool, error) {
	m.requestReviewCalls++
	m.lastRequestReview = req
	if m.requestReviewErr != nil {
		return taskpkg.RunReview{}, false, m.requestReviewErr
	}
	if m.requestReviewResult.ReviewID != "" {
		return m.requestReviewResult, m.requestReviewCreated, nil
	}
	return taskpkg.RunReview{
		ReviewID:    "review-native",
		TaskID:      req.TaskID,
		RunID:       req.RunID,
		Policy:      req.Policy,
		ReviewRound: req.ReviewRound,
		Attempt:     req.Attempt,
		Status:      taskpkg.RunReviewStatusRequested,
		Reason:      req.Reason,
	}, true, nil
}

func (m *nativeTaskManager) GetRunReview(
	_ context.Context,
	reviewID string,
	_ taskpkg.ActorContext,
) (taskpkg.RunReview, error) {
	m.getReviewCalls++
	m.lastGetReviewID = reviewID
	if m.getReviewErr != nil {
		return taskpkg.RunReview{}, m.getReviewErr
	}
	if m.getReview.ReviewID != "" {
		return m.getReview, nil
	}
	return taskpkg.RunReview{ReviewID: reviewID, Status: taskpkg.RunReviewStatusRequested}, nil
}

func (m *nativeTaskManager) ListRunReviews(
	_ context.Context,
	query taskpkg.RunReviewQuery,
	_ taskpkg.ActorContext,
) ([]taskpkg.RunReview, error) {
	m.listReviewCalls++
	m.lastListReviewQuery = query
	if m.listReviewErr != nil {
		return nil, m.listReviewErr
	}
	return append([]taskpkg.RunReview(nil), m.listReviews...), nil
}

func (m *nativeTaskManager) RecordRunReview(
	_ context.Context,
	req taskpkg.RecordRunReviewRequest,
	_ taskpkg.ActorContext,
) (taskpkg.RunReviewResult, error) {
	m.recordReviewCalls++
	m.lastRecordReview = req
	if m.recordReviewErr != nil {
		return taskpkg.RunReviewResult{}, m.recordReviewErr
	}
	if m.recordReviewResult.Review.ReviewID != "" {
		return m.recordReviewResult, nil
	}
	review := m.reviewBinding.Review
	review.Status = taskpkg.RunReviewStatusRecorded
	review.Outcome = req.Verdict.Outcome
	review.Confidence = req.Verdict.Confidence
	review.Reason = req.Verdict.Reason
	review.DeliveryID = req.Verdict.DeliveryID
	review.MissingWork = cloneJSON(req.Verdict.MissingWork)
	review.NextRoundGuidance = req.Verdict.NextRoundGuidance
	review.ReviewText = req.Verdict.ReviewText
	return taskpkg.RunReviewResult{Review: review}, nil
}

func (m *nativeTaskManager) totalCalls() int {
	return m.createCalls +
		m.childCreateCalls +
		m.listCalls +
		m.getCalls +
		m.updateCalls +
		m.cancelCalls +
		m.runListCalls +
		m.profileGetCalls +
		m.profileSetCalls +
		m.profileDeleteCalls +
		m.claimNextCalls +
		m.lookupCalls +
		m.heartbeatCalls +
		m.completeCalls +
		m.failCalls +
		m.releaseCalls +
		m.lookupReviewCalls +
		m.requestReviewCalls +
		m.getReviewCalls +
		m.listReviewCalls +
		m.recordReviewCalls
}

func nativeLeaseRun(
	runID string,
	status taskpkg.RunStatus,
	handle taskpkg.AutonomyLeaseHandle,
) taskpkg.Run {
	return taskpkg.Run{
		ID:              runID,
		TaskID:          firstNonEmpty(handle.TaskID, "task-1"),
		Status:          status,
		SessionID:       handle.SessionID,
		ClaimTokenHash:  handle.ClaimTokenHash,
		RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: daemonTestLiveParticipation("ws-1", "builders")},
		LeaseUntil:      handle.LeaseUntil,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (m *nativeTaskManager) CreateChildTask(
	_ context.Context,
	parentTaskID string,
	spec taskpkg.CreateTask,
	_ taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	m.childCreateCalls++
	m.childParentID = parentTaskID
	m.childSpec = spec
	if m.childErr != nil {
		return nil, m.childErr
	}
	return &taskpkg.Task{
		ID:           firstNonEmpty(spec.ID, "task-child-created"),
		Scope:        spec.Scope,
		WorkspaceID:  spec.WorkspaceID,
		ParentTaskID: parentTaskID,
		Title:        spec.Title,
		Status:       taskpkg.TaskStatusPending,
	}, nil
}

type unsupportedNativeTaskManager struct{}

func (unsupportedNativeTaskManager) CreateTask(
	context.Context,
	taskpkg.CreateTask,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) CreateChildTask(
	context.Context,
	string,
	taskpkg.CreateTask,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) DeleteTask(context.Context, string, taskpkg.ActorContext) error {
	return errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) UpdateTask(
	context.Context,
	string,
	taskpkg.Patch,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) PublishTask(
	context.Context,
	string,
	taskpkg.ExecutionRequest,
	taskpkg.ActorContext,
) (*taskpkg.Execution, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) StartTask(
	context.Context,
	string,
	taskpkg.ExecutionRequest,
	taskpkg.ActorContext,
) (*taskpkg.Execution, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ApproveTask(
	context.Context,
	string,
	taskpkg.ExecutionRequest,
	taskpkg.ActorContext,
) (*taskpkg.Execution, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RejectTask(
	context.Context,
	string,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) CancelTask(
	context.Context,
	string,
	taskpkg.CancelTask,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) PauseTask(
	context.Context,
	string,
	taskpkg.PauseTaskRequest,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ResumeTask(
	context.Context,
	string,
	taskpkg.ResumeTaskRequest,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) MarkTaskRead(
	context.Context,
	string,
	taskpkg.ActorContext,
) (taskpkg.TriageState, error) {
	return taskpkg.TriageState{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ArchiveTask(
	context.Context,
	string,
	taskpkg.ActorContext,
) (taskpkg.TriageState, error) {
	return taskpkg.TriageState{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) DismissTask(
	context.Context,
	string,
	taskpkg.ActorContext,
) (taskpkg.TriageState, error) {
	return taskpkg.TriageState{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) GetExecutionProfile(
	context.Context,
	string,
	taskpkg.ActorContext,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) SetExecutionProfile(
	context.Context,
	string,
	*taskpkg.ExecutionProfile,
	taskpkg.ActorContext,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) DeleteExecutionProfile(
	context.Context,
	string,
	taskpkg.ActorContext,
) error {
	return errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RequestRunReview(
	context.Context,
	taskpkg.RunReviewRequest,
	taskpkg.ActorContext,
) (taskpkg.RunReview, bool, error) {
	return taskpkg.RunReview{}, false, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) GetRunReview(
	context.Context,
	string,
	taskpkg.ActorContext,
) (taskpkg.RunReview, error) {
	return taskpkg.RunReview{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RecordRunReview(
	context.Context,
	taskpkg.RecordRunReviewRequest,
	taskpkg.ActorContext,
) (taskpkg.RunReviewResult, error) {
	return taskpkg.RunReviewResult{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) BindRunReviewSession(
	context.Context,
	taskpkg.BindRunReviewSessionRequest,
	taskpkg.ActorContext,
) (taskpkg.RunReviewBinding, error) {
	return taskpkg.RunReviewBinding{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) LookupRunReviewForSession(
	context.Context,
	string,
	taskpkg.ActorContext,
) (taskpkg.RunReviewBinding, error) {
	return taskpkg.RunReviewBinding{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ListRunReviews(
	context.Context,
	taskpkg.RunReviewQuery,
	taskpkg.ActorContext,
) ([]taskpkg.RunReview, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) AddDependency(
	context.Context,
	taskpkg.AddDependency,
	taskpkg.ActorContext,
) error {
	return errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RemoveDependency(
	context.Context,
	string,
	string,
	taskpkg.ActorContext,
) error {
	return errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) EnqueueRun(
	context.Context,
	taskpkg.EnqueueRun,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ClaimNextRun(
	context.Context,
	taskpkg.ClaimCriteria,
	taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) StartRun(
	context.Context,
	string,
	taskpkg.StartRun,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) AttachRunSession(
	context.Context,
	string,
	string,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) HeartbeatRunLease(
	context.Context,
	taskpkg.LeaseHeartbeat,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ReleaseRunLease(
	context.Context,
	taskpkg.LeaseRelease,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ForceReleaseRun(
	context.Context,
	string,
	taskpkg.ForceReleaseRun,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ForceFailRun(
	context.Context,
	string,
	taskpkg.ForceFailRun,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RetryRun(
	context.Context,
	string,
	taskpkg.RetryRunRequest,
	taskpkg.ActorContext,
) (*taskpkg.RetryRunResult, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RecoverRun(
	context.Context,
	string,
	taskpkg.RecoverRunRequest,
	taskpkg.ActorContext,
) (*taskpkg.RetryRunResult, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) BulkForceReleaseRuns(
	context.Context,
	taskpkg.BulkForceRunRequest,
	taskpkg.ActorContext,
) (taskpkg.BulkForceRunResult, error) {
	return taskpkg.BulkForceRunResult{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) BulkForceFailRuns(
	context.Context,
	taskpkg.BulkForceRunRequest,
	taskpkg.ActorContext,
) (taskpkg.BulkForceRunResult, error) {
	return taskpkg.BulkForceRunResult{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) SchedulerStatus(
	context.Context,
	taskpkg.ActorContext,
) (taskpkg.SchedulerStatus, error) {
	return taskpkg.SchedulerStatus{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) PauseScheduler(
	context.Context,
	taskpkg.SchedulerPauseRequest,
	taskpkg.ActorContext,
) (taskpkg.SchedulerStatus, error) {
	return taskpkg.SchedulerStatus{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ResumeScheduler(
	context.Context,
	taskpkg.SchedulerResumeRequest,
	taskpkg.ActorContext,
) (taskpkg.SchedulerStatus, error) {
	return taskpkg.SchedulerStatus{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) DrainScheduler(
	context.Context,
	taskpkg.SchedulerDrainRequest,
	taskpkg.ActorContext,
) (taskpkg.SchedulerDrainResult, error) {
	return taskpkg.SchedulerDrainResult{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) SchedulerBacklog(
	context.Context,
	taskpkg.SchedulerBacklogQuery,
	taskpkg.ActorContext,
) (taskpkg.SchedulerBacklog, error) {
	return taskpkg.SchedulerBacklog{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) CompleteRunLease(
	context.Context,
	taskpkg.LeaseCompletion,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) FailRunLease(
	context.Context,
	taskpkg.LeaseFailure,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) SettleNetworkWake(
	context.Context,
	taskpkg.NetworkWakeSettlement,
	taskpkg.ActorContext,
) (*taskpkg.NetworkWakeSettlementResult, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) CompleteRun(
	context.Context,
	string,
	taskpkg.RunResult,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) FailRun(
	context.Context,
	string,
	taskpkg.RunFailure,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) CancelRun(
	context.Context,
	string,
	taskpkg.CancelRun,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RecoverExpiredRunLeases(
	context.Context,
	taskpkg.ExpiredLeaseRecovery,
	taskpkg.ActorContext,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RecoverTask(
	context.Context,
	string,
	string,
	taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ExpireTaskBlocks(
	context.Context,
	time.Time,
	taskpkg.ActorContext,
) (taskpkg.ExpireTaskBlocksResult, error) {
	return taskpkg.ExpireTaskBlocksResult{}, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ListTaskBlocks(
	context.Context,
	string,
	bool,
	taskpkg.ActorContext,
) ([]taskpkg.TaskBlock, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) GetTask(
	context.Context,
	string,
	taskpkg.ActorContext,
) (*taskpkg.View, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) InspectTask(
	context.Context,
	string,
	taskpkg.ActorContext,
) (*taskpkg.InspectView, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) InspectRun(
	context.Context,
	string,
	taskpkg.ActorContext,
) (*taskpkg.InspectView, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ListTaskRuns(
	context.Context,
	string,
	taskpkg.RunQuery,
	taskpkg.ActorContext,
) ([]taskpkg.Run, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) ListTasks(
	context.Context,
	taskpkg.Query,
	taskpkg.ActorContext,
) ([]taskpkg.Summary, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) Timeline(
	context.Context,
	string,
	taskpkg.TimelineQuery,
	taskpkg.ActorContext,
) ([]taskpkg.TimelineItem, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) Stream(
	context.Context,
	string,
	taskpkg.StreamQuery,
	taskpkg.ActorContext,
) (<-chan taskpkg.StreamEvent, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) Tree(
	context.Context,
	string,
	taskpkg.ActorContext,
) (*taskpkg.TreeView, error) {
	return nil, errUnexpectedNativeTaskCall
}

func (unsupportedNativeTaskManager) RunDetail(
	context.Context,
	string,
	taskpkg.ActorContext,
) (*taskpkg.RunDetailView, error) {
	return nil, errUnexpectedNativeTaskCall
}
