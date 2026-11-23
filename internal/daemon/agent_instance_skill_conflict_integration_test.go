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
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Invariant (IT-019): a retained install conflict left on disk is listed and resolved by the
// operator over HTTP and UDS, agents are refused, the conflict list is empty afterwards, a repeat
// is answered from the install log, and the catalog of a new session shows the restored version.
// Owning layer: daemon routes over real HTTP/UDS, SQLite and disk.
func TestInstanceSkillInstallConflictIntegration(t *testing.T) {
	t.Run("Should list and resolve a retained conflict as the operator only", func(t *testing.T) {
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
		skillsRoot := filepath.Join(rootDir, compozyconfig.DirName, compozyconfig.SkillsDirName)
		opID := seedSkillInstallConflict(t, skillsRoot)

		uds := gatewayDaemonUDSClient(d.homePaths.DaemonSocket)
		t.Cleanup(uds.CloseIdleConnections)
		transports := []struct {
			base   string
			client *http.Client
		}{
			{fmt.Sprintf("http://127.0.0.1:%d", d.info.Port), http.DefaultClient},
			{"http://unix", uds},
		}
		call := func(base string, client *http.Client, method, path string, body any,
			headers map[string]string,
		) (int, []byte) {
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
			for key, value := range headers {
				req.Header.Set(key, value)
			}
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
		list := "/api/agent-skill-install-conflicts?workspace=" + workspace
		resolve := "/api/agent-skill-install-conflicts/" + opID + "/resolve?workspace=" + workspace
		for _, transport := range transports {
			status, raw := call(transport.base, transport.client, http.MethodGet, list, nil, nil)
			var conflicts contract.AgentSkillInstallConflictsResponse
			if status != http.StatusOK || json.Unmarshal(raw, &conflicts) != nil || len(conflicts.Conflicts) != 1 ||
				conflicts.Conflicts[0].OpID != opID || conflicts.Conflicts[0].Skill != "release-checklist" ||
				conflicts.Conflicts[0].State != "conflict" {
				t.Fatalf("list via %s = %d %s", transport.base, status, raw)
			}
		}
		agent := map[string]string{
			agentidentity.HeaderSessionID: main.ID, agentidentity.HeaderAgent: main.AgentName,
		}
		status, raw := call(transports[1].base, transports[1].client, http.MethodPost, resolve,
			contract.AgentSkillInstallConflictResolveRequest{Keep: "previous"}, agent)
		if status != http.StatusForbidden || !strings.Contains(string(raw), "skill_review_operator_only") {
			t.Fatalf("agent resolve = %d %s, want 403 skill_review_operator_only", status, raw)
		}
		status, raw = call(transports[0].base, transports[0].client, http.MethodPost, resolve,
			contract.AgentSkillInstallConflictResolveRequest{Keep: "previous"}, nil)
		var resolved contract.AgentSkillInstallConflictResolveResponse
		if status != http.StatusOK || json.Unmarshal(raw, &resolved) != nil || resolved.AlreadyResolved ||
			filepath.Base(resolved.KeptCopy) != ".skill-conflict-"+opID || resolved.Skill != "release-checklist" {
			t.Fatalf("operator resolve = %d %s", status, raw)
		}
		edited, err := os.ReadFile(filepath.Join(resolved.KeptCopy, "SKILL.md"))
		if err != nil || !strings.Contains(string(edited), "Edited checklist") {
			t.Fatalf("kept copy = %q, %v", edited, err)
		}
		for _, transport := range transports {
			status, raw := call(transport.base, transport.client, http.MethodGet, list, nil, nil)
			if status != http.StatusOK || !strings.Contains(string(raw), `"conflicts":[]`) {
				t.Fatalf("list after resolve via %s = %d %s", transport.base, status, raw)
			}
		}
		status, raw = call(transports[1].base, transports[1].client, http.MethodPost, resolve,
			contract.AgentSkillInstallConflictResolveRequest{Keep: "previous"}, nil)
		if status != http.StatusOK || !strings.Contains(string(raw), `"already_resolved":true`) {
			t.Fatalf("repeated resolve = %d %s", status, raw)
		}
		status, raw = call(transports[0].base, transports[0].client, http.MethodPost,
			"/api/agent-skill-install-conflicts/sop_unknown/resolve?workspace="+workspace,
			contract.AgentSkillInstallConflictResolveRequest{Keep: "previous"}, nil)
		if status != http.StatusNotFound || !strings.Contains(string(raw), "skill_install_conflict_not_found") {
			t.Fatalf("unknown op = %d %s", status, raw)
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
		workspaceRef, err := d.workspaceResolver.Resolve(t.Context(), workspace)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := d.skillsRegistry.ForAgentSession(t.Context(), &workspaceRef, main.AgentName, fresh.ID)
		if err != nil {
			t.Fatal(err)
		}
		description := ""
		for _, entry := range entries {
			if entry.Meta.Name == "release-checklist" {
				description = entry.Meta.Description
			}
		}
		if description != "Previous checklist" {
			t.Fatalf("catalog description = %q, want the restored previous version", description)
		}
	})
}

// seedSkillInstallConflict leaves release-checklist as a retained conflict: the edited copy at the
// target, the original in the backup slot, and the journal in conflict state. It returns the op ID.
func seedSkillInstallConflict(t *testing.T, skillsRoot string) string {
	t.Helper()
	const opID = "sop_seededconflict"
	writeSkill := func(dir, description string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: release-checklist\ndescription: " + description + "\n---\n\nCheck the changelog."
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill(filepath.Join(skillsRoot, "release-checklist"), "Edited checklist")
	journal, err := dirpublish.NewJournal("release-checklist", nil)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(skillsRoot, journal.BackupName)
	writeSkill(backup, "Previous checklist")
	original, err := agentinstance.SkillTreeDigest(backup)
	if err != nil {
		t.Fatal(err)
	}
	journal.Payload, err = json.Marshal(agentinstance.SkillInstallPayload{
		OpID: opID, ReviewID: "skr_seeded", Action: "install", PreExists: true, PreDigest: original,
		StagedDigest: "sha256:staged", State: "conflict", OriginalSource: "backup", ConflictAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	journal.Phase = dirpublish.PhaseBackup
	parent, err := fileutil.OpenDirectoryForMutation(skillsRoot)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := dirpublish.WriteJournal(parent, journal, false)
	if err := errors.Join(writeErr, parent.Close()); err != nil {
		t.Fatal(err)
	}
	return opID
}
