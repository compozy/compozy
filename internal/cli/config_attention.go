package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const configAttentionKey = "attention"

func attentionConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		"attention.toasts": configSetBool,
		"attention.sound":  configSetBool,
		"attention.system": configSetBool,
	}
}

func applyAttentionConfigValue(cfg *compozyconfig.AttentionConfig, path []string, value any) error {
	if cfg == nil || len(path) != 2 || path[0] != configAttentionKey {
		return fmt.Errorf("cli: unsupported attention config path %q", strings.Join(path, "."))
	}
	switch path[1] {
	case "toasts":
		return assignAttentionConfigValue(path, value, &cfg.Toasts, "boolean")
	case "sound":
		return assignAttentionConfigValue(path, value, &cfg.Sound, "boolean")
	case "system":
		return assignAttentionConfigValue(path, value, &cfg.System, "boolean")
	default:
		return fmt.Errorf("cli: unsupported attention config path %q", strings.Join(path, "."))
	}
}

func assignAttentionConfigValue[T any](path []string, value any, target *T, want string) error {
	typed, ok := value.(T)
	if !ok {
		return fmt.Errorf(
			"cli: config set %q expects %s, got %T",
			strings.Join(path, "."), want, value,
		)
	}
	*target = typed
	return nil
}

func settingsAttentionPayloadFromConfig(
	cfg compozyconfig.AttentionConfig,
) contract.UpdateSettingsAttentionPayload {
	return contract.UpdateSettingsAttentionPayload{
		Toasts: cfg.Toasts,
		Sound:  cfg.Sound,
		System: cfg.System,
	}
}
