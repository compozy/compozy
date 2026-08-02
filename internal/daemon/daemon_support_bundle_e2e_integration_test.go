//go:build integration && !windows

package daemon

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/support"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

type supportBundleE2EResult struct {
	Operation compozycontract.SupportBundleOperationPayload `json:"operation"`
	Path      string                                        `json:"path"`
}

func TestDaemonE2ESupportBundleCompletesThroughCLI(t *testing.T) {
	t.Run("Should create poll and download a support bundle", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})
		outputPath := filepath.Join(t.TempDir(), "support-bundle.tar.gz")
		stdout, stderr, err := harness.CLI.RunInDir(
			ctx,
			harness.WorkspaceRoot,
			"support",
			"bundle",
			"--yes",
			"--output",
			outputPath,
			"--json",
		)
		if err != nil {
			t.Fatalf("support bundle CLI error = %v; stdout=%s stderr=%s", err, stdout, stderr)
		}
		if stderr != "" {
			t.Fatalf("support bundle CLI stderr = %q, want empty", stderr)
		}

		var result supportBundleE2EResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(support bundle result) error = %v; stdout=%s", err, stdout)
		}
		if result.Operation.Status != string(support.OperationCompleted) {
			t.Fatalf(
				"support bundle status = %q, want %q",
				result.Operation.Status,
				support.OperationCompleted,
			)
		}
		if result.Path != outputPath {
			t.Fatalf("support bundle path = %q, want %q", result.Path, outputPath)
		}
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("os.Stat(%q) error = %v", outputPath, err)
		}
		if info.Size() <= 0 || info.Size() != result.Operation.SizeBytes {
			t.Fatalf(
				"downloaded support bundle size = %d, operation size = %d, want matching positive values",
				info.Size(),
				result.Operation.SizeBytes,
			)
		}

		manifest := readSupportBundleE2EManifest(t, outputPath)
		if manifest.OperationID != result.Operation.OperationID {
			t.Fatalf(
				"support bundle manifest operation id = %q, want %q",
				manifest.OperationID,
				result.Operation.OperationID,
			)
		}
	})
}

func readSupportBundleE2EManifest(t *testing.T, path string) support.Manifest {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close support bundle %q error = %v", path, err)
		}
	}()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip.NewReader(%q) error = %v", path, err)
	}
	defer func() {
		if err := gzipReader.Close(); err != nil {
			t.Errorf("close support bundle gzip reader error = %v", err)
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read support bundle tar entry error = %v", err)
		}
		if header.Name != "manifest.json" {
			continue
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read support bundle manifest error = %v", err)
		}
		var manifest support.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("json.Unmarshal(support bundle manifest) error = %v", err)
		}
		return manifest
	}
	t.Fatalf("support bundle %q is missing manifest.json", path)
	return support.Manifest{}
}
