package builtin

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterBuiltins(t *testing.T) {
	t.Run("Should register provided builtin definitions", func(t *testing.T) {
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(ctx, config.NewDefaultProvider())
		require.NoError(t, err)
		ctx = config.ContextWithManager(ctx, manager)
		calls := 0
		registeredNames := make([]string, 0, 1)
		registerFn := func(_ context.Context, tool Tool) error {
			calls++
			registeredNames = append(registeredNames, tool.Name())
			return nil
		}
		definition := BuiltinDefinition{
			ID:          "cp__demo",
			Description: "demo builtin",
			Handler: func(_ context.Context, _ map[string]any) (core.Output, error) {
				return core.Output{"ok": true}, nil
			},
		}
		result, err := RegisterBuiltins(ctx, registerFn, Options{
			Definitions: []BuiltinDefinition{definition},
			ExtraExecCommands: []config.NativeExecCommandConfig{{
				Path: "/usr/bin/git",
			}},
		})
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
		assert.Equal(t, []string{"cp__demo"}, registeredNames)
		require.Len(t, result.ExecCommands, 1)
		assert.Equal(t, "/usr/bin/git", result.ExecCommands[0].Path)
		assert.Equal(t, []string{"cp__demo"}, result.RegisteredIDs)
	})

	t.Run("Should skip registration when disabled", func(t *testing.T) {
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(ctx, config.NewDefaultProvider())
		require.NoError(t, err)
		cfg := manager.Get()
		cfg.Runtime.NativeTools.Enabled = false
		ctx = config.ContextWithManager(ctx, manager)
		registerFn := func(_ context.Context, _ Tool) error {
			t.Fatalf("expected no registrations")
			return nil
		}
		result, err := RegisterBuiltins(ctx, registerFn, Options{})
		require.NoError(t, err)
		assert.Empty(t, result.RegisteredIDs)
	})

	t.Run("Should override enable flag via options", func(t *testing.T) {
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(ctx, config.NewDefaultProvider())
		require.NoError(t, err)
		cfg := manager.Get()
		cfg.Runtime.NativeTools.Enabled = false
		ctx = config.ContextWithManager(ctx, manager)
		calls := 0
		registerFn := func(_ context.Context, _ Tool) error {
			calls++
			return nil
		}
		definition := BuiltinDefinition{
			ID:          "cp__override",
			Description: "override",
			Handler: func(_ context.Context, _ map[string]any) (core.Output, error) {
				return core.Output{}, nil
			},
		}
		enable := true
		_, err = RegisterBuiltins(ctx, registerFn, Options{
			Definitions:    []BuiltinDefinition{definition},
			EnableOverride: &enable,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
	})

	t.Run("Should error for non cp prefix", func(t *testing.T) {
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		result, err := RegisterBuiltins(ctx, func(_ context.Context, _ Tool) error { return nil }, Options{
			Definitions: []BuiltinDefinition{{
				ID:      "read_file",
				Handler: func(context.Context, map[string]any) (core.Output, error) { return core.Output{}, nil },
			}},
		})
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should error on duplicate ids", func(t *testing.T) {
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		definition := BuiltinDefinition{
			ID:      "cp__dup",
			Handler: func(_ context.Context, _ map[string]any) (core.Output, error) { return core.Output{}, nil },
		}
		_, err := RegisterBuiltins(ctx, func(_ context.Context, _ Tool) error { return nil }, Options{
			Definitions: []BuiltinDefinition{definition, definition},
		})
		require.Error(t, err)
	})
}
