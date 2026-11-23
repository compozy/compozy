package agentinstance

import (
	"errors"
	"testing"
)

// Invariant: a rejection never makes terminal a review whose install the install log names as
// committed, even when the instance resolves no installation target any more.
// Owning layer: agentinstance review decisions; canonical suite: TestSkillReviewRejection.
func TestSkillReviewRejection(t *testing.T) {
	t.Run("Should check the recorded agent root when the target is gone", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		svc.Evidence = scopedEvidenceSessions{"sess-a": {"prof-a", ""}}
		global := f.instance
		global.WorkspaceID = ""
		propose := func(body string) SkillReview {
			t.Helper()
			result, err := svc.Propose(t.Context(), skillOperator, global, f.root, SkillProposalInput{
				Name: "release-checklist", Description: "Use before tagging a release.", Body: body,
				Lesson:             SkillLesson{Cue: "Tagging", Lesson: "Check the changelog.", Evidence: "sess-a."},
				EvidenceSessionIDs: []string{"sess-a"},
			})
			if err != nil {
				t.Fatalf("Propose() = %v", err)
			}
			return result.Review
		}
		committed := propose("Check the changelog.")
		pending := propose("Check the changelog twice.")
		if recordedAgentSkillRoot(committed.SkillKey) == "" {
			t.Fatalf("skill key %q records no agent root", committed.SkillKey)
		}
		skillStore(svc).projectErr = errors.New("projection unavailable")
		result, err := svc.Approve(t.Context(), skillOperator, global, committed.ID, committed.Digest,
			"operator:prof-a", f.root)
		if err != nil || !result.Committed || result.Warning == nil {
			t.Fatalf("Approve() = %+v, %v; want a commit whose projection failed", result, err)
		}
		requireReviewState(t, svc, committed.ID, SkillReviewPending)
		// The agent now resolves to a definition without a source path: no target is passed.
		if err := svc.Reject(t.Context(), skillOperator, global, committed.ID, committed.Digest,
			"operator:prof-a", ""); !errors.Is(err, ErrSkillReviewStale) {
			t.Fatalf("Reject() of a committed review without a target = %v, want skill_review_stale", err)
		}
		requireReviewState(t, svc, committed.ID, SkillReviewPending)
		if err := svc.Reject(t.Context(), skillOperator, global, pending.ID, pending.Digest,
			"operator:prof-a", ""); err != nil {
			t.Fatalf("Reject() of an uninstalled review without a target = %v", err)
		}
		requireReviewState(t, svc, pending.ID, "rejected")
	})
}
