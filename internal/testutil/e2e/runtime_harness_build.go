package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"maps"

	"net/url"
	"os"

	"path/filepath"
	"runtime"

	"strings"

	"testing"
	"time"

	"github.com/compozy/agh/internal/store"

	"github.com/compozy/agh/internal/testutil/acpmock"

	"golang.org/x/sys/execabs"
)

func runtimeRunDirectories(root string) ([]string, error) {
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(trimmedRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read runtime run root %q: %w", trimmedRoot, err)
	}

	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directories = append(directories, filepath.Join(trimmedRoot, entry.Name()))
	}
	sortStrings(directories)
	return directories, nil
}

func readNetworkAuditSnapshot(path string) (_ []store.NetworkAuditEntry, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open network audit file %q: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close network audit file %q: %w", path, closeErr))
		}
	}()

	entries := make([]store.NetworkAuditEntry, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var entry store.NetworkAuditEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("decode network audit line: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan network audit file %q: %w", path, err)
	}
	return entries, nil
}

func buildAGHBinary(t testing.TB) string {
	t.Helper()

	repoRoot := mustRepoRoot(t)
	if override := strings.TrimSpace(os.Getenv(daemonBinaryEnvVar)); override != "" {
		if filepath.IsAbs(override) {
			return override
		}
		return filepath.Clean(filepath.Join(repoRoot, override))
	}

	buildBinaryMu.Lock()
	defer buildBinaryMu.Unlock()

	if builtBinaryPath != "" {
		if _, err := os.Stat(builtBinaryPath); err == nil {
			return builtBinaryPath
		}
	}

	binaryPath := filepath.Join(os.TempDir(), fmt.Sprintf("agh-e2e-%d", os.Getpid()))
	// #nosec G204 -- test harness builds the local agh binary from the checked-out repository.
	cmd := execabs.CommandContext(context.Background(), "go", "build", "-o", binaryPath, "./cmd/agh")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/agh error = %v\n%s", err, strings.TrimSpace(string(output)))
	}

	builtBinaryPath = binaryPath
	return builtBinaryPath
}

// BuildAGHBinary builds or reuses the current checkout's agh binary for integration fixtures.
func BuildAGHBinary(t testing.TB) string {
	t.Helper()
	return buildAGHBinary(t)
}

func cloneMockAgentRegistrations(
	in map[string]acpmock.Registration,
) map[string]acpmock.Registration {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]acpmock.Registration, len(in))
	maps.Copy(out, in)
	return out
}

func mustRepoRoot(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("failed to locate repository root from runtime_harness.go")
		}
		dir = parent
	}
}

func ensureLeadingSlash(path string) string {
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func encodeQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

func defaultDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
