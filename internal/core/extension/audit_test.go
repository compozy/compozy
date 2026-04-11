package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestAuditLoggerOpenRejectsMissingDirectory(t *testing.T) {
	t.Parallel()

	var logger AuditLogger
	err := logger.Open(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("Open() error = nil, want failure")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Open() error = %v, want os.ErrNotExist", err)
	}
}

func TestAuditLoggerOpenAlreadyOpenAndPath(t *testing.T) {
	t.Parallel()

	runArtifactsPath := t.TempDir()

	var logger AuditLogger
	if err := logger.Open(runArtifactsPath); err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if got := logger.Path(); got != filepath.Join(runArtifactsPath, AuditLogFileName) {
		t.Fatalf("Path() = %q, want %q", got, filepath.Join(runArtifactsPath, AuditLogFileName))
	}

	err := logger.Open(runArtifactsPath)
	if err == nil {
		t.Fatal("second Open() error = nil, want failure")
	}
	if !errors.Is(err, ErrAuditLoggerOpen) {
		t.Fatalf("second Open() error = %v, want ErrAuditLoggerOpen", err)
	}

	if err := logger.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestAuditLoggerRoundTripsRecordedEntry(t *testing.T) {
	t.Parallel()

	runArtifactsPath := t.TempDir()

	var logger AuditLogger
	if err := logger.Open(runArtifactsPath); err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	recordedAt := time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC)
	entry := AuditEntry{
		Timestamp:   recordedAt,
		Extension:   "example-ext",
		Direction:   AuditDirectionExtToHost,
		Method:      "host.tasks.create",
		Capability:  CapabilityTasksCreate,
		Latency:     12 * time.Millisecond,
		Result:      AuditResultError,
		ErrorDetail: "capability denied",
	}

	if err := logger.Record(entry); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := logger.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	lines := readAuditLines(t, filepath.Join(runArtifactsPath, AuditLogFileName))
	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1", len(lines))
	}

	var decoded auditEntryJSON
	if err := json.Unmarshal(lines[0], &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := auditEntryJSON{
		Timestamp:   recordedAt,
		Extension:   "example-ext",
		Direction:   AuditDirectionExtToHost,
		Method:      "host.tasks.create",
		Capability:  "tasks.create",
		LatencyMS:   12,
		Result:      AuditResultError,
		ErrorDetail: "capability denied",
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded = %#v, want %#v", decoded, want)
	}
}

func TestAuditLoggerCloseFlushesEntriesBeforeReturning(t *testing.T) {
	t.Parallel()

	runArtifactsPath := t.TempDir()

	var logger AuditLogger
	if err := logger.Open(runArtifactsPath); err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := logger.Record(AuditEntry{
		Extension:  "flush-ext",
		Direction:  AuditDirectionHostToExt,
		Method:     "execute_hook",
		Capability: CapabilityPromptMutate,
		Latency:    time.Millisecond,
		Result:     AuditResultOK,
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if err := logger.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	lines := readAuditLines(t, filepath.Join(runArtifactsPath, AuditLogFileName))
	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1", len(lines))
	}
}

func TestAuditLoggerRecordLifecycleErrors(t *testing.T) {
	t.Parallel()

	var logger AuditLogger
	if err := logger.Record(AuditEntry{}); !errors.Is(err, ErrAuditLoggerNotOpen) {
		t.Fatalf("Record() before Open error = %v, want ErrAuditLoggerNotOpen", err)
	}

	runArtifactsPath := t.TempDir()
	if err := logger.Open(runArtifactsPath); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := logger.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := logger.Record(AuditEntry{}); !errors.Is(err, ErrAuditLoggerClosed) {
		t.Fatalf("Record() after Close error = %v, want ErrAuditLoggerClosed", err)
	}
}

func TestAuditLoggerConcurrentWritesProduceValidJSONL(t *testing.T) {
	t.Parallel()

	runArtifactsPath := t.TempDir()

	var logger AuditLogger
	if err := logger.Open(runArtifactsPath); err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	const goroutines = 100

	var wg sync.WaitGroup
	for idx := 0; idx < goroutines; idx++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := logger.Record(AuditEntry{
				Extension:  "concurrent-ext",
				Direction:  AuditDirectionExtToHost,
				Method:     fmt.Sprintf("host.tasks.create.%03d", idx),
				Capability: CapabilityTasksCreate,
				Latency:    time.Duration(idx) * time.Millisecond,
				Result:     AuditResultOK,
			})
			if err != nil {
				t.Errorf("Record() error = %v", err)
			}
		}(idx)
	}
	wg.Wait()

	if err := logger.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	lines := readAuditLines(t, filepath.Join(runArtifactsPath, AuditLogFileName))
	if len(lines) != goroutines {
		t.Fatalf("len(lines) = %d, want %d", len(lines), goroutines)
	}

	seenMethods := make(map[string]struct{}, goroutines)
	for _, line := range lines {
		var decoded auditEntryJSON
		if err := json.Unmarshal(line, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", string(line), err)
		}
		if decoded.Extension != "concurrent-ext" {
			t.Fatalf("Extension = %q, want concurrent-ext", decoded.Extension)
		}
		if _, exists := seenMethods[decoded.Method]; exists {
			t.Fatalf("duplicate method %q", decoded.Method)
		}
		seenMethods[decoded.Method] = struct{}{}
	}
}

func TestAuditLoggerKillLeavesReadablePrefix(t *testing.T) {
	runArtifactsPath := t.TempDir()
	logPath := filepath.Join(runArtifactsPath, AuditLogFileName)

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestAuditLoggerHelperProcess", "--")
	cmd.Env = append(
		os.Environ(),
		"GO_WANT_AUDIT_LOGGER_HELPER_PROCESS=1",
		"GO_AUDIT_HELPER_RUN_ARTIFACTS="+runArtifactsPath,
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitForAuditFileLines(t, logPath, 5)

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("Process.Kill() error = %v", err)
	}
	_ = cmd.Wait()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected readable prefix on disk")
	}

	lines := bytes.Split(content, []byte("\n"))
	parsed := 0
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var decoded auditEntryJSON
		if err := json.Unmarshal(line, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", string(line), err)
		}
		parsed++
	}
	if parsed == 0 {
		t.Fatal("expected at least one parsed audit record")
	}
}

func TestAuditLoggerCloseRespectsContextWhenAlreadyClosing(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	var logger AuditLogger
	logger.closeDone = done

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := logger.Close(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Close() error = %v, want context canceled", err)
	}
}

func TestAuditEntryToJSONValidatesAndNormalizes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		entry      AuditEntry
		wantErrSub string
		assert     func(*testing.T, auditEntryJSON)
	}{
		{
			name:       "missing extension",
			entry:      AuditEntry{Direction: AuditDirectionExtToHost, Method: "host.tasks.create"},
			wantErrSub: "extension is required",
		},
		{
			name:       "missing method",
			entry:      AuditEntry{Extension: "ext", Direction: AuditDirectionExtToHost},
			wantErrSub: "method is required",
		},
		{
			name: "invalid direction",
			entry: AuditEntry{
				Extension: "ext",
				Direction: AuditDirection("sideways"),
				Method:    "host.tasks.create",
			},
			wantErrSub: `unsupported audit direction "sideways"`,
		},
		{
			name: "invalid result",
			entry: AuditEntry{
				Extension: "ext",
				Direction: AuditDirectionExtToHost,
				Method:    "host.tasks.create",
				Result:    AuditResult("broken"),
			},
			wantErrSub: `unsupported audit result "broken"`,
		},
		{
			name: "default result and clamp latency",
			entry: AuditEntry{
				Extension:   " ext ",
				Direction:   AuditDirectionExtToHost,
				Method:      " host.tasks.create ",
				Capability:  CapabilityTasksCreate,
				Latency:     -1 * time.Second,
				ErrorDetail: " failed ",
			},
			assert: func(t *testing.T, payload auditEntryJSON) {
				t.Helper()
				if payload.Extension != "ext" {
					t.Fatalf("Extension = %q, want ext", payload.Extension)
				}
				if payload.Method != "host.tasks.create" {
					t.Fatalf("Method = %q, want host.tasks.create", payload.Method)
				}
				if payload.Result != AuditResultError {
					t.Fatalf("Result = %q, want error", payload.Result)
				}
				if payload.LatencyMS != 0 {
					t.Fatalf("LatencyMS = %d, want 0", payload.LatencyMS)
				}
				if payload.ErrorDetail != "failed" {
					t.Fatalf("ErrorDetail = %q, want failed", payload.ErrorDetail)
				}
				if payload.Timestamp.IsZero() {
					t.Fatal("Timestamp = zero, want current UTC timestamp")
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload, err := tc.entry.toJSON()
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatal("toJSON() error = nil, want failure")
				}
				if err.Error() != tc.wantErrSub {
					t.Fatalf("toJSON() error = %q, want %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("toJSON() error = %v", err)
			}
			if tc.assert != nil {
				tc.assert(t, payload)
			}
		})
	}
}

func TestSyncAndCloseFileNilAndWriteAllError(t *testing.T) {
	t.Parallel()

	if err := syncAndCloseFile(nil); err != nil {
		t.Fatalf("syncAndCloseFile(nil) error = %v, want nil", err)
	}

	path := filepath.Join(t.TempDir(), "readonly")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if err := writeAll(file, []byte("more")); err == nil {
		t.Fatal("writeAll() error = nil, want write failure")
	}
}

func TestAuditLoggerHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_AUDIT_LOGGER_HELPER_PROCESS") != "1" {
		return
	}

	runArtifactsPath := os.Getenv("GO_AUDIT_HELPER_RUN_ARTIFACTS")

	var logger AuditLogger
	if err := logger.Open(runArtifactsPath); err != nil {
		fmt.Fprintf(os.Stderr, "open audit helper: %v\n", err)
		os.Exit(2)
	}

	for idx := 0; ; idx++ {
		if err := logger.Record(AuditEntry{
			Extension:  "helper-ext",
			Direction:  AuditDirectionExtToHost,
			Method:     fmt.Sprintf("host.tasks.create.%06d", idx),
			Capability: CapabilityTasksCreate,
			Latency:    time.Millisecond,
			Result:     AuditResultOK,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "record audit helper: %v\n", err)
			os.Exit(3)
		}
		runtime.Gosched()
	}
}

func readAuditLines(t *testing.T, path string) [][]byte {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	rawLines := bytes.Split(content, []byte("\n"))
	lines := make([][]byte, 0, len(rawLines))
	for _, line := range rawLines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func waitForAuditFileLines(t *testing.T, path string, minimum int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		lines := 0
		content, err := os.ReadFile(path)
		if err == nil {
			lines = bytes.Count(content, []byte("\n"))
		}
		if lines >= minimum {
			return
		}
		<-ticker.C
	}

	t.Fatalf("timed out waiting for %d lines in %q", minimum, path)
}
