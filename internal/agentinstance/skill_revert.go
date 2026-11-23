package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// SkillReviewReverted is the state of an approved review whose install was undone.
const SkillReviewReverted = "reverted"

const skillReviewApproved = "approved"

// Revert eligibility reasons (Business Rule 11).
const (
	SkillRevertNotApproved       = "not_approved"
	SkillRevertLegacy            = "legacy"
	SkillRevertSuperseded        = "superseded"
	SkillRevertChangedOnDisk     = "changed_on_disk"
	SkillRevertTargetUnavailable = "target_unavailable"
)

// ErrSkillReviewRevertConflict refuses a revert the install log or the live tree does not allow.
var ErrSkillReviewRevertConflict = errors.New("skill_review_revert_conflict")

// SkillRevertConflictError carries the eligibility reason of a refused revert.
type SkillRevertConflictError struct {
	Name   string
	Reason string
}

func (e *SkillRevertConflictError) Error() string {
	var detail string
	switch e.Reason {
	case SkillRevertSuperseded:
		detail = e.Name + " has a newer approved change; revert that one first"
	case SkillRevertChangedOnDisk:
		detail = e.Name + " changed after this review was approved; revert the latest change or edit the skill directly"
	case SkillRevertLegacy:
		detail = e.Name + " was approved before reverts were recorded and cannot be reverted"
	case SkillRevertTargetUnavailable:
		detail = "the previous contents of " + e.Name + " are unavailable; edit the skill directly"
	default:
		detail = e.Name + " is not approved"
	}
	return fmt.Sprintf("%s: %s", ErrSkillReviewRevertConflict, detail)
}

func (e *SkillRevertConflictError) Unwrap() error { return ErrSkillReviewRevertConflict }

// SkillRevertReason returns the eligibility reason of a revert conflict, "" for other errors.
func SkillRevertReason(err error) string {
	if conflict, ok := errors.AsType[*SkillRevertConflictError](err); ok {
		return conflict.Reason
	}
	return ""
}

// RevertEligibility says whether an approval can be reverted now; Reason is "" when Allowed.
type RevertEligibility struct {
	Allowed bool
	Reason  string
}

// RevertEligibility reads the install log and the live tree without locks; Revert evaluates the same
// rule under both skill locks before writing.
func (s *SkillProposalService) RevertEligibility(
	_ context.Context,
	review SkillReview,
	targetRoot string,
) (RevertEligibility, error) {
	if reason := revertPrecondition(review, targetRoot); reason != "" {
		return RevertEligibility{Reason: reason}, nil
	}
	root, err := fileutil.CanonicalPathWithExistingPrefix(targetRoot)
	if err != nil {
		return RevertEligibility{}, err
	}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return RevertEligibility{Reason: SkillRevertTargetUnavailable}, nil
	}
	return readRevertEligibility(review, root, review.SkillName)
}

// revertPrecondition is the part of eligibility that needs no files.
func revertPrecondition(review SkillReview, targetRoot string) string {
	switch {
	case review.State != skillReviewApproved:
		return SkillRevertNotApproved
	case review.InstalledDigest == "" || review.SkillKey == "":
		// Approved before the install log existed: no installed digest to compare.
		return SkillRevertLegacy
	case targetRoot == "" || !proposedSkillName.MatchString(review.SkillName):
		return SkillRevertTargetUnavailable
	}
	return ""
}

// readRevertEligibility applies the file part of Business Rule 11: the review must be the install
// log's current writer, the live tree must equal its installed digest, and an edit's base snapshot
// must still verify.
func readRevertEligibility(review SkillReview, root, name string) (RevertEligibility, error) {
	entries, err := readSkillInstallLog(root, name)
	if err != nil {
		return RevertEligibility{}, err
	}
	current := currentSkillWriter(entries)
	if current == nil || current.ReviewID != review.ID {
		return RevertEligibility{Reason: SkillRevertSuperseded}, nil
	}
	live, err := skillTreeState(filepath.Join(root, name))
	if errors.Is(err, ErrSkillTreeUnsupported) {
		return RevertEligibility{Reason: SkillRevertChangedOnDisk}, nil
	}
	if err != nil {
		return RevertEligibility{}, err
	}
	if live == "" || live != current.InstalledDigest || live != review.InstalledDigest {
		return RevertEligibility{Reason: SkillRevertChangedOnDisk}, nil
	}
	if review.Kind == SkillProposalEdit {
		if _, err := readSkillSnapshot(review); err != nil {
			return RevertEligibility{Reason: SkillRevertTargetUnavailable}, nil
		}
	}
	return RevertEligibility{Allowed: true}, nil
}

// Revert undoes the latest approval of a skill through the install transaction: a `new` review's
// skill is removed; an `edit` restores its verified base snapshot byte- and mode-exact, and the
// review's previous writer becomes the current writer again. Reverting a reverted review is an
// idempotent success with no writes.
func (s *SkillProposalService) Revert(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	id, digest, operator, targetRoot string,
) (_ SkillInstallResult, err error) {
	if !actor.Operator {
		return SkillInstallResult{}, ErrSkillReviewOperatorOnly
	}
	if operator == "" {
		return SkillInstallResult{}, errors.New("skill review operator identity is required")
	}
	if s.Store == nil {
		return SkillInstallResult{}, errors.New("skill proposal store is required")
	}
	if err := s.Authority.Authorize(ctx, actor, instance, "skill_reviews"); err != nil {
		return SkillInstallResult{}, err
	}
	review, err := s.reviewRow(ctx, instance, id)
	if err != nil {
		return SkillInstallResult{}, err
	}
	switch {
	case review.Digest != digest:
		return SkillInstallResult{}, ErrSkillReviewStale
	case review.State == SkillReviewReverted:
		return SkillInstallResult{}, nil
	case review.State != skillReviewApproved:
		return SkillInstallResult{}, ErrSkillReviewStale
	}
	if reason := revertPrecondition(review, targetRoot); reason != "" {
		return SkillInstallResult{}, &SkillRevertConflictError{Name: review.SkillName, Reason: reason}
	}
	t, err := s.acquireSkill(ctx, targetRoot, review.SkillName, false)
	if err != nil {
		return SkillInstallResult{}, err
	}
	defer func() { err = errors.Join(err, t.close()) }()
	return s.revertLocked(ctx, t, instance, id, operator)
}

func (s *SkillProposalService) revertLocked(
	ctx context.Context,
	t *skillTarget,
	instance Instance,
	id, operator string,
) (SkillInstallResult, error) {
	if t.parent == nil {
		return SkillInstallResult{}, &SkillRevertConflictError{Name: t.name, Reason: SkillRevertTargetUnavailable}
	}
	blocked, warning, err := s.reconcileSkill(ctx, t)
	if err != nil {
		return SkillInstallResult{}, err
	}
	if blocked != nil {
		return SkillInstallResult{}, skillInstallConflictError(t.name, blocked.OpID)
	}
	// Step 2: re-read the review after Reconcile projected this home's entries.
	review, err := s.reviewRow(ctx, instance, id)
	if err != nil {
		return SkillInstallResult{}, err
	}
	switch review.State {
	case SkillReviewReverted:
		return SkillInstallResult{Warning: warning}, nil
	case skillReviewApproved:
	default:
		return SkillInstallResult{}, ErrSkillReviewStale
	}
	eligibility, err := readRevertEligibility(review, t.root, t.name)
	if err != nil {
		return SkillInstallResult{}, err
	}
	if !eligibility.Allowed {
		return SkillInstallResult{}, &SkillRevertConflictError{Name: t.name, Reason: eligibility.Reason}
	}
	op, err := s.revertOp(t, instance, review, operator)
	if err != nil {
		return SkillInstallResult{}, err
	}
	result, err := s.runSkillOp(ctx, t, op)
	if err != nil {
		return SkillInstallResult{}, err
	}
	slog.InfoContext(ctx, "skill.review.reverted", "instance_id", instance.ID, "review_id", review.ID,
		"kind", string(review.Kind), "decided_via", review.DecidedVia, "skill_key", string(review.SkillKey))
	result.Warning = errors.Join(warning, result.Warning)
	return result, nil
}

// revertOp builds the revert transaction: the live tree must still be the review's installed tree.
// A `new` review stages nothing (the skill is removed) and leaves no writer. An `edit` stages its base
// snapshot, verified again, and restores the review's previous writer only when that writer's installed
// digest is the restored base; otherwise the restored contents have no writer (operator-authored).
func (s *SkillProposalService) revertOp(
	t *skillTarget,
	instance Instance,
	review SkillReview,
	operator string,
) (skillOp, error) {
	changed := &SkillRevertConflictError{Name: t.name, Reason: SkillRevertChangedOnDisk}
	current, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return skillOp{}, err
	}
	op := skillOp{
		reviewID: review.ID, instanceID: instance.ID, action: SkillInstallActionRevert,
		decidedVia: skillDecidedViaOperator, decidedBy: operator,
		preExists: true, preDigest: review.InstalledDigest, changed: changed,
		prev: currentSkillWriter(current),
	}
	if review.Kind != SkillProposalEdit {
		return op, nil
	}
	if len(review.PrevInstallMarker) > 0 {
		var previous SkillInstallMarker
		if err := json.Unmarshal(review.PrevInstallMarker, &previous); err != nil {
			return skillOp{}, fmt.Errorf("decode previous install marker of %s: %w", review.ID, err)
		}
		if previous.InstalledDigest == review.BaseDigest {
			op.next = &previous
		}
	}
	base, err := readSkillSnapshot(review)
	if err != nil {
		return skillOp{}, errors.Join(
			&SkillRevertConflictError{Name: t.name, Reason: SkillRevertTargetUnavailable},
			err,
		)
	}
	op.stage = func(dir string) error {
		if err := writeSkillTreeFiles(dir, base); err != nil {
			return err
		}
		staged, err := SkillTreeDigest(dir)
		if err != nil || staged != review.BaseDigest {
			return errors.Join(&SkillRevertConflictError{Name: t.name, Reason: SkillRevertTargetUnavailable}, err)
		}
		return nil
	}
	return op, nil
}
