//go:build integration

package skills

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegistryIntegrationRefreshPromotesSidecarBackedSkillToMarketplace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	content := skillWithDescription("installed", "Installed from marketplace")
	skillPath := writeSkillFile(t, userDir, filepath.Join("installed", skillFileName), content)

	registry := newTestRegistry(t, RegistryConfig{
		UserSkillsDir: userDir,
	})

	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	skill, ok := registry.Get("installed")
	if !ok {
		t.Fatal("Get(installed) ok = false, want initial user skill")
	}
	if skill.Source != SourceUser {
		t.Fatalf("initial Source = %v, want %v", skill.Source, SourceUser)
	}

	if err := WriteSidecar(filepath.Dir(skillPath), Provenance{
		Hash:        mustComputeDirectoryHash(t, filepath.Dir(skillPath)),
		Registry:    "clawhub",
		Slug:        "@author/installed",
		Version:     "1.0.0",
		InstalledAt: time.Date(2026, 4, 7, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	if err := registry.RefreshGlobal(context.Background()); err != nil {
		t.Fatalf("RefreshGlobal() error = %v", err)
	}

	skill, ok = registry.Get("installed")
	if !ok {
		t.Fatal("Get(installed) ok = false after refresh, want marketplace skill")
	}
	if skill.Source != SourceMarketplace {
		t.Fatalf("refreshed Source = %v, want %v", skill.Source, SourceMarketplace)
	}
	if skill.InstalledFrom != "@author/installed" {
		t.Fatalf("InstalledFrom = %q, want %q", skill.InstalledFrom, "@author/installed")
	}
	if skill.Provenance == nil || skill.Provenance.Slug != "@author/installed" {
		t.Fatalf("Provenance = %#v, want loaded sidecar provenance", skill.Provenance)
	}
}

func TestRegistryIntegrationRefreshBlocksTamperedMarketplaceSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	original := skillWithDescription("tampered-reload", "Original marketplace skill")
	tampered := skillWithDescription("tampered-reload", "Tampered marketplace skill")
	skillPath := writeSkillFile(t, userDir, filepath.Join("tampered-reload", skillFileName), original)
	originalHash := mustComputeDirectoryHash(t, filepath.Dir(skillPath))
	if err := WriteSidecar(filepath.Dir(skillPath), Provenance{
		Hash:        originalHash,
		Registry:    "clawhub",
		Slug:        "@author/tampered-reload",
		Version:     "1.0.0",
		InstalledAt: time.Date(2026, 4, 7, 14, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	registry := newTestRegistry(t, RegistryConfig{
		UserSkillsDir: userDir,
	}, WithLogger(logger))

	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	rewriteSkillFile(t, skillPath, tampered)
	actualHash := mustComputeDirectoryHash(t, filepath.Dir(skillPath))

	if err := registry.RefreshGlobal(context.Background()); err != nil {
		t.Fatalf("RefreshGlobal() error = %v", err)
	}

	if _, ok := registry.Get("tampered-reload"); ok {
		t.Fatal("Get(tampered-reload) ok = true after tamper refresh, want marketplace skill blocked")
	}

	output := logs.String()
	if !strings.Contains(output, "marketplace skill hash mismatch") {
		t.Fatalf("logs = %q, want hash mismatch warning", output)
	}
	if !strings.Contains(output, "skill_name=tampered-reload") {
		t.Fatalf("logs = %q, want skill_name field", output)
	}
	if !strings.Contains(output, "expected_hash="+originalHash) {
		t.Fatalf("logs = %q, want expected hash", output)
	}
	if !strings.Contains(output, "actual_hash="+actualHash) {
		t.Fatalf("logs = %q, want actual hash", output)
	}
}
