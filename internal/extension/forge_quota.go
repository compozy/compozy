package extensionpkg

import (
	"strconv"
	"strings"
	"sync"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

const (
	// forgeCapabilitiesTTL bounds how long a capability answer is reused for one remote set. A provider
	// restart or a changed provider set changes the cache key, so a stale winner is never reused.
	forgeCapabilitiesTTL = 10 * time.Minute
	// forgeRateLimitFallback holds a rate-limited provider when it named no reset time, as GitHub
	// advises for secondary limits without Retry-After.
	forgeRateLimitFallback = time.Minute
)

// forgeQuota is the daemon-wide forge read budget. Every forge read goes through the Manager, so a
// provider that reported a rate limit is not called again by any reader until its reset.
type forgeQuota struct {
	mu           sync.Mutex
	capabilities map[string]forgeCapabilitiesEntry
	limited      map[string]time.Time
}

type forgeCapabilitiesEntry struct {
	response extensioncontract.ForgeCapabilitiesResponse
	until    time.Time
}

func forgeCapabilitiesKey(generation string, remotes []string) string {
	return generation + "\x00" + strings.Join(remotes, "\n")
}

func forgeProviderGeneration(name string, startedAt time.Time) string {
	return name + "@" + strconv.FormatInt(startedAt.UnixNano(), 10)
}

func (q *forgeQuota) cachedCapabilities(
	key string,
	now time.Time,
) (extensioncontract.ForgeCapabilitiesResponse, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.capabilities[key]
	if !ok || !now.Before(entry.until) {
		return extensioncontract.ForgeCapabilitiesResponse{}, false
	}
	return cloneForgeCapabilities(entry.response), true
}

func (q *forgeQuota) storeCapabilities(
	key string,
	response extensioncontract.ForgeCapabilitiesResponse,
	now time.Time,
) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.capabilities == nil {
		q.capabilities = make(map[string]forgeCapabilitiesEntry)
	}
	for cached, entry := range q.capabilities {
		if !now.Before(entry.until) {
			delete(q.capabilities, cached)
		}
	}
	q.capabilities[key] = forgeCapabilitiesEntry{
		response: cloneForgeCapabilities(response), until: now.Add(forgeCapabilitiesTTL),
	}
}

// limitedUntil reports the reset time while provider is rate-limited.
func (q *forgeQuota) limitedUntil(provider string, now time.Time) (time.Time, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	until, ok := q.limited[provider]
	if !ok {
		return time.Time{}, false
	}
	if !now.Before(until) {
		delete(q.limited, provider)
		return time.Time{}, false
	}
	return until, true
}

// limit records a rate limit and returns the time the provider may be called again. A later reset
// wins, so a short secondary-limit hint never shortens a known primary reset.
func (q *forgeQuota) limit(provider string, retryAfter *time.Time, now time.Time) time.Time {
	until := now.Add(forgeRateLimitFallback)
	if retryAfter != nil && retryAfter.After(now) {
		until = retryAfter.UTC()
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.limited == nil {
		q.limited = make(map[string]time.Time)
	}
	if current, ok := q.limited[provider]; ok && current.After(until) {
		return current
	}
	q.limited[provider] = until
	return until
}

func forgeRateLimitedError(until time.Time) error {
	return &ForgeProviderError{cause: extensioncontract.ForgeCauseRateLimited, RetryAfter: &until}
}

func cloneForgeCapabilities(
	response extensioncontract.ForgeCapabilitiesResponse,
) extensioncontract.ForgeCapabilitiesResponse {
	response.TemplatePaths = append([]string(nil), response.TemplatePaths...)
	if response.RetryAfter != nil {
		response.RetryAfter = new(*response.RetryAfter)
	}
	return response
}
