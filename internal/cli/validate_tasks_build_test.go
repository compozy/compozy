package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildValidateTasksBinaryDisablesVCSStamping(t *testing.T) {
	t.Parallel()

	t.Run("Should build binary without embedded VCS metadata", func(t *testing.T) {
		repoRoot, err := validateTasksRepoRoot()
		if err != nil {
			t.Fatalf("resolve repo root: %v", err)
		}

		outputPath := filepath.Join(t.TempDir(), "compozy")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		output, err := buildValidateTasksBinary(ctx, repoRoot, outputPath)
		if err != nil {
			t.Fatalf("build validate-tasks binary: %v\n%s", err, output)
		}

		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("stat built binary: %v", err)
		}
		if info.IsDir() {
			t.Fatalf("expected binary file, got directory at %s", outputPath)
		}

		meta, err := exec.CommandContext(ctx, "go", "version", "-m", outputPath).CombinedOutput()
		if err != nil {
			t.Fatalf("inspect binary build metadata: %v\n%s", err, meta)
		}
		metaStr := string(meta)
		if strings.Contains(metaStr, "vcs.revision=") ||
			strings.Contains(metaStr, "vcs.time=") ||
			strings.Contains(metaStr, "vcs.modified=") {
			t.Fatalf("expected no VCS stamping in binary metadata, got:\n%s", metaStr)
		}
	})
}
