package docpost

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessInputValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject ambiguous agh-prefixed source filenames", func(t *testing.T) {
		t.Parallel()

		srcDir := t.TempDir()
		dstDir := filepath.Join(t.TempDir(), "output")
		sourcePath := filepath.Join(srcDir, "aghost.md")
		if err := os.WriteFile(sourcePath, []byte("## aghost\n\nBad input\n"), 0o600); err != nil {
			t.Fatalf("write ambiguous source file: %v", err)
		}

		err := Process(context.Background(), srcDir, dstDir)
		if err == nil || !strings.Contains(err.Error(), "must be 'agh.md' or start with 'agh_'") {
			t.Fatalf("Process() error = %v, want ambiguous source filename rejection", err)
		}
		if _, statErr := os.Stat(filepath.Join(dstDir, "agh.mdx")); !os.IsNotExist(statErr) {
			t.Fatalf("Process() wrote root output for rejected filename, stat err = %v", statErr)
		}
	})

	t.Run("Should preserve existing generated output when source validation fails", func(t *testing.T) {
		t.Parallel()

		srcDir := t.TempDir()
		dstDir := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(srcDir, "aghost.md"),
			[]byte("## aghost\n\nBad input\n"),
			0o600,
		); err != nil {
			t.Fatalf("write ambiguous source file: %v", err)
		}
		seeded := map[string]string{
			"index.mdx":       "editorial index",
			"meta.json":       "{\"title\":\"CLI Reference\"}",
			"agh.mdx":         "previous root",
			"agent/index.mdx": "previous agent index",
			"agent/list.mdx":  "previous agent list",
			"agent/meta.json": "{\"title\":\"Agent\"}",
		}
		for rel, body := range seeded {
			path := filepath.Join(dstDir, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", rel, err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write %s: %v", rel, err)
			}
		}

		err := Process(context.Background(), srcDir, dstDir)
		if err == nil || !strings.Contains(err.Error(), "must be 'agh.md' or start with 'agh_'") {
			t.Fatalf("Process() error = %v, want ambiguous source filename rejection", err)
		}
		for rel, want := range seeded {
			got, readErr := os.ReadFile(filepath.Join(dstDir, rel))
			if readErr != nil {
				t.Fatalf("seeded output %s should remain after rejected input: %v", rel, readErr)
			}
			if string(got) != want {
				t.Fatalf("seeded output %s = %q, want %q", rel, string(got), want)
			}
		}
	})

	t.Run("Should reject invalid empty command segments", func(t *testing.T) {
		t.Parallel()

		_, err := commandSegments("agh__list.md", "agh__list")
		if err == nil || !strings.Contains(err.Error(), "invalid command segment") {
			t.Fatalf("commandSegments() error = %v, want invalid segment rejection", err)
		}
	})

	t.Run("Should reject duplicate planned output paths", func(t *testing.T) {
		t.Parallel()

		inputs := []input{
			{fileName: "agh.md", baseName: "agh"},
			{fileName: "aghost.md", baseName: "aghost"},
		}

		err := validateOutputPaths(inputs, map[string]bool{})
		if err == nil || !strings.Contains(err.Error(), `output path collision "agh.mdx"`) {
			t.Fatalf("validateOutputPaths() error = %v, want output path collision", err)
		}
	})
}

func TestLinkRewriteCodeRegionHandling(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve command links inside code regions", func(t *testing.T) {
		t.Parallel()

		raw := strings.Join([]string{
			"See [agh agent](agh_agent.md).",
			"Inline `[agh agent](agh_agent.md)` stays literal.",
			"```",
			"[agh task](agh_task.md)",
			"```",
		}, "\n")

		got := rewriteLinks(raw)
		want := strings.Join([]string{
			"See [agh agent](agh_agent).",
			"Inline `[agh agent](agh_agent.md)` stays literal.",
			"```",
			"[agh task](agh_task.md)",
			"```",
		}, "\n")
		if got != want {
			t.Fatalf("rewriteLinks() = %q, want %q", got, want)
		}
	})

	t.Run("Should remap only non-code command links", func(t *testing.T) {
		t.Parallel()

		targets := map[string]string{
			"agh_agent": "/runtime/cli-reference/agent",
			"agh_task":  "/runtime/cli-reference/task",
		}
		raw := strings.Join([]string{
			"See [agh agent](agh_agent).",
			"Inline `[agh agent](agh_agent)` stays literal.",
			"```",
			"[agh task](agh_task)",
			"```",
		}, "\n")

		got := remapLinks(raw, targets)
		want := strings.Join([]string{
			"See [agh agent](/runtime/cli-reference/agent).",
			"Inline `[agh agent](agh_agent)` stays literal.",
			"```",
			"[agh task](agh_task)",
			"```",
		}, "\n")
		if got != want {
			t.Fatalf("remapLinks() = %q, want %q", got, want)
		}
	})
}
