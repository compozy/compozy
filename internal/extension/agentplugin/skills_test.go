package agentplugin

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadSkills(t *testing.T) {
	t.Parallel()

	t.Run("Should discover only immediate child skills", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "skill-depth")
		writeFile(t, filepath.Join(root, "skills", "review", "SKILL.md"), validSkill("review", "Review code."))
		writeFile(
			t,
			filepath.Join(root, "skills", "group", "nested", "SKILL.md"),
			validSkill("nested", "Nested skill."),
		)
		pkg := loadForTest(t, root)
		if got, want := skillNames(pkg.Skills), []string{"review"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() skill names = %#v, want %#v", got, want)
		}
	})

	t.Run("Should skip a mismatched skill name while preserving its sibling", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "skill-mismatch")
		writeFile(t, filepath.Join(root, "skills", "deploy", "SKILL.md"), validSkill("other-name", "Deploy code."))
		writeFile(t, filepath.Join(root, "skills", "review", "SKILL.md"), validSkill("review", "Review code."))
		pkg := loadForTest(t, root)
		if got, want := skillNames(pkg.Skills), []string{"review"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() skill names = %#v, want %#v", got, want)
		}
		if !hasDiagnostic(pkg.Diagnostics, "skill:deploy", "name must match the directory") {
			t.Fatalf("Load() diagnostics = %#v, want deploy name mismatch", pkg.Diagnostics)
		}
	})

	t.Run("Should distinguish malformed frontmatter from a missing description", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "skill-frontmatter")
		writeFile(t, filepath.Join(root, "skills", "broken", "SKILL.md"), []byte("---\nname: broken\n"))
		writeFile(t, filepath.Join(root, "skills", "silent", "SKILL.md"), []byte("---\nname: silent\n---\nBody\n"))
		pkg := loadForTest(t, root)
		if !hasDiagnostic(pkg.Diagnostics, "skill:broken", "unterminated") {
			t.Fatalf("Load() diagnostics = %#v, want unterminated frontmatter", pkg.Diagnostics)
		}
		if !hasDiagnostic(pkg.Diagnostics, "skill:silent", "description is required") {
			t.Fatalf("Load() diagnostics = %#v, want missing description", pkg.Diagnostics)
		}
	})

	t.Run("Should reject a regular file at the skills path without blocking MCP", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "skills-file")
		writeFile(t, filepath.Join(root, "skills"), []byte("not a directory"))
		writeMCPFile(t, root, map[string]any{
			"remote": map[string]any{"type": "streamable-http", "url": "https://example.com/mcp"},
		})
		pkg := loadForTest(t, root)
		if !hasDiagnostic(pkg.Diagnostics, "skills", "skills must be an in-root directory") {
			t.Fatalf("Load() diagnostics = %#v, want skills kind diagnostic", pkg.Diagnostics)
		}
		if len(pkg.Servers) != 1 || pkg.Servers[0].Name != "remote" {
			t.Fatalf("Load() servers = %#v, want remote server", pkg.Servers)
		}
	})

	t.Run("Should enforce Agent Skills metadata bounds", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "skill-bounds")
		writeFile(t, filepath.Join(root, "skills", "long", "SKILL.md"), validSkill("long", strings.Repeat("x", 1025)))
		writeFile(t, filepath.Join(root, "skills", "valid", "SKILL.md"), validSkill("valid", "Valid skill."))
		pkg := loadForTest(t, root)
		if got, want := skillNames(pkg.Skills), []string{"valid"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() skill names = %#v, want %#v", got, want)
		}
		if !hasDiagnostic(pkg.Diagnostics, "skill:long", "1024") {
			t.Fatalf("Load() diagnostics = %#v, want description bound", pkg.Diagnostics)
		}
	})

	t.Run("Should ignore a symlinked SKILL file", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "skill-symlink")
		target := filepath.Join(t.TempDir(), "SKILL.md")
		writeFile(t, target, validSkill("linked", "Linked skill."))
		dir := filepath.Join(root, "skills", "linked")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "SKILL.md")); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}
		pkg := loadForTest(t, root)
		if len(pkg.Skills) != 0 {
			t.Fatalf("Load() skills = %#v, want none", pkg.Skills)
		}
		if !hasDiagnostic(pkg.Diagnostics, "skill:linked", "regular in-root file") {
			t.Fatalf("Load() diagnostics = %#v, want linked skill rejection", pkg.Diagnostics)
		}
	})

	t.Run("Should diagnose malformed immediate entries and ignore grouping directories", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "skill-immediate-entries")
		writeFile(t, filepath.Join(root, "skills", "README.md"), []byte("not a skill"))
		writeFile(
			t,
			filepath.Join(root, "skills", "group", "nested", "SKILL.md"),
			validSkill("nested", "Nested skill."),
		)
		pkg := loadForTest(t, root)
		if !hasDiagnostic(pkg.Diagnostics, "skill:README.md", "in-root directory") {
			t.Fatalf("Load() diagnostics = %#v, want immediate file rejection", pkg.Diagnostics)
		}
		if hasDiagnostic(pkg.Diagnostics, "skill:group", "SKILL.md") {
			t.Fatalf("Load() diagnostics = %#v, want grouping directory ignored", pkg.Diagnostics)
		}
	})
}
