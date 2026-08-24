package daemon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	registrypkg "github.com/compozy/compozy/internal/registry"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestDaemonExtensionServiceConsumerSync(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a canceled sync before entering resource consumers", func(t *testing.T) {
		t.Parallel()

		entered := make(chan struct{})
		publisher := agentSkillPublisherFunc(func(context.Context) error {
			close(entered)
			return nil
		})
		service := &daemonExtensionService{agentSkill: publisher}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := service.syncExtensionConsumers(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("sync error = %v, want context cancellation", err)
		}
		select {
		case <-entered:
			t.Fatal("canceled sync entered a resource consumer")
		default:
		}
	})

	t.Run("Should cancel a queued sync without entering resource consumers", func(t *testing.T) {
		t.Parallel()

		firstEntered := make(chan struct{})
		firstRelease := make(chan struct{})
		secondEntered := make(chan struct{})
		publisher := &blockingExtensionConsumerPublisher{
			firstEntered:  firstEntered,
			firstRelease:  firstRelease,
			secondEntered: secondEntered,
		}
		registry := extensionpkg.NewRegistry(openDaemonTestGlobalDB(t).DB())
		service := newDaemonExtensionService(&daemonExtensionServiceDeps{
			Registry: registry,
			Loops:    publisher,
		}).(*daemonExtensionService)

		firstResult := make(chan error, 1)
		go func() {
			firstResult <- service.syncExtensionConsumers(t.Context())
		}()
		requireLifecycleSignal(t, firstEntered, "first resource consumer sync")

		secondCtx, cancelSecond := context.WithCancel(t.Context())
		secondStarted := make(chan struct{})
		secondResult := make(chan error, 1)
		go func() {
			close(secondStarted)
			secondResult <- service.syncExtensionConsumers(secondCtx)
		}()
		requireLifecycleSignal(t, secondStarted, "queued resource consumer sync")
		cancelSecond()

		queuedErr := requireLifecycleResult(t, secondResult, "queued resource consumer sync")
		if !errors.Is(queuedErr, context.Canceled) {
			t.Fatalf("queued sync error = %v, want context cancellation", queuedErr)
		}
		select {
		case <-secondEntered:
			t.Fatal("queued sync entered the resource consumer while another sync was active")
		default:
		}

		close(firstRelease)
		if err := requireLifecycleResult(t, firstResult, "first resource consumer sync"); err != nil {
			t.Fatalf("first sync error = %v", err)
		}
	})
}

type blockingExtensionConsumerPublisher struct {
	mu            sync.Mutex
	calls         int
	firstEntered  chan<- struct{}
	firstRelease  <-chan struct{}
	secondEntered chan<- struct{}
}

func (p *blockingExtensionConsumerPublisher) Sync(ctx context.Context) error {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()
	if call == 1 {
		close(p.firstEntered)
		select {
		case <-p.firstRelease:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	close(p.secondEntered)
	return ctx.Err()
}

func TestExtensionLifecycleCoordinator(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a committed install successful when staging cleanup fails", func(t *testing.T) {
		t.Parallel()

		cleanupErr := errors.New("staging cleanup failed")
		service := &daemonExtensionService{logger: discardLogger()}
		prepared := preparedDaemonExtensionInstall{
			name: "portable",
			cleanup: func() error {
				return cleanupErr
			},
		}
		if err := service.finishPreparedInstall(prepared, nil); err != nil {
			t.Fatalf("finishPreparedInstall(committed) error = %v, want nil", err)
		}
		mutationErr := errors.New("commit failed")
		if err := service.finishPreparedInstall(prepared, mutationErr); !errors.Is(err, mutationErr) ||
			!errors.Is(err, cleanupErr) {
			t.Fatalf("finishPreparedInstall(failed) error = %v, want mutation and cleanup failures", err)
		}
	})

	t.Run("Should serialize same-name mutations while allowing other names to proceed", func(t *testing.T) {
		t.Parallel()

		coordinator := newExtensionLifecycleCoordinator()
		firstEntered := make(chan struct{})
		releaseFirst := make(chan struct{})
		releaseFirstMutation := newLifecycleRelease(t, releaseFirst)
		secondEntered := make(chan struct{})
		otherEntered := make(chan struct{})
		errorsCh := make(chan error, 3)

		go func() {
			errorsCh <- coordinator.withName(context.Background(), "alpha", func() error {
				close(firstEntered)
				<-releaseFirst
				return nil
			})
		}()
		requireLifecycleSignal(t, firstEntered, "first alpha mutation")

		go func() {
			errorsCh <- coordinator.withName(context.Background(), "alpha", func() error {
				close(secondEntered)
				return nil
			})
		}()
		go func() {
			errorsCh <- coordinator.withName(context.Background(), "beta", func() error {
				close(otherEntered)
				return nil
			})
		}()

		requireLifecycleSignal(t, otherEntered, "independent beta mutation")
		select {
		case <-secondEntered:
			t.Fatal("second alpha mutation entered before first released")
		default:
		}
		releaseFirstMutation()
		requireLifecycleSignal(t, secondEntered, "second alpha mutation")
		for range 3 {
			if err := <-errorsCh; err != nil {
				t.Fatalf("withName() error = %v", err)
			}
		}
	})

	t.Run("Should isolate a global instance from a same-name workspace instance", func(t *testing.T) {
		t.Parallel()

		coordinator := newExtensionLifecycleCoordinator()
		globalEntered := make(chan struct{})
		releaseGlobal := make(chan struct{})
		releaseGlobalMutation := newLifecycleRelease(t, releaseGlobal)
		workspaceEntered := make(chan struct{})
		results := make(chan error, 2)
		key := extensionpkg.InstanceKey{Name: "shared", WorkspaceID: "workspace-a"}

		go func() {
			results <- coordinator.withName(t.Context(), "shared", func() error {
				close(globalEntered)
				<-releaseGlobal
				return nil
			})
		}()
		requireLifecycleSignal(t, globalEntered, "global instance mutation")

		go func() {
			results <- coordinator.withInstance(t.Context(), key, func() error {
				close(workspaceEntered)
				return nil
			})
		}()
		requireLifecycleSignal(t, workspaceEntered, "same-name workspace mutation")
		releaseGlobalMutation()
		for range 2 {
			if err := <-results; err != nil {
				t.Fatalf("instance lifecycle mutation error = %v", err)
			}
		}
	})

	t.Run("Should cancel a waiter without retaining entries or running its mutation", func(t *testing.T) {
		t.Parallel()

		coordinator := newExtensionLifecycleCoordinator()
		firstEntered := make(chan struct{})
		releaseFirst := make(chan struct{})
		releaseFirstMutation := newLifecycleRelease(t, releaseFirst)
		firstDone := make(chan error, 1)
		go func() {
			firstDone <- coordinator.withName(context.Background(), "alpha", func() error {
				close(firstEntered)
				<-releaseFirst
				return nil
			})
		}()
		requireLifecycleSignal(t, firstEntered, "first alpha mutation")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ran := false
		err := coordinator.withName(ctx, "alpha", func() error {
			ran = true
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("withName(canceled) error = %v, want %v", err, context.Canceled)
		}
		if ran {
			t.Fatal("withName(canceled) ran mutation")
		}
		coordinator.mu.Lock()
		retained := len(coordinator.entries)
		coordinator.mu.Unlock()
		if retained != 1 {
			t.Fatalf("retained lifecycle entries = %d, want only the active mutation", retained)
		}

		releaseFirstMutation()
		if err := <-firstDone; err != nil {
			t.Fatalf("withName(first) error = %v", err)
		}
		coordinator.mu.Lock()
		retained = len(coordinator.entries)
		coordinator.mu.Unlock()
		if retained != 0 {
			t.Fatalf("retained lifecycle entries after release = %d, want 0", retained)
		}
	})

	t.Run("Should acquire opposing name sets in stable order without deadlock", func(t *testing.T) {
		t.Parallel()

		coordinator := newExtensionLifecycleCoordinator()
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		releaseAlpha := make(chan struct{})
		releaseAlphaHolder := newLifecycleRelease(t, releaseAlpha)
		alphaHeld := make(chan struct{})
		releaseFirst := make(chan struct{})
		releaseFirstCaller := newLifecycleRelease(t, releaseFirst)
		releaseSecond := make(chan struct{})
		releaseSecondCaller := newLifecycleRelease(t, releaseSecond)
		firstEntered := make(chan struct{})
		secondEntered := make(chan struct{})
		results := make(chan error, 3)
		var mu sync.Mutex
		calls := map[string]int{}

		go func() {
			results <- coordinator.withName(ctx, "alpha", func() error {
				close(alphaHeld)
				<-releaseAlpha
				return nil
			})
		}()
		requireLifecycleSignal(t, alphaHeld, "alpha holder")

		go func() {
			results <- coordinator.withNames(ctx, []string{"alpha", "beta"}, func() error {
				mu.Lock()
				calls["first"]++
				mu.Unlock()
				close(firstEntered)
				<-releaseFirst
				return nil
			})
		}()
		waitForLifecycleRefs(t, coordinator, "alpha", 2)

		go func() {
			results <- coordinator.withNames(ctx, []string{"beta", "alpha", "beta", " "}, func() error {
				mu.Lock()
				calls["second"]++
				mu.Unlock()
				close(secondEntered)
				<-releaseSecond
				return nil
			})
		}()
		waitForLifecycleRefs(t, coordinator, "alpha", 3)

		releaseAlphaHolder()
		select {
		case <-firstEntered:
			select {
			case <-secondEntered:
				t.Fatal("second opposing caller entered while first held both names")
			default:
			}
			releaseFirstCaller()
			requireLifecycleSignal(t, secondEntered, "second opposing caller after first released")
		case <-secondEntered:
			select {
			case <-firstEntered:
				t.Fatal("first opposing caller entered while second held both names")
			default:
			}
			releaseSecondCaller()
			requireLifecycleSignal(t, firstEntered, "first opposing caller after second released")
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for an opposing name-set caller")
		}
		releaseFirstCaller()
		releaseSecondCaller()
		for range 3 {
			if err := requireLifecycleResult(t, results, "opposing name-set caller"); err != nil {
				t.Fatalf("withNames() error = %v", err)
			}
		}
		mu.Lock()
		gotCalls := map[string]int{"first": calls["first"], "second": calls["second"]}
		mu.Unlock()
		if !reflect.DeepEqual(gotCalls, map[string]int{"first": 1, "second": 1}) {
			t.Fatalf("withNames() callback calls = %#v, want each callback once", gotCalls)
		}
		coordinator.mu.Lock()
		retained := len(coordinator.entries)
		coordinator.mu.Unlock()
		if retained != 0 {
			t.Fatalf("retained lifecycle entries = %d, want 0", retained)
		}
	})

	t.Run("Should serialize enable update and disable as whole service operations", func(t *testing.T) {
		// This assertion intentionally owns one mutable extension lifecycle.
		deps, registry, source, _ := newNativeExtensionToolDeps(t)
		runtime := newLifecycleStateRuntime(registry)
		service := newDaemonExtensionService(
			&daemonExtensionServiceDeps{
				Registry:  registry,
				Runtime:   runtime,
				HomePaths: deps.HomePaths,
				Logger:    discardLogger(),
				Now:       time.Now,
			},
			withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
			withDaemonExtensionAutomation(&fakeAutomationManager{}),
			withDaemonExtensionMCPRuntimeHealth(mcppkg.NewRuntimeHealthRegistry()),
		).(*daemonExtensionService)
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "lifecycle serialization")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		source.latestVersion = "1.0.0"
		if _, err := service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceGitHub, Ref: "acme/tool-ext", AllowUnverified: true,
		}, actor); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		healthObservation := service.mcpRuntimeHealth.Begin(mcppkg.RuntimeHealthKey{
			InstanceName: "tool-ext", BundleGeneration: "generation-before-disable", ServerName: "remote",
		})
		service.mcpRuntimeHealth.RecordFailure(healthObservation, errors.New("connection failed"))
		runtime.resetReloads()
		source.latestVersion = "2.0.0"
		entered, release := runtime.blockNextReload()
		releaseReload := newLifecycleRelease(t, release)

		type operationResult struct {
			name string
			err  error
		}
		results := make(chan operationResult, 3)
		go func() {
			_, enableErr := service.Enable(t.Context(), "tool-ext", contract.EnableExtensionRequest{}, actor)
			results <- operationResult{name: "enable", err: enableErr}
		}()
		requireLifecycleSignal(t, entered, "enable reload")
		go func() {
			_, updateErr := service.Update(t.Context(), "tool-ext", contract.UpdateExtensionRequest{
				CheckOnly: true, AllowUnverified: true,
			}, actor)
			results <- operationResult{name: "update", err: updateErr}
		}()
		go func() {
			_, disableErr := service.Disable(t.Context(), "tool-ext", actor)
			results <- operationResult{name: "disable", err: disableErr}
		}()
		waitForLifecycleRefs(t, service.lifecycle, "tool-ext", 3)
		blockedInfo, err := registry.Get("tool-ext")
		if err != nil {
			t.Fatalf("registry.Get(while enable reload blocked) error = %v", err)
		}
		if !blockedInfo.Enabled || runtime.reloadCount() != 1 {
			t.Fatalf(
				"blocked lifecycle state = enabled:%t reloads:%d, want only enable mutation in progress",
				blockedInfo.Enabled,
				runtime.reloadCount(),
			)
		}
		releaseReload()

		seen := make(map[string]error, 3)
		for range 3 {
			result := <-results
			seen[result.name] = result.err
		}
		for _, name := range []string{"enable", "update", "disable"} {
			if err := seen[name]; err != nil {
				t.Fatalf("%s operation error = %v", name, err)
			}
		}
		finalInfo, err := registry.Get("tool-ext")
		if err != nil {
			t.Fatalf("registry.Get(final) error = %v", err)
		}
		if finalInfo.Enabled || runtime.reloadCount() != 2 {
			t.Fatalf(
				"final lifecycle state = enabled:%t reloads:%d, want disabled after two ordered reloads",
				finalInfo.Enabled,
				runtime.reloadCount(),
			)
		}
		if entries := service.mcpRuntimeHealth.Entries(
			"tool-ext", "", "generation-before-disable",
		); len(entries) != 0 {
			t.Fatalf("runtime health after disable = %#v, want evicted", entries)
		}
	})

	t.Run("Should evict MCP health after a committed update and removal", func(t *testing.T) {
		t.Parallel()

		// This assertion intentionally owns one mutable extension lifecycle.
		deps, registry, source, _ := newNativeExtensionToolDeps(t)
		runtime := newLifecycleStateRuntime(registry)
		health := mcppkg.NewRuntimeHealthRegistry()
		service := newDaemonExtensionService(
			&daemonExtensionServiceDeps{
				Registry: registry, Runtime: runtime, HomePaths: deps.HomePaths,
				Logger: discardLogger(), Now: time.Now,
			},
			withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
			withDaemonExtensionAutomation(&fakeAutomationManager{}),
			withDaemonExtensionMCPRuntimeHealth(health),
		).(*daemonExtensionService)
		actor, err := taskpkg.DeriveHumanActorContext(
			"operator", taskpkg.OriginKindCLI, "lifecycle MCP health eviction",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		source.latestVersion = "1.0.0"
		if _, err := service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceGitHub, Ref: "acme/tool-ext", AllowUnverified: true,
		}, actor); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		recordRuntimeHealthFailure(health, "tool-ext", "before-update")
		source.latestVersion = "2.0.0"
		if _, err := service.Update(
			t.Context(), "tool-ext", contract.UpdateExtensionRequest{AllowUnverified: true}, actor,
		); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if entries := health.Entries("tool-ext", "", "before-update"); len(entries) != 0 {
			t.Fatalf("runtime health after update = %#v, want evicted", entries)
		}
		recordRuntimeHealthFailure(health, "tool-ext", "before-remove")
		if _, err := service.Remove(t.Context(), "tool-ext", actor); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		if entries := health.Entries("tool-ext", "", "before-remove"); len(entries) != 0 {
			t.Fatalf("runtime health after remove = %#v, want evicted", entries)
		}
	})

	t.Run("Should restore files version confirmation and runtime after a confirmed update fails", func(t *testing.T) {
		// This assertion intentionally owns one mutable marketplace extension lifecycle.
		deps, registry, source, _ := newNativeExtensionToolDeps(t)
		runtime := newLifecycleStateRuntime(registry)
		publisher := &lifecycleFailingPublisher{}
		service := newDaemonExtensionService(
			&daemonExtensionServiceDeps{
				Registry:   registry,
				Runtime:    runtime,
				AgentSkill: publisher,
				HomePaths:  deps.HomePaths,
				Logger:     discardLogger(),
				Now:        time.Now,
			},
			withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
			withDaemonExtensionAutomation(&fakeAutomationManager{}),
		).(*daemonExtensionService)
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "lifecycle update rollback")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}

		source.downloads["1.0.0"] = lifecycleNetworkExtensionDownloadResult(t, "1.0.0", "builders")
		source.downloads["2.0.0"] = lifecycleNetworkExtensionDownloadResult(t, "2.0.0", "reviewers")
		source.latestVersion = "1.0.0"
		if _, err := service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceGitHub, Ref: "acme/tool-ext", AllowUnverified: true,
		}, actor); err != nil {
			t.Fatalf("Install(v1) error = %v", err)
		}
		firstDigest := lifecycleNetworkDigest(t, "builders")
		if _, err := service.Enable(
			t.Context(),
			"tool-ext",
			contract.EnableExtensionRequest{ConfirmNetworkDigest: firstDigest},
			actor,
		); err != nil {
			t.Fatalf("Enable(v1 confirmation) error = %v", err)
		}

		beforeInfo, err := registry.Get("tool-ext")
		if err != nil {
			t.Fatalf("registry.Get(v1) error = %v", err)
		}
		beforeConfirmation, err := registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey("tool-ext"))
		if err != nil {
			t.Fatalf("NetworkConfirmation(v1) error = %v", err)
		}
		beforeRunning, ok := runtime.current()
		if !ok {
			t.Fatal("runtime current(v1) missing")
		}
		installDir := filepath.Dir(beforeInfo.ManifestPath)
		beforeManifest, err := os.ReadFile(filepath.Join(installDir, "extension.toml"))
		if err != nil {
			t.Fatalf("os.ReadFile(extension.toml v1) error = %v", err)
		}
		beforeVersionFile, err := os.ReadFile(filepath.Join(installDir, "VERSION.txt"))
		if err != nil {
			t.Fatalf("os.ReadFile(VERSION.txt v1) error = %v", err)
		}

		runtime.resetReloads()
		source.latestVersion = "2.0.0"
		secondDigest := lifecycleNetworkDigest(t, "reviewers")
		publisher.failNextSyncs(1)
		_, err = service.Update(t.Context(), "tool-ext", contract.UpdateExtensionRequest{
			AllowUnverified: true, ConfirmNetworkDigest: secondDigest,
		}, actor)
		if err == nil || !strings.Contains(err.Error(), "injected extension resource reconcile failure") {
			t.Fatalf("Update(v2 reconcile failure) error = %v, want injected reconcile failure", err)
		}

		afterInfo, err := registry.Get("tool-ext")
		if err != nil {
			t.Fatalf("registry.Get(after update rollback) error = %v", err)
		}
		if afterInfo.Version != beforeInfo.Version || derefNativeExtensionString(afterInfo.RemoteVersion) != "1.0.0" {
			t.Fatalf("registry version after rollback = %#v, want v1", afterInfo)
		}
		afterConfirmation, err := registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey("tool-ext"))
		if err != nil {
			t.Fatalf("NetworkConfirmation(after update rollback) error = %v", err)
		}
		if !reflect.DeepEqual(afterConfirmation, beforeConfirmation) {
			t.Fatalf("network confirmation after rollback = %#v, want %#v", afterConfirmation, beforeConfirmation)
		}
		afterManifest, err := os.ReadFile(filepath.Join(installDir, "extension.toml"))
		if err != nil {
			t.Fatalf("os.ReadFile(extension.toml after rollback) error = %v", err)
		}
		afterVersionFile, err := os.ReadFile(filepath.Join(installDir, "VERSION.txt"))
		if err != nil {
			t.Fatalf("os.ReadFile(VERSION.txt after rollback) error = %v", err)
		}
		if !reflect.DeepEqual(afterManifest, beforeManifest) ||
			!reflect.DeepEqual(afterVersionFile, beforeVersionFile) {
			t.Fatalf(
				"managed files after rollback = manifest:%q version:%q, want original v1 files",
				afterManifest,
				afterVersionFile,
			)
		}
		afterRunning, ok := runtime.current()
		if !ok {
			t.Fatalf("runtime after update rollback = %#v/%t, want %#v", afterRunning, ok, beforeRunning)
		}
		assertExtensionPublicState(t, "runtime after update rollback", afterRunning, beforeRunning)
		reloads := runtime.reloadSnapshot()
		if len(reloads) != 2 || reloads[0].Version != "2.0.0" ||
			reloads[0].NetworkRequirementDigest != secondDigest ||
			reloads[1].Version != "1.0.0" || reloads[1].NetworkRequirementDigest != firstDigest {
			t.Fatalf("runtime update/rollback sequence = %#v, want confirmed v2 then restored v1", reloads)
		}
	})

	t.Run("Should restore persisted and running state after every enable stage failure", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			configure func(*testing.T, *lifecycleFailureHarness)
			wantError string
		}{
			{
				name: "confirmation write",
				configure: func(t *testing.T, harness *lifecycleFailureHarness) {
					harness.installFailureTrigger(t, "network_confirmed_by", "NEW.network_confirmed_by IS NOT NULL")
				},
				wantError: "injected lifecycle failure",
			},
			{
				name: "registry mutation",
				configure: func(t *testing.T, harness *lifecycleFailureHarness) {
					harness.installFailureTrigger(t, "enabled", "NEW.enabled = 1")
				},
				wantError: "injected lifecycle failure",
			},
			{
				name: "runtime reload",
				configure: func(_ *testing.T, harness *lifecycleFailureHarness) {
					harness.runtime.failNextReloads(1)
				},
				wantError: "injected extension runtime reload failure",
			},
			{
				name: "resource reconcile",
				configure: func(_ *testing.T, harness *lifecycleFailureHarness) {
					harness.publisher.failNextSyncs(1)
				},
				wantError: "injected extension resource reconcile failure",
			},
		} {
			t.Run("Should roll back after "+testCase.name+" fails", func(t *testing.T) {
				harness := newLifecycleFailureHarness(t, "network-failure-"+lifecycleTestSlug(testCase.name))
				testCase.configure(t, harness)
				_, err := harness.service.Enable(
					t.Context(),
					harness.name,
					contract.EnableExtensionRequest{ConfirmNetworkDigest: harness.digest},
					harness.actor,
				)
				if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
					t.Fatalf("Enable(%s failure) error = %v, want %q", testCase.name, err, testCase.wantError)
				}
				harness.assertRestored(t)
			})
		}
	})

	t.Run("Should reload restored runtime even when confirmation restoration fails", func(t *testing.T) {
		harness := newLifecycleFailureHarness(t, "network-failure-rollback-confirmation")
		harness.publisher.failNextSyncs(1)
		harness.installFailureTrigger(
			t,
			"network_confirmed_by",
			"OLD.network_confirmed_by IS NOT NULL AND NEW.network_confirmed_by IS NULL",
		)
		_, err := harness.service.Enable(
			t.Context(),
			harness.name,
			contract.EnableExtensionRequest{ConfirmNetworkDigest: harness.digest},
			harness.actor,
		)
		if err == nil || !strings.Contains(err.Error(), "injected lifecycle failure") {
			t.Fatalf("Enable(confirmation rollback failure) error = %v, want injected lifecycle failure", err)
		}
		if got := harness.runtime.reloadCount(); got != 2 {
			t.Fatalf("runtime reload calls = %d, want initial and rollback reload", got)
		}
		running, ok := harness.runtime.current()
		if !ok || running.Enabled {
			t.Fatalf("runtime after partial rollback = %#v/%t, want disabled registry state", running, ok)
		}
	})

	t.Run("Should continue development rollback after intermediate compensation failures", func(t *testing.T) {
		db := openDaemonTestGlobalDB(t)
		registry := extensionpkg.NewRegistry(db.DB())
		key := extensionpkg.InstanceKey{Name: "dev-rollback", WorkspaceID: "workspace-1"}
		candidateConfirmation := extensionpkg.NetworkConfirmation{
			Digest: strings.Repeat("c", 64), ConfirmedBy: "candidate", ConfirmedAt: time.Now().UTC(),
		}
		if _, err := registry.LinkDev(extensionpkg.DevLinkRequest{
			Name: key.Name, WorkspaceID: key.WorkspaceID, OriginPath: "/candidate",
			GenerationHash: strings.Repeat("a", 64),
		}); err != nil {
			t.Fatalf("LinkDev(candidate) error = %v", err)
		}
		if err := registry.RestoreNetworkConfirmation(key, candidateConfirmation); err != nil {
			t.Fatalf("RestoreNetworkConfirmation(candidate) error = %v", err)
		}
		originalConfirmation := extensionpkg.NetworkConfirmation{
			Digest: strings.Repeat("d", 64), ConfirmedBy: "operator", ConfirmedAt: time.Now().Add(-time.Hour).UTC(),
		}
		snapshot := &extensionpkg.DevLink{
			ExtensionName: key.Name, WorkspaceID: key.WorkspaceID, OriginPath: "/original",
			BundleGeneration: strings.Repeat("b", 64), NetworkRequirementDigest: originalConfirmation.Digest,
			NetworkConfirmedBy: originalConfirmation.ConfirmedBy, NetworkConfirmedAt: originalConfirmation.ConfirmedAt,
		}
		stageErr := errors.New("injected development stage rollback failure")
		activateErr := errors.New("injected development activation rollback failure")
		syncErr := errors.New("injected development consumer rollback failure")
		cause := errors.New("injected development lifecycle failure")
		runtime := &rollbackDevRuntime{stageErr: stageErr, activateErr: activateErr}
		syncCalls := 0
		service := &daemonExtensionService{
			registry: registry,
			agentSkill: agentSkillPublisherFunc(func(context.Context) error {
				syncCalls++
				return syncErr
			}),
		}

		err := service.rollbackDevLifecycle(t.Context(), runtime, key, snapshot, cause)
		for label, want := range map[string]error{
			"cause": cause, "stage": stageErr, "activation": activateErr, "consumer sync": syncErr,
		} {
			if !errors.Is(err, want) {
				t.Fatalf("rollbackDevLifecycle() error = %v, want joined %s error %v", err, label, want)
			}
		}
		if runtime.stageCalls != 1 || runtime.activateCalls != 1 || syncCalls != 1 {
			t.Fatalf(
				"rollback calls = stage:%d activate:%d sync:%d, want every compensation once",
				runtime.stageCalls,
				runtime.activateCalls,
				syncCalls,
			)
		}
		link, err := registry.GetDevLink(key.Name, key.WorkspaceID)
		if err != nil {
			t.Fatalf("GetDevLink(after rollback) error = %v", err)
		}
		confirmation := extensionpkg.NetworkConfirmation{
			Digest: link.NetworkRequirementDigest, ConfirmedBy: link.NetworkConfirmedBy,
			ConfirmedAt: link.NetworkConfirmedAt,
		}
		if !reflect.DeepEqual(confirmation, originalConfirmation) {
			t.Fatalf("development confirmation after rollback = %#v, want %#v", confirmation, originalConfirmation)
		}
	})

	t.Run("Should roll back an install when its completion event cannot be recorded", func(t *testing.T) {
		deps, registry, source, _ := newNativeExtensionToolDeps(t)
		source.latestVersion = "1.0.0"
		writeErr := errors.New("injected install completion event failure")
		writer := &daemonExtensionEventStoreStub{writeErr: writeErr}
		service := newDaemonExtensionService(
			&daemonExtensionServiceDeps{
				Registry: registry, HomePaths: deps.HomePaths, Logger: discardLogger(), Now: time.Now,
			},
			withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
			withDaemonExtensionEventWriter(writer),
		).(*daemonExtensionService)
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "install event rollback")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}

		_, err = service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceGitHub, Ref: "acme/tool-ext", AllowUnverified: true,
		}, actor)
		if !errors.Is(err, writeErr) {
			t.Fatalf("Install(event failure) error = %v, want %v", err, writeErr)
		}
		if _, getErr := registry.Get("tool-ext"); !errors.Is(getErr, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("registry.Get(after event failure) error = %v, want no installed row", getErr)
		}
		if _, statErr := os.Stat(
			extensionpkg.ManagedInstallPath(deps.HomePaths, "tool-ext"),
		); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(after event failure) error = %v, want no managed files", statErr)
		}
		if writer.writeCalls != 2 {
			t.Fatalf("event writer calls = %d, want completed then failed event attempts", writer.writeCalls)
		}
	})
}

func requireLifecycleSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func requireLifecycleResult(t *testing.T, result <-chan error, label string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s result", label)
		return nil
	}
}

func newLifecycleRelease(t *testing.T, signal chan struct{}) func() {
	t.Helper()
	release := sync.OnceFunc(func() { close(signal) })
	t.Cleanup(release)
	return release
}

type rollbackDevRuntime struct {
	stageErr      error
	activateErr   error
	stageCalls    int
	activateCalls int
}

var _ extensionDevRuntime = (*rollbackDevRuntime)(nil)

func (*rollbackDevRuntime) GetForInstance(extensionpkg.InstanceKey) (*extensionpkg.Extension, error) {
	return nil, extensionpkg.ErrExtensionNotFound
}

func (*rollbackDevRuntime) ListForWorkspace(string) []extensionpkg.ExtensionInfo { return nil }

func (*rollbackDevRuntime) InspectDevelopmentGeneration(
	context.Context,
	string,
	string,
	string,
) (extensionpkg.DevelopmentGeneration, error) {
	return extensionpkg.DevelopmentGeneration{}, nil
}

func (*rollbackDevRuntime) LinkDevelopmentFromOrigin(
	context.Context,
	string,
	string,
	string,
) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (r *rollbackDevRuntime) StageDevelopmentLink(
	context.Context,
	extensionpkg.InstanceKey,
	string,
	string,
) (*extensionpkg.DevLink, error) {
	r.stageCalls++
	return nil, r.stageErr
}

func (r *rollbackDevRuntime) ActivateDevelopmentLink(
	context.Context,
	extensionpkg.InstanceKey,
) (*extensionpkg.Extension, error) {
	r.activateCalls++
	return nil, r.activateErr
}

func (*rollbackDevRuntime) ReloadExtension(
	context.Context,
	extensionpkg.InstanceKey,
	string,
) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (*rollbackDevRuntime) UnlinkDevelopment(context.Context, extensionpkg.InstanceKey) error {
	return nil
}

func (*rollbackDevRuntime) Logs(
	extensionpkg.InstanceKey,
	extensionpkg.ExtensionLogCursor,
) (extensionpkg.ExtensionLogSnapshot, error) {
	return extensionpkg.ExtensionLogSnapshot{}, nil
}

type lifecycleStateRuntime struct {
	mu            sync.Mutex
	registry      *extensionpkg.Registry
	reloads       []extensionpkg.ExtensionInfo
	reloadCalls   int
	failReloads   int
	blockedReload chan struct{}
	reloadEntered chan struct{}
}

func newLifecycleStateRuntime(registry *extensionpkg.Registry) *lifecycleStateRuntime {
	return &lifecycleStateRuntime{registry: registry}
}

func (r *lifecycleStateRuntime) Start(context.Context) error { return nil }
func (r *lifecycleStateRuntime) Stop(context.Context) error  { return nil }

func (r *lifecycleStateRuntime) Reload(ctx context.Context) error {
	r.mu.Lock()
	r.reloadCalls++
	blocked := r.blockedReload
	entered := r.reloadEntered
	if entered != nil {
		close(entered)
		r.reloadEntered = nil
	}
	if r.failReloads > 0 {
		r.failReloads--
		r.mu.Unlock()
		return errors.New("injected extension runtime reload failure")
	}
	r.mu.Unlock()
	if blocked != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-blocked:
		}
	}
	infos, err := r.registry.List()
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.reloads = append(r.reloads, infos...)
	r.blockedReload = nil
	r.mu.Unlock()
	return nil
}

func (r *lifecycleStateRuntime) Get(name string) (*extensionpkg.Extension, error) {
	info, err := r.registry.Get(name)
	if err != nil {
		return nil, err
	}
	manifest, err := extensionpkg.LoadManifest(filepath.Dir(info.ManifestPath))
	if err != nil {
		return nil, err
	}
	return &extensionpkg.Extension{
		Info: *info, Manifest: manifest,
		Status: extensionpkg.ExtensionStatus{
			Name: info.Name, Version: info.Version, Source: info.Source,
			Enabled: info.Enabled, Registered: info.Enabled, Active: info.Enabled,
		},
	}, nil
}

func (r *lifecycleStateRuntime) InspectPackageResources(
	_ context.Context,
	name string,
) (*extensionpkg.Extension, error) {
	return r.Get(name)
}

func (r *lifecycleStateRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}

func (r *lifecycleStateRuntime) blockNextReload() (<-chan struct{}, chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entered := make(chan struct{})
	release := make(chan struct{})
	r.reloadEntered = entered
	r.blockedReload = release
	return entered, release
}

func (r *lifecycleStateRuntime) failNextReloads(count int) {
	r.mu.Lock()
	r.failReloads = count
	r.mu.Unlock()
}

func (r *lifecycleStateRuntime) resetReloads() {
	r.mu.Lock()
	r.reloads = nil
	r.reloadCalls = 0
	r.mu.Unlock()
}

func (r *lifecycleStateRuntime) reloadCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reloadCalls
}

func (r *lifecycleStateRuntime) current() (extensionpkg.ExtensionInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reloads) == 0 {
		return extensionpkg.ExtensionInfo{}, false
	}
	return r.reloads[len(r.reloads)-1], true
}

func (r *lifecycleStateRuntime) reloadSnapshot() []extensionpkg.ExtensionInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]extensionpkg.ExtensionInfo(nil), r.reloads...)
}

func assertExtensionPublicState(
	t *testing.T,
	label string,
	got extensionpkg.ExtensionInfo,
	want extensionpkg.ExtensionInfo,
) {
	t.Helper()
	infoType := reflect.TypeOf(got)
	gotValue := reflect.ValueOf(got)
	wantValue := reflect.ValueOf(want)
	for index := range infoType.NumField() {
		field := infoType.Field(index)
		if !field.IsExported() {
			continue
		}
		gotField := gotValue.Field(index).Interface()
		wantField := wantValue.Field(index).Interface()
		if !reflect.DeepEqual(gotField, wantField) {
			t.Fatalf("%s %s = %#v, want %#v", label, field.Name, gotField, wantField)
		}
	}
}

type lifecycleFailingPublisher struct {
	mu        sync.Mutex
	remaining int
}

func (p *lifecycleFailingPublisher) Sync(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.remaining == 0 {
		return nil
	}
	p.remaining--
	return errors.New("injected extension resource reconcile failure")
}

func (p *lifecycleFailingPublisher) SyncSkills(ctx context.Context) error {
	return p.Sync(ctx)
}

func (p *lifecycleFailingPublisher) failNextSyncs(count int) {
	p.mu.Lock()
	p.remaining = count
	p.mu.Unlock()
}

type lifecycleFailureHarness struct {
	name      string
	digest    string
	registry  *extensionpkg.Registry
	runtime   *lifecycleStateRuntime
	publisher *lifecycleFailingPublisher
	service   *daemonExtensionService
	actor     taskpkg.ActorContext
	before    extensionpkg.ExtensionInfo
}

func newLifecycleFailureHarness(t *testing.T, name string) *lifecycleFailureHarness {
	t.Helper()
	db := openDaemonTestGlobalDB(t)
	registry, manifest := installNetworkLifecycleExtension(t, db, name)
	runtime := newLifecycleStateRuntime(registry)
	publisher := &lifecycleFailingPublisher{}
	service := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry:   registry,
		Runtime:    runtime,
		AgentSkill: publisher,
		HomePaths:  testHomePaths(t),
		Logger:     discardLogger(),
		Now:        time.Now,
	}, withDaemonExtensionAutomation(&fakeAutomationManager{})).(*daemonExtensionService)
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "lifecycle rollback")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		t.Fatalf("NetworkParticipationRequirementDigest() error = %v", err)
	}
	before, err := registry.Get(name)
	if err != nil {
		t.Fatalf("registry.Get(before) error = %v", err)
	}
	runtime.mu.Lock()
	runtime.reloads = []extensionpkg.ExtensionInfo{*before}
	runtime.mu.Unlock()
	return &lifecycleFailureHarness{
		name: name, digest: digest, registry: registry, runtime: runtime,
		publisher: publisher, service: service, actor: actor, before: *before,
	}
}

func (h *lifecycleFailureHarness) installFailureTrigger(t *testing.T, column string, condition string) {
	t.Helper()
	statement := "CREATE TEMP TRIGGER fail_extension_lifecycle_" + column +
		" BEFORE UPDATE OF " + column + " ON extensions WHEN NEW.name = '" + h.name + "' AND " + condition +
		" BEGIN SELECT RAISE(ABORT, 'injected lifecycle failure'); END"
	if _, err := h.registry.DB().ExecContext(t.Context(), statement); err != nil {
		t.Fatalf("install lifecycle failure trigger error = %v", err)
	}
}

func (h *lifecycleFailureHarness) assertRestored(t *testing.T) {
	t.Helper()
	after, err := h.registry.Get(h.name)
	if err != nil {
		t.Fatalf("registry.Get(after failure) error = %v", err)
	}
	assertExtensionPublicState(t, "registry after failure", *after, h.before)
	running, ok := h.runtime.current()
	if !ok {
		t.Fatalf("running state after failure = %#v/%t, want %#v", running, ok, h.before)
	}
	assertExtensionPublicState(t, "running state after failure", running, h.before)
}

func recordRuntimeHealthFailure(registry *mcppkg.RuntimeHealthRegistry, name string, generation string) {
	observation := registry.Begin(mcppkg.RuntimeHealthKey{
		InstanceName: name, BundleGeneration: generation, ServerName: "remote",
	})
	registry.RecordFailure(observation, errors.New("connection failed"))
}

func waitForLifecycleRefs(
	t *testing.T,
	coordinator *extensionLifecycleCoordinator,
	name string,
	want int,
) {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		coordinator.mu.Lock()
		entry := coordinator.entries[name]
		got := 0
		if entry != nil {
			got = entry.refs
		}
		changed := coordinator.changed
		coordinator.mu.Unlock()
		if got == want {
			return
		}
		select {
		case <-changed:
		case <-timer.C:
			t.Fatalf("lifecycle refs for %q did not reach %d", name, want)
		}
	}
}

func lifecycleTestSlug(value string) string {
	return strings.ReplaceAll(value, " ", "-")
}

func lifecycleNetworkExtensionDownloadResult(
	t *testing.T,
	version string,
	channelScope string,
) *registrypkg.DownloadResult {
	t.Helper()
	return &registrypkg.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(nativeExtensionTarGzWithNetwork(t, version, channelScope))),
		Slug:        "acme/tool-ext",
		Version:     version,
		ContentSize: -1,
		ContentType: "application/gzip",
	}
}

func lifecycleNetworkDigest(t *testing.T, channelScope string) string {
	t.Helper()
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(
		&extensionpkg.NetworkParticipationRequirement{
			Required: true, Mode: "live", ChannelScopes: []string{channelScope},
		},
	)
	if err != nil {
		t.Fatalf("NetworkParticipationRequirementDigest() error = %v", err)
	}
	return digest
}
