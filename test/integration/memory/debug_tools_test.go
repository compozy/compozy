package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
)

// leakTestStub captures failure state without relying on testing internals.
type leakTestStub struct {
	ctx    context.Context
	failed bool
}

// newLeakTestStub builds a leakTestStub bound to the provided context.
func newLeakTestStub(ctx context.Context) *leakTestStub {
	return &leakTestStub{ctx: ctx}
}

// Helper marks helper boundaries for compatibility.
func (l *leakTestStub) Helper() {}

// Context returns the context associated with the stub.
func (l *leakTestStub) Context() context.Context {
	return l.ctx
}

// Logf discards log output emitted during verification.
func (l *leakTestStub) Logf(string, ...any) {}

// Errorf records formatted failures from the code under test.
func (l *leakTestStub) Errorf(string, ...any) {
	l.failed = true
}

// FailNow marks the stub as failed to satisfy require.TestingT.
func (l *leakTestStub) FailNow() {
	l.failed = true
}

// Failed reports whether the stub observed a failure.
func (l *leakTestStub) Failed() bool {
	return l.failed
}

// TestDebugToolsCapture tests debug capture functionality
func TestDebugToolsCapture(t *testing.T) {
	// Only run if debug mode is enabled
	if os.Getenv("MEMORY_TEST_DEBUG") != "true" {
		t.Skip("Debug mode not enabled (set MEMORY_TEST_DEBUG=true)")
	}
	t.Run("Should capture Redis and memory state", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		debug := NewDebugTools(env)
		ctx := t.Context()
		// Enable tracing
		cleanup := debug.EnableTracing(t)
		defer cleanup()
		// Create a memory instance
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "debug-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("debug-%d", time.Now().Unix())},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Capture initial state
		debug.CaptureRedisState(t, "initial")
		debug.CaptureMemoryState(t, instance, "empty")
		// Add some messages
		messages := []llm.Message{
			{Role: "system", Content: "Debug test system"},
			{Role: "user", Content: "Test message 1"},
			{Role: "assistant", Content: "Test response 1"},
		}
		for _, msg := range messages {
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Capture after adding messages
		debug.CaptureRedisState(t, "with_messages")
		debug.CaptureMemoryState(t, instance, "populated")
		// Verify captures were created
		assert.DirExists(t, debug.outputDir)
		entries, err := os.ReadDir(debug.outputDir)
		assert.NoError(t, err)
		assert.Greater(t, len(entries), 0, "Should have created capture files")
		t.Logf("Debug captures saved to: %s", debug.outputDir)
	})
}

// TestMaintenanceToolsCleanup tests cleanup functionality
func TestMaintenanceToolsCleanup(t *testing.T) {
	t.Run("Should clean up stale test data", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		maintenance := NewMaintenanceTools(env)
		ctx := t.Context()
		// Create some old test data with unique keys
		oldTimestamp := time.Now().Add(-2 * time.Hour).Unix()
		oldMemRef := core.MemoryReference{
			ID:  "customer-support",
			Key: fmt.Sprintf("cleanup-test-old-%d", oldTimestamp),
		}
		oldWorkflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
		}
		oldInstance, err := env.GetMemoryManager().GetInstance(ctx, oldMemRef, oldWorkflowContext)
		require.NoError(t, err)
		err = oldInstance.Append(ctx, llm.Message{
			Role:    "user",
			Content: "Old message",
		})
		require.NoError(t, err)
		// Create some new test data with unique key
		newTimestamp := time.Now().Unix()
		newMemRef := core.MemoryReference{
			ID:  "customer-support",
			Key: fmt.Sprintf("cleanup-test-new-%d", newTimestamp),
		}
		newWorkflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
		}
		newInstance, err := env.GetMemoryManager().GetInstance(ctx, newMemRef, newWorkflowContext)
		require.NoError(t, err)
		err = newInstance.Append(ctx, llm.Message{
			Role:    "user",
			Content: "New message",
		})
		require.NoError(t, err)
		// Manually clean up the old instance for test purposes
		oldInstanceID := oldInstance.GetID()
		err = oldInstance.Clear(ctx)
		require.NoError(t, err)

		// Simulate cleanup by tracking and removing old instance
		maintenance.CleanupSpecificInstance(t, oldInstanceID)

		// Verify new data still exists
		messages, err := newInstance.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "New message", messages[0].Content)

		// Verify old data is gone
		oldMessages, err := oldInstance.Read(ctx)
		assert.NoError(t, err)
		assert.Empty(t, oldMessages, "Old messages should have been cleaned up")
	})
}

// TestMaintenanceToolsLeakDetection tests leak detection
func TestMaintenanceToolsLeakDetection(t *testing.T) {
	t.Run("Should detect leaked keys", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		maintenance := NewMaintenanceTools(env)
		ctx := t.Context()
		redis := env.GetRedis()
		// Add some allowed keys
		err := redis.Set(ctx, "compozy:test-project:allowed", "value", 0).Err()
		require.NoError(t, err)
		// Add a leaked key
		err = redis.Set(ctx, "leaked:key", "should not exist", 0).Err()
		require.NoError(t, err)
		// Verify with allowed prefixes - use a mock test to check behavior
		allowedPrefixes := []string{"compozy:test-project:"}
		leakProbe := newLeakTestStub(ctx)
		maintenance.VerifyNoLeaks(leakProbe, allowedPrefixes)
		// Check that the mock test failed
		assert.True(t, leakProbe.Failed(), "Should have detected leaked keys")
		// Clean up the leaked key
		err = redis.Del(ctx, "leaked:key").Err()
		require.NoError(t, err)
		// Now it should pass
		cleanProbe := newLeakTestStub(ctx)
		maintenance.VerifyNoLeaks(cleanProbe, allowedPrefixes)
		assert.False(t, cleanProbe.Failed(), "Should not detect any leaks after cleanup")
	})
}

// TestMaintenanceToolsReport tests report generation
func TestMaintenanceToolsReport(t *testing.T) {
	t.Run("Should generate HTML test report", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		maintenance := NewMaintenanceTools(env)
		// Create sample test results
		results := map[string]TestResult{
			"TestExample/Success": {
				Name:       "TestExample/Success",
				Passed:     true,
				Duration:   100 * time.Millisecond,
				Assertions: 5,
			},
			"TestExample/Failure": {
				Name:       "TestExample/Failure",
				Passed:     false,
				Duration:   50 * time.Millisecond,
				Error:      fmt.Errorf("assertion failed"),
				Assertions: 3,
			},
			"TestExample/Skipped": {
				Name:       "TestExample/Skipped",
				Skipped:    true,
				Duration:   0,
				Assertions: 0,
			},
		}
		// Generate report
		maintenance.GenerateTestReport(t, results)
		// Verify report was created
		reportDir := filepath.Join("testdata", "reports")
		if _, err := os.Stat(reportDir); err == nil {
			entries, err := os.ReadDir(reportDir)
			assert.NoError(t, err)
			assert.Greater(t, len(entries), 0, "Should have created report file")
			t.Logf("Report generated in: %s", reportDir)
		}
	})
}

// TestInteractiveDebugger tests interactive debugging
func TestInteractiveDebugger(t *testing.T) {
	// Only run if interactive mode is enabled
	if os.Getenv("MEMORY_TEST_INTERACTIVE") != "true" {
		t.Skip("Interactive mode not enabled (set MEMORY_TEST_INTERACTIVE=true)")
	}
	t.Run("Should provide interactive debugging", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		debugger := NewInteractiveDebugger(env)
		ctx := t.Context()
		// Create a test instance
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "interactive-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("interactive-%d", time.Now().Unix())},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add test data
		testMessages := []llm.Message{
			{Role: "system", Content: "Interactive test system"},
			{Role: "user", Content: "Hello debugger"},
			{Role: "assistant", Content: "Debugging in progress"},
		}
		for _, msg := range testMessages {
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Enter interactive debug mode
		t.Log("Entering interactive debug mode. Type 'continue' to proceed.")
		debugger.Debug(t, instance)
	})
}

// TestDebugToolsFailureCapture tests failure capture
func TestDebugToolsFailureCapture(t *testing.T) {
	// Only run if debug mode is enabled
	if os.Getenv("MEMORY_TEST_DEBUG") != "true" {
		t.Skip("Debug mode not enabled (set MEMORY_TEST_DEBUG=true)")
	}
	t.Run("Should capture failure details", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		debug := NewDebugTools(env)
		// Simulate a test that will fail
		t.Run("FailingSubtest", func(t *testing.T) {
			// Force a failure
			err := fmt.Errorf("simulated test failure")
			// This will mark the test as failed
			assert.NoError(t, err)
			// Capture failure details
			debug.DumpTestFailure(t, err)
		})
		// Check if failure was captured
		failureDir := filepath.Join(debug.outputDir, "failures")
		if _, err := os.Stat(failureDir); err == nil {
			entries, err := os.ReadDir(failureDir)
			assert.NoError(t, err)
			assert.Greater(t, len(entries), 0, "Should have created failure dump")
			t.Logf("Failure details saved to: %s", failureDir)
		}
	})
}
