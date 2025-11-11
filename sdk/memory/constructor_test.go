package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	memorycore "github.com/compozy/compozy/engine/memory/core"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
)

func TestNew(t *testing.T) {
	t.Run("Should create memory with minimal redis configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test-memory", "token_based",
			WithMaxTokens(1000),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.ID != "test-memory" {
			t.Errorf("expected id 'test-memory', got '%s'", cfg.ID)
		}
		if cfg.Type != memorycore.TokenBasedMemory {
			t.Errorf("expected type 'token_based', got '%s'", cfg.Type)
		}
		if cfg.Resource != string(core.ConfigMemory) {
			t.Errorf("expected resource 'memory', got '%s'", cfg.Resource)
		}
	})
	t.Run("Should create memory with in_memory persistence", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test-memory", "buffer",
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.InMemoryPersistence,
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Persistence.Type != memorycore.InMemoryPersistence {
			t.Errorf("expected in_memory persistence, got '%s'", cfg.Persistence.Type)
		}
	})
	t.Run("Should trim whitespace from id and type", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "  test-memory  ", "  token_based  ",
			WithMaxTokens(1000),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ID != "test-memory" {
			t.Errorf("expected trimmed id 'test-memory', got '%s'", cfg.ID)
		}
		if cfg.Type != memorycore.TokenBasedMemory {
			t.Errorf("expected type 'token_based', got '%s'", cfg.Type)
		}
	})
	t.Run("Should normalize type to lowercase", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test-memory", "TOKEN_BASED",
			WithMaxTokens(1000),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Type != memorycore.TokenBasedMemory {
			t.Errorf("expected type 'token_based', got '%s'", cfg.Type)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		var nilCtx context.Context
		_, err := New(nilCtx, "test-memory", "token_based")
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when id is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "", "token_based",
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err == nil {
			t.Fatal("expected error for empty id")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when type is invalid", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "invalid-type",
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err == nil {
			t.Fatal("expected error for invalid type")
		}
	})
	t.Run("Should fail when persistence type is missing", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "token_based",
			WithMaxTokens(1000),
		)
		if err == nil {
			t.Fatal("expected error for missing persistence type")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when redis persistence has no ttl", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "token_based",
			WithMaxTokens(1000),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
			}),
		)
		if err == nil {
			t.Fatal("expected error for missing TTL with redis")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when token_based has no limits", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "token_based",
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.InMemoryPersistence,
			}),
		)
		if err == nil {
			t.Fatal("expected error for token_based without limits")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should create memory with all options", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test-memory", "token_based",
			WithDescription("Test memory"),
			WithVersion("1.0.0"),
			WithMaxTokens(4000),
			WithMaxMessages(100),
			WithExpiration("48h"),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Description != "Test memory" {
			t.Errorf("expected description 'Test memory', got '%s'", cfg.Description)
		}
		if cfg.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got '%s'", cfg.Version)
		}
		if cfg.MaxTokens != 4000 {
			t.Errorf("expected max_tokens 4000, got %d", cfg.MaxTokens)
		}
		if cfg.MaxMessages != 100 {
			t.Errorf("expected max_messages 100, got %d", cfg.MaxMessages)
		}
		if cfg.Expiration != "48h" {
			t.Errorf("expected expiration '48h', got '%s'", cfg.Expiration)
		}
	})
	t.Run("Should support all memory types", func(t *testing.T) {
		ctx := context.Background()
		types := []struct {
			name     string
			expected memorycore.Type
		}{
			{"token_based", memorycore.TokenBasedMemory},
			{"message_count_based", memorycore.MessageCountBasedMemory},
			{"buffer", memorycore.BufferMemory},
		}
		for _, tt := range types {
			opts := []Option{
				WithPersistence(memorycore.PersistenceConfig{
					Type: memorycore.InMemoryPersistence,
				}),
			}
			if tt.name == "token_based" {
				opts = append(opts, WithMaxTokens(1000))
			}
			cfg, err := New(ctx, "test-memory", tt.name, opts...)
			if err != nil {
				t.Errorf("type %s: unexpected error: %v", tt.name, err)
				continue
			}
			if cfg.Type != tt.expected {
				t.Errorf("type %s: expected %s, got %s", tt.name, tt.expected, cfg.Type)
			}
		}
	})
	t.Run("Should validate negative max_tokens", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "token_based",
			WithMaxTokens(-1),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.InMemoryPersistence,
			}),
		)
		if err == nil {
			t.Fatal("expected error for negative max_tokens")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate negative max_messages", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "buffer",
			WithMaxMessages(-1),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.InMemoryPersistence,
			}),
		)
		if err == nil {
			t.Fatal("expected error for negative max_messages")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate max_context_ratio range", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "token_based",
			WithMaxTokens(1000),
			WithMaxContextRatio(1.5),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.InMemoryPersistence,
			}),
		)
		if err == nil {
			t.Fatal("expected error for invalid max_context_ratio")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate expiration duration format", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-memory", "buffer",
			WithExpiration("invalid-duration"),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.InMemoryPersistence,
			}),
		)
		if err == nil {
			t.Fatal("expected error for invalid expiration")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should perform deep copy", func(t *testing.T) {
		ctx := context.Background()
		cfg1, err := New(ctx, "test-memory", "token_based",
			WithMaxTokens(1000),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg1.Description = "modified"
		cfg2, err := New(ctx, "test-memory", "token_based",
			WithMaxTokens(1000),
			WithPersistence(memorycore.PersistenceConfig{
				Type: memorycore.RedisPersistence,
				TTL:  "24h",
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg2.Description == "modified" {
			t.Error("deep copy failed: configuration was modified")
		}
	})
	t.Run("Should accumulate multiple validation errors", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "", "token_based",
			WithMaxTokens(-1),
			WithMaxContextRatio(2.0),
		)
		if err == nil {
			t.Fatal("expected error for multiple validation failures")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Fatalf("expected BuildError, got %T", err)
		}
		if len(buildErr.Errors) < 2 {
			t.Errorf("expected multiple errors, got %d", len(buildErr.Errors))
		}
	})
}
