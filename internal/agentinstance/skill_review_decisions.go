package agentinstance

import (
	"context"
	"errors"
	"log/slog"

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
	review, err := s.Store.GetReview(ctx, id)
	if err != nil {
		return SkillReview{}, SkillDraft{}, err
	}
	if review.InstanceID != instance.ID || review.ProfileID != instance.ProfileID ||
		review.WorkspaceID != instance.WorkspaceID {
		return SkillReview{}, SkillDraft{}, ErrSkillReviewNotFound
	}
	draft, err := ReadSkillDraft(review.DraftPath, review.Digest)
	if err != nil {
		return SkillReview{}, SkillDraft{}, err
	}
	return review, draft, nil
}

func (s *SkillProposalService) Reject(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	id, digest, operator string,
) error {
	if !actor.Operator {
		return ErrSkillReviewOperatorOnly
	}
	if operator == "" {
		return errors.New("skill review operator identity is required")
	}
	review, _, err := s.Review(ctx, actor, instance, id)
	if err != nil {
		return err
	}
	if review.State != SkillReviewPending || review.Digest != digest {
		return ErrSkillReviewStale
	}
	if err := s.Store.DecideReview(ctx, id, digest, "rejected", operator); err != nil {
		return err
	}
	slog.InfoContext(ctx, "skill.review.decided", "instance_id", instance.ID, "review_id", id, "decision", "rejected")
	return nil
}
