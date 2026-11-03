package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/fileutil"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func TestEditAgentDefFileCategoryPath(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve category path on skill toggle", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), agentDefName)
		writeFile(t, path, `---
name: coder
provider: claude
speed: fast
acp_options:
  - id: thinking
    bool_value: true
  - id: context
    value_id: 1m
category_path:
  - Marketing
  - Sales
skills:
  disabled:
    - old-skill
---

Prompt.
`)

		agent, err := EditAgentDefFile(path, func(agent *AgentDef) error {
			agent.Skills.Disabled = []string{"old-skill", "new-skill"}
			return nil
		})
		if err != nil {
			t.Fatalf("EditAgentDefFile() error = %v", err)
		}
		if !equalStringSlicesForTest(agent.CategoryPath, []string{"Marketing", "Sales"}) {
			t.Fatalf("EditAgentDefFile() CategoryPath = %#v", agent.CategoryPath)
		}

		reloaded, err := LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile() error = %v", err)
		}
		if !equalStringSlicesForTest(reloaded.CategoryPath, []string{"Marketing", "Sales"}) {
			t.Fatalf("LoadAgentDefFile() CategoryPath = %#v", reloaded.CategoryPath)
		}
		if !equalStringSlicesForTest(reloaded.Skills.Disabled, []string{"old-skill", "new-skill"}) {
			t.Fatalf("LoadAgentDefFile() Skills.Disabled = %#v", reloaded.Skills.Disabled)
		}
		reloadedOptions := reloaded.ACPOptionsValue()
		if reloaded.SpeedValue() != "fast" || len(reloadedOptions) != 2 || reloadedOptions[0].ID != "context" ||
			reloadedOptions[0].ValueID != "1m" || reloadedOptions[1].ID != "thinking" ||
			reloadedOptions[1].BoolValue == nil || !*reloadedOptions[1].BoolValue {
			t.Fatalf("LoadAgentDefFile() runtime defaults = %#v", reloaded)
		}
	})
}

func TestEditAgentDefFilePersistsHookMutations(t *testing.T) {
	t.Parallel()

	t.Run("Should persist an added hook", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), agentDefName)
		writeFile(t, path, `---
name: coder
provider: claude
---

Prompt.
`)

		_, err := EditAgentDefFile(path, func(agent *AgentDef) error {
			agent.Hooks = append(agent.Hooks, hookspkg.HookDecl{
				Name:    "prompt-sanitizer",
				Event:   hookspkg.HookPromptPostAssemble,
				Source:  hookspkg.HookSourceAgentDefinition,
				Mode:    hookspkg.HookModeSync,
				Command: "/bin/echo",
				Args:    []string{"sanitize"},
				Matcher: hookspkg.HookMatcher{InputClass: "user"},
			})
			return nil
		})
		if err != nil {
			t.Fatalf("EditAgentDefFile() error = %v", err)
		}

		reloaded := loadEditedAgentDefFile(t, path)
		hook := singleEditedAgentHook(t, reloaded)
		if got, want := hook.Name, "prompt-sanitizer"; got != want {
			t.Fatalf("hook.Name = %q, want %q", got, want)
		}
		if got, want := hook.Event, hookspkg.HookPromptPostAssemble; got != want {
			t.Fatalf("hook.Event = %q, want %q", got, want)
		}
		if got, want := hook.Source, hookspkg.HookSourceAgentDefinition; got != want {
			t.Fatalf("hook.Source = %q, want %q", got, want)
		}
		if got, want := hook.Matcher.AgentName, "coder"; got != want {
			t.Fatalf("hook.Matcher.AgentName = %q, want %q", got, want)
		}
		if got, want := hook.Matcher.InputClass, "user"; got != want {
			t.Fatalf("hook.Matcher.InputClass = %q, want %q", got, want)
		}
		if !equalStringSlicesForTest(hook.Args, []string{"sanitize"}) {
			t.Fatalf("hook.Args = %#v, want sanitize arg", hook.Args)
		}
	})

	t.Run("Should persist an updated hook", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), agentDefName)
		writeFile(t, path, `---
name: coder
provider: claude
hooks:
  - name: prompt-sanitizer
    event: prompt.post_assemble
    mode: sync
    command: /bin/echo
    args: ["old"]
---

Prompt.
`)

		_, err := EditAgentDefFile(path, func(agent *AgentDef) error {
			agent.Hooks[0].Command = "/usr/bin/printf"
			agent.Hooks[0].Args = []string{"new"}
			agent.Hooks[0].Matcher.InputClass = "system"
			return nil
		})
		if err != nil {
			t.Fatalf("EditAgentDefFile() error = %v", err)
		}

		hook := singleEditedAgentHook(t, loadEditedAgentDefFile(t, path))
		if got, want := hook.Command, "/usr/bin/printf"; got != want {
			t.Fatalf("hook.Command = %q, want %q", got, want)
		}
		if !equalStringSlicesForTest(hook.Args, []string{"new"}) {
			t.Fatalf("hook.Args = %#v, want updated arg", hook.Args)
		}
		if got, want := hook.Matcher.InputClass, "system"; got != want {
			t.Fatalf("hook.Matcher.InputClass = %q, want %q", got, want)
		}
	})

	t.Run("Should persist hook removal", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), agentDefName)
		writeFile(t, path, `---
name: coder
provider: claude
hooks:
  - name: prompt-sanitizer
    event: prompt.post_assemble
    mode: sync
    command: /bin/echo
---

Prompt.
`)

		_, err := EditAgentDefFile(path, func(agent *AgentDef) error {
			agent.Hooks = nil
			return nil
		})
		if err != nil {
			t.Fatalf("EditAgentDefFile() error = %v", err)
		}

		reloaded := loadEditedAgentDefFile(t, path)
		if got := len(reloaded.Hooks); got != 0 {
			t.Fatalf("len(LoadAgentDefFile().Hooks) = %d, want 0", got)
		}
	})
}

func TestEditAgentDefFileWritesThroughTheHeldParentAfterPathReplacement(t *testing.T) {
	t.Parallel()
	t.Run("Should write only through the opened agent parent", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		root := t.TempDir()
		liveParent := filepath.Join(root, "live")
		archivedParent := filepath.Join(root, "archived")
		externalParent := filepath.Join(root, "external")
		livePath := filepath.Join(liveParent, AgentDefinitionFileName)
		externalPath := filepath.Join(externalParent, AgentDefinitionFileName)
		if err := os.MkdirAll(liveParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(live parent) error = %v", err)
		}
		if err := os.MkdirAll(externalParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(external parent) error = %v", err)
		}
		writeFile(t, livePath, "---\nname: coder\nprovider: claude\nmodel: old\n---\n\nPrompt.\n")
		sentinel := "---\nname: external\nprovider: claude\nmodel: sentinel\n---\n\nExternal.\n"
		writeFile(t, externalPath, sentinel)

		directory, err := fileutil.OpenDirectory(liveParent)
		if err != nil {
			t.Fatalf("fileutil.OpenDirectory(live parent) error = %v", err)
		}
		defer func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		}()
		if err := os.Rename(liveParent, archivedParent); err != nil {
			t.Fatalf("os.Rename(live parent) error = %v", err)
		}
		if err := os.Symlink(externalParent, liveParent); err != nil {
			t.Fatalf("os.Symlink(external parent) error = %v", err)
		}

		_, err = editAgentDefFileInDirectory(directory, AgentDefinitionFileName, livePath, func(agent *AgentDef) error {
			agent.Model = "updated"
			return nil
		})
		if err != nil {
			t.Fatalf("editAgentDefFileInDirectory() error = %v", err)
		}
		assertConfigSentinelUnchanged(t, externalPath, sentinel)
		updated, err := os.ReadFile(filepath.Join(archivedParent, AgentDefinitionFileName))
		if err != nil {
			t.Fatalf("os.ReadFile(archived definition) error = %v", err)
		}
		if !strings.Contains(string(updated), "model: updated") {
			t.Fatalf("archived definition = %q, want updated model", updated)
		}
	})
}

func loadEditedAgentDefFile(t *testing.T, path string) AgentDef {
	t.Helper()

	agent, err := LoadAgentDefFile(path)
	if err != nil {
		t.Fatalf("LoadAgentDefFile() error = %v", err)
	}
	return agent
}

func singleEditedAgentHook(t *testing.T, agent AgentDef) hookspkg.HookDecl {
	t.Helper()

	if got, want := len(agent.Hooks), 1; got != want {
		t.Fatalf("len(LoadAgentDefFile().Hooks) = %d, want %d", got, want)
	}
	return agent.Hooks[0]
}

func assertConfigSentinelUnchanged(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(external sentinel) error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("external sentinel = %q, want %q", got, want)
	}
}

// TestRewriteAgentKindAliases owns UT-021: the v0.5.1 boot rewrite edits only the top-level kind
// value span of daemon-owned AGENT.md files, keeps every other byte and the mode, and leaves
// symlinked, read-only, changed, unsupported, and unreadable files intact. Removal: v0.5.2.
func TestRewriteAgentKindAliases(t *testing.T) {
	t.Parallel()

	rewrites := []struct {
		name, before, after string
	}{
		{
			name: "Should rewrite only the top-level YAML value",
			before: "---\nname: scout\nkind: stays_on\npermission_rules:\n  - kind: stays_on\n    decision: allow\n" +
				"---\nkind: stays_on\n",
			after: "---\nname: scout\nkind: always_on\npermission_rules:\n  - kind: stays_on\n    decision: allow\n" +
				"---\nkind: stays_on\n",
		},
		{
			name:   "Should keep double quotes",
			before: "---\nname: scout\nkind: \"stays_on\"\n---\nBody.\n",
			after:  "---\nname: scout\nkind: \"always_on\"\n---\nBody.\n",
		},
		{
			name:   "Should keep single quotes",
			before: "---\nname: scout\nkind: 'stays_on'\n---\nBody.\n",
			after:  "---\nname: scout\nkind: 'always_on'\n---\nBody.\n",
		},
		{
			name:   "Should keep a trailing comment",
			before: "---\nname: scout\nkind: stays_on # note\n---\nBody.\n",
			after:  "---\nname: scout\nkind: always_on # note\n---\nBody.\n",
		},
		{
			name:   "Should rewrite TOML frontmatter",
			before: "---\nname = \"scout\"\nkind = \"stays_on\"\n---\nBody.\n",
			after:  "---\nname = \"scout\"\nkind = \"always_on\"\n---\nBody.\n",
		},
		{
			name:   "Should keep CRLF line endings",
			before: "---\r\nname: scout\r\nkind: stays_on\r\n---\r\nBody.\r\n",
			after:  "---\r\nname: scout\r\nkind: always_on\r\n---\r\nBody.\r\n",
		},
	}
	for _, tc := range rewrites {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := writeRewriteAgent(t, root, "scout", tc.before, 0o644)
			log, logs := captureRewriteLog()
			rewritten, skipped, err := RewriteAgentKindAliases(root, log)
			if err != nil {
				t.Fatalf("RewriteAgentKindAliases() error = %v", err)
			}
			if len(rewritten) != 1 || rewritten[0] != path || len(skipped) != 0 {
				t.Fatalf("rewritten = %v skipped = %v, want only %s", rewritten, skipped, path)
			}
			assertRewriteFile(t, path, tc.after, 0o644)
			if !strings.Contains(logs.String(), "rewrote agent kind stays_on to always_on") ||
				!strings.Contains(logs.String(), "path="+path) {
				t.Fatalf("logs = %q, want one rewrite line for %s", logs.String(), path)
			}

			rewritten, skipped, err = RewriteAgentKindAliases(root, log)
			if err != nil || len(rewritten) != 0 || len(skipped) != 0 {
				t.Fatalf("second pass rewritten = %v skipped = %v err = %v, want no-op", rewritten, skipped, err)
			}
			assertRewriteFile(t, path, tc.after, 0o644)
		})
	}

	t.Run("Should leave canonical and kindless files untouched", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		files := map[string]string{
			"always": "---\nname: always\nkind: always_on\n---\nBody.\n",
			"demand": "---\nname: demand\nkind: on_demand\n---\nBody.\n",
			"plain":  "---\nname: plain\n---\nBody.\n",
		}
		for name, body := range files {
			writeRewriteAgent(t, root, name, body, 0o644)
		}
		rewritten, skipped, err := RewriteAgentKindAliases(root, discardRewriteLog())
		if err != nil || len(rewritten) != 0 || len(skipped) != 0 {
			t.Fatalf("rewritten = %v skipped = %v err = %v, want no-op", rewritten, skipped, err)
		}
		for name, body := range files {
			assertRewriteFile(t, filepath.Join(root, name, agentDefName), body, 0o644)
		}
	})

	t.Run("Should skip a read-only file", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		body := "---\nname: locked\nkind: stays_on\n---\nBody.\n"
		path := writeRewriteAgent(t, root, "locked", body, 0o444)
		assertRewriteSkip(t, root, path, AgentKindRewriteSkipReadOnly)
		assertRewriteFile(t, path, body, 0o444)
	})

	t.Run("Should skip symlinked agent directories and files", func(t *testing.T) {
		t.Parallel()

		root, outside := t.TempDir(), t.TempDir()
		body := "---\nname: linked\nkind: stays_on\n---\nBody.\n"
		target := writeRewriteAgent(t, outside, "linked", body, 0o644)
		if err := os.Symlink(filepath.Join(outside, "linked"), filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "filelink"), 0o755); err != nil {
			t.Fatal(err)
		}
		fileLink := filepath.Join(root, "filelink", agentDefName)
		if err := os.Symlink(target, fileLink); err != nil {
			t.Fatal(err)
		}
		rewritten, skipped, err := RewriteAgentKindAliases(root, discardRewriteLog())
		if err != nil || len(rewritten) != 0 {
			t.Fatalf("rewritten = %v err = %v, want none", rewritten, err)
		}
		if skipped[filepath.Join(root, "linked")] != AgentKindRewriteSkipSymlink ||
			skipped[fileLink] != AgentKindRewriteSkipSymlink || len(skipped) != 2 {
			t.Fatalf("skipped = %v, want two symlink skips", skipped)
		}
		assertRewriteFile(t, target, body, 0o644)
		if info, err := os.Lstat(fileLink); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("file link after rewrite = %v %v, want a symlink", info, err)
		}
	})

	t.Run("Should skip an unsupported YAML anchor that still loads through the alias", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		body := "---\nname: anchored\nkind: &k stays_on\n---\nBody.\n"
		path := writeRewriteAgent(t, root, "anchored", body, 0o644)
		assertRewriteSkip(t, root, path, AgentKindRewriteSkipUnsupported)
		assertRewriteFile(t, path, body, 0o644)
		agent, err := LoadAgentDefFile(path)
		if err != nil || agent.Kind != AgentKindAlwaysOn {
			t.Fatalf("LoadAgentDefFile() = %q, %v; want always_on", agent.Kind, err)
		}
	})

	t.Run("Should skip a stays_on file that fails strict decode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		body := "---\nname: strict\nkind: stays_on\nunknown_field: true\n---\nBody.\n"
		path := writeRewriteAgent(t, root, "strict", body, 0o644)
		if _, _, err := decodeAgentFileKind([]byte(body)); err == nil {
			t.Fatal("decodeAgentFileKind() error = nil, want a strict decode failure")
		}
		assertRewriteSkip(t, root, path, AgentKindRewriteSkipUnsupported)
		assertRewriteFile(t, path, body, 0o644)
	})

	t.Run("Should not publish when the file changes before the final compare", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		path := writeRewriteAgent(t, root, "raced", "---\nname: raced\nkind: stays_on\n---\nBody.\n", 0o644)
		edited := "---\nname: raced\nkind: stays_on\n---\nEdited by someone else.\n"
		rewritten, skipped, err := rewriteAgentKindAliases(root, discardRewriteLog(), agentKindRewriteHooks{
			beforeCompare: func(string) {
				if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
					t.Errorf("replace file before compare: %v", err)
				}
			},
		})
		if err != nil || len(rewritten) != 0 || skipped[path] != AgentKindRewriteSkipChanged {
			t.Fatalf("rewritten = %v skipped = %v err = %v, want changed_during_rewrite", rewritten, skipped, err)
		}
		assertRewriteFile(t, path, edited, 0o644)
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil || len(entries) != 1 {
			t.Fatalf("agent directory entries = %v %v, want only AGENT.md (temp file removed)", entries, err)
		}
	})

	t.Run("Should count a post-publish durability error as rewritten", func(t *testing.T) {
		t.Parallel()

		published, reason := classifyAgentKindRewriteError(
			fmt.Errorf("%w: sync directory: boom", fileutil.ErrPublishedNotDurable),
		)
		if !published || reason != "" {
			t.Fatalf("classify durability error = %v %q, want published", published, reason)
		}
		pass := &agentKindRewritePass{skipped: map[string]AgentKindRewriteSkip{}}
		log, logs := captureRewriteLog()
		pass.log = log
		pass.record("/agents/x/AGENT.md", fmt.Errorf("%w: boom", fileutil.ErrPublishedNotDurable))
		if len(pass.rewritten) != 1 || !strings.Contains(logs.String(), "agent kind rewrite durability error") {
			t.Fatalf("rewritten = %v logs = %q, want one durability WARN", pass.rewritten, logs.String())
		}
	})

	t.Run("Should skip an unreadable file", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("POSIX permission bits drive the read failure")
		}
		root := t.TempDir()
		body := "---\nname: hidden\nkind: stays_on\n---\nBody.\n"
		path := writeRewriteAgent(t, root, "hidden", body, 0o200)
		assertRewriteSkip(t, root, path, AgentKindRewriteSkipIO)
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		assertRewriteFile(t, path, body, 0o600)
	})

	t.Run("Should skip a symlinked root", func(t *testing.T) {
		t.Parallel()

		outside := t.TempDir()
		body := "---\nname: scout\nkind: stays_on\n---\nBody.\n"
		target := writeRewriteAgent(t, outside, "scout", body, 0o644)
		root := filepath.Join(t.TempDir(), "agents")
		if err := os.Symlink(outside, root); err != nil {
			t.Fatal(err)
		}
		rewritten, skipped, err := RewriteAgentKindAliases(root, discardRewriteLog())
		if err != nil || len(rewritten) != 0 || len(skipped) != 1 || skipped[root] != AgentKindRewriteSkipSymlink {
			t.Fatalf("rewritten = %v skipped = %v err = %v, want one root symlink skip", rewritten, skipped, err)
		}
		assertRewriteFile(t, target, body, 0o644)
	})
}

func writeRewriteAgent(t *testing.T, root, name, body string, mode os.FileMode) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, agentDefName)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertRewriteFile(t *testing.T, path, want string, mode os.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != mode {
		t.Fatalf("%s mode = %v, want %v", path, info.Mode().Perm(), mode)
	}
}

func assertRewriteSkip(t *testing.T, root, path string, reason AgentKindRewriteSkip) {
	t.Helper()
	log, logs := captureRewriteLog()
	rewritten, skipped, err := RewriteAgentKindAliases(root, log)
	if err != nil || len(rewritten) != 0 || len(skipped) != 1 || skipped[path] != reason {
		t.Fatalf("rewritten = %v skipped = %v err = %v, want %s skip for %s", rewritten, skipped, err, reason, path)
	}
	if !strings.Contains(logs.String(), "reason="+string(reason)) {
		t.Fatalf("logs = %q, want reason=%s", logs.String(), reason)
	}
}

func captureRewriteLog() (*slog.Logger, *strings.Builder) {
	var logs strings.Builder
	return slog.New(slog.NewTextHandler(&lockedRewriteWriter{w: &logs}, nil)), &logs
}

func discardRewriteLog() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

type lockedRewriteWriter struct {
	mu sync.Mutex
	w  *strings.Builder
}

func (l *lockedRewriteWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// TestAgentKindAliasEdit owns UT-004 and UT-005: edits and renders of a v0.5.0 stays_on definition
// are not kind changes and always write always_on.
func TestAgentKindAliasEdit(t *testing.T) {
	t.Parallel()

	t.Run("Should edit a stays_on file without a kind change and write always_on", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), agentDefName)
		writeFile(
			t,
			path,
			"---\nname: triage\nprovider: claude\nkind: stays_on\ndescription: Old\npermissions: approve-edits\n---\nWatch.\n",
		)
		agent, err := EditAgentDefFile(path, func(agent *AgentDef) error {
			agent.Description = "Watches issues"
			return nil
		})
		if err != nil {
			t.Fatalf("EditAgentDefFile() error = %v", err)
		}
		if agent.Kind != AgentKindAlwaysOn {
			t.Fatalf("edited kind = %q, want always_on", agent.Kind)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"kind: always_on", "name: triage", "provider: claude", "description: Watches issues", "permissions: approve-edits"} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("edited file = %q, want %q", body, want)
			}
		}
		if strings.Contains(string(body), "stays_on") {
			t.Fatalf("edited file = %q, want no stays_on", body)
		}
	})

	t.Run("Should render a stays_on draft as always_on", func(t *testing.T) {
		t.Parallel()

		rendered, agent, err := RenderAgentDefinition(
			AgentDefinitionDraft{Name: "triage", Kind: "stays_on", Prompt: "Watch."},
		)
		if err != nil {
			t.Fatalf("RenderAgentDefinition() error = %v", err)
		}
		if agent.Kind != AgentKindAlwaysOn || !strings.Contains(string(rendered), "kind: always_on") ||
			strings.Contains(string(rendered), "stays_on") {
			t.Fatalf("rendered = %q kind = %q, want kind: always_on", rendered, agent.Kind)
		}
	})
}
