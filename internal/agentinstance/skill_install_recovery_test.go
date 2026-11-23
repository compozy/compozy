package agentinstance

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
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
	t.Run("Should retain a target that gained a symlink as a conflict instead of failing", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "target_moved"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		link := filepath.Join(f.root, "release-checklist", "link.md")
		if err := os.Symlink("SKILL.md", link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		for range 2 {
			f.reconcile(t, f.service(t, "home-b", nil))
			payload, ok := f.journal(t, "release-checklist")
			if !ok || payload.State != skillInstallStateConflict || payload.OriginalSource != skillOriginalInBackup {
				t.Fatalf("journal = %+v, %v; want a retained conflict with the original in backup", payload, ok)
			}
		}
		if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("the edited tree's symlink was not preserved: %v", err)
		}
	})
	t.Run("Should retain a symlink that replaced the target as a conflict without following it", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		base := f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "target_moved"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		target := filepath.Join(f.root, "release-checklist")
		if err := os.RemoveAll(target); err != nil {
			t.Fatal(err)
		}
		elsewhere := t.TempDir()
		if err := os.WriteFile(filepath.Join(elsewhere, "SKILL.md"), []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, target); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		journal := f.rawJournal(t, "release-checklist")
		for range 2 {
			f.reconcile(t, f.service(t, "home-b", nil))
			payload, ok := f.journal(t, "release-checklist")
			if !ok || payload.State != skillInstallStateConflict || payload.OriginalSource != skillOriginalInBackup {
				t.Fatalf("journal = %+v, %v; want a retained conflict with the original in backup", payload, ok)
			}
		}
		if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("the symlink at the target was not kept: %v", err)
		}
		if data, err := os.ReadFile(filepath.Join(elsewhere, "SKILL.md")); err != nil || string(data) != "outside" {
			t.Fatalf("recovery followed the symlink: %q, %v", data, err)
		}
		if got, err := skillTreeState(filepath.Join(f.root, journal.BackupName)); err != nil || got != base {
			t.Fatalf("backup = %s, %v; want the original", got, err)
		}
	})
	t.Run("Should keep a tree a crashed verification moved into the incoming slot", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "target_moved"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		journal := f.rawJournal(t, "release-checklist")
		target := filepath.Join(f.root, "release-checklist")
		if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("edited by the operator"), 0o600); err != nil {
			t.Fatal(err)
		}
		edited := f.digest(t, "release-checklist")
		// The rollback moved the edited target aside for verification, then the daemon stopped.
		if err := os.Rename(target, filepath.Join(f.root, journal.IncomingName)); err != nil {
			t.Fatal(err)
		}
		f.reconcile(t, f.service(t, "home-b", nil))
		if f.digest(t, "release-checklist") != edited {
			t.Fatal("recovery lost the edited tree from the incoming slot")
		}
		if payload, ok := f.journal(t, "release-checklist"); !ok || payload.State != skillInstallStateConflict {
			t.Fatalf("journal = %+v, %v; want a retained conflict", payload, ok)
		}
	})
	t.Run("Should leave a journal whose payload names a foreign sibling untouched", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		victim := f.install(t, "victim-skill", skillBaseFiles...)
		data, err := json.Marshal(SkillInstallPayload{
			OpID: "sop-corrupt", Action: SkillInstallActionInstall, PreExists: true, PreDigest: victim,
			State: skillInstallStateActive, OriginalCopyName: "victim-skill",
		})
		if err != nil {
			t.Fatal(err)
		}
		journal, err := dirpublish.NewJournal("release-checklist", data)
		if err != nil {
			t.Fatal(err)
		}
		parent, err := fileutil.OpenDirectoryForMutation(f.root)
		if err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(dirpublish.WriteJournal(parent, journal, false), parent.Close()); err != nil {
			t.Fatal(err)
		}
		before := f.journalBytes(t, "release-checklist")
		err = svc.ReconcileTargetRoot(t.Context(), f.root)
		if !errors.Is(err, dirpublish.ErrRecoveryRequired) ||
			!strings.Contains(err.Error(), `"victim-skill" does not belong to op sop-corrupt`) {
			t.Fatalf("ReconcileTargetRoot() = %v, want the payload ownership refusal", err)
		}
		if f.digest(t, "victim-skill") != victim {
			t.Fatal("recovery removed a sibling the operation does not own")
		}
		if after := f.journalBytes(t, "release-checklist"); !bytes.Equal(after, before) {
			t.Fatal("recovery rewrote the corrupt journal instead of keeping it for diagnosis")
		}
	})
	t.Run("Should retain a backup edited after its verification instead of deleting it", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "published"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		journal := f.rawJournal(t, "release-checklist")
		payload, _ := f.journal(t, "release-checklist")
		backup := filepath.Join(f.root, journal.BackupName)
		if err := os.WriteFile(filepath.Join(backup, "late.md"), []byte("saved late"), 0o600); err != nil {
			t.Fatal(err)
		}
		edited, err := skillTreeState(backup)
		if err != nil {
			t.Fatal(err)
		}
		installed := f.digest(t, "release-checklist")
		f.reconcile(t, svc)
		retained := filepath.Join(f.root, skillConflictCopyPrefix+payload.OpID+"-edited-backup")
		if got, err := skillTreeState(retained); err != nil || got != edited {
			t.Fatalf("retained backup = %s, %v; want the edited backup", got, err)
		}
		if f.digest(t, "release-checklist") != installed {
			t.Fatal("finalize changed the committed tree")
		}
		f.requireClean(t)
	})
	t.Run("Should keep all 200 committed log entries when an operation rolls back", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := make([]skillInstallLogEntry, skillInstallLogMaxEntries)
		for i := range seeded {
			seeded[i] = skillInstallLogEntry{
				OpID: fmt.Sprintf("sop-old-%03d", i), Action: SkillInstallActionInstall, At: time.Now().UTC(),
			}
		}
		if err := writeSkillInstallLog(f.root, "release-checklist", seeded); err != nil {
			t.Fatal(err)
		}
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		if _, err := f.approve(t, crashAt(svc, "logged"), review); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("Approve() = %v", err)
		}
		f.reconcile(t, svc)
		entries, err := readSkillInstallLog(f.root, "release-checklist")
		if err != nil || len(entries) != len(seeded) {
			t.Fatalf("log after rollback: %d entries, %v; want the 200 committed ones", len(entries), err)
		}
		for i := range seeded {
			if entries[i].OpID != seeded[i].OpID || entries[i].Action != seeded[i].Action ||
				!entries[i].At.Equal(seeded[i].At) || entries[i].Current != nil {
				t.Fatalf("log entry %d after rollback = %+v, want %+v", i, entries[i], seeded[i])
			}
		}
		if result, err := f.approve(t, svc, review); err != nil || !result.Committed {
			t.Fatalf("Approve() = %+v, %v", result, err)
		}
		entries, err = readSkillInstallLog(f.root, "release-checklist")
		if err != nil || len(entries) != skillInstallLogMaxEntries {
			t.Fatalf("log after commit: %d entries, %v; want the newest 200", len(entries), err)
		}
		last := entries[len(entries)-1]
		if entries[0].OpID != "sop-old-001" || last.Current == nil || last.Current.ReviewID != review.ID {
			t.Fatalf("log after commit: %d entries, %v; want the newest 200 ending with the review", len(entries), err)
		}
	})
	t.Run("Should leave a journal with an unknown state untouched", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		if _, err := f.approve(t, crashAt(svc, "target_moved"), review); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("Approve() = %v", err)
		}
		staged := f.digest(t, "release-checklist")
		journal := f.rawJournal(t, "release-checklist")
		payload, _ := f.journal(t, "release-checklist")
		payload.State = "actve"
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		journal.Payload = data
		parent, err := fileutil.OpenDirectoryForMutation(f.root)
		if err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(dirpublish.WriteJournal(parent, journal, true), parent.Close()); err != nil {
			t.Fatal(err)
		}
		before := f.journalBytes(t, "release-checklist")
		err = svc.ReconcileTargetRoot(t.Context(), f.root)
		if !errors.Is(err, dirpublish.ErrRecoveryRequired) || !strings.Contains(err.Error(), `unknown state "actve"`) {
			t.Fatalf("ReconcileTargetRoot() = %v, want the unknown-state refusal", err)
		}
		if f.digest(t, "release-checklist") != staged || !bytes.Equal(f.journalBytes(t, "release-checklist"), before) {
			t.Fatal("recovery changed files for a journal in an unknown state")
		}
	})
	t.Run("Should retain an edited original copy instead of deleting it at finalize", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "published"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		payload, _ := f.journal(t, "release-checklist")
		original := filepath.Join(f.root, payload.OriginalCopyName)
		if err := os.WriteFile(filepath.Join(original, "late.md"), []byte("saved late"), 0o600); err != nil {
			t.Fatal(err)
		}
		edited, err := skillTreeState(original)
		if err != nil {
			t.Fatal(err)
		}
		f.reconcile(t, svc)
		retained := filepath.Join(f.root, skillRetainedName(payload.OpID, "edited-original"))
		if got, err := skillTreeState(retained); err != nil || got != edited {
			t.Fatalf("retained original copy = %s, %v; want the edited copy", got, err)
		}
		f.requireClean(t)
	})
	t.Run("Should verify a tree a crashed disposal left in the verification slot", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "published"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		journal := f.rawJournal(t, "release-checklist")
		payload, _ := f.journal(t, "release-checklist")
		// Finalize moved the backup aside for verification, a write landed in it, then the daemon stopped.
		aside := filepath.Join(f.root, journal.IncomingName)
		if err := os.Rename(filepath.Join(f.root, journal.BackupName), aside); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(aside, "late.md"), []byte("saved late"), 0o600); err != nil {
			t.Fatal(err)
		}
		edited, err := skillTreeState(aside)
		if err != nil {
			t.Fatal(err)
		}
		f.reconcile(t, svc)
		retained := filepath.Join(f.root, skillRetainedName(payload.OpID, "edited-aside"))
		if got, err := skillTreeState(retained); err != nil || got != edited {
			t.Fatalf("retained aside tree = %s, %v; want the edited tree", got, err)
		}
		f.requireClean(t)
	})
	t.Run("Should retain a symlink found in a journal slot at finalize without following it", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "published"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		journal := f.rawJournal(t, "release-checklist")
		payload, _ := f.journal(t, "release-checklist")
		backup := filepath.Join(f.root, journal.BackupName)
		if err := os.RemoveAll(backup); err != nil {
			t.Fatal(err)
		}
		elsewhere := t.TempDir()
		if err := os.WriteFile(filepath.Join(elsewhere, "SKILL.md"), []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, backup); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		installed := f.digest(t, "release-checklist")
		f.reconcile(t, svc)
		retained := filepath.Join(f.root, skillRetainedName(payload.OpID, "edited-backup"))
		if info, err := os.Lstat(retained); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("retained backup slot = %v; want the symlink kept as a link", err)
		}
		if data, err := os.ReadFile(filepath.Join(elsewhere, "SKILL.md")); err != nil || string(data) != "outside" {
			t.Fatalf("finalize followed the symlink: %q, %v", data, err)
		}
		if f.digest(t, "release-checklist") != installed {
			t.Fatal("finalize changed the committed tree")
		}
		f.requireClean(t)
	})
	t.Run("Should keep a conflict when a crash followed retaining the unverified tree", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "target_moved"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		payload, _ := f.journal(t, "release-checklist")
		// The retain move completed (incoming slot empty), then the daemon stopped before the conflict.
		retained := filepath.Join(f.root, skillRetainedName(payload.OpID, "unverified"))
		if err := os.MkdirAll(retained, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(retained, "SKILL.md"), []byte("first edit"), 0o600); err != nil {
			t.Fatal(err)
		}
		edit, err := skillTreeState(retained)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			f.reconcile(t, f.service(t, "home-b", nil))
			if got, ok := f.journal(t, "release-checklist"); !ok || got.State != skillInstallStateConflict {
				t.Fatalf("journal = %+v, %v; want the conflict kept so the retained tree stays discoverable", got, ok)
			}
		}
		if got, err := skillTreeState(retained); err != nil || got != edit {
			t.Fatalf("retained unverified tree = %s, %v; want it unchanged", got, err)
		}
	})
	t.Run("Should record a conflict when an unverified tree meets an occupied target", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, "release-checklist", skillBaseFiles...)
		if err := f.runOp(t, crashAt(svc, "target_moved"), "release-checklist",
			f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		journal := f.rawJournal(t, "release-checklist")
		payload, _ := f.journal(t, "release-checklist")
		target := filepath.Join(f.root, "release-checklist")
		if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("first edit"), 0o600); err != nil {
			t.Fatal(err)
		}
		unverified := f.digest(t, "release-checklist")
		if err := os.Rename(target, filepath.Join(f.root, journal.IncomingName)); err != nil {
			t.Fatal(err)
		}
		occupant := f.install(t, "release-checklist",
			skillTestFile{path: "SKILL.md", content: "second edit", mode: 0o600})
		f.reconcile(t, f.service(t, "home-b", nil))
		if got, ok := f.journal(t, "release-checklist"); !ok || got.State != skillInstallStateConflict {
			t.Fatalf("journal = %+v, %v; want a retained conflict", got, ok)
		}
		retained := filepath.Join(f.root, skillRetainedName(payload.OpID, "unverified"))
		if got, err := skillTreeState(retained); err != nil || got != unverified {
			t.Fatalf("retained unverified tree = %s, %v; want the first edit", got, err)
		}
		if f.digest(t, "release-checklist") != occupant {
			t.Fatal("recovery changed the occupying target")
		}
		recorded, _ := f.journal(t, "release-checklist")
		if recorded.RetainedUnverifiedName != filepath.Base(retained) {
			t.Fatalf("journal retained unverified name = %q, want %q", recorded.RetainedUnverifiedName,
				filepath.Base(retained))
		}
		conflicts, err := svc.InstallConflicts(t.Context(), skillOperator, []string{f.root})
		if err != nil || len(conflicts) != 1 || conflicts[0].RetainedCopy != retained {
			t.Fatalf("InstallConflicts() = %+v, %v; want retained copy %s", conflicts, err, retained)
		}
		result, err := f.resolve(t, svc, payload.OpID, SkillConflictKeepCurrent)
		if err != nil || result.RetainedCopy != retained {
			t.Fatalf("ResolveInstallConflict() = %+v, %v; want retained copy %s", result, err, retained)
		}
		if got, err := skillTreeState(retained); err != nil || got != unverified {
			t.Fatalf("retained unverified tree after resolution = %s, %v; want it kept", got, err)
		}
		if f.digest(t, "release-checklist") != occupant {
			t.Fatal("keep=current changed the occupying target")
		}
	})
}

func (f *skillTxFixture) journalBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, dirpublish.JournalName(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (f *skillTxFixture) rawJournal(t *testing.T, name string) dirpublish.Journal {
	t.Helper()
	parent, err := fileutil.OpenDirectory(f.root)
	if err != nil {
		t.Fatal(err)
	}
	journal, exists, err := dirpublish.ReadJournal(parent, name)
	if closeErr := parent.Close(); err != nil || closeErr != nil || !exists {
		t.Fatalf("read journal %s: %v %v %v", name, exists, err, closeErr)
	}
	return journal
}
