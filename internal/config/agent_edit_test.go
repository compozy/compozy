package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	t.Run("Should keep bounded input forms across edits, drafts and clones", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), agentDefName)
		forms := "accepts:\n  - \"intake:   <text>\"\n  - \"  \"\n  - " + strings.Repeat(
			"x",
			MaxAgentAcceptRunes+40,
		) + "\n"
		for index := range MaxAgentAccepts {
			forms += fmt.Sprintf("  - \"extra-%d:\"\n", index)
		}
		writeFile(t, path, "---\nname: writer\nprovider: codex\nkind: always_on\n"+forms+"---\n\nPrompt.\n")

		agent, err := EditAgentDefFile(path, func(agent *AgentDef) error {
			agent.Description = "Files issues"
			return nil
		})
		if err != nil {
			t.Fatalf("EditAgentDefFile() error = %v", err)
		}
		reloaded, err := LoadAgentDefFile(path)
		if err != nil {
			t.Fatalf("LoadAgentDefFile() error = %v", err)
		}
		for _, got := range [][]string{
			agent.AcceptsValue(), reloaded.AcceptsValue(), CloneAgentDef(reloaded).AcceptsValue(),
			AgentDefinitionDraftFromDef(reloaded).Accepts,
		} {
			if len(got) != MaxAgentAccepts || got[0] != "intake: <text>" ||
				len([]rune(got[1])) != MaxAgentAcceptRunes || got[2] != "extra-0:" {
				t.Fatalf("accepts = %#v", got)
			}
		}
		if got := reloaded.AcceptedInputs(); len(got) != MaxAgentAccepts || got[0] != "intake: <text>" {
			t.Fatalf("declared accepted inputs = %#v", got)
		}
		prose := AgentDef{Prompt: "Intro.\n\n## Rules\n\n```text\nnot: input\n```\n\n## Input\n\n" +
			"Send one of:\n\n```text\nintake: <text>\nsweep:\n```\n\n```text\nlater: block\n```\n"}
		if got := prose.AcceptedInputs(); len(got) != 2 || got[0] != "intake: <text>" || got[1] != "sweep:" {
			t.Fatalf("accepted inputs from the Input section = %#v", got)
		}
		nested := AgentDef{Prompt: "## Example\n\n````markdown\n## Input\n```text\nexample: <text>\n```\n````\n\n" +
			"## Input\n\n```text\nreal: <text>\n```\n"}
		if got := nested.AcceptedInputs(); len(got) != 1 || got[0] != "real: <text>" {
			t.Fatalf("accepted inputs with a fenced example = %#v", got)
		}
		if got := (AgentDef{Prompt: "## Input\n\nFree text."}).AcceptedInputs(); got != nil {
			t.Fatalf("accepted inputs without a block = %#v", got)
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

func TestAgentKindAliasEdit(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"stays_on", "\"stays_on\"", "'stays_on'", "stays_on # note"} {
		t.Run("Should reject retired persisted kind while editing "+kind, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "triage", AgentDefinitionFileName)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			body := "---\nname: triage\nkind: " + kind + "\n---\nWatch.\n"
			writeFile(t, path, body)
			_, err := EditAgentDefFile(path, func(a *AgentDef) error { a.Description = "New"; return nil })
			if !errors.Is(err, ErrAgentKindInvalid) {
				t.Fatalf("edit error = %v, want ErrAgentKindInvalid", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != body {
				t.Fatalf("rejected definition changed: %q, %v", got, err)
			}
		})
	}
	t.Run("Should reject rendering retired kind", func(t *testing.T) {
		t.Parallel()
		if _, _, err := RenderAgentDefinition(
			AgentDefinitionDraft{Name: "triage", Kind: "stays_on", Prompt: "Watch."},
		); !errors.Is(
			err,
			ErrAgentKindInvalid,
		) {
			t.Fatalf("render error = %v", err)
		}
	})
}
