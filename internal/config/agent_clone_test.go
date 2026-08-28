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
			Name:         "coder",
			Provider:     "claude",
			CategoryPath: []string{"Marketing", "Sales"},
			Skills:       AgentSkillsConfig{Disabled: []string{"one"}},
			Prompt:       "Prompt.",
		}
		source.SetSpeed(speedpkg.SpeedFast)
		source.SetACPOptions([]ACPOptionSelection{
			{ID: "thinking", BoolValue: new(true)},
			{ID: "context", ValueID: "1m"},
		})
		cloned := CloneAgentDef(source)
		source.CategoryPath[0] = "Changed"
		source.Skills.Disabled[0] = "changed"
		sourceOptions := source.ACPOptionsValue()
		*sourceOptions[0].BoolValue = false
		sourceOptions[1].ValueID = "changed"

		if !equalStringSlicesForTest(cloned.CategoryPath, []string{"Marketing", "Sales"}) {
			t.Fatalf("CloneAgentDef() CategoryPath = %#v", cloned.CategoryPath)
		}
		if !equalStringSlicesForTest(cloned.Skills.Disabled, []string{"one"}) {
			t.Fatalf("CloneAgentDef() Skills.Disabled = %#v", cloned.Skills.Disabled)
		}
		clonedOptions := cloned.ACPOptionsValue()
		if cloned.SpeedValue() != speedpkg.SpeedFast || len(clonedOptions) != 2 ||
			clonedOptions[0].BoolValue == nil || !*clonedOptions[0].BoolValue ||
			clonedOptions[1].ValueID != "1m" {
			t.Fatalf("CloneAgentDef() runtime defaults = %#v", cloned)
		}
	})
}
