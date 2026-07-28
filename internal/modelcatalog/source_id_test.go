package modelcatalog

import (
	"strings"
	"testing"
)

func TestValidateSourceID(t *testing.T) {
	t.Parallel()

	t.Run("Should reject surrounding whitespace", func(t *testing.T) {
		t.Parallel()

		err := ValidateSourceID(" builtin ")
		if err == nil || !strings.Contains(err.Error(), "surrounding whitespace") {
			t.Fatalf("ValidateSourceID() error = %v, want surrounding whitespace error", err)
		}
	})
}

func TestValidateSourceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject surrounding whitespace in source id", func(t *testing.T) {
		t.Parallel()

		err := ValidateSourceIdentity(" extension:demo ", SourceKindExtension)
		if err == nil || !strings.Contains(err.Error(), "surrounding whitespace") {
			t.Fatalf("ValidateSourceIdentity() error = %v, want surrounding whitespace error", err)
		}
	})
}
