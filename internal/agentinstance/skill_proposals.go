package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

var ErrSkillReviewNoTarget = errors.New("skill_review_no_target")

type SkillProposalInput struct {
	Name               string
	Description        string
	Body               string
	Files              []SkillDraftFile
	EvidenceSessionIDs []string
}

type SkillProposalService struct {
	Store     SkillReviewStore
	Authority Authority
	DraftRoot string
	ScanRoots []string
}

func (s *SkillProposalService) Propose(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	targetRoot string,
	input SkillProposalInput,
) (SkillReview, error) {
	if s.Store == nil {
		return SkillReview{}, errors.New("skill proposal store is required")
	}
	if err := s.Authority.Authorize(ctx, actor, instance, "skill_propose"); err != nil {
		return SkillReview{}, err
	}
	if targetRoot == "" {
		return SkillReview{}, ErrSkillReviewNoTarget
	}
	draft, err := PrepareSkillDraft(input.Name, input.Description, input.Body, input.Files)
	if err != nil {
		return SkillReview{}, err
	}
	id, err := store.NewID("skr")
	if err != nil {
		return SkillReview{}, err
	}
	payload, err := json.Marshal(struct {
		SkillReviewID      string              `json:"skill_review_id"`
		EvidenceSessionIDs []string            `json:"evidence_session_ids,omitempty"`
		Proposals          []map[string]string `json:"proposals"`
	}{SkillReviewID: id, EvidenceSessionIDs: input.EvidenceSessionIDs, Proposals: []map[string]string{{
		"title": "Review proposed skill: " + draft.Name,
		"body":  "Inspect the exact contents before approving or rejecting this skill.",
	}}})
	if err != nil {
		return SkillReview{}, err
	}
	report := ReportInput{SourceKey: "skill_review:" + id, Kind: ReportKindProposal, Payload: payload}
	if err := report.Validate(); err != nil {
		return SkillReview{}, err
	}
	path, digest, err := WriteSkillDraft(s.DraftRoot, s.ScanRoots, draft)
	if err != nil {
		return SkillReview{}, err
	}
	review, err := s.Store.CreateReview(ctx, SkillReview{
		ID:              id,
		InstanceID:      instance.ID,
		ProfileID:       instance.ProfileID,
		WorkspaceID:     instance.WorkspaceID,
		AgentName:       instance.AgentName,
		SkillName:       input.Name,
		Digest:          digest,
		DraftPath:       path,
		SourceSessionID: actor.SessionID,
	}, report)
	if errors.Is(err, ErrSkillReviewLimit) || (err == nil && review.DraftPath != path) {
		return review, errors.Join(err, os.Remove(path))
	}
	// An uncertain database commit must retain the inert draft for recovery.
	return review, err
}
