package workpackages

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Suite: Work Package domain
// Invariant: A plan has one canonical, contained, deterministic package state.
// Boundary IN: Markdown manifests and workspace paths.
// Boundary OUT: Persistence, lifecycle transport, planning UI, and Git state.

func TestValidatePlan(t *testing.T) {
	t.Parallel()

	t.Run("UT-003 rejects missing title and outcome", func(t *testing.T) {
		t.Parallel()
		content := string(twoPackagePlan(t))
		content = strings.Replace(content, "## [ ] WP-001 — Persistence\n", "## [ ] WP-001 — \n", 1)
		content = strings.Replace(content, "- Outcome: Persist customer data\n", "- Outcome: \n", 1)
		_, err := ValidatePlan(content)
		assertDomainError(t, err, ErrInvalidPlan)
		assertIssueContains(t, err, "body.WP-001.title")
		assertIssueContains(t, err, "body.WP-001.outcome")
	})

	t.Run("UT-005 identifies edges affected by package removal", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoPackagePlan(t))
		issues := ValidatePackageRemoval(plan, "WP-001")
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "WP-002") {
			t.Fatalf("removal issues = %#v, want WP-002 dependency diagnostic", issues)
		}
	})

	t.Run("UT-008 rejects self and unknown dependencies", func(t *testing.T) {
		t.Parallel()
		for name, replacement := range map[string]string{
			"self":    "to: WP-001",
			"unknown": "to: WP-999",
		} {
			t.Run(name, func(t *testing.T) {
				content := strings.Replace(string(twoPackagePlan(t)), "to: WP-002", replacement, 1)
				_, err := ValidatePlan(content)
				assertDomainError(t, err, ErrInvalidPlan)
			})
		}
	})

	t.Run("UT-009 reports complete dependency cycle", func(t *testing.T) {
		t.Parallel()
		content := threePackagePlan(t, []Dependency{
			{From: "WP-001", To: "WP-002", Rationale: "one"},
			{From: "WP-002", To: "WP-003", Rationale: "two"},
			{From: "WP-003", To: "WP-001", Rationale: "three"},
		})
		_, err := ValidatePlan(string(content))
		assertDomainError(t, err, ErrInvalidPlan)
		if !strings.Contains(err.Error(), "WP-001 -> WP-002 -> WP-003 -> WP-001") {
			t.Fatalf("cycle error = %v", err)
		}
	})

	t.Run("UT-011 is deterministic and non-mutating", func(t *testing.T) {
		t.Parallel()
		content := twoPackagePlan(t)
		first := mustParsePlan(t, content)
		second := mustParsePlan(t, content)
		if first.Checksum != second.Checksum || !slices.EqualFunc(first.Packages, second.Packages, equalPackage) {
			t.Fatalf("plans differ: %#v %#v", first, second)
		}
		if !slices.Equal(content, twoPackagePlan(t)) {
			t.Fatal("validation changed source bytes")
		}
	})

	t.Run("UT-012 reports all large-plan conflicts", func(t *testing.T) {
		t.Parallel()
		var body strings.Builder
		body.WriteString("---\n")
		body.WriteString("schema_version: compozy.work-packages/v1\ninitiative: demo\ngraph:\n  nodes:\n")
		for index := 0; index < 300; index++ {
			fmt.Fprint(&body, "    - id: WP-001\n      directory: _packages/WP-001\n")
		}
		body.WriteString("  edges:\n    - from: WP-001\n      to: WP-999\n      rationale: bad\n---\n\n")
		body.WriteString(
			"## [ ] WP-001 — One\n\n- Reference: `demo/WP-001`\n- Outcome: one\n- Owns:\n  - one\n- Dependencies: None\n",
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
				return strings.Replace(content, "WP-002 — Interface", "Removed body package", 1)
			},
			"markdown-only": func(content string) string {
				return content + "\n## [ ] WP-003 — Extra\n\n- Reference: `demo/WP-003`\n- Outcome: extra\n- Owns:\n  - extra\n- Dependencies: None\n"
			},
			"directory": func(content string) string {
				return strings.Replace(content, "directory: _packages/WP-001", "directory: _packages/WP-002", 1)
			},
			"dependency": func(content string) string {
				return strings.Replace(content, "`WP-001` — API contract", "`WP-001` — changed", 1)
			},
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := ValidatePlan(mutate(string(twoPackagePlan(t))))
				assertDomainError(t, err, ErrInvalidPlan)
			})
		}
	})

	t.Run("UT-028 rejects consumed empty producer outcomes", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(string(twoPackagePlan(t)), "- Outcome: Persist customer data", "- Outcome: ", 1)
		_, err := ValidatePlan(content)
		assertDomainError(t, err, ErrInvalidPlan)
		if !strings.Contains(err.Error(), "outcome") {
			t.Fatalf("error = %v, want outcome diagnostic", err)
		}
	})

	t.Run("UT-032 parses canonical fields", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoPackagePlan(t))
		first, found := plan.Package("WP-001")
		if !found || first.Directory != "_packages/WP-001" || first.Title != "Persistence" ||
			first.Outcome != "Persist customer data" ||
			first.Completed {
			t.Fatalf("WP-001 = %#v", first)
		}
		second, found := plan.Package("WP-002")
		if !found || len(second.Dependencies) != 1 || second.Dependencies[0].Rationale != "API contract" ||
			!second.Completed {
			t.Fatalf("WP-002 = %#v", second)
		}
	})

	t.Run("UT-039 renders and reparses normalized state", func(t *testing.T) {
		t.Parallel()
		original := mustParsePlan(t, twoPackagePlan(t))
		rendered, err := RenderPlan(original)
		if err != nil {
			t.Fatalf("RenderPlan() error = %v", err)
		}
		reparsed := mustParsePlan(t, rendered)
		if !slices.EqualFunc(original.Packages, reparsed.Packages, equalPackage) ||
			!slices.EqualFunc(original.Edges, reparsed.Edges, equalDependency) {
			t.Fatalf("render/reparse mismatch: %#v %#v", original, reparsed)
		}
	})

	t.Run("renders only the selected package excerpt", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoPackagePlan(t))
		excerpt, err := RenderPackageExcerpt(plan, "WP-002")
		if err != nil {
			t.Fatalf("RenderPackageExcerpt() error = %v", err)
		}
		markdown := string(excerpt)
		if !strings.Contains(markdown, "WP-002 — Interface") || !strings.Contains(markdown, "API contract") {
			t.Fatalf("RenderPackageExcerpt() = %q, want selected package and dependency rationale", markdown)
		}
		if strings.Contains(markdown, "WP-001 — Persistence") {
			t.Fatalf("RenderPackageExcerpt() leaked sibling package: %q", markdown)
		}
	})
}

func TestTaskOwnership(t *testing.T) {
	t.Parallel()
	plan := mustParsePlan(t, twoPackagePlan(t))

	t.Run("UT-006 identifies unowned qualified task", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, []string{"WP-001/task_01", "WP-002/task_01"}, []PackageManifest{
			{PackageID: "WP-001", TaskIDs: []string{"WP-001/task_02"}},
			{PackageID: "WP-002", TaskIDs: []string{"WP-002/task_01"}},
		})
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "unowned task") {
			t.Fatalf("issues = %#v", issues)
		}
	})

	t.Run("UT-007 identifies duplicate owners", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, []string{"WP-001/task_01"}, []PackageManifest{
			{PackageID: "WP-001", TaskIDs: []string{"WP-001/task_01"}},
			{PackageID: "WP-002", TaskIDs: []string{"WP-001/task_01"}},
		})
		if len(issues) == 0 || !strings.Contains(issues[0].Message, "WP-001, WP-002") {
			t.Fatalf("issues = %#v", issues)
		}
	})

	t.Run("UT-010 rejects packages with no executable tasks", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, nil, []PackageManifest{{PackageID: "WP-001"}, {PackageID: "WP-002"}})
		if len(issues) != 2 || !strings.Contains(issues[0].Message, "no executable tasks") {
			t.Fatalf("issues = %#v", issues)
		}
	})

	t.Run("UT-040 permits local repeated task names and rejects cross-package references", func(t *testing.T) {
		t.Parallel()
		issues := AuditTaskOwnership(plan, []string{"WP-001/task_01", "WP-002/task_01"}, []PackageManifest{
			{PackageID: "WP-001", TaskIDs: []string{"WP-001/task_01"}},
			{PackageID: "WP-002", TaskIDs: []string{"WP-002/task_01"}},
		})
		if len(issues) != 0 {
			t.Fatalf("local repeated task IDs issues = %#v", issues)
		}
		issues = AuditTaskOwnership(
			plan,
			[]string{"WP-002/task_01"},
			[]PackageManifest{{PackageID: "WP-002", TaskIDs: []string{"WP-001/task_01"}}},
		)
		if len(issues) == 0 || !strings.Contains(strings.Join(issueMessages(issues), " "), "cross-package") {
			t.Fatalf("cross-package issues = %#v", issues)
		}
	})
}

func TestValidatePackageManifest(t *testing.T) {
	t.Parallel()
	tasksDir := t.TempDir()
	writeTestFile(t, tasksDir, "_tasks.md", `---
schema_version: compozy.tasks/v2
workflow: demo/WP-002
graph:
  nodes:
    - id: task_01
      file: ../WP-001/task_01.md
  edges: []
---
`)
	_, issues, err := ValidatePackageManifest(context.Background(), tasksDir, "demo/WP-002", "WP-002")
	if err != nil {
		t.Fatalf("ValidatePackageManifest() error = %v", err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "sibling-ownership") {
		t.Fatalf("UT-017 issues = %#v", issues)
	}
}

func TestResolver(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
	planContent := strings.ReplaceAll(string(twoPackagePlan(t)), "demo", "customer-management")
	writeTestFile(t, initiativeDir, ManifestFileName, planContent)
	writeTestFile(
		t,
		initiativeDir,
		"_packages/WP-001/task_01.md",
		"---\nstatus: pending\ntitle: one\ntype: backend\ncomplexity: low\n---\n",
	)
	writeTestFile(
		t,
		initiativeDir,
		"_packages/WP-002/task_01.md",
		"---\nstatus: pending\ntitle: two\ntype: backend\ncomplexity: low\n---\n",
	)
	resolver := TargetResolver{}

	t.Run("UT-013 never falls back to a similar initiative", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), workspace, "customer-managemen")
		assertDomainError(t, err, ErrInitiativeNotFound)
	})

	t.Run("UT-014 lists sorted stable IDs for unknown package", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/WP-999")
		assertDomainError(t, err, ErrPackageNotFound)
		var domainErr *Error
		if !errors.As(err, &domainErr) || !slices.Equal(domainErr.ValidPackageIDs, []string{"WP-001", "WP-002"}) {
			t.Fatalf("not found error = %#v", domainErr)
		}
	})

	t.Run("UT-025 rejects unsafe and package-required references", func(t *testing.T) {
		for _, reference := range []string{"", "demo/WP-001/extra", "../WP-001", "demo/wp-001"} {
			_, err := ParseRef(reference)
			assertDomainError(t, err, ErrInvalidReference)
		}
		_, err := ParsePackageRef("customer-management")
		assertDomainError(t, err, ErrSelectionRequired)
	})

	t.Run("UT-027 resolves stable IDs not duplicate titles", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/Persistence")
		assertDomainError(t, err, ErrInvalidReference)
		target, err := resolver.Resolve(context.Background(), workspace, "customer-management/WP-002")
		if err != nil || target.Package.ID != "WP-002" {
			t.Fatalf("target = %#v, error = %v", target, err)
		}
	})

	t.Run("UT-030 preserves ordinary workflows without marker", func(t *testing.T) {
		ordinaryDir := filepath.Join(workspace, ".compozy", "tasks", "ordinary")
		writeTestFile(t, ordinaryDir, "WP-001/task_01.md", "content")
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

	t.Run("UT-033 rejects escape paths before package task access", func(t *testing.T) {
		for _, reference := range []string{"/tmp/demo", "../customer-management", "customer-management/WP-001/more"} {
			_, err := resolver.Resolve(context.Background(), workspace, reference)
			assertDomainError(t, err, ErrInvalidReference)
		}
		escaped := filepath.Join(t.TempDir(), "outside")
		if err := os.MkdirAll(escaped, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(initiativeDir, "_packages", "WP-001")
		if err := os.RemoveAll(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(escaped, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := resolver.Resolve(context.Background(), workspace, "customer-management/WP-001")
		assertDomainError(t, err, ErrContainment)
	})
}

func TestExecutionScope(t *testing.T) {
	// INVARIANT: a package reference always separates root specifications from
	// one contained package operational directory.
	// OWNING_LAYER: unit. EXISTING_SUITE: internal/core/workpackages/workpackages_test.go.
	workspace := t.TempDir()
	initiativeDir := filepath.Join(workspace, ".compozy", "tasks", "customer-management")
	planContent := strings.ReplaceAll(string(twoPackagePlan(t)), "demo", "customer-management")
	writeTestFile(t, initiativeDir, ManifestFileName, planContent)
	writeTestFile(
		t,
		initiativeDir,
		"_packages/WP-001/task_01.md",
		"---\nstatus: pending\ntitle: one\ntype: backend\ncomplexity: low\n---\n",
	)

	target, err := (TargetResolver{}).ResolvePackage(context.Background(), workspace, "customer-management/WP-001")
	if err != nil {
		t.Fatalf("ResolvePackage() error = %v", err)
	}
	scope, err := BuildExecutionScope(target)
	if err != nil {
		t.Fatalf("BuildExecutionScope() error = %v", err)
	}
	canonicalInitiativeDir, err := filepath.EvalSymlinks(initiativeDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(initiative): %v", err)
	}
	if scope.WorkflowRef != "customer-management/WP-001" || scope.SpecDir != canonicalInitiativeDir ||
		scope.OperationalDir != filepath.Join(canonicalInitiativeDir, "_packages", "WP-001") ||
		scope.TasksDir != scope.OperationalDir || scope.ReviewsDir != scope.OperationalDir ||
		scope.MemoryDir != filepath.Join(scope.OperationalDir, "memory") {
		t.Fatalf("UT-037 execution scope = %#v", scope)
	}

	target.PackageDir = ""
	if _, err := BuildExecutionScope(target); err == nil {
		t.Fatal("BuildExecutionScope() error = nil for incomplete target")
	}
}

func TestReadiness(t *testing.T) {
	t.Parallel()
	t.Run("UT-016 keeps direct blockers before transitive context", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, threePackagePlan(t, []Dependency{
			{From: "WP-001", To: "WP-002", Rationale: "one"},
			{From: "WP-002", To: "WP-003", Rationale: "two"},
		}))
		readiness, err := EvaluateReadiness(plan, "WP-003")
		if err != nil || readiness.Eligible || len(readiness.DirectUnmet) != 1 ||
			readiness.DirectUnmet[0].From != "WP-002" {
			t.Fatalf("readiness = %#v, error = %v", readiness, err)
		}
		if len(readiness.TransitiveUnmet) != 1 ||
			!slices.Equal(readiness.TransitiveUnmet[0].PackageIDs, []string{"WP-001", "WP-002"}) {
			t.Fatalf("transitive = %#v", readiness.TransitiveUnmet)
		}
	})

	t.Run("UT-029 keeps dependencies attached to stable IDs on rename", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(string(twoPackagePlan(t)), "Persistence", "Renamed persistence", 1)
		plan := mustParsePlan(t, []byte(content))
		readiness, err := EvaluateReadiness(plan, "WP-002")
		if err != nil || len(readiness.DirectUnmet) != 1 || readiness.DirectUnmet[0].From != "WP-001" {
			t.Fatalf("readiness = %#v, error = %v", readiness, err)
		}
	})

	t.Run("UT-035 marks checked prerequisites eligible and independent peers", func(t *testing.T) {
		t.Parallel()
		plan := packagePlan(t, []fixturePackage{
			{id: "WP-001", title: "one", completed: true},
			{id: "WP-002", title: "two"},
			{id: "WP-003", title: "three"},
		}, []Dependency{{From: "WP-001", To: "WP-002", Rationale: "one"}})
		readiness, err := EvaluateReadiness(mustParsePlan(t, plan), "WP-002")
		if err != nil || !readiness.Eligible || !slices.Equal(readiness.IndependentPeers, []string{"WP-003"}) {
			t.Fatalf("readiness = %#v, error = %v", readiness, err)
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

	t.Run("UT-019 is byte-identical for an already checked package", func(t *testing.T) {
		t.Parallel()
		content := twoPackagePlan(t)
		rewrite, err := RewriteCompletion(content, "WP-002")
		if err != nil || !rewrite.AlreadyCompleted || rewrite.WriteRequired || !slices.Equal(rewrite.Content, content) {
			t.Fatalf("rewrite = %#v, error = %v", rewrite, err)
		}
	})

	t.Run("UT-020 preserves original bytes before atomic failures", func(t *testing.T) {
		t.Parallel()
		for _, failAt := range []string{"write", "sync", "close", "rename", "directory-sync"} {
			t.Run(failAt, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), ManifestFileName)
				original := twoPackagePlan(t)
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
		content := strings.Replace(string(twoPackagePlan(t)), "Persistence", "Renamed", 1)
		content = strings.Replace(content, "  - Database schema", "  - Updated scope", 1)
		before := []byte(content)
		rewrite, err := RewriteCompletion(before, "WP-001")
		if err != nil || !rewrite.WriteRequired ||
			!strings.Contains(string(rewrite.Content), "## [x] WP-001 — Renamed") {
			t.Fatalf("rewrite = %#v, error = %v", rewrite, err)
		}
		if strings.Replace(string(rewrite.Content), "[x] WP-001", "[ ] WP-001", 1) != string(before) {
			t.Fatalf("rewrite changed bytes other than selected checkbox\nwant %q\ngot  %q", before, rewrite.Content)
		}
	})

	t.Run("UT-022 through UT-024 keep lifecycle independent of Git or PR state", func(t *testing.T) {
		t.Parallel()
		plan := mustParsePlan(t, twoPackagePlan(t))
		state, err := ProjectLifecycleState(plan, "WP-002")
		if err != nil || !state.LifecycleComplete {
			t.Fatalf("state = %#v, error = %v", state, err)
		}
		rewrite, err := RewriteCompletion(twoPackagePlan(t), "WP-001")
		if err != nil || !rewrite.WriteRequired || !strings.Contains(string(rewrite.Content), "## [x] WP-001") {
			t.Fatalf("Git/remote-independent rewrite = %#v, error = %v", rewrite, err)
		}
	})

	t.Run("IT-028 rejects a read-only plan without changing its bytes", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("read-only replacement semantics differ on Windows")
		}
		initiativeDir := filepath.Join(t.TempDir(), "demo")
		before := twoPackagePlan(t)
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

		result, err := NewStore().MarkComplete(context.Background(), initiativeDir, "WP-001")
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
		content := packagePlan(t, []fixturePackage{
			{id: "WP-001", title: "One", outcome: "One outcome"},
			{id: "WP-002", title: "Two", outcome: "Two outcome"},
		}, nil)
		writeTestFile(t, initiativeDir, ManifestFileName, string(content))
		start := make(chan struct{})
		errs := make(chan error, 2)
		var group sync.WaitGroup
		for _, packageID := range []string{"WP-001", "WP-002"} {
			group.Add(1)
			go func(id string) {
				defer group.Done()
				<-start
				_, err := NewStore().MarkComplete(context.Background(), initiativeDir, id)
				errs <- err
			}(packageID)
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
		if !plan.IsComplete("WP-001") || !plan.IsComplete("WP-002") {
			t.Fatalf("concurrent completion plan = %#v", plan)
		}
	})
}

type fixturePackage struct {
	id        string
	title     string
	outcome   string
	completed bool
}

func twoPackagePlan(t *testing.T) []byte {
	t.Helper()
	return packagePlan(t, []fixturePackage{
		{id: "WP-001", title: "Persistence", outcome: "Persist customer data"},
		{id: "WP-002", title: "Interface", outcome: "Render customer data", completed: true},
	}, []Dependency{{From: "WP-001", To: "WP-002", Rationale: "API contract"}})
}

func threePackagePlan(t *testing.T, edges []Dependency) []byte {
	t.Helper()
	return packagePlan(t, []fixturePackage{
		{id: "WP-001", title: "One", outcome: "One outcome"},
		{id: "WP-002", title: "Two", outcome: "Two outcome"},
		{id: "WP-003", title: "Three", outcome: "Three outcome"},
	}, edges)
}

func packagePlan(t *testing.T, packages []fixturePackage, edges []Dependency) []byte {
	t.Helper()
	plan := Plan{SchemaVersion: SchemaVersion, Initiative: "demo", Edges: slices.Clone(edges)}
	for _, spec := range packages {
		outcome := spec.outcome
		if outcome == "" {
			outcome = spec.id + " outcome"
		}
		plan.Packages = append(plan.Packages, Package{
			ID:         spec.id,
			Title:      spec.title,
			Outcome:    outcome,
			Directory:  "_packages/" + spec.id,
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

func equalPackage(left, right Package) bool {
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
