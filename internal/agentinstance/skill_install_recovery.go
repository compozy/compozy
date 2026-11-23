package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
)

// reconcileSkill is Reconcile for one skill held under both locks: it recovers an interrupted
// operation from the on-disk journal, then projects this home's install-log entries. blocked is
// the journal payload of a retained conflict; warning carries projection failures, which never
// change files.
func (s *SkillProposalService) reconcileSkill(
	ctx context.Context,
	t *skillTarget,
) (blocked *SkillInstallPayload, warning error, err error) {
	if t.parent == nil {
		return nil, nil, nil
	}
	if blocked, err = s.recoverSkill(ctx, t); err != nil {
		return nil, nil, err
	}
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range entries {
		warning = errors.Join(warning, s.projectSkillInstall(ctx, entry))
	}
	return blocked, warning, nil
}

// recoverSkill applies the skill recovery policy to the journal of t, if any.
func (s *SkillProposalService) recoverSkill(ctx context.Context, t *skillTarget) (*SkillInstallPayload, error) {
	policy := &skillRecoveryPolicy{ctx: ctx, s: s, t: t}
	if err := dirpublish.Recover(t.parent, t.name, policy); err != nil {
		return nil, err
	}
	return policy.blocked, nil
}

// ReconcileTargetRoot runs Reconcile for every skill of targetRoot that has a journal or an install
// log. The daemon calls it at startup for each target root of this home before serving. A skill
// whose lock another process holds is skipped (that process reconciles it); conflicts stay retained.
func (s *SkillProposalService) ReconcileTargetRoot(ctx context.Context, targetRoot string) error {
	names, err := reconcilableSkillNames(targetRoot)
	if err != nil || len(names) == 0 {
		return err
	}
	var result error
	for _, name := range names {
		t, err := s.acquireSkill(ctx, targetRoot, name, false)
		if errors.Is(err, ErrSkillInstallBusy) {
			continue
		}
		if err != nil {
			result = errors.Join(result, fmt.Errorf("reconcile skill %s: %w", name, err))
			continue
		}
		_, warning, err := s.reconcileSkill(ctx, t)
		if warning != nil {
			slog.WarnContext(ctx, "skill.install.projection_failed", "skill", name, "error", warning)
		}
		result = errors.Join(result, err, t.close())
	}
	return result
}

// reconcilableSkillNames lists the skills of root with a publication journal or an install log.
func reconcilableSkillNames(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if proposedSkillName.MatchString(name) && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".compozy-publication-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var journal struct {
			TargetName string `json:"target_name"`
		}
		data, _, err := fileutil.ReadRegularFile(filepath.Join(root, entry.Name()), 1<<16)
		if err == nil && json.Unmarshal(data, &journal) == nil &&
			dirpublish.JournalName(journal.TargetName) == entry.Name() {
			add(journal.TargetName)
		}
	}
	logs, err := os.ReadDir(filepath.Join(root, skillInstallLogDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entry := range logs {
		if name, ok := strings.CutSuffix(entry.Name(), ".jsonl"); ok {
			add(name)
		}
	}
	return names, nil
}

// skillRecoveryPolicy is the skills' dirpublish recovery policy: roll back anything before
// published, only finalize at published, and retain unexplained contents as a conflict.
type skillRecoveryPolicy struct {
	ctx     context.Context
	s       *SkillProposalService
	t       *skillTarget
	blocked *SkillInstallPayload
}

var _ dirpublish.RecoveryPolicy = (*skillRecoveryPolicy)(nil)

func (p *skillRecoveryPolicy) RecoverLayout(
	_ *fileutil.Directory,
	journal dirpublish.Journal,
	_ dirpublish.Layout,
) error {
	payload, err := decodeSkillInstallPayload(journal.Payload)
	if err != nil {
		return dirpublish.RecoveryError("decode skill install payload", err)
	}
	switch {
	case payload.State == skillInstallStateConflict:
		p.blocked = &payload
		return nil
	case payload.State == skillInstallStateResolving:
		return p.resume(journal, payload)
	case journal.Phase == dirpublish.PhasePublished:
		if err := finalizeSkillOp(p.t, journal, payload); err != nil {
			return dirpublish.RecoveryError("finalize committed skill install", err)
		}
		p.logged(payload, "finalized")
		return nil
	}
	return p.rollback(journal, payload)
}

// rollback applies the rollback table of an uncommitted operation, rows evaluated in order. Before
// backup_staged nothing has moved (the target is moved only after that phase is durable), so an
// earlier journal only needs its own leftovers removed, whatever the live target holds.
func (p *skillRecoveryPolicy) rollback(journal dirpublish.Journal, payload SkillInstallPayload) error {
	if journal.Phase == dirpublish.PhasePrepared || journal.Phase == dirpublish.PhaseIncoming {
		return p.finishRollback(journal, payload)
	}
	original := expectedSkillOriginal(payload)
	target, err := skillTreeState(p.t.dir)
	if err != nil {
		return dirpublish.RecoveryError("inspect skill target", err)
	}
	backup, err := skillTreeState(p.t.path(journal.BackupName))
	if err != nil {
		return dirpublish.RecoveryError("inspect skill backup", err)
	}
	staged := payload.StagedDigest
	switch {
	case target == original && backup == "":
		// Not yet moved, or the rollback already restored the original.
	case (target == "" || (staged != "" && target == staged)) && backup == original:
		if target != "" {
			if err := discardVerifiedChild(p.t, journal, p.t.name, staged); err != nil {
				return p.conflict(journal, payload)
			}
		}
		if payload.PreExists {
			if err := moveSkillChild(p.t.parent, journal.BackupName, p.t.name); err != nil {
				return dirpublish.RecoveryError("restore skill backup", err)
			}
		}
		if p.s.crashed("rollback_restored") {
			return errSkillInstallCrashed
		}
	default:
		return p.conflict(journal, payload)
	}
	return p.finishRollback(journal, payload)
}

func (p *skillRecoveryPolicy) finishRollback(journal dirpublish.Journal, payload SkillInstallPayload) error {
	if err := removeSkillOpLeftovers(p.t, journal, payload, true); err != nil {
		return dirpublish.RecoveryError("clean rolled back skill install", err)
	}
	if err := dirpublish.RemoveJournal(p.t.parent, p.t.name); err != nil {
		return dirpublish.RecoveryError("remove rolled back skill journal", err)
	}
	p.logged(payload, "rolled_back")
	return nil
}

// conflict normalizes the conflict layout: the edited copy E ends at the target and the original
// stays in the backup slot, in the verified original copy, or is absent. The journal is kept with
// state = conflict until the operator resolves it.
func (p *skillRecoveryPolicy) conflict(journal dirpublish.Journal, payload SkillInstallPayload) error {
	target, err := skillTreeState(p.t.dir)
	if err != nil {
		return dirpublish.RecoveryError("inspect conflicting skill target", err)
	}
	backup, err := skillTreeState(p.t.path(journal.BackupName))
	if err != nil {
		return dirpublish.RecoveryError("inspect conflicting skill backup", err)
	}
	copyValid := p.originalCopyValid(payload)
	original := expectedSkillOriginal(payload)
	targetExplained := target == "" || target == original ||
		(payload.StagedDigest != "" && target == payload.StagedDigest)
	if targetExplained && backup != "" && backup != original {
		if copyValid, err = p.moveEditedToTarget(journal, payload, target, copyValid); err != nil {
			return err
		}
		backup = ""
	}
	payload.OriginalSource = skillOriginalAbsent
	switch {
	case payload.PreExists && backup == payload.PreDigest:
		payload.OriginalSource = skillOriginalInBackup
	case payload.PreExists && copyValid:
		payload.OriginalSource = skillOriginalInCopy
	}
	if err := removeSkillOpLeftovers(p.t, journal, payload, payload.OriginalSource != skillOriginalInCopy); err != nil {
		return dirpublish.RecoveryError("clean conflicting skill install", err)
	}
	payload.State = skillInstallStateConflict
	if payload.ConflictAt.IsZero() {
		payload.ConflictAt = time.Now().UTC()
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	journal.Payload = encoded
	if err := dirpublish.WriteJournal(p.t.parent, journal, true); err != nil {
		return dirpublish.RecoveryError("record skill install conflict", err)
	}
	slog.WarnContext(p.ctx, "skill.install.conflict", "op_id", payload.OpID, "skill", p.t.name,
		"review_id", payload.ReviewID, "action", payload.Action, "original_source", payload.OriginalSource)
	p.logged(payload, "conflict")
	p.blocked = &payload
	return nil
}

func (p *skillRecoveryPolicy) originalCopyValid(payload SkillInstallPayload) bool {
	if !payload.PreExists || payload.OriginalCopyName == "" {
		return false
	}
	digest, err := skillTreeState(p.t.path(payload.OriginalCopyName))
	return err == nil && digest != "" && digest == payload.PreDigest
}

// moveEditedToTarget handles E in the backup slot: it clears the target (keeping an original that
// has no verified copy as that copy), then moves E to the target. It reports whether the original
// copy is now valid.
func (p *skillRecoveryPolicy) moveEditedToTarget(
	journal dirpublish.Journal,
	payload SkillInstallPayload,
	target string,
	copyValid bool,
) (bool, error) {
	keepAsCopy := payload.PreExists && target != "" && target == payload.PreDigest && !copyValid
	switch {
	case keepAsCopy:
		if err := p.t.parent.RemoveAll(payload.OriginalCopyName); err != nil {
			return false, dirpublish.RecoveryError("clear partial original copy", err)
		}
		if err := moveSkillChild(p.t.parent, p.t.name, payload.OriginalCopyName); err != nil {
			return false, dirpublish.RecoveryError("keep original skill for conflict", err)
		}
		copyValid = true
	case target != "":
		if err := discardVerifiedChild(p.t, journal, p.t.name, target); err != nil {
			return false, dirpublish.RecoveryError("clear skill target for conflict", err)
		}
	}
	if err := moveSkillChild(p.t.parent, journal.BackupName, p.t.name); err != nil {
		return false, dirpublish.RecoveryError("move edited skill to target", err)
	}
	return copyValid, nil
}

// removeSkillOpLeftovers removes the incoming directory, optionally the original copy, and the
// uncommitted operation's install-log entries.
func removeSkillOpLeftovers(
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
	dropCopy bool,
) error {
	err := t.parent.RemoveAll(journal.IncomingName)
	if dropCopy && payload.OriginalCopyName != "" {
		err = errors.Join(err, t.parent.RemoveAll(payload.OriginalCopyName))
	}
	if err != nil {
		return err
	}
	return removeSkillInstallLogOp(t.root, t.name, payload.OpID)
}

// discardVerifiedChild moves name aside to the journal's incoming name and removes it only when its
// digest is want; other contents are moved back and reported.
func discardVerifiedChild(t *skillTarget, journal dirpublish.Journal, name, want string) error {
	if err := t.parent.RemoveAll(journal.IncomingName); err != nil {
		return err
	}
	if err := moveSkillChild(t.parent, name, journal.IncomingName); err != nil {
		return err
	}
	digest, err := skillTreeState(t.path(journal.IncomingName))
	if err != nil || digest != want {
		return errors.Join(errSkillInstallRaced, err, moveSkillChild(t.parent, journal.IncomingName, name))
	}
	return t.parent.RemoveAll(journal.IncomingName)
}

func (p *skillRecoveryPolicy) logged(payload SkillInstallPayload, outcome string) {
	slog.InfoContext(p.ctx, "skill.install.reconciled", "op_id", payload.OpID, "skill", p.t.name,
		"action", payload.Action, "outcome", outcome)
}
