package registry

import (
	"errors"
	"fmt"
	"testing"
)

func TestRegistrySourcePrimitives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "Should keep SourceCaps zero value search disabled",
			run: func(t *testing.T) {
				var caps SourceCaps
				if caps.Search {
					t.Fatal("SourceCaps zero value Search = true, want false")
				}
			},
		},
		{
			name: "Should match ErrNotSupported when wrapped",
			run: func(t *testing.T) {
				err := fmt.Errorf("wrapped: %w", ErrNotSupported)
				if !errors.Is(err, ErrNotSupported) {
					t.Fatalf("errors.Is(%v, ErrNotSupported) = false, want true", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}
