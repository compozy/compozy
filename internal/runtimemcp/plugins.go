package runtimemcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func (r *reader) contained(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (r *reader) claudePlugins(settings object, entries map[string]entry) {
	root := r.context.homeDir("CLAUDE_CONFIG_DIR", ".claude")
	value, ok := r.load(filepath.Join(root, "plugins", "installed_plugins.json"))
	if !ok {
		return
	}
	for key, items := range obj(obj(value)["plugins"]) {
		if !boolean(obj(settings["enabledPlugins"])[key]) {
			continue
		}
		installs, ok := items.([]any)
		if !ok {
			continue
		}
		for _, install := range installs {
			item := obj(install)
			path := str(item["installPath"])
			if !r.contained(filepath.Join(root, "plugins"), path) {
				continue
			}
			if project := str(item["projectPath"]); project != "" && !r.contained(project, r.context.CWD) {
				continue
			}
			plugin, _, _ := strings.Cut(key, "@")
			r.pluginServers(runtimeClaude, plugin, path, entries)
		}
	}
}

func (r *reader) codexPlugins(doc object, entries map[string]entry) {
	root := r.context.homeDir("CODEX_HOME", ".codex")
	for key, value := range obj(doc["plugins"]) {
		if !boolean(obj(value)["enabled"]) {
			continue
		}
		plugin, market, _ := strings.Cut(key, "@")
		if fileutil.ValidatePathComponent(plugin) != nil || fileutil.ValidatePathComponent(market) != nil {
			continue
		}
		cache := filepath.Join(root, "plugins", "cache", market, plugin)
		// The cache root is observed so installing a new version invalidates existing contexts.
		r.paths[cache] = struct{}{}
		dir, err := fileutil.OpenDirectory(cache)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			r.failure = "Couldn't read " + r.label(cache) + ": plugin cache unavailable"
			continue
		}
		versions, readErr := dir.ReadDir()
		closeErr := dir.Close()
		if readErr != nil || closeErr != nil {
			r.failure = "Couldn't read " + r.label(cache) + ": plugin cache unavailable"
			continue
		}
		if version := activeCodexPluginVersion(cache, versions); version != "" {
			r.pluginServers(runtimeCodex, plugin, filepath.Join(cache, version), entries)
		}
	}
}

func (r *reader) pluginServers(runtime, plugin, root string, entries map[string]entry) {
	paths := []string{
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, ".claude-plugin", "plugin.json"),
		filepath.Join(root, ".codex-plugin", "plugin.json"),
	}
	for _, path := range paths {
		value, ok := r.load(path)
		if !ok {
			continue
		}
		doc := obj(value)
		if doc == nil {
			r.failure = "Couldn't read " + r.label(path) + ": invalid plugin configuration"
			continue
		}
		declaration := doc["mcpServers"]
		if filepath.Base(path) == ".mcp.json" && declaration == nil {
			declaration = doc
		}
		declarations, array := declaration.([]any)
		if !array {
			declarations = []any{declaration}
		}
		for _, declaration := range declarations {
			r.pluginDeclaration(runtime, plugin, root, path, declaration, entries)
		}
	}
}

func (r *reader) pluginDeclaration(runtime, plugin, root, path string, declaration any, entries map[string]entry) {
	if declaration == nil {
		return
	}
	definitions := obj(declaration)
	if external, file := declaration.(string); file {
		resolved := filepath.Join(root, external)
		if !r.contained(root, resolved) {
			r.failure = "Couldn't read " + r.label(path) + ": plugin config escapes its root"
			return
		}
		value, ok := r.load(resolved)
		if !ok {
			r.failure = "Couldn't read " + r.label(path) + ": plugin MCP file unavailable"
			return
		}
		path, definitions = resolved, obj(value)
		if nested := obj(definitions["mcpServers"]); nested != nil {
			definitions = nested
		}
	}
	if definitions == nil {
		r.failure = "Couldn't read " + r.label(path) + ": unsupported plugin MCP declaration"
		return
	}
	for name, definition := range definitions {
		if runtime == runtimeClaude {
			name = "plugin:" + plugin + ":" + name
		}
		if _, exists := entries[name]; exists && runtime == runtimeCodex {
			continue
		}
		r.add(runtime, source{path, scopePlugin}, object{name: definition}, entries)
	}
}
