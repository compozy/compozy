package monitor

import (
	"slices"
	"time"
)

const (
	pullRequestMerged = "merged"
	pullRequestClosed = "closed"
)

// detectPullRequest is the whole change policy: from the base cursor and one observation, it returns what
// a wake must say, if anything, and the next cursor. It never mutates cur.
func detectPullRequest(cur prCursor, obs PullRequestObservation, now time.Time) (PullRequestChanges, prCursor) {
	next := cur
	switch obs.Lookup {
	case PullRequestFailed:
		return detectOutage(cur, obs, now)
	case PullRequestAbsent:
		next.OutageSince, next.OutageReported = time.Time{}, false
		if burstDue(next.Burst, now) {
			return flushBurst(PullRequestChanges{Number: cur.Number, Head: cur.Head}, next)
		}
		return PullRequestChanges{}, next
	}
	missedReads := !cur.OutageSince.IsZero()
	next.OutageSince, next.OutageReported = time.Time{}, false
	terminal := obs.State == pullRequestMerged || obs.State == pullRequestClosed
	if terminal && obs.Number != cur.Number {
		// A branch can be reused: an ended pull request this watch never saw open is history, not an outcome.
		next.EndedNumber = obs.Number
		return PullRequestChanges{}, next
	}
	changes := PullRequestChanges{Number: obs.Number, URL: obs.URL, State: obs.State, Head: obs.Head}
	next = resetForNewPullRequest(next, obs)
	if terminal {
		changes.Terminal, changes.Wake = obs.State, true
		return flushBurst(changes, next)
	}
	next = detectChecks(&changes, next, obs.Checks)
	next = detectReviews(&changes, next, obs, now, missedReads)
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
	if next.Number == obs.Number && next.Head == obs.Head {
		return next
	}
	if next.Number != obs.Number {
		next.ReviewsBaselined, next.Burst = false, nil
		next.SeenReviews, next.SeenThreads = nil, nil
	}
	if burst := next.Burst; burst != nil {
		// Forgetting the burst's ids lets an item that really belongs to the new head, such as one from a
		// forge that reports no commit, be seen again; anything tied to the old head stays filtered out.
		next.SeenReviews = slices.DeleteFunc(slices.Clone(next.SeenReviews), func(id string) bool {
			return slices.ContainsFunc(burst.Reviews, func(r PullRequestReview) bool { return r.ID == id })
		})
		next.SeenThreads = slices.DeleteFunc(slices.Clone(next.SeenThreads), func(id string) bool {
			return slices.Contains(burst.Threads, id)
		})
		next.Burst = nil
	}
	next.Number, next.Head = obs.Number, obs.Head
	next.Failed, next.FailedNames, next.Finished = 0, nil, false
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
	// With more failures than names, a named failure that goes away while the total holds means an unnamed
	// one took its place. Comparing against the stored names reports that swap once.
	swapped := checks.Failed > len(checks.FailedNames) && checks.Failed >= next.Failed &&
		slices.ContainsFunc(next.FailedNames, func(name string) bool {
			return !slices.Contains(checks.FailedNames, name)
		})
	changes.FailureRose = swapped || checks.Failed > next.Failed
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
	obs PullRequestObservation,
	now time.Time,
	missedReads bool,
) prCursor {
	if obs.ReviewsFailed {
		return detectReviewsDown(changes, next, now)
	}
	reviews := obs.Reviews
	if reviews == nil {
		return next
	}
	missedReads = missedReads || !next.ReviewsDownSince.IsZero()
	next.ReviewsDownSince, next.ReviewsDownSaid = time.Time{}, false
	changes.ReviewsKnown, changes.Truncated = true, reviews.Truncated
	changes.Unresolved = reviews.UnresolvedThreads
	freshReviews := unseenReviews(reviews.Items, next.SeenReviews, obs.Head)
	freshThreads := unseenThreads(reviews.Threads, next.SeenThreads, obs.Head)
	seen := slices.Clone(next.SeenReviews)
	for _, review := range freshReviews {
		seen = append(seen, review.ID)
	}
	next.SeenReviews = seen[max(len(seen)-maxSeenReviews, 0):]
	if !next.ReviewsBaselined && !missedReads {
		next.ReviewsBaselined = true
		next.SeenThreads = threadIDs(reviews.Threads)
		return next
	}
	// A baseline taken after failed reads cannot tell old activity from what arrived meanwhile, so it is
	// reported instead of being absorbed.
	next.ReviewsBaselined = true
	unresolved := threadIDs(reviews.Threads)
	next.SeenThreads = keepUnresolved(append(slices.Clone(next.SeenThreads), freshThreads...), unresolved)
	return extendBurst(next, freshReviews, freshThreads, unresolved, now)
}

// detectReviewsDown keeps the baseline, the seen sets and any burst exactly as they were: a failed read
// says nothing about reviews. It speaks once if the failure outlasts the grace.
func detectReviewsDown(changes *PullRequestChanges, next prCursor, now time.Time) prCursor {
	changes.ReviewsFailed = true
	if next.ReviewsDownSince.IsZero() {
		next.ReviewsDownSince = now
	}
	if !next.ReviewsDownSaid && now.Sub(next.ReviewsDownSince) >= forgeOutageGrace {
		next.ReviewsDownSaid, changes.ReviewsDown, changes.Wake = true, true, true
	}
	return next
}

func unseenReviews(items []PullRequestReview, seen []string, head string) []PullRequestReview {
	var fresh []PullRequestReview
	for _, review := range items {
		if (review.Commit == "" || review.Commit == head) && !slices.Contains(seen, review.ID) {
			fresh = append(fresh, review)
		}
	}
	return fresh
}

func unseenThreads(threads []PullRequestThread, seen []string, head string) []string {
	var fresh []string
	for _, thread := range threads {
		if (thread.Commit == "" || thread.Commit == head) && !slices.Contains(seen, thread.ID) {
			fresh = append(fresh, thread.ID)
		}
	}
	return fresh
}

func threadIDs(threads []PullRequestThread) []string {
	ids := make([]string, 0, len(threads))
	for _, thread := range threads {
		ids = append(ids, thread.ID)
	}
	return ids
}

// keepUnresolved forgets resolved threads, so one that reopens counts as new again.
func keepUnresolved(ids, unresolved []string) []string {
	return slices.DeleteFunc(ids, func(id string) bool { return !slices.Contains(unresolved, id) })
}

func extendBurst(
	next prCursor,
	freshReviews []PullRequestReview,
	freshThreads, unresolved []string,
	now time.Time,
) prCursor {
	active := len(freshReviews) > 0 || len(freshThreads) > 0
	if next.Burst == nil && !active {
		return next
	}
	burst := prReviewBurst{Since: now, Last: now}
	if next.Burst != nil {
		burst = *next.Burst
	}
	if active {
		burst.Last = now
	}
	burst.Reviews = slices.Concat(burst.Reviews, freshReviews)
	burst.Reviews = burst.Reviews[max(len(burst.Reviews)-maxBurstReviews, 0):]
	burst.Threads = keepUnresolved(slices.Concat(burst.Threads, freshThreads), unresolved)
	next.Burst = &burst
	if len(burst.Reviews) == 0 && len(burst.Threads) == 0 {
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
	changes.NewThreads = len(next.Burst.Threads)
	next.Burst = nil
	return changes, next
}

// mergePullRequestChanges folds what later reports into what is already waiting for the next fire. It
// keeps everything the agent still has to hear, except what a new head or pull request superseded.
func mergePullRequestChanges(carry, later PullRequestChanges) PullRequestChanges {
	if carry.Head != "" && later.Head != "" && (carry.Head != later.Head || carry.Number != later.Number) {
		carry = PullRequestChanges{}
	}
	if !later.Wake {
		return carry
	}
	if !carry.Wake {
		return later
	}
	if later.State == "" { // No observation behind it: only the outage is news.
		carry.ForgeDown, carry.Cause = carry.ForgeDown || later.ForgeDown, later.Cause
		return carry
	}
	merged := later
	for _, name := range carry.NewlyFailed {
		if !slices.Contains(merged.NewlyFailed, name) {
			merged.NewlyFailed = append(merged.NewlyFailed, name)
		}
	}
	merged.Reviews = slices.Concat(carry.Reviews, later.Reviews)
	merged.Reviews = merged.Reviews[max(len(merged.Reviews)-maxBurstReviews, 0):]
	merged.NewThreads += carry.NewThreads
	merged.FailureRose = merged.FailureRose || carry.FailureRose
	merged.AllFinished = merged.AllFinished || carry.AllFinished
	merged.ForgeDown = merged.ForgeDown || carry.ForgeDown
	merged.ReviewsDown = merged.ReviewsDown || carry.ReviewsDown
	if merged.Cause == "" {
		merged.Cause = carry.Cause
	}
	return merged
}
