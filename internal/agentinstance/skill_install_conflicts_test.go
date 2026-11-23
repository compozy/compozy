package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// countingPublisher counts catalog publications and fails the next `fail` calls.
type countingPublisher struct {
	calls atomic.Int32
	fail  atomic.Int32
}

func (p *countingPublisher) SkillPublished(context.Context) error {
	p.calls.Add(1)
	if p.fail.Add(-1) >= 0 {
		return errors.New("publication failed")
	}
	p.fail.Store(0)
	return nil
}

var skillPreviousWriter = &SkillInstallMarker{ReviewID: "skr-base", InstanceID: "inst-a", OpID: "sop-base"}

type seededSkillConflict struct {
	opID, base, edited string
}

// seedConflict leaves release-checklist in a retained conflict whose original is in source
// (backup or copy) and returns the digests of the original and the edited tree.
func (f *skillTxFixture) seedConflict(t *testing.T, svc *SkillProposalService, source string) seededSkillConflict {
	t.Helper()
	base := f.install(t, "release-checklist", skillBaseFiles...)
	op := f.replaceOp(t, "release-checklist")
	op.prev = skillPreviousWriter
	crash := "target_moved"
	if source == skillOriginalInCopy {
		crash = "backup_moved"
	}
	if err := f.runOp(t, crashAt(svc, crash), "release-checklist", op); !errors.Is(err, errSkillInstallCrashed) {
		t.Fatalf("runSkillOp() = %v", err)
	}
	edit := filepath.Join(f.root, "release-checklist", "SKILL.md")
	if source == skillOriginalInCopy {
		for _, name := range f.leftovers(t) {
			if strings.HasPrefix(name, ".compozy-backup-") {
				edit = filepath.Join(f.root, name, "SKILL.md")
			}
		}
	}
	if err := os.WriteFile(edit, []byte("edited by the operator"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, svc)
	payload, ok := f.journal(t, "release-checklist")
	if !ok || payload.State != skillInstallStateConflict || payload.OriginalSource != source {
		t.Fatalf("seeded journal = %+v, %v; want a conflict with the original in %s", payload, ok, source)
	}
	return seededSkillConflict{opID: payload.OpID, base: base, edited: f.digest(t, "release-checklist")}
}

// rewriteJournal edits the payload of a skill's journal in place, for states the filesystem alone
// cannot reach.
func (f *skillTxFixture) rewriteJournal(t *testing.T, name string, edit func(*SkillInstallPayload)) {
	t.Helper()
	parent, err := fileutil.OpenDirectoryForMutation(f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := parent.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	journal, exists, err := dirpublish.ReadJournal(parent, name)
	if err != nil || !exists {
		t.Fatalf("ReadJournal(%s) = %v, %v", name, exists, err)
	}
	payload, err := decodeSkillInstallPayload(journal.Payload)
	if err != nil {
		t.Fatal(err)
	}
	edit(&payload)
	if journal.Payload, err = json.Marshal(payload); err != nil {
		t.Fatal(err)
	}
	if err := dirpublish.WriteJournal(parent, journal, true); err != nil {
		t.Fatal(err)
	}
}

// writeTree replaces SKILL.md under the root-relative dir with content and returns the tree digest.
func (f *skillTxFixture) writeTree(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.root, dir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return f.digest(t, dir)
}

func (f *skillTxFixture) resolve(
	t *testing.T,
	svc *SkillProposalService,
	opID, keep string,
) (SkillConflictResolution, error) {
	t.Helper()
	return svc.ResolveInstallConflict(t.Context(), skillOperator, []string{f.root}, opID, keep, "operator:prof-a")
}

func (f *skillTxFixture) lastInstallEntry(t *testing.T) skillInstallLogEntry {
	t.Helper()
	entries, err := readSkillInstallLog(f.root, "release-checklist")
	if err != nil || len(entries) == 0 {
		t.Fatalf("install log = %v, %v", entries, err)
	}
	return entries[len(entries)-1]
}

// requirePreviousKept checks a finished keep=previous resolution of seeded.
func (f *skillTxFixture) requirePreviousKept(t *testing.T, seeded seededSkillConflict) {
	t.Helper()
	if f.digest(t, "release-checklist") != seeded.base {
		t.Fatal("the original was not restored byte- and mode-exact")
	}
	if f.digest(t, skillConflictCopyPrefix+seeded.opID) != seeded.edited {
		t.Fatal("the edited tree is not in the keep-copy directory")
	}
	if _, ok := f.journal(t, "release-checklist"); ok {
		t.Fatal("the resolution left its journal")
	}
	f.requireClean(t)
	entry := f.lastInstallEntry(t)
	if entry.OpID != seeded.opID || entry.Action != SkillInstallActionResolve || entry.Current == nil ||
		entry.Current.ReviewID != skillPreviousWriter.ReviewID {
		t.Fatalf("last log entry = %+v; want a resolve restoring the previous writer", entry)
	}
}

// Invariant: an operator resolves a retained install conflict from any home by keeping either
// version; the choice is journaled before anything moves, Reconcile finishes it after a crash, the
// edited copy is never overwritten, and a repeat is answered from the install log.
// Owning layer: agentinstance install recovery; canonical suite: TestSkillInstallConflictResolution.
func TestSkillInstallConflictResolution(t *testing.T) {
	t.Run("Should keep the previous version and set the edited copy aside [UT-065]", func(t *testing.T) {
		t.Parallel()
		for _, source := range []string{skillOriginalInBackup, skillOriginalInCopy} {
			t.Run("Should restore the original from its "+source, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				svc := f.service(t, "home-a", nil)
				seeded := f.seedConflict(t, svc, source)
				publisher := &countingPublisher{}
				other := f.service(t, "home-b", nil)
				other.Publisher = publisher
				result, err := f.resolve(t, other, seeded.opID, SkillConflictKeepPrevious)
				if err != nil || result.AlreadyResolved || result.Warning != nil ||
					result.KeptCopy != filepath.Join(f.root, skillConflictCopyPrefix+seeded.opID) ||
					result.Skill != "release-checklist" {
					t.Fatalf("ResolveInstallConflict() = %+v, %v", result, err)
				}
				f.requirePreviousKept(t, seeded)
				if publisher.calls.Load() != 1 {
					t.Fatalf("publications = %d, want 1", publisher.calls.Load())
				}
			})
		}
	})
	t.Run("Should keep the current version and discard the previous one [UT-065]", func(t *testing.T) {
		t.Parallel()
		for _, source := range []string{skillOriginalInBackup, skillOriginalInCopy} {
			t.Run("Should discard the original in its "+source, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				svc := f.service(t, "home-a", nil)
				seeded := f.seedConflict(t, svc, source)
				if result, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); err != nil ||
					result.KeptCopy != "" {
					t.Fatalf("ResolveInstallConflict() = %+v, %v", result, err)
				}
				if f.digest(t, "release-checklist") != seeded.edited {
					t.Fatal("keep=current changed the edited tree")
				}
				if _, err := os.Stat(filepath.Join(f.root, skillConflictCopyPrefix+seeded.opID)); !os.IsNotExist(err) {
					t.Fatalf("keep=current created a keep-copy: %v", err)
				}
				f.requireClean(t)
				entry := f.lastInstallEntry(t)
				if entry.Action != SkillInstallActionResolve || entry.Current == nil || entry.Current.ReviewID != "" ||
					entry.Current.InstalledDigest != seeded.edited || entry.Current.OpID != seeded.opID {
					t.Fatalf("last log entry = %+v; want an operator-authored writer of the edited tree", entry)
				}
			})
		}
	})
	t.Run("Should block the skill, refuse agents and answer repeats from the log [UT-066]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		publisher := &countingPublisher{}
		svc.Publisher = publisher
		// Proposed before the skill exists: a new proposal of an installed name is refused.
		review := f.propose(t, svc, "release-checklist", "Another change.")
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		if _, err := f.approve(t, svc, review); !errors.Is(err, ErrSkillInstallConflict) ||
			!strings.Contains(err.Error(), "install-conflict resolve "+seeded.opID) {
			t.Fatalf("Approve() during a conflict = %v, want skill_install_conflict naming the op", err)
		}
		agent := workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: "sess-a"}
		if _, err := svc.ResolveInstallConflict(t.Context(), agent, []string{f.root}, seeded.opID,
			SkillConflictKeepPrevious, "operator:prof-a"); !errors.Is(err, ErrSkillReviewOperatorOnly) {
			t.Fatalf("agent ResolveInstallConflict() = %v, want skill_review_operator_only", err)
		}
		if _, err := svc.InstallConflicts(t.Context(), agent, []string{f.root}); !errors.Is(
			err, ErrSkillReviewOperatorOnly) {
			t.Fatalf("agent InstallConflicts() = %v, want skill_review_operator_only", err)
		}
		conflicts, err := svc.InstallConflicts(t.Context(), skillOperator, []string{f.root})
		if err != nil || len(conflicts) != 1 || conflicts[0].OpID != seeded.opID ||
			conflicts[0].Skill != "release-checklist" || conflicts[0].State != skillInstallStateConflict ||
			conflicts[0].Since.IsZero() {
			t.Fatalf("InstallConflicts() = %+v, %v", conflicts, err)
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious); err != nil {
			t.Fatal(err)
		}
		restored := f.digest(t, "release-checklist")
		calls := publisher.calls.Load()
		publisher.fail.Store(1)
		repeat, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious)
		if err != nil || !repeat.AlreadyResolved || repeat.Warning == nil {
			t.Fatalf("repeated resolve = %+v, %v; want already_resolved with the publication warning", repeat, err)
		}
		if repeat, err = f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious); err != nil ||
			!repeat.AlreadyResolved || repeat.Warning != nil {
			t.Fatalf("publication retry = %+v, %v", repeat, err)
		}
		if publisher.calls.Load() != calls+2 || f.digest(t, "release-checklist") != restored {
			t.Fatal("a repeated resolve changed files or did not publish again")
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); !errors.Is(
			err, ErrSkillInstallConflictResolving) {
			t.Fatalf("repeat with the other keep = %v, want skill_install_conflict_resolving", err)
		}
		if conflicts, err := svc.InstallConflicts(t.Context(), skillOperator, []string{f.root}); err != nil ||
			len(conflicts) != 0 {
			t.Fatalf("InstallConflicts() after resolve = %+v, %v", conflicts, err)
		}
		if _, err := f.resolve(t, svc, "sop_unknown", SkillConflictKeepPrevious); !errors.Is(
			err, ErrSkillInstallConflictNotFound) {
			t.Fatalf("unknown op = %v, want skill_install_conflict_not_found", err)
		}
	})
	t.Run("Should finish an interrupted keep=previous on Reconcile [UT-075]", func(t *testing.T) {
		t.Parallel()
		for _, step := range []string{"edited_kept", "original_restored", "resolve_logged"} {
			t.Run("Should finish after "+step, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				svc := f.service(t, "home-a", nil)
				seeded := f.seedConflict(t, svc, skillOriginalInBackup)
				if _, err := f.resolve(t, crashAt(svc, step), seeded.opID, SkillConflictKeepPrevious); !errors.Is(
					err, errSkillInstallCrashed) {
					t.Fatalf("ResolveInstallConflict() = %v", err)
				}
				conflicts, err := svc.InstallConflicts(t.Context(), skillOperator, []string{f.root})
				if err != nil || len(conflicts) != 1 || conflicts[0].State != skillInstallStateResolving ||
					conflicts[0].Keep != SkillConflictKeepPrevious {
					t.Fatalf("InstallConflicts() mid-resolution = %+v, %v", conflicts, err)
				}
				publisher := &countingPublisher{}
				other := f.service(t, "home-b", nil)
				other.Publisher = publisher
				f.reconcile(t, other)
				f.requirePreviousKept(t, seeded)
				if publisher.calls.Load() != 1 {
					t.Fatalf("publications = %d, want 1", publisher.calls.Load())
				}
			})
		}
	})
	t.Run("Should never overwrite an occupied keep-copy [UT-075]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		occupant := filepath.Join(f.root, skillConflictCopyPrefix+seeded.opID, "mine.md")
		if err := os.MkdirAll(filepath.Dir(occupant), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(occupant, []byte("not yours"), 0o600); err != nil {
			t.Fatal(err)
		}
		occupied := f.digest(t, skillConflictCopyPrefix+seeded.opID)
		if _, err := f.resolve(
			t,
			svc,
			seeded.opID,
			SkillConflictKeepPrevious,
		); !errors.Is(
			err,
			ErrSkillInstallConflict,
		) {
			t.Fatalf("ResolveInstallConflict() = %v, want skill_install_conflict", err)
		}
		f.reconcile(t, svc)
		payload, ok := f.journal(t, "release-checklist")
		if !ok || payload.State != skillInstallStateResolving {
			t.Fatalf("journal = %+v, %v; want resolving", payload, ok)
		}
		if f.digest(t, skillConflictCopyPrefix+seeded.opID) != occupied ||
			f.digest(t, "release-checklist") != seeded.edited {
			t.Fatal("an occupied keep-copy was overwritten or the edited tree moved")
		}
	})
	t.Run("Should finish an interrupted keep=current and refuse the other keep [UT-076]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		if _, err := f.resolve(t, crashAt(svc, "backup_removed"), seeded.opID, SkillConflictKeepCurrent); !errors.Is(
			err, errSkillInstallCrashed) {
			t.Fatalf("ResolveInstallConflict() = %v", err)
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious); !errors.Is(
			err, ErrSkillInstallConflictResolving) {
			t.Fatalf("resolve with the other keep = %v, want skill_install_conflict_resolving", err)
		}
		f.reconcile(t, svc)
		if f.digest(t, "release-checklist") != seeded.edited {
			t.Fatal("the resumed keep=current changed the edited tree")
		}
		f.requireClean(t)
		if entry := f.lastInstallEntry(t); entry.OpID != seeded.opID || entry.Current == nil ||
			entry.Current.ReviewID != "" {
			t.Fatalf("last log entry = %+v; want the operator-authored writer", entry)
		}
		result, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent)
		if err != nil || !result.AlreadyResolved || f.digest(t, "release-checklist") != seeded.edited {
			t.Fatalf("same keep=current = %+v, %v; want a no-op success", result, err)
		}
	})
	t.Run("Should retain a second external edit in the backup slot through either resolution [UT-064]",
		func(t *testing.T) {
			t.Parallel()
			for _, keep := range []string{SkillConflictKeepCurrent, SkillConflictKeepPrevious} {
				t.Run("Should keep both edited trees with keep="+keep, func(t *testing.T) {
					t.Parallel()
					f := newSkillTxFixture(t)
					svc := f.service(t, "home-a", nil)
					base := f.install(t, "release-checklist", skillBaseFiles...)
					if err := f.runOp(t, crashAt(svc, "target_moved"), "release-checklist",
						f.replaceOp(t, "release-checklist")); !errors.Is(err, errSkillInstallCrashed) {
						t.Fatalf("runSkillOp() = %v", err)
					}
					backupSlot := ""
					for _, name := range f.leftovers(t) {
						if strings.HasPrefix(name, ".compozy-backup-") {
							backupSlot = name
						}
					}
					atTarget := f.writeTree(t, "release-checklist", "edited at the target")
					inBackup := f.writeTree(t, backupSlot, "edited in the backup slot")
					f.reconcile(t, svc)
					payload, ok := f.journal(t, "release-checklist")
					retained := skillConflictCopyPrefix + payload.OpID + "-backup"
					if !ok || payload.State != skillInstallStateConflict || payload.RetainedBackupName != retained ||
						payload.OriginalSource != skillOriginalInCopy {
						t.Fatalf("journal = %+v, %v; want a conflict retaining %s", payload, ok, retained)
					}
					retainedPath := filepath.Join(f.root, retained)
					conflicts, err := svc.InstallConflicts(t.Context(), skillOperator, []string{f.root})
					if err != nil || len(conflicts) != 1 || conflicts[0].RetainedCopy != retainedPath {
						t.Fatalf("InstallConflicts() = %+v, %v; want retained copy %s", conflicts, err, retainedPath)
					}
					result, err := f.resolve(t, svc, payload.OpID, keep)
					if err != nil || result.RetainedCopy != retainedPath {
						t.Fatalf("ResolveInstallConflict() = %+v, %v; want retained copy %s", result, err, retainedPath)
					}
					if f.digest(t, retained) != inBackup {
						t.Fatal("the resolution removed the edit retained from the backup slot")
					}
					wantTarget, keptCopy := atTarget, ""
					if keep == SkillConflictKeepPrevious {
						wantTarget, keptCopy = base, skillConflictCopyPrefix+payload.OpID
					}
					if f.digest(t, "release-checklist") != wantTarget ||
						(keptCopy != "" && f.digest(t, keptCopy) != atTarget) {
						t.Fatal("the resolution lost the edit at the target or did not restore the original")
					}
					f.requireClean(t)
				})
			}
		})
	t.Run("Should refuse keep=previous when the original is gone [UT-065]", func(t *testing.T) {
		t.Parallel()
		type loseOriginal func(t *testing.T, f *skillTxFixture, payload SkillInstallPayload)
		cases := map[string]loseOriginal{
			"Should refuse a lost original copy": func(t *testing.T, f *skillTxFixture, p SkillInstallPayload) {
				if err := os.RemoveAll(filepath.Join(f.root, p.OriginalCopyName)); err != nil {
					t.Fatal(err)
				}
			},
			"Should refuse an absent original": func(t *testing.T, f *skillTxFixture, _ SkillInstallPayload) {
				f.rewriteJournal(t, "release-checklist", func(payload *SkillInstallPayload) {
					payload.OriginalSource = skillOriginalAbsent
				})
			},
		}
		for name, lose := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				svc := f.service(t, "home-a", nil)
				seeded := f.seedConflict(t, svc, skillOriginalInCopy)
				payload, _ := f.journal(t, "release-checklist")
				lose(t, f, payload)
				if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious); !errors.Is(
					err, ErrSkillInstallConflict) || !strings.Contains(err.Error(), "--keep current") {
					t.Fatalf("keep=previous without an original = %v, want skill_install_conflict", err)
				}
				if after, ok := f.journal(t, "release-checklist"); !ok || after.State != skillInstallStateConflict {
					t.Fatalf("journal = %+v, %v; want it still in conflict", after, ok)
				}
				if f.digest(t, "release-checklist") != seeded.edited ||
					f.digest(t, skillConflictCopyPrefix+seeded.opID) != "" {
					t.Fatal("a refused keep=previous moved files")
				}
				if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); err != nil {
					t.Fatalf("keep=current after the refusal = %v", err)
				}
			})
		}
	})
	t.Run("Should answer a repeated keep=current from the recorded keep [UT-076]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		if err := os.RemoveAll(filepath.Join(f.root, "release-checklist")); err != nil {
			t.Fatal(err)
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); err != nil {
			t.Fatalf("keep=current with an absent edited tree = %v", err)
		}
		if entry := f.lastInstallEntry(t); entry.Keep != SkillConflictKeepCurrent || entry.Current != nil {
			t.Fatalf("last log entry = %+v; want keep=current removing the skill", entry)
		}
		if repeat, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); err != nil ||
			!repeat.AlreadyResolved {
			t.Fatalf("repeated keep=current = %+v, %v; want already_resolved", repeat, err)
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious); !errors.Is(
			err, ErrSkillInstallConflictResolving) {
			t.Fatalf("repeat with the other keep = %v, want skill_install_conflict_resolving", err)
		}
	})
	t.Run("Should list and resolve around an unreadable journal of another skill [UT-066]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		broken := filepath.Join(f.root, dirpublish.JournalName("broken-skill"))
		if err := os.WriteFile(broken, []byte(`{"target_name":"broken-skill"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		conflicts, err := svc.InstallConflicts(t.Context(), skillOperator, []string{f.root})
		if err != nil || len(conflicts) != 1 || conflicts[0].OpID != seeded.opID {
			t.Fatalf("InstallConflicts() = %+v, %v; want the readable conflict only", conflicts, err)
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); err != nil {
			t.Fatalf("ResolveInstallConflict() = %v", err)
		}
	})
	t.Run("Should return busy to resolve while another process holds the lock [UT-083]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		parent, err := fileutil.OpenDirectoryForMutation(f.root)
		if err != nil {
			t.Fatal(err)
		}
		lock, err := dirpublish.AcquireLock(parent, "release-checklist")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); !errors.Is(err, ErrSkillInstallBusy) {
			t.Fatalf("ResolveInstallConflict() = %v, want skill_install_busy", err)
		}
		if payload, ok := f.journal(t, "release-checklist"); !ok || payload.State != skillInstallStateConflict {
			t.Fatalf("journal = %+v, %v; want it untouched", payload, ok)
		}
		if err := errors.Join(lock.Close(), parent.Close()); err != nil {
			t.Fatal(err)
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); err != nil {
			t.Fatalf("ResolveInstallConflict() after release = %v", err)
		}
	})
	t.Run("Should resolve an edited copy replaced by a symlink without following it [UT-065]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(f.root, "release-checklist")
		if err := os.RemoveAll(target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, target); err != nil {
			t.Fatal(err)
		}
		_, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent)
		if !errors.Is(err, ErrSkillInstallConflict) || !strings.Contains(err.Error(), "--keep previous") {
			t.Fatalf("keep=current over a symlink = %v, want skill_install_conflict naming --keep previous", err)
		}
		if payload, ok := f.journal(t, "release-checklist"); !ok || payload.State != skillInstallStateConflict {
			t.Fatalf("journal = %+v, %v; want it still in conflict", payload, ok)
		}
		if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("refused keep=current changed the target: %v, %v", info, err)
		}
		result, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious)
		if err != nil || result.KeptCopy != filepath.Join(f.root, skillConflictCopyPrefix+seeded.opID) {
			t.Fatalf("keep=previous over a symlink = %+v, %v", result, err)
		}
		if f.digest(t, "release-checklist") != seeded.base {
			t.Fatal("keep=previous did not restore the original")
		}
		if link, err := os.Readlink(result.KeptCopy); err != nil || link != outside {
			t.Fatalf("keep-copy = %q, %v; want the symlink itself moved aside", link, err)
		}
		content, err := os.ReadFile(filepath.Join(outside, "SKILL.md"))
		if err != nil || string(content) != "outside" {
			t.Fatalf("the symlink's referent changed: %q, %v", content, err)
		}
		f.requireClean(t)
	})
	t.Run("Should reopen keep=current when the target changed after the choice [UT-076]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		f.recordChoice(t, svc, seeded.opID, SkillConflictKeepCurrent)
		changed := f.writeTree(t, "release-checklist", "edited again before restart")
		f.reconcile(t, svc)
		payload, ok := f.journal(t, "release-checklist")
		if !ok || payload.State != skillInstallStateConflict || payload.Keep != "" {
			t.Fatalf("journal = %+v, %v; want the conflict reopened", payload, ok)
		}
		if f.digest(t, f.backupName(t)) != seeded.base || f.digest(t, "release-checklist") != changed {
			t.Fatal("reopening the conflict changed the backup or the target")
		}
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); err != nil {
			t.Fatal(err)
		}
		if entry := f.lastInstallEntry(t); entry.Current == nil || entry.Current.InstalledDigest != changed {
			t.Fatalf("last log entry = %+v; want the writer of the live tree %s", entry, changed)
		}
	})
	t.Run("Should record the live tree when keep=current already discarded the original [UT-076]",
		func(t *testing.T) {
			t.Parallel()
			f := newSkillTxFixture(t)
			svc := f.service(t, "home-a", nil)
			seeded := f.seedConflict(t, svc, skillOriginalInBackup)
			crashing := crashAt(svc, "backup_removed")
			if _, err := f.resolve(t, crashing, seeded.opID, SkillConflictKeepCurrent); !errors.Is(
				err, errSkillInstallCrashed) {
				t.Fatalf("ResolveInstallConflict() = %v", err)
			}
			changed := f.writeTree(t, "release-checklist", "edited again before restart")
			f.reconcile(t, svc)
			if _, ok := f.journal(t, "release-checklist"); ok {
				t.Fatal("the resumed keep=current left its journal")
			}
			if entry := f.lastInstallEntry(t); entry.Current == nil || entry.Current.InstalledDigest != changed {
				t.Fatalf("last log entry = %+v; want the writer of the live tree %s", entry, changed)
			}
		})
	t.Run("Should keep a backup edited after keep=current was recorded [UT-076]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		f.recordChoice(t, svc, seeded.opID, SkillConflictKeepCurrent)
		backup := f.backupName(t)
		edited := f.writeTree(t, backup, "backup edited before restart")
		f.reconcile(t, svc)
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepCurrent); !errors.Is(
			err, ErrSkillInstallConflict) || !strings.Contains(err.Error(), backup) {
			t.Fatalf("keep=current over an edited backup = %v, want skill_install_conflict naming it", err)
		}
		if payload, ok := f.journal(t, "release-checklist"); !ok || payload.State != skillInstallStateResolving {
			t.Fatalf("journal = %+v, %v; want it still resolving", payload, ok)
		}
		if f.digest(t, backup) != edited || f.digest(t, "release-checklist") != seeded.edited {
			t.Fatal("a refused keep=current discarded the edited backup or changed the target")
		}
	})
	t.Run("Should keep the live tree when the original changed after keep=previous [UT-075]", func(t *testing.T) {
		t.Parallel()
		for _, source := range []string{skillOriginalInBackup, skillOriginalInCopy} {
			t.Run("Should refuse a changed original in its "+source, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				svc := f.service(t, "home-a", nil)
				seeded := f.seedConflict(t, svc, source)
				f.recordChoice(t, svc, seeded.opID, SkillConflictKeepPrevious)
				var original string
				if source == skillOriginalInCopy {
					payload, _ := f.journal(t, "release-checklist")
					original = payload.OriginalCopyName
					if err := os.RemoveAll(filepath.Join(f.root, original)); err != nil {
						t.Fatal(err)
					}
				} else {
					original = f.backupName(t)
					f.writeTree(t, original, "backup edited before restart")
				}
				f.reconcile(t, svc)
				if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious); !errors.Is(
					err, ErrSkillInstallConflict) || !strings.Contains(err.Error(), original) {
					t.Fatalf("keep=previous over a changed original = %v, want skill_install_conflict naming it", err)
				}
				if f.digest(t, "release-checklist") != seeded.edited {
					t.Fatal("the live edited tree was moved aside before the original was verified")
				}
				if _, err := os.Stat(filepath.Join(f.root, skillConflictCopyPrefix+seeded.opID)); !os.IsNotExist(err) {
					t.Fatalf("a refused keep=previous created the keep-copy: %v", err)
				}
				payload, ok := f.journal(t, "release-checklist")
				if !ok || payload.State != skillInstallStateResolving {
					t.Fatalf("journal = %+v, %v; want it still resolving", payload, ok)
				}
			})
		}
	})
	t.Run("Should leave a resolving journal with an unknown keep unchanged [UT-076]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		f.recordChoice(t, svc, seeded.opID, SkillConflictKeepCurrent)
		f.rewriteJournal(t, "release-checklist", func(payload *SkillInstallPayload) { payload.Keep = "" })
		f.reconcile(t, svc)
		payload, ok := f.journal(t, "release-checklist")
		if !ok || payload.State != skillInstallStateResolving || payload.Keep != "" {
			t.Fatalf("journal = %+v, %v; want it unchanged", payload, ok)
		}
		if f.digest(t, f.backupName(t)) != seeded.base || f.digest(t, "release-checklist") != seeded.edited {
			t.Fatal("recovery of an unknown keep changed files")
		}
	})
	t.Run("Should refuse to reject a review whose install is a retained conflict [UT-066]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		// Proposed before the skill exists, so the proposal stays a new skill in every later slice.
		review := f.propose(t, svc, "release-checklist", "Another change.")
		seeded := f.seedConflict(t, svc, skillOriginalInBackup)
		f.rewriteJournal(t, "release-checklist", func(payload *SkillInstallPayload) { payload.ReviewID = review.ID })
		reject := func() error {
			return svc.Reject(t.Context(), skillOperator, f.instance, review.ID, review.Digest,
				"operator:prof-a", f.root)
		}
		if err := reject(); !errors.Is(err, ErrSkillReviewStale) ||
			!strings.Contains(err.Error(), "install-conflict resolve "+seeded.opID) {
			t.Fatalf("Reject() during its own conflict = %v, want skill_review_stale naming the op", err)
		}
		requireReviewState(t, svc, review.ID, SkillReviewPending)
		if _, err := f.resolve(t, svc, seeded.opID, SkillConflictKeepPrevious); err != nil {
			t.Fatal(err)
		}
		if err := reject(); err != nil {
			t.Fatalf("Reject() after keep=previous = %v", err)
		}
		requireReviewState(t, svc, review.ID, "rejected")
	})
}

// recordChoice records keep for opID and stops as a crash would, before any file moves.
func (f *skillTxFixture) recordChoice(t *testing.T, svc *SkillProposalService, opID, keep string) {
	t.Helper()
	if _, err := f.resolve(t, crashAt(svc, "resolving_recorded"), opID, keep); !errors.Is(err, errSkillInstallCrashed) {
		t.Fatalf("ResolveInstallConflict(%s) = %v", keep, err)
	}
}

// backupName is the backup slot left next to the skill.
func (f *skillTxFixture) backupName(t *testing.T) string {
	t.Helper()
	for _, name := range f.leftovers(t) {
		if strings.HasPrefix(name, ".compozy-backup-") {
			return name
		}
	}
	t.Fatal("no backup slot")
	return ""
}
