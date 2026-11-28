//go:build windows

package fileutil

import "testing"

// Regression: a candidate on another volume made filepath.Rel fail, so
// registering a workspace on a drive other than the Compozy home returned 500.
func TestPathWithinRootAcrossWindowsVolumes(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		root      string
		candidate string
		want      bool
	}{
		{
			name:      "Should reject a candidate on another drive",
			root:      `D:\Projects`,
			candidate: `C:\Users\me\.compozy\profiles\default`,
			want:      false,
		},
		{
			name:      "Should reject a drive candidate under a UNC root",
			root:      `\\server\share\projects`,
			candidate: `C:\projects\app`,
			want:      false,
		},
		{
			name:      "Should reject a candidate on another UNC share",
			root:      `\\server\share\projects`,
			candidate: `\\server\other\projects\app`,
			want:      false,
		},
		{
			name:      "Should accept a descendant with a different drive letter case",
			root:      `c:\Projects`,
			candidate: `C:\Projects\app`,
			want:      true,
		},
		{
			name:      "Should reject a sibling on the same drive",
			root:      `C:\Projects`,
			candidate: `C:\Projects-other`,
			want:      false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := PathWithinRoot(testCase.root, testCase.candidate)
			if err != nil {
				t.Fatalf("PathWithinRoot() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("PathWithinRoot() = %t, want %t", got, testCase.want)
			}
		})
	}
}
