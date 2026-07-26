package devcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	codeRabbitCommitStatusContext = "CodeRabbit"

	codeRabbitWatchCurrentReviewed   = "current_reviewed"
	codeRabbitWatchCurrentSettled    = "current_settled"
	codeRabbitWatchLocalHeadMismatch = "local_head_mismatch"
	codeRabbitWatchPending           = "pending"
	codeRabbitWatchStale             = "stale"
)

func (p *codeRabbitProvider) fetchPullRequestReviews(
	ctx context.Context,
	owner string,
	repo string,
	pr string,
) ([]pullRequestReview, error) {
	reviews := make([]pullRequestReview, 0, 16)
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("repos/%s/%s/pulls/%s/reviews?per_page=100&page=%d", owner, repo, pr, page)
		output, err := p.runner.Run(ctx, "gh", []string{ghAPIArg, endpoint}, "")
		if err != nil {
			return nil, fmt.Errorf("dev-cycle: fetch pull request reviews page %d: %w", page, err)
		}
		var pageReviews []pullRequestReview
		if err := json.Unmarshal(output, &pageReviews); err != nil {
			return nil, fmt.Errorf("dev-cycle: decode pull request reviews page %d: %w", page, err)
		}
		reviews = append(reviews, pageReviews...)
		if len(pageReviews) < 100 {
			break
		}
	}
	return reviews, nil
}

func (p *codeRabbitProvider) fetchCommitStatuses(
	ctx context.Context,
	owner string,
	repo string,
	sha string,
) ([]commitStatus, error) {
	statuses := make([]commitStatus, 0, 8)
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("repos/%s/%s/commits/%s/statuses?per_page=100&page=%d", owner, repo, sha, page)
		output, err := p.runner.Run(ctx, "gh", []string{ghAPIArg, endpoint}, "")
		if err != nil {
			return nil, fmt.Errorf("dev-cycle: fetch commit statuses page %d: %w", page, err)
		}
		var pageStatuses []commitStatus
		if err := json.Unmarshal(output, &pageStatuses); err != nil {
			return nil, fmt.Errorf("dev-cycle: decode commit statuses page %d: %w", page, err)
		}
		statuses = append(statuses, pageStatuses...)
		if len(pageStatuses) < 100 {
			break
		}
	}
	return statuses, nil
}

func buildWatchPayload(
	pr string,
	prHeadSHA string,
	localHeadSHA string,
	reviews []pullRequestReview,
	latestStatus commitStatus,
	hasStatus bool,
) (codeRabbitWatchPayload, error) {
	payload := codeRabbitWatchPayload{
		PR: pr,
		Review: codeRabbitReview{
			HeadSHA:      strings.TrimSpace(prHeadSHA),
			LocalHeadSHA: strings.TrimSpace(localHeadSHA),
		},
		ProviderState: codeRabbitStatus{
			State: codeRabbitWatchPending,
		},
		Metadata: map[string]string{
			"provider": "coderabbit",
		},
	}
	if hasStatus {
		applyCommitStatus(&payload.ProviderState, latestStatus)
	}
	if payload.Review.HeadSHA == "" {
		return codeRabbitWatchPayload{}, fmt.Errorf("dev-cycle: pull request head SHA is required")
	}
	if payload.Review.LocalHeadSHA == "" {
		return codeRabbitWatchPayload{}, fmt.Errorf("dev-cycle: local HEAD SHA is required")
	}
	if payload.Review.LocalHeadSHA != payload.Review.HeadSHA {
		payload.ProviderState.State = codeRabbitWatchLocalHeadMismatch
		return payload, nil
	}
	if !hasStatus {
		return payload, nil
	}
	ready, err := applyCodeRabbitStatusGate(&payload.ProviderState, latestStatus, payload.Review.HeadSHA)
	if err != nil || !ready {
		return payload, err
	}
	review, ok := latestCodeRabbitReview(reviews)
	if !ok {
		payload.ProviderState.State = codeRabbitWatchCurrentSettled
		return payload, nil
	}
	payload.Review.ReviewID = strconv.Itoa(review.ID)
	payload.Review.ReviewCommitSHA = strings.TrimSpace(review.CommitID)
	payload.Review.ReviewState = strings.ToLower(strings.TrimSpace(review.State))
	if !reviewStateIsPending(review.State) && strings.TrimSpace(review.SubmittedAt) != "" {
		submittedAt, err := parseReviewSubmittedAt(review.SubmittedAt)
		if err != nil {
			return codeRabbitWatchPayload{}, fmt.Errorf(
				"dev-cycle: decode pull request review %d submitted_at: %w",
				review.ID,
				err,
			)
		}
		payload.SubmittedAt = &submittedAt
	}
	if payload.Review.ReviewCommitSHA == "" || reviewStateIsPending(review.State) {
		payload.ProviderState.State = codeRabbitWatchCurrentSettled
		return payload, nil
	}
	if payload.Review.ReviewCommitSHA != payload.Review.HeadSHA {
		payload.ProviderState.State = codeRabbitWatchStale
		return payload, nil
	}
	switch strings.ToUpper(strings.TrimSpace(review.State)) {
	case "COMMENTED", "CHANGES_REQUESTED", "APPROVED":
		payload.ProviderState.State = codeRabbitWatchCurrentReviewed
	default:
		payload.ProviderState.State = codeRabbitWatchCurrentSettled
	}
	return payload, nil
}

func latestCodeRabbitCommitStatus(statuses []commitStatus) (commitStatus, bool) {
	var latest commitStatus
	found := false
	for _, status := range statuses {
		if !strings.EqualFold(strings.TrimSpace(status.Context), codeRabbitCommitStatusContext) {
			continue
		}
		if !found || commitStatusIsNewer(status, latest) {
			latest = status
			found = true
		}
	}
	return latest, found
}

func commitStatusIsNewer(candidate commitStatus, current commitStatus) bool {
	candidateTime := commitStatusTimestamp(candidate)
	currentTime := commitStatusTimestamp(current)
	if candidateTime.IsZero() {
		return false
	}
	if currentTime.IsZero() {
		return true
	}
	return candidateTime.After(currentTime)
}

func commitStatusTimestamp(status commitStatus) time.Time {
	if updatedAt := parseCommitTimestamp(status.UpdatedAt); !updatedAt.IsZero() {
		return updatedAt
	}
	return parseCommitTimestamp(status.CreatedAt)
}

func applyCommitStatus(target *codeRabbitStatus, status commitStatus) {
	if target == nil {
		return
	}
	target.ProviderState = strings.TrimSpace(status.State)
	target.Description = strings.TrimSpace(status.Description)
	if updatedAt := commitStatusTimestamp(status); !updatedAt.IsZero() {
		target.UpdatedAt = &updatedAt
	}
}

func applyCodeRabbitStatusGate(target *codeRabbitStatus, status commitStatus, headSHA string) (bool, error) {
	if commitStatusIsPending(status) {
		target.State = codeRabbitWatchPending
		return false, nil
	}
	if commitStatusFailed(status) {
		return false, fmt.Errorf(
			"dev-cycle: coderabbit status %q for head %s: %s",
			strings.TrimSpace(status.State),
			strings.TrimSpace(headSHA),
			strings.TrimSpace(status.Description),
		)
	}
	if !commitStatusSucceeded(status) {
		return false, fmt.Errorf(
			"dev-cycle: coderabbit status %q for head %s is unsupported",
			strings.TrimSpace(status.State),
			strings.TrimSpace(headSHA),
		)
	}
	return true, nil
}

func latestCodeRabbitReview(reviews []pullRequestReview) (pullRequestReview, bool) {
	var latest pullRequestReview
	found := false
	for _, review := range reviews {
		if !strings.EqualFold(strings.TrimSpace(review.User.Login), codeRabbitBotLogin) {
			continue
		}
		if !found || reviewIsNewer(review, latest) {
			latest = review
			found = true
		}
	}
	return latest, found
}

func reviewIsNewer(candidate pullRequestReview, current pullRequestReview) bool {
	candidateTime := parseCommitTimestamp(candidate.SubmittedAt)
	currentTime := parseCommitTimestamp(current.SubmittedAt)
	if candidateTime.IsZero() || currentTime.IsZero() {
		return compareReviewIDs(strconv.Itoa(candidate.ID), strconv.Itoa(current.ID)) > 0
	}
	if candidateTime.After(currentTime) {
		return true
	}
	if currentTime.After(candidateTime) {
		return false
	}
	return compareReviewIDs(strconv.Itoa(candidate.ID), strconv.Itoa(current.ID)) > 0
}

func reviewStateIsPending(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "PENDING")
}

func commitStatusIsPending(status commitStatus) bool {
	return strings.EqualFold(strings.TrimSpace(status.State), "pending")
}

func commitStatusSucceeded(status commitStatus) bool {
	return strings.EqualFold(strings.TrimSpace(status.State), "success")
}

func commitStatusFailed(status commitStatus) bool {
	state := strings.ToLower(strings.TrimSpace(status.State))
	return state == "failure" || state == "error"
}

func parseReviewSubmittedAt(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func parseCommitTimestamp(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func compareReviewIDs(left string, right string) int {
	leftID, leftErr := strconv.ParseInt(strings.TrimSpace(left), 10, 64)
	rightID, rightErr := strconv.ParseInt(strings.TrimSpace(right), 10, 64)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftID > rightID:
			return 1
		case leftID < rightID:
			return -1
		default:
			return 0
		}
	}
	return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
}
