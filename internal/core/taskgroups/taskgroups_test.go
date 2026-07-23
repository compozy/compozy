package taskgroups

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Suite: Task Group domain
// Invariant: A plan has one canonical, contained, deterministic task group state.
// Boundary IN: Markdown manifests and workspace paths.
// Boundary OUT: Persistence, lifecycle transport, planning UI, and Git state.

func TestValidatePlan(t *testing.T) {
	t.Parallel()

	t.Run("UT-003 rejects missing title and outcome", func(t *testing.T) {
		t.Parallel()
		content := string(twoTaskGroupPlan(t))
		content = strings.Replace(content, "## [ ] TG-001 — Persistence\n", "## [ ] TG-001 — \n", 1)
		content = strings.Replace(content, "- Outcome: Persist customer data\n", "- Outcome: \n", 1)
		_, err := ValidatePlan(content)
		assertDomainError(t, err, ErrInvalidPlan)
		assertIssueContains(t, err, "body.TG-001.title")
		assertIssueContains(t, err, "body.TG-001.outcome")
	})

	t.Run("UT-005 identifies edges affected by task group removal", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoTaskGroupPlan(t))
		issues := ValidateTaskGroupRemoval(plan, "TG-001")
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "TG-002") {
			t.Fatalf("removal issues = %#v, want TG-002 dependency diagnostic", issues)
		}
	})

	t.Run("UT-008 rejects self and unknown dependencies", func(t *testing.T) {
		t.Parallel()
		for name, replacement := range map[string]string{
			"self":    "to: TG-001",
			"unknown": "to: TG-999",
		} {
			t.Run(name, func(t *testing.T) {
				content := strings.Replace(string(twoTaskGroupPlan(t)), "to: TG-002", replacement, 1)
				_, err := ValidatePlan(content)
				assertDomainError(t, err, ErrInvalidPlan)
			})
		}
	})

	t.Run("UT-009 reports complete dependency cycle", func(t *testing.T) {
		t.Parallel()
		content := threeTaskGroupPlan(t, []Dependency{
			{From: "TG-001", To: "TG-002", Rationale: "one"},
			{From: "TG-002", To: "TG-003", Rationale: "two"},
			{From: "TG-003", To: "TG-001", Rationale: "three"},
		})
		_, err := ValidatePlan(string(content))
		assertDomainError(t, err, ErrInvalidPlan)
		if !strings.Contains(err.Error(), "TG-001 -> TG-002 -> TG-003 -> TG-001") {
			t.Fatalf("cycle error = %v", err)
		}
	})

	t.Run("UT-011 is deterministic and non-mutating", func(t *testing.T) {
		t.Parallel()
		content := twoTaskGroupPlan(t)
		first := mustParsePlan(t, content)
		second := mustParsePlan(t, content)
		if first.Checksum != second.Checksum || !slices.EqualFunc(first.TaskGroups, second.TaskGroups, equalTaskGroup) {
			t.Fatalf("plans differ: %#v %#v", first, second)
		}
		if !slices.Equal(content, twoTaskGroupPlan(t)) {
			t.Fatal("validation changed source bytes")
		}
	})

	t.Run("UT-012 reports all large-plan conflicts", func(t *testing.T) {
		t.Parallel()
		var body strings.Builder
		body.WriteString("---\n")
		body.WriteString("schema_version: compozy.task-groups/v1\ninitiative: demo\ngraph:\n  nodes:\n")
		for index := 0; index < 300; index++ {
			fmt.Fprint(&body, "    - id: TG-001\n      directory: _task_groups/TG-001\n")
		}
		body.WriteString("  edges:\n    - from: TG-001\n      to: TG-999\n      rationale: bad\n---\n\n")
		body.WriteString(
			"## [ ] TG-001 — One\n\n- Reference: `demo/TG-001`\n- Outcome: one\n- Owns:\n  - one\n- Dependencies: None\n",
		)
		_, err := ValidatePlan(body.String())
		assertDomainError(t, err, ErrInvalidPlan)
		var domainErr *Error
		if !errors.As(err, &domainErr) || len(domainErr.Issues) < 300 {
			t.Fatalf("issues = %#v, want every large-plan conflict", domainErr)
		}
	})

	t.Run("UT-015 and UT-036 reject graph and body drift", func(t *testing.T) {
		t.Parallel()
		cases := map[string]func(string) string{
			"yaml-only": func(content string) string {
				return strings.Replace(content, "TG-002 — Interface", "Removed body task group", 1)
			},
			"markdown-only": func(content string) string {
				return content + "\n## [ ] TG-003 — Extra\n\n- Reference: `demo/TG-003`\n- Outcome: extra\n- Owns:\n  - extra\n- Dependencies: None\n"
			},
			"directory": func(content string) string {
				return strings.Replace(content, "directory: _task_groups/TG-001", "directory: _task_groups/TG-002", 1)
			},
			"dependency": func(content string) string {
				return strings.Replace(content, "`TG-001` — API contract", "`TG-001` — changed", 1)
			},
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := ValidatePlan(mutate(string(twoTaskGroupPlan(t))))
				assertDomainError(t, err, ErrInvalidPlan)
			})
		}
	})

	t.Run("UT-028 rejects consumed empty producer outcomes", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(string(twoTaskGroupPlan(t)), "- Outcome: Persist customer data", "- Outcome: ", 1)
		_, err := ValidatePlan(content)
		assertDomainError(t, err, ErrInvalidPlan)
		if !strings.Contains(err.Error(), "outcome") {
			t.Fatalf("error = %v, want outcome diagnostic", err)
		}
	})

	t.Run("UT-032 parses canonical fields", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoTaskGroupPlan(t))
		first, found := plan.TaskGroup("TG-001")
		if !found || first.Directory != "_task_groups/TG-001" || first.Title != "Persistence" ||
			first.Outcome != "Persist customer data" ||
			first.Completed {
			t.Fatalf("TG-001 = %#v", first)
		}
		second, found := plan.TaskGroup("TG-002")
		if !found || len(second.Dependencies) != 1 || second.Dependencies[0].Rationale != "API contract" ||
			!second.Completed {
			t.Fatalf("TG-002 = %#v", second)
		}
	})

	t.Run("accepts readable task group directories without changing stable IDs", func(t *testing.T) {
		t.Parallel()
		content := strings.ReplaceAll(
			string(twoTaskGroupPlan(t)),
			"_task_groups/TG-001",
			"_task_groups/001-persistence-foundation",
		)
		content = strings.ReplaceAll(content, "_task_groups/TG-002", "_task_groups/002-interface-delivery")

		plan := mustParsePlan(t, []byte(content))
		first, found := plan.TaskGroup("TG-001")
		if !found || first.Directory != "_task_groups/001-persistence-foundation" {
			t.Fatalf("TG-001 = %#v", first)
		}
	})

	t.Run("rejects unsafe or mismatched readable task group directories", func(t *testing.T) {
		t.Parallel()
		for name, directory := range map[string]string{
			"mismatched ordinal": "_task_groups/002-persistence",
			"nested brief":       "_task_groups/001/persistence",
			"uppercase brief":    "_task_groups/001-Persistence",
			"missing brief":      "_task_groups/001-",
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				content := strings.Replace(
					string(twoTaskGroupPlan(t)),
					"directory: _task_groups/TG-001",
					"directory: "+directory,
					1,
				)
				_, err := ValidatePlan(content)
				assertDomainError(t, err, ErrInvalidPlan)
				assertIssueContains(t, err, "graph.nodes[0].directory")
			})
		}
	})

	t.Run("renders readable directories for task groups without a persisted path", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoTaskGroupPlan(t))
		for index := range plan.TaskGroups {
			plan.TaskGroups[index].Directory = ""
		}

		rendered, err := RenderPlan(plan)
		if err != nil {
			t.Fatalf("RenderPlan() error = %v", err)
		}
		for _, expected := range []string{
			"directory: _task_groups/001-persistence",
			"directory: _task_groups/002-interface",
		} {
			if !strings.Contains(string(rendered), expected) {
				t.Fatalf("RenderPlan() = %q, want %q", rendered, expected)
			}
		}
		mustParsePlan(t, rendered)
	})

	t.Run("UT-039 renders and reparses normalized state", func(t *testing.T) {
		t.Parallel()
		original := mustParsePlan(t, twoTaskGroupPlan(t))
		rendered, err := RenderPlan(original)
		if err != nil {
			t.Fatalf("RenderPlan() error = %v", err)
		}
		reparsed := mustParsePlan(t, rendered)
		if !slices.EqualFunc(original.TaskGroups, reparsed.TaskGroups, equalTaskGroup) ||
			!slices.EqualFunc(original.Edges, reparsed.Edges, equalDependency) {
			t.Fatalf("render/reparse mismatch: %#v %#v", original, reparsed)
		}
	})

	t.Run("renders only the selected task group excerpt", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoTaskGroupPlan(t))
		excerpt, err := RenderTaskGroupExcerpt(plan, "TG-002")
		if err != nil {
			t.Fatalf("RenderTaskGroupExcerpt() error = %v", err)
		}
		markdown := string(excerpt)
		if !strings.Contains(markdown, "TG-002 — Interface") || !strings.Contains(markdown, "API contract") {
			t.Fatalf("RenderTaskGroupExcerpt() = %q, want selected task group and dependency rationale", markdown)
		}
		if strings.Contains(markdown, "TG-001 — Persistence") {
			t.Fatalf("RenderTaskGroupExcerpt() leaked sibling task group: %q", markdown)
		}
	})
}

func TestTaskOwnership(t *testing.T) {
	t.Parallel()
	plan := mustParsePlan(t, twoTaskGroupPlan(t))

	t.Run("UT-006 identifies unowned qualified task", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, []string{"TG-001/task_01", "TG-002/task_01"}, []TaskGroupManifest{
			{TaskGroupID: "TG-001", TaskIDs: []string{"TG-001/task_02"}},
			{TaskGroupID: "TG-002", TaskIDs: []string{"TG-002/task_01"}},
		})
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "unowned task") {
			t.Fatalf("issues = %#v", issues)
		}
	})

	t.Run("UT-007 identifies duplicate owners", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, []string{"TG-001/task_01"}, []TaskGroupManifest{
			{TaskGroupID: "TG-001", TaskIDs: []string{"TG-001/task_01"}},
			{TaskGroupID: "TG-002", TaskIDs: []string{"TG-001/task_01"}},
		})
		if len(issues) == 0 || !strings.Contains(issues[0].Message, "TG-001, TG-002") {
			t.Fatalf("issues = %#v", issues)
		}
	})

	t.Run("UT-010 rejects task groups with no executable tasks", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, nil, []TaskGroupManifest{{TaskGroupID: "TG-001"}, {TaskGroupID: "TG-002"}})
		if len(issues) != 2 || !strings.Contains(issues[0].Message, "no executable tasks") {
			t.Fatalf("issues = %#v", issues)
		}
	})

	t.Run("UT-040 permits local repeated task names and rejects cross-task-group references", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, []string{"TG-001/task_01", "TG-002/task_01"}, []TaskGroupManifest{
			{TaskGroupID: "TG-001", TaskIDs: []string{"TG-001/task_01"}},
			{TaskGroupID: "TG-002", TaskIDs: []string{"TG-002/task_01"}},
		})
		if len(issues) != 0 {
			t.Fatalf("local repeated task IDs issues = %#v", issues)
		}
		issues = AuditTaskOwnership(
			plan,
			[]string{"TG-002/task_01"},
			[]TaskGroupManifest{{TaskGroupID: "TG-002", TaskIDs: []string{"TG-001/task_01"}}},
		)
		if len(issues) == 0 || !strings.Contains(strings.Join(issueMessages(issues), " "), "cross-task-group") {
			t.Fatalf("cross-task-group issues = %#v", issues)
		}
	})
}

func TestValidateTaskGroupManifest(t *testing.T) {
	t.Parallel()
	tasksDir := t.TempDir()
	writeTestFile(t, tasksDir, "_tasks.md", `---
schema_version: compozy.tasks/v2
workflow: demo/TG-002
graph:
  nodes:
    - id: task_01
      file: ../TG-001/task_01.md
  edges: []
---
`)
	_, issues, err := ValidateTaskGroupManifest(context.Background(), tasksDir, "demo/TG-002", "TG-002")
	if err != nil {
		t.Fatalf("ValidateTaskGroupManifest() error = %v", err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "sibling-ownership") {
		t.Fatalf("UT-017 issues = %#v", issues)
	}
}

func TestResolver(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
	planContent := strings.ReplaceAll(string(twoTaskGroupPlan(t)), "demo", "customer-management")
	writeTestFile(t, initiativeDir, ManifestFileName, planContent)
	writeTestFile(
		t,
		initiativeDir,
		"_task_groups/TG-001/task_01.md",
		"---\nstatus: pending\ntitle: one\ntype: backend\ncomplexity: low\n---\n",
	)
	writeTestFile(
		t,
		initiativeDir,
		"_task_groups/TG-002/task_01.md",
		"---\nstatus: pending\ntitle: two\ntype: backend\ncomplexity: low\n---\n",
	)
	resolver := TargetResolver{}

	t.Run("UT-013 never falls back to a similar initiative", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), workspace, "customer-managemen")
		assertDomainError(t, err, ErrInitiativeNotFound)
	})

	t.Run("UT-014 lists sorted stable IDs for unknown task group", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/TG-999")
		assertDomainError(t, err, ErrTaskGroupNotFound)
		var domainErr *Error
		if !errors.As(err, &domainErr) || !slices.Equal(domainErr.ValidTaskGroupIDs, []string{"TG-001", "TG-002"}) {
			t.Fatalf("not found error = %#v", domainErr)
		}
	})

	t.Run("UT-025 rejects unsafe and task-group-required references", func(t *testing.T) {
		for _, reference := range []string{"", "demo/TG-001/extra", "../TG-001", "demo/tg-001"} {
			_, err := ParseRef(reference)
			assertDomainError(t, err, ErrInvalidReference)
		}
		_, err := ParseTaskGroupRef("customer-management")
		assertDomainError(t, err, ErrSelectionRequired)
	})

	t.Run("UT-027 resolves stable IDs not duplicate titles", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/Persistence")
		assertDomainError(t, err, ErrInvalidReference)
		target, err := resolver.Resolve(context.Background(), workspace, "customer-management/TG-002")
		if err != nil || target.TaskGroup.ID != "TG-002" {
			t.Fatalf("target = %#v, error = %v", target, err)
		}
	})

	t.Run("resolves a stable ID to its readable task group directory", func(t *testing.T) {
		t.Parallel()
		readableWorkspace := t.TempDir()
		readableInitiativeDir := filepath.Join(
			readableWorkspace,
			".compozy",
			"tasks",
			"customer-management",
		)
		content := strings.ReplaceAll(planContent, "_task_groups/TG-001", "_task_groups/001-persistence-foundation")
		content = strings.ReplaceAll(content, "_task_groups/TG-002", "_task_groups/002-interface-delivery")
		writeTestFile(t, readableInitiativeDir, ManifestFileName, content)
		writeTestFile(
			t,
			readableInitiativeDir,
			"_task_groups/001-persistence-foundation/task_01.md",
			"---\nstatus: pending\ntitle: one\ntype: backend\ncomplexity: low\n---\n",
		)

		target, err := resolver.Resolve(
			context.Background(),
			readableWorkspace,
			"customer-management/TG-001",
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if filepath.Base(target.TaskGroupDir) != "001-persistence-foundation" {
			t.Fatalf("TaskGroupDir = %q", target.TaskGroupDir)
		}
	})

	t.Run("UT-030 preserves ordinary workflows without marker", func(t *testing.T) {
		ordinaryDir := filepath.Join(workspace, ".compozy", "tasks", "ordinary")
		writeTestFile(t, ordinaryDir, "TG-001/task_01.md", "content")
		mode, err := resolver.ClassifyTarget(context.Background(), workspace, "ordinary")
		if err != nil || mode != TargetModeOrdinary {
			t.Fatalf("mode = %q, error = %v", mode, err)
		}
	})

	t.Run("UT-031 fails closed for blank and malformed markers", func(t *testing.T) {
		for name, content := range map[string]string{"blank": "", "malformed": "not a plan"} {
			t.Run(name, func(t *testing.T) {
				initiative := filepath.Join(workspace, ".compozy", "tasks", "invalid-"+name)
				writeTestFile(t, initiative, ManifestFileName, content)
				mode, err := resolver.ClassifyTarget(context.Background(), workspace, "invalid-"+name)
				if err != nil || mode != TargetModeInvalidOptIn {
					t.Fatalf("mode = %q, error = %v", mode, err)
				}
			})
		}
	})

	t.Run("UT-033 rejects escape paths before task group task access", func(t *testing.T) {
		for _, reference := range []string{"/tmp/demo", "../customer-management", "customer-management/TG-001/more"} {
			_, err := resolver.Resolve(context.Background(), workspace, reference)
			assertDomainError(t, err, ErrInvalidReference)
		}
		escaped := filepath.Join(t.TempDir(), "outside")
		if err := os.MkdirAll(escaped, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(initiativeDir, "_task_groups", "TG-001")
		if err := os.RemoveAll(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(escaped, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/TG-001")
		assertDomainError(t, err, ErrContainment)
	})
}

func TestResolveTaskGroupMissingTaskGroupsRoot(t *testing.T) {
	// INVARIANT: an absent _task_groups root is the aggregate form of a missing
	// task group directory, so it classifies as ErrTaskGroupNotFound (which aggregate
	// sync degrades to a Missing placeholder) rather than ErrContainment (a hard
	// abort). A root that resolves outside the initiative still fails closed.
	// OWNING_LAYER: unit. EXISTING_SUITE: internal/core/taskgroups/taskgroups_test.go.
	t.Parallel()
	resolver := TargetResolver{}
	newWorkspace := func(t *testing.T) string {
		t.Helper()
		workspace := t.TempDir()
		initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
		planContent := strings.ReplaceAll(string(twoTaskGroupPlan(t)), "demo", "customer-management")
		writeTestFile(t, initiativeDir, ManifestFileName, planContent)
		return workspace
	}

	t.Run("missing root degrades to task group not found", func(t *testing.T) {
		t.Parallel()
		workspace := newWorkspace(t)
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/TG-001")
		assertDomainError(t, err, ErrTaskGroupNotFound)
	})

	t.Run("root escaping the initiative still fails closed", func(t *testing.T) {
		t.Parallel()
		workspace := newWorkspace(t)
		initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
		escaped := filepath.Join(t.TempDir(), "outside")
		if err := os.MkdirAll(filepath.Join(escaped, "TG-001"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(escaped, filepath.Join(initiativeDir, "_task_groups")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/TG-001")
		assertDomainError(t, err, ErrContainment)
	})
}

func TestResolveOperationalPathsReadableDirectory(t *testing.T) {
	t.Parallel()

	t.Run("finds a readable directory without trusting the current plan", func(t *testing.T) {
		t.Parallel()
		workspace := t.TempDir()
		initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
		writeTestFile(t, initiativeDir, ManifestFileName, "---\ninvalid")
		writeTestFile(
			t,
			initiativeDir,
			"_task_groups/001-persistence-foundation/task_01.md",
			"---\nstatus: completed\ntitle: one\ntype: backend\ncomplexity: low\n---\n",
		)

		paths, err := ResolveOperationalPaths(
			context.Background(),
			workspace,
			"customer-management/TG-001",
		)
		if err != nil {
			t.Fatalf("ResolveOperationalPaths() error = %v", err)
		}
		if filepath.Base(paths.TaskGroupDir) != "001-persistence-foundation" {
			t.Fatalf("TaskGroupDir = %q", paths.TaskGroupDir)
		}
	})

	t.Run("rejects ambiguous readable directories", func(t *testing.T) {
		t.Parallel()
		workspace := t.TempDir()
		initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
		writeTestFile(t, initiativeDir, "_task_groups/001-one/task_01.md", "content")
		writeTestFile(t, initiativeDir, "_task_groups/001-two/task_01.md", "content")

		_, err := ResolveOperationalPaths(
			context.Background(),
			workspace,
			"customer-management/TG-001",
		)
		assertDomainError(t, err, ErrInvalidPlan)
	})
}

func TestExecutionScope(t *testing.T) {
	// INVARIANT: a task group reference always separates root specifications from
	// one contained task group operational directory.
	// OWNING_LAYER: unit. EXISTING_SUITE: internal/core/taskgroups/taskgroups_test.go.
	workspace := t.TempDir()
	initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
	planContent := strings.ReplaceAll(string(twoTaskGroupPlan(t)), "demo", "customer-management")
	writeTestFile(t, initiativeDir, ManifestFileName, planContent)
	writeTestFile(
		t,
		initiativeDir,
		"_task_groups/TG-001/task_01.md",
		"---\nstatus: pending\ntitle: one\ntype: backend\ncomplexity: low\n---\n",
	)

	target, err := (TargetResolver{}).ResolveTaskGroup(context.Background(), workspace, "customer-management/TG-001")
	if err != nil {
		t.Fatalf("ResolveTaskGroup() error = %v", err)
	}
	scope, err := BuildExecutionScope(target)
	if err != nil {
		t.Fatalf("BuildExecutionScope() error = %v", err)
	}
	canonicalInitiativeDir, err := filepath.EvalSymlinks(initiativeDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(initiative): %v", err)
	}
	if scope.WorkflowRef != "customer-management/TG-001" || scope.SpecDir != canonicalInitiativeDir ||
		scope.OperationalDir != filepath.Join(canonicalInitiativeDir, "_task_groups", "TG-001") ||
		scope.TasksDir != scope.OperationalDir || scope.ReviewsDir != scope.OperationalDir ||
		scope.MemoryDir != filepath.Join(scope.OperationalDir, "memory") {
		t.Fatalf("UT-037 execution scope = %#v", scope)
	}

	target.TaskGroupDir = ""
	if _, err := BuildExecutionScope(target); err == nil {
		t.Fatal("BuildExecutionScope() error = nil for incomplete target")
	}
}

func TestReadiness(t *testing.T) {
	t.Parallel()
	t.Run("UT-016 keeps direct blockers before transitive context", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, threeTaskGroupPlan(t, []Dependency{
			{From: "TG-001", To: "TG-002", Rationale: "one"},
			{From: "TG-002", To: "TG-003", Rationale: "two"},
		}))
		readiness, err := EvaluateReadiness(plan, "TG-003")
		if err != nil || readiness.Eligible || len(readiness.DirectUnmet) != 1 ||
			readiness.DirectUnmet[0].From != "TG-002" {
			t.Fatalf("readiness = %#v, error = %v", readiness, err)
		}
		if len(readiness.TransitiveUnmet) != 1 ||
			!slices.Equal(readiness.TransitiveUnmet[0].TaskGroupIDs, []string{"TG-001", "TG-002"}) {
			t.Fatalf("transitive = %#v", readiness.TransitiveUnmet)
		}
	})

	t.Run("UT-029 keeps dependencies attached to stable IDs on rename", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(string(twoTaskGroupPlan(t)), "Persistence", "Renamed persistence", 1)
		plan := mustParsePlan(t, []byte(content))
		readiness, err := EvaluateReadiness(plan, "TG-002")
		if err != nil || len(readiness.DirectUnmet) != 1 || readiness.DirectUnmet[0].From != "TG-001" {
			t.Fatalf("readiness = %#v, error = %v", readiness, err)
		}
	})

	t.Run("UT-035 marks checked prerequisites eligible and independent peers", func(t *testing.T) {
		t.Parallel()
		plan := taskGroupPlan(t, []fixtureTaskGroup{
			{id: "TG-001", title: "one", completed: true},
			{id: "TG-002", title: "two"},
			{id: "TG-003", title: "three"},
		}, []Dependency{{From: "TG-001", To: "TG-002", Rationale: "one"}})
		readiness, err := EvaluateReadiness(mustParsePlan(t, plan), "TG-002")
		if err != nil || !readiness.Eligible || !slices.Equal(readiness.IndependentPeers, []string{"TG-003"}) {
			t.Fatalf("readiness = %#v, error = %v", readiness, err)
		}
	})
}

func TestValidateIndependentSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		plan         func(*testing.T) Plan
		selected     []string
		wantEligible []string
		wantRejected map[string]Rejection
		wantErr      error
	}{
		{
			name: "UT-001 accepts an independent pair",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, threeTaskGroupPlan(t, nil))
			},
			selected:     []string{"TG-001", "TG-002"},
			wantEligible: []string{"TG-001", "TG-002"},
			wantRejected: map[string]Rejection{},
		},
		{
			name: "UT-002 rejects an in-set dependency",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, threeTaskGroupPlan(t, []Dependency{{
					From: "TG-001", To: "TG-003", Rationale: "one before three",
				}}))
			},
			selected:     []string{"TG-001", "TG-003"},
			wantEligible: []string{},
			wantRejected: map[string]Rejection{
				"TG-001": {Reason: "depends_on_selected", Blockers: []string{"TG-003"}},
				"TG-003": {Reason: "depends_on_selected", Blockers: []string{"TG-001"}},
			},
		},
		{
			name: "UT-003 rejects an unmet out-of-set dependency",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, taskGroupPlan(t, []fixtureTaskGroup{
					{id: "TG-001", title: "One"},
					{id: "TG-002", title: "Two"},
					{id: "TG-003", title: "Three"},
					{id: "TG-004", title: "Four"},
				}, []Dependency{{From: "TG-002", To: "TG-004", Rationale: "two before four"}}))
			},
			selected:     []string{"TG-004"},
			wantEligible: []string{},
			wantRejected: map[string]Rejection{
				"TG-004": {Reason: "unmet_dependency", Blockers: []string{"TG-002"}},
			},
		},
		{
			name: "sorts and deduplicates unmet blockers",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, taskGroupPlan(t, []fixtureTaskGroup{
					{id: "TG-001", title: "One"},
					{id: "TG-002", title: "Two"},
					{id: "TG-003", title: "Three"},
					{id: "TG-004", title: "Four"},
				}, []Dependency{
					{From: "TG-003", To: "TG-004", Rationale: "three before four"},
					{From: "TG-001", To: "TG-002", Rationale: "one before two"},
					{From: "TG-002", To: "TG-004", Rationale: "two before four"},
				}))
			},
			selected:     []string{"TG-004"},
			wantEligible: []string{},
			wantRejected: map[string]Rejection{
				"TG-004": {
					Reason:   "unmet_dependency",
					Blockers: []string{"TG-001", "TG-002", "TG-003"},
				},
			},
		},
		{
			name: "UT-004 rejects an already-completed member",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, taskGroupPlan(t, []fixtureTaskGroup{{
					id: "TG-001", title: "One", completed: true,
				}}, nil))
			},
			selected:     []string{"TG-001"},
			wantEligible: []string{},
			wantRejected: map[string]Rejection{
				"TG-001": {Reason: "already_completed"},
			},
		},
		{
			name: "UT-005 rejects an unknown member",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, threeTaskGroupPlan(t, nil))
			},
			selected:     []string{"TG-999"},
			wantEligible: []string{},
			wantRejected: map[string]Rejection{
				"TG-999": {Reason: "unknown"},
			},
		},
		{
			name: "UT-006 rejects an empty selection with a typed error",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, threeTaskGroupPlan(t, nil))
			},
			selected:     nil,
			wantEligible: []string{},
			wantRejected: map[string]Rejection{},
			wantErr:      ErrSelectionRequired,
		},
		{
			name: "UT-007 accepts a single independent member",
			plan: func(t *testing.T) Plan {
				return mustParsePlan(t, threeTaskGroupPlan(t, nil))
			},
			selected:     []string{"TG-001"},
			wantEligible: []string{"TG-001"},
			wantRejected: map[string]Rejection{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			plan := test.plan(t)

			got, err := ValidateIndependentSet(plan, test.selected)
			if test.wantErr != nil {
				assertDomainError(t, err, test.wantErr)
				var domainErr *Error
				if !errors.As(err, &domainErr) {
					t.Fatalf("ValidateIndependentSet() error = %v, want *Error", err)
				}
			} else if err != nil {
				t.Fatalf("ValidateIndependentSet() error = %v", err)
			}
			if !slices.Equal(got.Eligible, test.wantEligible) {
				t.Fatalf("ValidateIndependentSet() eligible = %#v, want %#v", got.Eligible, test.wantEligible)
			}
			if !reflect.DeepEqual(got.Rejected, test.wantRejected) {
				t.Fatalf("ValidateIndependentSet() rejected = %#v, want %#v", got.Rejected, test.wantRejected)
			}
			if got.PlanChecksum != plan.Checksum {
				t.Fatalf("ValidateIndependentSet() checksum = %q, want %q", got.PlanChecksum, plan.Checksum)
			}
		})
	}

	t.Run("UT-008 is symmetric and order-invariant", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, threeTaskGroupPlan(t, nil))
		firstReadiness, err := EvaluateReadiness(plan, "TG-001")
		if err != nil {
			t.Fatalf("EvaluateReadiness(TG-001) error = %v", err)
		}
		secondReadiness, err := EvaluateReadiness(plan, "TG-002")
		if err != nil {
			t.Fatalf("EvaluateReadiness(TG-002) error = %v", err)
		}
		if !slices.Contains(firstReadiness.IndependentPeers, "TG-002") ||
			!slices.Contains(secondReadiness.IndependentPeers, "TG-001") {
			t.Fatalf(
				"IndependentPeers are not symmetric: TG-001=%#v TG-002=%#v",
				firstReadiness.IndependentPeers,
				secondReadiness.IndependentPeers,
			)
		}

		first, err := ValidateIndependentSet(plan, []string{"TG-001", "TG-002"})
		if err != nil {
			t.Fatalf("ValidateIndependentSet(A,B) error = %v", err)
		}
		second, err := ValidateIndependentSet(plan, []string{"TG-002", "TG-001"})
		if err != nil {
			t.Fatalf("ValidateIndependentSet(B,A) error = %v", err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("order changed validation result: first=%#v second=%#v", first, second)
		}
	})
}

func TestCompletion(t *testing.T) {
	t.Parallel()
	t.Run("UT-018 checks every completion prerequisite", func(t *testing.T) {
		t.Parallel()
		cases := map[CompletionBlockReason]CompletionPreconditions{
			CompletionBlockVerificationFailed: {HeadingExists: true},
			CompletionBlockReviewInterrupted: {
				VerificationPassed: true,
				ReviewInterrupted:  true,
				HeadingExists:      true,
			},
			CompletionBlockNewIssues: {VerificationPassed: true, NewIssues: true, HeadingExists: true},
			CompletionBlockPriorIssuesUnresolved: {
				VerificationPassed: true,
				PriorIssueStatuses: []string{"pending"},
				HeadingExists:      true,
			},
			CompletionBlockHeadingMissing: {VerificationPassed: true},
		}
		for want, preconditions := range cases {
			got := CanRecordCompletion(preconditions)
			if got.Eligible || got.Reason != want {
				t.Fatalf("CanRecordCompletion(%#v) = %#v, want %q", preconditions, got, want)
			}
		}
	})

	t.Run("UT-019 is byte-identical for an already checked task group", func(t *testing.T) {
		t.Parallel()
		content := twoTaskGroupPlan(t)
		rewrite, err := RewriteCompletion(content, "TG-002")
		if err != nil || !rewrite.AlreadyCompleted || rewrite.WriteRequired || !slices.Equal(rewrite.Content, content) {
			t.Fatalf("rewrite = %#v, error = %v", rewrite, err)
		}
	})

	t.Run("UT-020 preserves original bytes before atomic failures", func(t *testing.T) {
		t.Parallel()
		for _, failAt := range []string{"write", "sync", "close", "rename", "directory-sync"} {
			t.Run(failAt, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), ManifestFileName)
				original := twoTaskGroupPlan(t)
				writeTestFile(t, filepath.Dir(path), filepath.Base(path), string(original))
				err := writePlanAtomically(failingAtomicOps(failAt), path, []byte("new"), 0o600)
				if err == nil {
					t.Fatal("expected injected atomic writer failure")
				}
				if failAt != "directory-sync" {
					got, readErr := os.ReadFile(path)
					if readErr != nil || !slices.Equal(got, original) {
						t.Fatalf("original bytes changed: %q, read error = %v", got, readErr)
					}
				}
			})
		}
	})

	t.Run("UT-021 and UT-034 use stable ID and preserve unrelated latest bytes", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(string(twoTaskGroupPlan(t)), "Persistence", "Renamed", 1)
		content = strings.Replace(content, "  - Database schema", "  - Updated scope", 1)
		before := []byte(content)
		rewrite, err := RewriteCompletion(before, "TG-001")
		if err != nil || !rewrite.WriteRequired ||
			!strings.Contains(string(rewrite.Content), "## [x] TG-001 — Renamed") {
			t.Fatalf("rewrite = %#v, error = %v", rewrite, err)
		}
		if strings.Replace(string(rewrite.Content), "[x] TG-001", "[ ] TG-001", 1) != string(before) {
			t.Fatalf("rewrite changed bytes other than selected checkbox\nwant %q\ngot  %q", before, rewrite.Content)
		}
	})

	t.Run("Should rewrite headings with parser-compatible separator whitespace", func(t *testing.T) {
		t.Parallel()
		for _, separator := range []string{"—", "—\t"} {
			content := bytes.Replace(twoTaskGroupPlan(t), []byte("— Persistence"), []byte(separator+"Persistence"), 1)
			rewrite, err := RewriteCompletion(content, "TG-001")
			if err != nil || !rewrite.WriteRequired || !bytes.Contains(rewrite.Content, []byte("[x] TG-001")) {
				t.Fatalf("RewriteCompletion(separator %q) = %#v, error = %v", separator, rewrite, err)
			}
		}
	})

	t.Run("Should restore the plan when completion evidence changes during the write", func(t *testing.T) {
		t.Parallel()
		initiativeDir := filepath.Join(t.TempDir(), "demo")
		before := twoTaskGroupPlan(t)
		writeTestFile(t, initiativeDir, ManifestFileName, string(before))
		validations := 0

		result, err := NewStore().MarkCompleteValidated(
			context.Background(),
			initiativeDir,
			"TG-001",
			func(context.Context) error {
				validations++
				if validations == 2 {
					return errors.New("task reopened")
				}
				return nil
			},
		)
		if err == nil || !strings.Contains(err.Error(), "task reopened") {
			t.Fatalf("MarkCompleteValidated() error = %v, want stale-evidence rejection", err)
		}
		if result.CompletionRecorded || result.AlreadyCompleted {
			t.Fatalf("MarkCompleteValidated() result = %#v, want no completion", result)
		}
		if validations != 2 {
			t.Fatalf("completion evidence validations = %d, want 2", validations)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, ManifestFileName)); !slices.Equal(got, before) {
			t.Fatalf("stale-evidence rollback changed plan bytes\nwant %q\ngot  %q", before, got)
		}
	})

	t.Run("Should preserve an external plan update when completion rollback conflicts", func(t *testing.T) {
		t.Parallel()
		initiativeDir := filepath.Join(t.TempDir(), "demo")
		before := twoTaskGroupPlan(t)
		planPath := filepath.Join(initiativeDir, ManifestFileName)
		externalUpdate := bytes.Replace(before, []byte("Persistence"), []byte("Externally updated"), 1)
		writeTestFile(t, initiativeDir, ManifestFileName, string(before))
		validations := 0

		result, err := NewStore().MarkCompleteValidated(
			context.Background(),
			initiativeDir,
			"TG-001",
			func(context.Context) error {
				validations++
				if validations != 2 {
					return nil
				}
				if writeErr := os.WriteFile(planPath, externalUpdate, 0o600); writeErr != nil {
					return fmt.Errorf("replace plan externally: %w", writeErr)
				}
				return errors.New("task reopened")
			},
		)
		if err == nil || !strings.Contains(err.Error(), "task reopened") {
			t.Fatalf("MarkCompleteValidated() error = %v, want stale-evidence rejection", err)
		}
		if result.CompletionRecorded || result.AlreadyCompleted {
			t.Fatalf("MarkCompleteValidated() result = %#v, want no completion", result)
		}
		if got := mustReadFile(t, planPath); !bytes.Equal(got, externalUpdate) {
			t.Fatalf("completion rollback overwrote external plan update\nwant %q\ngot  %q", externalUpdate, got)
		}
		assertDomainError(t, err, ErrCompletionConflict)
		assertIssueContains(t, err, "write")
		var conflictErr *Error
		if !errors.As(err, &conflictErr) || conflictErr.PlanPath != planPath || conflictErr.TaskGroupID != "TG-001" {
			t.Fatalf("completion conflict evidence = %#v", conflictErr)
		}
	})

	t.Run("UT-022 through UT-024 keep lifecycle independent of Git or PR state", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoTaskGroupPlan(t))
		state, err := ProjectLifecycleState(plan, "TG-002")
		if err != nil || !state.LifecycleComplete {
			t.Fatalf("state = %#v, error = %v", state, err)
		}
		rewrite, err := RewriteCompletion(twoTaskGroupPlan(t), "TG-001")
		if err != nil || !rewrite.WriteRequired || !strings.Contains(string(rewrite.Content), "## [x] TG-001") {
			t.Fatalf("Git/remote-independent rewrite = %#v, error = %v", rewrite, err)
		}
	})

	t.Run("IT-028 rejects a read-only plan without changing its bytes", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("read-only replacement semantics differ on Windows")
		}
		initiativeDir := filepath.Join(t.TempDir(), "demo")
		before := twoTaskGroupPlan(t)
		writeTestFile(t, initiativeDir, ManifestFileName, string(before))
		planPath := filepath.Join(initiativeDir, ManifestFileName)
		if err := os.Chmod(planPath, 0o444); err != nil {
			t.Fatalf("chmod read-only plan: %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(planPath, 0o600); err != nil {
				t.Errorf("restore plan permissions: %v", err)
			}
		})

		result, err := NewStore().MarkComplete(context.Background(), initiativeDir, "TG-001")
		assertDomainError(t, err, ErrPlanReadOnly)
		if result.CompletionRecorded || result.AlreadyCompleted {
			t.Fatalf("MarkComplete() result = %#v, want no completion", result)
		}
		if got := mustReadFile(t, planPath); !slices.Equal(got, before) {
			t.Fatalf("read-only completion changed plan bytes\nwant %q\ngot  %q", before, got)
		}
	})

	t.Run("locks concurrent durable completions by stable ID", func(t *testing.T) {
		initiativeDir := filepath.Join(t.TempDir(), "demo")
		content := taskGroupPlan(t, []fixtureTaskGroup{
			{id: "TG-001", title: "One", outcome: "One outcome"},
			{id: "TG-002", title: "Two", outcome: "Two outcome"},
		}, nil)
		writeTestFile(t, initiativeDir, ManifestFileName, string(content))
		start := make(chan struct{})
		errs := make(chan error, 2)
		var group sync.WaitGroup
		for _, taskGroupID := range []string{"TG-001", "TG-002"} {
			group.Add(1)
			go func(id string) {
				defer group.Done()
				<-start
				_, err := NewStore().MarkComplete(context.Background(), initiativeDir, id)
				errs <- err
			}(taskGroupID)
		}
		close(start)
		group.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent MarkComplete() error = %v", err)
			}
		}
		plan := mustParsePlan(t, mustReadFile(t, filepath.Join(initiativeDir, ManifestFileName)))
		if !plan.IsComplete("TG-001") || !plan.IsComplete("TG-002") {
			t.Fatalf("concurrent completion plan = %#v", plan)
		}
	})
}

func TestHydrateCompletion(t *testing.T) {
	t.Parallel()
	t.Run("Should skip an absent completed heading and still mark the rest", func(t *testing.T) {
		t.Parallel()
		initiativeDir := filepath.Join(t.TempDir(), "demo")
		content := taskGroupPlan(t, []fixtureTaskGroup{
			{id: "TG-001", title: "One", outcome: "One outcome"},
			{id: "TG-002", title: "Two", outcome: "Two outcome"},
		}, nil)
		writeTestFile(t, initiativeDir, ManifestFileName, string(content))

		marked, err := NewStore().HydrateCompletion(
			context.Background(),
			initiativeDir,
			[]string{"TG-001", "TG-404", "TG-002"},
		)
		if err != nil {
			t.Fatalf("HydrateCompletion() error = %v, want nil for an absent heading", err)
		}
		if !slices.Equal(marked, []string{"TG-001", "TG-002"}) {
			t.Fatalf("HydrateCompletion() marked = %v, want [TG-001 TG-002]", marked)
		}
		plan := mustParsePlan(t, mustReadFile(t, filepath.Join(initiativeDir, ManifestFileName)))
		if !plan.IsComplete("TG-001") || !plan.IsComplete("TG-002") {
			t.Fatalf("hydrated plan = %#v, want TG-001 and TG-002 complete", plan)
		}
	})

	t.Run("Should surface a duplicated heading without writing a partial hydration", func(t *testing.T) {
		t.Parallel()
		initiativeDir := filepath.Join(t.TempDir(), "demo")
		base := taskGroupPlan(t, []fixtureTaskGroup{
			{id: "TG-001", title: "One", outcome: "One outcome"},
			{id: "TG-002", title: "Two", outcome: "Two outcome"},
		}, nil)
		duplicated := appendDuplicateHeading(t, base, "TG-002")
		writeTestFile(t, initiativeDir, ManifestFileName, string(duplicated))

		marked, err := NewStore().HydrateCompletion(
			context.Background(),
			initiativeDir,
			[]string{"TG-002"},
		)
		assertDomainError(t, err, ErrCompletionConflict)
		if marked != nil {
			t.Fatalf("HydrateCompletion() marked = %v, want nil on an ambiguous heading", marked)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, ManifestFileName)); !slices.Equal(got, duplicated) {
			t.Fatalf("ambiguous hydration changed plan bytes\nwant %q\ngot  %q", duplicated, got)
		}
	})
}

// appendDuplicateHeading returns content with a second copy of taskGroupID's
// stable heading, producing the ambiguous (>1) match hydration must reject.
func appendDuplicateHeading(t *testing.T, content []byte, taskGroupID string) []byte {
	t.Helper()
	for _, match := range completionHeadingPattern.FindAllSubmatchIndex(content, -1) {
		if string(content[match[4]:match[5]]) != taskGroupID {
			continue
		}
		heading := slices.Clone(content[match[0]:match[1]])
		duplicated := slices.Clone(content)
		duplicated = append(duplicated, '\n', '\n')
		return append(duplicated, heading...)
	}
	t.Fatalf("content missing stable heading for %s", taskGroupID)
	return nil
}

type fixtureTaskGroup struct {
	id        string
	title     string
	outcome   string
	completed bool
}

func twoTaskGroupPlan(t *testing.T) []byte {
	t.Helper()
	return taskGroupPlan(t, []fixtureTaskGroup{
		{id: "TG-001", title: "Persistence", outcome: "Persist customer data"},
		{id: "TG-002", title: "Interface", outcome: "Render customer data", completed: true},
	}, []Dependency{{From: "TG-001", To: "TG-002", Rationale: "API contract"}})
}

func threeTaskGroupPlan(t *testing.T, edges []Dependency) []byte {
	t.Helper()
	return taskGroupPlan(t, []fixtureTaskGroup{
		{id: "TG-001", title: "One", outcome: "One outcome"},
		{id: "TG-002", title: "Two", outcome: "Two outcome"},
		{id: "TG-003", title: "Three", outcome: "Three outcome"},
	}, edges)
}

func taskGroupPlan(t *testing.T, taskGroups []fixtureTaskGroup, edges []Dependency) []byte {
	t.Helper()
	plan := Plan{SchemaVersion: SchemaVersion, Initiative: "demo", Edges: slices.Clone(edges)}
	for _, spec := range taskGroups {
		outcome := spec.outcome
		if outcome == "" {
			outcome = spec.id + " outcome"
		}
		plan.TaskGroups = append(plan.TaskGroups, TaskGroup{
			ID:         spec.id,
			Title:      spec.title,
			Outcome:    outcome,
			Directory:  "_task_groups/" + spec.id,
			Completed:  spec.completed,
			OwnedScope: []string{spec.id + " scope"},
		})
	}
	content, err := RenderPlan(plan)
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	return content
}

func mustParsePlan(t *testing.T, content []byte) Plan {
	t.Helper()
	plan, err := ParsePlan(string(content))
	if err != nil {
		t.Fatalf("ParsePlan() error = %v", err)
	}
	return plan
}

func assertDomainError(t *testing.T, err error, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want errors.Is(_, %v)", err, want)
	}
}

func assertIssueContains(t *testing.T, err error, field string) {
	t.Helper()
	var domainErr *Error
	if !errors.As(err, &domainErr) {
		t.Fatalf("error = %v, want domain error", err)
	}
	for _, issue := range domainErr.Issues {
		if issue.Field == field {
			return
		}
	}
	t.Fatalf("issues = %#v, want field %q", domainErr.Issues, field)
}

func equalTaskGroup(left, right TaskGroup) bool {
	return left.ID == right.ID &&
		left.Title == right.Title &&
		left.Outcome == right.Outcome &&
		left.Reference == right.Reference &&
		left.Directory == right.Directory &&
		left.Completed == right.Completed &&
		slices.EqualFunc(left.Dependencies, right.Dependencies, equalDependency) &&
		slices.Equal(left.OwnedScope, right.OwnedScope)
}

func equalDependency(left, right Dependency) bool {
	return left.From == right.From && left.To == right.To && left.Rationale == right.Rationale
}

func issueMessages(issues []Issue) []string {
	result := make([]string, 0, len(issues))
	for _, issue := range issues {
		result = append(result, issue.Message)
	}
	return result
}

func writeTestFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

type failingTempFile struct {
	file   *os.File
	failAt string
}

func (f *failingTempFile) Name() string { return f.file.Name() }

func (f *failingTempFile) Write(content []byte) (int, error) {
	if f.failAt == "write" {
		return 0, errors.New("injected write failure")
	}
	return f.file.Write(content)
}

func (f *failingTempFile) Sync() error {
	if f.failAt == "sync" {
		return errors.New("injected file sync failure")
	}
	return f.file.Sync()
}

func (f *failingTempFile) Close() error {
	if f.failAt == "close" {
		return errors.New("injected close failure")
	}
	return f.file.Close()
}

func (f *failingTempFile) Chmod(mode fs.FileMode) error { return f.file.Chmod(mode) }

type failingDirectoryFile struct{ failAt string }

func (f failingDirectoryFile) Sync() error {
	if f.failAt == "directory-sync" {
		return errors.New("injected directory sync failure")
	}
	return nil
}

func (f failingDirectoryFile) Close() error { return nil }

func failingAtomicOps(failAt string) atomicFileOps {
	return atomicFileOps{
		createTemp: func(directory, pattern string) (atomicTempFile, error) {
			file, err := os.CreateTemp(directory, pattern)
			if err != nil {
				return nil, err
			}
			return &failingTempFile{file: file, failAt: failAt}, nil
		},
		stat:   os.Stat,
		remove: os.Remove,
		rename: func(oldPath, newPath string) error {
			if failAt == "rename" {
				return errors.New("injected rename failure")
			}
			return os.Rename(oldPath, newPath)
		},
		openDir: func(string) (syncFile, error) {
			return failingDirectoryFile{failAt: failAt}, nil
		},
	}
}
