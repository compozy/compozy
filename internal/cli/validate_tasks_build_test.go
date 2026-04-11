package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildValidateTasksBinaryDisablesVCSStamping(t *testing.T) {
	t.Parallel()

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
}
