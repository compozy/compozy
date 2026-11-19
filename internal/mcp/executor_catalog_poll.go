package mcp

import (
	"context"
	"maps"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

type mcpCatalogPoll struct {
	listingKey string
	interval   time.Duration
	expiresAt  time.Time
	failure    tools.ReasonCode
	updatedAt  time.Time
}

func (e *CallExecutor) toolPollKey(ctx context.Context, resolved *ResolvedServer, header string) (string, error) {
	if resolved.listingSpec == nil {
		return e.toolCacheKey(ctx, resolved, "", header)
	}
	// The fallback interval belongs to the launch binding, even when its child has been recycled.
	binding := *resolved
	binding.listingBinding = resolved.listingSpec.digest
	return e.toolCacheKey(ctx, &binding, "", header)
}

func (e *CallExecutor) catalogPoll(key string) mcpCatalogPoll {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	return e.toolCache.polls[key]
}

func catalogPollAttemptKey(binding string) string { return "catalog-poll\x00" + binding }

func (e *CallExecutor) recordCatalogListing(key, pollKey string, interval time.Duration, fallback bool, now time.Time) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	poll := mcpCatalogPoll{listingKey: key, interval: interval, updatedAt: now}
	if fallback {
		poll.expiresAt = now.Add(interval)
	}
	e.storeCatalogPollLocked(pollKey, poll)
}

func (e *CallExecutor) recordCatalogPollFailure(
	key string, previous mcpCatalogPoll, attempt mcpListAttempt, err error,
) {
	e.toolCache.publicationMu.Lock()
	defer e.toolCache.publicationMu.Unlock()
	if e.toolCache.attempts[catalogPollAttemptKey(key)].check != attempt.check {
		return
	}
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	// An explicit check or notification can supersede the failed background attempt.
	if e.toolCache.polls[key] != previous {
		return
	}
	interval := previous.interval
	if interval == 0 {
		interval = e.effectiveToolListTTL(0)
	}
	reason, _ := tools.ReasonOf(err)
	if reason == "" {
		reason = tools.ReasonBackendUnhealthy
	}
	now := time.Now()
	e.storeCatalogPollLocked(key, mcpCatalogPoll{
		interval: interval, expiresAt: now.Add(interval), failure: reason, updatedAt: now,
	})
}

func (e *CallExecutor) storeCatalogPollLocked(key string, poll mcpCatalogPoll) {
	if e.toolCache.polls == nil {
		e.toolCache.polls = make(map[string]mcpCatalogPoll)
	}
	if _, exists := e.toolCache.polls[key]; !exists && len(e.toolCache.polls) >= defaultToolNormalizationCacheCapacity {
		var oldest string
		var updatedAt time.Time
		for binding, previous := range e.toolCache.polls {
			if oldest == "" || previous.updatedAt.Before(updatedAt) {
				oldest, updatedAt = binding, previous.updatedAt
			}
		}
		delete(e.toolCache.polls, oldest)
	}
	e.toolCache.polls[key] = poll
}

func (e *CallExecutor) retainCatalogPolls(active map[string]bool) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	maps.DeleteFunc(e.toolCache.polls, func(key string, _ mcpCatalogPoll) bool {
		return !active[key]
	})
}

// expireSubscribedToolListing falls back to a bounded refresh without waking another reconnect.
func (e *CallExecutor) expireSubscribedToolListing(key string) {
	e.toolCache.publicationMu.Lock()
	defer e.toolCache.publicationMu.Unlock()
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	now := time.Now()
	expiry := now.Add(defaultMCPToolListTTL)
	if poll, ok := e.toolCache.polls[key]; ok && poll.expiresAt.IsZero() {
		expiry = now.Add(poll.interval)
		poll.expiresAt = expiry
		e.toolCache.polls[key] = poll
	}
	if entry, ok := e.toolCache.entries[key]; ok && entry.expiresAt.IsZero() {
		entry.expiresAt = expiry
		e.toolCache.entries[key] = entry
	}
	for source, state := range e.toolCache.projectionStates {
		if state.listingKey == key && state.expiresAt.IsZero() {
			state.expiresAt = expiry
			e.toolCache.projectionStates[source] = state
		}
	}
}
