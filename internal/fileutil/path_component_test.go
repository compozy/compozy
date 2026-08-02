package fileutil

import (
	"errors"
	"testing"
)

// Invariant: every component admitted to a capability-rooted operation has a
// single portable filename interpretation on every supported platform.
// Owner: fileutil capability-rooted path boundary.
// Canonical suite: fileutil path-component unit behavior.
func TestValidatePathComponent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component string
		wantErr   bool
	}{
		{name: "Should accept an ordinary component", component: "agent-config.toml"},
		{name: "Should reject a path separator", component: "nested/child", wantErr: true},
		{name: "Should reject a Windows path separator on every platform", component: `nested\child`, wantErr: true},
		{name: "Should reject a colon", component: "alternate:stream", wantErr: true},
		{name: "Should reject a NUL byte", component: "agent\x00config", wantErr: true},
		{name: "Should reject invalid UTF-8", component: "agent\xffconfig", wantErr: true},
		{name: "Should reject a control character", component: "agent\nconfig", wantErr: true},
		{name: "Should reject a Windows reserved character", component: "agent?config", wantErr: true},
		{name: "Should reject a trailing dot", component: "agent.", wantErr: true},
		{name: "Should reject a trailing space", component: "agent ", wantErr: true},
		{name: "Should reject a DOS device basename", component: "con", wantErr: true},
		{name: "Should reject a console input device basename", component: "CONIN$", wantErr: true},
		{name: "Should reject a DOS device basename with extension", component: "Lpt9.json", wantErr: true},
		{name: "Should reject a DOS device basename with superscript digit", component: "COM¹.log", wantErr: true},
		{
			name:      "Should reject a DOS device basename with space before extension",
			component: "COM1 .json",
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePathComponent(test.component)
			if test.wantErr && !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("ValidatePathComponent(%q) error = %v, want ErrInvalidPath", test.component, err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ValidatePathComponent(%q) error = %v, want nil", test.component, err)
			}
		})
	}
}
