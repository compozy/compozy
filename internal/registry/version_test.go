package registry

import "testing"

func TestVersionIsNewer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "semver newer", current: "1.1.0", latest: "1.2.0", want: true},
		{name: "semver older", current: "1.2.0", latest: "1.1.0", want: false},
		{name: "prerelease older than release", current: "1.0.0-beta", latest: "1.0.0", want: true},
		{name: "release not older than prerelease", current: "1.0.0", latest: "1.0.0-beta", want: false},
		{name: "numeric prerelease comparison", current: "1.0.0-beta.1", latest: "1.0.0-beta.2", want: true},
		{name: "lexical prerelease comparison", current: "1.0.0-alpha", latest: "1.0.0-beta", want: true},
		{name: "numeric prerelease not newer", current: "1.0.0-beta.10", latest: "1.0.0-beta.2", want: false},
		{name: "empty current with valid latest", current: "", latest: "1.0.0", want: true},
		{name: "invalid latest", current: "1.0.0", latest: "banana", want: false},
		{name: "invalid current", current: "banana", latest: "1.0.1", want: false},
		{name: "malformed latest with extra core segment", current: "1.2.3", latest: "1.2.3.1", want: false},
		{name: "malformed latest with leading zero core segment", current: "1.2.3", latest: "1.02.3", want: false},
		{name: "malformed latest with negative core segment", current: "1.2.3", latest: "1.2.-1", want: false},
		{
			name:    "malformed latest with leading zero prerelease segment",
			current: "1.2.3",
			latest:  "1.2.4-alpha.01",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := VersionIsNewer(tt.current, tt.latest); got != tt.want {
				t.Fatalf("VersionIsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}
