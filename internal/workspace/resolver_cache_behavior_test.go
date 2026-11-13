package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func BenchmarkResolveWarmWorkspace(b *testing.B) {
	home := newTestHomePaths(b)
	ws := Workspace{ID: "ws_bench", Name: "bench", RootDir: b.TempDir()}
	for i := range 100 {
		writeSkill(
			b,
			filepath.Join(ws.RootDir, compozyconfig.DirName, compozyconfig.SkillsDirName, fmt.Sprintf("skill-%03d", i)),
		)
	}
	r := newTestResolver(b, newMockWorkspaceStore(ws), WithHomePaths(home), WithSnapshotTTL(time.Second))
	if _, err := r.Resolve(b.Context(), ws.ID); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := r.Resolve(b.Context(), ws.ID); err != nil {
			b.Fatal(err)
		}
	}
}

func TestResolveCacheHitInvalidateAndEviction(t *testing.T) {
	t.Parallel()
	t.Run("Should refresh configuration changes within the discovery TTL", func(t *testing.T) {
		for _, layer := range []string{"user", "workspace", "profile", "workspace-profile"} {
			t.Run("Should refresh "+layer, func(t *testing.T) {
				home := newTestHomePaths(t)
				ws := Workspace{ID: "ws_config_ttl", Name: "repo", RootDir: t.TempDir()}
				paths := map[string]string{
					"user":      home.ConfigFile,
					"workspace": filepath.Join(ws.RootDir, compozyconfig.DirName, compozyconfig.ConfigName),
					"profile":   filepath.Join(home.ProfilesDir, "design", compozyconfig.ConfigName),
					"workspace-profile": filepath.Join(
						ws.RootDir,
						compozyconfig.DirName,
						compozyconfig.ProfilesDirName,
						"design",
						compozyconfig.ConfigName,
					),
				}
				r := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home), WithSnapshotTTL(time.Minute))
				resolve := func() string {
					t.Helper()
					got, err := r.resolve(t.Context(), ws.ID, "design", "profile-design")
					if err != nil {
						t.Fatal(err)
					}
					return got.Config.Defaults.Agent
				}
				baseline := resolve()
				for _, agent := range []string{"designer", "reviewer"} {
					writeFile(t, paths[layer], "[defaults]\nagent = \""+agent+"\"\n")
					if got := resolve(); got != agent {
						t.Fatalf("agent = %q, want %q", got, agent)
					}
				}
				if err := os.Remove(paths[layer]); err != nil {
					t.Fatal(err)
				}
				if got := resolve(); got != baseline {
					t.Fatalf("agent after deletion = %q, want %q", got, baseline)
				}
			})
		}
	})

	t.Run("Should reload skills config only after a file the loader reads changes", func(t *testing.T) {
		t.Parallel()
		for _, layer := range []string{
			"user", "workspace", "profile", "workspace-profile", "dotenv",
			"user-mcp", "workspace-mcp", "profile-mcp", "workspace-profile-mcp",
		} {
			t.Run("Should reload after a "+layer+" edit", func(t *testing.T) {
				t.Parallel()
				home := newTestHomePaths(t)
				ws := Workspace{ID: "ws_skills_config", Name: "repo", RootDir: t.TempDir()}
				workspaceDir := filepath.Join(ws.RootDir, compozyconfig.DirName)
				paths := map[string]string{
					"user":      home.ConfigFile,
					"workspace": filepath.Join(workspaceDir, compozyconfig.ConfigName),
					"profile":   filepath.Join(home.ProfilesDir, "design", compozyconfig.ConfigName),
					"workspace-profile": filepath.Join(
						workspaceDir, compozyconfig.ProfilesDirName, "design", compozyconfig.ConfigName,
					),
					"dotenv":        compozyconfig.WorkspaceDotEnvFile(ws.RootDir),
					"user-mcp":      filepath.Join(home.HomeDir, compozyconfig.MCPJSONName),
					"workspace-mcp": filepath.Join(workspaceDir, compozyconfig.MCPJSONName),
					"profile-mcp":   filepath.Join(home.ProfilesDir, "design", compozyconfig.MCPJSONName),
					"workspace-profile-mcp": filepath.Join(
						workspaceDir, compozyconfig.ProfilesDirName, "design", compozyconfig.MCPJSONName,
					),
				}
				var loads atomic.Int32
				r := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home),
					WithProfileConfigLoader(func(rootDir, profileName string) (compozyconfig.Config, error) {
						loads.Add(1)
						return compozyconfig.LoadForHome(home,
							compozyconfig.WithWorkspaceRoot(rootDir), compozyconfig.WithProfile(profileName))
					}))
				customSources := func() []string {
					t.Helper()
					got, err := r.SkillsConfig(ws, "design")
					if err != nil {
						t.Fatalf("SkillsConfig() error = %v", err)
					}
					return got.CustomSources
				}
				assertLoads := func(want int32) {
					t.Helper()
					if got := loads.Load(); got != want {
						t.Fatalf("config loads = %d, want %d", got, want)
					}
				}
				for range 3 {
					customSources()
				}
				assertLoads(1)

				content, want := "[skills]\ncustom_sources = [\"/opt/team-skills\"]\n", []string{"/opt/team-skills"}
				if layer == "dotenv" {
					content, want = "COMPOZY_SKILLS_WATCHER_TEST=1\n", nil
				} else if strings.HasSuffix(layer, "-mcp") {
					content, want = "{\"mcpServers\":{}}\n", nil
				}
				writeFile(t, paths[layer], content)
				got := customSources()
				if !slices.Equal(got, want) {
					t.Fatalf("custom sources after %s edit = %v, want %v", layer, got, want)
				}
				for i := range got {
					got[i] = "mutated"
				}
				if again := customSources(); !slices.Equal(again, want) {
					t.Fatalf("custom sources after caller mutation = %v, want isolated %v", again, want)
				}
				assertLoads(2)

				writeFile(t, paths[layer], content+"\n")
				customSources()
				customSources()
				assertLoads(3)

				if err := os.Remove(paths[layer]); err != nil {
					t.Fatal(err)
				}
				if got := customSources(); len(got) != 0 {
					t.Fatalf("custom sources after deletion = %v, want none", got)
				}
				customSources()
				assertLoads(4)

				r.Invalidate(ws.ID)
				customSources()
				assertLoads(5)

				r.InvalidateAll()
				customSources()
				assertLoads(6)
			})
		}
	})

	t.Run("Should not publish a skills config load overtaken by invalidation", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name       string
			invalidate func(*Resolver, string)
		}{
			{
				name:       "Should reload after InvalidateAll runs during the load",
				invalidate: func(r *Resolver, _ string) { r.InvalidateAll() },
			},
			{
				name:       "Should reload after Invalidate of the workspace runs during the load",
				invalidate: func(r *Resolver, workspaceID string) { r.Invalidate(workspaceID) },
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				home := newTestHomePaths(t)
				ws := Workspace{ID: "ws_skills_config_race", Name: "repo", RootDir: t.TempDir()}
				var loads atomic.Int32
				firstLoadStarted, releaseFirstLoad := make(chan struct{}), make(chan struct{})
				r := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home),
					WithProfileConfigLoader(func(rootDir, profileName string) (compozyconfig.Config, error) {
						if loads.Add(1) == 1 {
							close(firstLoadStarted)
							<-releaseFirstLoad
						}
						return compozyconfig.LoadForHome(home,
							compozyconfig.WithWorkspaceRoot(rootDir), compozyconfig.WithProfile(profileName))
					}))
				firstErr := make(chan error, 1)
				go func() {
					_, err := r.SkillsConfig(ws, "design")
					firstErr <- err
				}()
				<-firstLoadStarted
				test.invalidate(r, ws.ID)
				close(releaseFirstLoad)
				if err := <-firstErr; err != nil {
					t.Fatalf("SkillsConfig(overtaken load) error = %v", err)
				}
				if _, err := r.SkillsConfig(ws, "design"); err != nil {
					t.Fatalf("SkillsConfig(after invalidation) error = %v", err)
				}
				if got := loads.Load(); got != 2 {
					t.Fatalf("config loads = %d, want 2: the load overtaken by invalidation was reused", got)
				}
			})
		}
	})

	t.Run("Should bound cached discovery age without extending it on cache hits", func(t *testing.T) {
		t.Parallel()
		home := newTestHomePaths(t)
		ws := Workspace{ID: "ws_snapshot_ttl", Name: "repo", RootDir: t.TempDir()}
		dir := filepath.Join(ws.RootDir, compozyconfig.DirName, compozyconfig.SkillsDirName, "alpha")
		writeSkill(t, dir)
		now := time.Unix(1_700_000_000, 0)
		r := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home),
			WithSnapshotTTL(time.Second), withNow(func() time.Time { return now }))
		assertSkills := func(want ...string) {
			t.Helper()
			resolved, err := r.Resolve(t.Context(), ws.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got := skillNames(resolved.Skills); !slices.Equal(got, want) {
				t.Fatalf("skills = %v, want %v", got, want)
			}
		}
		assertSkills("alpha")
		if err := os.Remove(filepath.Join(dir, skillDefinitionFile)); err != nil {
			t.Fatal(err)
		}
		now = now.Add(500 * time.Millisecond)
		assertSkills("alpha")
		now = now.Add(500 * time.Millisecond)
		assertSkills()
		writeSkill(t, dir)
		r.Invalidate(ws.ID)
		assertSkills("alpha")
		if err := os.Remove(filepath.Join(dir, skillDefinitionFile)); err != nil {
			t.Fatal(err)
		}
		r.InvalidateAll()
		assertSkills()

		// Invariant: a held phase discovers a workspace once instead of once per TTL window, still
		// honors explicit invalidation, and returns to the age bound when released.
		writeSkill(t, dir)
		r.HoldSnapshots()
		now = now.Add(2 * time.Second)
		assertSkills()
		r.InvalidateAll()
		assertSkills("alpha")
		if err := os.Remove(filepath.Join(dir, skillDefinitionFile)); err != nil {
			t.Fatal(err)
		}
		now = now.Add(2 * time.Second)
		assertSkills("alpha")
		r.ReleaseSnapshots()
		assertSkills()
	})
	t.Run("Should isolate cached config and invalidate every watched input", func(t *testing.T) {
		t.Parallel()
		runResolveCacheHitInvalidateAndEviction(t)
	})

	t.Run("Should immediately refresh skill identities discovered after warming an empty source", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		home := newTestHomePaths(t)
		ws := Workspace{ID: "ws_skill_cache", RootDir: root, Name: "repo"}
		resolver := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home))
		assertNames := func(want ...string) {
			t.Helper()
			resolved, err := resolver.Resolve(t.Context(), ws.ID)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got := skillNames(resolved.Skills); !slices.Equal(got, want) {
				t.Fatalf("Resolve().Skills names = %#v, want %#v", got, want)
			}
		}
		assertNames()
		source := filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName)
		nested := filepath.Join(source, "group", "empty")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		invalid := filepath.Join(source, "broken", skillDefinitionFile)
		writeFile(t, invalid, "invalid skill")
		assertNames()
		assertNames()

		added := filepath.Join(nested, "added")
		writeSkill(t, added)
		assertNames("added")
		assertNames("added")
		writeFile(t, filepath.Join(added, skillDefinitionFile), "---\nname: renamed\ndescription: test\n---\nBody.\n")
		assertNames("renamed")
		writeSkill(t, filepath.Dir(invalid))
		assertNames("broken", "renamed")
		if err := os.Remove(filepath.Join(added, skillDefinitionFile)); err != nil {
			t.Fatal(err)
		}
		assertNames("broken")
	})

	t.Run("Should refresh skill names after an atomic replacement preserves file metadata", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		home := newTestHomePaths(t)
		ws := Workspace{ID: "ws_skill_replacement", RootDir: root, Name: "repo"}
		skillDir := filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName, "alpha")
		writeSkill(t, skillDir)
		resolver := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home))
		if _, err := resolver.Resolve(t.Context(), ws.ID); err != nil {
			t.Fatal(err)
		}
		definition := filepath.Join(skillDir, skillDefinitionFile)
		original, err := os.Stat(definition)
		if err != nil {
			t.Fatal(err)
		}
		directory, err := os.Stat(skillDir)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(definition)
		if err != nil {
			t.Fatal(err)
		}
		replacement := filepath.Join(t.TempDir(), skillDefinitionFile)
		writeFile(t, replacement, strings.Replace(string(content), "name: alpha", "name: omega", 1))
		if err := os.Chtimes(replacement, original.ModTime(), original.ModTime()); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, definition); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(skillDir, directory.ModTime(), directory.ModTime()); err != nil {
			t.Fatal(err)
		}
		resolved, err := resolver.Resolve(t.Context(), ws.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := skillNames(resolved.Skills), []string{"omega"}; !slices.Equal(got, want) {
			t.Fatalf("Resolve(replacement).Skills = %#v, want %#v", got, want)
		}
	})

	for _, test := range []struct {
		name       string
		validFirst bool
	}{
		{
			name: "Should immediately discover a repaired skill when in-place edits preserve file metadata",
		},
		{
			name:       "Should immediately exclude an invalidated skill when in-place edits preserve file metadata",
			validFirst: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			home := newTestHomePaths(t)
			ws := Workspace{ID: "ws_skill_validity", RootDir: root, Name: "repo"}
			skillDir := filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName, "alpha")
			writeSkill(t, skillDir)
			definition := filepath.Join(skillDir, skillDefinitionFile)
			validContent, err := os.ReadFile(definition)
			if err != nil {
				t.Fatalf("ReadFile(skill definition) error = %v", err)
			}
			invalidContent := strings.Replace(string(validContent), "name: alpha", "nope: alpha", 1)
			var initialNames []string
			if test.validFirst {
				initialNames = []string{"alpha"}
			} else {
				writeFile(t, definition, invalidContent)
			}
			resolver := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home),
				withNow(func() time.Time { return time.Unix(1_700_060_000, 0).UTC() }),
			)
			initial, err := resolver.Resolve(t.Context(), ws.ID)
			if err != nil {
				t.Fatalf("Resolve(before validity change) error = %v", err)
			}
			if got := skillNames(initial.Skills); !slices.Equal(got, initialNames) {
				t.Fatalf("initial skill names = %#v, want %#v", got, initialNames)
			}
			before, err := os.Stat(definition)
			if err != nil {
				t.Fatalf("Stat(before skill edit) error = %v", err)
			}
			updatedContent, wantNames := string(validContent), []string{"alpha"}
			if test.validFirst {
				updatedContent, wantNames = invalidContent, nil
			}
			writeFile(t, definition, updatedContent)
			touchPath(t, definition, before.ModTime())
			after, err := os.Stat(definition)
			if err != nil {
				t.Fatalf("Stat(after skill edit) error = %v", err)
			}
			if !os.SameFile(before, after) || before.Size() != after.Size() || before.Mode() != after.Mode() ||
				!before.ModTime().Equal(after.ModTime()) {
				t.Fatal("skill edit must preserve file identity, size, mode, and modification time")
			}
			current, err := resolver.Resolve(t.Context(), ws.ID)
			if err != nil {
				t.Fatalf("Resolve(after validity change) error = %v", err)
			}
			if got := skillNames(current.Skills); !slices.Equal(got, wantNames) {
				t.Fatalf("skill names after validity change = %#v, want %#v", got, wantNames)
			}
		})
	}

	t.Run("Should discard skill discovery with its expired workspace cache entry", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		home := newTestHomePaths(t)
		ws := Workspace{ID: "ws_skill_expiration", RootDir: root, Name: "repo"}
		skillDir := filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName, "alpha")
		writeSkill(t, skillDir)
		currentTime := time.Unix(1_700_000_000, 0)
		resolver := newTestResolver(t, newMockWorkspaceStore(ws), WithHomePaths(home),
			WithCacheTTL(time.Minute), withNow(func() time.Time { return currentTime }),
		)
		if _, err := resolver.Resolve(t.Context(), ws.ID); err != nil {
			t.Fatal(err)
		}
		definition := filepath.Join(skillDir, skillDefinitionFile)
		original, err := os.Stat(definition)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(definition)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, definition, strings.Replace(string(content), "name: alpha", "name: omega", 1))
		if err := os.Chtimes(definition, original.ModTime(), original.ModTime()); err != nil {
			t.Fatal(err)
		}
		currentTime = currentTime.Add(time.Minute + time.Second)
		resolved, err := resolver.Resolve(t.Context(), ws.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := skillNames(resolved.Skills), []string{"omega"}; !slices.Equal(got, want) {
			t.Fatalf("Resolve(expired).Skills = %#v, want %#v", got, want)
		}
	})
}
