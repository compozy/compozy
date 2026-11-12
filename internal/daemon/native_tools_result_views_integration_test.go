//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestNativeResultViewsIntegration(t *testing.T) {
	t.Run("Should return both views through real session lifecycle and SQLite", func(t *testing.T) {
		home := integrationHomePaths(t)
		cfg := testConfig(t, home)
		workspace := newHarnessIntegrationWorkspace(t, home, cfg, filepath.Join(home.HomeDir, "workspace"))
		workspace.Agents[0].Name = "general"
		workspace.Agents[0].Tools = nativeResultViewPermissions().Tools
		daemon, deps := bootHarnessPolicyDaemon(t, home, &cfg)
		t.Cleanup(func() {
			if err := daemon.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := daemon.registry.InsertWorkspace(t.Context(), workspace.Workspace); err != nil {
			t.Fatal(err)
		}
		manager := newHarnessIntegrationManager(
			t,
			home,
			deps,
			workspace,
			newHarnessIntegrationDriver(),
			session.WithHostedMCPLauncher(deps.HostedMCP),
			session.WithSessionCatalog(deps.SessionCatalog),
		)
		t.Cleanup(func() {
			if err := manager.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		parent, err := manager.Create(
			t.Context(),
			&session.CreateOpts{AgentName: "general", Workspace: workspace.ID, ProfileID: store.DefaultProfileID},
		)
		if err != nil {
			t.Fatal(err)
		}
		// Invariant: native status hydrates pending watches from SQLite before projecting either view.
		db := daemon.registry.(*globaldb.GlobalDB)
		watchStore := daemon.registry.(store.ReplyWatchStore)
		replies, err := session.NewReplyWatchService(watchStore, manager)
		if err != nil {
			t.Fatal(err)
		}
		manager.SetReplyWatchService(replies)
		for _, state := range []string{"armed", "fired", "delivered", "abandoned"} {
			if _, err := db.DB().ExecContext(t.Context(), `INSERT INTO session_prompt_reply_watches
				(id, workspace_id, sender_session_id, target_workspace_id, target_session_id,
				message_id, admission_id, hop, state, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`, "rw-"+state, workspace.ID, parent.ID,
				workspace.ID, "target-"+state, "message-"+state, "admission-"+state, state,
				store.FormatTimestamp(time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC))); err != nil {
				t.Fatal(err)
			}
		}
		registry := newDaemonNativeRegistry(
			t,
			&daemonNativeToolsDeps{
				Sessions:   manager,
				Workspaces: nativeTestWorkspaceServiceWithRoot(t, workspace.RootDir),
			},
			nativeApproveAllPolicyInputs(),
		)
		scope := toolspkg.Scope{
			ProfileID:   store.DefaultProfileID,
			SessionID:   parent.ID,
			WorkspaceID: workspace.ID,
			AgentName:   "general",
		}
		for _, view := range []toolspkg.ResultView{toolspkg.ResultViewCompact, toolspkg.ResultViewFull} {
			createInput := fmt.Sprintf(`{"agent":"general","result_view":%q}`, view)
			created := callNativeResultViewIntegration(t, registry, scope, toolspkg.ToolIDSessionCreate, createInput)
			var record struct {
				Session struct {
					ID string `json:"id"`
				} `json:"session"`
			}
			if err := json.Unmarshal(created.Structured, &record); err != nil {
				t.Fatal(err)
			}
			if record.Session.ID == "" {
				t.Fatal("created session has no identity")
			}
			info, err := manager.Status(t.Context(), record.Session.ID)
			if err != nil {
				t.Fatal(err)
			}
			if info.RuntimeStatus != session.RuntimeStatusUnbound {
				t.Fatalf("created runtime=%s", info.RuntimeStatus)
			}
			status := callNativeResultViewIntegration(
				t,
				registry,
				scope,
				toolspkg.ToolIDSessionStatus,
				fmt.Sprintf(`{"session_id":%q,"result_view":%q}`, parent.ID, view),
			)
			var statusPayload struct {
				Session contract.SessionPayload `json:"session"`
			}
			if err := json.Unmarshal(status.Structured, &statusPayload); err != nil {
				t.Fatal(err)
			}
			watches := statusPayload.Session.ReplyWatches
			if len(watches) != 2 || watches[0].ID != "rw-armed" || watches[0].State != "armed" ||
				watches[1].ID != "rw-fired" || watches[1].State != "fired" ||
				watches[0].TargetSessionID != "target-armed" || watches[0].MessageID != "message-armed" {
				t.Fatalf("native status %s pending watches: %+v", view, watches)
			}
			t.Logf("real status %s bytes=%d", view, len(status.Structured))
			spawn := callNativeResultViewIntegration(
				t,
				registry,
				scope,
				toolspkg.ToolIDSessionSpawn,
				fmt.Sprintf(
					`{"agent_name":"general","ttl_seconds":600,"inherit_permissions":true,"result_view":%q}`,
					view,
				),
			)
			var child struct {
				SessionID string `json:"session_id"`
			}
			if err := json.Unmarshal(spawn.Structured, &child); err != nil {
				t.Fatal(err)
			}
			if child.SessionID == "" {
				t.Fatal("spawn returned no child")
			}
			if err := manager.Stop(t.Context(), child.SessionID); err != nil {
				t.Fatal(err)
			}
			archived := callNativeResultViewIntegration(
				t,
				registry,
				scope,
				toolspkg.ToolIDSessionArchive,
				fmt.Sprintf(`{"session_id":%q,"result_view":%q}`, child.SessionID, view),
			)
			info, err = manager.Status(t.Context(), child.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			if info.ArchivedAt == nil {
				t.Fatal("archive did not persist")
			}
			t.Logf(
				"real lifecycle %s create=%d spawn=%d archive=%d",
				view,
				len(created.Structured),
				len(spawn.Structured),
				len(archived.Structured),
			)
		}
	})
}

func callNativeResultViewIntegration(
	t *testing.T,
	registry *toolspkg.RuntimeRegistry,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	input string,
) toolspkg.ToolResult {
	t.Helper()
	result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{ToolID: id, Input: json.RawMessage(input)})
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	return result
}
