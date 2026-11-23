package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"regexp"
	"time"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

// Journal payload states.
const (
	skillInstallStateActive    = "active"
	skillInstallStateConflict  = "conflict"
	skillInstallStateResolving = "resolving"
)

// Conflict OriginalSource values.
const (
	skillOriginalInBackup = "backup"
	skillOriginalInCopy   = "copy"
	skillOriginalAbsent   = "absent"
)

const skillOriginalCopyPrefix = ".compozy-original-"

// errSkillInstallCrashed stops a transaction exactly where a test seam simulated a crash, without the
// rollback a returning error would run.
var errSkillInstallCrashed = errors.New("skill install crashed (test seam)")

// errSkillInstallRaced reports that the moved-aside tree is not the checked original.
var errSkillInstallRaced = errors.New("skill changed while it was being replaced")

// errSkillCommitUncertain reports a published journal whose durability could not be confirmed. The
// transaction neither rolls back nor projects; the next Reconcile decides from disk.
var errSkillCommitUncertain = errors.New("skill install commit is not confirmed durable")

// SkillInstallPayload is the opaque payload carried in the dirpublish journal (≤ 2 KiB).
type SkillInstallPayload struct {
	OpID         string `json:"op_id"`
	ReviewID     string `json:"review_id"`
	Action       string `json:"action"` // install | revert | resolve
	PreExists    bool   `json:"pre_exists"`
	PreDigest    string `json:"pre_digest"`    // tree digest v1 before the operation; "" when !PreExists
	StagedDigest string `json:"staged_digest"` // "" when nothing is staged
	// PrevMarker is the current writer before the operation; NextMarker the one written at commit
	// (nil = the skill is removed).
	PrevMarker *SkillInstallMarker `json:"prev_marker"`
	NextMarker *SkillInstallMarker `json:"next_marker"`
	State      string              `json:"state"` // active | conflict | resolving
	// OriginalSource locates the original of a conflict: backup | copy | absent.
	OriginalSource string `json:"original_source,omitempty"`
	// OriginalCopyName is ".compozy-original-<op-id>", the verified copy of the original next to the
	// target; "" when !PreExists.
	OriginalCopyName string `json:"original_copy_name,omitempty"`
	Keep             string `json:"keep,omitempty"`           // resolving only: current | previous
	KeepCopyName     string `json:"keep_copy_name,omitempty"` // resolving only: ".skill-conflict-<op-id>"
	// RetainedBackupName is ".skill-conflict-<op-id>-backup": unexplained contents found in the backup
	// slot while the target also held unexplained contents. No resolution removes it.
	RetainedBackupName string `json:"retained_backup_name,omitempty"`
	// ConflictAt is when the conflict was first retained; ResolvedBy the operator resolving it.
	ConflictAt time.Time `json:"conflict_at,omitzero"`
	ResolvedBy string    `json:"resolved_by,omitempty"`
}

func decodeSkillInstallPayload(data []byte) (SkillInstallPayload, error) {
	var payload SkillInstallPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return SkillInstallPayload{}, err
	}
	if payload.State == "" || !skillOpIDPattern.MatchString(payload.OpID) {
		return SkillInstallPayload{}, errors.New("skill install payload is incomplete or names an invalid op")
	}
	// Recovery removes and moves these names, so each must be exactly the one derived from the op.
	for _, name := range []struct{ got, want string }{
		{payload.OriginalCopyName, skillOriginalCopyPrefix + payload.OpID},
		{payload.KeepCopyName, skillConflictCopyPrefix + payload.OpID},
		{payload.RetainedBackupName, skillConflictCopyPrefix + payload.OpID + "-backup"},
	} {
		if name.got != "" && name.got != name.want {
			return SkillInstallPayload{}, fmt.Errorf("skill install payload name %q does not belong to op %s",
				name.got, payload.OpID)
		}
	}
	return payload, nil
}

// skillOpIDPattern matches the op IDs runSkillOp writes ("sop-<hex>") and keeps every name derived
// from them a single safe path component.
var skillOpIDPattern = regexp.MustCompile(`^sop[-_][A-Za-z0-9_-]{1,64}$`)

// SkillInstallResult describes a successful install-transaction call.
type SkillInstallResult struct {
	// Committed is true when this call moved files and appended the install-log entry; false for an
	// idempotent repeat that wrote nothing.
	Committed bool
	// Warning carries a projection, cleanup, or catalog-publication failure after the commit point.
	// The files stay committed; Reconcile retries the projection.
	Warning error
}

// skillOp is one filesystem mutation of a skill run through the transaction.
type skillOp struct {
	reviewID, instanceID  string
	action                string
	decidedVia, decidedBy string
	preExists             bool
	preDigest             string
	// changed is the step-2 error returned when the live tree no longer matches the expectation.
	changed error
	// stage writes the incoming tree into an existing empty directory; nil stages nothing (the
	// operation removes the skill).
	stage func(dir string) error
	prev  *SkillInstallMarker
	// next is the writer recorded at commit; empty OpID and InstalledDigest are filled from this
	// operation. nil removes the writer.
	next *SkillInstallMarker
}

type skillTx struct {
	s       *SkillProposalService
	t       *skillTarget
	op      skillOp
	journal dirpublish.Journal
	payload SkillInstallPayload
}

// runSkillOp runs steps 3–6 of the install transaction under both locks, after Reconcile and the
// step-2 checks. Any failure before the commit point rolls back through the recovery policy.
func (s *SkillProposalService) runSkillOp(ctx context.Context, t *skillTarget, op skillOp) (SkillInstallResult, error) {
	opID, err := store.NewID("sop")
	if err != nil {
		return SkillInstallResult{}, err
	}
	tx := &skillTx{s: s, t: t, op: op, payload: SkillInstallPayload{
		OpID: opID, ReviewID: op.reviewID, Action: op.action, PreExists: op.preExists, PreDigest: op.preDigest,
		PrevMarker: op.prev, NextMarker: op.next, State: skillInstallStateActive,
	}}
	if op.preExists {
		tx.payload.OriginalCopyName = skillOriginalCopyPrefix + opID
	}
	if tx.payload.NextMarker != nil {
		next := *tx.payload.NextMarker
		if next.OpID == "" {
			next.OpID = opID
		}
		tx.payload.NextMarker = &next
	}
	encoded, err := json.Marshal(tx.payload)
	if err != nil {
		return SkillInstallResult{}, err
	}
	if tx.journal, err = dirpublish.NewJournal(t.name, encoded); err != nil {
		return SkillInstallResult{}, err
	}
	// The journal names every directory before any of them exists.
	if err := dirpublish.WriteJournal(t.parent, tx.journal, false); err != nil {
		return SkillInstallResult{}, err
	}
	if err := tx.prepare(); err != nil {
		return SkillInstallResult{}, tx.abort(ctx, err)
	}
	// A canceled request stops before the target moves; abort removes the stage and the journal.
	if err := ctx.Err(); err != nil {
		return SkillInstallResult{}, tx.abort(ctx, err)
	}
	if err := tx.swap(); err != nil {
		return SkillInstallResult{}, tx.abort(ctx, err)
	}
	entry, err := tx.commit()
	if err != nil {
		return SkillInstallResult{}, tx.abort(ctx, err)
	}
	if s.crashed("published") {
		return SkillInstallResult{}, errSkillInstallCrashed
	}
	return SkillInstallResult{Committed: true, Warning: s.afterCommit(ctx, t, entry, tx.journal, tx.payload)}, nil
}

// prepare copies the original aside (verified against PreDigest) and stages the incoming tree.
func (tx *skillTx) prepare() error {
	if tx.s.crashed("prepared") {
		return errSkillInstallCrashed
	}
	if tx.op.preExists {
		copyPath := tx.t.path(tx.payload.OriginalCopyName)
		if err := copySkillTree(tx.t.dir, copyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if tx.s.crashed("original_copied") {
			return errSkillInstallCrashed
		}
		digest, err := skillTreeState(copyPath)
		if err != nil || digest != tx.op.preDigest {
			return errors.Join(tx.op.changed, err)
		}
	}
	if tx.op.stage == nil {
		return tx.advance(dirpublish.PhaseIncoming)
	}
	incoming := tx.t.path(tx.journal.IncomingName)
	if err := os.Mkdir(incoming, 0o700); err != nil {
		return err
	}
	if tx.s.crashed("incoming_created") {
		return errSkillInstallCrashed
	}
	if err := tx.op.stage(incoming); err != nil {
		return err
	}
	if err := syncSkillTree(incoming); err != nil {
		return err
	}
	staged, err := SkillTreeDigest(incoming)
	if err != nil {
		return err
	}
	tx.payload.StagedDigest = staged
	if tx.payload.NextMarker != nil && tx.payload.NextMarker.InstalledDigest == "" {
		tx.payload.NextMarker.InstalledDigest = staged
	}
	return tx.advance(dirpublish.PhaseIncoming)
}

// swap re-checks the live target, moves it to the backup slot (verified), and publishes incoming.
func (tx *skillTx) swap() error {
	if tx.s.crashed("incoming_ready") {
		return errSkillInstallCrashed
	}
	live, err := skillTreeState(tx.t.dir)
	if err != nil {
		return errors.Join(tx.op.changed, err)
	}
	if live != expectedSkillOriginal(tx.payload) {
		return tx.op.changed
	}
	if tx.s.crashed("rechecked") {
		return errSkillInstallCrashed
	}
	if err := tx.advance(dirpublish.PhaseBackup); err != nil {
		return err
	}
	if tx.op.preExists {
		if err := moveSkillChild(tx.t.parent, tx.t.name, tx.journal.BackupName); err != nil {
			return err
		}
		if tx.s.crashed("backup_moved") {
			return errSkillInstallCrashed
		}
		backup, err := skillTreeState(tx.t.path(tx.journal.BackupName))
		if err != nil || backup != tx.op.preDigest {
			return errors.Join(errSkillInstallRaced, err)
		}
	}
	if tx.op.stage != nil {
		if err := moveSkillChild(tx.t.parent, tx.journal.IncomingName, tx.t.name); err != nil {
			return err
		}
		if tx.s.crashed("target_moved") {
			return errSkillInstallCrashed
		}
	}
	return nil
}

// commit re-checks that the live target is the staged tree, appends the install-log entry, then
// sets the journal to published: the single commit point.
func (tx *skillTx) commit() (skillInstallLogEntry, error) {
	live, err := recoveryTreeState(tx.t.dir)
	if err != nil || live != tx.payload.StagedDigest {
		return skillInstallLogEntry{}, errors.Join(errSkillInstallRaced, err)
	}
	entry := skillInstallLogEntry{
		OpID: tx.payload.OpID, HomeID: tx.s.HomeID, InstanceID: tx.op.instanceID, Action: tx.op.action,
		ReviewID: tx.op.reviewID, DecidedVia: tx.op.decidedVia, DecidedBy: tx.op.decidedBy,
		Current: tx.payload.NextMarker, Previous: tx.payload.PrevMarker, At: time.Now().UTC(),
	}
	if entry.Current != nil {
		entry.InstalledDigest = entry.Current.InstalledDigest
	}
	if err := appendSkillInstallLog(tx.t.root, tx.t.name, entry); err != nil {
		return skillInstallLogEntry{}, err
	}
	if tx.s.crashed("logged") {
		return skillInstallLogEntry{}, errSkillInstallCrashed
	}
	err = tx.advance(dirpublish.PhasePublished)
	if err == nil && tx.s.crashed("published_unsynced") {
		err = fileutil.ErrPublishedNotDurable
	}
	if err == nil {
		return entry, nil
	}
	return entry, tx.confirmPublished(err)
}

// confirmPublished handles a failed published write. A journal still unpublished rolls back. A
// published one counts only after the directory sync succeeds: until then a crash could revert it,
// so the transaction neither rolls back nor projects and leaves the decision to Reconcile.
func (tx *skillTx) confirmPublished(writeErr error) error {
	journal, exists, readErr := dirpublish.ReadJournal(tx.t.parent, tx.t.name)
	if readErr != nil || !exists || journal.Phase != dirpublish.PhasePublished {
		return errors.Join(writeErr, readErr)
	}
	tx.journal = journal
	syncErr := tx.t.parent.Sync()
	if tx.s.crashed("published_sync_failed") {
		syncErr = errors.New("simulated directory sync failure")
	}
	if syncErr != nil {
		return fmt.Errorf("%w: %w", errSkillCommitUncertain, errors.Join(writeErr, syncErr))
	}
	return nil
}

func (tx *skillTx) advance(phase string) error {
	encoded, err := json.Marshal(tx.payload)
	if err != nil {
		return err
	}
	tx.journal.Payload = encoded
	return dirpublish.AdvancePhase(tx.t.parent, &tx.journal, phase)
}

// abort rolls an uncommitted transaction back through Reconcile's recovery policy. A simulated
// crash leaves everything for the next Reconcile.
func (tx *skillTx) abort(ctx context.Context, cause error) error {
	if errors.Is(cause, errSkillInstallCrashed) || errors.Is(cause, errSkillCommitUncertain) {
		return cause
	}
	blocked, err := tx.s.recoverSkill(ctx, tx.t)
	if blocked != nil {
		return errors.Join(skillInstallConflictError(tx.t.name, blocked.OpID), err)
	}
	if errors.Is(cause, errSkillInstallRaced) {
		return errors.Join(tx.op.changed, err)
	}
	return errors.Join(cause, err)
}

// afterCommit is step 6: projection, finalize, publication. None of them can undo the commit.
func (s *SkillProposalService) afterCommit(
	ctx context.Context,
	t *skillTarget,
	entry skillInstallLogEntry,
	journal dirpublish.Journal,
	payload SkillInstallPayload,
) error {
	warning := s.projectSkillInstall(ctx, entry)
	warning = errors.Join(warning, finalizeSkillOp(t, journal, payload))
	warning = errors.Join(warning, s.publishSkills(ctx))
	if warning != nil {
		slog.WarnContext(ctx, "skill.install.committed_with_warning", "op_id", entry.OpID,
			"review_id", entry.ReviewID, "action", entry.Action, "error", warning)
	}
	return warning
}

// publishSkills refreshes the catalog. A failure leaves the refresh pending for this daemon, and the
// next Reconcile of any skill retries it without touching files; a daemon restart rebuilds the
// catalog from disk.
func (s *SkillProposalService) publishSkills(ctx context.Context) error {
	if s.Publisher == nil {
		return nil
	}
	err := s.Publisher.SkillPublished(ctx)
	s.installLocks().publishPending.Store(err != nil)
	return err
}

// projectSkillInstall projects one install-log entry of this home into the store; idempotent.
func (s *SkillProposalService) projectSkillInstall(ctx context.Context, entry skillInstallLogEntry) error {
	if entry.HomeID != s.HomeID || entry.Action != SkillInstallActionInstall || entry.Current == nil ||
		entry.Current.ReviewID == "" {
		return nil
	}
	var previous []byte
	if entry.Previous != nil {
		encoded, err := json.Marshal(entry.Previous)
		if err != nil {
			return err
		}
		previous = encoded
	}
	if err := s.Store.ProjectApproval(ctx, *entry.Current, entry.InstalledDigest, previous, entry.At); err != nil {
		return fmt.Errorf("project skill install %s: %w", entry.OpID, err)
	}
	return nil
}

// finalizeSkillOp trims the install log, removes the backup (retaining it when it no longer holds
// the verified original), the original copy, and then the journal of a committed operation.
func finalizeSkillOp(t *skillTarget, journal dirpublish.Journal, payload SkillInstallPayload) error {
	if err := trimSkillInstallLog(t.root, t.name); err != nil {
		return err
	}
	if err := retainChangedBackup(t, journal, payload); err != nil {
		return err
	}
	err := errors.Join(t.parent.RemoveAll(journal.BackupName), t.parent.RemoveAll(journal.IncomingName))
	if payload.OriginalCopyName != "" {
		err = errors.Join(err, t.parent.RemoveAll(payload.OriginalCopyName))
	}
	if err != nil {
		return err
	}
	return dirpublish.RemoveJournal(t.parent, t.name)
}

// retainChangedBackup moves a backup edited after its verification to a sibling the operator can
// inspect, instead of deleting the only copy of that edit.
func retainChangedBackup(t *skillTarget, journal dirpublish.Journal, payload SkillInstallPayload) error {
	backup, err := recoveryTreeState(t.path(journal.BackupName))
	if err != nil {
		return err
	}
	if backup == "" || backup == expectedSkillOriginal(payload) {
		return nil
	}
	name := skillConflictCopyPrefix + payload.OpID + "-edited-backup"
	if err := moveSkillChild(t.parent, journal.BackupName, name); err != nil {
		return fmt.Errorf("retain edited skill backup: %w", err)
	}
	slog.Warn("skill.install.backup_retained", "op_id", payload.OpID, "skill", t.name, "retained", name)
	return nil
}

// expectedSkillOriginal is the digest of the original target, "" when it did not exist.
func expectedSkillOriginal(payload SkillInstallPayload) string {
	if !payload.PreExists {
		return ""
	}
	return payload.PreDigest
}

// moveEditedAside moves the skill entry at the target to name: a symlink entry is renamed itself,
// never its referent; anything else moves as a directory.
func moveEditedAside(t *skillTarget, name string) error {
	info, err := os.Lstat(t.dir)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return t.parent.MoveSymlinkNoReplace(t.name, name)
	}
	return moveSkillChild(t.parent, t.name, name)
}

// moveSkillChild renames a direct child directory without replacing an existing name.
func moveSkillChild(parent *fileutil.Directory, from, to string) error {
	dir, err := parent.OpenDirectoryForMove(from)
	if err != nil {
		return err
	}
	result, moveErr := dir.MoveTo(parent, to, false)
	if result.Committed() {
		// A post-commit durability failure never means the move may be undone.
		moveErr = nil
	}
	return errors.Join(moveErr, dir.Close())
}

func skillInstallConflictError(name, opID string) error {
	return fmt.Errorf(
		"%w: %s changed while it was being replaced (op %s); run `compozy agent skill install-conflict resolve %s --keep current|previous`",
		ErrSkillInstallConflict,
		name,
		opID,
		opID,
	)
}

func (s *SkillProposalService) crashed(step string) bool {
	return s.crashAt != nil && s.crashAt(step)
}
