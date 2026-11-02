package extensionpkg

import (
	"errors"
	"slices"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

const (
	forgeFactNone    = "none"
	forgeFactUnknown = "unknown"
)

func validateForgeStatusFacts(value extensioncontract.ForgeStatusResponse) error {
	if value.RetryAfter != nil && value.Cause == "" {
		return errors.New("retry_after requires cause")
	}
	if !slices.Contains(
		[]string{"", "approved", "changes_requested", "review_required", forgeFactNone, forgeFactUnknown},
		value.ReviewDecision,
	) {
		return errors.New("invalid review decision")
	}
	if !slices.Contains([]string{"", "mergeable", "conflicting", forgeFactUnknown}, value.Mergeable) {
		return errors.New("invalid mergeability")
	}
	if !slices.Contains([]string{"", "complete", "incomplete"}, value.Selection) {
		return errors.New("invalid selection completeness")
	}
	if checks := value.Checks; checks != nil {
		if !slices.Contains([]string{"passing", "failing", "pending", forgeFactNone, forgeFactUnknown}, checks.State) {
			return errors.New("invalid checks state")
		}
		if checks.Passed < 0 || checks.Failed < 0 || checks.Pending < 0 ||
			len(checks.FailedNames) > extensioncontract.MaxForgeFailedNames {
			return errors.New("invalid check counts or failed names")
		}
	}
	if value.ReviewsUnavailable && value.Reviews != nil {
		return errors.New("reviews_unavailable contradicts reported reviews")
	}
	return validateForgeReviews(value.Reviews)
}

func validateForgeReviews(reviews *extensioncontract.ForgeReviews) error {
	if reviews == nil {
		return nil
	}
	if len(reviews.Items) > extensioncontract.MaxForgeReviews ||
		len(reviews.Threads) > extensioncontract.MaxForgeThreads ||
		reviews.UnresolvedThreads < len(reviews.Threads) ||
		(reviews.UnresolvedThreads > len(reviews.Threads) && !reviews.Truncated) {
		return errors.New("invalid review counts")
	}
	for i := range reviews.Threads {
		if reviews.Threads[i].ID == "" {
			return errors.New("invalid review thread id")
		}
	}
	states := []string{
		extensioncontract.ForgeReviewApproved, extensioncontract.ForgeReviewChangesRequested,
		extensioncontract.ForgeReviewCommented, extensioncontract.ForgeReviewDismissed,
	}
	for i := range reviews.Items {
		if reviews.Items[i].ID == "" || !slices.Contains(states, reviews.Items[i].State) {
			return errors.New("invalid review id or state")
		}
	}
	return nil
}
