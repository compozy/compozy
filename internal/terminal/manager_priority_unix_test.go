//go:build !windows

package terminal

import (
	"testing"

	"github.com/compozy/compozy/internal/procutil"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
)

// Invariant: only agent-owned terminal groups receive configured POSIX niceness.
// Owner and canonical suite: terminal manager lifecycle; platform-specific process assertions.
func TestManagerProcessPriority(t *testing.T) {
	t.Run("Should lower only agent terminal process groups to the configured priority", func(t *testing.T) {
		inherited, err := procutil.CurrentProcessNiceness()
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range []ActorKind{ActorKindAgent, ActorKindHuman, ActorKindSystem} {
			t.Run("Should preserve priority ownership for "+string(kind), func(t *testing.T) {
				manager, _, _ := newTestManager(t, DefaultSettings(), WithPTY(terminalpty.New()), WithAgentNiceness(7))
				actor := Actor{Kind: kind, ID: "priority-owner", ProfileID: "profile-a"}
				if kind == ActorKindAgent {
					actor.SessionID, actor.RunID, actor.Generation = "session-a", "run-a", 1
				}
				handle, err := manager.OpenPipe(t.Context(), PipeRequest{
					WS: "workspace-a", Argv: []string{"sh", "-c", "sleep 300"},
					Actor: actor,
				})
				if err != nil {
					t.Fatal(err)
				}
				got, err := procutil.ProcessGroupNiceness(handle.(*session).proc.ProcessGroupID())
				want := inherited
				if kind == ActorKindAgent {
					want = max(7, inherited)
				}
				if err != nil || got != want {
					t.Fatalf("terminal process group niceness = %d, want %d, error=%v", got, want, err)
				}
			})
		}
	})
}
