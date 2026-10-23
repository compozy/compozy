package extensionpkg

import (
	"errors"
	"slices"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

func validateForgeStatusFacts(value extensioncontract.ForgeStatusResponse) error {
	if value.RetryAfter != nil && value.Cause == "" {
		return errors.New("retry_after requires cause")
	}
	if !slices.Contains([]string{"", "approved", "changes_requested", "review_required", "none", "unknown"}, value.ReviewDecision) {
		return errors.New("invalid review decision")
	}
	if !slices.Contains([]string{"", "mergeable", "conflicting", "unknown"}, value.Mergeable) {
		return errors.New("invalid mergeability")
	}
	if !slices.Contains([]string{"", "complete", "incomplete"}, value.Selection) {
		return errors.New("invalid selection completeness")
	}
	if checks := value.Checks; checks != nil {
		if !slices.Contains([]string{"passing", "failing", "pending", "none", "unknown"}, checks.State) {
			return errors.New("invalid checks state")
		}
		if checks.Passed < 0 || checks.Failed < 0 || checks.Pending < 0 || len(checks.FailedNames) > 5 {
			return errors.New("invalid check counts or failed names")
		}
	}
	return nil
}
