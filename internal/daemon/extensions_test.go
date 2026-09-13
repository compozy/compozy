package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionenv"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/extensionmcp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	profilepkg "github.com/compozy/compozy/internal/profile"
	registrypkg "github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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

func TestDaemonExtensionProfileReads(t *testing.T) {
	t.Parallel()
	// Invariant: the persistence fallback cannot resurrect a package whose
	// installation is absent. Owner: daemon snapshot loader; canonical suite: profile reads.
	t.Run("Should enforce attachment authority when falling back from the runtime", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		registry := extensionpkg.NewRegistry(db.DB())
		dir := writeNativeLocalExtensionFixture(t, "unattached-package", "1.0.0")
		manifest, err := extensionpkg.LoadManifest(dir)
		if err != nil {
			t.Fatal(err)
		}
		checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := registry.Install(manifest, dir, checksum); err != nil {
			t.Fatal(err)
		}
		if err := registry.DetachInstallation(
			t.Context(),
			manifest.Name,
			extensionpkg.InstallationScope{},
		); err != nil {
			t.Fatal(err)
		}
		for _, runtime := range []extensionRuntime{nil, extensionpkg.NewManager(registry)} {
			if _, err := loadExtensionSnapshot(
				registry,
				runtime,
				discardLogger(),
				manifest.Name,
			); !errors.Is(
				err,
				extensionpkg.ErrExtensionNotFound,
			) {
				t.Fatalf("unattached persistence fallback with runtime %T = %v, want not found", runtime, err)
			}
		}
		if err := registry.AttachInstallation(
			t.Context(),
			manifest.Name,
			extensionpkg.InstallationScope{},
		); err != nil {
			t.Fatal(err)
		}
		snapshot, err := loadExtensionSnapshot(registry, nil, discardLogger(), manifest.Name)
		if err != nil || snapshot.Manifest == nil || snapshot.Info.Name != manifest.Name {
			t.Fatalf("attached persistence fallback = %#v, %v", snapshot, err)
		}
	})

	t.Run("Should resolve the selected profile name", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		profiles, err := profilepkg.NewManager(
			profilepkg.WithStore(db),
			profilepkg.WithHomePaths(testHomePaths(t)),
			profilepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("profile.NewManager() error = %v", err)
		}
		created, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatalf("profiles.Create() error = %v", err)
		}

		service := &daemonExtensionService{profiles: profiles}
		got, err := service.extensionReadProfile(t.Context(), taskpkg.ActorContext{
			ReadScope: store.ReadScope{ProfileID: created.ID},
		})
		if err != nil {
			t.Fatalf("extensionReadProfile() error = %v", err)
		}
		if got.ID != created.ID || got.Name != created.Name {
			t.Fatalf("extensionReadProfile() = %#v, want profile %#v", got, created)
		}
	})

	t.Run("Should reject aggregate reads", func(t *testing.T) {
		t.Parallel()

		service := &daemonExtensionService{}
		_, err := service.extensionReadProfile(t.Context(), taskpkg.ActorContext{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err == nil || !strings.Contains(err.Error(), "one extension read profile is required") {
			t.Fatalf("extensionReadProfile(aggregate) error = %v, want aggregate rejection", err)
		}
	})

	t.Run("Should resolve the default profile without a manager", func(t *testing.T) {
		t.Parallel()

		service := &daemonExtensionService{}
		got, err := service.extensionReadProfile(t.Context(), taskpkg.ActorContext{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		})
		if err != nil {
			t.Fatalf("extensionReadProfile(default) error = %v", err)
		}
		if got.ID != store.DefaultProfileID || got.Name != "default" {
			t.Fatalf("extensionReadProfile(default) = %#v, want default profile lens", got)
		}
	})

	t.Run("Should require a manager for non-default profiles", func(t *testing.T) {
		t.Parallel()

		service := &daemonExtensionService{}
		_, err := service.extensionReadProfile(t.Context(), taskpkg.ActorContext{
			ReadScope: store.ReadScope{ProfileID: "profile-marketing"},
		})
		if err == nil || !strings.Contains(err.Error(), "profile manager is required") {
			t.Fatalf("extensionReadProfile(non-default) error = %v, want manager requirement", err)
		}
	})

	t.Run("Should pass the selected profile to a profile-aware runtime", func(t *testing.T) {
		t.Parallel()

		ext := &extensionpkg.Extension{Info: extensionpkg.ExtensionInfo{Name: "profile-aware"}}
		runtime := &profileReadProjectedRuntime{
			profileReadDevRuntime: &profileReadDevRuntime{ext: ext},
		}
		profile := extensionpkg.ProfileLens{ID: "profile-marketing", Name: "marketing"}
		service := &daemonExtensionService{}
		got, err := service.projectExtensionReadProfile(
			t.Context(), runtime, extensionpkg.InstanceKey{Name: "profile-aware"}, profile,
		)
		if err != nil {
			t.Fatalf("projectExtensionReadProfile(profile-aware) error = %v", err)
		}
		if got != ext || runtime.lastProfile != profile {
			t.Fatalf(
				"projectExtensionReadProfile(profile-aware) = %#v, profile = %#v; want original extension and %#v",
				got,
				runtime.lastProfile,
				profile,
			)
		}
	})

	t.Run("Should apply profile enablement when the runtime is not profile-aware", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		const name = "profile-read-kit"
		installDaemonTestExtension(t, db, name, daemonTestExtensionOptions{}, true)
		ext := &extensionpkg.Extension{
			Info:   extensionpkg.ExtensionInfo{Name: name, Enabled: true},
			Status: extensionpkg.ExtensionStatus{Name: name, Enabled: true, Registered: true},
		}
		runtime := &profileReadDevRuntime{ext: ext}
		service := &daemonExtensionService{registry: extensionpkg.NewRegistry(db.DB())}
		profile := extensionpkg.ProfileLens{ID: store.DefaultProfileID, Name: "default"}
		got, err := service.projectExtensionReadProfile(
			t.Context(), runtime, extensionpkg.InstanceKey{Name: name}, profile,
		)
		if err != nil {
			t.Fatalf("projectExtensionReadProfile(fallback) error = %v", err)
		}
		if got.Info.Enabled != true || got.Status.Enabled != true {
			t.Fatalf("projectExtensionReadProfile(fallback) = %#v, want enabled extension", got)
		}
	})

	t.Run("Should preserve the selected profile through scoped list and status reads", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		profiles, err := profilepkg.NewManager(
			profilepkg.WithStore(db),
			profilepkg.WithHomePaths(testHomePaths(t)),
			profilepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("profile.NewManager() error = %v", err)
		}
		created, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatalf("profiles.Create() error = %v", err)
		}
		ext := &extensionpkg.Extension{
			Info: extensionpkg.ExtensionInfo{Name: "profile-aware", Enabled: true},
			Status: extensionpkg.ExtensionStatus{
				Name: "profile-aware", Enabled: true, Registered: true,
			},
		}
		runtime := &profileReadProjectedRuntime{
			profileReadDevRuntime: &profileReadDevRuntime{ext: ext},
		}
		service := &daemonExtensionService{profiles: profiles, runtime: runtime}
		actor := taskpkg.ActorContext{
			Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "operator"},
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "extensions read"},
			Authority: taskpkg.Authority{Read: true},
			Scope:     taskpkg.CallerScope{Operator: true},
			ReadScope: store.ReadScope{ProfileID: created.ID},
		}

		listed, err := service.ListScoped(t.Context(), actor)
		if err != nil {
			t.Fatalf("ListScoped() error = %v", err)
		}
		if len(listed) != 1 || listed[0].Profile != created.Name ||
			runtime.lastProfile.ID != created.ID || runtime.lastProfile.Name != created.Name {
			t.Fatalf(
				"ListScoped() = %#v, projected profile = %#v; want owner %#v",
				listed,
				runtime.lastProfile,
				created,
			)
		}

		status, err := service.StatusScoped(t.Context(), ext.Info.Name, actor)
		if err != nil {
			t.Fatalf("StatusScoped() error = %v", err)
		}
		if status.Profile != created.Name || runtime.lastProfile.ID != created.ID ||
			runtime.lastProfile.Name != created.Name {
			t.Fatalf(
				"StatusScoped() = %#v, projected profile = %#v; want owner %#v",
				status,
				runtime.lastProfile,
				created,
			)
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
		firstDigest := lifecycleNetworkDigest(t, "builders")
		if _, err := service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceGitHub, Ref: "acme/tool-ext", AllowUnverified: true,
			ConfirmNetworkDigest: firstDigest,
		}, actor); err != nil {
			t.Fatalf("Install(v1) error = %v", err)
		}
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
					harness.installEnablementDeleteFailureTrigger(t)
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

		// Invariant: dev rollback retains candidate names when restoring the link fails, and releases only new names after success.
		// Owner: daemon lifecycle coordinator; canonical suite: TestExtensionLifecycleCoordinator.
		service.mcpAllocations = db.ExtensionMCP
		priorTarget := extensionmcp.Target{
			Extension:   key.Name,
			WorkspaceID: key.WorkspaceID,
			ProfileID:   store.DefaultProfileID,
			ServerName:  "retained",
		}
		if _, err := db.ExtensionMCP.Reserve(t.Context(), priorTarget, "retained-name", nil); err != nil {
			t.Fatal(err)
		}
		globalTarget := priorTarget
		globalTarget.WorkspaceID = ""
		if _, err := db.ExtensionMCP.Reserve(t.Context(), globalTarget, "global-name", nil); err != nil {
			t.Fatal(err)
		}
		allocations, err := service.snapshotMCPAllocations(t.Context(), key)
		if err != nil {
			t.Fatal(err)
		}
		candidateTarget := priorTarget
		candidateTarget.ServerName = "candidate-only"
		if _, err := db.ExtensionMCP.Reserve(t.Context(), candidateTarget, "candidate-name", nil); err != nil {
			t.Fatal(err)
		}
		err = service.rollbackDevLifecycle(t.Context(), runtime, key, snapshot, allocations, cause)
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
		rows, err := db.ExtensionMCP.ListAll(t.Context())
		if err != nil || len(rows) != 3 {
			t.Fatalf("failed link restoration released a name: %#v %v", rows, err)
		}
		runtime.stageErr, runtime.activateErr = nil, nil
		service.agentSkill = agentSkillPublisherFunc(func(ctx context.Context) error {
			rows, err := db.ExtensionMCP.ListAll(ctx)
			if err != nil || len(rows) != 2 {
				t.Fatalf("restored publication saw candidate name: %#v %v", rows, err)
			}
			return nil
		})
		if err := service.rollbackDevLifecycle(
			t.Context(),
			runtime,
			key,
			snapshot,
			allocations,
			cause,
		); !errors.Is(
			err,
			cause,
		) {
			t.Fatalf("rollback lost its cause: %v", err)
		}
		retained, err := db.ExtensionMCP.List(t.Context(), store.DefaultProfileID, key.WorkspaceID)
		if err != nil || len(retained) != 1 || retained[0].RuntimeName != "retained-name" {
			t.Fatalf("dev rollback changed prior name: %#v %v", retained, err)
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

type profileReadDevRuntime struct {
	ext *extensionpkg.Extension
}

var _ extensionDevRuntime = (*profileReadDevRuntime)(nil)
var _ extensionRuntime = (*profileReadDevRuntime)(nil)

func (*profileReadDevRuntime) Start(context.Context) error { return nil }

func (*profileReadDevRuntime) Stop(context.Context) error { return nil }

func (*profileReadDevRuntime) Reload(context.Context) error { return nil }

func (r *profileReadDevRuntime) Get(string) (*extensionpkg.Extension, error) {
	if r.ext == nil {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	return r.ext, nil
}

func (r *profileReadDevRuntime) InspectPackageResources(
	context.Context,
	string,
) (*extensionpkg.Extension, error) {
	if r.ext == nil {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	return r.ext, nil
}

func (r *profileReadDevRuntime) GetForInstance(extensionpkg.InstanceKey) (*extensionpkg.Extension, error) {
	if r.ext == nil {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	return r.ext, nil
}

func (r *profileReadDevRuntime) ListForWorkspace(string) []extensionpkg.ExtensionInfo {
	if r.ext == nil {
		return nil
	}
	return []extensionpkg.ExtensionInfo{r.ext.Info}
}

func (*profileReadDevRuntime) InspectDevelopmentGeneration(
	context.Context,
	string,
	string,
	string,
) (extensionpkg.DevelopmentGeneration, error) {
	return extensionpkg.DevelopmentGeneration{}, nil
}

func (*profileReadDevRuntime) LinkDevelopmentFromOrigin(
	context.Context,
	string,
	string,
	string,
) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (*profileReadDevRuntime) StageDevelopmentLink(
	context.Context,
	extensionpkg.InstanceKey,
	string,
	string,
) (*extensionpkg.DevLink, error) {
	return nil, nil
}

func (*profileReadDevRuntime) ActivateDevelopmentLink(
	context.Context,
	extensionpkg.InstanceKey,
) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (*profileReadDevRuntime) ReloadExtension(
	context.Context,
	extensionpkg.InstanceKey,
	string,
) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (*profileReadDevRuntime) UnlinkDevelopment(context.Context, extensionpkg.InstanceKey) error {
	return nil
}

func (*profileReadDevRuntime) Logs(
	extensionpkg.InstanceKey,
	extensionpkg.ExtensionLogCursor,
) (extensionpkg.ExtensionLogSnapshot, error) {
	return extensionpkg.ExtensionLogSnapshot{}, nil
}

type profileReadProjectedRuntime struct {
	*profileReadDevRuntime
	lastProfile extensionpkg.ProfileLens
}

var _ profiledExtensionRuntime = (*profileReadProjectedRuntime)(nil)

func (r *profileReadProjectedRuntime) ProjectForProfile(
	_ context.Context,
	_ extensionpkg.InstanceKey,
	profile extensionpkg.ProfileLens,
) (*extensionpkg.Extension, bool, error) {
	r.lastProfile = profile
	return r.ext, true, nil
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

func (h *lifecycleFailureHarness) installEnablementDeleteFailureTrigger(t *testing.T) {
	t.Helper()
	statement := "CREATE TEMP TRIGGER fail_extension_lifecycle_enablement" +
		" BEFORE DELETE ON extension_profile_enablement" +
		" WHEN OLD.extension_name = '" + h.name + "'" +
		" BEGIN SELECT RAISE(ABORT, 'injected lifecycle failure'); END"
	if _, err := h.registry.DB().ExecContext(t.Context(), statement); err != nil {
		t.Fatalf("install lifecycle enablement failure trigger error = %v", err)
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

// Invariant: install/update input preflight precedes managed writes, and failed resource publication restores
// files, registry, input rows, and vault state before reloading the old extension.
// Owner: daemon lifecycle orchestration. Canonical suite: extensions_test.go.
func TestDaemonExtensionInputLifecycle(t *testing.T) {
	t.Parallel()
	// Invariant: install attachment, inputs, secret ownership and response address the selected cell.
	// Owner: daemon install coordination; canonical suite: TestDaemonExtensionInputLifecycle.
	for _, scenario := range []daemonScopedInstallCase{
		{name: "Should retain global all-profile installation by default"},
		{name: "Should install for an explicit global profile", profile: "marketing"},
		{name: "Should install for an explicit default profile", profile: "default"},
		{name: "Should install for all profiles in a workspace", workspaceID: "ws-install"},
		{name: "Should install for one profile in a workspace", workspaceID: "ws-install", profile: "marketing"},
		{name: "Should bind an agent install to its trusted cell", workspaceID: "ws-install", profile: "marketing", agent: true},
		{name: "Should scope local package installation", workspaceID: "ws-install", profile: "marketing", local: true},
		{name: "Should honor workspace default in trusted operator context", workspaceID: "ws-install", manifestScope: "workspace", useDefaults: true},
		{name: "Should honor local package workspace default", workspaceID: "ws-install", manifestScope: "workspace", useDefaults: true, local: true},
		{name: "Should let explicit global scope override a workspace default", scope: "global", manifestScope: "workspace"},
		{name: "Should require workspace context for a workspace default", manifestScope: "workspace", useDefaults: true, errorField: "workspace_id"},
		{name: "Should require explicit scope for mixed server defaults", manifestScope: "workspace", mixed: true, errorField: "scope"},
		{name: "Should honor explicit workspace scope over mixed defaults", workspaceID: "ws-install", scope: "workspace", manifestScope: "workspace", mixed: true},
		// Invariant: updates validate/restore only selected inputs and preserve every attachment.
		// Owner: daemon lifecycle coordination; canonical suite: TestDaemonExtensionInputLifecycle.
		{name: "Should update and roll back inputs for an explicit global profile", profile: "marketing", update: true},
		{name: "Should update and roll back inputs for a workspace profile", workspaceID: "ws-install", profile: "marketing", update: true},
		{name: "Should update and roll back inputs for a trusted agent", workspaceID: "ws-install", profile: "marketing", agent: true, update: true},
		{name: "Should persist scoped native update inputs", workspaceID: "ws-install", profile: "marketing", update: true, native: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			testDaemonScopedInstall(t, scenario)
		})
	}
	t.Run("Should reject invalid selectors and cross-scope writes before acquisition", func(t *testing.T) {
		t.Parallel()
		deps, registry, _, _ := newNativeExtensionToolDeps(t)
		marketing, err := deps.ProfileManager.Create(t.Context(), profilepkg.CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatal(err)
		}
		service := newDaemonExtensionService(&daemonExtensionServiceDeps{
			Registry: registry, Profiles: deps.ProfileManager, HomePaths: deps.HomePaths,
			Logger: discardLogger(),
		}, withDaemonExtensionWorkspaceResolver(&daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-install", RootDir: t.TempDir()}, WorkspaceID: "ws-install",
		}})).(*daemonExtensionService)
		operator, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "scope validation")
		if err != nil {
			t.Fatal(err)
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("session-install", "ws-install")
		if err != nil {
			t.Fatal(err)
		}
		agent.ReadScope = store.ReadScope{ProfileID: marketing.ID}
		for _, scenario := range []struct {
			name    string
			request contract.InstallExtensionRequest
			actor   taskpkg.ActorContext
			field   string
		}{
			{name: "Should reject unknown scope", request: contract.InstallExtensionRequest{Scope: "other"}, actor: operator, field: "scope"},
			{name: "Should reject contradictory workspace", request: contract.InstallExtensionRequest{Scope: "global", WorkspaceID: "ws-install"}, actor: operator, field: "workspace_id"},
			{name: "Should require a workspace", request: contract.InstallExtensionRequest{Scope: "workspace"}, actor: operator, field: "workspace_id"},
			{name: "Should require an existing profile", request: contract.InstallExtensionRequest{Profile: "absent"}, actor: operator, field: "profile"},
			{name: "Should forbid another workspace", request: contract.InstallExtensionRequest{WorkspaceID: "ws-other"}, actor: agent},
			{name: "Should forbid global escape", request: contract.InstallExtensionRequest{Scope: "global"}, actor: agent},
			{name: "Should forbid another profile", request: contract.InstallExtensionRequest{Profile: "default"}, actor: agent},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				t.Parallel()
				// Invalid source deliberately cannot reach acquisition; the selector error must win.
				_, err := service.Install(t.Context(), scenario.request, scenario.actor)
				if scenario.field == "" {
					if !errors.Is(err, taskpkg.ErrPermissionDenied) {
						t.Fatalf("install error = %v", err)
					}
				} else if validation, ok := errors.AsType[*extensionpkg.ManifestValidationError](err); !ok || validation.Field != scenario.field {
					t.Fatalf("install error = %v, want selector %s", err, scenario.field)
				}
			})
		}
	})

	// Invariant: automatic allocations survive success and only new allocations are removed on failed installation.
	// Owner: daemon install coordinator; canonical suite: TestDaemonExtensionInputLifecycle with real SQLite.
	for _, scenario := range []string{"success", "publication failure", "completion failure"} {
		t.Run("Should handle runtime allocation on "+scenario, func(t *testing.T) {
			t.Parallel()
			deps, registry, source, runtime := newNativeExtensionToolDeps(t)
			db := deps.ExtensionEvents.(*globaldb.GlobalDB)
			sections := "[resources.mcp_servers.server]\ncommand = \"server\"\n"
			archive := nativeExtensionTarGzWithNetwork(t, "1.0.0", "", sections)
			source.latestVersion = "1.0.0"
			source.downloads["1.0.0"] = &registrypkg.DownloadResult{
				Reader:      io.NopCloser(bytes.NewReader(archive)),
				Slug:        "acme/tool-ext",
				Version:     "1.0.0",
				ContentSize: int64(len(archive)),
				ContentType: "application/gzip",
			}
			for _, target := range []extensionmcp.Target{
				{Extension: "tool-ext", ProfileID: store.DefaultProfileID, ServerName: "legacy"},
				{Extension: "tool-ext", ProfileID: store.DefaultProfileID, WorkspaceID: "workspace-kept", ServerName: "server"},
			} {
				if _, err := db.ExtensionMCP.Reserve(t.Context(), target, "", nil); err != nil {
					t.Fatal(err)
				}
			}
			service := newDaemonExtensionService(&daemonExtensionServiceDeps{Registry: registry, Runtime: runtime, HomePaths: deps.HomePaths, Logger: discardLogger(), Now: time.Now},
				withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources), withDaemonExtensionMCPAllocations(db.ExtensionMCP),
			).(*daemonExtensionService)
			sentinel := errors.New("injected post-allocation failure")
			if scenario == "completion failure" {
				service.eventWriter = &daemonExtensionEventStoreStub{writeErr: sentinel}
			}
			runtime.onReload = func(ctx context.Context) error {
				if _, err := registry.Get("tool-ext"); errors.Is(err, extensionpkg.ErrExtensionNotFound) {
					return nil
				}
				for _, profileID := range []string{store.DefaultProfileID, "another-profile"} {
					if _, err := db.ExtensionMCP.Reserve(ctx, extensionmcp.Target{
						Extension: "tool-ext", ProfileID: profileID, ServerName: "server",
					}, "", nil); err != nil {
						return err
					}
				}
				if scenario == "publication failure" {
					return sentinel
				}
				return nil
			}
			actor, err := taskpkg.DeriveHumanActorContext(
				"operator",
				taskpkg.OriginKindCLI,
				"runtime allocation lifecycle",
			)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Install(
				t.Context(),
				contract.InstallExtensionRequest{
					Source:          contract.InstallExtensionSourceGitHub,
					Ref:             "acme/tool-ext",
					AllowUnverified: true,
				},
				actor,
			)
			records, readErr := db.ExtensionMCP.ListAll(t.Context())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if scenario == "success" {
				if err != nil || len(records) != 4 {
					t.Fatalf("successful allocation: %#v %v", records, err)
				}
				found := false
				for _, record := range records {
					if record.ProfileID == store.DefaultProfileID && record.WorkspaceID == "" &&
						record.ServerName == "server" {
						found = record.RuntimeName == "server"
					}
				}
				if !found {
					t.Fatal("automatic runtime name was not retained")
				}
			} else {
				if err == nil || len(records) != 2 {
					t.Fatalf("failed install changed prior allocations: %#v %v", records, err)
				}
				if _, getErr := registry.Get("tool-ext"); !errors.Is(getErr, extensionpkg.ErrExtensionNotFound) {
					t.Fatalf("failed install left registry row: %v", getErr)
				}
			}
		})
	}

	t.Run(
		"Should validate inputs before install and restore them before update rollback reload [IT-020]",
		func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			deps, registry, source, runtime := newNativeExtensionToolDeps(t)
			db, ok := deps.ExtensionEvents.(*globaldb.GlobalDB)
			if !ok {
				t.Fatal("fixture must expose its real global database")
			}
			secretVault, err := vault.NewService(db.VaultRepo, vault.NewFileKeyProvider(t.TempDir(), nil))
			if err != nil {
				t.Fatal(err)
			}
			publisher := &lifecycleFailingPublisher{}
			service := newDaemonExtensionService(&daemonExtensionServiceDeps{
				Registry: registry, Runtime: runtime, AgentSkill: publisher, HomePaths: deps.HomePaths,
				Logger: discardLogger(), Now: time.Now, Getenv: func(string) string { return "" },
			}, withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
				withDaemonExtensionInputs(db.ExtensionInputs), withDaemonExtensionSecrets(db.ExtensionEnvRepo, secretVault),
				withDaemonExtensionMCPAllocations(db.ExtensionMCP),
			).(*daemonExtensionService)
			actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "input lifecycle")
			if err != nil {
				t.Fatal(err)
			}
			setSource := func(version string, addRegion bool) string {
				sections := `[resources.mcp_servers.server]
command = "server"
secret_env = { TOKEN = "token" }
[resources.mcp_servers.remote]
transport = "http"
url = "https://example.com/mcp?ws=&region="
[[inputs]]
id = "workspace"
prompt = "Workspace"
type = "identifier"
required = true
binding = { type = "url_query", name = "ws" }
[[inputs]]
id = "token"
prompt = "Token"
type = "secret"
required = true
binding = { type = "env", name = "TOKEN" }
`
				if addRegion {
					sections += `
[[inputs]]
id = "region"
prompt = "Region"
type = "identifier"
required = true
binding = { type = "url_query", name = "region" }
`
				}
				archive := nativeExtensionTarGzWithNetwork(t, version, "", sections)
				source.latestVersion = version
				source.downloads[version] = &registrypkg.DownloadResult{
					Reader: io.NopCloser(bytes.NewReader(archive)), Slug: "acme/tool-ext", Version: version,
					ContentSize: int64(len(archive)), ContentType: "application/gzip",
				}
				return fmt.Sprintf("%x", sha256.Sum256(archive))
			}
			digest := setSource("1.0.0", false)
			request := contract.InstallExtensionRequest{Source: contract.InstallExtensionSourceGitHub,
				Ref: "acme/tool-ext", AllowUnverified: true, ExpectedDigest: digest}
			profiles, err := profilepkg.NewManager(profilepkg.WithStore(db),
				profilepkg.WithHomePaths(deps.HomePaths), profilepkg.WithLogger(discardLogger()))
			if err != nil {
				t.Fatal(err)
			}
			service.profiles = profiles
			preview, err := service.PreviewInstall(ctx, request, actor)
			if err != nil || preview.DigestSHA256 != digest || len(preview.Inputs) != 2 {
				t.Fatalf("unconfigured install preview must expose types and approved digest: %v, %v", preview, err)
			}
			request.ExpectedDigest = setSource("1.0.0", false)
			if _, err := service.Install(
				ctx,
				request,
				actor,
			); !errors.Is(
				err,
				extensionpkg.ErrExtensionInputsRequired,
			) {
				t.Fatalf("missing-input install error = %v", err)
			} else if required, ok := errors.AsType[*extensionpkg.InputsRequiredError](err); !ok ||
				!slices.Equal(required.MissingInputs, []string{"token", "workspace"}) {
				t.Fatalf("missing-input error must name every manifest input, including secret ids: %v", err)
			}
			if _, err := registry.Get("tool-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
				t.Fatalf("preflight changed registry: %v", err)
			}
			if _, err := os.Stat(
				extensionpkg.ManagedInstallPath(deps.HomePaths, "tool-ext"),
			); !errors.Is(
				err,
				os.ErrNotExist,
			) {
				t.Fatalf("preflight changed managed files: %v", err)
			}
			request.ExpectedDigest = setSource("1.0.0", false)
			request.Inputs = map[string]extensioninput.Value{
				"workspace": {
					Value: json.RawMessage(`"original-team"`),
				},
				"token": {Value: json.RawMessage(`"original-secret"`)},
			}
			installed, err := service.Install(ctx, request, actor)
			if err != nil {
				t.Fatal(err)
			}
			if len(installed.MissingInputs) != 0 || len(installed.MissingEnv) != 0 {
				t.Fatalf("installed readiness = %#v", installed)
			}
			instance := extensioninput.Instance{Extension: "tool-ext", ProfileID: store.DefaultProfileID}
			before, err := db.ExtensionInputs.List(ctx, instance)
			if err != nil {
				t.Fatal(err)
			}
			setSource("2.0.0", true)
			if _, err := service.Update(
				ctx,
				"tool-ext",
				contract.UpdateExtensionRequest{AllowUnverified: true},
				actor,
			); !errors.Is(
				err,
				extensionpkg.ErrExtensionInputsRequired,
			) {
				t.Fatalf("new required input update error = %v", err)
			}
			info, err := registry.Get("tool-ext")
			if err != nil || info.Version != "1.0.0" {
				t.Fatalf("preflight changed installed version: %#v %v", info, err)
			}
			// Invariant: a failed update drops only candidate allocations before restoring publication.
			// Owner: daemon lifecycle coordination; canonical suite: TestDaemonExtensionInputLifecycle.
			priorTarget := extensionmcp.Target{
				Extension:  "tool-ext",
				ProfileID:  store.DefaultProfileID,
				ServerName: "server",
			}
			priorAllocation, err := db.ExtensionMCP.Reserve(ctx, priorTarget, "original-server", nil)
			if err != nil {
				t.Fatal(err)
			}
			workspaceTarget := priorTarget
			workspaceTarget.WorkspaceID = "other-workspace"
			if _, err := db.ExtensionMCP.Reserve(ctx, workspaceTarget, "workspace-server", nil); err != nil {
				t.Fatal(err)
			}
			candidateTarget := priorTarget
			candidateTarget.ServerName = "candidate-only"
			var observations []string
			runtime.onReload = func(ctx context.Context) error {
				current, err := registry.Get("tool-ext")
				if err != nil {
					return err
				}
				rows, err := db.ExtensionInputs.List(ctx, instance)
				if err != nil {
					return err
				}
				if current.Version == "2.0.0" {
					if _, err := db.ExtensionMCP.Reserve(ctx, candidateTarget, "candidate-server", nil); err != nil {
						return err
					}
				} else {
					allocations, err := db.ExtensionMCP.ListAll(ctx)
					if err != nil || len(allocations) != 2 {
						t.Fatalf("rollback publication saw candidate allocation: %#v %v", allocations, err)
					}
				}
				observations = append(observations, current.Version+":"+string(rows["workspace"].Value))
				return nil
			}
			setSource("2.0.0", true)
			publisher.failNextSyncs(1)
			_, err = service.Update(ctx, "tool-ext", contract.UpdateExtensionRequest{
				AllowUnverified: true, Inputs: map[string]extensioninput.Value{
					"workspace": {
						Value: json.RawMessage(`"updated-team"`),
					},
					"token":  {Value: json.RawMessage(`"updated-secret"`)},
					"region": {Value: json.RawMessage(`"us"`)},
				},
			}, actor)
			if err == nil {
				t.Fatal("publication failure was ignored")
			}
			if !slices.Equal(observations, []string{`2.0.0:"updated-team"`, `1.0.0:"original-team"`}) {
				t.Fatalf("rollback reload order = %v", observations)
			}
			allocations, err := db.ExtensionMCP.List(ctx, store.DefaultProfileID, "")
			if err != nil || len(allocations) != 1 || allocations[0].RuntimeName != priorAllocation.RuntimeName {
				t.Fatalf("update rollback changed retained allocation: %#v %v", allocations, err)
			}
			after, err := db.ExtensionInputs.List(ctx, instance)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("update rollback rows = %#v want %#v error %v", after, before, err)
			}
			ref := vault.ExtensionProfileSecretRef("tool-ext", store.DefaultProfileID, "", "TOKEN")
			if value, err := secretVault.ResolveRef(ctx, ref); err != nil || value != "original-secret" {
				t.Fatal("update rollback did not restore vault material")
			}
			version, err := os.ReadFile(
				filepath.Join(extensionpkg.ManagedInstallPath(deps.HomePaths, "tool-ext"), "VERSION.txt"),
			)
			if err != nil || strings.TrimSpace(string(version)) != "1.0.0" {
				t.Fatalf("update rollback file version = %q error %v", version, err)
			}
			setSource("2.0.0", true)
			updated, err := service.Update(ctx, "tool-ext", contract.UpdateExtensionRequest{
				AllowUnverified: true,
				Inputs:          map[string]extensioninput.Value{"region": {Value: json.RawMessage(`"us"`)}},
			}, actor)
			if err != nil || updated.Status != extensionpkg.MarketplaceUpdateStatusUpdated {
				t.Fatalf("retry update = %#v error %v", updated, err)
			}
			allocations, err = db.ExtensionMCP.List(ctx, store.DefaultProfileID, "")
			if err != nil || len(allocations) != 2 {
				t.Fatalf("successful update lost allocation: %#v %v", allocations, err)
			}
		},
	)
}

type daemonScopedInstallCase struct {
	native        bool
	update        bool
	name          string
	workspaceID   string
	profile       string
	agent         bool
	local         bool
	scope         string
	manifestScope string
	mixed         bool
	useDefaults   bool
	errorField    string
}

func testDaemonScopedInstall(t *testing.T, scenario daemonScopedInstallCase) {
	t.Helper()
	workspaceID, profileName, agent, local := scenario.workspaceID, scenario.profile, scenario.agent, scenario.local
	ctx := t.Context()
	deps, registry, source, _ := newNativeExtensionToolDeps(t)
	db := deps.ExtensionEvents.(*globaldb.GlobalDB)
	workspaceRoot := t.TempDir()
	stamp := store.FormatTimestamp(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))
	if _, err := db.DB().ExecContext(ctx, `INSERT INTO workspaces
		(id, root_dir, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"ws-install", workspaceRoot, "Install workspace", stamp, stamp); err != nil {
		t.Fatal(err)
	}

	marketing, err := deps.ProfileManager.Create(ctx, profilepkg.CreateInput{Name: "marketing"})
	if err != nil {
		t.Fatal(err)
	}
	profileID := store.DefaultProfileID
	if profileName == "marketing" {
		profileID = marketing.ID
	}
	secretVault, err := vault.NewService(db.VaultRepo, vault.NewFileKeyProvider(t.TempDir(), nil))
	if err != nil {
		t.Fatal(err)
	}
	service := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry: registry, Profiles: deps.ProfileManager, HomePaths: deps.HomePaths,
		Logger: discardLogger(), Getenv: func(string) string { return "" },
	}, withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
		withDaemonExtensionInputs(db.ExtensionInputs), withDaemonExtensionSecrets(db.ExtensionEnvRepo, secretVault),
		withDaemonExtensionWorkspaceResolver(&daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-install", RootDir: workspaceRoot}, WorkspaceID: "ws-install",
		}}),
	).(*daemonExtensionService)
	sections := `[resources.mcp_servers.server]
command = "server"
secret_env = { TOKEN = "token" }
[resources.mcp_servers.remote]
transport = "http"
url = "https://example.com/mcp?ws="
[[inputs]]
id = "workspace"
prompt = "Workspace"
type = "identifier"
required = true
binding = { type = "url_query", name = "ws" }
[[inputs]]
id = "token"
prompt = "Token"
type = "secret"
required = true
binding = { type = "env", name = "TOKEN" }
`
	if scenario.manifestScope != "" {
		sections = strings.Replace(sections, "[resources.mcp_servers.server]\n",
			fmt.Sprintf("[resources.mcp_servers.server]\ndefault_scope = %q\n", scenario.manifestScope), 1)
		if !scenario.mixed {
			sections = strings.Replace(sections, "[resources.mcp_servers.remote]\n",
				fmt.Sprintf("[resources.mcp_servers.remote]\ndefault_scope = %q\n", scenario.manifestScope), 1)
		}
	}
	archive := nativeExtensionTarGzWithNetwork(t, "1.0.0", "", sections)
	source.latestVersion = "1.0.0"
	source.downloads["1.0.0"] = &registrypkg.DownloadResult{
		Reader: io.NopCloser(bytes.NewReader(archive)), Slug: "acme/tool-ext", Version: "1.0.0",
		ContentSize: int64(len(archive)), ContentType: "application/gzip",
	}
	request := contract.InstallExtensionRequest{Source: contract.InstallExtensionSourceGitHub,
		Ref: "acme/tool-ext", AllowUnverified: true, Profile: profileName, WorkspaceID: workspaceID, Scope: scenario.scope,
		Inputs: map[string]extensioninput.Value{
			"workspace": {Value: json.RawMessage(`"selected-team"`)}, "token": {Value: json.RawMessage(`"scoped-secret"`)},
		},
	}
	if local {
		request.Source = contract.InstallExtensionSourceLocalPath
		request.Ref = writeNativeLocalExtensionFixture(t, "tool-ext", "1.0.0")
		manifestPath := filepath.Join(request.Ref, "extension.toml")
		manifest, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(manifestPath, append(manifest, []byte("\n"+sections)...), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "scoped install")
	if err != nil {
		t.Fatal(err)
	}
	if scenario.useDefaults {
		request.WorkspaceID = ""
		actor.Scope.WorkspaceID = workspaceID
	}
	if agent {
		actor, err = taskpkg.DeriveAgentSessionActorContext("session-install", workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		actor.ReadScope = store.ReadScope{ProfileID: profileID}
		request.Profile, request.WorkspaceID = "", ""
	}
	installed, err := service.Install(ctx, request, actor)
	if scenario.errorField != "" {
		if validation, ok := errors.AsType[*extensionpkg.ManifestValidationError](err); !ok || validation.Field != scenario.errorField {
			t.Fatalf("install error = %v, want field %s", err, scenario.errorField)
		}
		if _, err := registry.Get("tool-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("failed install persisted registry row: %v", err)
		}
		if _, err := os.Stat(extensionpkg.ManagedInstallPath(deps.HomePaths, "tool-ext")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed default selection left managed package: %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if profileName == "" {
		profileName = "default"
	}
	if installed.Profile != profileName || installed.WorkspaceID != workspaceID || len(installed.MissingInputs) != 0 {
		t.Fatalf("installed scope/readiness = %#v", installed)
	}
	attachments, err := registry.Installations(ctx, "tool-ext")
	if err != nil {
		t.Fatal(err)
	}
	attachmentProfile := ""
	if request.Profile != "" || agent {
		attachmentProfile = profileID
	}
	wantScope := extensionpkg.InstallationScope{ProfileID: attachmentProfile, WorkspaceID: workspaceID}
	if len(attachments) != 1 || attachments[0].Scope != wantScope {
		t.Fatalf("attachments = %#v, want %v", attachments, wantScope)
	}
	for _, cell := range []extensioninput.Instance{
		{Extension: "tool-ext", ProfileID: store.DefaultProfileID},
		{Extension: "tool-ext", ProfileID: marketing.ID},
		{Extension: "tool-ext", ProfileID: store.DefaultProfileID, WorkspaceID: "ws-install"},
		{Extension: "tool-ext", ProfileID: marketing.ID, WorkspaceID: "ws-install"},
	} {
		rows, readErr := db.ExtensionInputs.List(ctx, cell)
		if readErr != nil {
			t.Fatal(readErr)
		}
		bindings, readErr := db.ExtensionEnvRepo.ListEnvBindings(ctx, "tool-ext", cell.ProfileID, cell.WorkspaceID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if cell.ProfileID != profileID || cell.WorkspaceID != workspaceID {
			if len(rows) != 0 || len(bindings) != 0 {
				t.Fatalf("install leaked to %v: %v, %v", cell, rows, bindings)
			}
			continue
		}
		if len(rows) != 1 || string(rows["workspace"].Value) != `"selected-team"` || !rows["workspace"].Active {
			t.Fatalf("selected inputs = %v", rows)
		}
		wantRef := vault.ExtensionProfileSecretRef("tool-ext", profileID, workspaceID, "TOKEN")
		secret, resolveErr := secretVault.ResolveRef(ctx, wantRef)
		if resolveErr != nil || secret != "scoped-secret" {
			t.Fatal("selected vault secret was not persisted", resolveErr)
		}

		if len(bindings) != 1 || bindings[0].SecretRef != wantRef || bindings[0].Inactive {
			t.Fatalf("selected secret bindings = %v, want %s", bindings, wantRef)
		}
	}

	if !scenario.update {
		return
	}
	selectedCell := extensioninput.Instance{Extension: "tool-ext", ProfileID: profileID, WorkspaceID: workspaceID}
	beforeRows, err := db.ExtensionInputs.List(ctx, selectedCell)
	if err != nil {
		t.Fatal(err)
	}
	otherWorkspace := "ws-install"
	if workspaceID != "" {
		otherWorkspace = ""
	}
	otherCell := extensioninput.Instance{Extension: "tool-ext", ProfileID: store.DefaultProfileID, WorkspaceID: otherWorkspace}
	otherScope := extensionpkg.InstallationScope{ProfileID: otherCell.ProfileID, WorkspaceID: otherCell.WorkspaceID}
	if err := registry.AttachInstallation(ctx, "tool-ext", otherScope); err != nil {
		t.Fatal(err)
	}
	otherRecord := extensioninput.Record{Type: "identifier", Value: json.RawMessage(`"other-team"`), Active: true,
		UpdatedAt: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)}
	if err := db.ExtensionInputs.Apply(ctx, otherCell, []extensioninput.Mutation{{InputID: "workspace", After: &otherRecord}}); err != nil {
		t.Fatal(err)
	}
	otherRef := vault.ExtensionProfileSecretRef("tool-ext", otherCell.ProfileID, otherCell.WorkspaceID, "TOKEN")
	if _, err := secretVault.PutSecret(ctx, otherRef, extensionenv.BindingKind, "other-secret"); err != nil {
		t.Fatal(err)
	}
	if err := db.ExtensionEnvRepo.PutEnvBinding(ctx, extensionenv.Binding{
		ExtensionName: "tool-ext", ProfileID: otherCell.ProfileID, WorkspaceID: otherCell.WorkspaceID,
		EnvName: "TOKEN", InputID: "token", SecretRef: otherRef, Kind: extensionenv.BindingKind,
		CreatedAt: otherRecord.UpdatedAt, UpdatedAt: otherRecord.UpdatedAt,
	}); err != nil {
		t.Fatal(err)
	}
	attachments, err = registry.Installations(ctx, "tool-ext")
	if err != nil {
		t.Fatal(err)
	}
	candidateSections := strings.Replace(sections, "mcp?ws=", "mcp?ws=&region=", 1) + `
[[inputs]]
id = "region"
prompt = "Region"
type = "identifier"
required = true
binding = { type = "url_query", name = "region" }
`
	setCandidate := func() {
		archive := nativeExtensionTarGzWithNetwork(t, "2.0.0", "", candidateSections)
		source.latestVersion = "2.0.0"
		source.downloads["2.0.0"] = &registrypkg.DownloadResult{
			Reader: io.NopCloser(bytes.NewReader(archive)), Slug: "acme/tool-ext", Version: "2.0.0",
			ContentSize: int64(len(archive)), ContentType: "application/gzip",
		}
	}
	setCandidate()
	batchRequest := contract.UpdateExtensionsRequest{All: true, CheckOnly: true, AllowUnverified: true,
		Profile: profileName, WorkspaceID: otherWorkspace}
	batchRequest.Scope = "workspace"
	if otherWorkspace == "" {
		batchRequest.Scope = "global"
	}
	filtered, err := service.UpdateBatch(ctx, batchRequest, actor)
	if agent {
		if !errors.Is(err, taskpkg.ErrPermissionDenied) {
			t.Fatalf("cross-workspace update batch error = %v", err)
		}
	} else if err != nil || len(filtered) != 0 {
		t.Fatalf("update batch included an installation outside the selected cell: %v, %v", filtered, err)
	}
	batchRequest.WorkspaceID = workspaceID
	batchRequest.Scope = "workspace"
	if workspaceID == "" {
		batchRequest.Scope = "global"
	}
	available, err := service.UpdateBatch(ctx, batchRequest, actor)
	if err != nil || len(available) != 1 || available[0].Status != extensionpkg.MarketplaceUpdateStatusAvailable {
		t.Fatalf("scoped update batch = %v, %v", available, err)
	}
	updateRequest := contract.UpdateExtensionRequest{AllowUnverified: true, Profile: profileName, WorkspaceID: workspaceID}
	if agent {
		updateRequest.Profile, updateRequest.WorkspaceID = "", ""
	}
	setCandidate()
	if _, err := service.Update(ctx, "tool-ext", updateRequest, actor); !errors.Is(err, extensionpkg.ErrExtensionInputsRequired) {
		t.Fatalf("missing scoped update input error = %v", err)
	}
	updateRequest.Inputs = map[string]extensioninput.Value{
		"workspace": {Value: json.RawMessage(`"updated-team"`)},
		"region":    {Value: json.RawMessage(`"west"`)},
		"token":     {Value: json.RawMessage(`"updated-secret"`)},
	}
	publisher := &lifecycleFailingPublisher{}
	// The public update must hold the input cell lock through candidate and rollback publication.
	service.runtime = &fakeExtensionRuntime{onReload: func(reloadCtx context.Context) error {
		lockCtx, cancel := context.WithTimeout(reloadCtx, 40*time.Millisecond)
		defer cancel()
		entered := false
		lockErr := service.lifecycle.withInstance(lockCtx, extensionpkg.InstanceKey{Name: "tool-ext", WorkspaceID: workspaceID}, func() error {
			entered = true
			return nil
		})
		if entered || !errors.Is(lockErr, context.DeadlineExceeded) {
			t.Errorf("update publication did not hold the selected input cell lock: %v", lockErr)
		}
		return nil
	}}
	service.agentSkill = publisher
	publisher.failNextSyncs(1)
	setCandidate()
	if _, err := service.Update(ctx, "tool-ext", updateRequest, actor); err == nil {
		t.Fatal("publication failure must fail update")
	}
	rolledBackRows, err := db.ExtensionInputs.List(ctx, selectedCell)
	if err != nil || !reflect.DeepEqual(rolledBackRows, beforeRows) {
		t.Fatalf("input rollback changed before-images: %v", err)
	}
	targetRef := vault.ExtensionProfileSecretRef("tool-ext", profileID, workspaceID, "TOKEN")
	secret, err := secretVault.ResolveRef(ctx, targetRef)
	if err != nil || secret != "scoped-secret" {
		t.Fatal("update rollback failed to restore selected secret", err)
	}
	oldManifest, err := extensionpkg.LoadManifest(extensionpkg.ManagedInstallPath(deps.HomePaths, "tool-ext"))
	if err != nil || oldManifest.Version != "1.0.0" {
		t.Fatalf("rollback did not restore manifest: %v", err)
	}
	setCandidate()
	var updated contract.ManagedExtensionUpdatePayload
	if scenario.native {
		deps.Extensions = func() apicore.ExtensionService { return service }
		nativeRegistry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
		body, marshalErr := json.Marshal(struct {
			Name string `json:"name"`
			contract.UpdateExtensionRequest
		}{"tool-ext", updateRequest})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		result, callErr := nativeRegistry.Call(ctx, toolspkg.Scope{Operator: true, ProfileID: profileID, WorkspaceID: workspaceID},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDExtensionsUpdate, Input: body})
		if callErr != nil {
			t.Fatal(callErr)
		}
		var payload struct {
			Updates []contract.ManagedExtensionUpdatePayload `json:"updates"`
		}
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Updates) != 1 {
			t.Fatalf("native updates = %#v", payload)
		}
		updated = payload.Updates[0]
	} else {
		updated, err = service.Update(ctx, "tool-ext", updateRequest, actor)
	}
	if err != nil || updated.Status != extensionpkg.MarketplaceUpdateStatusUpdated {
		t.Fatalf("scoped update = %#v, %v", updated, err)
	}
	afterRows, err := db.ExtensionInputs.List(ctx, selectedCell)
	if err != nil || len(afterRows) != 2 || string(afterRows["workspace"].Value) != `"updated-team"` ||
		string(afterRows["region"].Value) != `"west"` {
		t.Fatalf("selected update rows = %v, %v", afterRows, err)
	}
	secret, err = secretVault.ResolveRef(ctx, targetRef)
	if err != nil || secret != "updated-secret" {
		t.Fatal("selected secret update was not persisted", err)
	}
	otherRows, err := db.ExtensionInputs.List(ctx, otherCell)
	if err != nil || len(otherRows) != 1 || !reflect.DeepEqual(otherRows["workspace"], otherRecord) {
		t.Fatalf("update changed another cell: %v, %v", otherRows, err)
	}
	secret, err = secretVault.ResolveRef(ctx, otherRef)
	if err != nil || secret != "other-secret" {
		t.Fatal("update changed another cell's secret", err)
	}
	afterAttachments, err := registry.Installations(ctx, "tool-ext")
	if err != nil || !reflect.DeepEqual(afterAttachments, attachments) {
		t.Fatalf("update changed attachment before-images: %v", err)
	}
}
