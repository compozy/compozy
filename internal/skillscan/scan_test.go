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
	"time"
)

func TestDirectoryResultUnchanged(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		directory bool
		linked    bool
	}{
		{name: "Should detect new definition membership when directory metadata is preserved"},
		{
			name:      "Should detect a new subtree at the same entry name when directory metadata is preserved",
			directory: true,
		},
		{name: "Should detect new linked definition membership when directory metadata is preserved", linked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			nested := filepath.Join(root, "empty", "nested")
			reportedNested := nested
			trusted := []string{root}
			if tc.linked {
				target := t.TempDir()
				link := filepath.Join(root, "linked")
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("Symlink unavailable: %v", err)
				}
				nested = filepath.Join(target, "empty", "nested")
				reportedNested = filepath.Join(link, "empty", "nested")
				trusted = append(trusted, target)
			}
			placeholder := writeDefinition(t, nested, "NOTES.md")
			before, err := os.Stat(nested)
			if err != nil {
				t.Fatal(err)
			}
			result, err := ScanDirectoryWithin(root, trusted)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Paths) != 0 || !result.Unchanged(t.Context(), root, trusted) {
				t.Fatalf("initial discovery = %#v, want empty reusable projection", result.Paths)
			}
			definition := filepath.Join(nested, SkillFileName)
			if tc.directory {
				if err := os.Remove(placeholder); err != nil {
					t.Fatal(err)
				}
				writeDefinition(t, placeholder, SkillFileName)
			} else if err := os.Rename(placeholder, definition); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(nested, before.ModTime(), before.ModTime()); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(nested)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() ||
				!before.ModTime().Equal(after.ModTime()) {
				t.Fatal("fixture did not preserve directory identity, mode, size and modification time")
			}
			if result.Unchanged(t.Context(), root, trusted) {
				t.Fatal("new discovery with preserved directory metadata was reported unchanged")
			}
			current, err := ScanDirectoryWithin(root, trusted)
			if err != nil {
				t.Fatal(err)
			}
			definition = filepath.Join(reportedNested, SkillFileName)
			if tc.directory {
				definition = filepath.Join(reportedNested, "NOTES.md", SkillFileName)
			}
			if want := []string{definition}; !slices.Equal(current.Paths, want) {
				t.Fatalf("new discovery = %#v, want %#v", current.Paths, want)
			}
		})
	}

	t.Run("Should discard conflicting discovery from overlapping linked roots", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		definition := writeDefinition(t, root, filepath.Join("real", SkillFileName))
		if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "linked")); err != nil {
			t.Skipf("Symlink unavailable: %v", err)
		}
		scanner, err := newDirectoryScanner(root, []string{root})
		if err != nil {
			t.Fatal(err)
		}
		if err := scanner.scanBase(); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(definition); err != nil {
			t.Fatal(err)
		}
		writeDefinition(t, definition, SkillFileName)
		if err := scanner.followFirstLevelLinks(); err != nil {
			t.Fatal(err)
		}
		if scanner.result.Unchanged(t.Context(), root, []string{root}) {
			t.Fatal("conflicting observations of a linked directory produced reusable discovery")
		}
	})

	for _, tc := range []struct {
		name   string
		change func(*testing.T, string, string)
	}{
		{
			name: "Should detect a definition in a previously empty nested directory",
			change: func(t *testing.T, root, _ string) {
				t.Helper()
				writeDefinition(t, root, filepath.Join("empty", "nested", "new", SkillFileName))
			},
		},
		{
			name: "Should detect definition edits",
			change: func(t *testing.T, _, definition string) {
				t.Helper()
				if err := os.WriteFile(definition, []byte("changed definition content"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "Should detect definition removal",
			change: func(t *testing.T, _, definition string) {
				t.Helper()
				if err := os.Remove(definition); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "Should detect definition renames",
			change: func(t *testing.T, _, definition string) {
				t.Helper()
				if err := os.Rename(definition, definition+".disabled"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "Should detect replacement with the same size and modification time",
			change: func(t *testing.T, _, definition string) {
				t.Helper()
				info, err := os.Stat(definition)
				if err != nil {
					t.Fatal(err)
				}
				replacement := filepath.Join(t.TempDir(), SkillFileName)
				if err := os.WriteFile(replacement, make([]byte, info.Size()), info.Mode()); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, definition); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			definition := writeDefinition(t, root, filepath.Join("original", SkillFileName))
			if err := os.MkdirAll(filepath.Join(root, "empty", "nested"), 0o755); err != nil {
				t.Fatal(err)
			}
			result, err := ScanDirectory(root)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Unchanged(t.Context(), root, []string{root}) {
				t.Fatal("fresh complete discovery must be reusable")
			}
			tc.change(t, root, definition)
			if result.Unchanged(t.Context(), root, []string{root}) {
				t.Fatal("changed discovery was reported unchanged")
			}
		})
	}

	t.Run(
		"Should revalidate stable rejected links and detect changed containment through an intermediate link",
		func(t *testing.T) {
			t.Parallel()

			root, allowed, outside := t.TempDir(), t.TempDir(), t.TempDir()
			writeDefinition(t, allowed, SkillFileName)
			writeDefinition(t, outside, SkillFileName)
			intermediate := filepath.Join(t.TempDir(), "target")
			if err := os.Symlink(outside, intermediate); err != nil {
				t.Skipf("Symlink unavailable: %v", err)
			}
			for index := range 5 {
				if err := os.Symlink(intermediate, filepath.Join(root, fmt.Sprintf("linked-%d", index))); err != nil {
					t.Fatal(err)
				}
			}
			trusted := []string{root, allowed}
			for _, target := range []string{outside, allowed, outside} {
				if err := os.Remove(intermediate); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, intermediate); err != nil {
					t.Fatal(err)
				}
				result, err := ScanDirectoryWithin(root, trusted)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := len(result.Paths), 0; target == outside && got != want {
					t.Fatalf("outside projection has %d paths, want %d", got, want)
				}
				if target == allowed && len(result.Paths) != 1 {
					t.Fatalf("allowed projection = %#v, want one deduplicated definition", result.Paths)
				}
				if !result.Unchanged(t.Context(), root, trusted) {
					t.Fatal("stable link discovery must be reusable")
				}
				if err := os.Remove(intermediate); err != nil {
					t.Fatal(err)
				}
				other := allowed
				if target == allowed {
					other = outside
				}
				if err := os.Symlink(other, intermediate); err != nil {
					t.Fatal(err)
				}
				if result.Unchanged(t.Context(), root, trusted) {
					t.Fatal("changed link containment was reported unchanged")
				}
			}
		},
	)

	t.Run("Should rescan when a missing root appears", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "missing")
		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatal(err)
		}
		writeDefinition(t, root, filepath.Join("added", SkillFileName))
		if result.Unchanged(t.Context(), root, []string{root}) {
			t.Fatal("new root was reported unchanged")
		}
	})

	for _, fileTarget := range []bool{false, true} {
		name := "Should discover a dangling first-level link when its target appears"
		if fileTarget {
			name = "Should discover a first-level link whose file target becomes a directory"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root, trustedRoot := t.TempDir(), t.TempDir()
			target := filepath.Join(trustedRoot, "target")
			if fileTarget {
				if err := os.WriteFile(target, []byte("plain file"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			link := filepath.Join(root, "linked")
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("Symlink unavailable: %v", err)
			}
			trusted := []string{root, trustedRoot}
			result, err := ScanDirectoryWithin(root, trusted)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Paths) != 0 || !result.Unchanged(t.Context(), root, trusted) {
				t.Fatalf("initial discovery = %#v, want empty reusable projection", result.Paths)
			}
			if fileTarget {
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
			}
			writeDefinition(t, target, SkillFileName)
			if result.Unchanged(t.Context(), root, trusted) {
				t.Fatal("new directory target was reported unchanged")
			}
			current, err := ScanDirectoryWithin(root, trusted)
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{filepath.Join(link, SkillFileName)}; !slices.Equal(current.Paths, want) {
				t.Fatalf("new link projection = %#v, want %#v", current.Paths, want)
			}
		})
	}

	t.Run("Should detect permission changes even when file content metadata stays the same", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		definition := writeDefinition(t, root, filepath.Join("original", SkillFileName))
		stamp := time.Unix(1_700_000_000, 0)
		if err := os.Chtimes(definition, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(definition, 0o444); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(definition, 0o644); err != nil {
				t.Errorf("restore definition permissions: %v", err)
			}
		})
		if result.Unchanged(t.Context(), root, []string{root}) {
			t.Fatal("changed file permissions were reported unchanged")
		}
	})
}

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

	t.Run("Should discover definitions below a symlinked root", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		targetRoot := filepath.Join(parent, "target")
		writeDefinition(t, targetRoot, filepath.Join("nested", SkillFileName))
		linkedRoot := filepath.Join(parent, "linked")
		if err := os.Symlink(targetRoot, linkedRoot); err != nil {
			t.Skipf("Symlink(%q, %q) unavailable: %v", targetRoot, linkedRoot, err)
		}

		result, err := ScanDirectory(linkedRoot)
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		definition := filepath.Join(linkedRoot, "nested", SkillFileName)
		if !reflect.DeepEqual(result.Paths, []string{definition}) {
			t.Fatalf("ScanDirectory().Paths = %#v, want %#v", result.Paths, []string{definition})
		}
		if _, ok := result.Snapshots[definition]; !ok {
			t.Fatalf("ScanDirectory().Snapshots missing %q", definition)
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
		if !result.Stats.Truncated || result.Stats.ScannedCount != MaxCandidates {
			t.Fatalf("ScanDirectory().Stats = %#v, want truncated count %d", result.Stats, MaxCandidates)
		}
		if result.Unchanged(t.Context(), root, []string{root}) {
			t.Fatal("truncated discovery was reported reusable")
		}
		if got, want := filepath.Base(filepath.Dir(result.Paths[0])), "skill-000"; got != want {
			t.Fatalf("first discovered definition = %q, want %q", got, want)
		}
		if got, want := filepath.Base(filepath.Dir(result.Paths[len(result.Paths)-1])), "skill-299"; got != want {
			t.Fatalf("last discovered definition = %q, want %q", got, want)
		}
	})

	t.Run("Should share candidate limits with followed first-level links", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		linkedRoot := t.TempDir()
		for index := range 150 {
			writeDefinition(t, root, filepath.Join(fmt.Sprintf("base-%03d", index), SkillFileName))
		}
		for index := range 200 {
			writeDefinition(t, linkedRoot, filepath.Join(fmt.Sprintf("linked-%03d", index), SkillFileName))
		}
		linkPath := filepath.Join(root, "linked")
		if err := os.Symlink(linkedRoot, linkPath); err != nil {
			t.Skipf("Symlink(%q, %q) unavailable: %v", linkedRoot, linkPath, err)
		}

		result, err := ScanDirectoryWithin(root, []string{root, linkedRoot})
		if err != nil {
			t.Fatalf("ScanDirectoryWithin() error = %v", err)
		}
		if got := result.Stats.ScannedCount; got != MaxCandidates {
			t.Fatalf("ScanDirectoryWithin().Stats.ScannedCount = %d, want shared cap %d", got, MaxCandidates)
		}
		if !result.Stats.Truncated || len(result.Paths) != MaxCandidates {
			t.Fatalf("ScanDirectoryWithin() = %#v, want %d paths and truncated state", result.Stats, MaxCandidates)
		}
	})

	t.Run("Should stop traversing after the filesystem entry limit", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		first := writeDefinition(t, root, filepath.Join("aaaa-skill", SkillFileName))
		for index := range MaxEntries + 5 {
			junkPath := filepath.Join(root, fmt.Sprintf("junk-%05d.txt", index))
			if err := os.WriteFile(junkPath, []byte("junk"), 0o644); err != nil {
				t.Fatalf("WriteFile(%q) error = %v", junkPath, err)
			}
		}
		writeDefinition(t, root, filepath.Join("zzzz-skill", SkillFileName))

		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		if !reflect.DeepEqual(result.Paths, []string{first}) {
			t.Fatalf("ScanDirectory().Paths = %#v, want only definition before traversal limit", result.Paths)
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

	t.Run("Should report a permission-denied root as unreadable", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeDefinition(t, root, filepath.Join("private", SkillFileName))
		if err := os.Chmod(root, 0); err != nil {
			t.Fatalf("Chmod(%q) error = %v", root, err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(root, 0o755); err != nil {
				t.Fatalf("restore Chmod(%q) error = %v", root, err)
			}
		})

		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		if result.Stats.Readable {
			t.Skip("current process can read mode-000 directories")
		}
		if !result.Stats.Exists || result.Stats.ScannedCount != 0 || len(result.Paths) != 0 {
			t.Fatalf("ScanDirectory().Stats = %#v, want existing unreadable root with omitted discovery", result.Stats)
		}
		if result.Unchanged(t.Context(), root, []string{root}) {
			t.Fatal("unreadable discovery was reported reusable")
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

	t.Run("Should enforce symlink containment and deduplicate realpath aliases", func(t *testing.T) {
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
		want := []string{insideTarget}
		if !reflect.DeepEqual(result.Paths, want) {
			t.Fatalf("ScanDirectory().Paths = %#v, want %#v", result.Paths, want)
		}
		if result.Stats.ScannedCount != 2 {
			t.Fatalf("ScanDirectory().Stats.ScannedCount = %d, want 2 pre-dedup candidates", result.Stats.ScannedCount)
		}
		if !slices.Contains(result.Stats.SkippedLinks, SkippedLink{Path: directoryLink, Reason: "escape"}) {
			t.Fatalf("ScanDirectory().Stats.SkippedLinks = %#v, want directory escape", result.Stats.SkippedLinks)
		}
	})

	t.Run("Should follow only first-level directory links inside the projection", func(t *testing.T) {
		t.Parallel()

		projectionRoot := t.TempDir()
		trustedRoot := t.TempDir()
		writeDefinition(t, trustedRoot, SkillFileName)
		linkPath := filepath.Join(projectionRoot, "linked-skill")
		if err := os.Symlink(trustedRoot, linkPath); err != nil {
			t.Skipf("Symlink(%q, %q) unavailable: %v", trustedRoot, linkPath, err)
		}

		blocked, err := ScanDirectoryWithin(projectionRoot, []string{projectionRoot})
		if err != nil {
			t.Fatalf("ScanDirectoryWithin(blocked) error = %v", err)
		}
		if len(blocked.Paths) != 0 || !slices.Contains(
			blocked.Stats.SkippedLinks,
			SkippedLink{Path: linkPath, Reason: "escape"},
		) {
			t.Fatalf("ScanDirectoryWithin(blocked) = %#v, want escape", blocked)
		}

		allowed, err := ScanDirectoryWithin(projectionRoot, []string{projectionRoot, trustedRoot})
		if err != nil {
			t.Fatalf("ScanDirectoryWithin(allowed) error = %v", err)
		}
		want := filepath.Join(linkPath, SkillFileName)
		if !reflect.DeepEqual(allowed.Paths, []string{want}) || allowed.RealPaths[want] == "" {
			t.Fatalf("ScanDirectoryWithin(allowed) = %#v, want linked candidate %q", allowed, want)
		}

		nested := filepath.Join(projectionRoot, "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", nested, err)
		}
		if err := os.Symlink(trustedRoot, filepath.Join(nested, "deeper-link")); err != nil {
			t.Skipf("nested Symlink unavailable: %v", err)
		}
		if err := os.Remove(linkPath); err != nil {
			t.Fatalf("Remove(%q) error = %v", linkPath, err)
		}
		deeperOnly, err := ScanDirectoryWithin(projectionRoot, []string{projectionRoot, trustedRoot})
		if err != nil {
			t.Fatalf("ScanDirectoryWithin(deeper only) error = %v", err)
		}
		if len(deeperOnly.Paths) != 0 {
			t.Fatalf("ScanDirectoryWithin(deeper only).Paths = %#v, want no nested-link traversal", deeperOnly.Paths)
		}
	})

	t.Run("Should classify dangling and cyclic first-level links", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		dangling := filepath.Join(root, "dangling")
		if err := os.Symlink(filepath.Join(root, "missing"), dangling); err != nil {
			t.Skipf("dangling Symlink unavailable: %v", err)
		}
		cycleA := filepath.Join(root, "cycle-a")
		cycleB := filepath.Join(root, "cycle-b")
		if err := os.Symlink(cycleB, cycleA); err != nil {
			t.Skipf("cycle Symlink A unavailable: %v", err)
		}
		if err := os.Symlink(cycleA, cycleB); err != nil {
			t.Skipf("cycle Symlink B unavailable: %v", err)
		}

		result, err := ScanDirectory(root)
		if err != nil {
			t.Fatalf("ScanDirectory() error = %v", err)
		}
		for _, want := range []SkippedLink{
			{Path: dangling, Reason: "dangling"},
			{Path: cycleA, Reason: "cycle"},
			{Path: cycleB, Reason: "cycle"},
		} {
			if !slices.Contains(result.Stats.SkippedLinks, want) {
				t.Fatalf("ScanDirectory().Stats.SkippedLinks = %#v, want %#v", result.Stats.SkippedLinks, want)
			}
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

	t.Run("Should stop filesystem traversal after the entry limit", func(t *testing.T) {
		t.Parallel()

		fsys := make(fstest.MapFS, MaxEntries+7)
		fsys["aaaa-skill/SKILL.md"] = &fstest.MapFile{Data: []byte("definition")}
		for index := range MaxEntries + 5 {
			fsys[fmt.Sprintf("junk-%05d.txt", index)] = &fstest.MapFile{Data: []byte("junk")}
		}
		fsys["zzzz-skill/SKILL.md"] = &fstest.MapFile{Data: []byte("definition")}

		got, err := ScanFS(fsys, ".")
		if err != nil {
			t.Fatalf("ScanFS() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"aaaa-skill/SKILL.md"}) {
			t.Fatalf("ScanFS() = %#v, want only definition before traversal limit", got)
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
