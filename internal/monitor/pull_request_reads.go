package monitor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"sync"
	"time"
)

// pullRequestReads shares forge reads: every monitor on a target ticks at the same instant, so one read
// per target per interval answers all of them.
type pullRequestReads struct {
	mu    sync.Mutex
	slots map[PullRequestTarget]*pullRequestRead
}

type pullRequestRead struct {
	startedAt time.Time
	done      chan struct{}
	result    PullRequestObservation
}

// readPullRequest returns the target's shared observation and reports false when ctx ended first. The read
// itself belongs to the service, so a canceled monitor never fails the others waiting on it.
func (s *Service) readPullRequest(ctx context.Context, target PullRequestTarget) (PullRequestObservation, bool) {
	now := s.now()
	reads := &s.prReads
	reads.mu.Lock()
	slot := reads.slots[target]
	if slot == nil || now.Sub(slot.startedAt) >= pullRequestReadSpacing {
		slot = &pullRequestRead{startedAt: now, done: make(chan struct{})}
		if reads.slots == nil {
			reads.slots = make(map[PullRequestTarget]*pullRequestRead)
		}
		for key, stale := range reads.slots {
			if now.Sub(stale.startedAt) >= 2*pullRequestPollInterval {
				delete(reads.slots, key)
			}
		}
		reads.slots[target] = slot
		s.wg.Go(func() {
			defer close(slot.done)
			slot.result = s.fetchPullRequest(target)
		})
	}
	reads.mu.Unlock()
	select {
	case <-ctx.Done():
		return PullRequestObservation{}, false
	case <-slot.done:
		return slot.result, true
	}
}

func (s *Service) fetchPullRequest(target PullRequestTarget) PullRequestObservation {
	ctx, cancel := context.WithTimeout(s.ctx, pullRequestReadTimeout)
	defer cancel()
	if s.pullRequests == nil {
		return PullRequestObservation{Lookup: PullRequestFailed, Cause: "provider_disabled"}
	}
	observation, err := s.pullRequests.ReadPullRequest(ctx, target)
	if err != nil {
		return PullRequestObservation{Lookup: PullRequestFailed, Cause: "read_failed"}
	}
	return boundPullRequestObservation(observation)
}

// boundPullRequestObservation caps forge-sourced facts before they reach a durable cursor.
func boundPullRequestObservation(obs PullRequestObservation) PullRequestObservation {
	if obs.Checks != nil {
		checks := *obs.Checks
		names := make([]string, 0, min(len(checks.FailedNames), maxTrackedFailedNames))
		for _, name := range checks.FailedNames[:min(len(checks.FailedNames), maxTrackedFailedNames)] {
			names = append(names, boundForgeText(name))
		}
		checks.FailedNames = names
		obs.Checks = &checks
	}
	if obs.Reviews != nil {
		reviews := *obs.Reviews
		items := reviews.Items
		if len(items) > maxTrackedReviews {
			items, reviews.Truncated = items[len(items)-maxTrackedReviews:], true
		}
		reviews.Items = make([]PullRequestReview, 0, len(items))
		for _, item := range items {
			item.ID, item.Author = boundForgeID(item.ID), boundForgeText(item.Author)
			item.State, item.Commit = boundForgeText(item.State), boundForgeText(item.Commit)
			reviews.Items = append(reviews.Items, item)
		}
		threads := reviews.Threads
		if len(threads) > maxTrackedThreads {
			threads, reviews.Truncated = threads[len(threads)-maxTrackedThreads:], true
		}
		reviews.Threads = make([]PullRequestThread, 0, len(threads))
		for _, thread := range threads {
			reviews.Threads = append(reviews.Threads, PullRequestThread{
				ID: boundForgeID(thread.ID), Commit: boundForgeText(thread.Commit),
			})
		}
		obs.Reviews = &reviews
	}
	return obs
}

// boundForgeID caps an id without merging two ids that share a long prefix, because seen sets compare it.
func boundForgeID(value string) string {
	if len([]rune(value)) <= maxForgeTextRunes {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func boundForgeText(value string) string {
	runes := []rune(value)
	if len(runes) <= maxForgeTextRunes {
		return value
	}
	return string(runes[:maxForgeTextRunes])
}

// nextPullRequestTick spreads targets across the interval while keeping every monitor of one target on the
// same instant, which is what lets them share a read.
func nextPullRequestTick(target PullRequestTarget, now time.Time) time.Time {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(target.WorkspaceID + "\x00" + target.RepoKey + "\x00" + target.Branch))
	offset := time.Duration(hash.Sum64() % uint64(pullRequestPollInterval))
	tick := now.Truncate(pullRequestPollInterval).Add(offset)
	if !tick.After(now) {
		tick = tick.Add(pullRequestPollInterval)
	}
	return tick
}
