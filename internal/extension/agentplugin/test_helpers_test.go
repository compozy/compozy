package agentplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newPackageRoot(t *testing.T, name string) string {
	t.Helper()

	root := canonicalPathForTest(t, t.TempDir())
	writeJSONFile(t, filepath.Join(root, "plugin.json"), map[string]any{
		"$schema": PluginSchemaID,
		"name":    name,
	})
	return root
}

func canonicalPathForTest(t *testing.T, path string) string {
	t.Helper()

	canonical, err := canonicalExistingPrefix(path)
	if err != nil {
		t.Fatalf("canonicalExistingPrefix(%q) error = %v", path, err)
	}
	return canonical
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%q) error = %v", path, err)
	}
	writeFile(t, path, content)
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func loadForTest(t *testing.T, root string) *Package {
	t.Helper()

	pkg, err := Load(root, LoadOptions{DataDir: filepath.Join(t.TempDir(), "plugin-data")})
	if err != nil {
		t.Fatalf("Load(%q) error = %v", root, err)
	}
	return pkg
}

func validSkill(name string, description string) []byte {
	return []byte("---\nname: " + name + "\ndescription: " + description + "\n---\n\nInstructions.\n")
}

func writeMCPFile(t *testing.T, root string, servers map[string]any) {
	t.Helper()

	writeJSONFile(t, filepath.Join(root, "mcp.json"), map[string]any{
		"$schema":    MCPSchemaID,
		"mcpServers": servers,
	})
}

func skillNames(skills []SkillRef) []string {
	names := make([]string, len(skills))
	for index, skill := range skills {
		names[index] = skill.Name
	}
	return names
}

func serverNames(servers []ServerSpec) []string {
	names := make([]string, len(servers))
	for index, server := range servers {
		names[index] = server.Name
	}
	return names
}

func hasDiagnostic(diagnostics []Diagnostic, scope string, messagePart string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Scope == scope && strings.Contains(diagnostic.Message, messagePart) {
			return true
		}
	}
	return false
}
