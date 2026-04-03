package setup

import (
	"strings"
	"testing"
)

func TestListSkillsRejectsInvalidSkillFrontmatter(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md": "---\n" +
			"name: cy-create-prd\n" +
			"description: Create a PRD\n" +
			"argument-hint: [feature-name-or-idea] [issue-file]\n" +
			"---\n",
	})

	_, err := ListSkills(bundle)
	if err == nil {
		t.Fatal("expected invalid frontmatter to fail")
	}
	if !strings.Contains(err.Error(), "unmarshal front matter") {
		t.Fatalf("expected YAML parse error, got %v", err)
	}
}
