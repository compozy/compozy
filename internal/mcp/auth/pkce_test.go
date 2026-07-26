package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPKCEGenerationWrapsRandomFailures(t *testing.T) {
	t.Parallel()

	root := errors.New("entropy unavailable")
	reader := failingReader{err: root}

	t.Run("Should wrap verifier generation failures", func(t *testing.T) {
		t.Parallel()

		_, err := newPKCEPair(reader)
		if !errors.Is(err, root) || !strings.Contains(err.Error(), "generate PKCE verifier") {
			t.Fatalf("newPKCEPair(failing reader) error = %v, want verifier context wrapping root", err)
		}
	})

	t.Run("Should wrap state generation failures", func(t *testing.T) {
		t.Parallel()

		_, err := newState(reader)
		if !errors.Is(err, root) || !strings.Contains(err.Error(), "generate oauth state") {
			t.Fatalf("newState(failing reader) error = %v, want state context wrapping root", err)
		}
	})
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}
