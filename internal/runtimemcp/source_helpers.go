package runtimemcp

import (
	"cmp"
	"os"
	"path/filepath"
)

func claudeSources(c ReadContext) []source {
	home, cwd := c.Home, c.CWD
	var list []source
	configHome := c.homeDir("CLAUDE_CONFIG_DIR", ".claude")
	global := filepath.Join(home, ".claude.json")
	if c.Env["CLAUDE_CONFIG_DIR"] != "" {
		global = filepath.Join(configHome, ".claude.json")
	}
	if _, err := os.Lstat(filepath.Join(configHome, ".config.json")); err == nil {
		global = filepath.Join(configHome, ".config.json")
	}
	list = append(list, source{global, scopeUser})
	for _, dir := range ancestorDirs(cwd, "") {
		list = append(list, source{filepath.Join(dir, ".mcp.json"), scopeProject})
	}
	return list
}

func codexSources(c ReadContext) []source {
	var list []source
	codexHome := c.homeDir("CODEX_HOME", ".codex")
	list = append(list,
		source{filepath.Join(c.systemDir(runtimeCodex), "config.toml"), scopeManaged},
		source{filepath.Join(codexHome, "config.toml"), scopeUser},
	)
	if validRuntimeProfile(c.RuntimeProfile) {
		list = append(list, source{filepath.Join(codexHome, c.RuntimeProfile+".config.toml"), scopeUser})
	}
	for _, dir := range ancestorDirs(c.CWD, cmp.Or(c.WorktreeRoot, c.GitRoot)) {
		list = append(list, source{filepath.Join(dir, ".codex", "config.toml"), scopeProject})
	}
	return list
}

func opencodeSources(c ReadContext) []source {
	home, cwd := c.Home, c.CWD
	user := func(path string) source { return source{path, scopeUser} }
	project := func(path string) source { return source{path, scopeProject} }
	var list []source
	root := cmp.Or(c.Env["XDG_CONFIG_HOME"], filepath.Join(home, ".config"))
	configDir := c.homeDir("OPENCODE_CONFIG_DIR", filepath.Join(root, runtimeOpenCode))
	for _, ext := range []string{"json", "jsonc"} {
		list = append(list, user(filepath.Join(configDir, "opencode."+ext)))
	}
	for _, dir := range ancestorDirs(cwd, "") {
		files := []string{"opencode.json", "opencode.jsonc", ".opencode/opencode.json", ".opencode/opencode.jsonc"}
		for _, name := range files {
			list = append(list, project(filepath.Join(dir, name)))
		}
	}
	for _, ext := range []string{"json", "jsonc"} {
		list = append(list, source{filepath.Join(c.systemDir(runtimeOpenCode), "opencode."+ext), scopeManaged})
	}
	return list
}
