package agentinstance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

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
}
