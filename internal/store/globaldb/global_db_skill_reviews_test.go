package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

// Invariant: skill review rows survive the lesson migration losslessly, and proposal identity
// (instance, skill, digest, base digest) dedupes pending and refuses rejected contents only.
// Owning layer: globaldb skill review repository on real SQLite; canonical suite: TestSkillReviewRepo.
func TestSkillReviewRepo(t *testing.T) {
	t.Run("Should migrate every review losslessly and keep legacy rows approvable [IT-012 IT-003]", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00220_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
		_, seedErr := prefix.ExecContext(ctx, `INSERT INTO agent_skill_reviews
 (id,instance_id,profile_id,workspace_id,agent_name,skill_name,digest,state,draft_path,source_session_id,decided_by,created_at,decided_at)
 VALUES
 ('skr-pending','inst','prof','ws','coder','pending-skill','sha256:p','pending','/drafts/p','sess-p','','2026-11-01T00:00:00Z',NULL),
 ('skr-approved','inst','prof','ws','coder','approved-skill','sha256:a','approved','/drafts/a',NULL,'operator:prof','2026-11-01T00:01:00Z','2026-11-02T00:00:00Z'),
 ('skr-rejected','inst','prof','','coder','rejected-skill','sha256:r','rejected','/drafts/r','sess-r','operator:prof','2026-11-01T00:02:00Z','2026-11-02T00:01:00Z')`)
		if seedErr == nil {
			// The prefix already records telemetry relocation; seed its required destination exactly as the
			// other post-relocation migration fixtures do.
			seedErr = bindSkillReviewMigrationTelemetry(ctx, prefix, path)
		}
		if err := errors.Join(seedErr, prefix.Close()); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			// t.Context is already canceled when cleanups run.
			if err := upgraded.Close(context.WithoutCancel(ctx)); err != nil {
				t.Error(err)
			}
		})
		rows, err := upgraded.DB().
			QueryContext(ctx, `SELECT id,instance_id,profile_id,workspace_id,agent_name,skill_name,
 digest,state,draft_path,COALESCE(source_session_id,''),decided_by,created_at,COALESCE(decided_at,''),kind,decided_via,
 lesson,evidence_session_ids,base_digest FROM agent_skill_reviews ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var cols [18]string
			dest := make([]any, len(cols))
			for i := range cols {
				dest[i] = &cols[i]
			}
			if err := rows.Scan(dest...); err != nil {
				t.Fatal(err)
			}
			got = append(got, strings.Join(cols[:], "|"))
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			t.Fatal(err)
		}
		want := []string{
			"skr-approved|inst|prof|ws|coder|approved-skill|sha256:a|approved|/drafts/a||operator:prof|" +
				"2026-11-01T00:01:00Z|2026-11-02T00:00:00Z|new|operator|{}|[]|",
			"skr-pending|inst|prof|ws|coder|pending-skill|sha256:p|pending|/drafts/p|sess-p||" +
				"2026-11-01T00:00:00Z||new||{}|[]|",
			"skr-rejected|inst|prof||coder|rejected-skill|sha256:r|rejected|/drafts/r|sess-r|operator:prof|" +
				"2026-11-01T00:02:00Z|2026-11-02T00:01:00Z|new|operator|{}|[]|",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("migrated rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		repo := upgraded.SkillReviews()
		legacy, err := repo.GetReview(ctx, "skr-pending")
		if err != nil || legacy.Kind != agentinstance.SkillProposalNew || legacy.Lesson.Recorded() ||
			len(legacy.EvidenceSessionIDs) != 0 {
			t.Fatalf("legacy review = %+v, %v", legacy, err)
		}
		if err := repo.DecideReview(ctx, "skr-pending", "sha256:p", "approved", "operator:prof"); err != nil {
			t.Fatalf("approve legacy review: %v", err)
		}
		approved, err := repo.GetReview(ctx, "skr-pending")
		if err != nil || approved.State != "approved" || approved.DecidedVia != "operator" {
			t.Fatalf("approved legacy review = %+v, %v", approved, err)
		}
	})

	t.Run("Should apply proposal identity on real SQLite [IT-013]", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
 (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
 VALUES ('identity-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z'),
 ('variant-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z')`,
			store.DefaultProfileID, store.DefaultProfileID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.DB().ExecContext(ctx, `INSERT INTO agent_skill_reviews
 (id,instance_id,profile_id,skill_name,digest,state,draft_path,created_at)
 VALUES
 ('legacy-pending','identity-instance',?,'legacy','sha256:legacy','pending','/drafts/l','2026-11-01T00:00:00Z'),
 ('was-approved','identity-instance',?,'approved','sha256:approved','approved','/drafts/a','2026-11-01T00:00:00Z'),
 ('was-reverted','identity-instance',?,'reverted','sha256:reverted','reverted','/drafts/r','2026-11-01T00:00:00Z'),
 ('was-rejected','identity-instance',?,'rejected','sha256:rejected','rejected','/drafts/x','2026-11-01T00:00:00Z')`,
			store.DefaultProfileID, store.DefaultProfileID, store.DefaultProfileID, store.DefaultProfileID)
		if err != nil {
			t.Fatal(err)
		}
		repo := db.SkillReviews()
		createFor := func(instanceID, id, skill, digest, baseDigest string) (agentinstance.SkillReview, bool, error) {
			t.Helper()
			return repo.CreateReview(ctx, agentinstance.SkillReview{
				ID: id, InstanceID: instanceID, ProfileID: store.DefaultProfileID, SkillName: skill,
				Digest: digest, BaseDigest: baseDigest, DraftPath: "/drafts/" + id,
				Lesson: agentinstance.SkillLesson{
					Cue: "cue", Lesson: "lesson", Evidence: "evidence", Confidence: "high",
				},
				EvidenceSessionIDs: []string{"sess-a"},
			}, agentinstance.ReportInput{
				SourceKey: "skill_review:" + id,
				Kind:      agentinstance.ReportKindProposal,
				Payload: []byte(
					`{"skill_review_id":"` + id + `","proposals":[{"title":"Review","body":"Review it."}]}`,
				),
			})
		}
		create := func(id, skill, digest, baseDigest string) (agentinstance.SkillReview, bool, error) {
			t.Helper()
			return createFor("identity-instance", id, skill, digest, baseDigest)
		}
		first, duplicate, err := create("fresh", "fresh", "sha256:fresh", "")
		if err != nil || duplicate || first.Lesson.Confidence != "high" || first.EvidenceSessionIDs[0] != "sess-a" {
			t.Fatalf("fresh review = %+v, %v, %v", first, duplicate, err)
		}
		again, duplicate, err := create("fresh-again", "fresh", "sha256:fresh", "")
		if err != nil || !duplicate || again.ID != "fresh" {
			t.Fatalf("pending duplicate = %+v, %v, %v", again, duplicate, err)
		}
		// Each identity field alone makes a new review; only an exact match is a duplicate. The variants
		// use their own instance so the pending-review limit stays out of the way.
		if _, _, err := createFor("variant-instance", "variant", "variant", "sha256:variant", ""); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ id, skill, digest, base string }{
			{id: "variant-skill", skill: "variant-other", digest: "sha256:variant"},
			{id: "variant-digest", skill: "variant", digest: "sha256:variant-other"},
			{id: "variant-base", skill: "variant", digest: "sha256:variant", base: "sha256:base"},
		} {
			created, duplicate, err := createFor("variant-instance", tc.id, tc.skill, tc.digest, tc.base)
			if err != nil || duplicate || created.ID != tc.id {
				t.Fatalf("%s identity = %+v, %v, %v; want a new review", tc.id, created, duplicate, err)
			}
		}
		legacy, duplicate, err := create("legacy-again", "legacy", "sha256:legacy", "")
		if err != nil || !duplicate || legacy.ID != "legacy-pending" {
			t.Fatalf("legacy pending duplicate = %+v, %v, %v", legacy, duplicate, err)
		}
		_, _, err = create("rejected-again", "rejected", "sha256:rejected", "")
		if !errors.Is(err, agentinstance.ErrSkillProposePreviouslyRejected) ||
			!strings.Contains(err.Error(), "was-rejected") {
			t.Fatalf("rejected identity: %v", err)
		}
		for _, skill := range []string{"approved", "reverted"} {
			t.Run("Should create a new review over a "+skill+" identity", func(t *testing.T) {
				created, duplicate, err := create(skill+"-again", skill, "sha256:"+skill, "")
				if err != nil || duplicate || created.ID != skill+"-again" {
					t.Fatalf("%s identity = %+v, %v, %v", skill, created, duplicate, err)
				}
			})
		}
		var reports int
		if err := db.DB().QueryRowContext(ctx, "SELECT count(*) FROM agent_instance_reports WHERE instance_id = ?",
			"identity-instance").Scan(&reports); err != nil || reports != 3 {
			t.Fatalf("reports = %d, %v; want one per new review", reports, err)
		}
	})

	t.Run("Should resolve evidence session scope and report missing sessions", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(t, db, "evidence", "sess-evidence")
		repo := db.SkillReviews()
		profileID, gotWorkspace, err := repo.SessionScope(t.Context(), sessionID)
		if err != nil || profileID != store.DefaultProfileID || gotWorkspace != workspaceID {
			t.Fatalf("SessionScope() = %q, %q, %v", profileID, gotWorkspace, err)
		}
		if _, _, err := repo.SessionScope(t.Context(), "sess-missing"); !errors.Is(
			err,
			agentinstance.ErrSkillProposeEvidenceNotFound,
		) {
			t.Fatalf("missing session: %v", err)
		}
	})
}

func bindSkillReviewMigrationTelemetry(ctx context.Context, db *sql.DB, path string) error {
	telemetry, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
	if err != nil {
		return err
	}
	return errors.Join(bindTelemetryDestination(ctx, db, telemetry.DB()), telemetry.Close(ctx))
}
