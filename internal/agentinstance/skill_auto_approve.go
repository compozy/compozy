package agentinstance

import (
	"context"
	"log/slog"
	"slices"
	"time"
)

// SkillAutoApproveDailyCap bounds automatic approvals per instance per UTC day (Business Rule 14).
// A constant, not configuration: compozy__config_set is reachable by agents (ADR-001).
const SkillAutoApproveDailyCap = 3

const (
	skillDecidedViaAutoApprove = "auto_approve"
	skillDecidedByAutoApprove  = "auto-approve"
	skillConfidenceHigh        = "high"
)

// Auto-approve skip reasons (Business Rule 9), in evaluation order; the first failing one is reported.
const (
	SkillAutoApproveToggledOff        = "toggled_off"
	SkillAutoApproveLowConfidence     = "low_confidence"
	SkillAutoApproveRunsAutomatically = "runs_automatically"
	SkillAutoApproveOperatorAuthored  = "operator_authored_skill"
	SkillAutoApproveBaseStale         = "base_stale"
	SkillAutoApproveDailyCapReached   = "daily_cap"
	SkillAutoApproveInstallFailed     = "install_failed"
)

// SkillAutoApproveReader reads an instance's current skill auto-approve toggle from the store.
type SkillAutoApproveReader func(ctx context.Context, instanceID string) (bool, error)

// lockInstance takes the per-instance proposal mutex. Every Propose holds it before the skill locks,
// so the toggle and the daily count are read by one proposal of an instance at a time.
func (s *SkillProposalService) lockInstance(ctx context.Context, instanceID string) (func(), error) {
	release, err := s.installLocks().lock(ctx, "instance:"+instanceID)
	if err == nil {
		s.step("instance_locked", nil)
	}
	return release, err
}

// autoApprove evaluates Business Rule 9 for a freshly created review under the instance mutex and
// the skill locks and, when eligible, installs it through the install transaction. It returns the
// resulting review and the skip reason ("" only when approved automatically). Install failures keep
// the review pending; they are never returned to the agent.
func (s *SkillProposalService) autoApprove(
	ctx context.Context,
	t *skillTarget,
	instance Instance,
	review SkillReview,
	draft SkillDraft,
	edit skillEdit,
) (SkillReview, string) {
	s.step("auto_approve_eval", t)
	reason, current := s.autoApproveSkipReason(ctx, t, instance, review, draft, edit)
	if reason != "" {
		return review, reason
	}
	s.step("auto_approve_eligible", t)
	if err := s.installAutomatically(ctx, t, instance, review, draft, current); err != nil {
		s.logAutoApproveSkipped(ctx, review, SkillAutoApproveInstallFailed, "error", err)
		return review, SkillAutoApproveInstallFailed
	}
	slog.InfoContext(ctx, "skill.review.decided", "instance_id", instance.ID, "review_id", review.ID,
		"decision", "approved", "kind", string(review.Kind), "decided_via", skillDecidedViaAutoApprove,
		"skill_key", string(review.SkillKey))
	if projected, ok := s.projectedAutoApproval(ctx, t, review.ID); ok {
		return projected, ""
	}
	// Committed but unprojected: the cap fails closed until Reconcile projects it.
	s.installLocks().markUnprojected(instance.ID, review.ID)
	review.State, review.DecidedVia, review.DecidedBy = skillReviewApproved, skillDecidedViaAutoApprove,
		skillDecidedByAutoApprove
	return review, ""
}

// projectedAutoApproval reads the projected review of a committed auto-approval, retrying the
// projection once from the install log when the post-commit projection did not land.
func (s *SkillProposalService) projectedAutoApproval(
	ctx context.Context,
	t *skillTarget,
	reviewID string,
) (SkillReview, bool) {
	if projected, err := s.Store.GetReview(ctx, reviewID); err == nil && projected.State == skillReviewApproved {
		return projected, true
	}
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return SkillReview{}, false
	}
	for _, entry := range slices.Backward(entries) {
		if entry.ReviewID == reviewID && entry.Action == SkillInstallActionInstall {
			if err := s.projectSkillInstall(ctx, entry); err != nil {
				slog.WarnContext(ctx, "skill.review.auto_approve_projection_retry_failed", "review_id", reviewID,
					"error", err)
				return SkillReview{}, false
			}
			break
		}
	}
	projected, err := s.Store.GetReview(ctx, reviewID)
	return projected, err == nil && projected.State == skillReviewApproved
}

// hasUnprojectedAutoApproval reports whether the instance still has a committed auto-approval whose
// review row is pending; a row that has been projected since is forgotten. A read error counts.
func (s *SkillProposalService) hasUnprojectedAutoApproval(ctx context.Context, instanceID string) bool {
	locks := s.installLocks()
	for _, id := range locks.unprojectedReviews(instanceID) {
		review, err := s.Store.GetReview(ctx, id)
		if err != nil || review.State == SkillReviewPending {
			return true
		}
		locks.clearUnprojected(instanceID, id)
	}
	return false
}

// autoApproveSkipReason returns the first failing Business Rule 9 check and the install log's current
// writer of the skill. Every read error fails closed as the reason of the check that could not run.
func (s *SkillProposalService) autoApproveSkipReason(
	ctx context.Context,
	t *skillTarget,
	instance Instance,
	review SkillReview,
	draft SkillDraft,
	edit skillEdit,
) (string, *SkillInstallMarker) {
	on := false
	if s.AutoApprove != nil {
		var err error
		if on, err = s.AutoApprove(ctx, instance.ID); err != nil {
			slog.WarnContext(ctx, "skill.review.auto_approve_toggle_unreadable", "instance_id", instance.ID,
				"error", err)
			on = false
		}
	}
	if !on {
		return SkillAutoApproveToggledOff, nil // the default state: not logged
	}
	if review.Lesson.Confidence != skillConfidenceHigh {
		return s.skipAutoApprove(ctx, review, SkillAutoApproveLowConfidence), nil
	}
	tree := edit.result
	if review.Kind != SkillProposalEdit {
		tree = draftSkillTree(draft)
	}
	if counts, err := tree.AutomaticContents(); err != nil || counts.Hooks+counts.MCPServers+counts.Scripts > 0 {
		return s.skipAutoApprove(ctx, review, SkillAutoApproveRunsAutomatically), nil
	}
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		// Rule 9.4 is about edits; a new skill only needs the log to install.
		if review.Kind != SkillProposalEdit {
			return s.skipAutoApprove(ctx, review, SkillAutoApproveInstallFailed), nil
		}
		return s.skipAutoApprove(ctx, review, SkillAutoApproveOperatorAuthored), nil
	}
	current := currentSkillWriter(entries)
	if review.Kind == SkillProposalEdit {
		if reason := s.editProvenance(ctx, t, instance, review, current); reason != "" {
			return s.skipAutoApprove(ctx, review, reason), nil
		}
	}
	if s.hasUnprojectedAutoApproval(ctx, instance.ID) {
		s.logAutoApproveSkipped(ctx, review, SkillAutoApproveDailyCapReached, "unprojected", true)
		return SkillAutoApproveDailyCapReached, nil
	}
	day := s.clock().UTC().Truncate(24 * time.Hour)
	count, err := s.Store.CountAutoApprovals(ctx, instance.ID, day)
	if err != nil || count >= SkillAutoApproveDailyCap {
		s.logAutoApproveSkipped(ctx, review, SkillAutoApproveDailyCapReached,
			"count", count, "cap", SkillAutoApproveDailyCap, "error", err)
		return SkillAutoApproveDailyCapReached, nil
	}
	return "", current
}

// editProvenance applies checks 4 and 5 to an edit: the install log's current writer must be an
// approved review of this instance in this home's store whose installed digest is still the live tree
// (external edits are never logged, so a writer whose contents were changed on disk no longer
// vouches for them), and the live tree must still equal the base. Only the install log (the sole
// authority, ADR-002), the home's own review rows and the live tree are trusted.
func (s *SkillProposalService) editProvenance(
	ctx context.Context,
	t *skillTarget,
	instance Instance,
	review SkillReview,
	current *SkillInstallMarker,
) string {
	if current == nil || current.ReviewID == "" || current.InstanceID != instance.ID {
		return SkillAutoApproveOperatorAuthored
	}
	writer, err := s.Store.GetReview(ctx, current.ReviewID)
	if err != nil || writer.InstanceID != instance.ID || writer.State != skillReviewApproved {
		return SkillAutoApproveOperatorAuthored
	}
	live, err := skillTreeState(t.dir)
	if err != nil || live != current.InstalledDigest {
		return SkillAutoApproveOperatorAuthored
	}
	if live != review.BaseDigest {
		return SkillAutoApproveBaseStale
	}
	return ""
}

func (s *SkillProposalService) skipAutoApprove(ctx context.Context, review SkillReview, reason string) string {
	s.logAutoApproveSkipped(ctx, review, reason)
	return reason
}

func (s *SkillProposalService) logAutoApproveSkipped(
	ctx context.Context,
	review SkillReview,
	reason string,
	args ...any,
) {
	attrs := append([]any{"instance_id", review.InstanceID, "review_id", review.ID, "kind", string(review.Kind),
		"skill_key", string(review.SkillKey), reasonKey, reason}, args...)
	slog.InfoContext(ctx, "skill.review.auto_approve_skipped", attrs...)
}

// installAutomatically runs the install transaction for an eligible review, recording the decision
// as auto_approve by auto-approve.
func (s *SkillProposalService) installAutomatically(
	ctx context.Context,
	t *skillTarget,
	instance Instance,
	review SkillReview,
	draft SkillDraft,
	current *SkillInstallMarker,
) error {
	if err := t.ensureParent(); err != nil {
		return err
	}
	live, err := skillTreeState(t.dir)
	if err != nil {
		return err
	}
	op := skillOp{
		reviewID: review.ID, instanceID: instance.ID, action: SkillInstallActionInstall,
		decidedVia: skillDecidedViaAutoApprove, decidedBy: skillDecidedByAutoApprove,
		prev: current,
		next: &SkillInstallMarker{
			ReviewID: review.ID, InstanceID: instance.ID, HomeID: s.HomeID,
			DecidedVia: skillDecidedViaAutoApprove, DecidedBy: skillDecidedByAutoApprove,
		},
	}
	if err := prepareSkillInstallOp(&op, t, review, draft, live); err != nil {
		return err
	}
	result, err := s.runSkillOp(ctx, t, op)
	if err != nil {
		return err
	}
	if result.Warning != nil {
		slog.WarnContext(ctx, "skill.review.auto_approve_committed_with_warning", "review_id", review.ID,
			"error", result.Warning)
	}
	return nil
}

func (s *SkillProposalService) clock() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// step calls the test seam observing the proposal's lock and evaluation order.
func (s *SkillProposalService) step(name string, t *skillTarget) {
	if s.onStep != nil {
		s.onStep(name, t)
	}
}
