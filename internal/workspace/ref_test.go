package workspace

import (
	"testing"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestParseRef(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, raw string
		want      Ref
	}{
		{"Should select User explicitly", workspaceaccess.UserSelector, Ref{Kind: RefUser}},
		{"Should preserve legacy user name", "user", Ref{Kind: RefWorkspace, Value: "user"}},
		{"Should preserve workspace id", "ws_1", Ref{Kind: RefWorkspace, Value: "ws_1"}},
		{"Should preserve workspace path", "~/x", Ref{Kind: RefWorkspace, Value: "~/x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ParseRef(tc.raw); got != tc.want {
				t.Fatalf("ParseRef(%q) = %#v, want %#v", tc.raw, got, tc.want)
			}
		})
	}
}
