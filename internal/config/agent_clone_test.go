package config

import (
	"testing"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestCloneAgentDefCategoryPath(t *testing.T) {
	t.Parallel()

	t.Run("Should deep copy category path", func(t *testing.T) {
		t.Parallel()

		source := AgentDef{
			Name:     "coder",
			Provider: "claude",
			Speed:    speedpkg.SpeedFast,
			ACPOptions: []ACPOptionSelection{
				{ID: "thinking", BoolValue: new(true)},
				{ID: "context", ValueID: "1m"},
			},
			CategoryPath: []string{"Marketing", "Sales"},
			Skills:       AgentSkillsConfig{Disabled: []string{"one"}},
			Prompt:       "Prompt.",
		}
		cloned := CloneAgentDef(source)
		source.CategoryPath[0] = "Changed"
		source.Skills.Disabled[0] = "changed"
		*source.ACPOptions[0].BoolValue = false
		source.ACPOptions[1].ValueID = "changed"

		if !equalStringSlicesForTest(cloned.CategoryPath, []string{"Marketing", "Sales"}) {
			t.Fatalf("CloneAgentDef() CategoryPath = %#v", cloned.CategoryPath)
		}
		if !equalStringSlicesForTest(cloned.Skills.Disabled, []string{"one"}) {
			t.Fatalf("CloneAgentDef() Skills.Disabled = %#v", cloned.Skills.Disabled)
		}
		if cloned.Speed != speedpkg.SpeedFast || len(cloned.ACPOptions) != 2 ||
			cloned.ACPOptions[0].BoolValue == nil || !*cloned.ACPOptions[0].BoolValue ||
			cloned.ACPOptions[1].ValueID != "1m" {
			t.Fatalf("CloneAgentDef() runtime defaults = %#v", cloned)
		}
	})
}
