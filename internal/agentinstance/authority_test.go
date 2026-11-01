package agentinstance

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type callerReaderStub struct{ caller Caller }

var _ CallerReader = callerReaderStub{}

func (r callerReaderStub) InstanceCaller(context.Context, string) (Caller, error) {
	return r.caller, nil
}

func TestInstanceAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, verb, reason, callerInstance string
		allowed                            bool
	}{
		{"Should allow own reports", "reports", "", "own", true},
		{"Should reject other instance reads", "reports", "", "other", false},
		{"Should reject manual resume", "resume", "manual", "own", false},
		{"Should reject fatigue resume", "resume", "fatigue", "own", false},
		{"Should allow definition pause resume", "resume", "definition_missing", "own", true},
		{"Should reject agent adoption", "adopt", "", "own", false},
		{"Should reject enabling checks", "checks_on", "", "own", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authority := Authority{Callers: callerReaderStub{Caller{InstanceID: tc.callerInstance, AgentName: "coder", ProfileID: "profile"}}}
			err := authority.Authorize(t.Context(), workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: "caller"}, Instance{ID: "own", AgentName: "coder", ProfileID: "profile", PauseReason: tc.reason}, tc.verb)
			if (err == nil) != tc.allowed {
				t.Fatalf("Authorize: %v", err)
			}
			if !tc.allowed && !errors.Is(err, ErrForbidden) {
				t.Fatalf("wrong denial: %v", err)
			}
		})
	}
	t.Run("Should intersect all permission categories without widening the child", func(t *testing.T) {
		t.Parallel()
		child := store.SessionPermissionPolicy{Tools: []string{"tool__read"}, Skills: []string{"read"}, MCPServers: []string{"docs"}, WorkspacePaths: []string{"/one"}}
		definition := store.SessionPermissionPolicy{Tools: []string{"tool__read", "tool__write"}, Skills: []string{"read", "write"}, MCPServers: []string{"docs", "admin"}, WorkspacePaths: []string{"/one", "/two"}}
		if got := IntersectPermissions(child, definition); !reflect.DeepEqual(got, child) {
			t.Fatalf("intersection: %#v", got)
		}
	})
}
