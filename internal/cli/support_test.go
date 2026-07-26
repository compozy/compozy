package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSupportBundleCommand(t *testing.T) {
	t.Parallel()

	t.Run("ShouldCreatePollAndDownloadThroughDaemon", func(t *testing.T) {
		t.Parallel()

		completedAt := fixedTestNow.Add(2 * time.Second)
		outputDir := t.TempDir()
		var createCalled bool
		var getCalled bool
		var downloadCalled bool
		client := &stubClient{
			createSupportBundleFn: func(_ context.Context, request CreateSupportBundleRequest) (SupportBundleOperationRecord, error) {
				createCalled = true
				if !request.Yes {
					t.Fatalf("CreateSupportBundle() Yes = false, want true")
				}
				if request.IncludeStatus == nil || !*request.IncludeStatus {
					t.Fatalf("CreateSupportBundle() IncludeStatus = %v, want true", request.IncludeStatus)
				}
				return SupportBundleOperationRecord{
					OperationID: "op_123",
					Status:      "pending",
					CreatedAt:   fixedTestNow,
					UpdatedAt:   fixedTestNow,
				}, nil
			},
			getSupportBundleFn: func(_ context.Context, operationID string) (SupportBundleOperationRecord, error) {
				getCalled = true
				if operationID != "op_123" {
					t.Fatalf("GetSupportBundle() operationID = %q, want op_123", operationID)
				}
				return SupportBundleOperationRecord{
					OperationID: "op_123",
					Status:      "completed",
					FileName:    "agh-support-bundle-20260520T120000Z.tar.gz",
					SizeBytes:   12,
					CreatedAt:   fixedTestNow,
					UpdatedAt:   completedAt,
					CompletedAt: &completedAt,
				}, nil
			},
			downloadSupportBundleFn: func(_ context.Context, operationID string, dst io.Writer) error {
				downloadCalled = true
				if operationID != "op_123" {
					t.Fatalf("DownloadSupportBundle() operationID = %q, want op_123", operationID)
				}
				if _, err := io.WriteString(dst, "bundle-bytes"); err != nil {
					return err
				}
				return nil
			},
		}
		deps := newTestDeps(t, client)
		deps.pollInterval = time.Millisecond

		stdout, stderr, err := executeRootCommand(
			t,
			deps,
			"support",
			"bundle",
			"--yes",
			"--output",
			outputDir,
			"--json",
		)
		if err != nil {
			t.Fatalf("support bundle error = %v stderr=%s", err, stderr)
		}
		if stderr != "" {
			t.Fatalf("support bundle stderr = %q, want empty", stderr)
		}
		if !createCalled || !getCalled || !downloadCalled {
			t.Fatalf(
				"daemon calls create=%t get=%t download=%t, want all true",
				createCalled,
				getCalled,
				downloadCalled,
			)
		}
		var result supportBundleResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%s", err, stdout)
		}
		wantPath := filepath.Join(outputDir, "agh-support-bundle-20260520T120000Z.tar.gz")
		if result.Path != wantPath {
			t.Fatalf("result.Path = %q, want %q", result.Path, wantPath)
		}
		data, err := os.ReadFile(wantPath)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", wantPath, err)
		}
		if string(data) != "bundle-bytes" {
			t.Fatalf("downloaded bundle = %q, want bundle-bytes", string(data))
		}
	})

	t.Run("ShouldRequireExplicitYesForStructuredOutput", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(t, deps, "support", "bundle", "--json")
		if err == nil {
			t.Fatal("support bundle error = nil, want --yes failure")
		}
		if !strings.Contains(err.Error(), "requires --yes") {
			t.Fatalf("support bundle error = %v, want --yes context", err)
		}
	})

	t.Run("Should require explicit yes when stdin is not interactive", func(t *testing.T) {
		t.Parallel()

		createCalled := false
		deps := newTestDeps(t, &stubClient{
			createSupportBundleFn: func(context.Context, CreateSupportBundleRequest) (SupportBundleOperationRecord, error) {
				createCalled = true
				return SupportBundleOperationRecord{}, nil
			},
		})
		cmd := newRootCommand(deps)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetIn(strings.NewReader("y\n"))
		cmd.SetArgs([]string{"support", "bundle"})

		err := cmd.ExecuteContext(t.Context())
		if err == nil {
			t.Fatal("support bundle error = nil, want non-interactive --yes failure")
		}
		if !strings.Contains(err.Error(), "requires --yes when stdin is not interactive") {
			t.Fatalf("support bundle error = %v, want non-interactive --yes context", err)
		}
		if createCalled {
			t.Fatal("CreateSupportBundle() called despite missing non-interactive consent")
		}
	})

	t.Run("ShouldPassNoStatusRequestToDaemon", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			createSupportBundleFn: func(_ context.Context, request CreateSupportBundleRequest) (SupportBundleOperationRecord, error) {
				if !request.Yes {
					t.Fatalf("CreateSupportBundle() Yes = false, want true")
				}
				if request.IncludeStatus == nil || *request.IncludeStatus {
					t.Fatalf("CreateSupportBundle() IncludeStatus = %v, want false", request.IncludeStatus)
				}
				return SupportBundleOperationRecord{
					OperationID: "op_456",
					Status:      "pending",
					CreatedAt:   fixedTestNow,
					UpdatedAt:   fixedTestNow,
				}, nil
			},
			getSupportBundleFn: func(context.Context, string) (SupportBundleOperationRecord, error) {
				return SupportBundleOperationRecord{
					OperationID: "op_456",
					Status:      "completed",
					FileName:    "bundle.tar.gz",
					CreatedAt:   fixedTestNow,
					UpdatedAt:   fixedTestNow,
				}, nil
			},
			downloadSupportBundleFn: func(_ context.Context, _ string, dst io.Writer) error {
				if _, err := io.WriteString(dst, "bundle"); err != nil {
					return err
				}
				return nil
			},
		}
		deps := newTestDeps(t, client)
		deps.pollInterval = time.Millisecond
		stdout, stderr, err := executeRootCommand(
			t,
			deps,
			"support",
			"bundle",
			"--yes",
			"--no-status",
			"--output",
			t.TempDir(),
			"--json",
		)
		if err != nil {
			t.Fatalf("support bundle --no-status error = %v stderr=%s", err, stderr)
		}
		if !strings.Contains(stdout, "op_456") {
			t.Fatalf("support bundle stdout = %s, want op_456", stdout)
		}
	})
}
