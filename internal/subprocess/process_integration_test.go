//go:build integration

package subprocess

import (
	"sync"
	"testing"
	"time"
)

func TestProcessIntegrationLifecycle(t *testing.T) {
	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)

	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 100,
		HealthCheckTimeoutMS:  25,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	var response struct {
		Message string `json:"message"`
	}
	if err := process.Call(testContext(t), "echo", map[string]string{"message": "integration"}, &response); err != nil {
		t.Fatalf("Call(echo) error = %v", err)
	}
	if response.Message != "integration" {
		t.Fatalf("Call(echo) response = %#v, want integration", response)
	}
}

func TestProcessIntegrationCrashRecovery(t *testing.T) {
	process := launchHelperProcess(t, "crash_after_init", LaunchConfig{})

	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 100,
		HealthCheckTimeoutMS:  25,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	select {
	case <-process.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for crash_after_init helper to exit")
	}

	if err := process.Wait(); err == nil {
		t.Fatal("Wait() error = nil, want unexpected exit")
	}
}

func TestProcessIntegrationConcurrentRequests(t *testing.T) {
	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)

	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 100,
		HealthCheckTimeoutMS:  25,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	type result struct {
		index   int
		message string
		err     error
	}

	results := make(chan result, 3)
	var wg sync.WaitGroup
	for index, delay := range []int64{80, 10, 40} {
		wg.Add(1)
		go func(index int, delay int64) {
			defer wg.Done()
			var response struct {
				Message string `json:"message"`
			}
			err := process.Call(testContext(t), "sleep", map[string]any{
				"delay_ms": delay,
				"message":  "req-" + string(rune('A'+index)),
			}, &response)
			results <- result{index: index, message: response.Message, err: err}
		}(index, delay)
	}

	wg.Wait()
	close(results)

	seen := make(map[int]string, 3)
	for item := range results {
		if item.err != nil {
			t.Fatalf("concurrent Call() error = %v", item.err)
		}
		seen[item.index] = item.message
	}

	if seen[0] != "req-A" || seen[1] != "req-B" || seen[2] != "req-C" {
		t.Fatalf("concurrent Call() results = %#v, want indexed responses", seen)
	}
}
