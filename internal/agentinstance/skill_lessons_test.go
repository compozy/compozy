package agentinstance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type recordingSkillReviewStore struct {
	SkillReviewStore
	created []SkillReview
}

func (s *recordingSkillReviewStore) CreateReview(
	_ context.Context,
	review SkillReview,
	_ ReportInput,
) (SkillReview, bool, error) {
	review.State = SkillReviewPending
	s.created = append(s.created, review)
	return review, false, nil
}

type scopedEvidenceSessions map[string][2]string

func (e scopedEvidenceSessions) SessionScope(_ context.Context, id string) (string, string, error) {
	scope, ok := e[id]
	if !ok {
		return "", "", ErrSkillProposeEvidenceNotFound
	}
	return scope[0], scope[1], nil
}

// Invariant: a proposal carries a validated, redacted lesson and 1-20 evidence sessions owned by the
// instance's profile and workspace; a refused proposal leaves no draft and no review.
// Owning layer: agentinstance propose validation; canonical suite: TestSkillProposalLessons.
func TestSkillProposalLessons(t *testing.T) {
	instance := Instance{ID: "inst-a", ProfileID: "prof-a", WorkspaceID: "ws_a", AgentName: "release-watch"}
	evidence := scopedEvidenceSessions{
		"sess-a": {"prof-a", "ws_a"},
		"sess-b": {"prof-a", "ws_b"},
		"sess-c": {"prof-b", "ws_a"},
	}
	validLesson := SkillLesson{
		Cue:      "Tagging a release",
		Lesson:   "Check that CHANGELOG.md has the new version before tagging.",
		Evidence: "Session sess-a failed at the release job with a missing changelog entry.",
	}
	propose := func(
		t *testing.T,
		mutate func(*SkillProposalInput),
	) (SkillProposalResult, *recordingSkillReviewStore, string, error) {
		t.Helper()
		root := t.TempDir()
		reviews := &recordingSkillReviewStore{}
		service := SkillProposalService{
			Store:     reviews,
			Evidence:  evidence,
			DraftRoot: filepath.Join(root, "drafts"),
			ScanRoots: []string{filepath.Join(root, "skills")},
		}
		input := SkillProposalInput{
			Name:               "release-checklist",
			Description:        "Use before tagging a release.",
			Body:               "1. Confirm CHANGELOG.md has the new version.\n",
			Lesson:             validLesson,
			EvidenceSessionIDs: []string{"sess-a"},
		}
		if mutate != nil {
			mutate(&input)
		}
		result, err := service.Propose(
			t.Context(),
			workspaceaccess.ActorRef{Operator: true},
			instance,
			filepath.Join(root, "skills"),
			input,
		)
		return result, reviews, service.DraftRoot, err
	}
	requireRefused := func(t *testing.T, want error, mutate func(*SkillProposalInput)) error {
		t.Helper()
		_, reviews, draftRoot, err := propose(t, mutate)
		if !errors.Is(err, want) {
			t.Fatalf("Propose() error = %v, want %v", err, want)
		}
		if len(reviews.created) != 0 {
			t.Fatalf("refused proposal created reviews: %+v", reviews.created)
		}
		if entries, readErr := os.ReadDir(draftRoot); !errors.Is(readErr, os.ErrNotExist) && len(entries) != 0 {
			t.Fatalf("refused proposal left drafts: %d %v", len(entries), readErr)
		}
		return err
	}

	t.Run("Should create a pending new review with the lesson and default confidence [UT-001]", func(t *testing.T) {
		t.Parallel()
		result, reviews, _, err := propose(t, nil)
		if err != nil {
			t.Fatal(err)
		}
		review := result.Review
		if result.Duplicate || review.State != SkillReviewPending || review.Kind != SkillProposalNew ||
			review.Lesson.Confidence != SkillLessonConfidenceMed || review.Lesson.Cue != validLesson.Cue ||
			len(
				review.EvidenceSessionIDs,
			) != 1 || review.EvidenceSessionIDs[0] != "sess-a" || len(reviews.created) != 1 {
			t.Fatalf("Propose() = %+v", result)
		}
	})
	t.Run("Should refuse a blank lesson field [UT-002]", func(t *testing.T) {
		t.Parallel()
		requireRefused(t, ErrSkillProposeLessonRequired, func(input *SkillProposalInput) { input.Lesson.Cue = "  " })
	})
	t.Run("Should refuse a proposal without a lesson [UT-003]", func(t *testing.T) {
		t.Parallel()
		requireRefused(
			t,
			ErrSkillProposeLessonRequired,
			func(input *SkillProposalInput) { input.Lesson = SkillLesson{} },
		)
	})
	t.Run("Should refuse a proposal without evidence sessions [UT-004]", func(t *testing.T) {
		t.Parallel()
		requireRefused(t, ErrSkillProposeEvidenceRequired, func(input *SkillProposalInput) {
			input.EvidenceSessionIDs = []string{}
		})
	})
	t.Run("Should refuse a missing evidence session and name it [UT-005]", func(t *testing.T) {
		t.Parallel()
		err := requireRefused(t, ErrSkillProposeEvidenceNotFound, func(input *SkillProposalInput) {
			input.EvidenceSessionIDs = []string{"sess-x"}
		})
		if !strings.Contains(err.Error(), "sess-x") {
			t.Fatalf("error does not name the session: %v", err)
		}
	})
	t.Run("Should refuse evidence from another workspace or profile [UT-006]", func(t *testing.T) {
		t.Parallel()
		for _, id := range []string{"sess-b", "sess-c"} {
			requireRefused(t, ErrSkillProposeEvidenceNotFound, func(input *SkillProposalInput) {
				input.EvidenceSessionIDs = []string{"sess-a", id}
			})
		}
	})
	t.Run("Should collapse duplicate evidence sessions [UT-008]", func(t *testing.T) {
		t.Parallel()
		result, _, _, err := propose(t, func(input *SkillProposalInput) {
			input.EvidenceSessionIDs = []string{"sess-a", "sess-a"}
		})
		if err != nil || len(result.Review.EvidenceSessionIDs) != 1 || result.Review.EvidenceSessionIDs[0] != "sess-a" {
			t.Fatalf("Propose() = %+v, %v", result.Review.EvidenceSessionIDs, err)
		}
	})
	t.Run("Should accept 2,000 characters and refuse 2,001 [UT-009]", func(t *testing.T) {
		t.Parallel()
		if _, _, _, err := propose(t, func(input *SkillProposalInput) {
			input.Lesson.Lesson = strings.Repeat("é", maxSkillLessonFieldChars)
		}); err != nil {
			t.Fatalf("2,000 characters: %v", err)
		}
		requireRefused(t, ErrSkillProposeLessonTooLong, func(input *SkillProposalInput) {
			input.Lesson.Lesson = strings.Repeat("a", maxSkillLessonFieldChars+1)
		})
	})
	t.Run("Should store lesson text redacted like draft contents [UT-010]", func(t *testing.T) {
		t.Parallel()
		raw := "The deploy call failed with Authorization: Bearer abcdefghijklmnopqrstuvwxyz0123456789"
		result, _, _, err := propose(t, func(input *SkillProposalInput) { input.Lesson.Evidence = raw })
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Review.Lesson.Evidence; got == raw || got != redact.String(raw) ||
			!strings.Contains(got, redact.Marker) {
			t.Fatalf("stored evidence = %q", got)
		}
	})
	t.Run("Should refuse a confidence outside low, medium and high without blaming the lesson", func(t *testing.T) {
		t.Parallel()
		_, reviews, _, err := propose(t, func(input *SkillProposalInput) { input.Lesson.Confidence = "certain" })
		if err == nil || errors.Is(err, ErrSkillProposeLessonRequired) || !strings.Contains(err.Error(), "certain") ||
			len(reviews.created) != 0 {
			t.Fatalf("Propose() error = %v, reviews = %d", err, len(reviews.created))
		}
	})
	t.Run("Should refuse a new skill whose name is already installed [UT-012]", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		target := filepath.Join(root, "skills")
		if err := os.MkdirAll(filepath.Join(target, "release-checklist"), 0o700); err != nil {
			t.Fatal(err)
		}
		reviews := &recordingSkillReviewStore{}
		service := SkillProposalService{
			Store:     reviews,
			Evidence:  evidence,
			DraftRoot: filepath.Join(root, "drafts"),
			ScanRoots: []string{target},
		}
		_, err := service.Propose(t.Context(), workspaceaccess.ActorRef{Operator: true}, instance, target,
			SkillProposalInput{
				Name:               "release-checklist",
				Description:        "Use before tagging a release.",
				Body:               "1. Confirm CHANGELOG.md has the new version.\n",
				Lesson:             validLesson,
				EvidenceSessionIDs: []string{"sess-a"},
			})
		if !errors.Is(err, ErrSkillProposeNameTaken) || len(reviews.created) != 0 {
			t.Fatalf("Propose() error = %v, reviews = %d", err, len(reviews.created))
		}
		if _, statErr := os.Stat(service.DraftRoot); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("refused proposal wrote a draft: %v", statErr)
		}
	})
}
