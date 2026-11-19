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

func (e *CallExecutor) recordCatalogListing(key, pollKey string, interval time.Duration, fallback bool, now time.Time) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	if e.toolCache.polls == nil {
		e.toolCache.polls = make(map[string]mcpCatalogPoll)
	}
	poll := mcpCatalogPoll{listingKey: key, interval: interval}
	if fallback {
		poll.expiresAt = now.Add(interval)
	}
	e.toolCache.polls[pollKey] = poll
}

func (e *CallExecutor) recordCatalogPollFailure(key string, previous mcpCatalogPoll, err error) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	// An explicit check or notification can supersede the failed background attempt.
	if e.toolCache.polls[key] != previous {
		return
	}
	if e.toolCache.polls == nil {
		e.toolCache.polls = make(map[string]mcpCatalogPoll)
	}
	interval := previous.interval
	if interval == 0 {
		interval = e.effectiveToolListTTL(0)
	}
	reason, _ := tools.ReasonOf(err)
	if reason == "" {
		reason = tools.ReasonBackendUnhealthy
	}
	e.toolCache.polls[key] = mcpCatalogPoll{
		interval: interval, expiresAt: time.Now().Add(interval), failure: reason,
	}
}

func (e *CallExecutor) retainCatalogPolls(active map[string]bool) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	maps.DeleteFunc(e.toolCache.polls, func(key string, _ mcpCatalogPoll) bool {
		return !active[key]
	})
}
