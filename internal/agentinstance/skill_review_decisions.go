package agentinstance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (s *SkillProposalService) Reviews(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	query SkillReviewQuery,
) ([]SkillReview, error) {
	if s.Store == nil {
		return nil, errors.New("skill proposal store is required")
	}
	if err := s.Authority.Authorize(ctx, actor, instance, "skill_reviews"); err != nil {
		return nil, err
	}
	query.ProfileID = instance.ProfileID
	query.WorkspaceID = instance.WorkspaceID
	query.InstanceID = instance.ID
	return s.Store.ListReviews(ctx, query)
}

func (s *SkillProposalService) Review(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	id string,
) (SkillReview, SkillDraft, error) {
	if s.Store == nil {
		return SkillReview{}, SkillDraft{}, errors.New("skill proposal store is required")
	}
	if err := s.Authority.Authorize(ctx, actor, instance, "skill_reviews"); err != nil {
		return SkillReview{}, SkillDraft{}, err
	}
	review, err := s.reviewRow(ctx, instance, id)
	if err != nil {
		return SkillReview{}, SkillDraft{}, err
	}
	draft, err := ReadSkillDraft(review.DraftPath, review.Digest)
	if err != nil {
		return SkillReview{}, SkillDraft{}, err
	}
	return review, draft, nil
}

// reviewRow reads one review bound to the instance, without its draft.
func (s *SkillProposalService) reviewRow(ctx context.Context, instance Instance, id string) (SkillReview, error) {
	review, err := s.Store.GetReview(ctx, id)
	if err != nil {
		return SkillReview{}, err
	}
	if review.InstanceID != instance.ID || review.ProfileID != instance.ProfileID ||
		review.WorkspaceID != instance.WorkspaceID {
		return SkillReview{}, ErrSkillReviewNotFound
	}
	return review, nil
}

// recordedAgentSkillRoot is the agent-local skills root an "agent-dir:" review was proposed against;
// "" for workspace keys and reviews created before skill keys.
func recordedAgentSkillRoot(key SkillKey) string {
	rest, ok := strings.CutPrefix(string(key), "agent-dir:")
	if !ok || !strings.Contains(rest, "/") {
		return ""
	}
	return filepath.Join(filepath.FromSlash(path.Dir(rest)), compozyconfig.SkillsDirName)
}

// Reject records an operator rejection under both skill locks, after Reconcile, so it can never
// race an approval of the same physical skill. targetRoot may be empty when the instance has no
// installation target now; an agent-local root recorded in the review's skill key is still checked.
func (s *SkillProposalService) Reject(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	id, digest, operator, targetRoot string,
) (err error) {
	if !actor.Operator {
		return ErrSkillReviewOperatorOnly
	}
	if operator == "" {
		return errors.New("skill review operator identity is required")
	}
	if s.Store == nil {
		return errors.New("skill proposal store is required")
	}
	if err := s.Authority.Authorize(ctx, actor, instance, "skill_reviews"); err != nil {
		return err
	}
	// Rejection never needs the draft, so a draft that cannot be installed can still be rejected.
	review, err := s.reviewRow(ctx, instance, id)
	if err != nil {
		return err
	}
	if review.State != SkillReviewPending || review.Digest != digest {
		return ErrSkillReviewStale
	}
	if targetRoot == "" {
		// The instance resolves no target now, but its agent may have had one when the review was
		// proposed (a deleted agent file falls back to a builtin of the same name): check that root.
		targetRoot = recordedAgentSkillRoot(review.SkillKey)
	}
	if targetRoot != "" && proposedSkillName.MatchString(review.SkillName) {
		target, lockErr := s.acquireSkill(ctx, targetRoot, review.SkillName, false)
		if lockErr != nil {
			return lockErr
		}
		defer func() { err = errors.Join(err, target.close()) }()
		if err := s.checkRejectable(ctx, target, id); err != nil {
			return err
		}
		if review, err = s.reviewRow(ctx, instance, id); err != nil {
			return err
		}
		if review.State != SkillReviewPending || review.Digest != digest {
			return ErrSkillReviewStale
		}
	}
	if err := s.Store.DecideReject(ctx, id, digest, operator); err != nil {
		return err
	}
	slog.InfoContext(ctx, "skill.review.decided", "instance_id", instance.ID, "review_id", id,
		"decision", "rejected", "kind", string(review.Kind), "decided_via", skillDecidedViaOperator,
		"skill_key", string(review.SkillKey))
	return nil
}

// checkRejectable reconciles the skill and refuses a review that the install log names as committed,
// even while its projection is still failing, or whose install is a retained conflict: resolving it
// with --keep current would install the review's tree while the review stayed rejected.
func (s *SkillProposalService) checkRejectable(ctx context.Context, t *skillTarget, id string) error {
	blocked, warning, err := s.reconcileSkill(ctx, t)
	if err != nil {
		return err
	}
	if blocked != nil && blocked.ReviewID == id {
		return errors.Join(fmt.Errorf(
			"%w: the install of this review is an unresolved conflict; run `compozy agent skill install-conflict "+
				"resolve %s --keep current|previous` first", ErrSkillReviewStale, blocked.OpID), warning)
	}
	if t.parent == nil {
		return nil
	}
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Current != nil && entry.Current.ReviewID == id {
			return errors.Join(ErrSkillReviewStale, warning)
		}
	}
	return nil
}
