package terminal

// Suite: terminal capability resolution.
// Invariant: platform/workspace support is reported honestly and recording is derived from interactivity.
// Boundary IN: platform and workspace kind. Boundary OUT: terminal capability value.

import "testing"

func TestResolveCapabilities(t *testing.T) { // IT-016
	t.Parallel()

	for _, testCase := range []struct {
		name          string
		goos          string
		workspaceKind string
		interactive   bool
	}{
		{name: "Should allow a local Unix terminal", goos: "darwin", workspaceKind: WorkspaceKindLocal, interactive: true},
		{name: "Should keep Windows execute-only before ConPTY", goos: "windows", workspaceKind: WorkspaceKindLocal},
		{name: "Should keep remote workspaces execute-only", goos: "linux", workspaceKind: "sandbox"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			capabilities := ResolveCapabilities(testCase.goos, testCase.workspaceKind)
			if capabilities.Interactive != testCase.interactive {
				t.Fatalf("ResolveCapabilities(%q, %q) = %#v", testCase.goos, testCase.workspaceKind, capabilities)
			}
			if RecordingAvailable(capabilities) != testCase.interactive {
				t.Fatalf("RecordingAvailable(%#v) did not derive from interactivity", capabilities)
			}
		})
	}
}
