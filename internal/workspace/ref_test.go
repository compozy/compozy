package workspace

import "testing"

func TestParseWorkspaceRef(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, raw string
		want      WorkspaceRef
	}{
		{"Should select User explicitly", "@user", WorkspaceRef{Kind: RefUser}},
		{"Should preserve legacy user name", "user", WorkspaceRef{Kind: RefWorkspace, Value: "user"}},
		{"Should preserve workspace id", "ws_1", WorkspaceRef{Kind: RefWorkspace, Value: "ws_1"}},
		{"Should preserve workspace path", "~/x", WorkspaceRef{Kind: RefWorkspace, Value: "~/x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ParseWorkspaceRef(tc.raw); got != tc.want {
				t.Fatalf("ParseWorkspaceRef(%q) = %#v, want %#v", tc.raw, got, tc.want)
			}
		})
	}
}
