package extensionpkg

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/testutil"
)

func TestManagerStopShutdownErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should return shutdown error when wait succeeds", func(t *testing.T) {
		t.Parallel()

		shutdownErr := errors.New("shutdown failed")
		proc := newFakeProcess(901)
		proc.shutdownFn = func(context.Context) error {
			proc.close(nil)
			return shutdownErr
		}

		manager := NewManager(nil)
		lifecycleCtx, cancel := context.WithCancel(context.Background())
		manager.mu.Lock()
		manager.started = true
		manager.lifecycleCtx = lifecycleCtx
		manager.cancel = cancel
		manager.extensions = map[string]*managedExtension{
			"ext-stop-error": {
				info: ExtensionInfo{
					Name:    "ext-stop-error",
					Version: "1.0.0",
					Source:  SourceUser,
					Enabled: true,
				},
				manifest: &Manifest{
					Name:    "ext-stop-error",
					Version: "1.0.0",
				},
				process:    proc,
				active:     true,
				registered: true,
			},
		}
		manager.mu.Unlock()

		err := manager.Stop(testutil.Context(t))
		if !errors.Is(err, shutdownErr) {
			t.Fatalf("Stop() error = %v, want shutdown error", err)
		}
	})

	t.Run("Should return shutdown deadline without waiting forever for process exit", func(t *testing.T) {
		t.Parallel()

		proc := newFakeProcess(903)
		proc.shutdownFn = func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}
		manager := NewManager(nil)
		lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
		manager.mu.Lock()
		manager.started = true
		manager.lifecycleCtx = lifecycleCtx
		manager.cancel = lifecycleCancel
		manager.extensions = map[string]*managedExtension{
			"ext-stop-deadline": {
				info: ExtensionInfo{
					Name:    "ext-stop-deadline",
					Version: "1.0.0",
					Source:  SourceUser,
					Enabled: true,
				},
				manifest: &Manifest{
					Name:    "ext-stop-deadline",
					Version: "1.0.0",
				},
				process:    proc,
				active:     true,
				registered: true,
			},
		}
		manager.mu.Unlock()
		manager.defaultShutdownTimeout = 10 * time.Millisecond

		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		err := manager.Stop(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop() error = %v, want detached shutdown deadline", err)
		}
		if proc.shutdownCount() != 1 {
			t.Fatalf("Shutdown() count = %d, want 1", proc.shutdownCount())
		}
	})
}

func TestManagerFailedLaunchCleanupKeepsRedactionThroughShutdown(t *testing.T) {
	t.Parallel()

	t.Run("Should unregister dynamic secrets only after the failed process is reaped", func(t *testing.T) {
		t.Parallel()

		secret := "runtime-cleanup-secret-510793"
		cleanup := diagnostics.RegisterDynamicSecret(secret)
		process := newFakeProcess(904)
		redactedDuringShutdown := false
		process.shutdownFn = func(context.Context) error {
			redactedDuringShutdown = !strings.Contains(diagnostics.Redact("stderr "+secret), secret)
			process.close(nil)
			return nil
		}
		manager := NewManager(nil)
		cause := errors.New("initialization failed")
		key := GlobalInstanceKey("runtime-cleanup")
		ownership := manager.newStartupRuntimeOwnership(key, process, []func(){cleanup})
		transaction := newExtensionStartupTransaction(manager, &managedExtension{key: key})
		transaction.add("redaction cleanup", func(context.Context) error {
			return ownership.releaseRedaction()
		})
		transaction.add("subprocess", ownership.stop)
		err := transaction.rollback(testutil.Context(t), cause)
		if !errors.Is(err, cause) {
			t.Fatalf("startup rollback error = %v, want initialization failure", err)
		}
		if !redactedDuringShutdown {
			t.Fatal("dynamic secret was unregistered before process shutdown completed")
		}
		if got := diagnostics.Redact("after " + secret); !strings.Contains(got, secret) {
			t.Fatalf("dynamic secret remained registered after cleanup: %q", got)
		}
	})
}

func TestManagerResourceSourceCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should remove persisted source state when stop receives a canceled context", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		resourceKernel, err := resources.NewKernel(env.registry.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		key := GlobalInstanceKey("ext-canceled-stop-cleanup")
		manager, _ := startedManagerWithProcess(env.registry, key.Name)
		manager.sourceSessions = resourceKernel
		seedManagedSourceRecord(t, resourceKernel, key, "nonce-ext-canceled-stop-cleanup", "canceled-stop-record")

		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if err := manager.Stop(ctx); err != nil {
			t.Fatalf("Stop(canceled context) error = %v", err)
		}
		assertManagedSourceEmpty(t, resourceKernel, env.db, key)
	})

	t.Run("Should drain an admitted development operation when stop context is canceled", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		resourceKernel, err := resources.NewKernel(env.registry.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		key := GlobalInstanceKey("ext-canceled-stop-admission")
		manager, _ := startedManagerWithProcess(env.registry, key.Name)
		manager.sourceSessions = resourceKernel
		seedManagedSourceRecord(
			t,
			resourceKernel,
			key,
			"nonce-ext-canceled-stop-admission",
			"canceled-stop-admission-record",
		)
		operation, err := manager.beginDevOperation(
			testutil.Context(t),
			InstanceKey{Name: "admitted-dev-operation", WorkspaceID: "workspace-admission"},
		)
		if err != nil {
			t.Fatalf("beginDevOperation() error = %v", err)
		}
		t.Cleanup(operation.close)

		stopCtx, cancelStop := context.WithCancel(testutil.Context(t))
		cancelStop()
		stopResult := make(chan error, 1)
		go func() {
			stopResult <- manager.Stop(stopCtx)
		}()
		<-manager.lifecycleDone()
		select {
		case err := <-stopResult:
			operation.close()
			t.Fatalf("Stop(canceled context) returned before draining admission: %v", err)
		case <-time.After(100 * time.Millisecond):
		}

		operation.close()
		if err := <-stopResult; err != nil {
			t.Fatalf("Stop(canceled context) error = %v", err)
		}
		assertManagedSourceEmpty(t, resourceKernel, env.db, key)
		manager.mu.RLock()
		started := manager.started
		stopping := manager.stopping
		manager.mu.RUnlock()
		if started || stopping {
			t.Fatalf("manager lifecycle = started %v stopping %v, want false/false", started, stopping)
		}
	})

	t.Run("Should stop every extension when the admitted development drain times out", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		key := GlobalInstanceKey("ext-drain-timeout")
		manager, proc := startedManagerWithProcess(env.registry, key.Name)
		manager.defaultShutdownTimeout = 20 * time.Millisecond
		operation, err := manager.beginDevOperation(
			testutil.Context(t),
			InstanceKey{Name: "stuck-dev-operation", WorkspaceID: "workspace-drain-timeout"},
		)
		if err != nil {
			t.Fatalf("beginDevOperation() error = %v", err)
		}
		t.Cleanup(operation.close)

		var supervisionJoined atomic.Bool
		released := make(chan struct{})
		manager.wg.Go(func() {
			<-released
			supervisionJoined.Store(true)
		})
		proc.shutdownFn = func(context.Context) error {
			close(released)
			proc.close(nil)
			return nil
		}

		stopErr := manager.Stop(testutil.Context(t))
		if !errors.Is(stopErr, context.DeadlineExceeded) {
			t.Fatalf("Stop() error = %v, want the admitted development drain deadline", stopErr)
		}
		if got := proc.shutdownCount(); got != 1 {
			t.Fatalf("Shutdown() count = %d, want 1", got)
		}
		if !supervisionJoined.Load() {
			t.Fatal("Stop() returned before joining supervision goroutines")
		}
		manager.mu.RLock()
		started := manager.started
		stopping := manager.stopping
		manager.mu.RUnlock()
		if started || stopping {
			t.Fatalf("manager lifecycle = started %v stopping %v, want false/false", started, stopping)
		}
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Start() after a drained shutdown error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t)); err != nil {
				t.Fatalf("Manager.Stop() cleanup error = %v", err)
			}
		})
	})

	t.Run("Should block reload until pending source cleanup succeeds", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		resourceKernel, err := resources.NewKernel(env.registry.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		sourceSessions := &faultingSourceSessionManager{delegate: resourceKernel}
		fixture := createManagerTestExtension(t, managerTestManifest("ext-pending-reload", managerManifestOptions{
			command: "fake-extension",
		}), nil)
		installManagerFixture(t, env.registry, fixture, SourceUser, true)
		firstProcess := newFakeProcess(906)
		secondProcess := newFakeProcess(907)
		launcher := &fakeLauncher{queue: []*fakeProcess{firstProcess, secondProcess}}
		manager := NewManager(
			env.registry,
			WithSourceSessionManager(sourceSessions),
			withProcessLauncher(launcher.launch),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			sourceSessions.setResetError(nil)
			if err := manager.Stop(testutil.Context(t)); err != nil {
				t.Fatalf("Manager.Stop() cleanup error = %v", err)
			}
		})

		key := GlobalInstanceKey("ext-pending-reload")
		manager.mu.RLock()
		initial := manager.instanceLocked(key)
		if initial == nil {
			manager.mu.RUnlock()
			t.Fatal("initial extension = nil")
		}
		initialNonce := initial.sessionNonce
		manager.mu.RUnlock()
		if initialNonce == "" {
			t.Fatal("initial extension session nonce = empty")
		}
		seedManagedSourceRecord(t, resourceKernel, key, initialNonce, "stale-before-reload")

		storageErr := errors.New("source storage unavailable")
		sourceSessions.setResetError(storageErr)
		if err := manager.Reload(testutil.Context(t)); !errors.Is(err, storageErr) {
			t.Fatalf("Manager.Reload(pending cleanup) error = %v, want source storage failure", err)
		}
		manager.mu.RLock()
		started := manager.started
		stopping := manager.stopping
		pending := append([]pendingExtensionCleanup(nil), manager.pendingCleanups...)
		manager.mu.RUnlock()
		if started || stopping {
			t.Fatalf("manager after blocked reload = started %v stopping %v, want false/false", started, stopping)
		}
		if len(pending) != 1 || pending[0].sourceSessionNonce != initialNonce {
			t.Fatalf("pending cleanup after blocked reload = %#v, want initial nonce %q", pending, initialNonce)
		}
		if got := launcher.launchCount(); got != 1 {
			t.Fatalf("process launch count after blocked reload = %d, want 1", got)
		}
		assertManagedSourceState(t, resourceKernel, env.db, key, 1, initialNonce)

		sourceSessions.setResetError(nil)
		if err := manager.Reload(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Reload(retry cleanup) error = %v", err)
		}
		manager.mu.RLock()
		reloaded := manager.instanceLocked(key)
		if reloaded == nil {
			manager.mu.RUnlock()
			t.Fatal("reloaded extension = nil")
		}
		reloadedNonce := reloaded.sessionNonce
		pendingCount := len(manager.pendingCleanups)
		manager.mu.RUnlock()
		if reloadedNonce == "" || reloadedNonce == initialNonce {
			t.Fatalf("reloaded session nonce = %q, want new non-empty nonce", reloadedNonce)
		}
		if pendingCount != 0 {
			t.Fatalf("pending cleanup count after reload retry = %d, want 0", pendingCount)
		}
		if got := launcher.launchCount(); got != 2 {
			t.Fatalf("process launch count after reload retry = %d, want 2", got)
		}
		assertManagedSourceState(t, resourceKernel, env.db, key, 0, reloadedNonce)
	})

	t.Run("Should reset extension source on stop", func(t *testing.T) {
		t.Parallel()

		sourceSessions := &recordingSourceSessionManager{}
		manager, proc := startedManagerWithProcess(nil, "ext-stop-cleanup")
		manager.sourceSessions = sourceSessions
		source := extensionResourceSource("ext-stop-cleanup")
		if err := sourceSessions.ActivateSourceSession(
			testutil.Context(t),
			extensionManagerResourceActor(),
			source,
			"nonce-ext-stop-cleanup",
		); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if !sourceSessions.hasReset(source) {
			t.Fatalf("ResetSource() calls = %#v, want ext-stop-cleanup source reset", sourceSessions.resetSources())
		}
		if proc.shutdownCount() != 1 {
			t.Fatalf("Shutdown() count = %d, want 1", proc.shutdownCount())
		}
	})

	t.Run("Should reset extension source when disabling failed extension", func(t *testing.T) {
		t.Parallel()

		sourceSessions := &recordingSourceSessionManager{}
		env := newRegistryTestEnv(t)
		manager, _ := startedManagerWithProcess(env.registry, "ext-disable-cleanup")
		manager.sourceSessions = sourceSessions
		source := extensionResourceSource("ext-disable-cleanup")
		if err := sourceSessions.ActivateSourceSession(
			testutil.Context(t),
			extensionManagerResourceActor(),
			source,
			"nonce-ext-disable-cleanup",
		); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		manager.disableExtension("ext-disable-cleanup", errors.New("too many failures"))

		if !sourceSessions.hasReset(source) {
			t.Fatalf("ResetSource() calls = %#v, want ext-disable-cleanup source reset", sourceSessions.resetSources())
		}
		loaded, ok := manager.lookupManaged("ext-disable-cleanup")
		if !ok {
			t.Fatal("lookupManaged(ext-disable-cleanup) = false, want true")
		}
		if loaded.registered {
			t.Fatal("registered = true, want false after disable cleanup")
		}
	})

	t.Run("Should retry a failed source cleanup after stop completes", func(t *testing.T) {
		t.Parallel()

		storageErr := errors.New("source storage unavailable")
		sourceSessions := &recordingSourceSessionManager{resetErr: storageErr}
		manager, _ := startedManagerWithProcess(nil, "ext-retry-cleanup")
		manager.sourceSessions = sourceSessions
		source := extensionResourceSource("ext-retry-cleanup")
		if err := sourceSessions.ActivateSourceSession(
			testutil.Context(t),
			extensionManagerResourceActor(),
			source,
			"nonce-ext-retry-cleanup",
		); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		if err := manager.Stop(testutil.Context(t)); !errors.Is(err, storageErr) {
			t.Fatalf("Stop() error = %v, want source storage failure", err)
		}
		manager.mu.RLock()
		pending := append([]pendingExtensionCleanup(nil), manager.pendingCleanups...)
		manager.mu.RUnlock()
		if len(pending) != 1 {
			t.Fatalf("pending cleanup count = %d, want 1", len(pending))
		}
		if pending[0].source != source.Normalize() ||
			pending[0].sourceSessionNonce != "nonce-ext-retry-cleanup" {
			t.Fatalf(
				"pending source cleanup = source %#v nonce %q, want %#v/%q",
				pending[0].source,
				pending[0].sourceSessionNonce,
				source.Normalize(),
				"nonce-ext-retry-cleanup",
			)
		}
		if got := sourceSessions.activeNonce(source); got != "nonce-ext-retry-cleanup" {
			t.Fatalf("active source nonce after failed stop = %q, want retained cleanup nonce", got)
		}

		sourceSessions.setResetError(nil)
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop(retry pending cleanup) error = %v", err)
		}
		manager.mu.RLock()
		pendingCount := len(manager.pendingCleanups)
		manager.mu.RUnlock()
		if pendingCount != 0 {
			t.Fatalf("pending cleanup count after retry = %d, want 0", pendingCount)
		}
		if got := sourceSessions.activeNonce(source); got != "" {
			t.Fatalf("active source nonce after retry = %q, want empty", got)
		}
	})

	t.Run("Should preserve a winner nonce while retrying an older pending cleanup", func(t *testing.T) {
		t.Parallel()

		storageErr := errors.New("source storage unavailable")
		sourceSessions := &recordingSourceSessionManager{resetErr: storageErr}
		manager, _ := startedManagerWithProcess(nil, "ext-stale-pending-cleanup")
		manager.sourceSessions = sourceSessions
		source := extensionResourceSource("ext-stale-pending-cleanup")
		oldSessionNonce := "nonce-ext-stale-pending-cleanup"
		if err := sourceSessions.ActivateSourceSession(
			testutil.Context(t),
			extensionManagerResourceActor(),
			source,
			oldSessionNonce,
		); err != nil {
			t.Fatalf("ActivateSourceSession(older) error = %v", err)
		}

		if err := manager.Stop(testutil.Context(t)); !errors.Is(err, storageErr) {
			t.Fatalf("Stop() error = %v, want source storage failure", err)
		}
		manager.mu.RLock()
		pending := append([]pendingExtensionCleanup(nil), manager.pendingCleanups...)
		manager.mu.RUnlock()
		if len(pending) != 1 {
			t.Fatalf("pending cleanup count = %d, want 1", len(pending))
		}
		if pending[0].sourceSessionNonce != oldSessionNonce {
			t.Fatalf("pending source nonce = %q, want %q", pending[0].sourceSessionNonce, oldSessionNonce)
		}
		if err := sourceSessions.ActivateSourceSession(
			testutil.Context(t),
			extensionManagerResourceActor(),
			source,
			"winner-session",
		); err != nil {
			t.Fatalf("ActivateSourceSession(winner) error = %v", err)
		}

		sourceSessions.setResetError(nil)
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop(retry stale cleanup) error = %v", err)
		}
		manager.mu.RLock()
		pendingCount := len(manager.pendingCleanups)
		manager.mu.RUnlock()
		if pendingCount != 0 {
			t.Fatalf("pending cleanup count after stale retry = %d, want 0", pendingCount)
		}
		if got := sourceSessions.activeNonce(source); got != "winner-session" {
			t.Fatalf("active source nonce after stale retry = %q, want winner-session", got)
		}
	})
}

func TestManagerCrashLoopGenerationFencing(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should preserve a replacement generation when an older crash loop reaches the disable threshold",
		func(t *testing.T) {
			t.Parallel()

			logged := make(chan struct{})
			releaseLog := make(chan struct{})
			checker := &CapabilityChecker{}
			sourceSessions := &recordingSourceSessionManager{}
			manager := NewManager(
				nil,
				WithCapabilityChecker(checker),
				WithLogger(slog.New(&blockingErrorHandler{started: logged, release: releaseLog})),
				WithSourceSessionManager(sourceSessions),
			)
			manager.restartFailureThreshold = 1
			key := InstanceKey{Name: "recovering-dev", WorkspaceID: "fenced-workspace"}
			source := extensionResourceSource(key)
			oldProcess := newFakeProcess(993)
			old := &managedExtension{
				key: key,
				info: ExtensionInfo{
					Name:    key.Name,
					Version: "1.0.0",
					Source:  SourceWorkspace,
					Enabled: true,
				},
				manifest:     &Manifest{Name: key.Name, Version: "1.0.0"},
				process:      oldProcess,
				generation:   1,
				sessionNonce: "older-session",
				active:       true,
				registered:   true,
			}
			manager.mu.Lock()
			manager.started = true
			manager.lifecycleCtx = context.Background()
			manager.devExtensions[key.Normalize()] = old
			manager.mu.Unlock()

			result := make(chan bool, 1)
			go func() {
				_, recovered := manager.recoverOwnedInstance(key, old, oldProcess, errors.New("candidate crashed"))
				result <- recovered
			}()
			<-logged

			newProcess := newFakeProcess(994)
			winner := &managedExtension{
				key: key,
				info: ExtensionInfo{
					Name:    key.Name,
					Version: "2.0.0",
					Source:  SourceWorkspace,
					Enabled: true,
				},
				manifest:          &Manifest{Name: key.Name, Version: "2.0.0"},
				process:           newProcess,
				generation:        2,
				capabilityGrantID: "winner-grant",
				sessionNonce:      "winner-session",
				active:            true,
				registered:        true,
			}
			if _, err := checker.RegisterForSession(
				winner.capabilityGrantID,
				winner.info.Source,
				winner.manifest,
				resources.ResourceScopeKindWorkspace,
			); err != nil {
				t.Fatalf("RegisterForSession(winner) error = %v", err)
			}
			if err := sourceSessions.ActivateSourceSession(
				testutil.Context(t),
				extensionManagerResourceActor(),
				source,
				winner.sessionNonce,
			); err != nil {
				t.Fatalf("ActivateSourceSession(winner) error = %v", err)
			}
			manager.swapDevInstance(key, winner)
			close(releaseLog)

			if recovered := <-result; recovered {
				t.Fatal("recoverOwnedInstance() recovered a threshold-exceeded candidate, want disable")
			}
			if !winner.info.Enabled || !winner.registered || !winner.active || winner.process != newProcess {
				t.Fatalf(
					"winner state after stale crash-loop disable = %#v, want enabled registered active winner",
					winner,
				)
			}
			if got := sourceSessions.activeNonce(source); got != winner.sessionNonce {
				t.Fatalf("active source nonce = %q, want winner nonce %q", got, winner.sessionNonce)
			}
			checker.mu.RLock()
			_, grantActive := checker.grants[winner.capabilityGrantID]
			checker.mu.RUnlock()
			if !grantActive {
				t.Fatalf("winner capability grant %q is not active", winner.capabilityGrantID)
			}
			select {
			case <-newProcess.Done():
				t.Fatal("winner process terminated after an older generation crash loop")
			default:
			}

			newProcess.close(nil)
		},
	)
}

func TestManagerStartupTransactionRollback(t *testing.T) {
	t.Parallel()

	t.Run("Should leave no published candidate state when initialize fails", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		fixture := createManagerTestExtension(t, managerTestManifest("startup-rollback", managerManifestOptions{
			command:      "fake-extension",
			withHooks:    true,
			capabilities: []string{"memory.backend"},
			permissions:  []string{"sessions/list"},
		}), nil)
		installManagerFixture(t, env.registry, fixture, SourceUser, true)

		initializeErr := errors.New("initialize failed")
		process := newFakeProcess(904)
		process.initErr = initializeErr
		sourceSessions := &recordingSourceSessionManager{}
		checker := &CapabilityChecker{}
		manager := NewManager(
			env.registry,
			WithCapabilityChecker(checker),
			WithSourceSessionManager(sourceSessions),
			withProcessLauncher((&fakeLauncher{queue: []*fakeProcess{process}}).launch),
		)

		err := manager.Start(testutil.Context(t))
		if !errors.Is(err, initializeErr) {
			t.Fatalf("Start() error = %v, want initialize failure", err)
		}
		loaded, getErr := manager.Get("startup-rollback")
		if getErr != nil {
			t.Fatalf("Get(startup-rollback) error = %v", getErr)
		}
		if loaded.Status.Registered || loaded.Status.Active {
			t.Fatalf("failed startup status = %#v, want neither registered nor active", loaded.Status)
		}
		if len(loaded.Hooks) != 0 || len(loaded.Agents) != 0 || len(loaded.StaticAgents) != 0 ||
			len(loaded.Skills) != 0 || len(loaded.Loops) != 0 || len(loaded.AutomationJobs) != 0 ||
			len(loaded.AutomationTriggers) != 0 || len(loaded.Layouts) != 0 {
			t.Fatalf("failed startup declarations = %#v, want no publishable declarations", loaded)
		}
		if got := sourceSessions.activations(); len(got) != 0 {
			t.Fatalf("ActivateSourceSession() calls = %#v, want no source activation before validated startup", got)
		}
		checker.mu.RLock()
		grantCount := len(checker.grants)
		checker.mu.RUnlock()
		if grantCount != 0 {
			t.Fatalf("capability grant count = %d, want zero after failed startup", grantCount)
		}
		process.mu.Lock()
		closed := process.closed
		shutdownCount := process.shutdownCnt
		process.mu.Unlock()
		if !closed || shutdownCount != 1 {
			t.Fatalf("failed startup process = closed %v, shutdowns %d, want true/1", closed, shutdownCount)
		}
	})

	t.Run("Should shut down the candidate when boot context is canceled during commit", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		fixture := createManagerTestExtension(t, managerTestManifest("startup-canceled", managerManifestOptions{
			command:      "fake-extension",
			capabilities: []string{"memory.backend"},
			permissions:  []string{"sessions/list"},
		}), nil)
		installManagerFixture(t, env.registry, fixture, SourceUser, true)

		startCtx, cancel := context.WithCancel(testutil.Context(t))
		t.Cleanup(cancel)
		process := newFakeProcess(905)
		process.initHook = cancel
		checker := &CapabilityChecker{}
		manager := NewManager(
			env.registry,
			WithCapabilityChecker(checker),
			WithSourceSessionManager(&recordingSourceSessionManager{}),
			withProcessLauncher((&fakeLauncher{queue: []*fakeProcess{process}}).launch),
		)

		err := manager.Start(startCtx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Start() error = %v, want canceled commit", err)
		}
		process.mu.Lock()
		closed := process.closed
		shutdownCount := process.shutdownCnt
		process.mu.Unlock()
		if !closed || shutdownCount != 1 {
			t.Fatalf("canceled startup process = closed %v, shutdowns %d, want true/1", closed, shutdownCount)
		}
		checker.mu.RLock()
		grantCount := len(checker.grants)
		checker.mu.RUnlock()
		if grantCount != 0 {
			t.Fatalf("capability grant count = %d, want zero after canceled startup", grantCount)
		}
	})
}

func startedManagerWithProcess(registry *Registry, extensionName string) (*Manager, *fakeProcess) {
	proc := newFakeProcess(902)
	manager := NewManager(registry)
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	manager.mu.Lock()
	manager.started = true
	manager.lifecycleCtx = lifecycleCtx
	manager.cancel = cancel
	manager.extensions = map[string]*managedExtension{
		extensionName: {
			info: ExtensionInfo{
				Name:    extensionName,
				Version: "1.0.0",
				Source:  SourceUser,
				Enabled: true,
			},
			manifest: &Manifest{
				Name:    extensionName,
				Version: "1.0.0",
			},
			process:      proc,
			active:       true,
			registered:   true,
			sessionNonce: "nonce-" + extensionName,
		},
	}
	manager.mu.Unlock()
	return manager, proc
}

type recordingSourceSessionManager struct {
	mu                sync.Mutex
	activationSources []resources.ResourceSource
	activeNonces      map[resources.ResourceSource]string
	resets            []resources.ResourceSource
	resetErr          error
}

type faultingSourceSessionManager struct {
	mu       sync.Mutex
	delegate resources.SourceSessionManager
	resetErr error
}

func (m *faultingSourceSessionManager) ActivateSourceSession(
	ctx context.Context,
	actor resources.MutationActor,
	source resources.ResourceSource,
	sessionNonce string,
) error {
	return m.delegate.ActivateSourceSession(ctx, actor, source, sessionNonce)
}

func (m *faultingSourceSessionManager) ResetSourceIfActiveSession(
	ctx context.Context,
	actor resources.MutationActor,
	source resources.ResourceSource,
	sessionNonce string,
) (bool, error) {
	m.mu.Lock()
	resetErr := m.resetErr
	m.mu.Unlock()
	if resetErr != nil {
		return false, resetErr
	}
	return m.delegate.ResetSourceIfActiveSession(ctx, actor, source, sessionNonce)
}

func (m *faultingSourceSessionManager) ResetSource(
	ctx context.Context,
	actor resources.MutationActor,
	source resources.ResourceSource,
) error {
	return m.delegate.ResetSource(ctx, actor, source)
}

func (m *faultingSourceSessionManager) setResetError(err error) {
	m.mu.Lock()
	m.resetErr = err
	m.mu.Unlock()
}

func (m *recordingSourceSessionManager) ActivateSourceSession(
	ctx context.Context,
	_ resources.MutationActor,
	source resources.ResourceSource,
	sessionNonce string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	normalizedSource := source.Normalize()
	if m.activeNonces == nil {
		m.activeNonces = make(map[resources.ResourceSource]string)
	}
	m.activationSources = append(m.activationSources, normalizedSource)
	m.activeNonces[normalizedSource] = sessionNonce
	return nil
}

func (m *recordingSourceSessionManager) ResetSourceIfActiveSession(
	ctx context.Context,
	_ resources.MutationActor,
	source resources.ResourceSource,
	sessionNonce string,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resetErr != nil {
		return false, m.resetErr
	}
	normalizedSource := source.Normalize()
	if m.activeNonces[normalizedSource] != sessionNonce {
		return false, nil
	}
	m.resets = append(m.resets, normalizedSource)
	delete(m.activeNonces, normalizedSource)
	return true, nil
}

func (m *recordingSourceSessionManager) ResetSource(
	_ context.Context,
	_ resources.MutationActor,
	source resources.ResourceSource,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalizedSource := source.Normalize()
	m.resets = append(m.resets, normalizedSource)
	delete(m.activeNonces, normalizedSource)
	return nil
}

func (m *recordingSourceSessionManager) hasReset(source resources.ResourceSource) bool {
	return slices.Contains(m.resetSources(), source.Normalize())
}

func (m *recordingSourceSessionManager) resetSources() []resources.ResourceSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]resources.ResourceSource(nil), m.resets...)
}

func (m *recordingSourceSessionManager) activations() []resources.ResourceSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]resources.ResourceSource(nil), m.activationSources...)
}

func (m *recordingSourceSessionManager) activeNonce(source resources.ResourceSource) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeNonces[source.Normalize()]
}

func (m *recordingSourceSessionManager) setResetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetErr = err
}

func seedManagedSourceRecord(
	t *testing.T,
	kernel *resources.Kernel,
	key InstanceKey,
	sessionNonce string,
	recordID string,
) {
	t.Helper()
	ctx := testutil.Context(t)
	source := extensionResourceSource(key)
	if err := kernel.ActivateSourceSession(
		ctx,
		extensionManagerResourceActor(),
		source,
		sessionNonce,
	); err != nil {
		t.Fatalf("ActivateSourceSession() error = %v", err)
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	if !key.IsGlobal() {
		scope = resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: key.WorkspaceID}
	}
	resourceKind := resources.ResourceKind("tool")
	actor := resources.MutationActor{
		Kind:          resources.MutationActorKindExtension,
		ID:            key.runtimeID(),
		SessionNonce:  sessionNonce,
		Source:        source,
		MaxScope:      scope,
		GrantedKinds:  []resources.ResourceKind{resourceKind},
		GrantedScopes: []resources.ResourceScopeKind{scope.Kind},
	}
	if err := kernel.ApplySourceSnapshotRaw(ctx, actor, resources.SourceSnapshot{
		SourceVersion: 1,
		Records: []resources.RawDraft{{
			Kind:            resourceKind,
			ID:              recordID,
			Scope:           scope,
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"cleanup-record"}`),
		}},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw() error = %v", err)
	}
}

func assertManagedSourceEmpty(
	t *testing.T,
	kernel *resources.Kernel,
	db *sql.DB,
	key InstanceKey,
) {
	t.Helper()
	assertManagedSourceState(t, kernel, db, key, 0, "")
}

func assertManagedSourceState(
	t *testing.T,
	kernel *resources.Kernel,
	db *sql.DB,
	key InstanceKey,
	wantRecordCount int,
	wantSessionNonce string,
) {
	t.Helper()
	ctx := testutil.Context(t)
	source := extensionResourceSource(key)
	records, err := kernel.ListRaw(ctx, extensionManagerResourceActor(), resources.ResourceFilter{Source: &source})
	if err != nil {
		t.Fatalf("ListRaw(cleaned source) error = %v", err)
	}
	if len(records) != wantRecordCount {
		t.Fatalf("managed source record count = %d, want %d; records = %#v", len(records), wantRecordCount, records)
	}
	var sessionNonce string
	if err := db.QueryRowContext(
		ctx,
		`SELECT session_nonce FROM resource_source_state WHERE source_kind = ? AND source_id = ?`,
		source.Kind,
		source.ID,
	).Scan(&sessionNonce); err != nil {
		if wantSessionNonce == "" && errors.Is(err, sql.ErrNoRows) {
			return
		}
		t.Fatalf("QueryRowContext(resource_source_state) error = %v", err)
	}
	if sessionNonce != wantSessionNonce {
		t.Fatalf("resource_source_state session nonce = %q, want %q", sessionNonce, wantSessionNonce)
	}
}

func (p *fakeProcess) shutdownCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shutdownCnt
}

type blockingErrorHandler struct {
	started chan<- struct{}
	release <-chan struct{}
	once    sync.Once
}

func (h *blockingErrorHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *blockingErrorHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Level < slog.LevelError {
		return nil
	}
	h.once.Do(func() {
		close(h.started)
		<-h.release
	})
	return nil
}

func (h *blockingErrorHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *blockingErrorHandler) WithGroup(string) slog.Handler {
	return h
}
