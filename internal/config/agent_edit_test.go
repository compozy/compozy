package config

import (
	"path/filepath"
	"testing"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestEditAgentDefFileCategoryPath(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve category path on skill toggle", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), agentDefName)
		writeFile(t, path, `---
name: coder
provider: claude
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
