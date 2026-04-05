package tasks

import "testing"

func TestRemapLegacyTaskTypeUsesRegistryAsAuthority(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry([]string{"backend", "refactor"})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	tests := []struct {
		name    string
		rawType string
		want    string
	}{
		{
			name:    "explicit remap allowed by registry",
			rawType: "Refactor",
			want:    "refactor",
		},
		{
			name:    "explicit remap rejected when registry disallows mapped slug",
			rawType: "Documentation",
			want:    "",
		},
		{
			name:    "case insensitive passthrough allowed by registry",
			rawType: "Backend",
			want:    "backend",
		},
		{
			name:    "manual legacy type still returns empty",
			rawType: "Feature Implementation",
			want:    "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := RemapLegacyTaskType(tt.rawType, registry); got != tt.want {
				t.Fatalf("unexpected remap for %q\nwant: %q\ngot:  %q", tt.rawType, tt.want, got)
			}
		})
	}
}
