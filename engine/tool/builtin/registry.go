package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// RegisterFunc adapts a registry Register method for builtin registration.
type RegisterFunc func(ctx context.Context, tool Tool) error

// Result exposes metadata captured during registration.
type Result struct {
	RegisteredIDs []string
	ExecCommands  []config.NativeExecCommandConfig
}

// RegisterBuiltins wires builtin definitions into the provided registry.
func RegisterBuiltins(ctx context.Context, register RegisterFunc, opts Options) (*Result, error) {
	log := logger.FromContext(ctx)
	nativeCfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		nativeCfg = appCfg.Runtime.NativeTools
	}
	if opts.EnableOverride != nil {
		nativeCfg.Enabled = *opts.EnableOverride
	}
	effectiveAllowlist := opts.execAllowlistMerge(nativeCfg.Exec.Allowlist)
	if !nativeCfg.Enabled {
		log.Warn("Native builtin tools disabled by configuration")
		return &Result{ExecCommands: effectiveAllowlist}, nil
	}
	definitions := opts.definitions()
	if len(definitions) == 0 {
		return nil, fmt.Errorf("builtin registration requires at least one definition")
	}
	seen := make(map[string]struct{}, len(definitions))
	registered := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(definition.ID, "cp__") {
			return nil, fmt.Errorf("builtin id %s must use cp__ prefix", definition.ID)
		}
		if _, exists := seen[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate builtin id %s", definition.ID)
		}
		seen[definition.ID] = struct{}{}
		tool, err := NewBuiltinTool(definition)
		if err != nil {
			return nil, err
		}
		if err := register(ctx, tool); err != nil {
			return nil, fmt.Errorf("failed to register builtin %s: %w", definition.ID, err)
		}
		registered = append(registered, definition.ID)
	}
	log.Debug("Registered native builtin tools", "count", len(registered))
	return &Result{RegisteredIDs: registered, ExecCommands: effectiveAllowlist}, nil
}
