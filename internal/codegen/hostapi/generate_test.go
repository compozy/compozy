package hostapi

import (
	"bytes"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Should render every catalog method into every derived registry", func(t *testing.T) {
		t.Parallel()

		methods, err := loadCatalog()
		if err != nil {
			t.Fatalf("loadCatalog() error = %v", err)
		}
		result, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if got, want := result.MethodCount, len(methods); got != want {
			t.Fatalf("MethodCount = %d, want %d", got, want)
		}
		if got, want := len(result.Files), len(familyOrder)+3; got != want {
			t.Fatalf("len(Files) = %d, want %d", got, want)
		}
		for _, entry := range methods {
			if !bytes.Contains(result.Files[ProtocolPath], []byte(entry.Constant)) {
				t.Fatalf("protocol output missing constant %q", entry.Constant)
			}
			if !bytes.Contains(result.Files[AliasesPath], []byte(entry.Constant)) {
				t.Fatalf("alias output missing constant %q", entry.Constant)
			}
			familyPath := registryFamilyPath(entry.Family)
			if !bytes.Contains(result.Files[familyPath], []byte(entry.Wire)) {
				t.Fatalf("family output %q missing wire method %q", familyPath, entry.Wire)
			}
		}
	})

	t.Run("Should reject duplicate wire identities", func(t *testing.T) {
		t.Parallel()

		methods, err := loadCatalog()
		if err != nil {
			t.Fatalf("loadCatalog() error = %v", err)
		}
		duplicate := append([]method(nil), methods...)
		duplicate[1].Wire = duplicate[0].Wire
		if err := validateCatalog(duplicate); err == nil {
			t.Fatal("validateCatalog() error = nil, want duplicate wire failure")
		}
	})
}
