package commands

import (
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
)

func runtimeConfigFromCore(cfg core.Config) model.RuntimeConfig {
	return *cfg.RuntimeConfig()
}

func cloneRuntimeConfig(cfg model.RuntimeConfig) *model.RuntimeConfig {
	cloned := cfg
	if len(cfg.AddDirs) > 0 {
		cloned.AddDirs = append([]string(nil), cfg.AddDirs...)
	}
	return &cloned
}
