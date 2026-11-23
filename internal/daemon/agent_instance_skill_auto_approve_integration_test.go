//go:build integration

package daemon

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// Invariant (IT-005, IT-006, IT-007, IT-022): the instance PATCH keeps one field per request on HTTP
// and UDS; with the toggle off a native proposal stays pending with toggled_off and writes no install
// log; once an operator turns it on, an eligible native proposal is installed through the service,
// listed with decided_via auto_approve, and visible in the next session's skill catalog.
// Owning layer: daemon routes and native tools over real HTTP/UDS, SQLite, disk and the catalog.
func TestInstanceSkillAutoApproveIntegration(t *testing.T) {
	t.Run("Should gate, install and list automatic approvals end to end", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonWithSetup(t, nil, "", func(d *Daemon) {
			path := filepath.Join(d.homePaths.AgentsDir, "subagent-test", "AGENT.md")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			body = []byte(
				strings.Replace(string(body), "name: subagent-test\n", "name: subagent-test\nkind: always_on\n", 1),
			)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		})
		main, err := manager.Create(t.Context(), &session.CreateOpts{AgentName: "subagent-test", Workspace: workspace})
		if err != nil {
			t.Fatal(err)
		}
		db := d.registry.(*globaldb.GlobalDB)
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: main.AgentName, WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main.ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		var rootDir string
		if err := db.DB().QueryRowContext(t.Context(), "SELECT root_dir FROM workspaces WHERE id = ?", workspace).
			Scan(&rootDir); err != nil {
			t.Fatal(err)
		}
		skillsRoot := filepath.Join(rootDir, compozyconfig.DirName, compozyconfig.SkillsDirName)
		scope := toolspkg.Scope{
			SessionID: main.ID, WorkspaceID: workspace, ProfileID: store.DefaultProfileID, AgentName: main.AgentName,
		}
		propose := func(name, description string) contract.AgentSkillReviewResponse {
			t.Helper()
			result, err := d.toolRegistry.Call(t.Context(), scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentSkillPropose,
				Input: []byte(`{"name":"` + name + `","description":"` + description +
					`","body":"Check the changelog before tagging.",` +
					`"lesson":{"cue":"Tagging a release","lesson":"Check the changelog first.",` +
					`"evidence":"The main session missed it.","confidence":"high"},` +
					`"evidence_session_ids":["` + main.ID + `"]}`),
				ApprovalGranted: true,
				ToolCallID:      "propose-" + name,
			})
			if err != nil {
				t.Fatal(err)
			}
			var proposed contract.AgentSkillReviewResponse
			if err := json.Unmarshal(result.Structured, &proposed); err != nil {
				t.Fatal(err)
			}
			return proposed
		}
		uds := gatewayDaemonUDSClient(d.homePaths.DaemonSocket)
		t.Cleanup(uds.CloseIdleConnections)
		httpBase := fmt.Sprintf("http://127.0.0.1:%d", d.info.Port)
		call := func(client *http.Client, base, method, path, body string) (int, []byte) {
			t.Helper()
			var reader io.Reader
			if body != "" {
				reader = bytes.NewReader([]byte(body))
			}
			req, err := http.NewRequestWithContext(t.Context(), method, base+path, reader)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			response, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			raw, readErr := io.ReadAll(response.Body)
			if err := errors.Join(readErr, response.Body.Close()); err != nil {
				t.Fatal(err)
			}
			return response.StatusCode, raw
		}
		instancePath := "/api/agent-instances/" + instance.ID

		// IT-007: off by default; the proposal stays pending and nothing is installed or logged.
		off := propose("off-checklist", "Off checklist")
		if off.Review.State != "pending" || off.AutoApproveSkipped != agentinstance.SkillAutoApproveToggledOff {
			t.Fatalf("propose with the toggle off = %+v", off)
		}
		if _, err := os.Stat(filepath.Join(skillsRoot, ".skill-installs", "off-checklist.jsonl")); !errors.Is(
			err, os.ErrNotExist) {
			t.Fatalf("install log written with the toggle off: %v", err)
		}

		// IT-022: one field per request on both transports; a single-field budget keeps its response.
		const xorMessage = "exactly one of checks_enabled, daily_budget, or skill_auto_approve is required"
		for _, transport := range []struct {
			client *http.Client
			base   string
		}{{http.DefaultClient, httpBase}, {uds, "http://unix"}} {
			status, raw := call(transport.client, transport.base, http.MethodPatch, instancePath,
				`{"checks_enabled":true,"skill_auto_approve":true}`)
			var failure contract.ErrorPayload
			if status != http.StatusUnprocessableEntity || json.Unmarshal(raw, &failure) != nil ||
				failure.Code != "" || failure.Error != xorMessage {
				t.Fatalf("%s PATCH two fields = %d %s", transport.base, status, raw)
			}
			if status, raw := call(transport.client, transport.base, http.MethodPatch, instancePath,
				`{"daily_budget":{}}`); status != http.StatusOK {
				t.Fatalf("%s PATCH daily_budget = %d %s", transport.base, status, raw)
			}
		}

		// IT-005: the operator turns it on; GET reads it back.
		if status, raw := call(http.DefaultClient, httpBase, http.MethodPatch, instancePath,
			`{"skill_auto_approve":true}`); status != http.StatusOK || !strings.Contains(string(raw),
			`"skill_auto_approve":true`) {
			t.Fatalf("PATCH on = %d %s", status, raw)
		}
		if status, raw := call(uds, "http://unix", http.MethodGet, instancePath, ""); status != http.StatusOK ||
			!strings.Contains(string(raw), `"skill_auto_approve":true`) {
			t.Fatalf("GET after on = %d %s", status, raw)
		}

		// IT-006: an eligible native proposal installs at once and reaches the catalog.
		approved := propose("release-checklist", "Automatic checklist")
		if approved.Review.State != "approved" || approved.Review.DecidedVia != "auto_approve" ||
			approved.AutoApproveSkipped != "" {
			t.Fatalf("propose with the toggle on = %+v", approved)
		}
		status, raw := call(http.DefaultClient, httpBase, http.MethodGet,
			"/api/agent-skill-reviews?workspace="+url.QueryEscape(workspace)+"&decided_via=auto_approve", "")
		var listed contract.AgentSkillReviewsResponse
		if status != http.StatusOK || json.Unmarshal(raw, &listed) != nil || len(listed.Reviews) != 1 ||
			listed.Reviews[0].ID != approved.Review.ID {
			t.Fatalf("list decided_via=auto_approve = %d %s", status, raw)
		}
		fresh, err := manager.Create(t.Context(), &session.CreateOpts{AgentName: main.AgentName, Workspace: workspace})
		if err != nil {
			t.Fatal(err)
		}
		events, err := manager.Prompt(t.Context(), fresh.ID, "child work")
		if err != nil {
			t.Fatal(err)
		}
		for event := range events {
			if event.Type == acp.EventTypeError || event.ToolError() {
				t.Fatalf("ACP prompt: %+v", event)
			}
		}
		resolved, err := d.workspaceResolver.Resolve(t.Context(), workspace)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := d.skillsRegistry.ForAgentSession(t.Context(), &resolved, main.AgentName, fresh.ID)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, entry := range entries {
			found = found || (entry.Meta.Name == "release-checklist" && entry.Meta.Description == "Automatic checklist")
		}
		if !found {
			t.Fatal("the automatically approved skill is missing from the next session's catalog")
		}
	})
}
