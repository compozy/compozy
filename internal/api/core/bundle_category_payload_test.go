package core_test

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/core"
	bundlepkg "github.com/compozy/compozy/internal/bundles"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func TestBundleActivationPayloadCategoryPath(t *testing.T) {
	t.Parallel()

	t.Run("Should expose category path on bundle agents", func(t *testing.T) {
		t.Parallel()

		payload := core.BundleActivationPayload(bundlepkg.ActivationPreview{
			SpecDrift: true,
			Activation: bundlepkg.Activation{
				ID:            "act_marketing",
				Version:       7,
				ExtensionName: "marketing-team",
				BundleName:    "marketing",
				ProfileName:   "default",
				Scope:         bundlepkg.ScopeGlobal,
			},
			Bundle: extensionpkg.BundleSpec{Name: "marketing"},
			Profile: extensionpkg.BundleProfile{
				Name: "default",
				Agents: []extensionpkg.BundleAgent{{
					Path: "agents/planner",
					Agent: compozyconfig.AgentDef{
						Name:         "planner",
						Model:        "sonnet",
						CategoryPath: []string{"Marketing", "Planning"},
						Prompt:       "Plan campaign work.",
					},
				}},
			},
		})
		if len(payload.Agents) != 1 {
			t.Fatalf("len(payload.Agents) = %d, want 1", len(payload.Agents))
		}
		if got, want := payload.Version, int64(7); got != want {
			t.Fatalf("payload.Version = %d, want %d", got, want)
		}
		if got, want := strings.Join(payload.Agents[0].CategoryPath, ","), "Marketing,Planning"; got != want {
			t.Fatalf("payload.Agents[0].CategoryPath = %#v, want %q", payload.Agents[0].CategoryPath, want)
		}
		if !payload.SpecDrift {
			t.Fatal("payload.SpecDrift = false, want true")
		}
	})
}
