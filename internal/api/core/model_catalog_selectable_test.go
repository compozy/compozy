package core

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
)

func TestSelectableProviderModels(t *testing.T) {
	t.Parallel()

	t.Run("Should drop models of disabled providers only", func(t *testing.T) {
		t.Parallel()
		cfg := &compozyconfig.Config{Providers: map[string]compozyconfig.ProviderConfig{
			"gemini": {Enabled: new(false)},
		}}
		models := []modelcatalog.Model{
			{ProviderID: "gemini", ModelID: "gemini-pro"},
			{ProviderID: "codex", ModelID: "gpt"},
		}
		got := SelectableProviderModels(cfg, models)
		if len(got) != 1 || got[0].ProviderID != "codex" {
			t.Fatalf("SelectableProviderModels() = %#v, want only codex", got)
		}
	})
}
