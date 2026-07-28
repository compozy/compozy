package hooks

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateHookDeclRejectsSyncForAsyncOnlyEvent(t *testing.T) {
	t.Parallel()

	err := ValidateHookDecl(HookDecl{
		Name:    "delta-blocker",
		Event:   HookMessageDelta,
		Source:  HookSourceConfig,
		Mode:    HookModeSync,
		Command: "./hook.sh",
	})
	if err == nil {
		t.Fatal("ValidateHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "async-only event") || !strings.Contains(err.Error(), string(HookMessageDelta)) {
		t.Fatalf("ValidateHookDecl() error = %q, want async-only message.delta detail", err)
	}
}

func TestValidateHookDeclRejectsRequiredAsyncHook(t *testing.T) {
	t.Parallel()

	err := ValidateHookDecl(HookDecl{
		Name:     "required-async",
		Event:    HookSessionPostCreate,
		Source:   HookSourceConfig,
		Mode:     HookModeAsync,
		Required: true,
		Command:  "./hook.sh",
	})
	if err == nil {
		t.Fatal("ValidateHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "required-async") || !strings.Contains(err.Error(), "async mode") {
		t.Fatalf("ValidateHookDecl() error = %q, want required async detail", err)
	}
}

func TestNormalizeHookDeclAppliesNativeDefaults(t *testing.T) {
	t.Parallel()

	hook, err := NormalizeHookDecl(HookDecl{
		Name:   "native-defaults",
		Event:  HookSessionPostCreate,
		Source: HookSourceNative,
	}, expectExecutorKind(t, HookExecutorNative))
	if err != nil {
		t.Fatalf("NormalizeHookDecl() error = %v", err)
	}

	if hook.Priority != 1000 {
		t.Fatalf("NormalizeHookDecl() priority = %d, want 1000", hook.Priority)
	}
	if hook.Mode != HookModeAsync {
		t.Fatalf("NormalizeHookDecl() mode = %q, want %q", hook.Mode, HookModeAsync)
	}
	if hook.Executor == nil || hook.Executor.Kind() != HookExecutorNative {
		t.Fatalf("NormalizeHookDecl() executor = %#v, want native executor", hook.Executor)
	}
}

func TestNormalizeHookDeclAppliesSkillDefaults(t *testing.T) {
	t.Parallel()

	hook, err := NormalizeHookDecl(HookDecl{
		Name:        "skill-defaults",
		Event:       HookSessionPostCreate,
		Source:      HookSourceSkill,
		Command:     "./hook.sh",
		SkillSource: HookSkillSourceUser,
	}, expectExecutorKind(t, HookExecutorSubprocess))
	if err != nil {
		t.Fatalf("NormalizeHookDecl() error = %v", err)
	}

	if hook.Priority != 0 {
		t.Fatalf("NormalizeHookDecl() priority = %d, want 0", hook.Priority)
	}
	if hook.Decl.ExecutorKind != HookExecutorSubprocess {
		t.Fatalf("NormalizeHookDecl() executor kind = %q, want %q", hook.Decl.ExecutorKind, HookExecutorSubprocess)
	}
}

func TestNormalizeHookDeclPreservesExplicitZeroPriority(t *testing.T) {
	t.Parallel()

	hook, err := NormalizeHookDecl(HookDecl{
		Name:        "config-explicit-zero",
		Event:       HookSessionPostCreate,
		Source:      HookSourceConfig,
		Priority:    0,
		PrioritySet: true,
		Command:     "./hook.sh",
	}, expectExecutorKind(t, HookExecutorSubprocess))
	if err != nil {
		t.Fatalf("NormalizeHookDecl() error = %v", err)
	}

	if hook.Priority != 0 {
		t.Fatalf("NormalizeHookDecl() priority = %d, want explicit 0", hook.Priority)
	}
}

func TestNormalizeHookDeclRequiresResolver(t *testing.T) {
	t.Parallel()

	_, err := NormalizeHookDecl(HookDecl{
		Name:   "no-resolver",
		Event:  HookSessionPostCreate,
		Source: HookSourceNative,
	}, nil)
	if err == nil {
		t.Fatal("NormalizeHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), ErrExecutorResolverRequired.Error()) {
		t.Fatalf("NormalizeHookDecl() error = %q, want resolver detail", err)
	}
}

func TestValidateHookDeclRejectsIllegalMatcherField(t *testing.T) {
	t.Parallel()

	err := ValidateHookDecl(HookDecl{
		Name:   "bad-matcher",
		Event:  HookSessionPostCreate,
		Source: HookSourceConfig,
		Matcher: HookMatcher{
			ToolName: "read_file",
		},
		Command: "./hook.sh",
	})
	if err == nil {
		t.Fatal("ValidateHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "tool_name") || !strings.Contains(err.Error(), string(HookSessionPostCreate)) {
		t.Fatalf("ValidateHookDecl() error = %q, want matcher field detail", err)
	}
}

func TestValidateHookDeclAllowsAutonomyMatcherFields(t *testing.T) {
	t.Parallel()

	for _, decl := range []HookDecl{
		{
			Name:    "coordinator",
			Event:   HookCoordinatorPreSpawn,
			Source:  HookSourceConfig,
			Command: "./hook.sh",
			Matcher: HookMatcher{
				WorkspaceID: "ws-1",
				Autonomy: &AutonomyMatcher{
					TaskID:               "task-*",
					RunID:                "run-1",
					ParticipationChannel: "coord-ch-1",
					CoordinatorSessionID: "coord-sess-1",
				},
			},
		},
		{
			Name:    "task-run",
			Event:   HookTaskRunPreClaim,
			Source:  HookSourceConfig,
			Command: "./hook.sh",
			Matcher: HookMatcher{
				WorkspaceID: "ws-1",
				Autonomy: &AutonomyMatcher{
					TaskID:               "task-1",
					RunID:                "run-*",
					ParticipationChannel: "coord-ch-1",
				},
			},
		},
		{
			Name:    "spawn",
			Event:   HookSpawnPreCreate,
			Source:  HookSourceConfig,
			Command: "./hook.sh",
			Matcher: HookMatcher{
				WorkspaceID: "ws-1",
				Autonomy: &AutonomyMatcher{
					ParentSessionID:      "parent-1",
					RootSessionID:        "root-1",
					ChildSessionID:       "child-*",
					SpawnRole:            "reviewer",
					ParticipationChannel: "coord-ch-1",
				},
			},
		},
	} {
		if err := ValidateHookDecl(decl); err != nil {
			t.Fatalf("ValidateHookDecl(%q) error = %v", decl.Name, err)
		}
	}
}

func TestValidateHookDeclRejectsIllegalAutonomyMatcherField(t *testing.T) {
	t.Parallel()

	err := ValidateHookDecl(HookDecl{
		Name:    "bad-task-run-matcher",
		Event:   HookTaskRunPreClaim,
		Source:  HookSourceConfig,
		Command: "./hook.sh",
		Matcher: HookMatcher{
			Autonomy: &AutonomyMatcher{ParentSessionID: "parent-1"},
		},
	})
	if err == nil {
		t.Fatal("ValidateHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parent_session_id") ||
		!strings.Contains(err.Error(), string(HookTaskRunPreClaim)) {
		t.Fatalf("ValidateHookDecl() error = %q, want autonomy matcher field detail", err)
	}
}

func TestNormalizeHookDeclClonesMutableFields(t *testing.T) {
	t.Parallel()

	env := map[string]string{"A": "1"}
	metadata := map[string]string{"team": "hooks"}
	args := []string{"--demo"}
	readOnly := true

	hook, err := NormalizeHookDecl(HookDecl{
		Name:     "clone-fields",
		Event:    HookToolPreCall,
		Source:   HookSourceConfig,
		Command:  "./hook.sh",
		Args:     args,
		Env:      env,
		Metadata: metadata,
		Matcher: HookMatcher{
			ToolReadOnly: &readOnly,
		},
		Timeout: 5 * time.Second,
	}, expectExecutorKind(t, HookExecutorSubprocess))
	if err != nil {
		t.Fatalf("NormalizeHookDecl() error = %v", err)
	}

	args[0] = "--changed"
	env["A"] = "2"
	metadata["team"] = "changed"
	readOnly = false

	if hook.Decl.Args[0] != "--demo" {
		t.Fatalf("NormalizeHookDecl() args = %#v, want cloned args", hook.Decl.Args)
	}
	if hook.Decl.Env["A"] != "1" {
		t.Fatalf("NormalizeHookDecl() env = %#v, want cloned env", hook.Decl.Env)
	}
	if hook.Decl.Metadata["team"] != "hooks" {
		t.Fatalf("NormalizeHookDecl() metadata = %#v, want cloned metadata", hook.Decl.Metadata)
	}
	if hook.Matcher.ToolReadOnly == nil || !*hook.Matcher.ToolReadOnly {
		t.Fatalf("NormalizeHookDecl() matcher read_only = %#v, want true clone", hook.Matcher.ToolReadOnly)
	}
}

func TestValidateAndNormalizeHookDecls(t *testing.T) {
	t.Parallel()

	decls := []HookDecl{
		{
			Name:   "first",
			Event:  HookSessionPostCreate,
			Source: HookSourceNative,
		},
		{
			Name:    "second",
			Event:   HookToolPreCall,
			Source:  HookSourceConfig,
			Command: "./hook.sh",
		},
	}

	if err := ValidateHookDecls(decls); err != nil {
		t.Fatalf("ValidateHookDecls() error = %v", err)
	}

	hooks, err := NormalizeHookDecls(decls, func(decl HookDecl) (Executor, error) {
		return stubExecutor{kind: decl.ExecutorKind}, nil
	})
	if err != nil {
		t.Fatalf("NormalizeHookDecls() error = %v", err)
	}
	if len(hooks) != len(decls) {
		t.Fatalf("NormalizeHookDecls() len = %d, want %d", len(hooks), len(decls))
	}
}

func TestNormalizeHookDeclsSkipsDisabledDeclarations(t *testing.T) {
	t.Parallel()

	disabled := false
	decls := []HookDecl{
		{
			Name:    "disabled",
			Event:   HookToolPreCall,
			Source:  HookSourceConfig,
			Command: "./disabled.sh",
			Enabled: &disabled,
		},
		{
			Name:    "enabled",
			Event:   HookToolPreCall,
			Source:  HookSourceConfig,
			Command: "./enabled.sh",
		},
	}

	hooks, err := NormalizeHookDecls(decls, func(decl HookDecl) (Executor, error) {
		return stubExecutor{kind: decl.ExecutorKind}, nil
	})
	if err != nil {
		t.Fatalf("NormalizeHookDecls() error = %v", err)
	}
	if len(hooks) != 1 || hooks[0].Name != "enabled" {
		t.Fatalf("NormalizeHookDecls() = %#v, want only enabled declaration", hooks)
	}
}

func TestValidateHookDeclRejectsNativeExecutorForNonNativeSource(t *testing.T) {
	t.Parallel()

	err := ValidateHookDecl(HookDecl{
		Name:         "config-native",
		Event:        HookSessionPostCreate,
		Source:       HookSourceConfig,
		ExecutorKind: HookExecutorNative,
	})
	if err == nil {
		t.Fatal("ValidateHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "native sources") {
		t.Fatalf("ValidateHookDecl() error = %q, want native-source detail", err)
	}
}

func TestValidateHookDeclRejectsSkillSourceOnNonSkillDeclaration(t *testing.T) {
	t.Parallel()

	err := ValidateHookDecl(HookDecl{
		Name:        "config-skill-source",
		Event:       HookSessionPostCreate,
		Source:      HookSourceConfig,
		Command:     "./hook.sh",
		SkillSource: HookSkillSourceWorkspace,
	})
	if err == nil {
		t.Fatal("ValidateHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "skill source is only valid") {
		t.Fatalf("ValidateHookDecl() error = %q, want skill-source detail", err)
	}
}

func TestDefaultHookPriorityRejectsUnknownSource(t *testing.T) {
	t.Parallel()

	if _, err := DefaultHookPriority(HookSource(99)); !errors.Is(err, ErrInvalidHookSource) {
		t.Fatalf("DefaultHookPriority() error = %v, want ErrInvalidHookSource", err)
	}
}

func TestValidateHookDeclRejectsInvalidMatcherPattern(t *testing.T) {
	t.Parallel()

	err := ValidateHookDecl(HookDecl{
		Name:    "bad-pattern",
		Event:   HookToolPreCall,
		Source:  HookSourceConfig,
		Command: "./hook.sh",
		Matcher: HookMatcher{
			ToolID: "[",
		},
	})
	if err == nil {
		t.Fatal("ValidateHookDecl() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "matcher.tool_id pattern") {
		t.Fatalf("ValidateHookDecl() error = %q, want matcher pattern detail", err)
	}
}

func expectExecutorKind(t *testing.T, kind HookExecutorKind) ExecutorResolver {
	t.Helper()

	return func(decl HookDecl) (Executor, error) {
		if decl.ExecutorKind != kind {
			t.Fatalf("executor resolver kind = %q, want %q", decl.ExecutorKind, kind)
		}
		return stubExecutor{kind: kind}, nil
	}
}
