package sessionprocess

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
	// A kernel identity grants lineage across clock corrections; an uncertain
	// published PID stays protected without granting lineage to a reused process.
	for _, tc := range []struct {
		name, token string
		want        int
	}{
		{"Should protect the leader and retain lineage after a clock correction", "boot:123", 1},
		{"Should protect an uncertain leader PID without adopting reused ancestry", "boot:456", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			leader := process
			leader.KernelToken = "boot:123"
			published := owner
			published.Starting = false
			published.LeaderPID = leader.PID
			published.LeaderStartedAt = start.Add(time.Hour)
			published.LeaderKernelToken = tc.token
			child := procutil.ScannedProcess{PID: 502, PPID: leader.PID, PGID: 502, StartedAt: start.Add(time.Second)}
			service := &Service{source: attributionSource{}, observations: make(map[Identity]Observation)}
			_, rows, err := service.reconcile(t.Context(), procutil.ProcessSnapshot{
				Supported: true, ScannedAt: start.Add(time.Hour), Processes: []procutil.ScannedProcess{leader, child},
			}, []Owner{published}, true)
			if err != nil || len(rows) != tc.want {
				t.Fatalf("rows=%+v error=%v", rows, err)
			}
			if len(rows) != 0 && (rows[0].PID != child.PID || rows[0].Kind != "tool") {
				t.Fatalf("wrong lineage: %+v", rows)
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

// Invariant: a failed audit write never skips later authorized terminations, and the failure remains visible.
// Owning layer/canonical suite: sessionprocess orchestration, with terminal/audit I/O boundaries stubbed.
func TestTerminationAuditFailure(t *testing.T) {
	t.Parallel()
	t.Run("Should attempt every selected terminal despite an audit failure", func(t *testing.T) {
		t.Parallel()
		source := &failingAuditSource{failure: errors.New("audit storage unavailable")}
		service := &Service{source: source}
		ids := []Identity{{PID: 900001}, {PID: 900002}}
		indexed := map[Identity]Process{}
		for _, id := range ids {
			indexed[id] = Process{PID: id.PID, TerminalID: "terminal", Owner: Owner{SessionID: "sess-test"}}
		}
		results, err := service.terminateTargets(t.Context(), ids, indexed)
		if !errors.Is(err, source.failure) || len(results) != 2 || source.closes != 2 || source.audits != 2 {
			t.Fatalf("results=%v closes=%d audits=%d error=%v", results, source.closes, source.audits, err)
		}
	})
}

type failingAuditSource struct {
	attributionSource
	failure        error
	closes, audits int
}

func (s *failingAuditSource) CloseTerminal(context.Context, Process) error { s.closes++; return nil }
func (s *failingAuditSource) RecordTermination(context.Context, Process, Result) error {
	s.audits++
	return s.failure
}

// Invariant: summary ports describe the same background/terminal population as its count.
// Owning layer/canonical suite: sessionprocess summary projection.
func TestProcessSummaryPorts(t *testing.T) {
	t.Parallel()
	t.Run("Should leave foreground tool ports in the detailed list only", func(t *testing.T) {
		t.Parallel()
		owner := Owner{SessionID: "sess-test"}
		summaries := summarize(Snapshot{Supported: true, Processes: []Process{
			{Owner: owner, Kind: "background", Ports: []int{3000}},
			{Owner: owner, Kind: "tool", Ports: []int{4000}},
			{Owner: owner, Kind: "terminal", Ports: []int{5000}},
		}})
		got := summaries[owner.SessionID]
		if got.Background != 1 || got.Terminals != 1 || !slices.Equal(got.Ports, []int{3000, 5000}) {
			t.Fatalf("summary=%+v", got)
		}
	})
}

// Invariant: persisted kernel identity survives clock steps but never PID reuse or reboot.
// Owner/canonical suite: sessionprocess attribution.
func TestRetainedKernelIdentity(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, token string
		want        bool
	}{
		{"Should retain after a wall clock step", "boot:123", true},
		{"Should reject PID reuse", "boot:456", false},
		{"Should reject another boot", "other:123", false},
		{"Should reject missing kernel evidence", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			old := Observation{PID: 501, StartedAt: start, KernelToken: "boot:123",
				Owner: Owner{SessionID: "retained", ProfileID: "default"}, PGID: 500,
				FirstSeen: start, GroupObservedAt: start, Attribution: "lineage"}
			current := procutil.ScannedProcess{PID: 501, PPID: 1, PGID: 500,
				StartedAt: start.Add(time.Hour), KernelToken: tc.token}
			service := &Service{source: attributionSource{}, observations: map[Identity]Observation{old.Identity: old}}
			next, rows, err := service.reconcile(t.Context(), procutil.ProcessSnapshot{
				Supported: true, ScannedAt: start.Add(2 * time.Hour), Processes: []procutil.ScannedProcess{current},
			}, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			if (len(rows) == 1) != tc.want {
				t.Fatalf("rows=%+v, want retained=%t", rows, tc.want)
			}
			if tc.want {
				retained := next[identity(current)]
				if retained.KernelToken != old.KernelToken || !retained.FirstSeen.Equal(old.FirstSeen) ||
					!retained.GroupObservedAt.Equal(current.StartedAt) {
					t.Fatalf("retained=%+v", retained)
				}
			}
		})
	}
}

// Invariant: a configured long stop gets its own wait budget and cleanup starts with a fresh bounded context.
// Owner/canonical suite: sessionprocess orchestration, observed at the source I/O boundary.
func TestAfterStopBudget(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve the configured wait budget and start a fresh cleanup deadline", func(t *testing.T) {
		source := &cleanupDeadlineSource{seen: make(chan context.Context, 1)}
		service := &Service{runCtx: t.Context(), source: source, stopBudget: 5 * time.Minute,
			logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		var waitCtx context.Context
		service.AfterStop("sess-test", func(ctx context.Context) error {
			waitCtx = ctx
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) < 4*time.Minute {
				t.Errorf("wait deadline=%v", deadline)
			}
			return nil
		})
		service.work.Wait()
		select {
		case cleanupCtx := <-source.seen:
			if cleanupCtx == waitCtx {
				t.Fatal("cleanup reused the wait context")
			}
		case <-time.After(time.Second):
			t.Fatal("cleanup did not start")
		}
		if !errors.Is(waitCtx.Err(), context.Canceled) {
			t.Fatalf("wait context not released: %v", waitCtx.Err())
		}
	})
}

type cleanupDeadlineSource struct {
	attributionSource
	seen chan context.Context
}

func (s *cleanupDeadlineSource) Owners(ctx context.Context) ([]Owner, error) {
	deadline, ok := ctx.Deadline()
	if ctx.Err() != nil || !ok || time.Until(deadline) < time.Minute || time.Until(deadline) > 2*time.Minute {
		return nil, errors.New("cleanup context is not fresh and bounded")
	}
	s.seen <- ctx
	return nil, errors.New("stop at the inspection I/O boundary")
}
