package test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledSkillsExistAndUsePortableReferences(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	requiredPaths := []string{
		"skills/cy-fix-reviews/SKILL.md",
		"skills/cy-final-verify/SKILL.md",
		"skills/cy-execute-task/SKILL.md",
		"skills/cy-execute-task/references/tracking-checklist.md",
		"skills/cy-create-prd/SKILL.md",
		"skills/cy-create-prd/references/prd-template.md",
		"skills/cy-create-prd/references/question-protocol.md",
		"skills/cy-create-prd/references/adr-template.md",
		"skills/cy-create-techspec/SKILL.md",
		"skills/cy-create-techspec/references/techspec-template.md",
		"skills/cy-create-techspec/references/adr-template.md",
		"skills/cy-create-tasks/SKILL.md",
		"skills/cy-create-tasks/references/task-template.md",
		"skills/cy-create-tasks/references/task-context-schema.md",
		"skills/cy-review-round/SKILL.md",
		"skills/cy-review-round/references/review-criteria.md",
		"skills/cy-review-round/references/issue-template.md",
	}

	for _, relativePath := range requiredPaths {
		t.Run(relativePath, func(t *testing.T) {
			t.Parallel()

			absPath := filepath.Join(root, relativePath)
			if _, err := os.Stat(absPath); err != nil {
				t.Fatalf("expected %s to exist: %v", relativePath, err)
			}
		})
	}

	checkPortableContent(t, filepath.Join(root, "skills", "cy-fix-reviews", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "cy-execute-task", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "cy-create-prd", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "cy-create-techspec", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "cy-create-tasks", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "cy-review-round", "SKILL.md"))
}

func TestBundledSkillMirrorMatchesPublicSkillsTree(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	source := filepath.Join(root, "skills")
	mirror := filepath.Join(root, "internal", "setup", "assets", "skills")

	sourceTree := snapshotTree(t, source)
	mirrorTree := snapshotTree(t, mirror)

	if len(sourceTree) != len(mirrorTree) {
		t.Fatalf("expected bundled mirror to contain %d files, got %d", len(sourceTree), len(mirrorTree))
	}
	for path, sourceContent := range sourceTree {
		mirrorContent, ok := mirrorTree[path]
		if !ok {
			t.Fatalf("expected bundled mirror to contain %s", path)
		}
		if sourceContent != mirrorContent {
			t.Fatalf("expected bundled mirror content for %s to match source skills directory", path)
		}
	}
}

func checkPortableContent(t *testing.T, path string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	forbiddenSnippets := []string{
		".claude/skills",
		"pnpm run",
		"scripts/read_pr_issues.sh",
	}
	for _, snippet := range forbiddenSnippets {
		if strings.Contains(text, snippet) {
			t.Fatalf("expected %s to omit %q", path, snippet)
		}
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()

	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relativePath)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}
