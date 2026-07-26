package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func BenchmarkMergedSkillList(b *testing.B) {
	b.ReportAllocs()

	global := benchmarkSkillMap(256, 0, SourceUser)
	workspace := benchmarkSkillMap(64, 32, SourceWorkspace)

	for b.Loop() {
		skills := mergedSkillList(global, workspace)
		if len(skills) == 0 {
			b.Fatal("mergedSkillList() returned no skills")
		}
	}
}

func BenchmarkBuildCatalog(b *testing.B) {
	b.ReportAllocs()

	skills := benchmarkSkillSlice(256, 0, SourceUser)

	for b.Loop() {
		catalog := BuildCatalog(skills)
		if catalog == "" {
			b.Fatal("BuildCatalog() returned empty catalog")
		}
	}
}

func BenchmarkMCPResolverResolve(b *testing.B) {
	b.ReportAllocs()

	resolver := NewMCPResolver(aghconfig.SkillsConfig{
		AllowedMarketplaceMCP: []string{"skill-001", "trusted-registry:skill-003", "hash-005"},
	}, nil)
	skills := benchmarkMCPSkills(96)

	for b.Loop() {
		servers := resolver.Resolve(skills)
		if len(servers) == 0 {
			b.Fatal("Resolve() returned no servers")
		}
	}
}

func BenchmarkComputeDirectoryHash(b *testing.B) {
	b.ReportAllocs()

	root := benchmarkHashTree(b)

	for b.Loop() {
		hash, err := ComputeDirectoryHash(root)
		if err != nil {
			b.Fatalf("ComputeDirectoryHash() error = %v", err)
		}
		if hash == "" {
			b.Fatal("ComputeDirectoryHash() returned empty hash")
		}
	}
}

func BenchmarkScanDirectoryWithSnapshots(b *testing.B) {
	b.ReportAllocs()

	root := benchmarkScanTree(b)

	for b.Loop() {
		paths, snapshots, err := scanDirectoryWithSnapshots(root)
		if err != nil {
			b.Fatalf("scanDirectoryWithSnapshots() error = %v", err)
		}
		if len(paths) == 0 || len(snapshots) == 0 {
			b.Fatal("scanDirectoryWithSnapshots() returned empty results")
		}
	}
}

func BenchmarkRegistryForWorkspaceCached(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	userDir := b.TempDir()
	for i := range 128 {
		benchmarkWriteSkill(
			b,
			userDir,
			filepath.Join(fmt.Sprintf("global-%03d", i), skillFileName),
			benchmarkSkillContent(fmt.Sprintf("global-%03d", i), benchmarkDescription(i)),
		)
	}

	workspaceRoot := b.TempDir()
	workspaceSkillDirs := make([]workspacepkg.SkillPath, 0, 48)
	for i := range 48 {
		dir := filepath.Join(workspaceRoot, fmt.Sprintf("workspace-%03d", i))
		benchmarkWriteSkill(
			b,
			dir,
			skillFileName,
			benchmarkSkillContent(fmt.Sprintf("workspace-%03d", i), benchmarkDescription(i)),
		)
		workspaceSkillDirs = append(workspaceSkillDirs, workspacepkg.SkillPath{
			Dir:    dir,
			Source: "workspace",
		})
	}

	registry := NewRegistry(RegistryConfig{UserSkillsDir: userDir})
	if err := registry.LoadAll(ctx); err != nil {
		b.Fatalf("LoadAll() error = %v", err)
	}

	resolved := &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      "bench-workspace",
			RootDir: workspaceRoot,
		},
		Skills: workspaceSkillDirs,
	}
	if _, err := registry.ForWorkspace(ctx, resolved); err != nil {
		b.Fatalf("ForWorkspace(warmup) error = %v", err)
	}

	for b.Loop() {
		skills, err := registry.ForWorkspace(ctx, resolved)
		if err != nil {
			b.Fatalf("ForWorkspace() error = %v", err)
		}
		if len(skills) == 0 {
			b.Fatal("ForWorkspace() returned no skills")
		}
	}
}

func benchmarkSkillMap(count int, overlap int, source SkillSource) map[string]*Skill {
	skills := make(map[string]*Skill, count)
	for i := range count {
		name := fmt.Sprintf("skill-%03d", i)
		if i < overlap {
			name = fmt.Sprintf("shared-%03d", i)
		}
		skills[name] = benchmarkSkill(name, benchmarkDescription(i), source)
	}
	return skills
}

func benchmarkSkillSlice(count int, overlap int, source SkillSource) []*Skill {
	skills := make([]*Skill, 0, count)
	for i := range count {
		name := fmt.Sprintf("skill-%03d", i)
		if i < overlap {
			name = fmt.Sprintf("shared-%03d", i)
		}
		skills = append(skills, benchmarkSkill(name, benchmarkDescription(i), source))
	}
	return skills
}

func benchmarkSkill(name string, description string, source SkillSource) *Skill {
	return &Skill{
		Meta: SkillMeta{
			Name:        name,
			Description: description,
			Metadata: map[string]any{
				"category": "benchmark",
				"tags":     []any{"skill", "bench", name},
			},
		},
		Source:   source,
		Dir:      filepath.Join("bench", name),
		FilePath: filepath.Join("bench", name, skillFileName),
		Enabled:  true,
		MCPServers: []MCPServerDecl{
			{
				Name:    name + "-mcp",
				Command: "cmd-" + name,
				Args:    []string{"--name", name},
				Env:     map[string]string{"MODE": "bench"},
			},
		},
	}
}

func benchmarkDescription(i int) string {
	return strings.Repeat(fmt.Sprintf("benchmark description %03d ", i), 6)
}

func benchmarkMCPSkills(count int) []*Skill {
	skills := make([]*Skill, 0, count)
	for i := range count {
		var source SkillSource
		provenance := (*Provenance)(nil)
		switch i % 4 {
		case 0:
			source = SourceBundled
		case 1:
			source = SourceUser
		case 2:
			source = SourceWorkspace
		default:
			source = SourceMarketplace
			provenance = &Provenance{
				Hash:     fmt.Sprintf("hash-%03d", i),
				Registry: "trusted-registry",
				Slug:     fmt.Sprintf("skill-%03d", i),
				Version:  "1.0.0",
			}
		}
		skills = append(skills, &Skill{
			Meta: SkillMeta{
				Name:        fmt.Sprintf("skill-%03d", i),
				Description: "benchmark mcp skill",
			},
			Source:     source,
			Enabled:    true,
			Provenance: provenance,
			MCPServers: []MCPServerDecl{
				{
					Name:    fmt.Sprintf("server-%02d", i%24),
					Command: fmt.Sprintf("cmd-%02d", i%24),
					Args:    []string{"--skill", fmt.Sprintf("skill-%03d", i)},
					Env: map[string]string{
						"INDEX": fmt.Sprintf("%03d", i),
					},
				},
			},
		})
	}
	return skills
}

func benchmarkHashTree(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	for dirIndex := range 16 {
		dir := filepath.Join(root, fmt.Sprintf("pkg-%02d", dirIndex))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
		for fileIndex := range 24 {
			path := filepath.Join(dir, fmt.Sprintf("file-%02d.md", fileIndex))
			payload := strings.Repeat(fmt.Sprintf("payload-%02d-%02d-", dirIndex, fileIndex), 128)
			if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
				b.Fatalf("WriteFile(%q) error = %v", path, err)
			}
		}
	}
	if err := os.Symlink(filepath.Join("pkg-00", "file-00.md"), filepath.Join(root, "current")); err != nil {
		b.Fatalf("Symlink() error = %v", err)
	}
	return root
}

func benchmarkScanTree(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	for i := range 96 {
		benchmarkWriteSkill(
			b,
			root,
			filepath.Join(fmt.Sprintf("skill-%03d", i), skillFileName),
			benchmarkSkillContent(fmt.Sprintf("skill-%03d", i), benchmarkDescription(i)),
		)
	}
	for i := range 24 {
		benchmarkWriteSkill(
			b,
			root,
			filepath.Join("nested", fmt.Sprintf("group-%02d", i), fmt.Sprintf("skill-%02d", i), skillFileName),
			benchmarkSkillContent(fmt.Sprintf("nested-%02d", i), benchmarkDescription(i)),
		)
	}
	return root
}

func benchmarkWriteSkill(tb testing.TB, root, relPath, content string) string {
	tb.Helper()

	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		tb.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}

func benchmarkSkillContent(name string, description string) string {
	return strings.Join([]string{
		"---",
		"name: " + name,
		"description: " + description,
		"metadata:",
		"  owner: benchmark",
		"---",
		"body",
	}, "\n")
}
