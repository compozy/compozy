// SPDX-License-Identifier: FSL-1.1-MIT

package observe

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Labels map[string]string

type Sample struct {
	Name   string
	Labels Labels
	Value  float64
}

type Metrics struct {
	mu     sync.Mutex
	values map[string]Sample
}

func NewMetrics() *Metrics {
	m := &Metrics{values: make(map[string]Sample)}
	m.add("cloud_stripe_webhook_failures_total", nil, 0)
	const providerMetricLabel = "provider"
	for _, provider := range [...]string{"github", "google"} {
		for _, outcome := range [...]string{"ok", "error"} {
			m.add(
				"cloud_sign_ins_total",
				Labels{providerMetricLabel: provider, "result": outcome, "new_account": "false"},
				0,
			)
		}
		m.add("cloud_sign_ins_total", Labels{providerMetricLabel: provider, "result": "ok", "new_account": "true"}, 0)
		m.add("cloud_new_accounts_total", Labels{providerMetricLabel: provider}, 0)
	}
	return m
}

func (m *Metrics) Paywall(reason string) {
	m.Count("cloud_paywall_hits_total", Labels{"reason": reason})
}

func (m *Metrics) Count(name string, labels Labels) {
	if m != nil {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.add(name, labels, 1)
	}
}

func (m *Metrics) Duration(name string, value time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seconds := value.Seconds()
	for _, bound := range [...]float64{.005, .01, .025, .05, .1, .2, .3, .5, 1, 2, 5} {
		var count float64
		if seconds <= bound {
			count = 1
		}
		m.add(name+"_bucket", Labels{"le": strconv.FormatFloat(bound, 'f', -1, 64)}, count)
	}
	m.add(name+"_bucket", Labels{"le": "+Inf"}, 1)
	m.add(name+"_count", nil, 1)
	m.add(name+"_sum", nil, seconds)
}

func (m *Metrics) add(name string, labels Labels, value float64) {
	var key strings.Builder
	key.WriteString(name)
	for _, label := range slices.Sorted(maps.Keys(labels)) {
		key.WriteByte(0)
		key.WriteString(label)
		key.WriteByte(0)
		key.WriteString(labels[label])
	}
	index := key.String()
	entry, exists := m.values[index]
	if !exists {
		entry = Sample{Name: name, Labels: maps.Clone(labels)}
	}
	entry.Value += value
	m.values[index] = entry
}

func (m *Metrics) Samples() []Sample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Collect(maps.Values(m.values))
}
