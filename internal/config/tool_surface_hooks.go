package config

import (
	"fmt"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

// OverlayHookDeclarations returns config-backed hook declarations from one overlay target.
func OverlayHookDeclarations(target WriteTarget) ([]hookspkg.HookDecl, error) {
	if !target.isConfigTarget() {
		return nil, fmt.Errorf("config: write target %q is not a config overlay", target.Kind())
	}
	overlay, err := loadConfigOverlayFile(target.path)
	if err != nil {
		return nil, err
	}
	decls := make([]hookspkg.HookDecl, 0, len(overlay.Hooks.Declarations))
	for idx := range overlay.Hooks.Declarations {
		raw := &overlay.Hooks.Declarations[idx]
		decl, err := raw.toHookDecl(hookspkg.HookSourceConfig, "")
		if err != nil {
			return nil, fmt.Errorf("hooks.declarations[%d]: %w", idx, err)
		}
		decls = append(decls, decl)
	}
	return decls, nil
}

// HookDeclarationOverlayValues converts a hook declaration to TOML overlay values.
func HookDeclarationOverlayValues(decl hookspkg.HookDecl) map[string]any {
	values := map[string]any{
		toolSurfaceEventKey: string(decl.Event),
	}
	if decl.Enabled != nil {
		values[string(ToolsExternalDefaultEnabled)] = *decl.Enabled
	}
	if decl.Mode != "" {
		values["mode"] = string(decl.Mode)
	}
	if decl.Required {
		values["required"] = true
	}
	if decl.PrioritySet || decl.Priority != 0 {
		values["priority"] = decl.Priority
	}
	if decl.Timeout > 0 {
		values["timeout"] = decl.Timeout.String()
	}
	if matcher := hookMatcherOverlayValues(decl.Matcher); len(matcher) > 0 {
		values["matcher"] = matcher
	}
	if decl.Command != "" {
		values[toolSurfaceCommandKey] = decl.Command
	}
	if len(decl.Args) > 0 {
		values["args"] = append([]string(nil), decl.Args...)
	}
	if len(decl.Env) > 0 {
		values["env"] = mergeStringMaps(nil, decl.Env)
	}
	if decl.ExecutorKind != "" && decl.ExecutorKind != hookspkg.HookExecutorSubprocess {
		values["executor"] = map[string]any{"kind": string(decl.ExecutorKind)}
	}
	return values
}
