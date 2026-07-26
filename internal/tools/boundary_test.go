package tools

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPackageBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should not import forbidden runtime domains", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "list", "-f", "{{range .Imports}}{{println .}}{{end}}", ".")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list internal/tools imports error = %v", err)
		}

		forbidden := []string{
			"/internal/daemon",
			"/internal/api/",
			"/internal/cli",
			"/internal/extension",
			"/internal/mcp",
			"/internal/session",
			"/internal/task",
			"/internal/skills",
			"/internal/network",
		}
		imports := strings.SplitSeq(strings.TrimSpace(string(out)), "\n")
		for importPath := range imports {
			for _, forbiddenPath := range forbidden {
				if strings.Contains(importPath, forbiddenPath) {
					t.Fatalf("internal/tools imports forbidden package %s", importPath)
				}
			}
		}
	})
}
