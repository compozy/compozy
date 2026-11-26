package core_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// Invariant: resolve searches every skill root that can be located and reports a failed root lookup
// only when the op is not found elsewhere; the list stays fail-closed on any failed lookup.
// Owning layer: api/core install-conflict handlers; canonical suite: TestAgentSkillInstallConflictRoutes.
func TestAgentSkillInstallConflictRoutes(t *testing.T) {
	t.Run("Should resolve past a failed root lookup and report it only for an unknown op", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "conflicts.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		repo := db.Instances()
		for _, name := range []string{"coder", "broken"} {
			if _, _, err := repo.AdmitCreate(t.Context(), agentinstance.Instance{
				AgentName: name, ProfileID: store.DefaultProfileID, CreatedByKind: "human", PermissionCeiling: "{}",
			}, 10); err != nil {
				t.Fatal(err)
			}
		}
		skillsRoot := filepath.Join(t.TempDir(), "skills")
		opID := seedCoreSkillInstallConflict(t, skillsRoot)
		lookupFailure := errors.New("agent catalog unavailable")
		fixture := newHandlerFixture(t, &testutil.StubSessionManager{}, testutil.StubObserver{},
			testutil.StubWorkspaceService{})
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Store:          repo,
			SkillProposals: &agentinstance.SkillProposalService{Store: db.SkillReviews(), HomeID: "home-a"},
			SkillTarget: func(context.Context, agentinstance.Instance) (string, []string, error) {
				return skillsRoot, []string{skillsRoot}, nil
			},
			SkillRecoveryRoot: func(_ context.Context, instance agentinstance.Instance) (string, error) {
				if instance.AgentName == "broken" {
					return "", lookupFailure
				}
				return skillsRoot, nil
			},
		}
		fixture.Handlers.WorkspaceAccess = testutil.StubWorkspaceAccessPolicy{
			AuthorizeFn: func(context.Context, workspaceaccess.Request) (workspaceaccess.Decision, error) {
				return workspaceaccess.Decision{Allowed: true}, nil
			},
		}
		fixture.Engine.GET("/conflicts", fixture.Handlers.ListAgentSkillInstallConflicts)
		fixture.Engine.POST("/conflicts/:op_id/resolve", fixture.Handlers.ResolveAgentSkillInstallConflict)
		request := func(method, path, body string) *httptest.ResponseRecorder {
			t.Helper()
			req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
			recorder := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(recorder, req)
			return recorder
		}

		listed := request(http.MethodGet, "/conflicts", "")
		if listed.Code != http.StatusInternalServerError ||
			!strings.Contains(listed.Body.String(), lookupFailure.Error()) {
			t.Fatalf("list with a failed root lookup = %d %s, want it fail-closed", listed.Code, listed.Body.String())
		}
		unknown := request(http.MethodPost, "/conflicts/sop_unknown/resolve", `{"keep":"previous"}`)
		if unknown.Code != http.StatusInternalServerError ||
			!strings.Contains(unknown.Body.String(), lookupFailure.Error()) {
			t.Fatalf("resolve of an unknown op with a failed root lookup = %d %s, want the lookup error",
				unknown.Code, unknown.Body.String())
		}
		resolved := request(http.MethodPost, "/conflicts/"+opID+"/resolve", `{"keep":"previous"}`)
		var result contract.AgentSkillInstallConflictResolveResponse
		decodeJSON(t, resolved.Body.Bytes(), &result)
		if resolved.Code != http.StatusOK || result.OpID != opID || result.Skill != "release-checklist" ||
			result.Keep != "previous" {
			t.Fatalf("resolve of an op under a located root = %d %s", resolved.Code, resolved.Body.String())
		}
		live, err := os.ReadFile(filepath.Join(skillsRoot, "release-checklist", "SKILL.md"))
		if err != nil || !strings.Contains(string(live), "Previous checklist") {
			t.Fatalf("live SKILL.md = %q, %v; want the previous version restored", live, err)
		}
	})
}

// seedCoreSkillInstallConflict leaves release-checklist under skillsRoot as a retained conflict: the
// edited copy at the target, the original in the backup slot, and the journal in conflict state.
func seedCoreSkillInstallConflict(t *testing.T, skillsRoot string) string {
	t.Helper()
	const opID = "sop_corerouteconflict"
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
