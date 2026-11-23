package agentinstance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/redact"
)

const (
	maxSkillLessonFieldChars  = 2000
	maxSkillEvidenceSessions  = 20
	SkillLessonConfidenceLow  = "low"
	SkillLessonConfidenceMed  = "medium"
	SkillLessonConfidenceHigh = "high"
)

var (
	ErrSkillProposeLessonRequired     = errors.New("skill_propose_lesson_required")
	ErrSkillProposeLessonTooLong      = errors.New("skill_propose_lesson_too_long")
	ErrSkillProposeEvidenceRequired   = errors.New("skill_propose_evidence_required")
	ErrSkillProposeEvidenceNotFound   = errors.New("skill_propose_evidence_not_found")
	ErrSkillProposePreviouslyRejected = errors.New("skill_propose_previously_rejected")
	ErrSkillProposeNameTaken          = errors.New("skill_propose_name_taken")
)

// SkillProposalMode is the review kind: a new skill or a change to an installed one.
type SkillProposalMode string

const (
	SkillProposalNew  SkillProposalMode = "new"
	SkillProposalEdit SkillProposalMode = "edit"
)

// SkillLesson is the structured claim a proposal carries. A zero lesson means a review created before
// lessons existed.
type SkillLesson struct {
	Cue        string `json:"cue"`
	Lesson     string `json:"lesson"`
	Evidence   string `json:"evidence"`
	Confidence string `json:"confidence"`
}

// Recorded reports whether the review carries a lesson.
func (l SkillLesson) Recorded() bool { return l.Cue != "" }

// EvidenceSessions resolves a session's ownership for evidence validation. A missing session returns
// ErrSkillProposeEvidenceNotFound.
type EvidenceSessions interface {
	SessionScope(ctx context.Context, sessionID string) (profileID, workspaceID string, err error)
}

// prepareSkillLesson validates the lesson fields and returns them trimmed and redacted like draft
// contents.
func prepareSkillLesson(lesson SkillLesson) (SkillLesson, error) {
	fields := []*string{&lesson.Cue, &lesson.Lesson, &lesson.Evidence}
	for _, field := range fields {
		*field = strings.TrimSpace(*field)
		if *field == "" {
			return SkillLesson{}, ErrSkillProposeLessonRequired
		}
		if utf8.RuneCountInString(*field) > maxSkillLessonFieldChars {
			return SkillLesson{}, fmt.Errorf("%w: each lesson field is limited to %d characters",
				ErrSkillProposeLessonTooLong, maxSkillLessonFieldChars)
		}
	}
	switch lesson.Confidence {
	case "":
		lesson.Confidence = SkillLessonConfidenceMed
	case SkillLessonConfidenceLow, SkillLessonConfidenceMed, SkillLessonConfidenceHigh:
	default:
		// The tool schema enumerates confidence; _dx.md defines no code for this, so it stays a plain
		// validation error like the draft's own schema checks.
		return SkillLesson{}, fmt.Errorf("lesson confidence %q must be low, medium or high", lesson.Confidence)
	}
	for _, field := range fields {
		*field = redact.String(*field)
	}
	return lesson, nil
}

// distinctEvidenceSessions collapses duplicates and enforces 1–20 distinct session IDs.
func distinctEvidenceSessions(ids []string) ([]string, error) {
	result := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, ErrSkillProposeEvidenceRequired
	}
	if len(result) > maxSkillEvidenceSessions {
		return nil, fmt.Errorf("%w: at most %d evidence sessions are allowed",
			ErrSkillProposeEvidenceRequired, maxSkillEvidenceSessions)
	}
	return result, nil
}

// checkEvidenceSessions requires every session to exist in the instance's profile and, for a workspace
// instance, in its workspace. The first offending ID is named.
func (s *SkillProposalService) checkEvidenceSessions(ctx context.Context, instance Instance, ids []string) error {
	if s.Evidence == nil {
		return errors.New("skill proposal evidence lookup is required")
	}
	for _, id := range ids {
		profileID, workspaceID, err := s.Evidence.SessionScope(ctx, id)
		if err != nil && !errors.Is(err, ErrSkillProposeEvidenceNotFound) {
			return err
		}
		if err != nil || profileID != instance.ProfileID ||
			(instance.WorkspaceID != "" && workspaceID != instance.WorkspaceID) {
			return fmt.Errorf("%w: session %s is not in this instance's profile or workspace",
				ErrSkillProposeEvidenceNotFound, id)
		}
	}
	return nil
}
