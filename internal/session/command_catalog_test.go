package session

import (
	"testing"

	commandpkg "github.com/compozy/compozy/internal/command"
)

// Canonical suite: session-owned runtime availability over the command catalog.
func TestWorktreeCommandAvailability(t *testing.T) {
	t.Parallel()

	base, err := commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), nil, nil)
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}
	for _, test := range []struct {
		name       string
		info       *Info
		prompting  bool
		available  bool
		wantReason string
	}{
		{
			name: "idle live session", info: &Info{State: StateActive}, available: true,
		},
		{
			name: "active turn", info: &Info{State: StateActive}, prompting: true,
			wantReason: "Wait for the current turn to finish.",
		},
		{
			name: "stopped session", info: &Info{State: StateStopped},
			wantReason: "Start or resume the session before forking it.",
		},
	} {
		t.Run("Should project "+test.name, func(t *testing.T) {
			t.Parallel()
			catalog := applyWorktreeCommandAvailability(base, test.info, test.prompting)
			for _, descriptor := range catalog.Commands {
				if descriptor.ID != "builtin:worktree" {
					continue
				}
				if descriptor.Available != test.available || descriptor.UnavailableReason != test.wantReason {
					t.Fatalf("worktree descriptor = %#v", descriptor)
				}
				return
			}
			t.Fatal("worktree descriptor is missing")
		})
	}
}
