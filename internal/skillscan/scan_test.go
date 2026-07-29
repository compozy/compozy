package skillscan

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"
)

func TestScanDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should discover nested definitions without a depth limit", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		expected := []string{
			writeDefinition(t, root, filepath.Join("marketing", "brand", "campaign", "email", "launch", "SKILL.md")),
			writeDefinition(t, root, filepath.Join("product", "SKILL.md")),
			writeDefinition(t, root, filepath.Join(".compozy", "agents", "shared", "SKILL.md")),
		}
		writeDefinition(t, root, filepath.Join(".hidden", "SKILL.md"))
		writeDefinition(t, root, filepath.Join(".git", "ignored", "SKILL.md"))
		writeDefinition(t, root, filepath.Join("node_modules", "package", "SKILL.md"))

		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		slices.Sort(expected)
		if !reflect.DeepEqual(result.Paths, expected) {
			t.Fatalf("ScanDirectory().Paths = %#v, want %#v", result.Paths, expected)
		}
		if len(result.Snapshots) != len(expected) {
			t.Fatalf("len(ScanDirectory().Snapshots) = %d, want %d", len(result.Snapshots), len(expected))
		}
	})

	t.Run("Should cap candidates deterministically", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		for index := range MaxCandidates + 5 {
			writeDefinition(t, root, filepath.Join(fmt.Sprintf("skill-%03d", index), SkillFileName))
		}

		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		if len(result.Paths) != MaxCandidates {
			t.Fatalf("len(ScanDirectory().Paths) = %d, want %d", len(result.Paths), MaxCandidates)
		}
		if got, want := filepath.Base(filepath.Dir(result.Paths[0])), "skill-000"; got != want {
			t.Fatalf("first discovered definition = %q, want %q", got, want)
		}
		if got, want := filepath.Base(filepath.Dir(result.Paths[len(result.Paths)-1])), "skill-299"; got != want {
			t.Fatalf("last discovered definition = %q, want %q", got, want)
		}
	})

	t.Run("Should return empty results for a missing root", func(t *testing.T) {
		t.Parallel()

		result, err := ScanDirectory(filepath.Join(t.TempDir(), "missing"))
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		if len(result.Paths) != 0 || len(result.Snapshots) != 0 {
			t.Fatalf("ScanDirectory() = %#v, want empty result", result)
		}
	})

	t.Run("Should reject invalid directory roots", func(t *testing.T) {
		t.Parallel()

		if _, err := ScanDirectory("   "); err == nil {
			t.Fatal("ScanDirectory() error = nil, want blank root error")
		}
		filePath := filepath.Join(t.TempDir(), SkillFileName)
		if err := os.WriteFile(filePath, []byte("definition"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", filePath, err)
		}
		if _, err := ScanDirectory(filePath); err == nil {
			t.Fatal("ScanDirectory() error = nil, want file root error")
		}
	})

	t.Run("Should enforce symlink containment without traversing directories", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		outside := t.TempDir()
		insideTarget := writeDefinition(t, root, filepath.Join("inside", SkillFileName))
		insideLink := filepath.Join(root, "linked", SkillFileName)
		if err := os.MkdirAll(filepath.Dir(insideLink), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(insideLink), err)
		}
		if err := os.Symlink(insideTarget, insideLink); err != nil {
			t.Skipf("Symlink(%q, %q) unavailable: %v", insideTarget, insideLink, err)
		}

		escapingTarget := writeDefinition(t, outside, SkillFileName)
		escapingLink := filepath.Join(root, "escaped", SkillFileName)
		if err := os.MkdirAll(filepath.Dir(escapingLink), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(escapingLink), err)
		}
		if err := os.Symlink(escapingTarget, escapingLink); err != nil {
			t.Skipf("Symlink(%q, %q) unavailable: %v", escapingTarget, escapingLink, err)
		}

		directoryTarget := filepath.Join(outside, "directory-target")
		writeDefinition(t, directoryTarget, SkillFileName)
		directoryLink := filepath.Join(root, "directory-link")
		if err := os.Symlink(directoryTarget, directoryLink); err != nil {
			t.Skipf("Symlink(%q, %q) unavailable: %v", directoryTarget, directoryLink, err)
		}

		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		want := []string{insideTarget, insideLink}
		slices.Sort(want)
		if !reflect.DeepEqual(result.Paths, want) {
			t.Fatalf("ScanDirectory().Paths = %#v, want %#v", result.Paths, want)
		}
	})
}

func TestScanFS(t *testing.T) {
	t.Parallel()

	t.Run("Should discover nested definitions from an fs source", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"marketing/brand/launch/email/brief/approval/SKILL.md": &fstest.MapFile{Data: []byte("definition")},
			"product/SKILL.md": &fstest.MapFile{Data: []byte("definition")},
			".hidden/SKILL.md": &fstest.MapFile{Data: []byte("definition")},
		}
		got, err := ScanFS(fsys, ".")
		if err != nil {
			t.Fatalf("ScanFS() error = %v", err)
		}
		want := []string{"marketing/brand/launch/email/brief/approval/SKILL.md", "product/SKILL.md"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ScanFS() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should cap filesystem candidates deterministically", func(t *testing.T) {
		t.Parallel()

		fsys := make(fstest.MapFS, MaxCandidates+5)
		for index := range MaxCandidates + 5 {
			fsys[fmt.Sprintf("skill-%03d/%s", index, SkillFileName)] = &fstest.MapFile{Data: []byte("definition")}
		}

		got, err := ScanFS(fsys, ".")
		if err != nil {
			t.Fatalf("ScanFS() error = %v", err)
		}
		if len(got) != MaxCandidates {
			t.Fatalf("len(ScanFS()) = %d, want %d", len(got), MaxCandidates)
		}
		if first, want := path.Base(path.Dir(got[0])), "skill-000"; first != want {
			t.Fatalf("first filesystem definition = %q, want %q", first, want)
		}
		if last, want := path.Base(path.Dir(got[len(got)-1])), "skill-299"; last != want {
			t.Fatalf("last filesystem definition = %q, want %q", last, want)
		}
	})

	t.Run("Should return empty results for a missing filesystem root", func(t *testing.T) {
		t.Parallel()

		got, err := ScanFS(fstest.MapFS{}, "missing")
		if err != nil {
			t.Fatalf("ScanFS() error = %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("ScanFS() = %#v, want empty result", got)
		}
	})

	t.Run("Should reject a nil filesystem", func(t *testing.T) {
		t.Parallel()

		if _, err := ScanFS(nil, "."); err == nil {
			t.Fatal("ScanFS() error = nil, want nil filesystem error")
		}
	})
}

func writeDefinition(t *testing.T, root string, relativePath string) string {
	t.Helper()

	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("definition"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
