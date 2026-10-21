package daemon

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/providerusage"
)

type providerUsageMetricCount struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Count    uint64 `json:"count"`
}

type providerUsageMetrics struct {
	mu      sync.Mutex
	counts  map[[2]string]uint64
	buckets [7]uint64
	count   uint64
	sum     float64
}

const providerUsageCountKey = "count"

var providerUsageDurationBounds = [...]float64{10, 100, 500, 1000, 5000, 15000}

func (m *providerUsageMetrics) Collect(provider string, status providerusage.Status, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counts == nil {
		m.counts = make(map[[2]string]uint64)
	}
	m.counts[[2]string{provider, string(status)}]++
	ms := float64(duration) / float64(time.Millisecond)
	m.count++
	m.sum += ms
	for i, bound := range providerUsageDurationBounds {
		if ms <= bound {
			m.buckets[i]++
		}
	}
	m.buckets[len(m.buckets)-1]++
}

func (r *providerUsageRuntime) UsageMetrics() map[string]any {
	m := r.metrics
	m.mu.Lock()
	defer m.mu.Unlock()
	counts := make([]providerUsageMetricCount, 0, len(m.counts))
	for labels, count := range m.counts {
		counts = append(counts, providerUsageMetricCount{Provider: labels[0], Status: labels[1], Count: count})
	}
	slices.SortFunc(counts, func(a, b providerUsageMetricCount) int {
		if a.Provider != b.Provider {
			return strings.Compare(a.Provider, b.Provider)
		}
		return strings.Compare(a.Status, b.Status)
	})
	return map[string]any{
		"provider_usage_collect_total": counts,
		"provider_usage_collect_duration_ms": map[string]any{
			"bounds_ms":           providerUsageDurationBounds,
			"buckets":             m.buckets,
			providerUsageCountKey: m.count,
			"sum":                 m.sum,
		},
	}
}
