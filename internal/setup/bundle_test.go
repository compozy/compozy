package setup

import (
	"io/fs"
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
		"compozy",
		"cy-create-prd",
		"cy-create-tasks",
		"cy-create-techspec",
		"cy-execute-task",
		"cy-final-verify",
		"cy-fix-reviews",
		"cy-idea-factory",
		"cy-review-round",
		"cy-workflow-memory",
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

func TestBundledWorkflowMemorySkillIncludesReferenceFile(t *testing.T) {
	t.Parallel()

	bundle, err := bundledSkillsRoot()
	if err != nil {
		t.Fatalf("bundled skills root: %v", err)
	}

	if _, err := fs.Stat(bundle, "cy-workflow-memory/SKILL.md"); err != nil {
		t.Fatalf("expected bundled workflow-memory skill, got %v", err)
	}
	if _, err := fs.Stat(bundle, "cy-workflow-memory/references/memory-guidelines.md"); err != nil {
		t.Fatalf("expected bundled workflow-memory reference file, got %v", err)
	}
}

func TestListBundledReusableAgentsExposesCouncilRoster(t *testing.T) {
	t.Parallel()

	reusableAgents, err := ListBundledReusableAgents()
	if err != nil {
		t.Fatalf("list bundled reusable agents: %v", err)
	}

	var names []string
	for _, reusableAgent := range reusableAgents {
		names = append(names, reusableAgent.Name)
		if reusableAgent.Title == "" {
			t.Fatalf("expected reusable agent %q to have a title", reusableAgent.Name)
		}
		if reusableAgent.Description == "" {
			t.Fatalf("expected reusable agent %q to have a description", reusableAgent.Name)
		}
	}

	want := []string{
		"architect-advisor",
		"devils-advocate",
		"pragmatic-engineer",
		"product-mind",
		"security-advocate",
		"the-thinker",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected bundled reusable-agent roster\nwant: %#v\ngot:  %#v", want, names)
	}
}

func TestBundledReusableAgentsExposeCanonicalAgentFiles(t *testing.T) {
	t.Parallel()

	bundle, err := bundledReusableAgentsRoot()
	if err != nil {
		t.Fatalf("bundled reusable agents root: %v", err)
	}

	for _, name := range []string{
		"architect-advisor",
		"devils-advocate",
		"pragmatic-engineer",
		"product-mind",
		"security-advocate",
		"the-thinker",
	} {
		if _, err := fs.Stat(bundle, name+"/AGENT.md"); err != nil {
			t.Fatalf("expected bundled reusable agent %q, got %v", name, err)
		}
	}
}
