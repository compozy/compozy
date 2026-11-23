package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

var (
	// ErrSkillInstallConflictNotFound means no retained conflict or completed resolution has the op ID.
	ErrSkillInstallConflictNotFound = errors.New("skill_install_conflict_not_found")
	// ErrSkillInstallConflictResolving means the conflict is being, or was, resolved with the other keep.
	ErrSkillInstallConflictResolving = errors.New("skill_install_conflict_resolving")
)

// Conflict resolution choices.
const (
	SkillConflictKeepCurrent  = "current"
	SkillConflictKeepPrevious = "previous"
)

const skillConflictCopyPrefix = ".skill-conflict-"

// SkillInstallConflict is one journal in conflict or resolving state found on disk.
type SkillInstallConflict struct {
	OpID       string
	Skill      string
	ReviewID   string
	Action     string
	State      string // conflict | resolving
	Keep       string // resolving only
	TargetRoot string
	Since      time.Time
}

// SkillConflictResolution is the result of ResolveInstallConflict.
type SkillConflictResolution struct {
	OpID  string
	Skill string
	Keep  string
	// KeptCopy is the absolute path of the edited copy kept aside by keep=previous.
	KeptCopy        string
	AlreadyResolved bool
	// Warning carries a finalize or catalog-publication failure after the commit point.
	Warning error
}

// InstallConflicts lists the retained conflicts under targetRoots, read from disk so conflicts left
// by any home are listed. Operator only.
func (s *SkillProposalService) InstallConflicts(
	_ context.Context,
	actor workspaceaccess.ActorRef,
	targetRoots []string,
) ([]SkillInstallConflict, error) {
	if !actor.Operator {
		return nil, ErrSkillReviewOperatorOnly
	}
	conflicts := []SkillInstallConflict{}
	err := walkSkillJournals(targetRoots, func(root, name string, payload SkillInstallPayload) bool {
		if payload.State == skillInstallStateConflict || payload.State == skillInstallStateResolving {
			conflicts = append(conflicts, SkillInstallConflict{
				OpID: payload.OpID, Skill: name, ReviewID: payload.ReviewID, Action: payload.Action,
				State: payload.State, Keep: payload.Keep, TargetRoot: root, Since: payload.ConflictAt,
			})
		}
		return false
	})
	return conflicts, err
}

// walkSkillJournals reads, without locks, the journal of every skill under roots and calls visit
// until it returns true.
func walkSkillJournals(roots []string, visit func(root, name string, payload SkillInstallPayload) bool) error {
	for _, root := range roots {
		names, err := reconcilableSkillNames(root)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			continue
		}
		parent, err := fileutil.OpenDirectory(root)
		if err != nil {
			return err
		}
		stop := false
		for _, name := range names {
			journal, exists, readErr := dirpublish.ReadJournal(parent, name)
			if readErr != nil || !exists {
				err = errors.Join(err, readErr)
				continue
			}
			payload, decodeErr := decodeSkillInstallPayload(journal.Payload)
			if decodeErr != nil {
				continue
			}
			if stop = visit(root, name, payload); stop {
				break
			}
		}
		if err = errors.Join(err, parent.Close()); err != nil || stop {
			return err
		}
	}
	return nil
}

// ResolveInstallConflict resolves the retained conflict opID found under targetRoots by keeping the
// current (edited) or the previous tree. The choice is recorded in the journal before anything
// moves, so Reconcile finishes an interrupted resolution. Repeating a completed resolution with the
// same keep returns AlreadyResolved and publishes the catalog again without touching files.
func (s *SkillProposalService) ResolveInstallConflict(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	targetRoots []string,
	opID, keep, operator string,
) (_ SkillConflictResolution, err error) {
	if !actor.Operator {
		return SkillConflictResolution{}, ErrSkillReviewOperatorOnly
	}
	if keep != SkillConflictKeepCurrent && keep != SkillConflictKeepPrevious {
		return SkillConflictResolution{}, fmt.Errorf("%w: keep must be current or previous", ErrSkillReviewInvalid)
	}
	if operator == "" {
		return SkillConflictResolution{}, errors.New("skill conflict operator identity is required")
	}
	root, name, err := findSkillInstallOp(targetRoots, opID)
	if err != nil {
		return SkillConflictResolution{}, err
	}
	t, err := s.acquireSkill(ctx, root, name, false)
	if err != nil {
		return SkillConflictResolution{}, err
	}
	defer func() { err = errors.Join(err, t.close()) }()
	if t.parent == nil {
		return SkillConflictResolution{}, ErrSkillInstallConflictNotFound
	}
	result := SkillConflictResolution{OpID: opID, Skill: name, Keep: keep}
	journal, exists, err := dirpublish.ReadJournal(t.parent, name)
	if err != nil {
		return SkillConflictResolution{}, err
	}
	payload := SkillInstallPayload{}
	if exists {
		if payload, err = decodeSkillInstallPayload(journal.Payload); err != nil {
			return SkillConflictResolution{}, err
		}
	}
	if !exists || payload.OpID != opID {
		return s.repeatResolution(ctx, t, result)
	}
	switch payload.State {
	case skillInstallStateConflict:
		if journal, payload, err = s.recordResolution(t, journal, payload, keep, operator); err != nil {
			return SkillConflictResolution{}, err
		}
	case skillInstallStateResolving:
		if payload.Keep != keep {
			return SkillConflictResolution{}, skillConflictResolvingError(payload.Keep)
		}
	default:
		return SkillConflictResolution{}, ErrSkillInstallConflictNotFound
	}
	if keep == SkillConflictKeepPrevious {
		result.KeptCopy = t.path(payload.KeepCopyName)
	}
	result.Warning, err = s.continueResolution(ctx, t, journal, payload)
	return result, err
}

// findSkillInstallOp locates the skill of opID: a journal carrying it, or a completed resolution in
// an install log.
func findSkillInstallOp(roots []string, opID string) (string, string, error) {
	var root, name string
	err := walkSkillJournals(roots, func(r, n string, payload SkillInstallPayload) bool {
		if payload.OpID == opID {
			root, name = r, n
			return true
		}
		return false
	})
	if err != nil || root != "" {
		return root, name, err
	}
	for _, r := range roots {
		names, err := reconcilableSkillNames(r)
		if err != nil {
			return "", "", err
		}
		for _, n := range names {
			entries, err := readSkillInstallLog(r, n)
			if err != nil {
				return "", "", err
			}
			if resolvedSkillEntry(entries, opID) != nil {
				return r, n, nil
			}
		}
	}
	return "", "", ErrSkillInstallConflictNotFound
}

func resolvedSkillEntry(entries []skillInstallLogEntry, opID string) *skillInstallLogEntry {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].OpID == opID && entries[i].Action == SkillInstallActionResolve {
			return &entries[i]
		}
	}
	return nil
}

// repeatResolution answers a resolve whose journal is gone from the install log.
func (s *SkillProposalService) repeatResolution(
	ctx context.Context,
	t *skillTarget,
	result SkillConflictResolution,
) (SkillConflictResolution, error) {
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return SkillConflictResolution{}, err
	}
	entry := resolvedSkillEntry(entries, result.OpID)
	if entry == nil {
		return SkillConflictResolution{}, ErrSkillInstallConflictNotFound
	}
	// keep=current is the only resolution that records an operator-authored writer of this op.
	kept := SkillConflictKeepPrevious
	if entry.Current != nil && entry.Current.ReviewID == "" && entry.Current.OpID == result.OpID {
		kept = SkillConflictKeepCurrent
	}
	if kept != result.Keep {
		return SkillConflictResolution{}, skillConflictResolvingError(kept)
	}
	result.AlreadyResolved = true
	if result.Warning = s.publishSkills(ctx); result.Warning != nil {
		slog.WarnContext(ctx, "skill.install.committed_with_warning", "op_id", result.OpID,
			"action", SkillInstallActionResolve, "error", result.Warning)
	}
	return result, nil
}

// recordResolution sets the journal to resolving with the operator's choice before anything moves.
func (s *SkillProposalService) recordResolution(
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
	keep, operator string,
) (dirpublish.Journal, SkillInstallPayload, error) {
	payload.State, payload.Keep, payload.ResolvedBy = skillInstallStateResolving, keep, operator
	payload.KeepCopyName = skillConflictCopyPrefix + payload.OpID
	payload.NextMarker = payload.PrevMarker
	if keep == SkillConflictKeepCurrent {
		edited, err := skillTreeState(t.dir)
		if err != nil {
			return journal, payload, err
		}
		payload.NextMarker = nil
		if edited != "" {
			payload.NextMarker = &SkillInstallMarker{
				HomeID: s.HomeID, InstalledDigest: edited, DecidedVia: skillDecidedViaOperator,
				DecidedBy: operator, OpID: payload.OpID,
			}
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return journal, payload, err
	}
	journal.Payload = encoded
	if err := dirpublish.WriteJournal(t.parent, journal, true); err != nil {
		return journal, payload, fmt.Errorf("record skill conflict resolution: %w", err)
	}
	if s.crashed("resolving_recorded") {
		return journal, payload, errSkillInstallCrashed
	}
	return journal, payload, nil
}

// continueResolution idempotently finishes a resolving journal: move the trees for keep, commit the
// `resolve` log entry and the published phase, then finalize and publish.
func (s *SkillProposalService) continueResolution(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) (warning error, err error) {
	if journal.Phase != dirpublish.PhasePublished {
		if payload.Keep == SkillConflictKeepPrevious {
			err = s.restorePrevious(ctx, t, journal, payload)
		} else {
			err = t.parent.RemoveAll(journal.BackupName)
			if err == nil && s.crashed("backup_removed") {
				err = errSkillInstallCrashed
			}
		}
		if err != nil {
			return nil, err
		}
		if err := s.commitResolution(t, &journal, payload); err != nil {
			return nil, err
		}
	}
	warning = errors.Join(finalizeSkillOp(t, journal, payload), s.publishSkills(ctx))
	slog.InfoContext(ctx, "skill.install.conflict_resolved", "op_id", payload.OpID, "skill", t.name,
		"keep", payload.Keep)
	if warning != nil {
		slog.WarnContext(ctx, "skill.install.committed_with_warning", "op_id", payload.OpID,
			"action", SkillInstallActionResolve, "error", warning)
	}
	return warning, nil
}

// restorePrevious moves the edited tree to the keep-copy directory and the original back to the
// target. An occupied keep-copy is never overwritten: the journal stays resolving.
func (s *SkillProposalService) restorePrevious(
	ctx context.Context,
	t *skillTarget,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	target, err := skillTreeState(t.dir)
	if err != nil {
		return err
	}
	if target != "" && target != expectedSkillOriginal(payload) {
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
		if err := moveSkillChild(t.parent, t.name, payload.KeepCopyName); err != nil {
			return err
		}
		if s.crashed("edited_kept") {
			return errSkillInstallCrashed
		}
		target = ""
	}
	if target != "" || !payload.PreExists {
		return nil
	}
	switch payload.OriginalSource {
	case skillOriginalInBackup:
		err = moveSkillChild(t.parent, journal.BackupName, t.name)
	case skillOriginalInCopy:
		digest, digestErr := skillTreeState(t.path(payload.OriginalCopyName))
		if digestErr != nil || digest != payload.PreDigest {
			return errors.Join(errors.New("the original copy no longer matches the previous skill"), digestErr)
		}
		err = moveSkillChild(t.parent, payload.OriginalCopyName, t.name)
	}
	if err == nil && s.crashed("original_restored") {
		err = errSkillInstallCrashed
	}
	return err
}

// commitResolution appends the `resolve` log entry once, then sets the journal to published.
func (s *SkillProposalService) commitResolution(
	t *skillTarget,
	journal *dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return err
	}
	if n := len(
		entries,
	); n == 0 || entries[n-1].OpID != payload.OpID ||
		entries[n-1].Action != SkillInstallActionResolve {
		entry := skillInstallLogEntry{
			OpID: payload.OpID, HomeID: s.HomeID, Action: SkillInstallActionResolve,
			DecidedVia: skillDecidedViaOperator, DecidedBy: payload.ResolvedBy,
			Current: payload.NextMarker, Previous: payload.PrevMarker, At: time.Now().UTC(),
		}
		if entry.Current != nil {
			entry.ReviewID, entry.InstanceID = entry.Current.ReviewID, entry.Current.InstanceID
			entry.InstalledDigest = entry.Current.InstalledDigest
		}
		if err := appendSkillInstallLog(t.root, t.name, entry); err != nil {
			return err
		}
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

func skillConflictResolvingError(keep string) error {
	return fmt.Errorf(
		"%w: resolved with --keep %s; repeat with the same --keep",
		ErrSkillInstallConflictResolving,
		keep,
	)
}
