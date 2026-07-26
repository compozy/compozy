//go:build integration

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	registrypkg "github.com/compozy/agh/internal/registry"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/testutil"
	skillbundled "github.com/compozy/agh/skills"
)

func TestSkillSearchInstallListRemoveIntegrationFlow(t *testing.T) {
	t.Parallel()

	server := newMarketplaceTestServer(t, marketplaceServerFixture{
		searchResults: []registrypkg.Listing{{
			Slug:        "@agh/review",
			Name:        "review",
			Description: "Review helper",
			Author:      "agh",
			Version:     "1.2.0",
		}},
		info: map[string]registrypkg.Detail{
			"@agh/review": {Listing: registrypkg.Listing{Slug: "@agh/review", Name: "review", Version: "1.2.0"}},
		},
		downloads: map[string]marketplaceDownloadFixture{
			"@agh/review": {
				version: "1.2.0",
				files: map[string]string{
					"review/SKILL.md": skillDocument("review", "Review helper", "body"),
				},
			},
		},
	})
	defer server.Close()

	env := newSkillTestEnv(t, func(cfg *aghconfig.Config) {
		cfg.Skills.Marketplace = aghconfig.MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  server.URL(),
		}
	})

	searchStdout, _, err := executeRootCommand(t, env.deps, "skill", "search", "review", "-o", "json")
	if err != nil {
		t.Fatalf("skill search error = %v", err)
	}

	var searchPayload []registrypkg.Listing
	if err := json.Unmarshal([]byte(searchStdout), &searchPayload); err != nil {
		t.Fatalf("json.Unmarshal(skill search) error = %v; stdout=%s", err, searchStdout)
	}
	if len(searchPayload) != 1 || searchPayload[0].Slug != "@agh/review" {
		t.Fatalf("skill search payload = %#v, want one review listing", searchPayload)
	}

	if _, _, err := executeRootCommand(t, env.deps, "skill", "install", "@agh/review", "-o", "json"); err != nil {
		t.Fatalf("skill install error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(env.homePaths.SkillsDir, "review", ".agh-meta.json")); err != nil {
		t.Fatalf("installed sidecar stat error = %v", err)
	}

	listStdout, _, err := executeRootCommand(t, env.deps, "skill", "list", "-o", "json")
	if err != nil {
		t.Fatalf("skill list error = %v", err)
	}

	var listPayload []skillListItem
	if err := json.Unmarshal([]byte(listStdout), &listPayload); err != nil {
		t.Fatalf("json.Unmarshal(skill list) error = %v; stdout=%s", err, listStdout)
	}
	item := findSkillListItem(t, listPayload, "review")
	if item.Source != "marketplace" {
		t.Fatalf("skill list item = %#v, want marketplace source", item)
	}

	if _, _, err := executeRootCommand(t, env.deps, "skill", "remove", "review", "-o", "json"); err != nil {
		t.Fatalf("skill remove error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.homePaths.SkillsDir, "review")); !os.IsNotExist(err) {
		t.Fatalf("removed skill stat error = %v, want not exist", err)
	}
}

func TestSkillInstallCommandIntegrationCreatesSkillDirectoryAndSidecar(t *testing.T) {
	t.Parallel()

	server := newMarketplaceTestServer(t, marketplaceServerFixture{
		downloads: map[string]marketplaceDownloadFixture{
			"@agh/review": {
				version: "1.2.0",
				files: map[string]string{
					"review/SKILL.md": skillDocument("review", "Review helper", "body"),
				},
			},
		},
	})
	defer server.Close()

	env := newSkillTestEnv(t, func(cfg *aghconfig.Config) {
		cfg.Skills.Marketplace = aghconfig.MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  server.URL(),
		}
	})

	stdout, _, err := executeRootCommand(t, env.deps, "skill", "install", "@agh/review", "-o", "json")
	if err != nil {
		t.Fatalf("skill install error = %v", err)
	}

	var payload skillInstallItem
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(skill install) error = %v; stdout=%s", err, stdout)
	}
	if payload.Status != "installed" {
		t.Fatalf("skill install payload = %#v, want installed status", payload)
	}

	skillDir := filepath.Join(env.homePaths.SkillsDir, "review")
	if _, err := os.Stat(filepath.Join(skillDir, skillMarkdownFileName)); err != nil {
		t.Fatalf("installed SKILL.md stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, ".agh-meta.json")); err != nil {
		t.Fatalf("installed sidecar stat error = %v", err)
	}
}

func TestSkillInstallAndRemoveIntegrationRefreshesRegistry(t *testing.T) {
	t.Parallel()

	server := newMarketplaceTestServer(t, marketplaceServerFixture{
		downloads: map[string]marketplaceDownloadFixture{
			"@agh/cleanup": {
				version: "1.0.0",
				files: map[string]string{
					"cleanup/SKILL.md": skillDocument("cleanup", "Cleanup helper", "body"),
				},
			},
		},
	})
	defer server.Close()

	env := newSkillTestEnv(t, func(cfg *aghconfig.Config) {
		cfg.Skills.Marketplace = aghconfig.MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  server.URL(),
		}
	})

	if _, _, err := executeRootCommand(t, env.deps, "skill", "install", "@agh/cleanup", "-o", "json"); err != nil {
		t.Fatalf("skill install error = %v", err)
	}

	registry := skills.NewRegistry(skills.RegistryConfig{
		BundledFS:     skillbundled.FS(),
		UserSkillsDir: env.homePaths.SkillsDir,
		UserAgentsDir: env.homePaths.AgentsDir,
	})
	if err := registry.LoadAll(testutil.Context(t)); err != nil {
		t.Fatalf("registry.LoadAll() error = %v", err)
	}

	installed, ok := registry.Get("cleanup")
	if !ok {
		t.Fatal("registry.Get(cleanup) ok = false, want installed marketplace skill")
	}
	if installed.Source != skills.SourceMarketplace {
		t.Fatalf("installed.Source = %v, want marketplace", installed.Source)
	}

	if _, _, err := executeRootCommand(t, env.deps, "skill", "remove", "cleanup", "-o", "json"); err != nil {
		t.Fatalf("skill remove error = %v", err)
	}
	if err := registry.RefreshGlobal(testutil.Context(t)); err != nil {
		t.Fatalf("registry.RefreshGlobal() error = %v", err)
	}
	if _, ok := registry.Get("cleanup"); ok {
		t.Fatal("registry.Get(cleanup) ok = true after remove, want missing skill")
	}
}

func TestSkillInstallCommandIntegrationWritesMatchingHash(t *testing.T) {
	t.Parallel()

	server := newMarketplaceTestServer(t, marketplaceServerFixture{
		downloads: map[string]marketplaceDownloadFixture{
			"@agh/hashcheck": {
				version: "2.0.0",
				files: map[string]string{
					"hashcheck/SKILL.md": skillDocument("hashcheck", "Hash helper", "body"),
				},
			},
		},
	})
	defer server.Close()

	env := newSkillTestEnv(t, func(cfg *aghconfig.Config) {
		cfg.Skills.Marketplace = aghconfig.MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  server.URL(),
		}
	})

	if _, _, err := executeRootCommand(t, env.deps, "skill", "install", "@agh/hashcheck", "-o", "json"); err != nil {
		t.Fatalf("skill install error = %v", err)
	}

	skillDir := filepath.Join(env.homePaths.SkillsDir, "hashcheck")
	hash, err := skills.ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash() error = %v", err)
	}
	provenance, err := skills.ReadSidecar(skillDir)
	if err != nil {
		t.Fatalf("ReadSidecar() error = %v", err)
	}
	if provenance == nil {
		t.Fatal("ReadSidecar() = nil, want provenance")
	}

	if got, want := provenance.Hash, hash; got != want {
		t.Fatalf("provenance.Hash = %q, want %q", got, want)
	}
}

func TestSkillInstallCommandIntegrationReplacesExistingSkillDirectory(t *testing.T) {
	t.Parallel()

	server := newMarketplaceTestServer(t, marketplaceServerFixture{
		info: map[string]registrypkg.Detail{
			"@agh/review": {Listing: registrypkg.Listing{Slug: "@agh/review", Name: "review", Version: "2.0.0"}},
		},
		downloads: map[string]marketplaceDownloadFixture{
			"@agh/review": {
				version: "2.0.0",
				files: map[string]string{
					"review/SKILL.md": skillDocument("review", "Review helper", "new body"),
				},
			},
		},
	})
	defer server.Close()

	env := newSkillTestEnv(t, func(cfg *aghconfig.Config) {
		cfg.Skills.Marketplace = aghconfig.MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  server.URL(),
		}
	})
	writeInstalledMarketplaceSkill(
		t,
		env.homePaths,
		"review",
		"@agh/review",
		"1.0.0",
		skillDocument("review", "Review helper", "old body"),
	)

	if _, _, err := executeRootCommand(t, env.deps, "skill", "install", "@agh/review", "-o", "json"); err != nil {
		t.Fatalf("skill install replace error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(env.homePaths.SkillsDir, "review", skillMarkdownFileName))
	if err != nil {
		t.Fatalf("ReadFile(updated skill) error = %v", err)
	}
	if !strings.Contains(string(content), "new body") {
		t.Fatalf("updated skill content = %q, want replacement content", string(content))
	}

	provenance, err := skills.ReadSidecar(filepath.Join(env.homePaths.SkillsDir, "review"))
	if err != nil {
		t.Fatalf("ReadSidecar() error = %v", err)
	}
	if provenance == nil || provenance.Version != "2.0.0" {
		t.Fatalf("provenance = %#v, want updated version", provenance)
	}
}
