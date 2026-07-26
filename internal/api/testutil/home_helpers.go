package testutil

import (
	"os"
	"path/filepath"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func NewTestHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func WriteAgentDef(t *testing.T, homePaths aghconfig.HomePaths, name string) {
	t.Helper()

	path := filepath.Join(homePaths.AgentsDir, name, "AGENT.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(agent dir) error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`---
name: `+name+`
provider: fake
permissions: approve-reads
---

You are `+name+`.
`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(AGENT.md) error = %v", err)
	}
}
