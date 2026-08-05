package task

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunReviewValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize review defaults and bounded JSON fields", func(t *testing.T) {
		t.Parallel()

		review, err := (&RunReview{
			ReviewID: " review-1 ",
			TaskID:   " task-1 ",
			RunID:    " run-1 ",
			Policy:   ReviewPolicyAlways,
			Status:   RunReviewStatusRequested,
			Reason:   "review compozy_claim_reason-secret",
			MissingWork: json.RawMessage(
				`["remove compozy_claim_missing-work-secret"]`,
			),
		}).Normalize(time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}

		if got, want := review.ReviewID, "review-1"; got != want {
			t.Fatalf("ReviewID = %q, want %q", got, want)
		}
		if got, want := review.ReviewRound, defaultRunReviewRound; got != want {
			t.Fatalf("ReviewRound = %d, want %d", got, want)
		}
		if strings.Contains(review.Reason, "compozy_claim_reason-secret") {
			t.Fatalf("Reason = %q, leaked raw claim token", review.Reason)
		}
		if strings.Contains(string(review.MissingWork), "compozy_claim_missing-work-secret") {
			t.Fatalf("MissingWork = %q, leaked raw claim token", review.MissingWork)
		}
	})

	t.Run("Should reject recorded reviews without outcomes", func(t *testing.T) {
		t.Parallel()

		_, err := (&RunReview{
			ReviewID:    "review-1",
			TaskID:      "task-1",
			RunID:       "run-1",
			Policy:      ReviewPolicyAlways,
			ReviewRound: 1,
			Attempt:     1,
			Status:      RunReviewStatusRecorded,
			MissingWork: json.RawMessage(`[]`),
			RequestedAt: time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}).Normalize(time.Now().UTC())
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Normalize(recorded without outcome) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject oversized guidance", func(t *testing.T) {
		t.Parallel()

		_, err := (&RunReview{
			ReviewID:          "review-1",
			TaskID:            "task-1",
			RunID:             "run-1",
			Policy:            ReviewPolicyAlways,
			ReviewRound:       1,
			Attempt:           1,
			Status:            RunReviewStatusRequested,
			MissingWork:       json.RawMessage(`[]`),
			NextRoundGuidance: strings.Repeat("x", maxRunReviewGuidanceBytes+1),
			RequestedAt:       time.Now().UTC(),
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}).Normalize(time.Now().UTC())
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Normalize(oversized guidance) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject review references above the domain limit", func(t *testing.T) {
		t.Parallel()

		request := RunReviewRequest{
			TaskID: strings.Repeat("t", MaxReferenceBytes+1),
			RunID:  "run-1",
			Reason: "operator supplied compozy_claim_request-secret",
		}.Normalize()
		if strings.Contains(request.Reason, "compozy_claim_request-secret") {
			t.Fatalf("RunReviewRequest.Reason = %q, leaked raw claim token", request.Reason)
		}
		if err := request.Validate("run_review_request"); !errors.Is(err, ErrValidation) {
			t.Fatalf("RunReviewRequest.Validate() error = %v, want %v", err, ErrValidation)
		}

		confidence := 0.8
		verdict := RunReviewVerdict{
			Outcome:     RunReviewOutcomeApproved,
			Confidence:  &confidence,
			Reason:      "approved",
			DeliveryID:  strings.Repeat("d", MaxReferenceBytes+1),
			MissingWork: json.RawMessage(`[]`),
		}.Normalize()
		if err := verdict.Validate("run_review_verdict"); !errors.Is(err, ErrValidation) {
			t.Fatalf("RunReviewVerdict.Validate() error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should match policies to terminal run status", func(t *testing.T) {
		t.Parallel()

		if !ReviewPolicyOnSuccess.MatchesRunStatus(TaskRunStatusCompleted) {
			t.Fatal("ReviewPolicyOnSuccess did not match completed run")
		}
		if ReviewPolicyOnSuccess.MatchesRunStatus(TaskRunStatusFailed) {
			t.Fatal("ReviewPolicyOnSuccess matched failed run")
		}
		if !ReviewPolicyOnFailure.MatchesRunStatus(TaskRunStatusCanceled) {
			t.Fatal("ReviewPolicyOnFailure did not match canceled run")
		}
		if ReviewPolicyAlways.MatchesRunStatus(TaskRunStatusRunning) {
			t.Fatal("ReviewPolicyAlways matched non-terminal running run")
		}
	})

	t.Run("Should validate verdict payload by outcome", func(t *testing.T) {
		t.Parallel()

		confidence := 0.8
		const rawToken = "compozy_claim_review-secret"
		approved := RunReviewVerdict{
			Outcome:     RunReviewOutcomeApproved,
			Confidence:  &confidence,
			Reason:      "result is correct",
			DeliveryID:  "delivery-1",
			MissingWork: json.RawMessage(`[]`),
		}.Normalize()
		if err := approved.Validate("verdict"); err != nil {
			t.Fatalf("Validate(approved) error = %v", err)
		}

		rejected := RunReviewVerdict{
			Outcome:           RunReviewOutcomeRejected,
			Confidence:        &confidence,
			Reason:            "work remains " + rawToken,
			DeliveryID:        "delivery-2",
			MissingWork:       json.RawMessage(`["add ` + rawToken + ` regression tests"]`),
			NextRoundGuidance: "remove " + rawToken,
			ReviewText:        "review " + rawToken,
		}.Normalize()
		if err := rejected.Validate("verdict"); err != nil {
			t.Fatalf("Validate(rejected) error = %v", err)
		}
		for field, value := range map[string]string{
			"reason":              rejected.Reason,
			"missing_work":        string(rejected.MissingWork),
			"next_round_guidance": rejected.NextRoundGuidance,
			"review_text":         rejected.ReviewText,
		} {
			if strings.Contains(value, rawToken) {
				t.Fatalf("rejected %s = %q, leaked raw claim token", field, value)
			}
		}

		rejected.MissingWork = json.RawMessage(`[]`)
		rejected.NextRoundGuidance = ""
		if err := rejected.Validate("verdict"); !errors.Is(err, ErrValidation) {
			t.Fatalf("Validate(rejected without work) error = %v, want %v", err, ErrValidation)
		}

		approved.MissingWork = json.RawMessage(`["not empty"]`)
		if err := approved.Validate("verdict"); !errors.Is(err, ErrValidation) {
			t.Fatalf("Validate(approved with work) error = %v, want %v", err, ErrValidation)
		}
	})
}
