//go:build test

package ingest

import "sync"

var metricsMu sync.Mutex

// ResetMetricsForTesting clears metric state to allow deterministic test assertions.
func ResetMetricsForTesting() {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	metricsOnce = sync.Once{}
	metricsInitErr = nil
	pipelineLatency = nil
	documentsCounter = nil
	chunksCounter = nil
	batchSizeHistogram = nil
	errorsCounter = nil
}
