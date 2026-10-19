package modelcatalog

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestOpenCodeOriginFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("Should change fingerprint on origin-only change", func(t *testing.T) {
		t.Parallel()
		builtin := compozyconfig.BuiltinProviders()["opencode"]
		authored := builtin
		authored.CommandIsBuiltin = false
		workdir := t.TempDir()
		builtinSource := newLiveSourceForTest(t, "opencode", builtin, &LiveProviderSourcesConfig{WorkingDir: workdir})
		authoredSource := newLiveSourceForTest(t, "opencode", authored, &LiveProviderSourcesConfig{WorkingDir: workdir})
		builtinFingerprint, err := builtinSource.CatalogExecutionFingerprint()
		if err != nil {
			t.Fatalf("builtin fingerprint error = %v", err)
		}
		authoredFingerprint, err := authoredSource.CatalogExecutionFingerprint()
		if err != nil {
			t.Fatalf("authored fingerprint error = %v", err)
		}
		if builtinFingerprint == "" || builtinFingerprint == authoredFingerprint {
			t.Fatalf(
				"fingerprints builtin=%q authored=%q, want distinct non-empty",
				builtinFingerprint,
				authoredFingerprint,
			)
		}
		clone, changed, err := builtinSource.CloneWithProvider(authored)
		if err != nil {
			t.Fatalf("CloneWithProvider() error = %v", err)
		}
		if !changed {
			t.Fatal("origin-only provider change reused discovery identity")
		}
		cloneFingerprint, err := clone.CatalogExecutionFingerprint()
		if err != nil {
			t.Fatalf("clone fingerprint error = %v", err)
		}
		if cloneFingerprint != authoredFingerprint {
			t.Fatalf("clone fingerprint = %q, want %q", cloneFingerprint, authoredFingerprint)
		}
	})
}
