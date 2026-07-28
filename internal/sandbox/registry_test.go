package sandbox

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryProviderReturnsRegisteredProvider(t *testing.T) {
	t.Parallel()

	want := registryTestProvider{backend: BackendLocal}
	registry, err := NewRegistry(want)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got, err := registry.Provider(BackendLocal)
	if err != nil {
		t.Fatalf("Provider(%q) error = %v", BackendLocal, err)
	}
	if got.Backend() != BackendLocal {
		t.Fatalf("Provider(%q).Backend() = %q, want %q", BackendLocal, got.Backend(), BackendLocal)
	}

	defaultProvider, err := registry.DefaultProvider()
	if err != nil {
		t.Fatalf("DefaultProvider() error = %v", err)
	}
	if defaultProvider.Backend() != DefaultBackend {
		t.Fatalf("DefaultProvider().Backend() = %q, want %q", defaultProvider.Backend(), DefaultBackend)
	}
}

func TestRegistryProviderReturnsErrorForUnregisteredBackend(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(registryTestProvider{backend: BackendLocal})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, err = registry.Provider(BackendDaytona)
	if !errors.Is(err, ErrProviderNotRegistered) {
		t.Fatalf("Provider(%q) error = %v, want ErrProviderNotRegistered", BackendDaytona, err)
	}

	var nilRegistry *Registry
	_, err = nilRegistry.Provider(BackendLocal)
	if !errors.Is(err, ErrProviderNotRegistered) {
		t.Fatalf("nil Registry.Provider() error = %v, want ErrProviderNotRegistered", err)
	}
}

func TestRegistryRejectsInvalidProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider Provider
		wantErr  error
	}{
		{name: "nil provider", provider: nil, wantErr: ErrNilProvider},
		{
			name:     "invalid backend",
			provider: registryTestProvider{backend: Backend("docker")},
			wantErr:  ErrInvalidProviderBackend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRegistry(tt.provider)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewRegistry() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistryRegisterInitializesEmptyRegistry(t *testing.T) {
	t.Parallel()

	var registry Registry
	if err := registry.Register(registryTestProvider{backend: BackendLocal}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	provider, err := registry.Provider(BackendLocal)
	if err != nil {
		t.Fatalf("Provider(%q) error = %v", BackendLocal, err)
	}
	if provider.Backend() != BackendLocal {
		t.Fatalf("Provider(%q).Backend() = %q, want %q", BackendLocal, provider.Backend(), BackendLocal)
	}
}

func TestRegistryProvidersReturnsSnapshot(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(registryTestProvider{backend: BackendLocal})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	snapshot := registry.Providers()
	if len(snapshot) != 1 {
		t.Fatalf("Providers() length = %d, want 1", len(snapshot))
	}
	snapshot[BackendLocal] = nil

	provider, err := registry.Provider(BackendLocal)
	if err != nil {
		t.Fatalf("Provider(%q) after snapshot mutation error = %v", BackendLocal, err)
	}
	if provider == nil {
		t.Fatal("Provider() = nil after snapshot mutation, want registry to remain unchanged")
	}

	var nilRegistry *Registry
	if got := nilRegistry.Providers(); len(got) != 0 {
		t.Fatalf("nil Registry.Providers() length = %d, want 0", len(got))
	}
}

type registryTestProvider struct {
	backend Backend
}

func (p registryTestProvider) Backend() Backend {
	return p.backend
}

func (p registryTestProvider) Prepare(context.Context, PrepareRequest) (Prepared, error) {
	return Prepared{}, nil
}

func (p registryTestProvider) SyncToRuntime(context.Context, SessionState, SyncOptions) (SyncResult, error) {
	return SyncResult{}, nil
}

func (p registryTestProvider) SyncFromRuntime(context.Context, SessionState, SyncOptions) (SyncResult, error) {
	return SyncResult{}, nil
}

func (p registryTestProvider) Destroy(context.Context, SessionState) error {
	return nil
}
