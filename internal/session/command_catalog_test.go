package session

import (
	"testing"

	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
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

func TestSideCommandAvailability(t *testing.T) {
	t.Parallel()
	base, err := commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Should expose side questions while a user session is busy and hide them on spawned sessions", func(t *testing.T) {
		t.Parallel()
		// UT-030
		catalog := applySideCommandAvailability(base, &Info{Type: SessionTypeUser, State: StateActive})
		found := false
		for _, d := range catalog.Commands {
			if d.ID == "builtin:side" {
				found = d.Available
			}
		}
		if !found {
			t.Fatal("side unavailable on user session")
		}
		spawned := applySideCommandAvailability(base, &Info{Type: SessionTypeSpawned})
		for _, d := range spawned.Commands {
			if d.ID == "builtin:side" {
				t.Fatal("side present on spawned session")
			}
		}
	})
	t.Run("Should explain side-only command refusals", func(t *testing.T) {
		t.Parallel()
		// UT-031
		catalog := applySideCommandAvailability(base, &Info{Type: SessionTypeUser, Lineage: &store.SessionLineage{Kind: store.LineageKindSide}})
		reasons := map[string]string{"builtin:side": ErrSideNested.Error(), "builtin:goal": ErrSideGoalUnsupported.Error(), "builtin:worktree": ErrSideWorktreeFixed.Error()}
		for _, d := range catalog.Commands {
			if reason, ok := reasons[d.ID]; ok {
				if d.Available || d.UnavailableReason != reason {
					t.Fatalf("descriptor=%+v", d)
				}
				delete(reasons, d.ID)
			}
		}
		if len(reasons) != 0 {
			t.Fatalf("missing descriptors=%v", reasons)
		}
	})
}
