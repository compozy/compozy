package looper_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBundledSkillsExistAndUsePortableReferences(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	requiredPaths := []string{
		"skills/fix-reviews/SKILL.md",
		"skills/verification-before-completion/SKILL.md",
		"skills/execute-prd-task/SKILL.md",
		"skills/execute-prd-task/references/tracking-checklist.md",
		"skills/create-prd/SKILL.md",
		"skills/create-prd/references/prd-template.md",
		"skills/create-prd/references/question-protocol.md",
		"skills/create-prd/references/adr-template.md",
		"skills/create-techspec/SKILL.md",
		"skills/create-techspec/references/techspec-template.md",
		"skills/create-techspec/references/adr-template.md",
		"skills/create-tasks/SKILL.md",
		"skills/create-tasks/references/task-template.md",
		"skills/create-tasks/references/task-context-schema.md",
		"skills/review-round/SKILL.md",
		"skills/review-round/references/review-criteria.md",
		"skills/review-round/references/issue-template.md",
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

	checkPortableContent(t, filepath.Join(root, "skills", "fix-reviews", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "execute-prd-task", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "create-prd", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "create-techspec", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "create-tasks", "SKILL.md"))
	checkPortableContent(t, filepath.Join(root, "skills", "review-round", "SKILL.md"))
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file location")
	}
	return filepath.Dir(file)
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
