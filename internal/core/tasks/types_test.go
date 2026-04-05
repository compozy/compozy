package tasks

import (
	"slices"
	"strings"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	defaults := slices.Clone(BuiltinTypes)
	slices.Sort(defaults)

	tests := []struct {
		name       string
		configured []string
		wantValues []string
		wantErr    string
	}{
		{
			name:       "uses builtins when config is nil",
			configured: nil,
			wantValues: defaults,
		},
		{
			name:       "sorts configured values",
			configured: []string{"frontend", "backend"},
			wantValues: []string{"backend", "frontend"},
		},
		{
			name:       "rejects duplicates",
			configured: []string{"frontend", "frontend"},
			wantErr:    `duplicate task type "frontend"`,
		},
		{
			name:       "rejects invalid slugs",
			configured: []string{"Invalid Slug"},
			wantErr:    `Invalid Slug`,
		},
		{
			name:       "rejects explicit empty lists",
			configured: []string{},
			wantErr:    "task type list cannot be empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry, err := NewRegistry(tt.configured)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("unexpected error\nwant substring: %q\ngot: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("new registry: %v", err)
			}
			if got := registry.Values(); !slices.Equal(got, tt.wantValues) {
				t.Fatalf("unexpected values\nwant: %#v\ngot:  %#v", tt.wantValues, got)
			}
		})
	}
}

func TestTypeRegistryIsAllowed(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry([]string{"frontend", "backend"})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	tests := []struct {
		name string
		slug string
		want bool
	}{
		{name: "allows configured slug", slug: "backend", want: true},
		{name: "rejects unknown slug", slug: "nope", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := registry.IsAllowed(tt.slug); got != tt.want {
				t.Fatalf("unexpected IsAllowed(%q): got %v want %v", tt.slug, got, tt.want)
			}
		})
	}
}
