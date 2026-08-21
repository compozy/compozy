package config

import (
	"fmt"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

// LoadHookDeclarationsFile reads config-backed hooks from one concrete overlay
// without merging declarations from any other config layer.
func LoadHookDeclarationsFile(path string) ([]hookspkg.HookDecl, error) {
	overlay, err := loadConfigOverlayFile(path)
	if err != nil {
		return nil, err
	}
	declarations := make([]hookspkg.HookDecl, 0, len(overlay.Hooks.Declarations))
	for index := range overlay.Hooks.Declarations {
		decl, err := overlay.Hooks.Declarations[index].toHookDecl(hookspkg.HookSourceConfig, "")
		if err != nil {
			return nil, fmt.Errorf("hooks.declarations[%d]: %w", index, err)
		}
		declarations = append(declarations, decl)
	}
	return declarations, nil
}
