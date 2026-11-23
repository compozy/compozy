package agentinstance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Invariant: Reconcile in any home recovers an interrupted skill operation from the on-disk journal
// alone: before published it rolls back (including the operation's install-log entries), and
// contents nobody can explain are retained as a conflict instead of being overwritten or deleted.
// Owning layer: agentinstance install recovery; canonical suite: TestSkillInstallRecovery.
func TestSkillInstallRecovery(t *testing.T) {
	t.Run("Should retain an external edit after the swap as a conflict across reconciles [UT-064]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		// Proposed before the skill exists: a new proposal of an installed name is refused.
		review := f.propose(t, svc, "release-checklist", "Another change.")
		base := f.install(t, "release-checklist", skillBaseFiles...)
		op := f.replaceOp(t, "release-checklist")
		if err := f.runOp(t, crashAt(svc, "target_moved"), "release-checklist", op); !errors.Is(
			err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		edited := filepath.Join(f.root, "release-checklist", "SKILL.md")
		if err := os.WriteFile(edited, []byte("edited by the operator"), 0o600); err != nil {
			t.Fatal(err)
		}
		editedDigest := f.digest(t, "release-checklist")
		for range 3 {
			f.reconcile(t, f.service(t, "home-b", nil))
			payload, ok := f.journal(t, "release-checklist")
			if !ok || payload.State != skillInstallStateConflict || payload.OriginalSource != skillOriginalInBackup {
				t.Fatalf("journal after reconcile = %+v, %v; want a conflict with the original in backup", payload, ok)
			}
			if f.digest(t, "release-checklist") != editedDigest {
				t.Fatal("reconcile changed the edited copy")
			}
		}
		backups := 0
		for _, name := range f.leftovers(t) {
			if strings.HasPrefix(name, ".compozy-backup-") {
				backups++
				if digest, err := skillTreeState(filepath.Join(f.root, name)); err != nil || digest != base {
					t.Fatalf("backup slot = %s, %v; want the original", digest, err)
				}
			}
		}
		if backups != 1 {
			t.Fatalf("backup slots = %d, want 1", backups)
		}
		if _, err := f.approve(t, svc, review); !errors.Is(err, ErrSkillInstallConflict) {
			t.Fatalf("Approve() during a conflict = %v, want skill_install_conflict", err)
		}
		if f.digest(t, "release-checklist") != editedDigest {
			t.Fatal("approval during a conflict changed files")
		}
	})
	t.Run("Should return busy to another home mid-swap and show it the committed result [UT-067]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		homeA := f.service(t, "home-a", nil)
		homeB := f.service(t, "home-b", nil)
		reviewB := f.propose(t, homeB, "shared-skill", "Home B version.")
		reviewA := f.propose(t, homeA, "shared-skill", "Home A version.")
		var busyErr error
		var during string
		swapping := withHook(homeA, func(step string) bool {
			if step == "rechecked" {
				_, busyErr = f.approve(t, homeB, reviewB)
				during = f.digest(t, "shared-skill")
			}
			return false
		})
		if result, err := f.approve(t, swapping, reviewA); err != nil || !result.Committed {
			t.Fatalf("home A Approve() = %+v, %v", result, err)
		}
		if !errors.Is(busyErr, ErrSkillInstallBusy) || during != "" {
			t.Fatalf("home B mid-swap = %v with target %q; want busy and untouched", busyErr, during)
		}
		requireReviewState(t, homeB, reviewB.ID, SkillReviewPending)
		if _, err := f.approve(t, homeB, reviewB); !errors.Is(err, ErrSkillReviewTargetExists) {
			t.Fatalf("home B retry = %v, want home A's skill to exist", err)
		}
	})
	t.Run("Should let another home roll back a crashed swap and drop its log entries [UT-068]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		homeA := f.service(t, "home-a", nil)
		review := f.propose(t, homeA, "release-checklist", "Check the changelog.")
		if _, err := f.approve(t, crashAt(homeA, "logged"), review); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("Approve() = %v", err)
		}
		if writers := installLogWriters(t, f.root, "release-checklist"); len(writers) != 1 {
			t.Fatalf("log before recovery = %v, want the uncommitted entry", writers)
		}
		f.reconcile(t, f.service(t, "home-b", nil))
		if f.digest(t, "release-checklist") != "" ||
			len(installLogWriters(t, f.root, "release-checklist")) != 0 {
			t.Fatal("home B did not roll back home A's operation")
		}
		f.requireClean(t)
		f.reconcile(t, homeA)
		requireReviewState(t, homeA, review.ID, SkillReviewPending)
	})
	t.Run("Should roll back a crash between the log append and published [UT-070]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		first := f.propose(t, svc, "release-checklist", "Check the changelog.")
		if _, err := f.approve(t, svc, first); err != nil {
			t.Fatal(err)
		}
		installed := f.digest(t, "release-checklist")
		entries, err := readSkillInstallLog(f.root, "release-checklist")
		if err != nil {
			t.Fatal(err)
		}
		op := f.replaceOp(t, "release-checklist")
		op.prev = currentSkillWriter(entries)
		if err := f.runOp(t, crashAt(svc, "logged"), "release-checklist", op); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		f.reconcile(t, svc)
		writers := installLogWriters(t, f.root, "release-checklist")
		if f.digest(t, "release-checklist") != installed || len(writers) != 1 || writers[0] != first.ID {
			t.Fatalf("after rollback: writers %v; want the previous writer %s and its tree", writers, first.ID)
		}
		f.requireClean(t)
	})
	t.Run("Should finish a rollback interrupted after restoring the original [UT-073]", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			crash  string
			exists bool
			remove bool
		}{
			{name: "Should finish restoring a replaced skill", crash: "target_moved", exists: true},
			{name: "Should finish removing a new skill", crash: "target_moved"},
			{name: "Should finish restoring a removed skill", crash: "backup_moved", exists: true, remove: true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				svc := f.service(t, "home-a", nil)
				base := ""
				var op skillOp
				if tc.exists {
					base = f.install(t, "release-checklist", skillBaseFiles...)
					op = f.replaceOp(t, "release-checklist")
				} else {
					op = skillOp{action: SkillInstallActionInstall, changed: errSkillTestChanged,
						stage: func(dir string) error {
							return writeSkillFile(filepath.Join(dir, "SKILL.md"), []byte("new"), 0o600)
						}}
				}
				if tc.remove {
					op.stage, op.next, op.action = nil, nil, SkillInstallActionRevert
				}
				if err := f.runOp(t, crashAt(svc, tc.crash), "release-checklist", op); !errors.Is(
					err, errSkillInstallCrashed) {
					t.Fatalf("runSkillOp() = %v", err)
				}
				if err := crashAt(svc, "rollback_restored").ReconcileTargetRoot(t.Context(), f.root); !errors.Is(
					err, errSkillInstallCrashed) {
					t.Fatalf("interrupted rollback = %v", err)
				}
				f.reconcile(t, svc)
				if _, ok := f.journal(t, "release-checklist"); ok || f.digest(t, "release-checklist") != base {
					t.Fatal("the resumed rollback did not finish at the original without a conflict")
				}
				f.requireClean(t)
			})
		}
	})
	t.Run("Should refuse when the original changes before it is copied [UT-082]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		op := f.replaceOp(t, "release-checklist")
		var changed string
		racing := withHook(svc, func(step string) bool {
			if step == "prepared" {
				changed = f.install(t, "release-checklist",
					skillTestFile{path: "notes.md", content: "external", mode: 0o600})
			}
			return false
		})
		if err := f.runOp(t, racing, "release-checklist", op); !errors.Is(err, errSkillTestChanged) {
			t.Fatalf("runSkillOp() = %v, want the step-2 error", err)
		}
		if f.digest(t, "release-checklist") != changed {
			t.Fatal("a refused operation moved the live skill")
		}
		f.requireClean(t)
	})
	t.Run("Should clean a crash during the original copy or staging from any home [UT-084]", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name    string
			step    string
			partial bool
		}{
			{name: "Should remove a partial original copy", step: "original_copied", partial: true},
			{name: "Should remove a complete original copy", step: "original_copied"},
			{name: "Should remove a partial incoming directory", step: "incoming_created"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				svc := f.service(t, "home-a", nil)
				base := f.install(t, "release-checklist", skillBaseFiles...)
				crashing := withHook(svc, func(step string) bool {
					if step != tc.step {
						return false
					}
					if tc.partial {
						for _, name := range f.leftovers(t) {
							if strings.HasPrefix(name, skillOriginalCopyPrefix) {
								if err := os.Remove(filepath.Join(f.root, name, "data.bin")); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
					return true
				})
				if err := f.runOp(t, crashing, "release-checklist", f.replaceOp(t, "release-checklist")); !errors.Is(
					err, errSkillInstallCrashed) {
					t.Fatalf("runSkillOp() = %v", err)
				}
				f.reconcile(t, f.service(t, "home-b", nil))
				if f.digest(t, "release-checklist") != base {
					t.Fatal("recovery changed the untouched target")
				}
				f.requireClean(t)
			})
		}
	})
}
