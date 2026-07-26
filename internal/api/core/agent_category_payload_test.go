package core_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestAgentPayloadCategoryPath(t *testing.T) {
	t.Parallel()

	t.Run("Should copy category path from agent definition", func(t *testing.T) {
		t.Parallel()

		payload := core.AgentPayloadFromDef(aghconfig.AgentDef{
			Name:         "coder",
			Provider:     "fake",
			CategoryPath: []string{"Marketing", "Sales"},
			Prompt:       "hello",
		})
		if got, want := payload.CategoryPath, []string{"Marketing", "Sales"}; !slices.Equal(got, want) {
			t.Fatalf("payload category_path = %#v, want %#v", got, want)
		}
	})

	t.Run("Should defensively copy category path", func(t *testing.T) {
		t.Parallel()

		agent := aghconfig.AgentDef{
			Name:         "coder",
			Provider:     "fake",
			CategoryPath: []string{"Marketing", "Sales"},
			Prompt:       "hello",
		}
		payload := core.AgentPayloadFromDef(agent)
		agent.CategoryPath[0] = "Changed"
		if got, want := payload.CategoryPath, []string{"Marketing", "Sales"}; !slices.Equal(got, want) {
			t.Fatalf("payload category_path = %#v, want %#v", got, want)
		}
	})

	t.Run("Should omit category path for diagnostic payload", func(t *testing.T) {
		t.Parallel()

		payload := core.AgentPayloadFromDiagnostic(workspacepkg.AgentDiagnostic{
			Name:      "broken",
			Path:      "/tmp/AGENT.md",
			ErrorKind: "parse",
			Message:   "invalid",
		})
		if payload.CategoryPath != nil {
			t.Fatalf("payload category_path = %#v, want nil", payload.CategoryPath)
		}
	})

	t.Run("Should omit nil category path in JSON", func(t *testing.T) {
		t.Parallel()

		raw, err := json.Marshal(contract.AgentPayload{Name: "coder", Provider: "fake", Prompt: "hello"})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if strings.Contains(string(raw), "category_path") {
			t.Fatalf("json = %s, want category_path omitted", string(raw))
		}
	})
}
