//go:build darwin || linux

package sessionprocess_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Invariant: exact lineage identity survives reparenting and database reopen, then remains killable.
// Owning layer: sessionprocess; this is the first integration suite for the new domain.
func TestDurableLineage(t *testing.T) {
	t.Run(
		"Should retain an escaped protected process through restart and prune it after termination",
		func(t *testing.T) {
			ctx := t.Context()
			shell := exec.CommandContext(
				context.WithoutCancel(ctx),
				"/bin/sh",
				"-c",
				"sleep 9999 & echo $!; read release",
			)
			// Explicitly remove the marker so this invariant exercises lineage on both OSes.
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "COMPOZY_SESSION_ID=") {
					shell.Env = append(shell.Env, entry)
				}
			}
			procutil.ConfigureCommandSession(shell)
			input, err := shell.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := shell.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := shell.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := input.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
					t.Errorf("close shell input: %v", err)
				}
				if shell.ProcessState == nil {
					if err := shell.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
						t.Errorf("kill shell: %v", err)
					}
					if err := shell.Wait(); err != nil {
						if _, expected := errors.AsType[*exec.ExitError](err); !expected {
							t.Errorf("wait for shell: %v", err)
						}
					}
				}
			})
			line, err := bufio.NewReader(output).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil {
				t.Fatal(err)
			}
			child, err := procutil.InspectProcess(pid)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				p := child
				p.SessionID = "lineage-test"
				result := procutil.TerminateObservedProcess(context.Background(), p, 100*time.Millisecond)
				if result.Outcome != "terminated" && result.Outcome != "already_exited" {
					t.Errorf("clean up child: %+v", result)
				}
			})
			leader, err := procutil.InspectProcess(os.Getpid())
			if err != nil {
				t.Fatal(err)
			}
			source := &processSource{
				owner: sessionprocess.Owner{
					SessionID:       "sess-lineage",
					ProfileID:       store.DefaultProfileID,
					WorkspaceID:     "ws-lineage",
					Name:            "Lineage",
					LeaderPID:       os.Getpid(),
					LeaderStartedAt: leader.StartedAt,
					Live:            true,
				},
			}
			path := filepath.Join(t.TempDir(), "global.db")
			db, err := globaldb.OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			service, err := sessionprocess.New(ctx, source, db, nil, 0)
			if err != nil {
				if closeErr := db.Close(ctx); closeErr != nil {
					t.Errorf("close database after initialization failure: %v", closeErr)
				}
				t.Fatal(err)
			}
			closed := false
			t.Cleanup(func() {
				if !closed {
					service.Shutdown()
					if err := db.Close(context.Background()); err != nil {
						t.Errorf("close database: %v", err)
					}
				}
			})
			snapshot, err := service.Refresh(ctx)
			if err != nil {
				t.Fatal(err)
			}
			process := findProcess(t, snapshot, pid)
			if process.Attribution != "lineage" || process.Kind != "tool" {
				t.Fatalf("attached process = %+v", process)
			}
			if _, err := io.WriteString(input, "release\n"); err != nil {
				t.Fatal(err)
			}
			if err := shell.Wait(); err != nil {
				t.Fatal(err)
			}
			source.mu.Lock()
			source.owner.Live = false
			source.owner.LeaderPID = 0
			source.mu.Unlock()
			snapshot, err = service.Refresh(ctx)
			if err != nil {
				t.Fatal(err)
			}
			process = findProcess(t, snapshot, pid)
			if process.Kind != "background" {
				t.Fatalf("reparented kind=%q", process.Kind)
			}
			service.Shutdown()
			if err := db.Close(ctx); err != nil {
				t.Fatal(err)
			}
			closed = true
			db, err = globaldb.OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			service, err = sessionprocess.New(ctx, source, db, nil, 0)
			if err != nil {
				if closeErr := db.Close(ctx); closeErr != nil {
					t.Errorf("close database after initialization failure: %v", closeErr)
				}
				t.Fatal(err)
			}
			closed = false
			snapshot, err = service.Refresh(ctx)
			if err != nil {
				t.Fatal(err)
			}
			restored := findProcess(t, snapshot, pid)
			if restored.Attribution != "lineage" || !restored.StartedAt.Equal(child.StartedAt) {
				t.Fatalf("reloaded=%+v", restored)
			}
			results, err := service.Terminate(
				ctx,
				source.owner.SessionID,
				[]sessionprocess.Identity{{PID: pid, StartedAt: child.StartedAt}},
				sessionprocess.Targets,
				func(sessionprocess.Owner) bool { return true },
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || results[0].Outcome != "terminated" {
				t.Fatalf("results=%+v", results)
			}
			rows, err := db.LoadProcessObservations(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.PID == pid {
					t.Fatalf("dead identity retained: %+v", row)
				}
			}
		},
	)
}

func findProcess(t *testing.T, snapshot sessionprocess.Snapshot, pid int) sessionprocess.Process {
	t.Helper()
	for _, p := range snapshot.Processes {
		if p.PID == pid {
			return p
		}
	}
	t.Fatalf("pid %d absent from %+v", pid, snapshot.Processes)
	return sessionprocess.Process{}
}

type processSource struct {
	mu        sync.Mutex
	owner     sessionprocess.Owner
	ownersErr error
}

func (s *processSource) Owners(context.Context) ([]sessionprocess.Owner, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []sessionprocess.Owner{s.owner}, s.ownersErr
}
func (s *processSource) Owner(_ context.Context, id string) (sessionprocess.Owner, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owner, id == s.owner.SessionID, nil
}
func (*processSource) Terminals(context.Context) ([]sessionprocess.Terminal, error) { return nil, nil }
func (*processSource) CloseTerminal(context.Context, sessionprocess.Process) error {
	return fmt.Errorf("unexpected terminal")
}
func (*processSource) ProcessesChanged(context.Context, string) {}
func (*processSource) RecordTermination(context.Context, sessionprocess.Process, sessionprocess.Result) error {
	return nil
}

func (*processSource) OrphanOwner(context.Context, procutil.ScannedProcess) (sessionprocess.Owner, bool, error) {
	return sessionprocess.Owner{}, false, nil
}

// Invariant: transient startup inspection failures do not abort the daemon or publish unverified ownership.
// Owning layer/canonical suite: sessionprocess initialization and real persistence recovery.
func TestInitialProcessScanRecovery(t *testing.T) {
	t.Run("Should recover inspection after an initial failure without failing initialization", func(t *testing.T) {
		source := &processSource{ownersErr: errors.New("process owners temporarily unavailable")}
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "global.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		service, err := sessionprocess.New(t.Context(), source, db, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(service.Shutdown)
		if service.Summary("sess-test").Supported {
			t.Fatal("failed initial scan published a supported snapshot")
		}
		source.mu.Lock()
		source.ownersErr = nil
		source.mu.Unlock()
		snapshot, err := service.Refresh(t.Context())
		if err != nil || !snapshot.Supported {
			t.Fatalf("recovered snapshot=%+v error=%v", snapshot, err)
		}
	})
}
