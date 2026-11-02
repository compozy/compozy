package monitor

import (
	"slices"
	"time"
)

const (
	pullRequestMerged = "merged"
	pullRequestClosed = "closed"
)

// detectPullRequest is the whole change policy: from what the agent was told and one observation, it
// returns what this wake must say, if anything, and the next cursor. It never mutates cur, so a wake
// that is not admitted leaves cur to be reported again.
func detectPullRequest(cur prCursor, obs PullRequestObservation, now time.Time) (PullRequestChanges, prCursor) {
	next := cur
	switch obs.Lookup {
	case PullRequestFailed:
		return detectOutage(cur, obs, now)
	case PullRequestAbsent:
		next.OutageSince, next.OutageReported = time.Time{}, false
		return PullRequestChanges{}, next
	}
	next.OutageSince, next.OutageReported = time.Time{}, false
	changes := PullRequestChanges{Number: obs.Number, URL: obs.URL, State: obs.State, Head: obs.Head}
	next = resetForNewPullRequest(next, obs)
	if obs.State == pullRequestMerged || obs.State == pullRequestClosed {
		changes.Terminal, changes.Wake = obs.State, true
		return flushBurst(changes, next)
	}
	next = detectChecks(&changes, next, obs.Checks)
	next = detectReviews(&changes, next, obs.Reviews, obs.Head, now)
	if changes.Wake || burstDue(next.Burst, now) {
		return flushBurst(changes, next)
	}
	return changes, next
}

func detectOutage(cur prCursor, obs PullRequestObservation, now time.Time) (PullRequestChanges, prCursor) {
	next := cur
	if next.OutageSince.IsZero() {
		next.OutageSince = now
	}
	if next.OutageReported || now.Sub(next.OutageSince) < forgeOutageGrace {
		return PullRequestChanges{}, next
	}
	next.OutageReported = true
	return PullRequestChanges{
		Wake: true, ForgeDown: true, Cause: obs.Cause, Number: cur.Number, Head: cur.Head,
	}, next
}

// resetForNewPullRequest drops what belongs to a superseded head. A new head does not wake by itself, and
// a different pull request on the branch also starts a new review baseline.
func resetForNewPullRequest(next prCursor, obs PullRequestObservation) prCursor {
	samePull := next.Number == obs.Number
	if samePull && next.Head == obs.Head {
		return next
	}
	if !samePull {
		next.ReviewsBaselined, next.Unresolved, next.Burst, next.SeenReviews = false, 0, nil, nil
	}
	next.Number, next.Head = obs.Number, obs.Head
	next.Failed, next.FailedNames, next.Finished = 0, nil, false
	if next.Burst != nil {
		burst := *next.Burst
		burst.Reviews = nil
		next.Burst = &burst
		if burst.Unresolved <= next.Unresolved {
			next.Burst = nil
		}
	}
	return next
}

func detectChecks(changes *PullRequestChanges, next prCursor, checks *PullRequestChecks) prCursor {
	if checks == nil {
		return next
	}
	changes.ChecksKnown = true
	changes.Passed, changes.Failed, changes.Pending = checks.Passed, checks.Failed, checks.Pending
	changes.FailedNames = checks.FailedNames
	for _, name := range checks.FailedNames {
		if !slices.Contains(next.FailedNames, name) {
			changes.NewlyFailed = append(changes.NewlyFailed, name)
		}
	}
	changes.FailureRose = checks.Failed > next.Failed
	if len(changes.NewlyFailed) > 0 || changes.FailureRose {
		changes.Wake = true
	}
	next.Failed, next.FailedNames = checks.Failed, checks.FailedNames
	if checks.Pending == 0 && checks.Passed+checks.Failed > 0 && !next.Finished {
		next.Finished, changes.AllFinished, changes.Wake = true, true, true
	}
	return next
}

func detectReviews(
	changes *PullRequestChanges,
	next prCursor,
	reviews *PullRequestReviews,
	head string,
	now time.Time,
) prCursor {
	if reviews == nil {
		return next
	}
	changes.ReviewsKnown, changes.Truncated = true, reviews.Truncated
	changes.Unresolved = reviews.UnresolvedThreads
	var fresh []PullRequestReview
	for _, review := range reviews.Items {
		if (review.Commit == "" || review.Commit == head) && !slices.Contains(next.SeenReviews, review.ID) {
			fresh = append(fresh, review)
		}
	}
	seen := slices.Clone(next.SeenReviews)
	for _, review := range fresh {
		seen = append(seen, review.ID)
	}
	next.SeenReviews = seen[max(len(seen)-maxSeenReviews, 0):]
	if !next.ReviewsBaselined {
		next.ReviewsBaselined, next.Unresolved = true, reviews.UnresolvedThreads
		return next
	}
	next.Unresolved = min(next.Unresolved, reviews.UnresolvedThreads)
	rising := reviews.UnresolvedThreads > next.Unresolved
	if next.Burst == nil {
		if len(fresh) == 0 && !rising {
			return next
		}
		next.Burst = &prReviewBurst{Since: now, Last: now}
	}
	burst := *next.Burst
	if len(fresh) > 0 || (rising && reviews.UnresolvedThreads != burst.Unresolved) {
		burst.Last = now
	}
	burst.Unresolved = reviews.UnresolvedThreads
	burst.Reviews = slices.Concat(burst.Reviews, fresh)
	burst.Reviews = burst.Reviews[max(len(burst.Reviews)-maxBurstReviews, 0):]
	next.Burst = &burst
	if len(burst.Reviews) == 0 && !rising {
		next.Burst = nil // The threads were resolved again before the burst woke anyone.
	}
	return next
}

func burstDue(burst *prReviewBurst, now time.Time) bool {
	return burst != nil && (now.Sub(burst.Last) >= reviewQuiet || now.Sub(burst.Since) >= reviewHold)
}

// flushBurst moves pending review activity into a wake that is firing anyway or whose debounce ended.
func flushBurst(changes PullRequestChanges, next prCursor) (PullRequestChanges, prCursor) {
	if next.Burst == nil {
		return changes, next
	}
	changes.Wake = true
	changes.Reviews = next.Burst.Reviews
	changes.NewThreads = max(next.Burst.Unresolved-next.Unresolved, 0)
	next.Unresolved = max(next.Unresolved, next.Burst.Unresolved)
	next.Burst = nil
	return changes, next
}
