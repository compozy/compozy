package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestLoadParsesConfigHookDeclarationWithAllFields(t *testing.T) {
	workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[[hooks.declarations]]
name = "audit-tool"
event = "tool.pre_call"
mode = "sync"
required = false
priority = 640
timeout = "7s"

[hooks.declarations.matcher]
tool_id = "agh__read_file"
tool_read_only = true

[hooks.declarations.executor]
command = "/bin/echo"
args = ["audit"]
env = { PHASE = "pre" }
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	decls, err := HookDeclarations(cfg.Hooks, nil)
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if got, want := len(decls), 1; got != want {
		t.Fatalf("len(HookDeclarations()) = %d, want %d", got, want)
	}

	hook := decls[0]
	if got, want := hook.Name, "audit-tool"; got != want {
		t.Fatalf("hook.Name = %q, want %q", got, want)
	}
	if got, want := hook.Event, hookspkg.HookToolPreCall; got != want {
		t.Fatalf("hook.Event = %q, want %q", got, want)
	}
	if got, want := hook.Source, hookspkg.HookSourceConfig; got != want {
		t.Fatalf("hook.Source = %q, want %q", got, want)
	}
	if got, want := hook.Mode, hookspkg.HookModeSync; got != want {
		t.Fatalf("hook.Mode = %q, want %q", got, want)
	}
	if got, want := hook.Priority, int32(640); got != want {
		t.Fatalf("hook.Priority = %d, want %d", got, want)
	}
	if got, want := hook.Timeout, 7*time.Second; got != want {
		t.Fatalf("hook.Timeout = %s, want %s", got, want)
	}
	if got, want := hook.ExecutorKind, hookspkg.HookExecutorSubprocess; got != want {
		t.Fatalf("hook.ExecutorKind = %q, want %q", got, want)
	}
	if got, want := hook.Command, "/bin/echo"; got != want {
		t.Fatalf("hook.Command = %q, want %q", got, want)
	}
	if got, want := strings.Join(hook.Args, ","), "audit"; got != want {
		t.Fatalf("hook.Args = %#v, want %q", hook.Args, want)
	}
	if got, want := hook.Env["PHASE"], "pre"; got != want {
		t.Fatalf("hook.Env[PHASE] = %q, want %q", got, want)
	}
	if got, want := hook.Matcher.ToolID, "agh__read_file"; got != want {
		t.Fatalf("hook.Matcher.ToolID = %q, want %q", got, want)
	}
	if hook.Matcher.ToolReadOnly == nil || !*hook.Matcher.ToolReadOnly {
		t.Fatalf("hook.Matcher.ToolReadOnly = %#v, want true", hook.Matcher.ToolReadOnly)
	}
}

func TestLoadParsesMinimalConfigHookAndAppliesDefaults(t *testing.T) {
	workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[[hooks.declarations]]
name = "workspace-ready"
event = "session.post_create"
command = "/bin/echo"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	decls, err := HookDeclarations(cfg.Hooks, nil)
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if got, want := len(decls), 1; got != want {
		t.Fatalf("len(HookDeclarations()) = %d, want %d", got, want)
	}

	hook := decls[0]
	if got, want := hook.Mode, hookspkg.HookModeAsync; got != want {
		t.Fatalf("hook.Mode = %q, want %q", got, want)
	}
	if got, want := hook.Priority, int32(500); got != want {
		t.Fatalf("hook.Priority = %d, want %d", got, want)
	}
	if hook.PrioritySet {
		t.Fatal("hook.PrioritySet = true, want false for default priority")
	}
	if got, want := hook.ExecutorKind, hookspkg.HookExecutorSubprocess; got != want {
		t.Fatalf("hook.ExecutorKind = %q, want %q", got, want)
	}
}

func TestLoadParsesNetworkHookMatcherFields(t *testing.T) {
	t.Run("Should parse network matcher fields", func(t *testing.T) {
		workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
		writeFile(t, homePaths.ConfigFile, `
[[hooks.declarations]]
name = "network-observer"
event = "network.message.persisted"
mode = "async"
command = "/bin/echo"

[hooks.declarations.matcher]
channel = "builders"
surface = "direct"
kind = "trace"
direction = "received"
work_state = "completed"
`)

		cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		decls, err := HookDeclarations(cfg.Hooks, nil)
		if err != nil {
			t.Fatalf("HookDeclarations() error = %v", err)
		}
		if got, want := len(decls), 1; got != want {
			t.Fatalf("len(HookDeclarations()) = %d, want %d", got, want)
		}

		matcher := decls[0].Matcher.NetworkMatcher
		if matcher == nil {
			t.Fatal("NetworkMatcher = nil, want parsed matcher")
		}
		if matcher.Channel != "builders" ||
			matcher.Surface != "direct" ||
			matcher.Kind != "trace" ||
			matcher.Direction != "received" ||
			matcher.WorkState != "completed" {
			t.Fatalf("NetworkMatcher = %#v, want parsed network fields", matcher)
		}
	})
}

func TestCloneHookDeclDeepCopiesMatcherPointers(t *testing.T) {
	t.Parallel()

	t.Run("Should clone matcher pointers independently", func(t *testing.T) {
		t.Parallel()

		toolReadOnly := true
		decl := hookspkg.HookDecl{
			Matcher: hookspkg.HookMatcher{
				ToolReadOnly: &toolReadOnly,
				NetworkMatcher: &hookspkg.NetworkMatcher{
					Channel: "builders",
				},
				CompactionMatcher: &hookspkg.CompactionMatcher{
					Reason: "size",
				},
				Autonomy: &hookspkg.AutonomyMatcher{
					TaskID: "task-1",
				},
			},
		}

		cloned := cloneHookDecl(decl)
		cloned.Matcher.Channel = "ops"
		cloned.Matcher.Reason = "time"
		cloned.Matcher.Autonomy.TaskID = "task-2"
		*cloned.Matcher.ToolReadOnly = false

		if got, want := decl.Matcher.Channel, "builders"; got != want {
			t.Fatalf("source NetworkMatcher.Channel = %q, want %q", got, want)
		}
		if got, want := decl.Matcher.Reason, "size"; got != want {
			t.Fatalf("source CompactionMatcher.Reason = %q, want %q", got, want)
		}
		if got, want := decl.Matcher.Autonomy.TaskID, "task-1"; got != want {
			t.Fatalf("source Autonomy.TaskID = %q, want %q", got, want)
		}
		if got, want := *decl.Matcher.ToolReadOnly, true; got != want {
			t.Fatalf("source ToolReadOnly = %v, want %v", got, want)
		}
	})
}

func TestLoadRejectsInvalidConfigHookEvent(t *testing.T) {
	workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[[hooks.declarations]]
name = "bad-event"
event = "bad.event"
command = "/bin/echo"
`)

	_, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "hooks.declarations") || !strings.Contains(err.Error(), "bad.event") {
		t.Fatalf("Load() error = %v, want hooks.declarations invalid event detail", err)
	}
}

func TestLoadRejectsRequiredAsyncConfigHook(t *testing.T) {
	workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[[hooks.declarations]]
name = "must-not-async"
event = "session.post_create"
mode = "async"
required = true
command = "/bin/echo"
`)

	_, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "must-not-async") || !strings.Contains(err.Error(), "async") {
		t.Fatalf("Load() error = %v, want required async detail", err)
	}
}

func TestLoadMergesConfigHooksAcrossPrecedenceLevels(t *testing.T) {
	workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[[hooks.declarations]]
name = "global-only"
event = "session.post_create"
command = "/bin/global"

[[hooks.declarations]]
name = "shared"
event = "session.post_stop"
command = "/bin/global-shared"
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[[hooks.declarations]]
name = "workspace-only"
event = "input.pre_submit"
command = "/bin/workspace"

[[hooks.declarations]]
name = "shared"
event = "session.post_stop"
command = "/bin/workspace-shared"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	decls, err := HookDeclarations(cfg.Hooks, nil)
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if got, want := len(decls), 3; got != want {
		t.Fatalf("len(HookDeclarations()) = %d, want %d", got, want)
	}

	got := map[string]hookspkg.HookDecl{}
	for _, decl := range decls {
		got[decl.Name] = decl
	}
	if _, ok := got["global-only"]; !ok {
		t.Fatalf("HookDeclarations() missing global-only: %#v", decls)
	}
	if _, ok := got["workspace-only"]; !ok {
		t.Fatalf("HookDeclarations() missing workspace-only: %#v", decls)
	}
	if got["shared"].Command != "/bin/workspace-shared" {
		t.Fatalf("shared command = %q, want %q", got["shared"].Command, "/bin/workspace-shared")
	}
}

func TestParseAgentDefParsesHookAndScopesMatcherToAgent(t *testing.T) {
	t.Parallel()

	agent, err := ParseAgentDef([]byte(`---
name: coder
provider: claude
hooks:
  - name: prompt-sanitizer
    event: prompt.post_assemble
    mode: sync
    command: /bin/echo
    args: ["sanitize"]
---

Keep prompts tight.
`))
	if err != nil {
		t.Fatalf("ParseAgentDef() error = %v", err)
	}
	if got, want := len(agent.Hooks), 1; got != want {
		t.Fatalf("len(agent.Hooks) = %d, want %d", got, want)
	}

	hook := agent.Hooks[0]
	if got, want := hook.Name, "prompt-sanitizer"; got != want {
		t.Fatalf("hook.Name = %q, want %q", got, want)
	}
	if got, want := hook.Source, hookspkg.HookSourceAgentDefinition; got != want {
		t.Fatalf("hook.Source = %q, want %q", got, want)
	}
	if got, want := hook.Matcher.AgentName, "coder"; got != want {
		t.Fatalf("hook.Matcher.AgentName = %q, want %q", got, want)
	}
}

func TestParseAgentDefParsesTOMLHookAndScopesMatcherToAgent(t *testing.T) {
	t.Parallel()

	agent, err := ParseAgentDef([]byte(`---
name = "reviewer"
provider = "codex"

[[hooks]]
name = "review-gate"
event = "input.pre_submit"
command = "/bin/echo"
---

Review carefully.
`))
	if err != nil {
		t.Fatalf("ParseAgentDef() error = %v", err)
	}
	if got, want := len(agent.Hooks), 1; got != want {
		t.Fatalf("len(agent.Hooks) = %d, want %d", got, want)
	}

	hook := agent.Hooks[0]
	if got, want := hook.Name, "review-gate"; got != want {
		t.Fatalf("hook.Name = %q, want %q", got, want)
	}
	if got, want := hook.Source, hookspkg.HookSourceAgentDefinition; got != want {
		t.Fatalf("hook.Source = %q, want %q", got, want)
	}
	if got, want := hook.Matcher.AgentName, "reviewer"; got != want {
		t.Fatalf("hook.Matcher.AgentName = %q, want %q", got, want)
	}
}

func TestParseAgentDefRejectsHookScopedToDifferentAgent(t *testing.T) {
	t.Parallel()

	_, err := ParseAgentDef([]byte(`---
name: coder
provider: claude
hooks:
  - name: prompt-sanitizer
    event: prompt.post_assemble
    command: /bin/echo
    matcher:
      agent_name: reviewer
---

Keep prompts tight.
`))
	if err == nil {
		t.Fatal("ParseAgentDef() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "matcher.agent_name") || !strings.Contains(err.Error(), "coder") {
		t.Fatalf("ParseAgentDef() error = %v, want scoped agent detail", err)
	}
}

func TestHookDeclarationsReturnsCombinedConfigAndAgentHooks(t *testing.T) {
	workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[[hooks.declarations]]
name = "global-create"
event = "session.post_create"
command = "/bin/global"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	agent, err := ParseAgentDef([]byte(`---
name: coder
provider: claude
hooks:
  - name: agent-input
    event: input.pre_submit
    command: /bin/echo
---

Prompt.
`))
	if err != nil {
		t.Fatalf("ParseAgentDef() error = %v", err)
	}

	decls, err := HookDeclarations(cfg.Hooks, []AgentDef{agent})
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if got, want := len(decls), 2; got != want {
		t.Fatalf("len(HookDeclarations()) = %d, want %d", got, want)
	}

	got := map[string]hookspkg.HookDecl{}
	for _, decl := range decls {
		got[decl.Name] = decl
	}
	if got["global-create"].Priority != 500 {
		t.Fatalf("global-create priority = %d, want 500", got["global-create"].Priority)
	}
	if got["agent-input"].Priority != 100 {
		t.Fatalf("agent-input priority = %d, want 100", got["agent-input"].Priority)
	}
	if got["agent-input"].Matcher.AgentName != "coder" {
		t.Fatalf("agent-input matcher.agent_name = %q, want %q", got["agent-input"].Matcher.AgentName, "coder")
	}
}

func TestHookDeclarationsReturnsEmptySliceForEmptyHooksSection(t *testing.T) {
	workspaceRoot, homePaths := prepareHookConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[hooks]
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	decls, err := HookDeclarations(cfg.Hooks, nil)
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if decls == nil {
		t.Fatal("HookDeclarations() = nil, want empty slice")
	}
	if got := len(decls); got != 0 {
		t.Fatalf("len(HookDeclarations()) = %d, want 0", got)
	}
}

func prepareHookConfigTestEnv(t *testing.T) (string, HomePaths) {
	t.Helper()

	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	return workspaceRoot, homePaths
}
