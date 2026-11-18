package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	ClientComponentIgnored = "client_component_ignored"
	ClientFieldIgnored     = "client_field_ignored"
	ClientMCPDiscovered    = "client_mcp_discovered"
)

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
	diagnostics := normalizeClientMetadata(metadata, opts)

	normalized, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("normalize client manifest metadata: %w", err)
	}
	pkg, _, err := decodeManifest(normalized)
	if err != nil {
		return nil, err
	}
	pkg.Diagnostics = append(pkg.Diagnostics, diagnostics...)
	discoverSkills(root, pkg)
	loadClientSkills(root, fields["skills"], pkg)
	raw, declared := fields[fieldMCPServers]
	if !declared {
		raw = discoverClientMCP(root, pkg)
	}
	loadClientMCP(root, opts.DataDir, raw, pkg)
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

func normalizeClientMetadata(metadata map[string]json.RawMessage, opts LoadOptions) []Diagnostic {
	var diagnostics []Diagnostic
	ignored := func(field string) {
		diagnostics = append(diagnostics, Diagnostic{Code: ClientFieldIgnored, Scope: scopeManifest,
			Message: fmt.Sprintf("%q client metadata was normalized or ignored", field)})
	}
	if raw, exists := metadata[fieldSchema]; exists && string(raw) != `"`+PluginSchemaID+`"` {
		ignored(fieldSchema)
	}
	metadata[fieldSchema] = json.RawMessage(`"` + PluginSchemaID + `"`)
	var name string
	if json.Unmarshal(metadata[fieldName], &name) == nil {
		name = strings.ToLower(name)
	}
	if ValidateName(name) != nil && opts.Name != "" {
		name = opts.Name
	}
	normalized := []byte(strconv.Quote(name))
	if string(metadata[fieldName]) != string(normalized) {
		ignored(fieldName)
	}
	metadata[fieldName] = normalized

	for _, field := range []string{fieldVersion, fieldDescription, fieldHomepage, fieldRepository, fieldLicense} {
		if raw, exists := metadata[field]; exists {
			var value string
			if isJSONNull(raw) || json.Unmarshal(raw, &value) != nil {
				delete(metadata, field)
				ignored(field)
			}
		}
	}
	if raw, exists := metadata[fieldKeywords]; exists {
		if _, ok := decodeStringSlice(raw); !ok {
			delete(metadata, fieldKeywords)
			ignored(fieldKeywords)
		}
	}
	normalizeClientAuthor(metadata, ignored)
	return diagnostics
}

func normalizeClientAuthor(metadata map[string]json.RawMessage, ignored func(string)) {
	if raw, exists := metadata[fieldAuthor]; exists {
		var author map[string]json.RawMessage
		if json.Unmarshal(raw, &author) != nil || author == nil {
			delete(metadata, fieldAuthor)
			ignored(fieldAuthor)
		} else {
			for field, value := range author {
				var text string
				if !slices.Contains([]string{fieldName, fieldEmail, fieldURL}, field) || isJSONNull(value) ||
					json.Unmarshal(value, &text) != nil {
					delete(author, field)
					ignored(fieldAuthor + "." + field)
				}
			}
			encoded, err := json.Marshal(author)
			if err == nil {
				metadata[fieldAuthor] = encoded
			}
		}
	}
}
