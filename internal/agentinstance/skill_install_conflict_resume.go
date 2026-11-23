package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/dirpublish"
)

// continueResolution idempotently finishes a resolving journal: move the trees for keep, commit the
// `resolve` log entry and the published phase, then finalize and publish. A journal whose recorded
// keep is neither current nor previous is corrupt and is left unchanged for diagnosis.
func (s *SkillProposalService) continueResolution(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) (warning error, err error) {
	if payload.Keep != SkillConflictKeepCurrent && payload.Keep != SkillConflictKeepPrevious {
		return nil, fmt.Errorf("%w: the journal of %s records an unknown keep %q (op %s); it is left unchanged",
			ErrSkillInstallConflict, t.name, payload.Keep, payload.OpID)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if journal.Phase != dirpublish.PhasePublished {
		if payload.Keep == SkillConflictKeepPrevious {
			err = s.restorePrevious(ctx, t, journal, payload)
		} else {
			payload, err = s.discardPrevious(ctx, t, journal, payload)
		}
		if err != nil {
			return nil, err
		}
		if err := s.commitResolution(ctx, t, &journal, payload); err != nil {
			return nil, err
		}
	}
	// A published resolution counts only once durable: confirm it before removing any copy, also when
	// an earlier attempt published it and then failed.
	if s.crashed("published_sync") {
		return nil, errSkillInstallCrashed
	}
	if err := t.parent.Sync(); err != nil {
		return nil, fmt.Errorf("confirm published skill conflict resolution: %w", err)
	}
	warning = errors.Join(finalizeSkillOp(t, journal, payload), s.publishSkills(ctx))
	slog.InfoContext(ctx, "skill.install.conflict_resolved", "op_id", payload.OpID, "skill", t.name,
		"keep", payload.Keep, "retained_backup", payload.RetainedBackupName)
	if warning != nil {
		slog.WarnContext(ctx, "skill.install.committed_with_warning", "op_id", payload.OpID,
			"action", SkillInstallActionResolve, "error", warning)
	}
	return warning, nil
}

// discardPrevious finishes keep=current: it removes the backup slot only while the live tree is still
// the one the operator chose. A tree changed since the choice was recorded reopens the conflict while
// the original can still be restored; once the original is gone, the live tree is the current version
// and the writer is recorded with its digest.
func (s *SkillProposalService) discardPrevious(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) (SkillInstallPayload, error) {
	live, err := skillTreeState(t.dir)
	if err != nil {
		return payload, err
	}
	chosen := ""
	if payload.NextMarker != nil {
		chosen = payload.NextMarker.InstalledDigest
	}
	if live != chosen {
		if previousRestorable(t, journal, payload) {
			return payload, s.reopenConflict(ctx, t, journal, payload)
		}
		slog.WarnContext(ctx, "skill.install.conflict_current_changed", "op_id", payload.OpID, "skill", t.name)
		if payload, err = s.recordCurrentWriter(ctx, t, journal, payload, live); err != nil {
			return payload, err
		}
	}
	if err := requireUnchangedOriginal(ctx, t, journal, payload); err != nil {
		return payload, err
	}
	if err := t.parent.RemoveAll(journal.BackupName); err != nil {
		return payload, err
	}
	if s.crashed("backup_removed") {
		return payload, errSkillInstallCrashed
	}
	return payload, nil
}

// requireUnchangedOriginal refuses to discard a backup slot or original copy whose contents changed
// after the conflict was recorded; finalize would otherwise delete an edit nobody chose to drop.
func requireUnchangedOriginal(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	for _, name := range []string{journal.BackupName, payload.OriginalCopyName} {
		if name == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		digest, err := skillTreeState(t.path(name))
		if err == nil && (digest == "" || digest == payload.PreDigest) {
			continue
		}
		return errors.Join(fmt.Errorf("%w: %s changed after --keep current was recorded (op %s); "+
			"move it away and resolve again", ErrSkillInstallConflict, t.path(name), payload.OpID), err)
	}
	return nil
}

// reopenConflict returns a resolving journal to the conflict state, keeping every copy, so the
// operator decides again on the tree that is now live.
func (s *SkillProposalService) reopenConflict(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	payload.State, payload.Keep, payload.KeepCopyName, payload.ResolvedBy = skillInstallStateConflict, "", "", ""
	if err := writeSkillResolution(ctx, t, journal, payload); err != nil {
		return err
	}
	slog.WarnContext(ctx, "skill.install.conflict_reopened", "op_id", payload.OpID, "skill", t.name)
	return fmt.Errorf("%w: %s changed after --keep current was recorded; review it and resolve %s again",
		ErrSkillInstallConflict, t.name, payload.OpID)
}

// recordCurrentWriter records the live tree as the operator-authored writer of a keep=current
// resolution whose original is already gone.
func (s *SkillProposalService) recordCurrentWriter(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
	live string,
) (SkillInstallPayload, error) {
	payload.NextMarker = nil
	if live != "" {
		payload.NextMarker = &SkillInstallMarker{
			HomeID: s.HomeID, InstalledDigest: live, DecidedVia: skillDecidedViaOperator,
			DecidedBy: payload.ResolvedBy, OpID: payload.OpID,
		}
	}
	return payload, writeSkillResolution(ctx, t, journal, payload)
}

func writeSkillResolution(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	journal.Payload = encoded
	if err := dirpublish.WriteJournal(t.parent, journal, true); err != nil {
		return fmt.Errorf("record skill conflict resolution: %w", err)
	}
	return nil
}

// restorePrevious moves the edited tree to the keep-copy directory and the original back to the
// target. The original is verified against PreDigest before anything moves, an occupied keep-copy is
// never overwritten, and on any refusal the live tree stays in place and the journal stays resolving.
func (s *SkillProposalService) restorePrevious(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	// An edited copy that is a symlink or holds other entries is moved aside as it is, never followed.
	target, err := skillTreeState(t.dir)
	unsupported := errors.Is(err, ErrSkillTreeUnsupported)
	if err != nil && !unsupported {
		return err
	}
	if !unsupported && target == expectedSkillOriginal(payload) {
		return nil // already restored by an interrupted run, or a new skill already absent
	}
	if !previousRestorable(t, journal, payload) {
		source := t.path(journal.BackupName)
		if payload.OriginalSource == skillOriginalInCopy {
			source = t.path(payload.OriginalCopyName)
		}
		slog.WarnContext(ctx, "skill.install.conflict_original_changed", "op_id", payload.OpID, "skill", t.name)
		return fmt.Errorf("%w: the previous %s at %s no longer matches what was installed (op %s); "+
			"restore it there and resolve %s again with --keep previous",
			ErrSkillInstallConflict, t.name, source, payload.OpID, payload.OpID)
	}
	if unsupported || target != "" {
		if err := s.moveEditedToKeepCopy(ctx, t, payload); err != nil {
			return err
		}
	}
	if !payload.PreExists {
		return nil
	}
	source := journal.BackupName
	if payload.OriginalSource == skillOriginalInCopy {
		source = payload.OriginalCopyName
	}
	err = moveSkillChild(t.parent, source, t.name)
	if err == nil && s.crashed("original_restored") {
		err = errSkillInstallCrashed
	}
	return err
}

// moveEditedToKeepCopy moves the live edited tree to the keep-copy directory unless it is occupied.
func (s *SkillProposalService) moveEditedToKeepCopy(
	ctx context.Context,
	t *skillTarget,
	payload SkillInstallPayload,
) error {
	occupied, err := dirpublish.DirectoryExists(t.parent, payload.KeepCopyName)
	if err != nil {
		return err
	}
	if occupied {
		slog.WarnContext(ctx, "skill.install.conflict_keep_copy_occupied", "op_id", payload.OpID,
			"skill", t.name, "keep_copy", payload.KeepCopyName)
		return fmt.Errorf("%w: %s already exists; move it away and resolve %s again",
			ErrSkillInstallConflict, t.path(payload.KeepCopyName), payload.OpID)
	}
	if err := moveEditedAside(t, payload.KeepCopyName); err != nil {
		return err
	}
	if s.crashed("edited_kept") {
		return errSkillInstallCrashed
	}
	return nil
}

// commitResolution records the `resolve` log entry, then sets the journal to published. An entry an
// interrupted run already appended is kept when it names the same writer and corrected in place
// otherwise, so the log never names a tree the resolution does not leave installed.
func (s *SkillProposalService) commitResolution(
	ctx context.Context,
	t *skillTarget,
	journal *dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return err
	}
	entry := skillResolutionEntry(s.HomeID, payload)
	n := len(entries)
	switch {
	case n == 0 || entries[n-1].OpID != payload.OpID || entries[n-1].Action != SkillInstallActionResolve:
		err = appendSkillInstallLog(t.root, t.name, entry)
	case !sameSkillWriter(entries[n-1].Current, payload.NextMarker):
		entry.At = entries[n-1].At
		entries[n-1] = entry
		err = writeSkillInstallLog(t.root, t.name, entries)
	}
	if err != nil {
		return err
	}
	if s.crashed("resolve_logged") {
		return errSkillInstallCrashed
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	journal.Payload = encoded
	return dirpublish.AdvancePhase(t.parent, journal, dirpublish.PhasePublished)
}

func skillResolutionEntry(homeID string, payload SkillInstallPayload) skillInstallLogEntry {
	entry := skillInstallLogEntry{
		OpID: payload.OpID, HomeID: homeID, Action: SkillInstallActionResolve,
		DecidedVia: skillDecidedViaOperator, DecidedBy: payload.ResolvedBy,
		Current: payload.NextMarker, Previous: payload.PrevMarker, Keep: payload.Keep, At: time.Now().UTC(),
	}
	if entry.Current != nil {
		entry.ReviewID, entry.InstanceID = entry.Current.ReviewID, entry.Current.InstanceID
		entry.InstalledDigest = entry.Current.InstalledDigest
	}
	return entry
}

func sameSkillWriter(a, b *SkillInstallMarker) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// resume is Reconcile for a resolving journal: it finishes the recorded choice. A resolution that
// cannot finish (such as an occupied keep-copy) stays resolving and keeps the skill blocked.
func (p *skillRecoveryPolicy) resume(journal dirpublish.Journal, payload SkillInstallPayload) error {
	warning, err := p.s.continueResolution(p.ctx, p.t, journal, payload)
	if errors.Is(err, errSkillInstallCrashed) {
		return err
	}
	if err != nil {
		slog.WarnContext(p.ctx, "skill.install.conflict_resolution_blocked", "op_id", payload.OpID,
			"skill", p.t.name, "keep", payload.Keep, "error", err)
		p.blocked = &payload
		return nil
	}
	if warning != nil {
		p.logged(payload, "resolved_with_warning")
		return nil
	}
	p.logged(payload, "resolved")
	return nil
}
