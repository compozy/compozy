package orchestrator

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Timeout constants for different operations
var (
	// DefaultWorkflowTimeout is the standard timeout for PR and dry-run workflows
	DefaultWorkflowTimeout = getTimeoutOrDefault("WORKFLOW_TIMEOUT", 60*time.Minute, 5*time.Second)
	// ReleaseWorkflowTimeout is the extended timeout for release operations
	ReleaseWorkflowTimeout = getTimeoutOrDefault("RELEASE_WORKFLOW_TIMEOUT", 120*time.Minute, 10*time.Second)
	// RollbackTimeout is the timeout for rollback operations
	RollbackTimeout = getTimeoutOrDefault("ROLLBACK_TIMEOUT", 10*time.Minute, 100*time.Millisecond)
	// DefaultRetryCount is the standard number of retries for operations
	DefaultRetryCount = uint64(getRetryCountOrDefault("RETRY_COUNT", 3, 1))
	// DefaultRetryDelay is the initial delay for exponential backoff
	DefaultRetryDelay = getTimeoutOrDefault("RETRY_DELAY", 1*time.Second, 100*time.Millisecond)
)

// isTestEnvironment detects if we're running in a test environment
func isTestEnvironment() bool {
	// Check for testing flags
	for _, arg := range os.Args {
		if strings.Contains(arg, ".test") || strings.Contains(arg, "go test") {
			return true
		}
	}
	// Check for test environment variables
	return os.Getenv("GO_TEST") == "true" || os.Getenv("TEST_MODE") == "true"
}

// getTimeoutOrDefault returns production timeout or test timeout based on environment
func getTimeoutOrDefault(envVar string, prodDefault, testDefault time.Duration) time.Duration {
	if env := os.Getenv(envVar); env != "" {
		if duration, err := time.ParseDuration(env); err == nil {
			return duration
		}
	}
	if isTestEnvironment() {
		return testDefault
	}
	return prodDefault
}

// getRetryCountOrDefault returns production retry count or test retry count based on environment
func getRetryCountOrDefault(envVar string, prodDefault, testDefault int) int {
	if env := os.Getenv(envVar); env != "" {
		if count, err := strconv.Atoi(env); err == nil {
			return count
		}
	}
	if isTestEnvironment() {
		return testDefault
	}
	return prodDefault
}

// File permission constants
const (
	// FilePermissionsReadWrite is the standard permission for created files
	FilePermissionsReadWrite = 0644
	// FilePermissionsSecure is the secure permission for sensitive files
	FilePermissionsSecure = 0600
	// DirPermissionsDefault is the standard permission for created directories
	DirPermissionsDefault = 0755
)
