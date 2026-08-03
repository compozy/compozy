package extensionpkg

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrapResourceValidationError(t *testing.T) {
	t.Parallel()

	cause := errors.New("invalid declaration")
	tests := []struct {
		name     string
		path     string
		err      error
		wantPath string
	}{
		{
			name:     "Should position an unwrapped resource error",
			path:     "outer.toml",
			err:      cause,
			wantPath: "outer.toml",
		},
		{
			name: "Should preserve the inner resource path through contextual wrapping",
			path: "outer.toml",
			err: fmt.Errorf(
				"extension: validate resources: %w",
				wrapResourceValidationError("inner.toml", cause),
			),
			wantPath: "inner.toml",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := wrapResourceValidationError(test.path, test.err)
			var positioned *resourceValidationError
			if !errors.As(got, &positioned) {
				t.Fatalf("wrapResourceValidationError() error = %T, want *resourceValidationError", got)
			}
			if positioned.Path != test.wantPath {
				t.Fatalf("wrapResourceValidationError().Path = %q, want %q", positioned.Path, test.wantPath)
			}
			if !errors.Is(got, cause) {
				t.Fatalf("wrapResourceValidationError() error = %v, want wrapped cause", got)
			}
		})
	}
}
