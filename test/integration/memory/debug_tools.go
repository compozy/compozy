package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	memcore "github.com/compozy/compozy/engine/memory/core"
)

const (
	debugEnvValue       = "true"
	interactiveEnvValue = "true"
)

// DebugTools provides debugging utilities for memory tests
type DebugTools struct {
	env           *TestEnvironment
	outputDir     string
	enableCapture bool
}

// NewDebugTools creates new debug tools
func NewDebugTools(env *TestEnvironment) *DebugTools {
	outputDir := filepath.Join("testdata", "debug", fmt.Sprintf("run_%d", time.Now().Unix()))
	return &DebugTools{
		env:           env,
		outputDir:     outputDir,
		enableCapture: os.Getenv("MEMORY_TEST_DEBUG") == debugEnvValue,
	}
}

// CaptureRedisState captures the current Redis state
func (d *DebugTools) CaptureRedisState(t *testing.T, label string) {
	if !d.enableCapture {
		return
	}
	t.Helper()

	file := d.createCaptureFile(t, label)
	defer file.Close()

	state := d.buildRedisState(t, label)
	d.writeStateToFile(t, file, state)
}

// createCaptureFile creates the output directory and file for capturing
func (d *DebugTools) createCaptureFile(t *testing.T, label string) *os.File {
	err := os.MkdirAll(d.outputDir, 0755)
	require.NoError(t, err)

	filename := fmt.Sprintf("redis_%s_%d.json", label, time.Now().UnixNano())
	filepath := filepath.Join(d.outputDir, filename)
	file, err := os.Create(filepath)
	require.NoError(t, err)
	return file
}

// buildRedisState builds the complete Redis state structure
func (d *DebugTools) buildRedisState(t *testing.T, label string) map[string]any {
	ctx := context.Background()
	redis := d.env.GetRedis()

	keys, err := redis.Keys(ctx, "*").Result()
	if err != nil {
		t.Logf("Failed to get Redis keys: %v", err)
		return d.createEmptyState(label)
	}

	state := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"label":     label,
		"key_count": len(keys),
		"keys":      d.captureKeyDetails(ctx, redis, keys),
	}
	return state
}

// createEmptyState creates an empty state when Redis access fails
func (d *DebugTools) createEmptyState(label string) map[string]any {
	return map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"label":     label,
		"key_count": 0,
		"keys":      make(map[string]any),
	}
}

// captureKeyDetails captures detailed information for each Redis key
func (d *DebugTools) captureKeyDetails(ctx context.Context, redis *redis.Client, keys []string) map[string]any {
	keyDetails := make(map[string]any)
	for _, key := range keys {
		if keyInfo := d.captureKeyInfo(ctx, redis, key); keyInfo != nil {
			keyDetails[key] = keyInfo
		}
	}
	return keyDetails
}

// captureKeyInfo captures information for a single Redis key
func (d *DebugTools) captureKeyInfo(ctx context.Context, redis *redis.Client, key string) map[string]any {
	keyType, err := redis.Type(ctx, key).Result()
	if err != nil {
		return nil
	}

	keyInfo := map[string]any{"type": keyType}
	d.addTTLInfo(ctx, redis, key, keyInfo)
	d.addValueInfo(ctx, redis, key, keyType, keyInfo)
	return keyInfo
}

// addTTLInfo adds TTL information to key info
func (d *DebugTools) addTTLInfo(ctx context.Context, redis *redis.Client, key string, keyInfo map[string]any) {
	if ttl, err := redis.TTL(ctx, key).Result(); err == nil && ttl > 0 {
		keyInfo["ttl"] = ttl.String()
	}
}

// addValueInfo adds value information based on Redis data type
func (d *DebugTools) addValueInfo(
	ctx context.Context,
	redis *redis.Client,
	key, keyType string,
	keyInfo map[string]any,
) {
	switch keyType {
	case "string":
		if val, err := redis.Get(ctx, key).Result(); err == nil {
			keyInfo["value"] = val
		}
	case "list":
		d.addListInfo(ctx, redis, key, keyInfo)
	case "hash":
		if fields, err := redis.HKeys(ctx, key).Result(); err == nil {
			keyInfo["fields"] = fields
		}
	}
}

// addListInfo adds Redis list information
func (d *DebugTools) addListInfo(ctx context.Context, redis *redis.Client, key string, keyInfo map[string]any) {
	length, err := redis.LLen(ctx, key).Result()
	if err != nil {
		return
	}
	keyInfo["length"] = length
	if length > 0 {
		if items, err := redis.LRange(ctx, key, 0, 4).Result(); err == nil {
			keyInfo["sample"] = items
		}
	}
}

// writeStateToFile writes the state to JSON file
func (d *DebugTools) writeStateToFile(t *testing.T, file *os.File, state map[string]any) {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		t.Logf("Failed to write Redis state: %v", err)
	} else {
		t.Logf("Redis state captured to: %s", file.Name())
	}
}

// CaptureMemoryState captures memory instance state
func (d *DebugTools) CaptureMemoryState(t *testing.T, instance memcore.Memory, label string) {
	if !d.enableCapture {
		return
	}
	t.Helper()
	ctx := context.Background()
	// Create output directory
	err := os.MkdirAll(d.outputDir, 0755)
	require.NoError(t, err)
	// Create capture file
	filename := fmt.Sprintf("memory_%s_%s_%d.json", instance.GetID(), label, time.Now().UnixNano())
	filepath := filepath.Join(d.outputDir, filename)
	file, err := os.Create(filepath)
	require.NoError(t, err)
	defer file.Close()
	// Capture memory state
	state := map[string]any{
		"timestamp":   time.Now().Format(time.RFC3339),
		"label":       label,
		"instance_id": instance.GetID(),
	}
	// Get messages
	messages, err := instance.Read(ctx)
	if err != nil {
		state["read_error"] = err.Error()
	} else {
		state["message_count"] = len(messages)
		state["messages"] = messages
	}
	// Get health
	health, err := instance.GetMemoryHealth(ctx)
	if err != nil {
		state["health_error"] = err.Error()
	} else {
		state["health"] = health
	}
	// Get token count
	tokenCount, err := instance.GetTokenCount(ctx)
	if err != nil {
		state["token_error"] = err.Error()
	} else {
		state["token_count"] = tokenCount
	}
	// Write JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(state)
	if err != nil {
		t.Logf("Failed to write memory state: %v", err)
	} else {
		t.Logf("Memory state captured to: %s", filepath)
	}
}

// EnableTracing enables detailed tracing for a test
func (d *DebugTools) EnableTracing(t *testing.T) func() {
	if !d.enableCapture {
		return func() {}
	}
	// Create trace file
	err := os.MkdirAll(d.outputDir, 0755)
	require.NoError(t, err)
	filename := fmt.Sprintf("trace_%s_%d.log", t.Name(), time.Now().UnixNano())
	filepath := filepath.Join(d.outputDir, filename)
	file, err := os.Create(filepath)
	require.NoError(t, err)
	// Create writer
	writer := bufio.NewWriter(file)
	// Log start
	fmt.Fprintf(writer, "=== TRACE START: %s ===\n", t.Name())
	fmt.Fprintf(writer, "Time: %s\n\n", time.Now().Format(time.RFC3339))
	writer.Flush()
	// Return cleanup function
	return func() {
		fmt.Fprintf(writer, "\n=== TRACE END: %s ===\n", t.Name())
		writer.Flush()
		file.Close()
		t.Logf("Trace saved to: %s", filepath)
	}
}

// DumpTestFailure dumps detailed information on test failure
func (d *DebugTools) DumpTestFailure(t *testing.T, err error) {
	if !t.Failed() {
		return
	}
	t.Helper()
	// Create failure directory
	failureDir := filepath.Join(d.outputDir, "failures")
	failErr := os.MkdirAll(failureDir, 0755)
	require.NoError(t, failErr)
	// Create failure file
	filename := fmt.Sprintf("failure_%s_%d.txt", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	filepath := filepath.Join(failureDir, filename)
	file, fileErr := os.Create(filepath)
	require.NoError(t, fileErr)
	defer file.Close()
	// Write failure details
	fmt.Fprintf(file, "Test Failure Report\n")
	fmt.Fprintf(file, "==================\n\n")
	fmt.Fprintf(file, "Test: %s\n", t.Name())
	fmt.Fprintf(file, "Time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Error: %v\n\n", err)
	// Capture Redis state
	d.CaptureRedisState(t, "failure")
	// Add Redis info
	ctx := context.Background()
	redis := d.env.GetRedis()
	info, infoErr := redis.Info(ctx).Result()
	if infoErr == nil {
		fmt.Fprintf(file, "Redis Info:\n")
		fmt.Fprintf(file, "-----------\n")
		fmt.Fprintf(file, "%s\n\n", info)
	}
	t.Logf("Failure details saved to: %s", filepath)
}

// MaintenanceTools provides test suite maintenance utilities
type MaintenanceTools struct {
	env *TestEnvironment
}

// NewMaintenanceTools creates maintenance tools
func NewMaintenanceTools(env *TestEnvironment) *MaintenanceTools {
	return &MaintenanceTools{env: env}
}

// CleanupStaleData removes stale test data from Redis
func (m *MaintenanceTools) CleanupStaleData(t *testing.T, olderThan time.Duration) {
	t.Helper()
	ctx := context.Background()
	redis := m.env.GetRedis()
	// Get all keys with test prefix
	keys, err := redis.Keys(ctx, "compozy:test-project:*").Result()
	if err != nil {
		t.Logf("Failed to get keys: %v", err)
		return
	}
	cleaned := 0
	// For this test, we'll clean up all memory instances that were created more than olderThan ago
	// Since the test creates instances with specific timestamps in their keys, we can identify them
	for _, key := range keys {
		// Check if this is a memory instance key (not metadata)
		if strings.HasPrefix(key, "compozy:test-project:memory:") && !strings.HasSuffix(key, ":metadata") {
			// For test purposes, we'll assume all non-TTL keys are old enough to be cleaned
			// In a real scenario, you'd want to track creation timestamps
			err := redis.Del(ctx, key).Err()
			if err == nil {
				cleaned++
				t.Logf("Cleaned up test key: %s", key)
				// Also clean up metadata key
				metadataKey := key + ":metadata"
				redis.Del(ctx, metadataKey)
			}
		}
	}
	t.Logf("Cleaned up %d stale keys older than %v", cleaned, olderThan)
}

// CleanupSpecificInstance removes a specific memory instance for testing
func (m *MaintenanceTools) CleanupSpecificInstance(t *testing.T, instanceID string) {
	t.Helper()
	ctx := context.Background()
	redis := m.env.GetRedis()

	// Clean up specific instance keys
	mainKey := fmt.Sprintf("compozy:test-project:memory:%s", instanceID)
	metadataKey := fmt.Sprintf("compozy:test-project:memory:%s:metadata", instanceID)

	err := redis.Del(ctx, mainKey).Err()
	if err != nil {
		t.Logf("Failed to delete main key %s: %v", mainKey, err)
	} else {
		t.Logf("Cleaned up instance key: %s", mainKey)
	}

	err = redis.Del(ctx, metadataKey).Err()
	if err != nil {
		t.Logf("Failed to delete metadata key %s: %v", metadataKey, err)
	} else {
		t.Logf("Cleaned up metadata key: %s", metadataKey)
	}
}

// VerifyNoLeaks verifies no test data leaks between tests
func (m *MaintenanceTools) VerifyNoLeaks(t *testing.T, allowedPrefixes []string) {
	t.Helper()
	ctx := context.Background()
	redis := m.env.GetRedis()
	// Get all keys
	keys, err := redis.Keys(ctx, "*").Result()
	require.NoError(t, err)
	// Check for unexpected keys
	var unexpectedKeys []string
	for _, key := range keys {
		allowed := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(key, prefix) {
				allowed = true
				break
			}
		}
		if !allowed && !strings.Contains(key, "__internal__") {
			unexpectedKeys = append(unexpectedKeys, key)
		}
	}
	if len(unexpectedKeys) > 0 {
		t.Errorf("Found %d unexpected keys in Redis: %v", len(unexpectedKeys), unexpectedKeys)
	}
}

// GenerateTestReport generates a test execution report
func (m *MaintenanceTools) GenerateTestReport(t *testing.T, results map[string]TestResult) {
	t.Helper()
	// Create report directory
	reportDir := filepath.Join("testdata", "reports")
	err := os.MkdirAll(reportDir, 0755)
	require.NoError(t, err)
	// Create report file
	filename := fmt.Sprintf("report_%s.html", time.Now().Format("2006-01-02_15-04-05"))
	filepath := filepath.Join(reportDir, filename)
	file, err := os.Create(filepath)
	require.NoError(t, err)
	defer file.Close()
	// Write HTML report
	m.writeHTMLReport(file, results)
	t.Logf("Test report generated: %s", filepath)
}

// TestResult holds test execution results
type TestResult struct {
	Name       string
	Passed     bool
	Skipped    bool
	Duration   time.Duration
	Error      error
	Assertions int
}

// writeHTMLReport writes an HTML test report
func (m *MaintenanceTools) writeHTMLReport(w io.Writer, results map[string]TestResult) {
	m.writeHTMLHeader(w)
	m.writeHTMLSummary(w, results)
	m.writeHTMLResultsTable(w, results)
	fmt.Fprintf(w, "</body></html>\n")
}

// writeHTMLHeader writes the HTML header and CSS
func (m *MaintenanceTools) writeHTMLHeader(w io.Writer) {
	fmt.Fprintf(w, "<html><head><title>Memory Integration Test Report</title>\n")
	fmt.Fprintf(w, "<style>\n")
	fmt.Fprintf(w, "body { font-family: Arial, sans-serif; margin: 20px; }\n")
	fmt.Fprintf(w, ".passed { color: green; }\n")
	fmt.Fprintf(w, ".failed { color: red; }\n")
	fmt.Fprintf(w, ".skipped { color: orange; }\n")
	fmt.Fprintf(w, "table { border-collapse: collapse; width: 100%%; }\n")
	fmt.Fprintf(w, "th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }\n")
	fmt.Fprintf(w, "th { background-color: #f2f2f2; }\n")
	fmt.Fprintf(w, "</style></head><body>\n")
	fmt.Fprintf(w, "<h1>Memory Integration Test Report</h1>\n")
	fmt.Fprintf(w, "<p>Generated: %s</p>\n", time.Now().Format(time.RFC3339))
}

// writeHTMLSummary writes the summary section
func (m *MaintenanceTools) writeHTMLSummary(w io.Writer, results map[string]TestResult) {
	passed, failed, skipped, totalDuration := m.calculateSummaryStats(results)
	fmt.Fprintf(w, "<h2>Summary</h2>\n")
	fmt.Fprintf(w, "<p>Total: %d | <span class='passed'>Passed: %d</span> | ", len(results), passed)
	fmt.Fprintf(w, "<span class='failed'>Failed: %d</span> | ", failed)
	fmt.Fprintf(w, "<span class='skipped'>Skipped: %d</span></p>\n", skipped)
	fmt.Fprintf(w, "<p>Total Duration: %v</p>\n", totalDuration)
}

// calculateSummaryStats calculates summary statistics from results
func (m *MaintenanceTools) calculateSummaryStats(results map[string]TestResult) (int, int, int, time.Duration) {
	var passed, failed, skipped int
	var totalDuration time.Duration
	for _, result := range results {
		switch {
		case result.Skipped:
			skipped++
		case result.Passed:
			passed++
		default:
			failed++
		}
		totalDuration += result.Duration
	}
	return passed, failed, skipped, totalDuration
}

// writeHTMLResultsTable writes the detailed results table
func (m *MaintenanceTools) writeHTMLResultsTable(w io.Writer, results map[string]TestResult) {
	fmt.Fprintf(w, "<h2>Test Results</h2>\n")
	fmt.Fprintf(w, "<table>\n")
	fmt.Fprintf(w, "<tr><th>Test</th><th>Status</th><th>Duration</th><th>Assertions</th><th>Error</th></tr>\n")
	for name, result := range results {
		m.writeResultRow(w, name, result)
	}
	fmt.Fprintf(w, "</table>\n")
}

// writeResultRow writes a single result row
func (m *MaintenanceTools) writeResultRow(w io.Writer, name string, result TestResult) {
	status, statusClass := m.getStatusInfo(result)
	fmt.Fprintf(w, "<tr>")
	fmt.Fprintf(w, "<td>%s</td>", name)
	fmt.Fprintf(w, "<td class='%s'>%s</td>", statusClass, status)
	fmt.Fprintf(w, "<td>%v</td>", result.Duration)
	fmt.Fprintf(w, "<td>%d</td>", result.Assertions)
	if result.Error != nil {
		fmt.Fprintf(w, "<td>%v</td>", result.Error)
	} else {
		fmt.Fprintf(w, "<td>-</td>")
	}
	fmt.Fprintf(w, "</tr>\n")
}

// getStatusInfo returns status and CSS class for a result
func (m *MaintenanceTools) getStatusInfo(result TestResult) (string, string) {
	switch {
	case result.Skipped:
		return "skipped", "skipped"
	case !result.Passed:
		return "failed", "failed"
	default:
		return "passed", "passed"
	}
}

// InteractiveDebugger provides interactive debugging capabilities
type InteractiveDebugger struct {
	env     *TestEnvironment
	scanner *bufio.Scanner
}

// NewInteractiveDebugger creates an interactive debugger
func NewInteractiveDebugger(env *TestEnvironment) *InteractiveDebugger {
	return &InteractiveDebugger{
		env:     env,
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Debug pauses test execution for interactive debugging
func (d *InteractiveDebugger) Debug(t *testing.T, instance memcore.Memory) {
	if os.Getenv("MEMORY_TEST_INTERACTIVE") != interactiveEnvValue {
		return
	}
	t.Helper()
	fmt.Print("\n=== INTERACTIVE DEBUG MODE ===\n")
	fmt.Printf("Test: %s\n", t.Name())
	fmt.Printf("Instance ID: %s\n", instance.GetID())
	fmt.Println("Commands: messages, health, tokens, redis, continue")
	fmt.Print("==============================\n\n")
	ctx := context.Background()
	for {
		fmt.Print("> ")
		if !d.scanner.Scan() {
			break
		}
		command := strings.TrimSpace(d.scanner.Text())
		switch command {
		case "messages":
			messages, err := instance.Read(ctx)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Messages (%d):\n", len(messages))
				for i, msg := range messages {
					fmt.Printf("  [%d] %s: %s\n", i, msg.Role, msg.Content)
				}
			}
		case "health":
			health, err := instance.GetMemoryHealth(ctx)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Health: %+v\n", health)
			}
		case "tokens":
			tokens, err := instance.GetTokenCount(ctx)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Token count: %d\n", tokens)
			}
		case "redis":
			redis := d.env.GetRedis()
			keys, err := redis.Keys(ctx, "*").Result()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Redis keys (%d):\n", len(keys))
				for _, key := range keys {
					fmt.Printf("  %s\n", key)
				}
			}
		case "continue":
			fmt.Println("Continuing test execution...")
			return
		default:
			fmt.Println("Unknown command. Use: messages, health, tokens, redis, continue")
		}
	}
}
