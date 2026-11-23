package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

var ErrSkillReviewNoTarget = errors.New("skill_review_no_target")

type SkillProposalInput struct {
	Name               string
	Description        string
	Body               string
	Files              []SkillDraftFile
	Lesson             SkillLesson
	EvidenceSessionIDs []string
}

type SkillProposalService struct {
	Store     SkillReviewStore
	Evidence  EvidenceSessions
	Authority Authority
	DraftRoot string
	ScanRoots []string
}

type SkillProposalResult struct {
	Review SkillReview
	// Duplicate is true when a pending review with the same identity was returned unchanged.
	Duplicate bool
}

func (s *SkillProposalService) Propose(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	targetRoot string,
	input SkillProposalInput,
) (SkillProposalResult, error) {
	lesson, err := prepareSkillLesson(input.Lesson)
	if err != nil {
		return SkillProposalResult{}, err
	}
	evidence, err := distinctEvidenceSessions(input.EvidenceSessionIDs)
	if err != nil {
		return SkillProposalResult{}, err
	}
	if s.Store == nil {
		return SkillProposalResult{}, errors.New("skill proposal store is required")
	}
	if err := s.Authority.Authorize(ctx, actor, instance, "skill_propose"); err != nil {
		return SkillProposalResult{}, err
	}
	if targetRoot == "" {
		return SkillProposalResult{}, ErrSkillReviewNoTarget
	}
	draft, err := PrepareSkillDraft(input.Name, input.Description, input.Body, input.Files)
	if err != nil {
		return SkillProposalResult{}, err
	}
	if err := refuseInstalledName(targetRoot, draft.Name); err != nil {
		return SkillProposalResult{}, err
	}
	if err := s.checkEvidenceSessions(ctx, instance, evidence); err != nil {
		return SkillProposalResult{}, err
	}
	id, err := store.NewID("skr")
	if err != nil {
		return SkillProposalResult{}, err
	}
	payload, err := json.Marshal(struct {
		SkillReviewID      string              `json:"skill_review_id"`
		EvidenceSessionIDs []string            `json:"evidence_session_ids,omitempty"`
		Proposals          []map[string]string `json:"proposals"`
	}{SkillReviewID: id, EvidenceSessionIDs: evidence, Proposals: []map[string]string{{
		"title": "Review proposed skill: " + draft.Name,
		"body":  "Inspect the exact contents before approving or rejecting this skill.",
	}}})
	if err != nil {
		return SkillProposalResult{}, err
	}
	report := ReportInput{SourceKey: "skill_review:" + id, Kind: ReportKindProposal, Payload: payload}
	if err := report.Validate(); err != nil {
		return SkillProposalResult{}, err
	}
	path, digest, err := WriteSkillDraft(s.DraftRoot, s.ScanRoots, draft)
	if err != nil {
		return SkillProposalResult{}, err
	}
	review, duplicate, err := s.Store.CreateReview(ctx, SkillReview{
		ID:                 id,
		InstanceID:         instance.ID,
		ProfileID:          instance.ProfileID,
		WorkspaceID:        instance.WorkspaceID,
		AgentName:          instance.AgentName,
		SkillName:          input.Name,
		Digest:             digest,
		DraftPath:          path,
		SourceSessionID:    actor.SessionID,
		Kind:               SkillProposalNew,
		Lesson:             lesson,
		EvidenceSessionIDs: evidence,
	}, report)
	result := SkillProposalResult{Review: review, Duplicate: duplicate}
	if errors.Is(err, ErrSkillReviewLimit) || errors.Is(err, ErrSkillProposePreviouslyRejected) ||
		(err == nil && review.DraftPath != path) {
		return result, errors.Join(err, os.Remove(path))
	}
	if err == nil {
		slog.InfoContext(ctx, "skill.review.created", "instance_id", instance.ID, "review_id", review.ID,
			"kind", string(review.Kind), "decided_via", review.DecidedVia)
	}
	// An uncertain database commit must retain the inert draft for recovery.
	return result, err
}

// refuseInstalledName applies Business Rule 4 for a new skill: a name already present in the target
// root is refused, so re-proposing installed contents never opens another review.
func refuseInstalledName(targetRoot, name string) error {
	target, err := fileutil.ResolvePathWithinRoot(targetRoot, filepath.Join(targetRoot, name))
	if err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("%w: a skill named %s is already installed for this instance", ErrSkillProposeNameTaken, name)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
