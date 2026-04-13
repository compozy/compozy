package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func testExtensionReusableAgents(t *testing.T, names ...string) []ReusableAgent {
	t.Helper()

	root := t.TempDir()
	sources := make([]ExtensionReusableAgentSource, 0, len(names))
	for _, name := range names {
		agentDir := filepath.Join(root, name)
		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", agentDir, err)
		}
		content := "---\n" +
			"title: " + name + "\n" +
			"description: Test reusable agent " + name + "\n" +
			"---\n"
		if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write AGENT.md for %s: %v", name, err)
		}
		sources = append(sources, ExtensionReusableAgentSource{
			ExtensionName:   "idea-ext",
			ExtensionSource: "workspace",
			ManifestPath:    filepath.Join(root, "extension.toml"),
			Pattern:         "agents/*",
			ResolvedPath:    agentDir,
		})
	}

	reusableAgents, err := ListExtensionReusableAgents(sources)
	if err != nil {
		t.Fatalf("ListExtensionReusableAgents() error = %v", err)
	}
	return reusableAgents
}
