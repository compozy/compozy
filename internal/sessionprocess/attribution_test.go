package sessionprocess

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
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
	t.Run("Should retain cleanup after the initial stop wait deadline", func(t *testing.T) {
		source := &cleanupDeadlineSource{seen: make(chan context.Context, 1)}
		service := &Service{runCtx: t.Context(), source: source, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		attempts := 0
		service.AfterStop("sess-test", func(context.Context) error {
			attempts++
			if attempts == 1 {
				return context.DeadlineExceeded
			}
			return nil
		})
		service.work.Wait()
		if attempts != 2 {
			t.Fatalf("await attempts = %d", attempts)
		}
		select {
		case <-source.seen:
		default:
			t.Fatal("cleanup did not run after retry")
		}
	})
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

// Invariant: targets outside the authorized set reveal no host process state.
// Owning layer/canonical suite: sessionprocess termination authorization.
func TestUnauthorizedTerminationTargets(t *testing.T) {
	t.Run("Should return neutral outcomes for unauthorized targets", func(t *testing.T) {
		t.Parallel()
		service := &Service{}
		targets := []Identity{
			{PID: os.Getpid(), StartedAt: time.Unix(1, 0)},
			{PID: -1, StartedAt: time.Unix(1, 0)},
		}
		results, err := service.terminateTargets(t.Context(), targets, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != len(targets) {
			t.Fatalf("results=%v", results)
		}
		for i, result := range results {
			if result.Identity != targets[i] || result.Outcome != "unproven" ||
				result.Error != "process ownership could not be verified" {
				t.Fatalf("unauthorized target revealed state: %+v", result)
			}
		}
	})
}

// Invariant: cleanup registered before a resume cannot signal the new run's processes.
// Owning layer/canonical suite: sessionprocess orchestration with real OS identity checks.
func TestAfterStopPreservesResumedProcesses(t *testing.T) {
	t.Run("Should preserve new process identities after resume", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("process inspection unsupported")
		}
		leader, err := procutil.InspectProcess(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		source := &resumedProcessSource{owner: Owner{SessionID: "sess-resumed", Live: true,
			LeaderPID: leader.PID, LeaderStartedAt: leader.StartedAt}}
		service := &Service{runCtx: t.Context(), source: source, store: discardObservations{},
			observations: make(map[Identity]Observation), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		release := make(chan struct{})
		service.AfterStop(source.owner.SessionID, func(ctx context.Context) error {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		t.Cleanup(func() { service.work.Wait() })
		defer close(release)
		child := exec.CommandContext(t.Context(), "/bin/sleep", "60")
		procutil.ConfigureCommandSession(child)
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Error(err)
			}
			if err := child.Wait(); err != nil {
				if _, expectedExit := errors.AsType[*exec.ExitError](err); !expectedExit {
					t.Errorf("wait for resumed process cleanup: %v", err)
				}
			}
		}()
		snapshot, err := service.Refresh(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(snapshot.Processes, func(p Process) bool { return p.PID == child.Process.Pid }) {
			t.Fatal("new process was not attributed to the resumed session")
		}
		release <- struct{}{}
		service.work.Wait()
		if _, err := procutil.InspectProcess(child.Process.Pid); err != nil {
			t.Fatalf("old cleanup ended resumed work: %v", err)
		}
	})
}

type resumedProcessSource struct {
	attributionSource
	owner Owner
}

func (s *resumedProcessSource) Owners(context.Context) ([]Owner, error) { return []Owner{s.owner}, nil }
func (s *resumedProcessSource) Owner(context.Context, string) (Owner, bool, error) {
	return s.owner, true, nil
}

type discardObservations struct{}

func (discardObservations) LoadProcessObservations(context.Context) ([]Observation, error) {
	return nil, nil
}
func (discardObservations) SaveProcessObservations(context.Context, []Observation, []Identity) error {
	return nil
}

// Invariant: a queued process capture obeys cancellation while another scan holds exclusivity.
// Owning layer/canonical suite: sessionprocess attribution and capture.
func TestProcessCaptureCancellation(t *testing.T) {
	t.Run("Should cancel a queued scan without waiting for its predecessor", func(t *testing.T) {
		var service Service
		if err := service.lockScan(t.Context()); err != nil {
			t.Fatal(err)
		}
		defer service.scanGate.Release(1)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := service.refresh(ctx, nil, true); !errors.Is(err, context.Canceled) {
			t.Fatalf("queued refresh error = %v", err)
		}
	})
}

// Invariant: a session's rows group into one tree per logical thing it started. A row joins its
// nearest attributed ancestor, then its process-group leader; detached helpers of the same app
// bundle fold into the earlier tree; wrapper chains name the tree by the process they start.
// Owning layer/canonical suite: sessionprocess attribution, which labels trees for every surface.
func TestProcessTrees(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	owner := Owner{SessionID: "sess-trees", ProfileID: "profile-test", WorkspaceID: "ws-test"}
	const chrome = "/Applications/Google Chrome.app/Contents"
	const helpers = chrome + "/Frameworks/Google Chrome Framework.framework/Helpers"
	row := func(pid, ppid, pgid int, after time.Duration, kind, argv0, command string, ports ...int) Process {
		return Process{
			PID: pid, PPID: ppid, PGID: pgid, StartedAt: start.Add(after),
			Argv0: argv0, Command: command, Ports: ports,
			Owner: owner, Kind: kind,
		}
	}
	browser := []Process{
		row(100, 1, 100, 0, "background", "/opt/homebrew/bin/node", "node /Users/dev/.agent-browser/daemon.js"),
		row(110, 100, 110, time.Second, "background", chrome+"/MacOS/Google Chrome", "Google Chrome --headless", 9222),
		row(111, 1, 111, 2*time.Second, "background", helpers+"/chrome_crashpad_handler", "chrome_crashpad_handler"),
		row(113, 1, 113, 2*time.Second, "background", helpers+"/chrome_crashpad_handler", "chrome_crashpad_handler"),
		row(120, 110, 110, 3*time.Second, "background", helpers+"/Google Chrome Helper (GPU)", "helper --type=gpu"),
		row(121, 110, 110, 3*time.Second, "background", helpers+"/Google Chrome Helper (Renderer)", "renderer"),
		row(122, 1, 110, 4*time.Second, "background", helpers+"/Google Chrome Helper (Renderer)", "renderer"),
	}
	wrapped := []Process{
		row(200, 1, 200, 0, "background", "/opt/homebrew/bin/bun", "bun run proto dev"),
		row(201, 200, 200, time.Second, "background", "/opt/homebrew/bin/bun", "bun run --cwd web proto dev"),
		row(202, 201, 200, 2*time.Second, "background", "/opt/homebrew/bin/node", "node proto dev", 4400, 4401),
	}
	tool := []Process{
		row(300, 50, 300, 0, "tool", "/usr/local/go/bin/go", "go test ./..."),
		row(301, 300, 300, time.Second, "tool", "/usr/local/go/pkg/tool/compile", "compile"),
	}
	// A daemonized node beside the browser's node shares only a bin directory, never a bundle.
	unrelated := row(400, 1, 400, 5*time.Second, "background", "/opt/homebrew/bin/node", "node server.js", 3000)
	id := func(p Process) Identity { return identity(p.ScannedProcess) }
	cases := []struct {
		name     string
		rows     []Process
		root     Process
		lead     Process
		wrappers []Process
		members  int
		ports    []int
	}{
		{"Should fold a browser, its group, and its detached crash handlers into one tree",
			browser, browser[0], browser[0], nil, len(browser), []int{9222}},
		{"Should name a wrapper chain by the process it starts, outermost wrapper first",
			wrapped, wrapped[0], wrapped[2], wrapped[:2], len(wrapped), []int{4400, 4401}},
		{"Should keep a tool call and its children as one tool tree",
			tool, tool[0], tool[0], nil, len(tool), []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rows := append(slices.Clone(tc.rows), unrelated)
			assignTrees(rows)
			trees := Trees(rows)
			if len(trees) != 2 {
				t.Fatalf("trees=%+v, want the case tree and the unrelated server", trees)
			}
			tree := trees[0]
			if tree.Root != id(tc.root) || tree.Lead != id(tc.lead) || len(tree.Members) != tc.members ||
				tree.Kind != tc.root.Kind || !slices.Equal(tree.Ports, tc.ports) {
				t.Fatalf("tree=%+v", tree)
			}
			wrappers := make([]Identity, len(tc.wrappers))
			for i, w := range tc.wrappers {
				wrappers[i] = id(w)
			}
			if !slices.Equal(tree.Wrappers, wrappers) {
				t.Fatalf("wrappers=%v, want %v", tree.Wrappers, wrappers)
			}
			if trees[1].Root != id(unrelated) || len(trees[1].Members) != 1 {
				t.Fatalf("unrelated=%+v", trees[1])
			}
		})
	}

	// Lab regression: two daemonized browsers of one app are two trees, each with its own handler.
	t.Run("Should keep each instance of an app apart with the crash handler it started", func(t *testing.T) {
		t.Parallel()
		rows := slices.Clone(browser[:3])
		second := []Process{
			row(500, 1, 500, 10*time.Second, "background", "/opt/homebrew/bin/node", "node daemon.js --session b"),
			row(510, 500, 510, 11*time.Second, "background", chrome+"/MacOS/Google Chrome", "Google Chrome --headless"),
			row(
				511,
				1,
				511,
				12*time.Second,
				"background",
				helpers+"/chrome_crashpad_handler",
				"chrome_crashpad_handler",
			),
		}
		rows = append(rows, second...)
		assignTrees(rows)
		trees := Trees(rows)
		if len(trees) != 2 ||
			!slices.Equal(trees[0].Members, []Identity{id(browser[0]), id(browser[1]), id(browser[2])}) ||
			!slices.Equal(trees[1].Members, []Identity{id(second[0]), id(second[1]), id(second[2])}) {
			t.Fatalf("trees=%+v", trees)
		}
	})

	// Review regression: one install's daemonized server started twice is two trees, not helpers.
	t.Run("Should keep two daemonized copies of one program apart", func(t *testing.T) {
		t.Parallel()
		rows := []Process{
			row(600, 1, 600, 0, "background", "/opt/redis/redis-server", "redis-server --port 6379", 6379),
			row(601, 1, 601, time.Second, "background", "/opt/redis/redis-server", "redis-server --port 6380", 6380),
			// A different program of that install started much later is not a helper either.
			row(602, 1, 602, time.Hour, "background", "/opt/redis/redis-sentinel", "redis-sentinel", 26379),
		}
		assignTrees(rows)
		if trees := Trees(rows); len(trees) != 3 {
			t.Fatalf("trees=%+v", trees)
		}
	})

	// Review regression: background work re-parented out of a tool call stays left running.
	t.Run("Should keep a re-parented background process out of a tool call's group", func(t *testing.T) {
		t.Parallel()
		orphan := row(302, 1, 300, 2*time.Second, "background", "/opt/homebrew/bin/node", "node watcher.js")
		rows := append(slices.Clone(tool), orphan)
		assignTrees(rows)
		trees := Trees(rows)
		if len(trees) != 2 || trees[1].Root != id(orphan) || trees[1].Kind != "background" {
			t.Fatalf("trees=%+v", trees)
		}
		indexed := map[Identity]Process{}
		for _, p := range rows {
			indexed[id(p)] = p
		}
		if got := expandTrees(rows, indexed, nil, AllLeftRunning); !slices.Equal(got, []Identity{id(orphan)}) {
			t.Fatalf("all left running=%v", got)
		}
	})

	t.Run("Should count trees, not processes, and leave tool calls out of the badge", func(t *testing.T) {
		t.Parallel()
		rows := slices.Concat(browser, wrapped, tool)
		assignTrees(rows)
		summary := summarize(Snapshot{Supported: true, Processes: rows})[owner.SessionID]
		if summary.Trees != 2 || summary.Background != len(browser)+len(wrapped) {
			t.Fatalf("summary=%+v", summary)
		}
	})

	t.Run("Should stop whole trees, and leave tool calls out of all unless included", func(t *testing.T) {
		t.Parallel()
		rows := slices.Concat(browser, wrapped, tool)
		assignTrees(rows)
		indexed := make(map[Identity]Process)
		for _, p := range rows {
			indexed[id(p)] = p
		}
		// Stopping a renderer stops its browser: the daemon first, then every helper.
		got := expandTrees(rows, indexed, []Identity{id(browser[5])}, Targets)
		if len(got) != len(browser) || got[0] != id(browser[0]) {
			t.Fatalf("targets=%v", got)
		}
		if got := expandTrees(rows, indexed, nil, AllLeftRunning); len(got) != len(browser)+len(wrapped) {
			t.Fatalf("all left running=%v", got)
		}
		if got := expandTrees(rows, indexed, nil, AllIncludingTools); len(got) != len(rows) {
			t.Fatalf("all including tools=%v", got)
		}
		// Delayed cleanup stops what it captured, never members its trees gained since.
		if got := expandTrees(rows, indexed, []Identity{id(browser[0])}, Captured); !slices.Equal(
			got, []Identity{id(browser[0])},
		) {
			t.Fatalf("captured=%v", got)
		}
		// A member outside the caller's authority is never added; an unknown target stays to report.
		delete(indexed, id(wrapped[2]))
		unknown := Identity{PID: 999, StartedAt: start}
		got = expandTrees(rows, indexed, []Identity{id(wrapped[0]), unknown}, Targets)
		if !slices.Equal(got, []Identity{id(wrapped[0]), id(wrapped[1]), unknown}) {
			t.Fatalf("targets=%v", got)
		}
	})
}
