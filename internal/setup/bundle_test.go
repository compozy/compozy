package setup

import (
	"reflect"
	"slices"
	"testing"
)

func TestListBundledSkillsExposesOnlyPublicCatalog(t *testing.T) {
	t.Parallel()

	skills, err := ListBundledSkills()
	if err != nil {
		t.Fatalf("list bundled skills: %v", err)
	}

	var names []string
	for _, skill := range skills {
		names = append(names, skill.Name)
	}

	want := []string{
		"create-prd",
		"create-tasks",
		"create-techspec",
		"execute-prd-task",
		"fix-reviews",
		"review-round",
		"verification-before-completion",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected bundled skill names\nwant: %#v\ngot:  %#v", want, names)
	}

	for _, forbidden := range []string{"brainstorming", "golang-pro", "testing-anti-patterns"} {
		if slices.Contains(names, forbidden) {
			t.Fatalf("expected internal skill %q to be excluded from bundled catalog", forbidden)
		}
	}
}
