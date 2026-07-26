package extensionpkg

import (
	"maps"

	"strings"
	"sync"
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/subprocess"
)

type hostAPIRateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   int
	burst   int
	entries map[string]hostAPIRateState
}

type hostAPIRateState struct {
	tokens    float64
	updatedAt time.Time
}

func newHostAPIRateLimiter(limit int, burst int, now func() time.Time) *hostAPIRateLimiter {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &hostAPIRateLimiter{
		now:     now,
		limit:   limit,
		burst:   burst,
		entries: make(map[string]hostAPIRateState),
	}
}

func (l *hostAPIRateLimiter) Allow(extName string, method string) error {
	if l == nil || l.limit <= 0 || l.burst <= 0 {
		return nil
	}

	key := strings.TrimSpace(extName)
	if key == "" {
		key = hostAPIUnknownExtensionName
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.entries[key]
	if state.updatedAt.IsZero() {
		state.tokens = float64(l.burst)
		state.updatedAt = now
	}

	elapsed := now.Sub(state.updatedAt).Seconds()
	if elapsed > 0 {
		state.tokens = minFloat(float64(l.burst), state.tokens+(elapsed*float64(l.limit)))
		state.updatedAt = now
	}

	if state.tokens >= 1 {
		state.tokens--
		l.entries[key] = state
		return nil
	}

	needed := 1 - state.tokens
	retryAfter := max(time.Duration((needed/float64(l.limit))*float64(time.Second)), time.Millisecond)
	l.entries[key] = state

	return subprocess.NewRPCError(HostAPIRateLimitedCode, "Rate limited", map[string]any{
		hostAPIScopeKey:  "host_api." + strings.TrimSpace(method),
		"retry_after_ms": retryAfter.Milliseconds(),
		hostAPILimitKey:  l.limit,
		"burst":          l.burst,
	})
}

func hostAPIJobUpdateTouchesImmutableConfigFields(req apicontract.UpdateJobRequest) bool {
	return req.Name != nil ||
		req.Prompt != nil ||
		req.Schedule != nil ||
		req.Task != nil ||
		req.LoopTarget != nil ||
		req.Retry != nil ||
		req.FireLimit != nil
}

func hostAPITriggerUpdateTouchesImmutableConfigFields(req apicontract.UpdateTriggerRequest) bool {
	return req.Name != nil ||
		req.Prompt != nil ||
		req.Event != nil ||
		req.Filter != nil ||
		req.LoopTarget != nil ||
		req.Retry != nil ||
		req.FireLimit != nil ||
		req.WebhookID != nil ||
		req.EndpointSlug != nil ||
		req.WebhookSecretValue != nil
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func cloneJSONMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(source))
	maps.Copy(cloned, source)
	return cloned
}

func cloneHostAPIStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}
