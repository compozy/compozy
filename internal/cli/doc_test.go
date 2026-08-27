package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/cli/docpost"
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

func TestCmdPaletteAvailableFlagDocContract(t *testing.T) {
	t.Parallel()

	t.Run("Should publish opt-in --available help without a default-true annotation", func(t *testing.T) {
		t.Parallel()

		cmd := newCmdPaletteListCommand(commandDeps{})
		flag := cmd.Flags().Lookup(cmdPaletteAvailableFlag)
		if flag == nil {
			t.Fatal("list command is missing --available")
		}
		if flag.Usage != cmdPaletteAvailableFlagHelp {
			t.Fatalf("usage = %q, want %q", flag.Usage, cmdPaletteAvailableFlagHelp)
		}
		if flag.DefValue != "false" {
			t.Fatalf("DefValue = %q, want false so cli-docs omit (default true)", flag.DefValue)
		}
		usages := cmd.NonInheritedFlags().FlagUsages()
		if !strings.Contains(usages, cmdPaletteAvailableFlagHelp) {
			t.Fatalf("FlagUsages = %q, want the opt-in help", usages)
		}
		if strings.Contains(usages, "(default true)") {
			t.Fatalf("FlagUsages = %q, must not emit (default true)", usages)
		}
		if !strings.Contains(cmd.Example, "--available=false") {
			t.Fatalf("Example = %q, want an explicit --available=false invocation", cmd.Example)
		}
	})
}

func TestDocOutputProfilesReflectCommandBehavior(t *testing.T) {
	t.Parallel()

	profiles := docOutputProfiles(newRootCommand(commandDeps{}))
	tests := map[string]struct {
		want docpost.OutputProfile
	}{
		"compozy agent":                       {want: docpost.OutputProfileHelp},
		"compozy agent list":                  {want: docpost.OutputProfileResult},
		"compozy approvals":                   {want: docpost.OutputProfileHelp},
		"compozy cmd-palette":                 {want: docpost.OutputProfileHelp},
		"compozy cmd-palette alias":           {want: docpost.OutputProfileHelp},
		"compozy cmd-palette personalization": {want: docpost.OutputProfileHelp},
		"compozy task":                        {want: docpost.OutputProfileHelp},
		"compozy task review": {
			want: docpost.OutputProfileHelp,
		},
		"compozy layout watch": {want: docpost.OutputProfileHumanJSONL},
		"compozy terminal quote": {
			want: docpost.OutputProfileTerminalQuote,
		},
		"compozy completion bash": {
			want: docpost.OutputProfileRaw,
		},
		"compozy mcp serve": {want: docpost.OutputProfileProtocol},
		"compozy open":      {want: docpost.OutputProfileNoOutput},
	}
	for commandPath, tt := range tests {
		t.Run("Should map "+commandPath+" to its output profile", func(t *testing.T) {
			t.Parallel()

			if got := profiles[commandPath]; got != tt.want {
				t.Errorf("docOutputProfiles()[%q] = %q, want %q", commandPath, got, tt.want)
			}
		})
	}

	t.Run("Should hide profile selection from profile-independent command help", func(t *testing.T) {
		t.Parallel()
		root := newRootCommand(commandDeps{})
		for _, commandPath := range []string{
			"completion",
			"completion bash",
			"daemon",
			"daemon bootstrap",
			"daemon start",
			"daemon stop",
			"doctor",
			"install",
			"network",
			"network directs",
			"network inbox",
			"network invitation",
			"network invitation dismiss",
			"network invitation reset",
			"network peers",
			"network send",
			"network status",
			"network subscriptions",
			"network threads",
			"update",
		} {
			command, _, err := root.Find(strings.Fields(commandPath))
			if err != nil {
				t.Fatalf("find %q: %v", commandPath, err)
			}
			if usage := command.UsageString(); strings.Contains(usage, "--profile") {
				t.Fatalf("help for %q advertises profile selection:\n%s", commandPath, usage)
			}
		}
		sessionList, _, err := root.Find([]string{"session", "list"})
		if err != nil {
			t.Fatalf("find session list: %v", err)
		}
		if usage := sessionList.UsageString(); !strings.Contains(usage, "--profile") {
			t.Fatalf("session list help omits profile selection:\n%s", usage)
		}
		for _, commandPath := range []string{"network threads list"} {
			command, _, err := root.Find(strings.Fields(commandPath))
			if err != nil {
				t.Fatalf("find %q: %v", commandPath, err)
			}
			usage := command.UsageString()
			if !strings.Contains(usage, "--profile") || !strings.Contains(usage, "--all-profiles") {
				t.Fatalf("profile-aware help for %q omits a selector:\n%s", commandPath, usage)
			}
		}
	})

	t.Run("Should reject profile selection before running profile-independent commands", func(t *testing.T) {
		t.Parallel()

		for _, args := range [][]string{
			{"--profile", "marketing", "completion", "bash"},
			{"completion", "bash", "--profile", "marketing"},
			{"--profile", "marketing", "install"},
			{"install", "--profile", "marketing"},
			{"--profile", "marketing", "network", "status"},
			{"network", "status", "--profile", "marketing"},
			{"--profile", "marketing", "network", "peers"},
			{"network", "peers", "--profile", "marketing"},
		} {
			_, _, err := executeRootCommand(t, commandDeps{}, args...)
			var profileErr *profileCommandError
			if !errors.As(err, &profileErr) || profileErr.payload.Error.Code != profileSelectionUnsupportedCode {
				t.Fatalf(
					"executeRootCommand(%v) error = %v, want %s payload",
					args,
					err,
					profileSelectionUnsupportedCode,
				)
			}
		}
	})
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

	// Verify compozy.mdx exists at the root (from compozy.md). Root index.mdx is
	// hand-authored by the site and preserved by the post-processor.
	rootMDX := filepath.Join(outputDir, "compozy.mdx")
	data, err := os.ReadFile(rootMDX)
	if err != nil {
		t.Fatalf("compozy.mdx should exist at root: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "---") {
		t.Error("compozy.mdx should have YAML frontmatter")
	}
	if !strings.Contains(content, `title: "compozy"`) {
		t.Error("compozy.mdx frontmatter should have title 'compozy'")
	}
	if !strings.Contains(content, `description: "CompozyOS — the system around your agents, already built"`) {
		t.Error("compozy.mdx should preserve the root Short verbatim from Cobra")
	}

	agentIndex, err := os.ReadFile(filepath.Join(outputDir, "agent", "index.mdx"))
	if err != nil {
		t.Fatalf("agent/index.mdx should exist: %v", err)
	}
	if !strings.Contains(string(agentIndex), "Not emitted for help") {
		t.Error("help-only agent group should not claim structured output")
	}
	if strings.Contains(string(agentIndex), "compozy agent -o json") {
		t.Error("help-only agent group should not advertise a JSON example")
	}

	taskIndex, err := os.ReadFile(filepath.Join(outputDir, "task", "index.mdx"))
	if err != nil {
		t.Fatalf("task/index.mdx should exist: %v", err)
	}
	if !strings.Contains(string(taskIndex), "Not emitted for help") {
		t.Error("RunE-free task group should be documented as help-only")
	}
	if strings.Contains(string(taskIndex), "compozy task -o json") {
		t.Error("help-only task group should not advertise a JSON example")
	}

	layoutWatch, err := os.ReadFile(filepath.Join(outputDir, "layout", "watch.mdx"))
	if err != nil {
		t.Fatalf("layout/watch.mdx should exist: %v", err)
	}
	if !strings.Contains(string(layoutWatch), "supports interactive human output") {
		t.Error("layout watch docs should describe the stream-specific output contract")
	}
	if !strings.Contains(string(layoutWatch), "compozy layout watch -o jsonl") {
		t.Error("layout watch docs should demonstrate its supported machine format")
	}
	if strings.Contains(string(layoutWatch), "compozy layout watch -o json\n") {
		t.Error("layout watch docs should not demonstrate unsupported JSON output")
	}

	completionBash, err := os.ReadFile(filepath.Join(outputDir, "completion", "bash.mdx"))
	if err != nil {
		t.Fatalf("completion/bash.mdx should exist: %v", err)
	}
	if !strings.Contains(string(completionBash), "Shell-completion script") {
		t.Error("completion docs should preserve their raw standard-output contract")
	}
	if strings.Contains(string(completionBash), "-o json") {
		t.Error("completion docs should not advertise a JSON result example")
	}

	agentUpdate, err := os.ReadFile(filepath.Join(outputDir, "agent", "update.mdx"))
	if err != nil {
		t.Fatalf("agent/update.mdx should exist: %v", err)
	}
	if !strings.Contains(string(agentUpdate), "Update an agent definition for future sessions") {
		t.Error("agent update docs should describe future-session mutability")
	}

	listPage, err := os.ReadFile(filepath.Join(outputDir, "cmd-palette", "list.mdx"))
	if err != nil {
		t.Fatalf("cmd-palette/list.mdx should exist: %v", err)
	}
	listBody := string(listPage)
	if !strings.Contains(listBody, cmdPaletteAvailableFlagHelp) {
		t.Error("cmd-palette list docs should say --available filtering is opt-in")
	}
	if !strings.Contains(listBody, "--available=false") {
		t.Error("cmd-palette list docs should keep an explicit --available=false example")
	}
	for line := range strings.SplitSeq(listBody, "\n") {
		if !strings.Contains(line, "--available") || !strings.Contains(line, cmdPaletteAvailableFlagHelp) {
			continue
		}
		if strings.Contains(line, "(default true)") {
			t.Errorf("cmd-palette list --available line %q must not advertise default true", line)
		}
	}

	generated := findMDX(t, outputDir)
	for _, expected := range []string{
		"mcp/auth/login.mdx",
		"memory/extractor/list-failures.mdx",
		"network/work/lookup.mdx",
	} {
		if !generated[expected] {
			t.Errorf("expected generated CLI doc %q", expected)
		}
	}
	for _, removed := range []string{
		"mcp/authorize.mdx",
		"memory/extractor/list-pending.mdx",
		"network/work/status.mdx",
	} {
		if generated[removed] {
			t.Errorf("removed CLI command doc %q must not be generated", removed)
		}
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
		"compozy.mdx",        // from compozy
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
