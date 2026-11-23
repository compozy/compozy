package globaldb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// Invariant: skill review rows survive the lesson migration losslessly, and proposal identity
// (instance, skill, digest, base digest) dedupes pending and refuses rejected contents only.
// Owning layer: globaldb skill review repository on real SQLite; canonical suite: TestSkillReviewRepo.
func TestSkillReviewRepo(t *testing.T) {
	t.Run("Should migrate every review losslessly and keep legacy rows approvable [IT-012 IT-003]", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00219_schema.sql"))
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
		marker := agentinstance.SkillInstallMarker{
			ReviewID: "skr-pending", DecidedVia: "operator", DecidedBy: "operator:prof", InstalledDigest: "sha256:t",
		}
		if err := repo.ProjectApproval(ctx, marker, "sha256:t", nil, time.Now()); err != nil {
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
 VALUES ('identity-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z')`,
			store.DefaultProfileID)
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
		create := func(id, skill, digest, baseDigest string) (agentinstance.SkillReview, bool, error) {
			t.Helper()
			return repo.CreateReview(ctx, agentinstance.SkillReview{
				ID: id, InstanceID: "identity-instance", ProfileID: store.DefaultProfileID, SkillName: skill,
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
		first, duplicate, err := create("fresh", "fresh", "sha256:fresh", "")
		if err != nil || duplicate || first.Lesson.Confidence != "high" || first.EvidenceSessionIDs[0] != "sess-a" {
			t.Fatalf("fresh review = %+v, %v, %v", first, duplicate, err)
		}
		again, duplicate, err := create("fresh-again", "fresh", "sha256:fresh", "")
		if err != nil || !duplicate || again.ID != "fresh" {
			t.Fatalf("pending duplicate = %+v, %v, %v", again, duplicate, err)
		}
		otherBase, duplicate, err := create("fresh-base", "fresh", "sha256:fresh", "sha256:base")
		if err != nil || duplicate || otherBase.ID != "fresh-base" {
			t.Fatalf("different base = %+v, %v, %v", otherBase, duplicate, err)
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
			created, duplicate, err := create(skill+"-again", skill, "sha256:"+skill, "")
			if err != nil || duplicate || created.ID != skill+"-again" {
				t.Fatalf("%s identity = %+v, %v, %v", skill, created, duplicate, err)
			}
		}
		var reports int
		if err := db.DB().QueryRowContext(ctx, "SELECT count(*) FROM agent_instance_reports WHERE instance_id = ?",
			"identity-instance").Scan(&reports); err != nil || reports != 4 {
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

	t.Run("Should project committed installs idempotently with the commit time", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
 (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
 VALUES ('projection-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z')`,
			store.DefaultProfileID)
		if err != nil {
			t.Fatal(err)
		}
		repo := db.SkillReviews()
		report := agentinstance.ReportInput{
			SourceKey: "skill_review:skr-projected", Kind: "proposal",
			Payload: []byte(`{"skill_review_id":"skr-projected","proposals":[{"title":"Review","body":"Inspect"}]}`),
		}
		created, _, err := repo.CreateReview(ctx, agentinstance.SkillReview{
			ID: "skr-projected", InstanceID: "projection-instance", ProfileID: store.DefaultProfileID,
			SkillName: "projected", Digest: "sha256:d", DraftPath: "/drafts/projected",
			SkillKey: "workspace:ws/projected",
		}, report)
		if err != nil || created.SkillKey != "workspace:ws/projected" {
			t.Fatalf("create = %+v, %v", created, err)
		}
		committedAt := time.Date(2026, 11, 22, 23, 59, 0, 0, time.UTC)
		marker := agentinstance.SkillInstallMarker{
			ReviewID: "skr-projected", InstanceID: "projection-instance", DecidedVia: "operator",
			DecidedBy: "operator:prof", InstalledDigest: "sha256:tree", OpID: "sop_1",
		}
		previous := []byte(`{"review_id":"skr-older"}`)
		for range 2 {
			if err := repo.ProjectApproval(ctx, marker, "sha256:tree", previous, committedAt); err != nil {
				t.Fatalf("ProjectApproval() = %v", err)
			}
		}
		approved, err := repo.GetReview(ctx, "skr-projected")
		if err != nil || approved.State != "approved" || approved.InstalledDigest != "sha256:tree" ||
			!bytes.Equal(approved.PrevInstallMarker, previous) || approved.DecidedBy != "operator:prof" ||
			approved.DecidedAt == nil || !approved.DecidedAt.Equal(committedAt) {
			t.Fatalf("projected review = %+v, %v", approved, err)
		}
		later := marker
		later.DecidedBy = "someone-else"
		if err := repo.ProjectApproval(ctx, later, "sha256:other", nil, committedAt.Add(time.Hour)); err != nil {
			t.Fatalf("repeat projection: %v", err)
		}
		unchanged, err := repo.GetReview(ctx, "skr-projected")
		if err != nil || unchanged.InstalledDigest != "sha256:tree" || unchanged.DecidedBy != "operator:prof" {
			t.Fatalf("a decided review changed on a later projection: %+v, %v", unchanged, err)
		}
		if err := repo.DecideReject(ctx, "skr-projected", "sha256:d", "operator"); !errors.Is(
			err, agentinstance.ErrSkillReviewStale) {
			t.Fatalf("reject after approval = %v, want stale", err)
		}
	})

	t.Run("Should read instances created before the auto-approve migration as off [IT-005]", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00220_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
		const columns = `id,profile_id,agent_name,workspace_id,state,pause_reason,grant_ceiling,permission_ceiling,
 checks_enabled,depth,created_by_kind,created_by_id,created_at,updated_at`
		_, seedErr := prefix.ExecContext(ctx, `INSERT INTO agent_instances (`+columns+`) VALUES
 ('inst-old',?,'coder','','paused','manual','','{}',0,0,'human','operator','2026-11-01T00:00:00Z','2026-11-02T00:00:00Z')`,
			store.DefaultProfileID)
		if seedErr == nil {
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
			if err := upgraded.Close(ctx); err != nil {
				t.Error(err)
			}
		})
		var row string
		var autoApprove int
		err = upgraded.DB().QueryRowContext(ctx, `SELECT id||'|'||profile_id||'|'||agent_name||'|'||state||'|'||
 pause_reason||'|'||checks_enabled||'|'||created_at||'|'||updated_at, skill_auto_approve FROM agent_instances
 WHERE id='inst-old'`).Scan(&row, &autoApprove)
		want := "inst-old|" + store.DefaultProfileID + "|coder|paused|manual|0|2026-11-01T00:00:00Z|2026-11-02T00:00:00Z"
		if err != nil || row != want || autoApprove != 0 {
			t.Fatalf(
				"migrated instance = %q auto=%d, %v; want %q with skill auto-approve off",
				row,
				autoApprove,
				err,
				want,
			)
		}
	})

	t.Run(
		"Should count and filter this instance's automatic approvals of one UTC day [IT-021 UT-030 UT-031]",
		func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := t.Context()
			_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_skill_reviews
 (id,instance_id,profile_id,skill_name,digest,state,draft_path,decided_by,created_at,decided_at,decided_via)
 VALUES
 ('skr-auto','inst','prof','a','sha256:1','approved','/d/1','auto-approve','2026-11-22T00:00:00Z','2026-11-22T08:00:00.000000000Z','auto_approve'),
 ('skr-reverted','inst','prof','b','sha256:2','reverted','/d/2','auto-approve','2026-11-22T00:00:00Z','2026-11-22T23:59:59.000000000Z','auto_approve'),
 ('skr-start','inst','prof','c','sha256:3','approved','/d/3','auto-approve','2026-11-22T00:00:00Z','2026-11-22T00:00:00.000000000Z','auto_approve'),
 ('skr-yesterday','inst','prof','d','sha256:4','approved','/d/4','auto-approve','2026-11-21T00:00:00Z','2026-11-21T23:59:59.000000000Z','auto_approve'),
 ('skr-tomorrow','inst','prof','e','sha256:5','approved','/d/5','auto-approve','2026-11-22T00:00:00Z','2026-11-23T00:00:00.000000000Z','auto_approve'),
 ('skr-operator','inst','prof','f','sha256:6','approved','/d/6','operator:prof','2026-11-22T00:00:00Z','2026-11-22T09:00:00.000000000Z','operator'),
 ('skr-other','inst-b','prof','g','sha256:7','approved','/d/7','auto-approve','2026-11-22T00:00:00Z','2026-11-22T09:00:00.000000000Z','auto_approve'),
 ('skr-pending','inst','prof','h','sha256:8','pending','/d/8','','2026-11-22T00:00:00Z',NULL,'')`)
			if err != nil {
				t.Fatal(err)
			}
			day := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
			for _, tc := range []struct {
				instance string
				day      time.Time
				want     int
			}{
				{"inst", day, 3}, {"inst", day.AddDate(0, 0, -1), 1}, {"inst", day.AddDate(0, 0, 1), 1},
				{"inst-b", day, 1}, {"inst-none", day, 0},
			} {
				got, err := db.SkillReviews().CountAutoApprovals(ctx, tc.instance, tc.day)
				if err != nil || got != tc.want {
					t.Fatalf("CountAutoApprovals(%s, %s) = %d, %v; want %d", tc.instance, tc.day, got, err, tc.want)
				}
			}
			// UT-030, UT-031: the decided_via filter returns only automatic decisions, and an empty match is
			// an empty page.
			automatic, err := db.SkillReviews().ListReviews(ctx, agentinstance.SkillReviewQuery{
				ProfileID: "prof", DecidedVia: "auto_approve", Limit: 100,
			})
			var ids []string
			for _, review := range automatic {
				ids = append(ids, review.ID)
			}
			slices.Sort(ids)
			wantIDs := []string{"skr-auto", "skr-other", "skr-reverted", "skr-start", "skr-tomorrow", "skr-yesterday"}
			if err != nil || !slices.Equal(ids, wantIDs) {
				t.Fatalf("ListReviews(decided_via=auto_approve) = %v, %v; want %v [UT-030]", ids, err, wantIDs)
			}
			none, err := db.SkillReviews().ListReviews(ctx, agentinstance.SkillReviewQuery{
				ProfileID: "prof", InstanceID: "inst-none", DecidedVia: "auto_approve", Limit: 100,
			})
			if err != nil || none == nil || len(none) != 0 {
				t.Fatalf("ListReviews(no automatic decisions) = %#v, %v; want an empty slice [UT-031]", none, err)
			}
			rows, err := db.DB().QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT count(*) FROM agent_skill_reviews
 WHERE instance_id = 'inst' AND state IN ('approved', 'reverted') AND decided_via = 'auto_approve'
 AND decided_at >= '2026-11-22T00:00:00.000000000Z' AND decided_at < '2026-11-23T00:00:00.000000000Z'`)
			if err != nil {
				t.Fatal(err)
			}
			var plan []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				plan = append(plan, detail)
			}
			if err := errors.Join(rows.Err(), rows.Close()); err != nil {
				t.Fatal(err)
			}
			if joined := strings.Join(plan, "\n"); !strings.Contains(joined, "idx_agent_skill_reviews_instance") ||
				strings.Contains(joined, "SCAN agent_skill_reviews") {
				t.Fatalf("query plan = %s; want the instance index, no table scan", joined)
			}
		},
	)

	t.Run("Should count an automatic approval on its commit day after a late projection [IT-023]",
		func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := t.Context()
			_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
 (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at,skill_auto_approve)
 VALUES ('auto-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z',1)`,
				store.DefaultProfileID)
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			scanRoot := filepath.Join(root, "skills")
			commitAt := time.Date(2026, 11, 22, 23, 59, 0, 0, time.UTC)
			failing := agentinstance.SkillProposalService{
				Store:     failingProjectionStore{SkillReviewStore: db.SkillReviews(), err: errors.New("ledger down")},
				Evidence:  instanceScopedEvidence{},
				DraftRoot: filepath.Join(root, "drafts"),
				ScanRoots: []string{scanRoot},
				HomeID:    "home-a",
				AutoApprove: func(ctx context.Context, id string) (bool, error) {
					instance, err := db.Instances().Get(ctx, id)
					return instance.SkillAutoApprove, err
				},
				Now: func() time.Time { return commitAt },
			}
			instance := agentinstance.Instance{
				ID:        "auto-instance",
				ProfileID: store.DefaultProfileID,
				AgentName: "coder",
			}
			input := testSkillProposalInput("late-projection", "Late", "Check the projection.")
			input.Lesson.Confidence = "high"
			proposed, err := failing.Propose(ctx, workspaceaccess.ActorRef{Operator: true}, instance, scanRoot, input)
			if err != nil || proposed.AutoApproveSkipped != "" {
				t.Fatalf("Propose() = %+v, %v; want an automatic approval", proposed, err)
			}
			if pending, err := db.SkillReviews().
				GetReview(ctx, proposed.Review.ID); err != nil ||
				pending.State != "pending" {
				t.Fatalf("review after the failed projection = %+v, %v; want pending", pending, err)
			}
			healthy := failing
			healthy.Store = db.SkillReviews()
			healthy.Now = func() time.Time { return commitAt.Add(2 * time.Minute) }
			if err := healthy.ReconcileTargetRoot(ctx, scanRoot); err != nil {
				t.Fatalf("ReconcileTargetRoot() = %v", err)
			}
			approved, err := db.SkillReviews().GetReview(ctx, proposed.Review.ID)
			if err != nil || approved.State != "approved" || approved.DecidedVia != "auto_approve" ||
				approved.DecidedAt == nil || !approved.DecidedAt.Equal(commitAt) {
				t.Fatalf("projected review = %+v, %v; want decided at the commit time %s", approved, err, commitAt)
			}
			day := commitAt.Truncate(24 * time.Hour)
			for _, tc := range []struct {
				day  time.Time
				want int
			}{{day, 1}, {day.AddDate(0, 0, 1), 0}} {
				got, err := db.SkillReviews().CountAutoApprovals(ctx, "auto-instance", tc.day)
				if err != nil || got != tc.want {
					t.Fatalf("CountAutoApprovals(%s) = %d, %v; want %d", tc.day, got, err, tc.want)
				}
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
