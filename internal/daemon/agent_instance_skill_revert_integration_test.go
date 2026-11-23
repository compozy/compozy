//go:build integration

package daemon

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// Invariant (IT-010, IT-016, IT-018): the revert route restores an edit and removes a new skill over
// HTTP and UDS, the detail's `revert` field equals the outcome of POST …/revert (allowed, superseded,
// changed_on_disk), and the next session's catalog reflects the restored contents.
// Owning layer: daemon routes over real HTTP/UDS, SQLite, disk and the skill catalog.
func TestInstanceSkillRevertIntegration(t *testing.T) {
	t.Run("Should revert an edit and a new skill with eligibility parity", func(t *testing.T) {
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
		if _, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: main.AgentName, WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main.ID, 10); err != nil {
			t.Fatal(err)
		}
		var rootDir string
		if err := db.DB().QueryRowContext(t.Context(), "SELECT root_dir FROM workspaces WHERE id = ?", workspace).
			Scan(&rootDir); err != nil {
			t.Fatal(err)
		}
		skillFile := filepath.Join(rootDir, compozyconfig.DirName, compozyconfig.SkillsDirName, "release-checklist",
			"SKILL.md")
		scope := toolspkg.Scope{
			SessionID: main.ID, WorkspaceID: workspace, ProfileID: store.DefaultProfileID, AgentName: main.AgentName,
		}
		propose := func(mode, description string) contract.AgentSkillReviewPayload {
			t.Helper()
			result, err := d.toolRegistry.Call(t.Context(), scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentSkillPropose,
				Input: []byte(`{"name":"release-checklist","mode":"` + mode + `","description":"` + description +
					`","body":"Check the changelog before tagging.",` +
					`"lesson":{"cue":"Tagging a release","lesson":"Check the changelog first.",` +
					`"evidence":"The main session missed it.","confidence":"high"},` +
					`"evidence_session_ids":["` + main.ID + `"]}`),
				ApprovalGranted: true,
				ToolCallID:      "propose-" + mode,
			})
			if err != nil {
				t.Fatal(err)
			}
			var proposed contract.AgentSkillReviewResponse
			if err := json.Unmarshal(result.Structured, &proposed); err != nil {
				t.Fatal(err)
			}
			return proposed.Review
		}
		uds := gatewayDaemonUDSClient(d.homePaths.DaemonSocket)
		t.Cleanup(uds.CloseIdleConnections)
		httpBase := fmt.Sprintf("http://127.0.0.1:%d", d.info.Port)
		call := func(client *http.Client, base, method, path string, body any) (int, []byte) {
			t.Helper()
			var reader io.Reader
			if body != nil {
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				reader = bytes.NewReader(encoded)
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
		decide := func(client *http.Client, base string, review contract.AgentSkillReviewPayload,
			action string,
		) (int, []byte) {
			t.Helper()
			return call(client, base, http.MethodPost, "/api/agent-skill-reviews/"+review.ID+"/"+action,
				contract.AgentSkillReviewDecisionRequest{Digest: review.Digest})
		}
		eligibility := func(review contract.AgentSkillReviewPayload) contract.AgentSkillRevertEligibility {
			t.Helper()
			status, raw := call(
				http.DefaultClient,
				httpBase,
				http.MethodGet,
				"/api/agent-skill-reviews/"+review.ID,
				nil,
			)
			var detail contract.AgentSkillReviewResponse
			if status != http.StatusOK || json.Unmarshal(raw, &detail) != nil || detail.Review.Revert == nil {
				t.Fatalf("detail of %s = %d %s", review.ID, status, raw)
			}
			return *detail.Review.Revert
		}
		requireRefused := func(review contract.AgentSkillReviewPayload, reason string) {
			t.Helper()
			if got := eligibility(review); got.Allowed || got.Reason != reason {
				t.Fatalf("detail revert of %s = %+v, want %s", review.ID, got, reason)
			}
			status, raw := decide(uds, "http://unix", review, "revert")
			var failure contract.ErrorPayload
			if status != http.StatusConflict || json.Unmarshal(raw, &failure) != nil ||
				failure.Code != "skill_review_revert_conflict" || failure.Reason != reason {
				t.Fatalf("POST revert of %s = %d %s, want 409 %s", review.ID, status, raw, reason)
			}
		}
		catalogDescription := func() string {
			t.Helper()
			fresh, err := manager.Create(
				t.Context(),
				&session.CreateOpts{AgentName: main.AgentName, Workspace: workspace},
			)
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
			for _, entry := range entries {
				if entry.Meta.Name == "release-checklist" {
					return entry.Meta.Description
				}
			}
			return ""
		}

		first := propose("new", "First checklist")
		if status, raw := decide(http.DefaultClient, httpBase, first, "approve"); status != http.StatusOK {
			t.Fatalf("approve new = %d %s", status, raw)
		}
		v1, err := os.ReadFile(skillFile)
		if err != nil {
			t.Fatal(err)
		}
		edit := propose("edit", "Edited checklist")
		if status, raw := decide(http.DefaultClient, httpBase, edit, "approve"); status != http.StatusOK {
			t.Fatalf("approve edit = %d %s", status, raw)
		}
		if got := catalogDescription(); got != "Edited checklist" {
			t.Fatalf("catalog after the edit = %q", got)
		}
		requireRefused(first, agentinstance.SkillRevertSuperseded)
		if got := eligibility(edit); !got.Allowed || got.Reason != "" {
			t.Fatalf("detail revert of the edit = %+v, want allowed", got)
		}

		status, raw := decide(uds, "http://unix", edit, "revert")
		var reverted contract.AgentSkillReviewResponse
		if status != http.StatusOK || json.Unmarshal(raw, &reverted) != nil || reverted.Review.State != "reverted" ||
			reverted.Review.RevertedBy == "" || reverted.Review.RevertedAt == nil {
			t.Fatalf("revert edit = %d %s", status, raw)
		}
		if restored, err := os.ReadFile(skillFile); err != nil || !bytes.Equal(restored, v1) {
			t.Fatalf("restored SKILL.md = %q, %v; want %q", restored, err, v1)
		}
		if got := catalogDescription(); got != "First checklist" {
			t.Fatalf("catalog after reverting the edit = %q, want the previous description", got)
		}
		if status, raw := decide(http.DefaultClient, httpBase, edit, "revert"); status != http.StatusOK {
			t.Fatalf("repeated revert = %d %s, want an idempotent 200", status, raw)
		}

		if err := os.WriteFile(skillFile, append(append([]byte{}, v1...), "\nEdited by hand."...), 0o600); err != nil {
			t.Fatal(err)
		}
		requireRefused(first, agentinstance.SkillRevertChangedOnDisk)
		if err := os.WriteFile(skillFile, v1, 0o600); err != nil {
			t.Fatal(err)
		}
		if got := eligibility(first); !got.Allowed {
			t.Fatalf("detail revert of the restored first approval = %+v, want allowed", got)
		}
		if status, raw := decide(http.DefaultClient, httpBase, first, "revert"); status != http.StatusOK {
			t.Fatalf("revert new = %d %s", status, raw)
		}
		if _, err := os.Stat(filepath.Dir(skillFile)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("reverted new skill still installed: %v", err)
		}
		if got := catalogDescription(); got != "" {
			t.Fatalf("catalog after reverting the new skill lists it: %q", got)
		}
	})
}
