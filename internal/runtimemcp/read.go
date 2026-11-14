package runtimemcp

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type entry struct {
	server     Server
	definition object
}

func read(c ReadContext, descriptor Descriptor) (Snapshot, []string) {
	snapshot, paths, _ := readDefinitions(c, descriptor)
	return snapshot, paths
}

func readDefinitions(c ReadContext, descriptor Descriptor) (Snapshot, []string, map[string]entry) {
	r := reader{context: c, paths: map[string]struct{}{}}
	snapshot := Snapshot{Runtime: descriptor.Runtime, DisplayName: descriptor.DisplayName, Installed: installed(c, descriptor),
		RuntimeMCP: RuntimeMode{Mode: "load", CanDisable: descriptor.CanDisableRuntimeMCP}, ReadState: "ok", ConfigFiles: []string{}, Servers: []Server{}}
	entries := map[string]entry{}
	if descriptor.Runtime == "claude" {
		r.readClaude(entries, &snapshot)
	} else {
		r.readSources(descriptor.Runtime, entries)
	}
	if descriptor.Runtime == "gemini" {
		r.applyGeminiEnablement(entries)
	}
	if descriptor.Runtime == "cursor" {
		r.applyCursorApprovals(entries)
	}
	snapshot.ConfigFiles = slices.Clone(r.files)
	for _, name := range sortedNames(entries) {
		server := entries[name].server
		server.ActivationDigest = digest(struct {
			Definition, Source, Scope, Approval string
			Enabled                             bool
		}{server.DefinitionDigest, server.SourcePath, server.Scope, server.Approval, server.EnabledInRuntime})
		snapshot.Servers = append(snapshot.Servers, server)
	}
	snapshot.ActivationDigest = digest(struct {
		Servers    []Server
		CanDisable bool
	}{snapshot.Servers, snapshot.RuntimeMCP.CanDisable})
	if r.failure != "" {
		snapshot.ReadState = "unreadable"
		snapshot.ReadError = r.failure
		snapshot.Servers = []Server{}
		snapshot.ActivationDigest = ""
	}
	paths := make([]string, 0, len(r.paths))
	for path := range r.paths {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return snapshot, paths, entries
}

func sortedNames(entries map[string]entry) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (r *reader) add(runtime string, src source, definitions object, entries map[string]entry) {
	for name, value := range definitions {
		definition := obj(value)
		if definition == nil {
			r.failure = "Couldn't read " + r.label(src.path) + ": invalid MCP server definition"
			continue
		}
		if old, ok := entries[name]; ok && old.server.Scope != "plugin" {
			if runtime == "codex" || (runtime == "pi" && !has(definition, "command") && !has(definition, "url") && !has(definition, "type")) {
				definition = mergeDefinition(old.definition, definition)
			}
		}
		server, err := describeServer(runtime, name, src.scope, src.path, r.label(src.path), definition)
		if err != nil {
			r.failure = "Couldn't read " + r.label(src.path) + ": invalid MCP server definition"
			continue
		}
		entries[name] = entry{server, definition}
	}
}

// Codex layers merge TOML tables recursively; arrays and scalar values replace.
func mergeDefinition(base, overlay object) object {
	merged := object{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overlay {
		if nested := obj(value); nested != nil && obj(merged[key]) != nil {
			value = mergeDefinition(obj(merged[key]), nested)
		}
		merged[key] = value
	}
	return merged
}

func validRuntimeProfile(profile string) bool {
	return profile != "" && profile != "." && profile != ".." && !strings.ContainsAny(profile, "/\\")
}

func (r *reader) readSources(runtime string, entries map[string]entry) {
	policy := object{}
	srcs := sources(r.context, runtime)
	for index := 0; index < len(srcs); index++ {
		src := srcs[index]
		value, ok := r.load(src.path)
		if !ok {
			continue
		}
		doc, err := checkedObject(value)
		if err != nil {
			r.failure = "Couldn't read " + r.label(src.path) + ": invalid configuration shape"
			continue
		}
		definitions, err := decodeServers(runtime, doc)
		if err != nil {
			r.failure = "Couldn't read " + r.label(src.path) + ": invalid MCP configuration shape"
			continue
		}
		r.add(runtime, src, definitions, entries)
		for key, value := range obj(doc["mcp"]) {
			if key == "allowed" || key == "excluded" {
				policy[key] = value
			}
		}
		if runtime == "codex" && src.scope == "user" {
			r.codexPlugins(doc, entries)
			profile := str(doc["profile"])
			if r.context.RuntimeProfile == "" && filepath.Base(src.path) == "config.toml" && validRuntimeProfile(profile) {
				path := filepath.Join(filepath.Dir(src.path), profile+".config.toml")
				srcs = slices.Insert(srcs, index+1, source{path, "user"})
			}
		}
	}
	r.applyMCPPolicy(runtime, policy, entries)
}

func (r *reader) readClaude(entries map[string]entry, snapshot *Snapshot) {
	snapshot.Notes = []string{"claude.ai connectors appear inside sessions"}
	managed := filepath.Join(r.context.systemDir("ClaudeCode"), "managed-mcp.json")
	if value, ok := r.load(managed); ok {
		snapshot.RuntimeMCP.CanDisable = false
		snapshot.Notes = append(snapshot.Notes, "Claude Code's managed-mcp.json controls its MCP servers; the runtime switch is unavailable")
		definitions, err := decodeServers("claude", obj(value))
		if err != nil {
			r.failure = "Couldn't read " + r.label(managed) + ": invalid MCP configuration shape"
			return
		}
		r.add("claude", source{managed, "managed"}, definitions, entries)
		return
	}
	// An unreadable managed file is still an exclusive machine policy.
	if _, err := os.Lstat(managed); err == nil {
		snapshot.RuntimeMCP.CanDisable = false
		return
	}
	srcs := sources(r.context, "claude")
	settings := object{}
	paths := []string{filepath.Join(r.context.homeDir("CLAUDE_CONFIG_DIR", ".claude"), "settings.json")}
	if r.context.CWD != "" {
		paths = append(paths, filepath.Join(r.context.CWD, ".claude/settings.json"), filepath.Join(r.context.CWD, ".claude/settings.local.json"))
	}
	paths = append(paths, filepath.Join(r.context.systemDir("ClaudeCode"), "managed-settings.json"))
	for _, path := range paths {
		if value, ok := r.load(path); ok {
			for k, v := range obj(value) {
				settings[k] = v
			}
		}
	}
	r.claudePlugins(settings, entries)
	var project object
	for _, src := range srcs {
		value, ok := r.load(src.path)
		if !ok {
			continue
		}
		doc := obj(value)
		if doc == nil {
			r.failure = "Couldn't read " + r.label(src.path) + ": invalid configuration shape"
			continue
		}
		definitions, err := decodeServers("claude", doc)
		if err != nil {
			r.failure = "Couldn't read " + r.label(src.path) + ": invalid MCP configuration shape"
			continue
		}
		r.add("claude", src, definitions, entries)
		if src.scope == "user" {
			project = obj(obj(doc["projects"])[cmp.Or(r.context.GitRoot, r.context.CWD)])
		}
	}
	if project != nil {
		r.add("claude", source{srcs[0].path, "local"}, obj(project["mcpServers"]), entries)
	}
	applyClaudeApprovals(project, settings, entries)
}

func applyClaudeApprovals(project, settings object, entries map[string]entry) {
	for name, item := range entries {
		if contains(project["disabledMcpServers"], name) {
			item.server.EnabledInRuntime = false
		}
		if item.server.Scope == "project" {
			item.server.Approval = "pending"
			if boolean(settings["enableAllProjectMcpServers"]) || contains(settings["enabledMcpjsonServers"], name) || contains(project["enabledMcpjsonServers"], name) {
				item.server.Approval = "approved"
			}
			if contains(settings["disabledMcpjsonServers"], name) || contains(project["disabledMcpjsonServers"], name) {
				item.server.Approval = "rejected"
				item.server.EnabledInRuntime = false
			}
		}
		entries[name] = item
	}
	applyClaudePolicy(settings, entries)
}

func (r *reader) applyGeminiEnablement(entries map[string]entry) {
	value, ok := r.load(filepath.Join(r.context.Home, ".gemini/mcp-server-enablement.json"))
	if !ok {
		return
	}
	doc := obj(value)
	if servers := obj(doc["servers"]); servers != nil {
		doc = servers
	}
	for name, item := range entries {
		v := doc[name]
		if enabled, ok := v.(bool); ok {
			item.server.EnabledInRuntime = item.server.EnabledInRuntime && enabled
		}
		if state := obj(v); state != nil {
			if enabled, ok := state["enabled"].(bool); ok {
				item.server.EnabledInRuntime = item.server.EnabledInRuntime && enabled
			}
		}
		entries[name] = item
	}
}

func installed(c ReadContext, d Descriptor) bool {
	roots := filepath.SplitList(c.Env["PATH"])
	roots = append(roots, filepath.Join(c.Home, ".local/bin"), filepath.Join(c.Home, ".opencode/bin"), filepath.Join(c.Home, ".kimi-code/bin"))
	for _, dir := range roots {
		if dir == "" {
			continue
		}
		for _, command := range d.Commands {
			path := filepath.Join(dir, command)
			info, err := os.Stat(path)
			if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
				return true
			}
		}
	}
	return false
}
