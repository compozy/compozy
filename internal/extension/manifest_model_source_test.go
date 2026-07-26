package extensionpkg

import (
	"errors"
	"testing"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

func TestManifestValidateModelSourceCapability(t *testing.T) {
	t.Run("Should accept normalizable model source capability", func(t *testing.T) {
		withDaemonVersion(t, "0.6.0")

		manifest := expectedManifest()
		manifest.Name = "OpenAI Models"
		manifest.Capabilities.Provides = []string{extensionprotocol.CapabilityProvideModelSource}

		if err := manifest.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("Should reject model source name without valid slug", func(t *testing.T) {
		withDaemonVersion(t, "0.6.0")

		manifest := expectedManifest()
		manifest.Name = "bad/source"
		manifest.Capabilities.Provides = []string{extensionprotocol.CapabilityProvideModelSource}

		err := manifest.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want ErrManifestInvalid")
		}
		if !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("Validate() error = %v, want ErrManifestInvalid", err)
		}
		var validationErr *ManifestValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("Validate() error = %T, want *ManifestValidationError", err)
		}
		if validationErr.Field != "name" {
			t.Fatalf("ManifestValidationError.Field = %q, want name", validationErr.Field)
		}
	})
}
