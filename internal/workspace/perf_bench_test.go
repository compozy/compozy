package workspace

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

type benchmarkResolverFixture struct {
	ctx        context.Context
	workspace  Workspace
	resolver   *Resolver
	store      *mockWorkspaceStore
	homePaths  string
	additional string
}

func newBenchmarkResolverFixture(tb testing.TB) benchmarkResolverFixture {
	tb.Helper()

	ctx := context.Background()
	homePaths := newTestHomePaths(tb)
	rootDir := tb.TempDir()
	additionalDir := tb.TempDir()

	writeAgentDef(tb, filepath.Join(homePaths.AgentsDir, "global", agentDefinitionFile), "global-agent", "sonnet")
	writeSkill(tb, filepath.Join(homePaths.SkillsDir, "global-skill"))
	writeAgentDef(
		tb,
		filepath.Join(rootDir, ".compozy", "agents", "local-agent", agentDefinitionFile),
		"local-agent",
		"haiku",
	)
	writeSkill(tb, filepath.Join(rootDir, ".compozy", "skills", "local-skill"))
	writeAgentDef(
		tb,
		filepath.Join(additionalDir, ".compozy", "agents", "additional-agent", agentDefinitionFile),
		"additional-agent",
		"opus",
	)
	writeSkill(tb, filepath.Join(additionalDir, ".compozy", "skills", "additional-skill"))
	writeFile(tb, filepath.Join(rootDir, ".compozy", "config.toml"), "[http]\nport = 4242\n")

	workspace := Workspace{
		ID:             "ws_bench",
		RootDir:        rootDir,
		AdditionalDirs: []string{additionalDir},
		Name:           "repo",
		CreatedAt:      time.Unix(1700000000, 0).UTC(),
		UpdatedAt:      time.Unix(1700000000, 0).UTC(),
	}
	store := newMockWorkspaceStore(workspace)
	resolver := newTestResolver(tb, store,
		WithHomePaths(homePaths),
		WithCacheTTL(time.Hour),
	)

	if _, err := resolver.Resolve(ctx, workspace.ID); err != nil {
		tb.Fatalf("Resolve(prewarm) error = %v", err)
	}

	return benchmarkResolverFixture{
		ctx:        ctx,
		workspace:  workspace,
		resolver:   resolver,
		store:      store,
		homePaths:  homePaths.HomeDir,
		additional: additionalDir,
	}
}

func BenchmarkResolverResolve(b *testing.B) {
	b.Run("cache_hit", func(b *testing.B) {
		fixture := newBenchmarkResolverFixture(b)

		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			_, err := fixture.resolver.Resolve(fixture.ctx, fixture.workspace.ID)
			if err != nil {
				b.Fatalf("Resolve(cache hit) error = %v", err)
			}
		}
	})

	b.Run("cache_miss", func(b *testing.B) {
		fixture := newBenchmarkResolverFixture(b)

		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			fixture.resolver.Invalidate(fixture.workspace.ID)
			_, err := fixture.resolver.Resolve(fixture.ctx, fixture.workspace.ID)
			if err != nil {
				b.Fatalf("Resolve(cache miss) error = %v", err)
			}
		}
	})
}

func BenchmarkResolverList(b *testing.B) {
	ctx := context.Background()
	rootPrefix := filepath.Join(string(filepath.Separator), "tmp", "workspace")
	newID := func(prefix string) string {
		id, err := generateID(prefix)
		if err != nil {
			b.Fatalf("generateID(%q) error = %v", prefix, err)
		}
		return id
	}
	workspaces := make([]Workspace, 128)
	for i := range workspaces {
		workspaces[i] = Workspace{
			ID:        newID("ws"),
			RootDir:   filepath.Join(rootPrefix, newID("bench")),
			Name:      newID("repo"),
			CreatedAt: time.Unix(1700000000, 0).UTC(),
			UpdatedAt: time.Unix(1700000000, 0).UTC(),
		}
	}

	store := newMockWorkspaceStore(workspaces...)
	resolver := newTestResolver(b, store, WithHomePaths(newTestHomePaths(b)))

	b.ReportAllocs()

	for b.Loop() {
		if _, err := resolver.List(ctx); err != nil {
			b.Fatalf("List() error = %v", err)
		}
	}
}

func BenchmarkCloneResolvedWorkspace(b *testing.B) {
	fixture := newBenchmarkResolverFixture(b)
	resolved, err := fixture.resolver.Resolve(fixture.ctx, fixture.workspace.ID)
	if err != nil {
		b.Fatalf("Resolve() error = %v", err)
	}

	b.ReportAllocs()

	for b.Loop() {
		cloneResolvedWorkspace(&resolved)
	}
}
