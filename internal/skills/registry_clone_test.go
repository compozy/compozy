package skills

import (
	"testing"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestCloneSkillHookSecretEnvContract(t *testing.T) {
	t.Parallel()

	t.Run("Should deep copy hook secret environment maps", func(t *testing.T) {
		t.Parallel()

		original := &Skill{
			Meta: SkillMeta{Name: "clone-secret-env", Description: "Clone hook secret env"},
			Hooks: []hookspkg.HookDecl{{
				Name:      "secret-env",
				Event:     hookspkg.HookSessionPostStop,
				Source:    hookspkg.HookSourceSkill,
				Mode:      hookspkg.HookModeAsync,
				Command:   "hook",
				SecretEnv: map[string]string{"TOKEN": "env:TOKEN"},
			}},
		}

		clone := cloneSkill(original)
		if clone == nil {
			t.Fatal("cloneSkill() = nil, want cloned skill")
		}
		if len(clone.Hooks) != 1 {
			t.Fatalf("cloneSkill() len(Hooks) = %d, want 1", len(clone.Hooks))
		}

		clone.Hooks[0].SecretEnv["TOKEN"] = "env:CLONE_TOKEN"
		clone.Hooks[0].SecretEnv["NEW_TOKEN"] = "env:NEW_TOKEN"

		if got, want := original.Hooks[0].SecretEnv["TOKEN"], "env:TOKEN"; got != want {
			t.Fatalf("original hook SecretEnv TOKEN mutated to %q, want %q", got, want)
		}
		if _, ok := original.Hooks[0].SecretEnv["NEW_TOKEN"]; ok {
			t.Fatal("original hook SecretEnv gained NEW_TOKEN from clone mutation")
		}
	})
}
