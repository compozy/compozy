package workspace

import (
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestCloneAgentDefs(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve category path", func(t *testing.T) {
		t.Parallel()

		cloned := cloneAgentDefs([]aghconfig.AgentDef{{
			Name:         "coder",
			Provider:     "claude",
			CategoryPath: []string{"Marketing", "Sales"},
			Prompt:       "Prompt.",
		}})
		if len(cloned) != 1 {
			t.Fatalf("len(cloneAgentDefs()) = %d, want 1", len(cloned))
		}
		if got, want := stringsForTest(cloned[0].CategoryPath), "Marketing,Sales"; got != want {
			t.Fatalf("CategoryPath = %#v, want %q", cloned[0].CategoryPath, want)
		}
	})

	t.Run("Should preserve skills", func(t *testing.T) {
		t.Parallel()

		cloned := cloneAgentDefs([]aghconfig.AgentDef{{
			Name:   "coder",
			Prompt: "Prompt.",
			Skills: aghconfig.AgentSkillsConfig{
				Disabled: []string{"build-site"},
			},
		}})
		if len(cloned) != 1 {
			t.Fatalf("len(cloneAgentDefs()) = %d, want 1", len(cloned))
		}
		if got, want := stringsForTest(cloned[0].Skills.Disabled), "build-site"; got != want {
			t.Fatalf("Skills.Disabled = %#v, want %q", cloned[0].Skills.Disabled, want)
		}
	})
}

func stringsForTest(values []string) string {
	if len(values) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(values[0])
	for _, value := range values[1:] {
		out.WriteString("," + value)
	}
	return out.String()
}
