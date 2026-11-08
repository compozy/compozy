//go:build !windows

package terminal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
)

// Invariant: only agent-owned terminal groups receive configured POSIX niceness.
// Owner and canonical suite: terminal manager lifecycle; platform-specific process assertions.
func TestManagerProcessPriority(t *testing.T) {
	// Invariant: exec applies agent priority for both visible and hidden commands.
	// Owner/canonical suite: terminal manager process priority through real exec requests.
	for _, visible := range []bool{false, true} {
		for _, kind := range []ActorKind{ActorKindAgent, ActorKindHuman, ActorKindSystem} {
			name := fmt.Sprintf("Should preserve exec priority ownership for %s visible=%t", kind, visible)
			t.Run(name, func(t *testing.T) {
				actor := Actor{Kind: kind, ID: "priority-owner", ProfileID: "profile-a"}
				if kind == ActorKindAgent {
					actor.SessionID, actor.RunID, actor.Generation = "session-a", "run-a", 1
				}
				inherited, err := procutil.CurrentProcessNiceness()
				if err != nil {
					t.Fatal(err)
				}
				if kind == ActorKindAgent && inherited >= 19 {
					t.Skip("inherited priority leaves no room to lower priority")
				}
				want := inherited
				if kind == ActorKindAgent {
					want = 19
				}
				starter := &startupPriorityProbe{PTY: terminalpty.New(), want: want}
				manager, _, _ := newTestManager(t, DefaultSettings(), WithPTY(starter), WithAgentNiceness(19))
				script := "read hold"
				if runtime.GOOS == "linux" {
					script = `ps -o ni= -p $$; sh -c 'ps -o ni= -p $$'; read hold`
				}
				result, err := manager.Exec(t.Context(), ExecRequest{
					WS: "workspace-a", Command: "sh", Args: []string{"-c", script},
					Visible: visible, Actor: actor, Approval: "allowlisted", YieldMs: 250,
				})
				if err != nil {
					t.Fatal(err)
				}
				if !result.StillRunning || result.TerminalID == nil {
					t.Fatalf("Exec() = %+v, want a live terminal", result)
				}
				handle, err := manager.Handle(t.Context(), "workspace-a", "profile-a", *result.TerminalID)
				if err != nil {
					t.Fatal(err)
				}
				// Exec has returned through the yield boundary, after the platform's group adjustment.
				got, err := procutil.ProcessGroupNiceness(handle.(*session).proc.ProcessGroupID())
				if err != nil || got != want {
					t.Fatalf("exec group priority = %d, error = %v, want %d", got, err, want)
				}
			})
		}
	}

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

// Invariant: scanner identity closes the same live terminal for both launch modes.
// Owner/canonical suite: terminal manager OS process lifecycle.
func TestManagerProcessIdentity(t *testing.T) {
	for _, mode := range []string{"pty", "pipe"} {
		t.Run("Should close a real "+mode+" terminal by kernel identity", func(t *testing.T) {
			manager, _, _ := newTestManager(t, DefaultSettings(), WithPTY(terminalpty.New()))
			actor := Actor{Kind: ActorKindSystem, ID: "session-processes", ProfileID: "profile-a"}
			var handle Handle
			var err error
			if mode == "pty" {
				handle, err = manager.Open(
					t.Context(),
					OpenRequest{
						WS:           "workspace-a",
						Shell:        "sh",
						Actor:        actor,
						Capabilities: Capabilities{Interactive: true},
					},
				)
			} else {
				handle, err = manager.OpenPipe(
					t.Context(),
					PipeRequest{WS: "workspace-a", Argv: []string{"sh", "-c", "sleep 300"}, Actor: actor},
				)
			}
			if err != nil {
				t.Fatal(err)
			}
			identity, err := procutil.InspectProcess(handle.(*session).proc.PID())
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS == "linux" && identity.KernelToken == "" {
				t.Fatal("Linux terminal identity has no kernel token")
			}
			if identity.KernelToken != "" {
				if _, err := manager.CloseProcess(
					t.Context(),
					"workspace-a",
					handle.Info().ID,
					actor,
					SignalKILL,
					identity.PID,
					identity.StartedAt,
					"different-token",
				); err == nil || !strings.Contains(err.Error(), "process identity changed") {
					t.Fatalf("mismatched kernel token error = %v, want process identity change", err)
				}
				identity.StartedAt = identity.StartedAt.Add(time.Hour)
			}
			exit, err := manager.CloseProcess(
				t.Context(),
				"workspace-a",
				handle.Info().ID,
				actor,
				SignalKILL,
				identity.PID,
				identity.StartedAt,
				identity.KernelToken,
			)
			if err != nil || exit == nil {
				t.Fatalf("close scanned process: exit=%+v err=%v", exit, err)
			}
		})
	}
}

// startupPriorityProbe observes the real command before the manager can adjust its group.
type startupPriorityProbe struct {
	PTY
	want int
}

func (p *startupPriorityProbe) Start(ctx context.Context, spec ProcSpec) (Proc, error) {
	proc, err := p.PTY.Start(ctx, spec)
	if err != nil || runtime.GOOS != "linux" {
		return proc, err
	}
	observed := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(proc.Reader())
		for range 2 {
			line, err := reader.ReadString('\n')
			if err != nil {
				observed <- err
				return
			}
			got, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil || got != p.want {
				observed <- fmt.Errorf("startup priority = %q, error = %v, want %d", line, err, p.want)
				return
			}
		}
		observed <- nil
	}()
	select {
	case err = <-observed:
	case <-time.After(10 * time.Second):
		err = errors.New("startup priority observation timed out")
	}
	if err != nil {
		return nil, errors.Join(err, cleanupUnregisteredProcess(ctx, proc))
	}
	return proc, nil
}
