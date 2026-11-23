package agentinstance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Invariant: the v1 tree digest is a length-prefixed manifest of every regular file's path,
// permission bits, size and content hash, so distinct trees never share a digest.
// Owning layer: agentinstance tree digest; canonical suite: TestSkillTreeDigest.
func TestSkillTreeDigest(t *testing.T) {
	writeTree := func(t *testing.T, files map[string]string, order []string) string {
		t.Helper()
		dir := t.TempDir()
		for _, path := range order {
			full := filepath.Join(dir, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(files[path]), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	digest := func(t *testing.T, dir string) string {
		t.Helper()
		value, err := SkillTreeDigest(dir)
		if err != nil {
			t.Fatalf("SkillTreeDigest() = %v", err)
		}
		return value
	}
	files := map[string]string{"SKILL.md": "body", "scripts/run.sh": "echo run", "z.txt": "z"}

	t.Run(
		"Should stay stable across creation order and change with any byte, path or mode [UT-039]",
		func(t *testing.T) {
			t.Parallel()
			base := digest(t, writeTree(t, files, []string{"SKILL.md", "scripts/run.sh", "z.txt"}))
			if again := digest(t, writeTree(t, files, []string{"z.txt", "scripts/run.sh", "SKILL.md"})); again != base {
				t.Fatalf("creation order changed the digest: %s != %s", again, base)
			}
			changedByte := writeTree(t, files, []string{"SKILL.md", "scripts/run.sh", "z.txt"})
			if err := os.WriteFile(filepath.Join(changedByte, "z.txt"), []byte("y"), 0o600); err != nil {
				t.Fatal(err)
			}
			renamed := writeTree(t, map[string]string{"SKILL.md": "body", "scripts/run.sh": "echo run", "y.txt": "z"},
				[]string{"SKILL.md", "scripts/run.sh", "y.txt"})
			chmodded := writeTree(t, files, []string{"SKILL.md", "scripts/run.sh", "z.txt"})
			if err := os.Chmod(filepath.Join(chmodded, "scripts", "run.sh"), 0o700); err != nil {
				t.Fatal(err)
			}
			for name, dir := range map[string]string{"byte": changedByte, "path": renamed, "mode": chmodded} {
				if digest(t, dir) == base {
					t.Fatalf("a changed %s kept the digest", name)
				}
			}
		},
	)
	t.Run("Should refuse a symlink in the tree [UT-039]", func(t *testing.T) {
		t.Parallel()
		dir := writeTree(t, files, []string{"SKILL.md"})
		if err := os.Symlink(filepath.Join(dir, "SKILL.md"), filepath.Join(dir, "link.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := SkillTreeDigest(dir); !errors.Is(err, ErrSkillTreeUnsupported) {
			t.Fatalf("SkillTreeDigest() = %v, want ErrSkillTreeUnsupported", err)
		}
	})
	t.Run("Should separate a NUL-joined file from two files [UT-048]", func(t *testing.T) {
		t.Parallel()
		joined := writeTree(
			t,
			map[string]string{"SKILL.md": "s", "a.txt": "x\x00b.txt\x00y"},
			[]string{"SKILL.md", "a.txt"},
		)
		split := writeTree(t, map[string]string{"SKILL.md": "s", "a.txt": "x", "b.txt": "y"},
			[]string{"SKILL.md", "a.txt", "b.txt"})
		if digest(t, joined) == digest(t, split) {
			t.Fatal("distinct trees share a digest")
		}
	})
	t.Run("Should copy a tree byte- and permission-exact", func(t *testing.T) {
		t.Parallel()
		src := writeTree(t, files, []string{"SKILL.md", "scripts/run.sh", "z.txt"})
		if err := os.Chmod(filepath.Join(src, "scripts", "run.sh"), 0o755); err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(t.TempDir(), "copy")
		if err := copySkillTree(src, dst); err != nil {
			t.Fatalf("copySkillTree() = %v", err)
		}
		if digest(t, dst) != digest(t, src) {
			t.Fatal("copy differs from its source")
		}
	})
}
