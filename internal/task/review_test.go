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
		if got, want := string(review.MissingWork), "[]"; got != want {
			t.Fatalf("MissingWork = %q, want %q", got, want)
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
			Outcome:     RunReviewOutcomeRejected,
			Confidence:  &confidence,
			Reason:      "work remains",
			DeliveryID:  "delivery-2",
			MissingWork: json.RawMessage(`["add regression tests"]`),
		}.Normalize()
		if err := rejected.Validate("verdict"); err != nil {
			t.Fatalf("Validate(rejected) error = %v", err)
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
