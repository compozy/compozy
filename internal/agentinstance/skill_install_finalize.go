package agentinstance

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/dirpublish"
)

// finalizeSkillOp trims the install log, disposes of the backup and the original copy, and then
// removes the journal of a committed operation. The backup and the copy are each removed only when
// the entry actually deleted holds the verified original; anything else is retained for the
// operator. The incoming slot serves as the verification slot and is drained first, so a crash in the
// middle of a disposal is verified again instead of being deleted as staging.
func finalizeSkillOp(t *skillTarget, journal dirpublish.Journal, payload SkillInstallPayload) error {
	if err := trimSkillInstallLog(t.root, t.name); err != nil {
		return err
	}
	want := expectedSkillOriginal(payload)
	slots := []struct{ name, retain string }{
		{journal.IncomingName, skillRetainedName(payload.OpID, "edited-aside")},
		{journal.BackupName, skillRetainedName(payload.OpID, "edited-backup")},
		{payload.OriginalCopyName, skillRetainedName(payload.OpID, "edited-original")},
	}
	for _, slot := range slots {
		if slot.name == "" {
			continue
		}
		if err := disposeVerified(t, slot.name, journal.IncomingName, want, slot.retain); err != nil {
			return err
		}
	}
	return dirpublish.RemoveJournal(t.parent, t.name)
}

func skillRetainedName(opID, suffix string) string {
	return skillConflictCopyPrefix + opID + "-" + suffix
}

// disposeVerified removes the entry name only after moving it to the aside slot and digesting the
// moved entry: a write that lands between a check and the removal can then never be deleted. A
// moved entry that is not want is kept under retain and logged.
func disposeVerified(t *skillTarget, name, aside, want, retain string) error {
	state, err := recoveryTreeState(t.path(name))
	if err != nil || state == "" {
		return err
	}
	if name != aside {
		if err := moveSkillChild(t.parent, name, aside); err != nil {
			return fmt.Errorf("set aside %s for verification: %w", name, err)
		}
	}
	moved, err := recoveryTreeState(t.path(aside))
	if err != nil {
		return err
	}
	if moved == want {
		return t.parent.RemoveAll(aside)
	}
	if err := moveSkillChild(t.parent, aside, retain); err != nil {
		return fmt.Errorf("retain changed skill tree %s: %w", name, err)
	}
	slog.Warn("skill.install.tree_retained", "skill", t.name, "slot", name, "retained", retain)
	return nil
}

// publishSkills refreshes the catalog. The refresh and the pending flag update form one step under
// the daemon's refresh mutex, so the flag always reflects the last refresh to complete: a late
// success can never clear a newer failure. A failure stays pending and the next Reconcile of any
// skill retries it without touching files; a daemon restart rebuilds the catalog from disk.
func (s *SkillProposalService) publishSkills(ctx context.Context) error {
	return s.refreshCatalog(ctx, false)
}

// retryCatalogRefresh runs a refresh only when one is pending.
func (s *SkillProposalService) retryCatalogRefresh(ctx context.Context) error {
	return s.refreshCatalog(ctx, true)
}

// markCatalogRefreshPending records that a committed change still needs a catalog refresh, such
// as an install that recovery confirmed after its call returned before publishing.
func (s *SkillProposalService) markCatalogRefreshPending() {
	locks := s.installLocks()
	locks.publishMu.Lock()
	locks.publishPending = true
	locks.publishMu.Unlock()
}

func (s *SkillProposalService) refreshCatalog(ctx context.Context, onlyPending bool) error {
	locks := s.installLocks()
	locks.publishMu.Lock()
	defer locks.publishMu.Unlock()
	if onlyPending && !locks.publishPending {
		return nil
	}
	if s.Publisher == nil {
		locks.publishPending = false
		return nil
	}
	err := s.Publisher.SkillPublished(ctx)
	locks.publishPending = err != nil
	return err
}
