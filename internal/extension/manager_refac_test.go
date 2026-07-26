package extensionpkg

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
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

		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		err := manager.Stop(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Stop() error = %v, want canceled shutdown context", err)
		}
		if proc.shutdownCount() != 1 {
			t.Fatalf("Shutdown() count = %d, want 1", proc.shutdownCount())
		}
	})
}

func TestManagerResourceSourceCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should reset extension source on stop", func(t *testing.T) {
		t.Parallel()

		sourceSessions := &recordingSourceSessionManager{}
		manager, proc := startedManagerWithProcess(nil, "ext-stop-cleanup")
		manager.sourceSessions = sourceSessions

		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if !sourceSessions.hasReset(extensionResourceSource("ext-stop-cleanup")) {
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

		manager.disableExtension("ext-disable-cleanup", errors.New("too many failures"))

		if !sourceSessions.hasReset(extensionResourceSource("ext-disable-cleanup")) {
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
	mu     sync.Mutex
	resets []resources.ResourceSource
}

func (m *recordingSourceSessionManager) ActivateSourceSession(
	context.Context,
	resources.MutationActor,
	resources.ResourceSource,
	string,
) error {
	return nil
}

func (m *recordingSourceSessionManager) ResetSource(
	_ context.Context,
	_ resources.MutationActor,
	source resources.ResourceSource,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resets = append(m.resets, source.Normalize())
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

func (p *fakeProcess) shutdownCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shutdownCnt
}
