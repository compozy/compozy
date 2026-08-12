package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
)

type createTestFixture struct {
	service         *Service
	store           *memoryWorktreeStore
	runner          *recordingGitRunner
	workspace       Workspace
	commonDir       string
	adminGitDir     string
	worktreesRoot   string
	listOutput      []byte
	failWorktreeAdd atomic.Bool
}

func newCreateTestFixture(t *testing.T, worktreesConfig config.WorktreesConfig) *createTestFixture {
	t.Helper()
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(commonDir, 0o700); err != nil {
		t.Fatalf("create Git common directory: %v", err)
	}
	worktreesRoot := t.TempDir()
	adminGitDir := filepath.Join(commonDir, "worktrees", "created")
	store := newMemoryWorktreeStore()
	workspace := Workspace{ID: "ws-create", Name: "Create Workspace", Root: root}
	fixture := &createTestFixture{
		store: store, workspace: workspace, commonDir: commonDir,
		adminGitDir: adminGitDir, worktreesRoot: worktreesRoot,
		listOutput: worktreeListFixture(root, "main"),
	}
	runner := &recordingGitRunner{}
	runner.run = func(call gitInvocation) gitResponse {
		command := strings.Join(call.args, " ")
		switch {
		case command == "rev-parse --path-format=absolute --git-common-dir":
			return gitResponse{stdout: []byte(commonDir + "\n")}
		case strings.Contains(command, "worktree list --porcelain -z"):
			return gitResponse{stdout: append([]byte(nil), fixture.listOutput...)}
		case command == "rev-parse --verify HEAD":
			return gitResponse{stdout: []byte("root-head\n")}
		case command == "show-ref --verify --quiet refs/heads/feature/existing":
			return gitResponse{}
		case strings.HasPrefix(command, "show-ref --verify --quiet refs/heads/"):
			return gitResponse{err: errors.New("reference does not exist")}
		case command == "symbolic-ref --quiet --short refs/remotes/origin/HEAD":
			return gitResponse{err: errors.New("origin HEAD is absent")}
		case command == "symbolic-ref --quiet --short HEAD":
			return gitResponse{stdout: []byte("main\n")}
		case strings.HasPrefix(command, "branch "):
			return gitResponse{}
		case strings.HasPrefix(command, "rev-parse --verify "):
			return gitResponse{stdout: []byte("created-head\n")}
		case strings.HasPrefix(command, "worktree add "):
			if fixture.failWorktreeAdd.Load() {
				return gitResponse{stderr: []byte("checkout failed"), err: errors.New("exit 1")}
			}
			target := call.args[len(call.args)-2]
			if err := os.MkdirAll(target, 0o700); err != nil {
				return gitResponse{err: err}
			}
			if err := os.MkdirAll(adminGitDir, 0o700); err != nil {
				return gitResponse{err: err}
			}
			return gitResponse{}
		case command == "rev-parse --path-format=absolute --git-dir":
			return gitResponse{stdout: []byte(adminGitDir + "\n")}
		case strings.HasPrefix(command, "config branch."):
			return gitResponse{}
		case strings.HasPrefix(command, "ls-files --others --ignored --exclude-standard -z --"):
			return gitResponse{stdout: []byte(".env\x00")}
		case strings.Contains(command, "worktree remove --force"):
			if err := os.RemoveAll(call.args[len(call.args)-1]); err != nil {
				return gitResponse{err: err}
			}
			return gitResponse{}
		case strings.HasPrefix(command, "update-ref -d "):
			return gitResponse{}
		default:
			return gitResponse{err: errors.New("unexpected Git command: " + command)}
		}
	}
	fixture.runner = runner
	var sequence atomic.Int64
	fixture.service = NewService(
		store,
		runner,
		WithCapabilityGate(readyCapabilityGate()),
		WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{workspace.ID: workspace}}),
		WithConfig(worktreesConfig, worktreesRoot),
		WithIDGenerator(func(string) (string, error) {
			return "wt-create-" + string(rune('a'+sequence.Add(1)-1)), nil
		}),
		WithClock(func() time.Time { return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC) }),
	)
	return fixture
}

func worktreeListFixture(root, branch string) []byte {
	return []byte("worktree " + root + "\x00HEAD root-head\x00branch refs/heads/" + branch + "\x00\x00")
}

// Canonical suite: durable phased creation, rollback, cancellation, bootstrap, and path safety.
func TestServiceCreate(t *testing.T) {
	t.Parallel()

	t.Run("Should materialize a per-run worktree with readable names and recorded namespace", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		request := RunWorktreeRequest{TaskSlug: "Review Docs", RunID: "run-123"}
		item, err := fixture.service.MaterializeForRun(context.Background(), fixture.workspace.ID, request)
		if err != nil {
			t.Fatalf("MaterializeForRun() error = %v", err)
		}
		wantName := RunWorktreeName(request.TaskSlug, request.RunID)
		if item.Name != wantName || item.Branch != "run/"+wantName || item.Origin != OriginPerRun ||
			item.RunID != request.RunID || item.RunNamespace != "run/" {
			t.Fatalf("MaterializeForRun() = %#v, want readable run identity", item)
		}
	})

	t.Run("Should roll back only the exact per-run materialization", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		request := RunWorktreeRequest{TaskSlug: "Review Docs", RunID: "run-rollback"}
		item, err := fixture.service.MaterializeForRun(context.Background(), fixture.workspace.ID, request)
		if err != nil {
			t.Fatalf("MaterializeForRun() error = %v", err)
		}
		if err := fixture.service.RollbackRunMaterialization(
			context.Background(),
			fixture.workspace.ID,
			item.ID,
			"different-run",
		); !errors.Is(err, ErrNotFound) {
			t.Fatalf("RollbackRunMaterialization(different run) error = %v, want ErrNotFound", err)
		}
		if _, err := fixture.store.Get(context.Background(), fixture.workspace.ID, item.ID); err != nil {
			t.Fatalf("Get() after rejected rollback error = %v", err)
		}
		if err := fixture.service.RollbackRunMaterialization(
			context.Background(),
			fixture.workspace.ID,
			item.ID,
			request.RunID,
		); err != nil {
			t.Fatalf("RollbackRunMaterialization() error = %v", err)
		}
		if got, err := fixture.store.Get(context.Background(), fixture.workspace.ID, item.ID); !errors.Is(err, ErrNotFound) || got != nil {
			t.Fatalf("Get() after rollback = %#v, %v, want ErrNotFound", got, err)
		}
		commands := invocationCommands(fixture.runner.invocations())
		if !slices.ContainsFunc(commands, func(command string) bool {
			return strings.HasSuffix(command, " worktree remove --force "+item.Path)
		}) {
			t.Fatalf("rollback Git calls = %#v, want forced checkout removal", commands)
		}
		if !containsCommandPrefix(commands, "update-ref -d refs/heads/"+item.Branch+" "+item.CreatedHead) {
			t.Fatalf("rollback Git calls = %#v, want compare-and-delete", commands)
		}
	})

	t.Run("Should create a branch and persist every phase before completing", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		events := &recordingEventSink{}
		fixture.service.events = events
		item, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{Name: "Docs Refresh"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if item.State != StateReady || !item.CreatedBranch || item.BaseRef != "main" ||
			item.CreatedHead != "created-head" || filepath.Base(item.GitDir) != "created" {
			t.Fatalf("Create() = %#v, want ready runtime-owned worktree", item)
		}
		wantPhases := []PendingPhase{PhaseBranch, PhaseCheckout, PhaseCopy, PhaseSetup}
		if got := fixture.store.phases(); !reflect.DeepEqual(got, wantPhases) {
			t.Fatalf("pending phases = %#v, want %#v", got, wantPhases)
		}
		commands := invocationCommands(fixture.runner.invocations())
		for _, want := range []string{
			"branch docs-refresh main",
			"worktree add " + item.Path + " docs-refresh",
			"config branch.docs-refresh.gh-merge-base main",
		} {
			if !slices.Contains(commands, want) {
				t.Fatalf("Git calls = %#v, missing %q", commands, want)
			}
		}
		if got := events.count(EventCreated); got != 1 {
			t.Fatalf("created events = %d, want one", got)
		}
	})

	t.Run("Should return a durable pending row before detached materialization completes", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		materializationStarted := make(chan struct{})
		continueMaterialization := make(chan struct{})
		baseRun := fixture.runner.run
		fixture.runner.run = func(call gitInvocation) gitResponse {
			if strings.HasPrefix(strings.Join(call.args, " "), "branch accepted-create ") {
				close(materializationStarted)
				<-continueMaterialization
			}
			return baseRun(call)
		}
		requestContext, cancelRequest := context.WithCancel(context.Background())
		item, err := fixture.service.CreateAccepted(
			requestContext,
			fixture.workspace.ID,
			CreateOptions{Name: "Accepted Create"},
		)
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		cancelRequest()
		select {
		case <-materializationStarted:
		case <-time.After(time.Second):
			t.Fatal("detached materialization did not start")
		}
		if item.State != StatePending {
			t.Fatalf("CreateAccepted() state = %q, want pending", item.State)
		}
		persisted, err := fixture.store.Get(context.Background(), fixture.workspace.ID, item.ID)
		if err != nil || persisted.State != StatePending {
			t.Fatalf("pending row = %#v, %v, want durable pending", persisted, err)
		}
		operation := fixture.service.getCreateOperation(fixture.workspace.ID, item.ID)
		if operation == nil {
			t.Fatal("accepted creation operation = nil")
		}
		close(continueMaterialization)
		select {
		case <-operation.done:
		case <-time.After(time.Second):
			t.Fatal("detached materialization did not complete")
		}
		persisted, err = fixture.store.Get(context.Background(), fixture.workspace.ID, item.ID)
		if err != nil || persisted.State != StateReady {
			t.Fatalf("completed row = %#v, %v, want ready after request cancellation", persisted, err)
		}
	})

	t.Run("Should expose pending state while CreateReady waits and return the ready row", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		materializationStarted := make(chan struct{})
		continueMaterialization := make(chan struct{})
		baseRun := fixture.runner.run
		fixture.runner.run = func(call gitInvocation) gitResponse {
			if strings.HasPrefix(strings.Join(call.args, " "), "branch ready-create ") {
				close(materializationStarted)
				<-continueMaterialization
			}
			return baseRun(call)
		}
		result := make(chan *Worktree, 1)
		errResult := make(chan error, 1)
		go func() {
			item, err := fixture.service.CreateReady(
				context.Background(),
				fixture.workspace.ID,
				CreateOptions{Name: "Ready Create"},
			)
			result <- item
			errResult <- err
		}()
		select {
		case <-materializationStarted:
		case <-time.After(time.Second):
			t.Fatal("ready materialization did not start")
		}
		rows, err := fixture.store.List(context.Background(), fixture.workspace.ID)
		if err != nil || len(rows) != 1 || rows[0].State != StatePending {
			t.Fatalf("rows while CreateReady waits = %#v, %v, want one pending row", rows, err)
		}
		close(continueMaterialization)
		if err := <-errResult; err != nil {
			t.Fatalf("CreateReady() error = %v", err)
		}
		if item := <-result; item == nil || item.State != StateReady {
			t.Fatalf("CreateReady() = %#v, want ready row", item)
		}
	})

	t.Run("Should cancel and roll back CreateReady when its caller stops waiting", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		materializationStarted := make(chan struct{})
		continueMaterialization := make(chan struct{})
		baseRun := fixture.runner.run
		fixture.runner.run = func(call gitInvocation) gitResponse {
			if strings.HasPrefix(strings.Join(call.args, " "), "branch canceled-ready ") {
				close(materializationStarted)
				<-continueMaterialization
			}
			return baseRun(call)
		}
		ctx, cancel := context.WithCancel(context.Background())
		errResult := make(chan error, 1)
		go func() {
			_, err := fixture.service.CreateReady(
				ctx,
				fixture.workspace.ID,
				CreateOptions{Name: "Canceled Ready"},
			)
			errResult <- err
		}()
		select {
		case <-materializationStarted:
		case <-time.After(time.Second):
			t.Fatal("cancelable materialization did not start")
		}
		rows, err := fixture.store.List(context.Background(), fixture.workspace.ID)
		if err != nil || len(rows) != 1 {
			t.Fatalf("rows before CreateReady cancellation = %#v, %v", rows, err)
		}
		fixture.failWorktreeAdd.Store(true)
		cancel()
		close(continueMaterialization)
		if err := <-errResult; !errors.Is(err, context.Canceled) {
			t.Fatalf("CreateReady() error = %v, want context.Canceled", err)
		}
		if _, err := fixture.store.Get(context.Background(), fixture.workspace.ID, rows[0].ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get(canceled CreateReady) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("Should remove a completed CreateReady when cancellation wins delivery", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		events := newBlockingCreatedEventSink()
		fixture.service.events = events
		baseRun := fixture.runner.run
		fixture.runner.run = func(call gitInvocation) gitResponse {
			switch strings.Join(call.args, " ") {
			case "status --porcelain=v2 --branch -z":
				return gitResponse{stdout: []byte("# branch.oid created-head\x00# branch.head canceled-after-ready\x00")}
			case "diff --numstat -z", "diff --cached --numstat -z", "branch -r --contains HEAD":
				return gitResponse{}
			case "rev-list --count main..HEAD":
				return gitResponse{stdout: []byte("0\n")}
			default:
				return baseRun(call)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan *Worktree, 1)
		errResult := make(chan error, 1)
		go func() {
			item, err := fixture.service.CreateReady(
				ctx,
				fixture.workspace.ID,
				CreateOptions{Name: "Canceled After Ready"},
			)
			result <- item
			errResult <- err
		}()

		select {
		case <-events.entered:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for completed creation event")
		}
		rows, err := fixture.store.List(context.Background(), fixture.workspace.ID)
		if err != nil || len(rows) != 1 || rows[0].State != StateReady {
			t.Fatalf("rows before delivery cancellation = %#v, %v, want one ready row", rows, err)
		}
		if err := os.WriteFile(
			filepath.Join(rows[0].Path, ".git"),
			[]byte("gitdir: "+fixture.adminGitDir+"\n"),
			0o600,
		); err != nil {
			t.Fatalf("write worktree pointer: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fixture.adminGitDir, "commondir"), []byte("../..\n"), 0o600); err != nil {
			t.Fatalf("write common pointer: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(fixture.adminGitDir, "gitdir"),
			[]byte(filepath.Join(rows[0].Path, ".git")+"\n"),
			0o600,
		); err != nil {
			t.Fatalf("write worktree backlink: %v", err)
		}
		cancel()
		close(events.release)
		if item := <-result; item != nil {
			t.Fatalf("CreateReady() item = %#v, want nil after caller cancellation", item)
		}
		createErr := <-errResult
		if !errors.Is(createErr, context.Canceled) {
			t.Fatalf("CreateReady() error = %v, want context.Canceled", createErr)
		}
		stored, err := fixture.store.Get(context.Background(), fixture.workspace.ID, rows[0].ID)
		if err != nil || stored.State != StateRemoved {
			t.Fatalf(
				"worktree after delivery cancellation = %#v, %v (create error %v), want removed tombstone",
				stored,
				err,
				createErr,
			)
		}
	})

	t.Run("Should reuse an existing branch without minting or configuring it", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		item, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{
			Name: "Existing", ExistingBranch: "feature/existing",
		})
		if err != nil {
			t.Fatalf("Create(existing branch) error = %v", err)
		}
		if item.CreatedBranch {
			t.Fatal("CreatedBranch = true, want false")
		}
		for _, command := range invocationCommands(fixture.runner.invocations()) {
			if strings.HasPrefix(command, "branch feature/existing ") || strings.HasPrefix(command, "config branch.feature/existing") {
				t.Fatalf("existing branch received a mint/config mutation: %q", command)
			}
		}
	})

	t.Run("Should reject a registered name before invoking Git", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		now := time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)
		if err := fixture.store.Insert(context.Background(), Worktree{
			ID: "wt-existing", WorkspaceID: fixture.workspace.ID, Name: "docs-refresh",
			Path: filepath.Join(fixture.worktreesRoot, "existing"), State: StateReady,
			Origin: OriginManual, SetupState: SetupNone, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		_, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{Name: "Docs Refresh"})
		if !errors.Is(err, ErrNameTaken) {
			t.Fatalf("Create(duplicate) error = %v, want ErrNameTaken", err)
		}
		if calls := fixture.runner.invocations(); len(calls) != 0 {
			t.Fatalf("Git calls = %#v, want none for a registered collision", calls)
		}
	})

	t.Run("Should classify pending-row constraint failures without leaking storage errors", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			err  error
			want error
		}{
			{name: "path", err: errors.New("UNIQUE constraint failed: worktrees.path"), want: ErrPathExists},
			{name: "name", err: errors.New("UNIQUE constraint failed: worktrees.workspace_id, worktrees.name"), want: ErrNameTaken},
			{name: "other", err: errors.New("database unavailable")},
		}
		for _, testCase := range cases {
			t.Run("Should map "+testCase.name+" insertion failure", func(t *testing.T) {
				t.Parallel()
				fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
				fixture.service.store = &insertErrorWorktreeStore{
					memoryWorktreeStore: fixture.store,
					err:                 testCase.err,
				}
				_, err := fixture.service.Create(
					context.Background(), fixture.workspace.ID, CreateOptions{Name: "Insert Conflict"},
				)
				if testCase.want != nil {
					if !errors.Is(err, testCase.want) {
						t.Fatalf("Create() error = %v, want %v", err, testCase.want)
					}
					return
				}
				if !errors.Is(err, testCase.err) || !strings.Contains(err.Error(), "insert pending row") {
					t.Fatalf("Create() error = %v, want wrapped storage error", err)
				}
			})
		}
	})

	t.Run("Should let exactly one concurrent creation own a name", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		start := make(chan struct{})
		results := make(chan error, 2)
		var group sync.WaitGroup
		for range 2 {
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				_, err := fixture.service.Create(
					context.Background(), fixture.workspace.ID, CreateOptions{Name: "Concurrent"},
				)
				results <- err
			}()
		}
		close(start)
		group.Wait()
		close(results)
		successes, conflicts := 0, 0
		for err := range results {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrNameTaken):
				conflicts++
			default:
				t.Fatalf("Create(concurrent) error = %v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("concurrent results = %d success, %d conflicts, want one each", successes, conflicts)
		}
	})

	t.Run("Should classify branches held by the root or another worktree", func(t *testing.T) {
		t.Parallel()
		rootFixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		if _, err := rootFixture.service.Create(context.Background(), rootFixture.workspace.ID, CreateOptions{
			Name: "Root Branch", Branch: "main",
		}); !errors.Is(err, ErrBranchCheckedOutAtRoot) {
			t.Fatalf("Create(root branch) error = %v, want ErrBranchCheckedOutAtRoot", err)
		}

		linkedFixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		linkedFixture.listOutput = append(
			linkedFixture.listOutput,
			[]byte("worktree /linked\x00HEAD held\x00branch refs/heads/feature/held\x00\x00")...,
		)
		if _, err := linkedFixture.service.Create(context.Background(), linkedFixture.workspace.ID, CreateOptions{
			Name: "Held Branch", Branch: "feature/held",
		}); !errors.Is(err, ErrBranchHeld) {
			t.Fatalf("Create(held branch) error = %v, want ErrBranchHeld", err)
		}
	})

	t.Run("Should reject missing refs and repositories without commits before mutation", func(t *testing.T) {
		t.Parallel()
		existingFixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		if _, err := existingFixture.service.Create(context.Background(), existingFixture.workspace.ID, CreateOptions{
			Name: "Missing Existing", ExistingBranch: "feature/missing",
		}); !errors.Is(err, ErrBaseRefNotFound) {
			t.Fatalf("Create(missing existing branch) error = %v, want ErrBaseRefNotFound", err)
		}

		baseFixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		baseRun := baseFixture.runner.run
		baseFixture.runner.run = func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") == "rev-parse --verify missing^{commit}" {
				return gitResponse{stderr: []byte("unknown revision"), err: errors.New("exit 128")}
			}
			return baseRun(call)
		}
		if _, err := baseFixture.service.Create(context.Background(), baseFixture.workspace.ID, CreateOptions{
			Name: "Missing Base", BaseRef: "missing",
		}); !errors.Is(err, ErrBaseRefNotFound) {
			t.Fatalf("Create(missing base) error = %v, want ErrBaseRefNotFound", err)
		}

		unbornFixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		unbornRun := unbornFixture.runner.run
		unbornFixture.runner.run = func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") == "rev-parse --verify HEAD" {
				return gitResponse{stderr: []byte("unknown revision"), err: errors.New("exit 128")}
			}
			return unbornRun(call)
		}
		if _, err := unbornFixture.service.Create(context.Background(), unbornFixture.workspace.ID, CreateOptions{
			Name: "Unborn",
		}); !errors.Is(err, ErrRepoHasNoCommits) {
			t.Fatalf("Create(unborn) error = %v, want ErrRepoHasNoCommits", err)
		}
	})

	t.Run("Should unwind a failed checkout and allow an exact retry", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		fixture.failWorktreeAdd.Store(true)
		_, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{Name: "Retry Me"})
		if err == nil || !strings.Contains(err.Error(), "checkout failed") {
			t.Fatalf("Create(failing checkout) error = %v, want checkout failure", err)
		}
		if item, getErr := fixture.store.Get(context.Background(), fixture.workspace.ID, "retry-me"); !errors.Is(getErr, ErrNotFound) || item != nil {
			t.Fatalf("failed row = %#v, %v, want deleted", item, getErr)
		}
		commands := invocationCommands(fixture.runner.invocations())
		if !containsCommandPrefix(commands, "update-ref -d refs/heads/retry-me created-head") {
			t.Fatalf("rollback Git calls = %#v, want compare-and-delete", commands)
		}
		fixture.failWorktreeAdd.Store(false)
		if _, retryErr := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{Name: "Retry Me"}); retryErr != nil {
			t.Fatalf("Create(retry) error = %v", retryErr)
		}
	})

	t.Run("Should copy ignored files without overwriting the checkout", func(t *testing.T) {
		t.Parallel()
		worktreesConfig := config.DefaultWorktreesConfig()
		worktreesConfig.CopyList = []string{".env"}
		fixture := newCreateTestFixture(t, worktreesConfig)
		if err := os.WriteFile(filepath.Join(fixture.workspace.Root, ".env"), []byte("source"), 0o600); err != nil {
			t.Fatalf("write bootstrap source: %v", err)
		}
		item, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{Name: "Bootstrap"})
		if err != nil {
			t.Fatalf("Create(bootstrap) error = %v", err)
		}
		contents, err := os.ReadFile(filepath.Join(item.Path, ".env"))
		if err != nil {
			t.Fatalf("read copied bootstrap file: %v", err)
		}
		if string(contents) != "source" {
			t.Fatalf("copied bootstrap contents = %q, want source", contents)
		}
		if err := os.WriteFile(filepath.Join(item.Path, ".env"), []byte("existing"), 0o600); err != nil {
			t.Fatalf("replace target fixture: %v", err)
		}
		if err := fixture.service.copyBootstrapFile(fixture.workspace.Root, item.Path, ".env"); err != nil {
			t.Fatalf("copyBootstrapFile(existing) error = %v", err)
		}
		contents, err = os.ReadFile(filepath.Join(item.Path, ".env"))
		if err != nil || string(contents) != "existing" {
			t.Fatalf("existing bootstrap target = %q, %v, want untouched", contents, err)
		}
	})

	t.Run("Should cancel only a pending creation and unwind recorded artifacts", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		events := &recordingEventSink{}
		fixture.service.events = events
		target := filepath.Join(fixture.worktreesRoot, "cancel")
		if err := os.MkdirAll(target, 0o700); err != nil {
			t.Fatalf("create pending target: %v", err)
		}
		now := time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)
		pending := Worktree{
			ID: "wt-cancel", WorkspaceID: fixture.workspace.ID, Name: "cancel", Branch: "cancel",
			Path: target, GitDir: fixture.adminGitDir, State: StatePending, PendingPhase: PhaseCheckout,
			Origin: OriginManual, SetupState: SetupNone, CreatedBranch: true, CreatedHead: "created-head",
			CreatedAt: now, UpdatedAt: now,
		}
		if err := fixture.store.Insert(context.Background(), pending); err != nil {
			t.Fatalf("seed pending worktree: %v", err)
		}
		if err := fixture.service.CancelCreate(context.Background(), fixture.workspace.ID, pending.ID); err != nil {
			t.Fatalf("CancelCreate() error = %v", err)
		}
		if _, err := fixture.store.Get(context.Background(), fixture.workspace.ID, pending.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get(canceled) error = %v, want ErrNotFound", err)
		}
		if got := events.count(EventCreationCanceled); got != 1 {
			t.Fatalf("creation canceled events = %d, want one", got)
		}
		ready := pending
		ready.ID, ready.Name, ready.Path, ready.State = "wt-ready", "ready", filepath.Join(fixture.worktreesRoot, "ready"), StateReady
		if err := fixture.store.Insert(context.Background(), ready); err != nil {
			t.Fatalf("seed ready worktree: %v", err)
		}
		if err := fixture.service.CancelCreate(context.Background(), fixture.workspace.ID, ready.ID); !errors.Is(err, ErrNotPending) {
			t.Fatalf("CancelCreate(ready) error = %v, want ErrNotPending", err)
		}
		if err := fixture.service.CancelCreate(context.Background(), "ws-other", ready.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("CancelCreate(cross-workspace) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("Should allowlist setup environment and report timeout without exposing unrelated values", func(t *testing.T) {
		t.Parallel()
		workspace := Workspace{ID: "ws-setup", Root: t.TempDir()}
		item := Worktree{ID: "wt-setup", Path: t.TempDir(), Branch: "feature/setup"}
		got := setupEnvironment([]string{
			"PATH=/bin", "HOME=/operator", "LANG=en_US.UTF-8", "LC_ALL=C", "TMPDIR=/tmp",
			"SHELL=/bad", "GIT_TOKEN=secret", "CLAIM_TOKEN=secret", "UNRELATED=value",
		}, workspace, item, "/bin/sh")
		joined := strings.Join(got, "\n")
		for _, required := range []string{
			"PATH=/bin", "HOME=/operator", "LANG=en_US.UTF-8", "LC_ALL=C", "TMPDIR=/tmp",
			"SHELL=/bin/sh", "COMPOZY_WORKSPACE_ROOT=" + workspace.Root,
			"COMPOZY_WORKTREE_PATH=" + item.Path, "COMPOZY_WORKTREE_ID=wt-setup",
			"COMPOZY_WORKTREE_BRANCH=feature/setup",
		} {
			if !strings.Contains(joined, required) {
				t.Fatalf("setup environment = %#v, missing %q", got, required)
			}
		}
		if strings.Contains(joined, "GIT_TOKEN") || strings.Contains(joined, "CLAIM_TOKEN") || strings.Contains(joined, "UNRELATED") {
			t.Fatalf("setup environment leaked unrelated values: %#v", got)
		}

		worktreesConfig := config.DefaultWorktreesConfig()
		worktreesConfig.SetupCommand = "printf timed-out >&2; sleep 5"
		worktreesConfig.SetupTimeout = 30 * time.Millisecond
		service := NewService(newMemoryWorktreeStore(), &recordingGitRunner{}, WithConfig(worktreesConfig, t.TempDir()))
		state, detail := service.runSetup(context.Background(), workspace, item, true)
		if state != SetupFailed || detail == "" || !strings.Contains(detail, context.DeadlineExceeded.Error()) {
			t.Fatalf("runSetup(timeout) = (%q, %q), want readable failed timeout", state, detail)
		}
	})

	t.Run("Should reject explicit paths that overlap workspace or live worktree roots", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		if _, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{
			Name: "Inside", Path: filepath.Join(fixture.workspace.Root, "nested"),
		}); !errors.Is(err, ErrPathExists) {
			t.Fatalf("Create(overlapping workspace) error = %v, want ErrPathExists", err)
		}
		now := time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)
		existingPath := filepath.Join(fixture.worktreesRoot, "existing")
		if err := fixture.store.Insert(context.Background(), Worktree{
			ID: "wt-existing-path", WorkspaceID: fixture.workspace.ID, Name: "existing-path",
			Path: existingPath, State: StateReady, Origin: OriginManual, SetupState: SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed live path: %v", err)
		}
		if _, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{
			Name: "Overlap", Path: filepath.Join(existingPath, "nested"),
		}); !errors.Is(err, ErrPathExists) {
			t.Fatalf("Create(overlapping worktree) error = %v, want ErrPathExists", err)
		}
		if _, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{
			Name: "Ancestor", Path: filepath.Dir(existingPath),
		}); !errors.Is(err, ErrPathExists) {
			t.Fatalf("Create(ancestor of worktree) error = %v, want ErrPathExists", err)
		}

		otherWorkspace := Workspace{ID: "ws-other", Name: "Other", Root: t.TempDir()}
		otherPath := filepath.Join(fixture.worktreesRoot, "other-existing")
		if err := fixture.store.Insert(context.Background(), Worktree{
			ID: "wt-other-path", WorkspaceID: otherWorkspace.ID, Name: "other-path",
			Path: otherPath, State: StateReady, Origin: OriginManual, SetupState: SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed other-workspace path: %v", err)
		}
		fixture.service.resolver = staticWorkspaceResolver{workspaces: map[string]Workspace{
			fixture.workspace.ID: fixture.workspace,
			otherWorkspace.ID:    otherWorkspace,
		}}
		if _, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{
			Name: "Cross Workspace", Path: filepath.Join(otherPath, "nested"),
		}); !errors.Is(err, ErrPathExists) {
			t.Fatalf("Create(cross-workspace overlap) error = %v, want ErrPathExists", err)
		}
	})

	t.Run("Should revalidate canonical containment between Git mutations", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		baseRun := fixture.runner.run
		inserted := false
		fixture.runner.run = func(call gitInvocation) gitResponse {
			if !inserted && strings.HasPrefix(strings.Join(call.args, " "), "branch race-revalidation ") {
				inserted = true
				now := time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)
				candidate := filepath.Join(fixture.worktreesRoot, "create-workspace", "race-revalidation")
				if err := fixture.store.Insert(context.Background(), Worktree{
					ID: "wt-racing-path", WorkspaceID: fixture.workspace.ID, Name: "racing-path",
					Path: filepath.Dir(candidate), State: StateReady, Origin: OriginManual,
					SetupState: SetupNone, CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed racing path: %v", err)
				}
			}
			return baseRun(call)
		}
		_, err := fixture.service.Create(
			context.Background(), fixture.workspace.ID, CreateOptions{Name: "Race Revalidation"},
		)
		if !errors.Is(err, ErrPathExists) {
			t.Fatalf("Create(racing containment) error = %v, want ErrPathExists", err)
		}
		if containsCommandPrefix(invocationCommands(fixture.runner.invocations()), "worktree add ") {
			t.Fatal("checkout mutation ran after containment changed")
		}
	})

	t.Run("Should preserve derived-path provenance when the configured root symlink changes", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		safeRoot := t.TempDir()
		otherRoot := t.TempDir()
		rootLink := filepath.Join(t.TempDir(), "worktrees-root")
		if err := os.Symlink(safeRoot, rootLink); err != nil {
			t.Fatalf("Symlink(safe root) error = %v", err)
		}
		item := Worktree{
			ID: "wt-derived-provenance", WorkspaceID: fixture.workspace.ID,
			Path: filepath.Join(safeRoot, "workspace", "derived"),
		}
		if err := os.Remove(rootLink); err != nil {
			t.Fatalf("Remove(root symlink) error = %v", err)
		}
		if err := os.Symlink(otherRoot, rootLink); err != nil {
			t.Fatalf("Symlink(replaced root) error = %v", err)
		}
		if err := fixture.service.revalidateMutationPath(
			context.Background(), fixture.workspace, item, rootLink, false,
		); !errors.Is(err, ErrConfigInvalid) {
			t.Fatalf("revalidateMutationPath(derived escape) error = %v, want ErrConfigInvalid", err)
		}
	})

	t.Run("Should reject a bootstrap destination parent symlink that escapes the worktree", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		sourceRoot := t.TempDir()
		targetRoot := t.TempDir()
		outside := t.TempDir()
		if err := os.MkdirAll(filepath.Join(sourceRoot, "nested"), 0o700); err != nil {
			t.Fatalf("MkdirAll(source nested) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourceRoot, "nested", "secret.env"), []byte("safe\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(source) error = %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(targetRoot, "nested")); err != nil {
			t.Fatalf("Symlink(destination escape) error = %v", err)
		}
		if err := fixture.service.copyBootstrapFile(
			sourceRoot, targetRoot, filepath.Join("nested", "secret.env"),
		); !errors.Is(err, ErrConfigInvalid) {
			t.Fatalf("copyBootstrapFile(symlink escape) error = %v, want ErrConfigInvalid", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "secret.env")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("outside destination stat error = %v, want no copied file", err)
		}
	})

	t.Run("Should redact setup and Git diagnostics at the worktree boundary", func(t *testing.T) {
		t.Parallel()
		const secret = "bound-worktree-secret-123456"
		cleanup := diagnostics.RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)
		workspace := Workspace{ID: "ws-redaction", Root: t.TempDir()}
		item := Worktree{ID: "wt-redaction", Path: t.TempDir(), Branch: "feature/redaction"}
		settings := config.DefaultWorktreesConfig()
		settings.SetupCommand = "printf 'token=" + secret + "' >&2; exit 1"
		service := NewService(newMemoryWorktreeStore(), &recordingGitRunner{}, WithConfig(settings, t.TempDir()))
		service.getenv = func(string) string { return "/bin/sh" }
		service.environ = func() []string {
			return []string{"PATH=/bin", "BOUND_SECRET=" + secret, "CLAIM_TOKEN=compozy_claim_" + secret}
		}
		state, detail := service.runSetup(context.Background(), workspace, item, true)
		if state != SetupFailed || strings.Contains(detail, secret) || !strings.Contains(detail, "[REDACTED]") {
			t.Fatalf("runSetup(secret) = (%q, %q), want redacted failure", state, detail)
		}
		for _, entry := range setupEnvironment(service.environ(), workspace, item, "/bin/sh") {
			if strings.Contains(entry, secret) {
				t.Fatalf("setup environment leaked registered secret: %q", entry)
			}
		}
		gitErr := gitCommandError([]string{"status"}, "token="+secret, errors.New("exit 1"))
		if strings.Contains(gitErr.Error(), secret) || !strings.Contains(gitErr.Error(), "[REDACTED]") {
			t.Fatalf("git diagnostic = %q, want redacted", gitErr)
		}
		refusalErr := refusal(ErrDeniedByHook, "token="+secret)
		if strings.Contains(refusalErr.Error(), secret) || !strings.Contains(refusalErr.Error(), "[REDACTED]") {
			t.Fatalf("refusal diagnostic = %q, want redacted", refusalErr)
		}
		requestType := reflect.TypeOf(RunWorktreeRequest{})
		for index := range requestType.NumField() {
			fieldName := strings.ToLower(requestType.Field(index).Name)
			if strings.Contains(fieldName, "claim") || strings.Contains(fieldName, "token") || strings.Contains(fieldName, "secret") {
				t.Fatalf("RunWorktreeRequest exposes secret-bearing field %q", requestType.Field(index).Name)
			}
		}
	})

	t.Run("Should reject an unwritable placement parent before mutating Git", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		blockedRoot := filepath.Join(t.TempDir(), "blocked")
		if err := os.WriteFile(blockedRoot, []byte("not a directory"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", blockedRoot, err)
		}
		fixture.service.Reconfigure(config.DefaultWorktreesConfig(), blockedRoot)

		_, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{Name: "blocked"})
		if !errors.Is(err, ErrConfigInvalid) {
			t.Fatalf("Create() error = %v, want ErrConfigInvalid", err)
		}
		got, getErr := fixture.store.Get(context.Background(), fixture.workspace.ID, "blocked")
		if !errors.Is(getErr, ErrNotFound) || got != nil {
			t.Fatalf("registry row = %#v, want rollback deletion", got)
		}
		for _, call := range fixture.runner.invocations() {
			if len(call.args) > 0 && (call.args[0] == "branch" || call.args[0] == "worktree") {
				t.Fatalf("mutating Git call = %#v, want none", call.args)
			}
		}
	})
}

type blockingCreatedEventSink struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingCreatedEventSink() *blockingCreatedEventSink {
	return &blockingCreatedEventSink{entered: make(chan struct{}), release: make(chan struct{})}
}

func (s *blockingCreatedEventSink) PublishWorktreeEvent(_ context.Context, event LifecycleEvent) error {
	if event.Name == EventCreated {
		s.once.Do(func() { close(s.entered) })
		<-s.release
	}
	return nil
}

type insertErrorWorktreeStore struct {
	*memoryWorktreeStore
	err error
}

func (s *insertErrorWorktreeStore) Insert(context.Context, Worktree) error {
	return s.err
}

func invocationCommands(calls []gitInvocation) []string {
	commands := make([]string, 0, len(calls))
	for _, call := range calls {
		commands = append(commands, strings.Join(call.args, " "))
	}
	return commands
}

func containsCommandPrefix(commands []string, prefix string) bool {
	for _, command := range commands {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}
