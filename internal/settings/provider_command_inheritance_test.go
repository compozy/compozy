package settings

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestProviderSettingsCommandInheritance(t *testing.T) {
	t.Parallel()

	t.Run("Should flag the inherited builtin command", func(t *testing.T) {
		t.Parallel()

		builtin := compozyconfig.BuiltinProviders()["opencode"]
		inherited := providerSettingsBaseFromConfig("opencode", builtin)
		if !inherited.CommandInherited {
			t.Fatalf(
				"CommandInherited = false for the builtin %q, want the inherited default",
				inherited.Command,
			)
		}
	})

	t.Run("Should not flag an authored command", func(t *testing.T) {
		t.Parallel()

		authored := compozyconfig.BuiltinProviders()["opencode"]
		authored.Command = "opencode-wrapper acp"
		authored.CommandIsBuiltin = false
		pinned := providerSettingsBaseFromConfig("opencode", authored)
		if pinned.CommandInherited {
			t.Fatalf("CommandInherited = true for the authored %q, want an override", pinned.Command)
		}
	})
}
