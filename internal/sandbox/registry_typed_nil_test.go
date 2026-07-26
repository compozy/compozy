package sandbox

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryRejectsTypedNilProvidersContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject typed nil providers before reading their backend", func(t *testing.T) {
		t.Parallel()

		var provider *typedNilRegistryProvider
		_, err := NewRegistry(provider)
		if !errors.Is(err, ErrNilProvider) {
			t.Fatalf("NewRegistry(typed nil) error = %v, want ErrNilProvider", err)
		}
	})

	t.Run("Should not return typed nil providers from a mutated registry", func(t *testing.T) {
		t.Parallel()

		var provider *typedNilRegistryProvider
		registry := &Registry{providers: map[Backend]Provider{
			BackendLocal: provider,
		}}
		_, err := registry.Provider(BackendLocal)
		if !errors.Is(err, ErrProviderNotRegistered) {
			t.Fatalf("Provider(typed nil) error = %v, want ErrProviderNotRegistered", err)
		}
	})
}

type typedNilRegistryProvider struct{}

func (*typedNilRegistryProvider) Backend() Backend {
	return BackendLocal
}

func (*typedNilRegistryProvider) Prepare(context.Context, PrepareRequest) (Prepared, error) {
	return Prepared{}, nil
}

func (*typedNilRegistryProvider) SyncToRuntime(context.Context, SessionState, SyncOptions) (SyncResult, error) {
	return SyncResult{}, nil
}

func (*typedNilRegistryProvider) SyncFromRuntime(context.Context, SessionState, SyncOptions) (SyncResult, error) {
	return SyncResult{}, nil
}

func (*typedNilRegistryProvider) Destroy(context.Context, SessionState) error {
	return nil
}
