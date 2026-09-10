package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestResolveCacheHitInvalidateAndEviction(t *testing.T) {
	t.Parallel()
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
