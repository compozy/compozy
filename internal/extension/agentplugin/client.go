package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

const ClientComponentIgnored = "client_component_ignored"

type clientManifestAdapter func(string, []byte, LoadOptions) (*Package, error)

var clientAdapters = map[string]clientManifestAdapter{
	"claude-plugin": loadClientManifest,
	"codex-plugin":  loadClientManifest,
	"cursor-plugin": loadClientManifest,
}

func loadClientManifest(root string, content []byte, opts LoadOptions) (*Package, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil || fields == nil {
		return nil, &ManifestError{Issues: []Issue{{Path: "$", Message: jsonObjectIssueMessage}}}
	}
	metadata := make(map[string]json.RawMessage, len(fields))
	for key, value := range fields {
		if key != "skills" && key != fieldMCPServers && key != "commands" && key != "agents" && key != "hooks" &&
			key != "displayName" {
			metadata[key] = value
		}
	}
	if _, declared := metadata[fieldSchema]; !declared {
		metadata[fieldSchema] = json.RawMessage(`"` + PluginSchemaID + `"`)
	}
	normalized, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("normalize client manifest metadata: %w", err)
	}
	pkg, _, err := decodeManifest(normalized)
	if err != nil {
		return nil, err
	}
	discoverSkills(root, pkg)
	loadClientSkills(root, fields["skills"], pkg)
	loadClientMCP(root, opts.DataDir, fields[fieldMCPServers], pkg)
	for _, field := range []string{"commands", "agents", "hooks"} {
		_, declared := fields[field]
		_, diskErr := os.Lstat(filepath.Join(root, field))
		if declared || diskErr == nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{
				Code: ClientComponentIgnored, Scope: field, Message: "client " + field + " are not loaded",
			})
		}
	}
	return pkg, nil
}

func loadClientSkills(root string, raw json.RawMessage, pkg *Package) {
	if len(raw) == 0 {
		return
	}
	paths, err := clientPaths(raw)
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeSkills, Message: err.Error()})
		return
	}
	for _, path := range paths {
		resolved, err := resolveContained(filepath.FromSlash(path), root)
		if err != nil {
			pkg.Diagnostics = append(
				pkg.Diagnostics,
				Diagnostic{Scope: scopeSkills, Message: "skills path must remain inside package root"},
			)
			continue
		}
		file := filepath.Join(resolved, "SKILL.md")
		if filepath.Base(resolved) == "SKILL.md" {
			file, resolved = resolved, filepath.Dir(resolved)
		}
		directory, err := fileutil.OpenDirectory(resolved)
		if err != nil {
			pkg.Diagnostics = append(
				pkg.Diagnostics,
				Diagnostic{Scope: scopeSkills, Message: "skills path must be an in-root directory"},
			)
			continue
		}
		if err := directory.Close(); err != nil {
			pkg.Diagnostics = append(
				pkg.Diagnostics,
				Diagnostic{Scope: scopeSkills, Message: "cannot close skills directory"},
			)
			continue
		}
		if info, err := os.Lstat(file); err == nil && info.Mode().IsRegular() {
			discoverSkill(root, resolved, pkg)
		} else {
			discoverSkillsDirectory(root, resolved, pkg)
		}
	}
	slices.SortFunc(pkg.Skills, func(a, b SkillRef) int { return strings.Compare(a.SkillFile, b.SkillFile) })
	pkg.Skills = slices.CompactFunc(pkg.Skills, func(a, b SkillRef) bool { return a.SkillFile == b.SkillFile })
}

func clientPaths(raw json.RawMessage) ([]string, error) {
	var path string
	if json.Unmarshal(raw, &path) == nil && path != "" {
		return []string{path}, nil
	}
	var paths []string
	if isJSONNull(raw) || json.Unmarshal(raw, &paths) != nil {
		return nil, fmt.Errorf("skills must be a path or an array of paths")
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("skills paths must be non-empty")
		}
	}
	return paths, nil
}
