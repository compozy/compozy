package config

import (
	core "github.com/compozy/compozy/engine/core"
)

// SugarDBBaseDir returns the base directory for embedded SugarDB persistence.
// Priority:
//  1. Explicit cfg.SugarDB.DBPath when set
//  2. <project_cwd>/.compozy derived from cfg.CLI.CWD via core.GetStoreDir
func SugarDBBaseDir(cfg *Config) string {
	if cfg != nil && cfg.SugarDB.DBPath != "" {
		return cfg.SugarDB.DBPath
	}
	var cwd string
	if cfg != nil {
		cwd = cfg.CLI.CWD
	}
	return core.GetStoreDir(cwd)
}
