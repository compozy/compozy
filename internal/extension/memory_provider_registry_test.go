package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestMemoryProviderRegistry(t *testing.T) {
	t.Run("Should register and select the local provider by default", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		provider := &stubMemoryProvider{}
		registry := NewMemoryProviderRegistry()
		if err := registry.Register(ctx, MemoryProviderRegistration{
			Name:          "LOCAL",
			Version:       "v1",
			ExtensionName: "builtin",
			Provider:      provider,
			Bundled:       true,
		}); err != nil {
			t.Fatalf("Register(local) error = %v", err)
		}
		registration, err := registry.Select(ctx, "ws-alpha", "")
		if err != nil {
			t.Fatalf("Select(default) error = %v", err)
		}
		if registration.Name != "local" {
			t.Fatalf("Select(default).Name = %q, want local", registration.Name)
		}
		if registration.Provider != provider {
			t.Fatal("Select(default).Provider mismatch")
		}
	})

	t.Run("Should reject provider name collisions and record observability", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
		writer := &recordingMemoryProviderEventWriter{}
		registry := NewMemoryProviderRegistry(
			WithMemoryProviderEventSummaryStore(writer),
			WithMemoryProviderRegistryClock(func() time.Time { return now }),
		)
		if err := registry.Register(ctx, MemoryProviderRegistration{
			Name:          "local",
			ExtensionName: "builtin",
			Provider:      &stubMemoryProvider{},
		}); err != nil {
			t.Fatalf("Register(first) error = %v", err)
		}

		err := registry.Register(ctx, MemoryProviderRegistration{
			Name:          "LOCAL",
			ExtensionName: "ext-memory",
			Provider:      &stubMemoryProvider{},
		})
		if !errors.Is(err, ErrMemoryProviderCollision) {
			t.Fatalf("Register(collision) error = %v, want ErrMemoryProviderCollision", err)
		}
		if got := len(writer.summaries); got != 1 {
			t.Fatalf("recorded summaries = %d, want 1", got)
		}
		summary := writer.summaries[0]
		if summary.Type != memoryProviderCollisionEvent {
			t.Fatalf("summary.Type = %q, want %q", summary.Type, memoryProviderCollisionEvent)
		}
		var payload memoryProviderCollisionPayload
		if err := json.Unmarshal(summary.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(summary.Content) error = %v", err)
		}
		if payload.Provider != "local" || payload.Reason != memoryProviderNameCollision {
			t.Fatalf("collision payload = %#v, want local provider-name collision", payload)
		}
	})

	t.Run("Should keep active selection stable after a rejected collision", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		localProvider := &stubMemoryProvider{}
		registry := NewMemoryProviderRegistry()
		if err := registry.Register(ctx, MemoryProviderRegistration{
			Name:     "local",
			Provider: localProvider,
		}); err != nil {
			t.Fatalf("Register(local) error = %v", err)
		}
		if err := registry.SetActive(ctx, "ws-alpha", "local"); err != nil {
			t.Fatalf("SetActive(local) error = %v", err)
		}
		err := registry.Register(ctx, MemoryProviderRegistration{
			Name:     "local",
			Provider: &stubMemoryProvider{},
		})
		if !errors.Is(err, ErrMemoryProviderCollision) {
			t.Fatalf("Register(collision) error = %v, want ErrMemoryProviderCollision", err)
		}
		registration, err := registry.Select(ctx, "ws-alpha", "")
		if err != nil {
			t.Fatalf("Select(active) error = %v", err)
		}
		if registration.Provider != localProvider {
			t.Fatal("Select(active).Provider changed after collision")
		}
	})

	t.Run("Should reject tool name collisions deterministically", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		registry := NewMemoryProviderRegistry(WithMemoryProviderReservedTools("agh__memory_search"))
		err := registry.Register(ctx, MemoryProviderRegistration{
			Name:      "custom",
			Provider:  &stubMemoryProvider{},
			ToolNames: []string{"AGH__MEMORY_SEARCH"},
		})
		if !errors.Is(err, ErrMemoryProviderCollision) {
			t.Fatalf("Register(tool collision) error = %v, want ErrMemoryProviderCollision", err)
		}
	})

	t.Run("Should return not found for unknown provider selection", func(t *testing.T) {
		t.Parallel()

		_, err := NewMemoryProviderRegistry().Select(testutil.Context(t), "ws-alpha", "missing")
		if !errors.Is(err, ErrMemoryProviderNotFound) {
			t.Fatalf("Select(missing) error = %v, want ErrMemoryProviderNotFound", err)
		}
	})
}

func TestHostAPIHandlerMemoryProviderRegistryOption(t *testing.T) {
	t.Run("Should attach memory provider registry to Host API handler", func(t *testing.T) {
		t.Parallel()

		registry := NewMemoryProviderRegistry()
		handler := NewHostAPIHandler(nil, nil, nil, nil, WithHostAPIMemoryProviderRegistry(registry))
		if handler.memoryProviders != registry {
			t.Fatal("HostAPIHandler.memoryProviders mismatch")
		}
	})
}

func TestMemoryProviderCollisionEventSummaryValidation(t *testing.T) {
	t.Run("Should allow provider collision as global observability", func(t *testing.T) {
		t.Parallel()

		if err := (store.EventSummary{Type: memoryProviderCollisionEvent}).Validate(); err != nil {
			t.Fatalf("EventSummary.Validate(provider collision) error = %v", err)
		}
	})
}

type recordingMemoryProviderEventWriter struct {
	summaries []store.EventSummary
}

func (w *recordingMemoryProviderEventWriter) WriteEventSummary(
	_ context.Context,
	summary store.EventSummary,
) error {
	w.summaries = append(w.summaries, summary)
	return nil
}

type stubMemoryProvider struct{}

func (p *stubMemoryProvider) Initialize(context.Context, memcontract.ProviderInit) error {
	return nil
}

func (p *stubMemoryProvider) SystemPromptBlock(
	context.Context,
	memcontract.SnapshotRequest,
) (memcontract.SnapshotResult, error) {
	return memcontract.SnapshotResult{}, nil
}

func (p *stubMemoryProvider) Recall(
	context.Context,
	memcontract.RecallRequest,
) (memcontract.RecallResult, error) {
	return memcontract.RecallResult{}, nil
}

func (p *stubMemoryProvider) Prefetch(context.Context, memcontract.PrefetchRequest) error {
	return nil
}

func (p *stubMemoryProvider) SyncTurn(context.Context, memcontract.TurnRecord) error {
	return nil
}

func (p *stubMemoryProvider) OnSessionEnd(context.Context, memcontract.SessionEndRecord) error {
	return nil
}

func (p *stubMemoryProvider) OnSessionSwitch(context.Context, memcontract.SessionSwitchRecord) error {
	return nil
}

func (p *stubMemoryProvider) OnPreCompress(
	context.Context,
	memcontract.PreCompressRequest,
) (memcontract.PreCompressHint, error) {
	return memcontract.PreCompressHint{}, nil
}

func (p *stubMemoryProvider) OnMemoryWrite(context.Context, memcontract.WriteRecord) error {
	return nil
}

func (p *stubMemoryProvider) Shutdown(context.Context) error {
	return nil
}
