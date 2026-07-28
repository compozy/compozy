package config

import "testing"

func TestCloneAgentDefCategoryPath(t *testing.T) {
	t.Parallel()

	t.Run("Should deep copy category path", func(t *testing.T) {
		t.Parallel()

		source := AgentDef{
			Name:         "coder",
			Provider:     "claude",
			CategoryPath: []string{"Marketing", "Sales"},
			Skills:       AgentSkillsConfig{Disabled: []string{"one"}},
			Prompt:       "Prompt.",
		}
		cloned := CloneAgentDef(source)
		source.CategoryPath[0] = "Changed"
		source.Skills.Disabled[0] = "changed"

		if !equalStringSlicesForTest(cloned.CategoryPath, []string{"Marketing", "Sales"}) {
			t.Fatalf("CloneAgentDef() CategoryPath = %#v", cloned.CategoryPath)
		}
		if !equalStringSlicesForTest(cloned.Skills.Disabled, []string{"one"}) {
			t.Fatalf("CloneAgentDef() Skills.Disabled = %#v", cloned.Skills.Disabled)
		}
	})
}
