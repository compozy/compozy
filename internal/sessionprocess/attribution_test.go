package sessionprocess

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

// Invariant: group inference requires a surviving exact identity in the same group.
// Owning layer/canonical suite: sessionprocess attribution; OS signal safety remains in procutil.
func TestGroupContinuity(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	owner := Owner{SessionID: "sess-test", ProfileID: "profile-test", WorkspaceID: "ws-test"}
	anchor := Observation{
		PID: 501, StartedAt: start,
		Owner:           owner,
		PGID:            500,
		FirstSeen:       start,
		GroupObservedAt: start,
		Attribution:     "lineage",
	}
	cases := []struct {
		name string
		rows []procutil.ScannedProcess
		want int
	}{
		{
			"Should retain an observed member and discover its later group member",
			[]procutil.ScannedProcess{
				{PID: 501, PPID: 1, PGID: 500, StartedAt: start},
				{PID: 502, PPID: 1, PGID: 500, StartedAt: start.Add(time.Second)},
			},
			2,
		},
		{
			"Should refuse a recycled PID and PGID without continuity",
			[]procutil.ScannedProcess{
				{PID: 501, PPID: 1, PGID: 500, StartedAt: start.Add(time.Hour)},
				{PID: 502, PPID: 1, PGID: 500, StartedAt: start.Add(time.Hour)},
			},
			0,
		},
		{
			"Should retain a moved identity without adopting its new group peers",
			[]procutil.ScannedProcess{
				{PID: 501, PPID: 1, PGID: 700, StartedAt: start},
				{PID: 502, PPID: 1, PGID: 700, StartedAt: start.Add(time.Second)},
			},
			1,
		},
		{
			"Should refuse a group member born before the observation anchor",
			[]procutil.ScannedProcess{
				{PID: 501, PPID: 1, PGID: 500, StartedAt: start},
				{PID: 502, PPID: 1, PGID: 500, StartedAt: start.Add(-time.Second)},
			},
			1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := &Service{
				source:       attributionSource{},
				observations: map[Identity]Observation{anchor.Identity: anchor},
			}
			_, rows, err := service.reconcile(
				t.Context(),
				procutil.ProcessSnapshot{Supported: true, ScannedAt: start.Add(time.Hour), Processes: tc.rows},
				[]Owner{owner},
				true,
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != tc.want {
				t.Fatalf("rows=%+v, want %d", rows, tc.want)
			}
		})
	}
}

// Invariant: startup cannot expose an agent as a killable session descendant.
// Owning layer/canonical suite: sessionprocess attribution, including recovery of retained identities.
func TestAgentStartupProtection(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	process := procutil.ScannedProcess{PID: 501, PPID: 1, PGID: 501, StartedAt: start, SessionID: "sess-starting"}
	owner := Owner{SessionID: process.SessionID, ProfileID: "profile-test", Live: true, Starting: true}
	for _, retained := range []bool{false, true} {
		name := "Should hide a newly marked agent until its leader identity is published"
		if retained {
			name = "Should hide a retained identity during agent recovery"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			service := &Service{source: attributionSource{}, observations: make(map[Identity]Observation)}
			if retained {
				service.observations[identity(process)] = Observation{
					Identity: identity(process),
					Owner:    owner,
					PGID:     process.PGID,
				}
			}
			snapshot := procutil.ProcessSnapshot{
				Supported: true,
				ScannedAt: start,
				Processes: []procutil.ScannedProcess{process},
			}
			_, rows, err := service.reconcile(t.Context(), snapshot, []Owner{owner}, true)
			if err != nil || len(rows) != 0 {
				t.Fatalf("starting agent exposed: rows=%+v err=%v", rows, err)
			}
			published := owner
			published.Starting, published.LeaderPID, published.LeaderStartedAt = false, process.PID, start
			_, rows, err = service.reconcile(t.Context(), snapshot, []Owner{published}, true)
			if err != nil || len(rows) != 0 {
				t.Fatalf("published agent exposed: rows=%+v err=%v", rows, err)
			}
		})
	}
}

type attributionSource struct{}

func (attributionSource) Owners(context.Context) ([]Owner, error) { return nil, nil }
func (attributionSource) Owner(context.Context, string) (Owner, bool, error) {
	return Owner{}, false, nil
}
func (attributionSource) OrphanOwner(context.Context, procutil.ScannedProcess) (Owner, bool, error) {
	return Owner{}, false, nil
}
func (attributionSource) Terminals(context.Context) ([]Terminal, error)            { return nil, nil }
func (attributionSource) CloseTerminal(context.Context, Process) error             { return nil }
func (attributionSource) ProcessesChanged(context.Context, string)                 {}
func (attributionSource) RecordTermination(context.Context, Process, Result) error { return nil }

// Invariant: open process lists refresh on identity or displayed-data changes, not just counts.
// Owning layer/canonical suite: sessionprocess observed-process attribution and catalog changes.
func TestProcessCatalogChanges(t *testing.T) {
	t.Parallel()
	original := Process{
		PID: 501, StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Owner: Owner{SessionID: "sess-test"}, Kind: "background", Attribution: "env",
		Command: "node server.js", CWD: "/project", Ports: []int{3000},
	}
	cases := []struct {
		name   string
		change func(*Process)
		want   []string
	}{
		{
			"Should refresh when a process is replaced without changing counts or ports",
			func(p *Process) { p.PID++ },
			[]string{"sess-test"},
		},
		{
			"Should refresh when a PID is reused",
			func(p *Process) { p.StartedAt = p.StartedAt.Add(time.Second) },
			[]string{"sess-test"},
		},
		{
			"Should refresh when command arguments change",
			func(p *Process) { p.Command = "node worker.js" },
			[]string{"sess-test"},
		},
		{
			"Should refresh when the working directory changes",
			func(p *Process) { p.CWD = "/other" },
			[]string{"sess-test"},
		},
		{
			"Should refresh both owners when attribution moves",
			func(p *Process) { p.Owner.SessionID = "sess-other" },
			[]string{"sess-other", "sess-test"},
		},
		{
			"Should avoid a notification for unchanged displayed data",
			func(*Process) {},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			next := original
			tc.change(&next)
			got := changedProcessSessions([]Process{original}, []Process{next})
			if !slices.Equal(got, tc.want) {
				t.Fatalf("changed sessions = %v, want %v", got, tc.want)
			}
		})
	}
}
