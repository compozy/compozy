package e2e

import (
	"slices"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestWriteAgentDefCategoryPath(t *testing.T) {
	t.Parallel()

	t.Run("Should persist category path", func(t *testing.T) {
		t.Parallel()

		homePaths := NewHomePaths(t)
		WriteAgentDef(t, homePaths, AgentSeed{
			Name:         "builder",
			Provider:     "fake",
			CategoryPath: []string{"Engineering", "Tools"},
			Prompt:       "You are a builder.",
		})

		agent, err := aghconfig.LoadAgentDef("builder", homePaths)
		if err != nil {
			t.Fatalf("LoadAgentDef(builder) error = %v", err)
		}
		if got, want := agent.CategoryPath, []string{"Engineering", "Tools"}; !slices.Equal(got, want) {
			t.Fatalf("agent.CategoryPath = %#v, want %#v", got, want)
		}
	})
}
