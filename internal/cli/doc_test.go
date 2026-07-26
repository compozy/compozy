package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNewDocCommand_Hidden(t *testing.T) {
	t.Parallel()

	cmd := newDocCommand()
	if !cmd.Hidden {
		t.Error("doc command should be hidden")
	}
}

func TestNewDocCommand_NotInHelp(t *testing.T) {
	t.Parallel()

	root := newRootCommand(commandDeps{})
	help := root.UsageString()
	if regexp.MustCompile(`(?m)^\s*doc\s+`).MatchString(help) {
		t.Error("doc command should not appear in root help output")
	}
}

func TestNewDocCommand_DefaultOutputDir(t *testing.T) {
	t.Parallel()

	cmd := newDocCommand()
	flag := cmd.Flags().Lookup("output-dir")
	if flag == nil {
		t.Fatal("doc command should have --output-dir flag")
		return
	}
	if flag.DefValue != defaultCLIDocsDir {
		t.Errorf("default output-dir = %q, want %q", flag.DefValue, defaultCLIDocsDir)
	}
}

func TestNewDocCommand_RejectsUnexpectedArgs(t *testing.T) {
	t.Parallel()

	cmd := newDocCommand()

	err := cmd.ValidateArgs([]string{"unexpected"})
	if err == nil {
		t.Fatal("doc command should reject unexpected positional args")
	}
	if !strings.Contains(err.Error(), `unknown command "unexpected" for "doc"`) {
		t.Fatalf("unexpected error for extra args: %v", err)
	}
}

// findMDX walks outputDir and returns a set of relative mdx paths (using /).
func findMDX(t *testing.T, outputDir string) map[string]bool {
	t.Helper()
	result := map[string]bool{}
	err := filepath.WalkDir(outputDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".mdx") {
			return nil
		}
		rel, err := filepath.Rel(outputDir, p)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk output dir: %v", err)
	}
	return result
}

func TestNewDocCommand_GeneratesDocs(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "cli-docs")

	root := newRootCommand(commandDeps{})
	root.SetArgs([]string{"doc", "--output-dir", outputDir})

	if err := root.Execute(); err != nil {
		t.Fatalf("doc command failed: %v", err)
	}

	// Verify output directory was created and contains mdx files.
	mdxFiles := findMDX(t, outputDir)
	if len(mdxFiles) == 0 {
		t.Fatal("doc command should generate .mdx files")
	}

	// Verify agh.mdx exists at the root (from agh.md). Root index.mdx is
	// hand-authored by the site and preserved by the post-processor.
	rootMDX := filepath.Join(outputDir, "agh.mdx")
	data, err := os.ReadFile(rootMDX)
	if err != nil {
		t.Fatalf("agh.mdx should exist at root: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "---") {
		t.Error("agh.mdx should have YAML frontmatter")
	}
	if !strings.Contains(content, `title: "agh"`) {
		t.Error("agh.mdx frontmatter should have title 'agh'")
	}

	// Per-subdirectory meta.json is auto-generated; walk and ensure at least
	// one such file exists (root meta.json is hand-maintained and NOT written).
	var subdirMetas int
	err = filepath.WalkDir(outputDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "meta.json" {
			return nil
		}
		rel, relErr := filepath.Rel(outputDir, p)
		if relErr != nil {
			return relErr
		}
		if filepath.Dir(rel) == "." {
			t.Errorf("doc command must not write root meta.json (hand-maintained)")
			return nil
		}
		subdirMetas++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if subdirMetas == 0 {
		t.Error("doc command should generate at least one subdirectory meta.json")
	}

	// Verify no absolute paths in any generated file.
	err = filepath.WalkDir(outputDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		fileData, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		fileContent := string(fileData)
		if strings.Contains(fileContent, "/Users/") || strings.Contains(fileContent, "/home/") {
			rel, relErr := filepath.Rel(outputDir, p)
			if relErr != nil {
				return relErr
			}
			t.Errorf("file %s contains absolute paths", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

func TestNewDocCommand_CreatesOutputDir(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "nested", "deep", "output")

	root := newRootCommand(commandDeps{})
	root.SetArgs([]string{"doc", "--output-dir", outputDir})

	if err := root.Execute(); err != nil {
		t.Fatalf("doc command should create output dir: %v", err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("output directory should have been created")
	}
}

func TestNewDocCommand_GeneratesAllCommands(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "cli-docs")

	root := newRootCommand(commandDeps{})
	root.SetArgs([]string{"doc", "--output-dir", outputDir})

	if err := root.Execute(); err != nil {
		t.Fatalf("doc command failed: %v", err)
	}

	mdxFiles := findMDX(t, outputDir)

	// Expected files in the nested layout: parents with children render as
	// <segment>/index.mdx; leaves render as <segment>.mdx.
	expected := []string{
		"agh.mdx",            // from agh
		"session/index.mdx",  // parent with children
		"daemon/index.mdx",   // parent with children
		"tool/index.mdx",     // parent with children
		"tool/list.mdx",      // tool registry operator command
		"tool/search.mdx",    // tool registry operator command
		"tool/info.mdx",      // tool registry operator command
		"tool/invoke.mdx",    // tool registry operator command
		"toolsets/index.mdx", // parent with children
		"toolsets/list.mdx",  // toolset operator command
		"toolsets/info.mdx",  // toolset operator command
		"version.mdx",        // leaf without children
	}
	for _, want := range expected {
		if !mdxFiles[want] {
			t.Errorf("expected file %q to exist, got files: %v", want, sortedKeys(mdxFiles))
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
