package clientstate

import (
	"sync"
	"time"
)

var latencyBounds = [...]time.Duration{
	100 * time.Microsecond,
	500 * time.Microsecond,
	time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	5 * time.Second,
}

// MetricsSnapshot is a low-cardinality runtime view of client-state health.
type MetricsSnapshot struct {
	ActiveConnections     int64
	ActiveSubscriptions   int64
	OutboundQueueDepthMax int
	OutboundQueueDepthP95 int
	SlowConsumerEvictions uint64
	Reconnects            uint64
	Snapshots             uint64
	SnapshotEntries       uint64
	SnapshotBytes         uint64
	SnapshotDurationP95   time.Duration
	Applies               uint64
	ApplyErrors           uint64
	ApplyLatencyP95       time.Duration
	RevConflicts          uint64
	PurgeFailures         uint64
}

type runtimeMetrics struct {
	mu sync.Mutex

	activeConnections     int64
	activeSubscriptions   int64
	queueDepthMax         int
	queueDepthSamples     [SubscriptionBufferSize + 1]uint64
	slowConsumerEvictions uint64
	reconnects            uint64
	snapshots             uint64
	snapshotEntries       uint64
	snapshotBytes         uint64
	snapshotLatency       [len(latencyBounds) + 1]uint64
	applies               uint64
	applyErrors           uint64
	applyLatency          [len(latencyBounds) + 1]uint64
	revConflicts          uint64
	purgeFailures         uint64
}

func (m *runtimeMetrics) snapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return MetricsSnapshot{
		ActiveConnections:     m.activeConnections,
		ActiveSubscriptions:   m.activeSubscriptions,
		OutboundQueueDepthMax: m.queueDepthMax,
		OutboundQueueDepthP95: percentileIndex(m.queueDepthSamples[:], 95),
		SlowConsumerEvictions: m.slowConsumerEvictions,
		Reconnects:            m.reconnects,
		Snapshots:             m.snapshots,
		SnapshotEntries:       m.snapshotEntries,
		SnapshotBytes:         m.snapshotBytes,
		SnapshotDurationP95:   percentileDuration(m.snapshotLatency[:], 95),
		Applies:               m.applies,
		ApplyErrors:           m.applyErrors,
		ApplyLatencyP95:       percentileDuration(m.applyLatency[:], 95),
		RevConflicts:          m.revConflicts,
		PurgeFailures:         m.purgeFailures,
	}
}

func (m *runtimeMetrics) connectionOpened() {
	m.mu.Lock()
	m.activeConnections++
	m.mu.Unlock()
}

func (m *runtimeMetrics) connectionClosed() {
	m.mu.Lock()
	if m.activeConnections > 0 {
		m.activeConnections--
	}
	m.mu.Unlock()
}

func (m *runtimeMetrics) reconnectRecorded() {
	m.mu.Lock()
	m.reconnects++
	m.mu.Unlock()
}

func (m *runtimeMetrics) subscriptionOpened() {
	m.mu.Lock()
	m.activeSubscriptions++
	m.mu.Unlock()
}

func (m *runtimeMetrics) subscriptionClosed() {
	m.subscriptionsClosed(1)
}

func (m *runtimeMetrics) subscriptionsClosed(count int) {
	m.mu.Lock()
	m.activeSubscriptions -= int64(count)
	if m.activeSubscriptions < 0 {
		m.activeSubscriptions = 0
	}
	m.mu.Unlock()
}

func (m *runtimeMetrics) observeQueueDepth(depth int) {
	if depth < 0 {
		depth = 0
	}
	if depth > SubscriptionBufferSize {
		depth = SubscriptionBufferSize
	}
	m.mu.Lock()
	if depth > m.queueDepthMax {
		m.queueDepthMax = depth
	}
	m.queueDepthSamples[depth]++
	m.mu.Unlock()
}

func (m *runtimeMetrics) slowConsumerEvicted() {
	m.mu.Lock()
	m.slowConsumerEvictions++
	m.mu.Unlock()
}

func (m *runtimeMetrics) snapshotObserved(entries []Entry, duration time.Duration) {
	var payloadBytes uint64
	for _, entry := range entries {
		payloadBytes += uint64(len(entry.Value))
	}
	m.mu.Lock()
	m.snapshots++
	m.snapshotEntries += uint64(len(entries))
	m.snapshotBytes += payloadBytes
	m.snapshotLatency[latencyBucket(duration)]++
	m.mu.Unlock()
}

func (m *runtimeMetrics) applyObserved(duration time.Duration, err error) {
	m.mu.Lock()
	m.applies++
	if err != nil {
		m.applyErrors++
	}
	m.applyLatency[latencyBucket(duration)]++
	m.mu.Unlock()
}

func (m *runtimeMetrics) revConflictObserved() {
	m.mu.Lock()
	m.revConflicts++
	m.mu.Unlock()
}

func (m *runtimeMetrics) purgeFailureObserved() {
	m.mu.Lock()
	m.purgeFailures++
	m.mu.Unlock()
}

func latencyBucket(duration time.Duration) int {
	for index, bound := range latencyBounds {
		if duration <= bound {
			return index
		}
	}
	return len(latencyBounds)
}

func percentileDuration(samples []uint64, percentile uint64) time.Duration {
	if sampleCount(samples) == 0 {
		return 0
	}
	index := percentileIndex(samples, percentile)
	if index < len(latencyBounds) {
		return latencyBounds[index]
	}
	if index == len(latencyBounds) {
		return latencyBounds[len(latencyBounds)-1]
	}
	return 0
}

func sampleCount(samples []uint64) uint64 {
	var total uint64
	for _, sample := range samples {
		total += sample
	}
	return total
}

func percentileIndex(samples []uint64, percentile uint64) int {
	total := sampleCount(samples)
	if total == 0 {
		return 0
	}
	target := (total*percentile + 99) / 100
	var cumulative uint64
	for index, sample := range samples {
		cumulative += sample
		if cumulative >= target {
			return index
		}
	}
	return len(samples) - 1
}
