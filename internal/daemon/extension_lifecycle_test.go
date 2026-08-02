package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestExtensionLifecycleCoordinator(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize same-name mutations while allowing other names to proceed", func(t *testing.T) {
		t.Parallel()

		coordinator := newExtensionLifecycleCoordinator()
		firstEntered := make(chan struct{})
		releaseFirst := make(chan struct{})
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
		close(releaseFirst)
		requireLifecycleSignal(t, secondEntered, "second alpha mutation")
		for range 3 {
			if err := <-errorsCh; err != nil {
				t.Fatalf("withName() error = %v", err)
			}
		}
	})

	t.Run("Should cancel a waiter without retaining entries or running its mutation", func(t *testing.T) {
		t.Parallel()

		coordinator := newExtensionLifecycleCoordinator()
		exclusiveEntered := make(chan struct{})
		releaseExclusive := make(chan struct{})
		exclusiveDone := make(chan error, 1)
		go func() {
			exclusiveDone <- coordinator.exclusive(context.Background(), func() error {
				close(exclusiveEntered)
				<-releaseExclusive
				return nil
			})
		}()
		requireLifecycleSignal(t, exclusiveEntered, "exclusive mutation")

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
		if retained != 0 {
			t.Fatalf("retained lifecycle entries = %d, want 0", retained)
		}
		coordinator.gate.mu.Lock()
		readers := coordinator.gate.readers
		waitingWriters := coordinator.gate.waitingWriters
		coordinator.gate.mu.Unlock()
		if readers != 0 || waitingWriters != 0 {
			t.Fatalf("gate after cancellation readers=%d waiting_writers=%d, want zero", readers, waitingWriters)
		}

		close(releaseExclusive)
		if err := <-exclusiveDone; err != nil {
			t.Fatalf("exclusive() error = %v", err)
		}
	})

	t.Run("Should acquire multiple names in stable order without deadlock", func(t *testing.T) {
		t.Parallel()

		coordinator := newExtensionLifecycleCoordinator()
		var mu sync.Mutex
		var got []string
		err := coordinator.withNames(
			context.Background(),
			[]string{"beta", "alpha", "beta", " "},
			func() error {
				mu.Lock()
				got = append(got, "ran")
				mu.Unlock()
				return nil
			},
		)
		if err != nil {
			t.Fatalf("withNames() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"ran"}) {
			t.Fatalf("withNames() calls = %#v, want one mutation", got)
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
			registry,
			runtime,
			nil,
			nil,
			nil,
			nil,
			nil,
			deps.HomePaths,
			discardLogger(),
			time.Now,
			withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
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
		runtime.resetReloads()
		source.latestVersion = "2.0.0"
		entered, release := runtime.blockNextReload()

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
		waitForLifecycleRefs(t, &service.lifecycle, "tool-ext", 3)
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
		close(release)

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
	})

	t.Run("Should restore persisted and running state after every enable stage failure", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			configure func(*testing.T, *lifecycleFailureHarness)
		}{
			{
				name: "confirmation write",
				configure: func(t *testing.T, harness *lifecycleFailureHarness) {
					harness.installFailureTrigger(t, "network_confirmed_by", "NEW.network_confirmed_by IS NOT NULL")
				},
			},
			{
				name: "registry mutation",
				configure: func(t *testing.T, harness *lifecycleFailureHarness) {
					harness.installFailureTrigger(t, "enabled", "NEW.enabled = 1")
				},
			},
			{
				name: "runtime reload",
				configure: func(_ *testing.T, harness *lifecycleFailureHarness) {
					harness.runtime.failNextReloads(1)
				},
			},
			{
				name: "resource reconcile",
				configure: func(_ *testing.T, harness *lifecycleFailureHarness) {
					harness.publisher.failNextSyncs(1)
				},
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
				if err == nil {
					t.Fatalf("Enable(%s failure) error = nil, want failure", testCase.name)
				}
				harness.assertRestored(t)
			})
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
	service := newDaemonExtensionService(
		registry,
		runtime,
		nil,
		publisher,
		nil,
		nil,
		nil,
		testHomePaths(t),
		discardLogger(),
		time.Now,
	).(*daemonExtensionService)
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
	if !reflect.DeepEqual(*after, h.before) {
		t.Fatalf("registry after failure = %#v, want byte-equivalent %#v", *after, h.before)
	}
	running, ok := h.runtime.current()
	if !ok || !reflect.DeepEqual(running, h.before) {
		t.Fatalf("running state after failure = %#v/%t, want %#v", running, ok, h.before)
	}
}

func waitForLifecycleRefs(
	t *testing.T,
	coordinator *extensionLifecycleCoordinator,
	name string,
	want int,
) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		coordinator.mu.Lock()
		entry := coordinator.entries[name]
		got := 0
		if entry != nil {
			got = entry.refs
		}
		coordinator.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("lifecycle refs for %q did not reach %d", name, want)
}

func lifecycleTestSlug(value string) string {
	return strings.ReplaceAll(value, " ", "-")
}
