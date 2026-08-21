package hooks

import (
	"errors"
	"testing"
)

func TestSortResolvedHooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		assert func(*testing.T)
	}{
		{
			name: "Should order hooks by source precedence",
			assert: func(t *testing.T) {
				hooks := []*ResolvedHook{
					testResolvedHook("skill", HookSourceSkill, 0, HookSkillSourceUser),
					testResolvedHook("agent", HookSourceAgentDefinition, 100, HookSkillSourceUnset),
					testResolvedHook("extension", HookSourceExtension, 300, HookSkillSourceUnset),
					testResolvedHook("config", HookSourceConfig, 500, HookSkillSourceUnset),
					testResolvedHook("native", HookSourceNative, 1000, HookSkillSourceUnset),
				}

				SortResolvedHooks(hooks)

				assertHookNames(t, hooks, []string{"native", "config", "extension", "agent", "skill"})
			},
		},
		{
			name: "Should order hooks by priority then name",
			assert: func(t *testing.T) {
				hooks := []*ResolvedHook{
					testResolvedHook("charlie", HookSourceConfig, 500, HookSkillSourceUnset),
					testResolvedHook("bravo", HookSourceConfig, 900, HookSkillSourceUnset),
					testResolvedHook("alpha", HookSourceConfig, 500, HookSkillSourceUnset),
				}

				SortResolvedHooks(hooks)

				assertHookNames(t, hooks, []string{"bravo", "alpha", "charlie"})
			},
		},
		{
			name: "Should order skill hooks by skill source before name",
			assert: func(t *testing.T) {
				hooks := []*ResolvedHook{
					testResolvedHook("workspace-skill", HookSourceSkill, 0, HookSkillSourceWorkspace),
					testResolvedHook("additional-skill", HookSourceSkill, 0, HookSkillSourceAdditional),
					testResolvedHook("user-skill", HookSourceSkill, 0, HookSkillSourceUser),
					testResolvedHook("profile-skill", HookSourceSkill, 0, HookSkillSourceProfile),
					testResolvedHook("marketplace-skill", HookSourceSkill, 0, HookSkillSourceMarketplace),
					testResolvedHook("bundled-skill", HookSourceSkill, 0, HookSkillSourceBundled),
					testResolvedHook("workspace-profile-skill", HookSourceSkill, 0, HookSkillSourceWorkspaceProfile),
				}

				SortResolvedHooks(hooks)

				assertHookNames(t, hooks, []string{
					"bundled-skill",
					"marketplace-skill",
					"user-skill",
					"profile-skill",
					"additional-skill",
					"workspace-skill",
					"workspace-profile-skill",
				})
			},
		},
		{
			name: "Should remain stable across repeated sorts",
			assert: func(t *testing.T) {
				first := testResolvedHook("same", HookSourceConfig, 500, HookSkillSourceUnset)
				second := testResolvedHook("same", HookSourceConfig, 500, HookSkillSourceUnset)
				hooks := []*ResolvedHook{first, second}

				SortResolvedHooks(hooks)
				SortResolvedHooks(hooks)

				if hooks[0] != first || hooks[1] != second {
					t.Fatalf("SortResolvedHooks() order = %#v, want stable original order", hooks)
				}
			},
		},
		{
			name: "Should return a sorted copy without mutating the original slice",
			assert: func(t *testing.T) {
				original := []*ResolvedHook{
					testResolvedHook("skill", HookSourceSkill, 0, HookSkillSourceUser),
					testResolvedHook("native", HookSourceNative, 1000, HookSkillSourceUnset),
				}

				ordered := OrderedResolvedHooks(original)

				assertHookNames(t, ordered, []string{"native", "skill"})
				assertHookNames(t, original, []string{"skill", "native"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.assert(t)
		})
	}
}

func TestDefaultHookPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source HookSource
		want   int32
	}{
		{source: HookSourceNative, want: 1000},
		{source: HookSourceConfig, want: 500},
		{source: HookSourceExtension, want: 300},
		{source: HookSourceAgentDefinition, want: 100},
		{source: HookSourceSkill, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.source.String(), func(t *testing.T) {
			t.Parallel()

			got, err := DefaultHookPriority(tt.source)
			if err != nil {
				t.Fatalf("DefaultHookPriority() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("DefaultHookPriority() = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("Should reject unknown sources with a sentinel error", func(t *testing.T) {
		t.Parallel()

		_, err := DefaultHookPriority(HookSource(99))
		if !errors.Is(err, ErrInvalidHookSource) {
			t.Fatalf("DefaultHookPriority() error = %v, want ErrInvalidHookSource", err)
		}
	})
}

func testResolvedHook(name string, source HookSource, priority int, skillSource HookSkillSource) *ResolvedHook {
	kind := HookExecutorSubprocess
	command := "./hook.sh"
	if source == HookSourceNative {
		kind = HookExecutorNative
		command = ""
	}

	return &ResolvedHook{
		RegisteredHook: RegisteredHook{
			Name:     name,
			Event:    HookSessionPostCreate,
			Source:   source,
			Mode:     HookModeAsync,
			Priority: int32(priority),
			Executor: stubExecutor{kind: kind},
		},
		Decl: HookDecl{
			Name:         name,
			Event:        HookSessionPostCreate,
			Source:       source,
			Mode:         HookModeAsync,
			Priority:     int32(priority),
			PrioritySet:  true,
			ExecutorKind: kind,
			Command:      command,
			SkillSource:  skillSource,
		},
	}
}

func assertHookNames(t *testing.T, hooks []*ResolvedHook, want []string) {
	t.Helper()

	if len(hooks) != len(want) {
		t.Fatalf("hook count = %d, want %d", len(hooks), len(want))
	}

	for idx, hook := range hooks {
		if hook.Name != want[idx] {
			t.Fatalf("hook[%d] name = %q, want %q", idx, hook.Name, want[idx])
		}
	}
}
