package test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/skills"
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

func TestBundledSkillFrontmatterParses(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatalf("glob bundled skills: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected bundled skills to exist")
	}

	for _, skillPath := range paths {
		skillPath := skillPath
		t.Run(filepath.Base(filepath.Dir(skillPath)), func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(skillPath)
			if err != nil {
				t.Fatalf("read %s: %v", skillPath, err)
			}

			var metadata struct {
				Name         string `yaml:"name"`
				Description  string `yaml:"description"`
				ArgumentHint any    `yaml:"argument-hint,omitempty"`
			}
			if _, err := frontmatter.Parse(string(content), &metadata); err != nil {
				t.Fatalf("parse frontmatter %s: %v", skillPath, err)
			}
			if metadata.Name == "" {
				t.Fatalf("expected %s to define a non-empty name", skillPath)
			}
			if metadata.Description == "" {
				t.Fatalf("expected %s to define a non-empty description", skillPath)
			}
		})
	}
}

func TestIdeaFactoryExtensionExistsAndUsesPortableReferences(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	requiredPaths := []string{
		"extensions/cy-idea-factory/extension.toml",
		"extensions/cy-idea-factory/skills/cy-idea-factory/SKILL.md",
		"extensions/cy-idea-factory/skills/cy-idea-factory/references/adr-template.md",
		"extensions/cy-idea-factory/skills/cy-idea-factory/references/council.md",
		"extensions/cy-idea-factory/agents/architect-advisor/AGENT.md",
		"extensions/cy-idea-factory/agents/devils-advocate/AGENT.md",
		"extensions/cy-idea-factory/agents/pragmatic-engineer/AGENT.md",
		"extensions/cy-idea-factory/agents/product-mind/AGENT.md",
		"extensions/cy-idea-factory/agents/security-advocate/AGENT.md",
		"extensions/cy-idea-factory/agents/the-thinker/AGENT.md",
	}

	for _, relativePath := range requiredPaths {
		relativePath := relativePath
		t.Run(fmt.Sprintf("Should contain %s", relativePath), func(t *testing.T) {
			t.Parallel()

			if _, err := os.Stat(filepath.Join(root, relativePath)); err != nil {
				t.Fatalf("expected %s to exist: %v", relativePath, err)
			}
		})
	}

	skillPath := filepath.Join(root, "extensions", "cy-idea-factory", "skills", "cy-idea-factory", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read %s: %v", skillPath, err)
	}

	var metadata struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if _, err := frontmatter.Parse(string(content), &metadata); err != nil {
		t.Fatalf("parse frontmatter %s: %v", skillPath, err)
	}
	if metadata.Name != "cy-idea-factory" {
		t.Fatalf("expected extension skill name cy-idea-factory, got %q", metadata.Name)
	}
	if metadata.Description == "" {
		t.Fatalf("expected non-empty description in %s", skillPath)
	}

	checkPortableContent(t, skillPath)
}

func TestCreateTasksSkillDocumentsTaskTypeRegistryAndValidation(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	skillPath := filepath.Join(root, "skills", "cy-create-tasks", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read %s: %v", skillPath, err)
	}

	text := string(content)
	required := []string{
		"Read `.compozy/config.toml`.",
		"[tasks].types",
		"`frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`",
		"Run `compozy tasks validate --name <feature>`.",
		"until it exits 0.",
	}
	for _, snippet := range required {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected %s to include %q", skillPath, snippet)
		}
	}
}

func TestCreatePRDUserStoriesTemplateRequiresAuthorizationCoverage(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	templatePath := filepath.Join(root, "skills", "cy-create-prd", "references", "user-stories-template.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	required := []string{
		"## Authorization Rule Pack",
		"`create`, `read`, `update`, `delete`, `transition`, and `replay`",
		"Data classification",
		"Actor / role / capability",
		"`allow`, `deny`, `redact`, or `ignore`",
		"Permitted side effects",
		"complete matrix",
		"documented pairwise coverage",
		"without a negative test",
		"client-controlled sensitive field",
		"protected state remains unchanged",
		"field-level read redaction",
	}
	for _, snippet := range required {
		snippet := snippet
		t.Run(snippet, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(string(content), snippet) {
				t.Fatalf("expected %s to include %q", templatePath, snippet)
			}
		})
	}
}

func TestCreatePRDUserStoriesTemplateRequiresUnknownOutcomeRecovery(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	templatePath := filepath.Join(root, "skills", "cy-create-prd", "references", "user-stories-template.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	required := []string{
		"## Uncertain-Outcome Recovery",
		"May execution begin or repeat?",
		"Client response",
		"Durable evidence",
		"Retry after restart or transport failure",
		"`no record`",
		"`pending / incomplete`",
		"`completed success`",
		"`completed failure`",
		"`fingerprint mismatch`",
		"`corrupt / unreadable`",
		"deterministic completed-result replay",
		"documented incomplete-record recovery",
		"mismatch rejection without execution or replay",
		"transport loss after commit",
		"process restart",
		"`UNKNOWN_OUTCOME` only when durable evidence cannot determine the result",
	}
	for _, snippet := range required {
		snippet := snippet
		t.Run(snippet, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(string(content), snippet) {
				t.Fatalf("expected %s to include %q", templatePath, snippet)
			}
		})
	}
}

func TestCreateTechSpecTemplateRequiresOperationalEventContracts(t *testing.T) {
	t.Parallel()

	// Invariant: every mandated operational event has a complete contract and matching test cases.
	root := repoRoot(t)
	templatePath := filepath.Join(root, "skills", "cy-create-techspec", "references", "techspec-template.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	text := string(content)
	required := []string{
		"machine-readable YAML contract",
		"event_contracts:",
		"requirement: \"<required|optional>\"",
		"privacy: \"<privacy classification>\"",
		"request_id:",
		"correlation_id:",
		"actor_id:",
		"resource_id:",
		"allowed_outcomes:",
		"success:",
		"rejection:",
		"replay:",
		"stale_command:",
		"delivery_semantics:",
		"sink_failure:",
		"blocking product decision",
		"Never infer",
		"required fields and outcome enums",
		"forbidden payload fields",
		"event-sink failures",
	}
	for _, snippet := range required {
		snippet := snippet
		t.Run(snippet, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(text, snippet) {
				t.Fatalf("expected %s to include %q", templatePath, snippet)
			}
		})
	}
}

func TestCreateTechSpecTestsTemplateRequiresQuantitativeVerification(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	templatePath := filepath.Join(root, "skills", "cy-create-techspec", "references", "tests-template.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	required := []string{
		"## Quantitative Verification",
		"| Source | Metric | Target value | Measurement method | Test environment | Category | Owning test ID |",
		"`correctness`, `capacity`, or `performance`",
		"repeat and use the exact quantity or threshold",
		"deterministic and reproducible",
		"Block `_tests.md` generation",
	}
	for _, snippet := range required {
		snippet := snippet
		t.Run(snippet, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(string(content), snippet) {
				t.Fatalf("expected %s to include %q", templatePath, snippet)
			}
		})
	}
}

func TestCreateTechSpecTestsTemplateRequiresAtomicityVerification(t *testing.T) {
	t.Parallel()

	templatePath := filepath.Join(
		repoRoot(t),
		"skills",
		"cy-create-techspec",
		"references",
		"tests-template.md",
	)
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	required := []string{
		"multi-write atomicity requirement",
		"`A` fails before any later write",
		"`A` succeeds and `B` fails",
		"`A` and `B` succeed and `C` fails",
		"transaction commit fails",
		"database constraints",
		"transaction callbacks",
		"dedicated test seams",
		"production-only flags",
		"domain, ledger, and event state",
		"nested or bound transaction scopes",
		"retry after rollback",
		"at least one failure-injection test ID",
	}
	for _, snippet := range required {
		snippet := snippet
		t.Run(snippet, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(string(content), snippet) {
				t.Fatalf("expected %s to include %q", templatePath, snippet)
			}
		})
	}
}

func TestTechSpecTemplateRequiresContractRolloutPlanning(t *testing.T) {
	t.Parallel()

	templatePath := filepath.Join(
		repoRoot(t),
		"skills",
		"cy-create-techspec",
		"references",
		"techspec-template.md",
	)
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	requiredInstructions := []string{
		"### Contract Change Analysis",
		"Derive known consumers from repository analysis.",
		"Contract diff:",
		"Active consumers:",
		"choose exactly one of atomic consumer updates, backward compatibility, versioning, content negotiation, feature flag, or temporary adapter",
		"same implementation task or a coordinated task linked by a declared dependency",
		"owner, cleanup condition, and removal task",
		"Block TechSpec generation when an active consumer exists and no rollout strategy is defined.",
	}
	for _, instruction := range requiredInstructions {
		t.Run(instruction, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(string(content), instruction) {
				t.Fatalf("expected %s to include %q", templatePath, instruction)
			}
		})
	}
}

func TestTaskTemplateRequiresObservablePersistenceCriteria(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	templatePath := filepath.Join(root, "skills", "cy-create-tasks", "references", "task-template.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	text := string(content)
	normalizedText := strings.Join(strings.Fields(text), " ")
	requiredPhrases := []struct {
		name string
		want string
	}{
		{
			name: "Should require a property, target, and runtime measurement for every persistence constraint",
			want: "Every persistence constraint MUST name its observable property, target operation or query, and runtime measurement mechanism.",
		},
		{
			name: "Should separate correctness and consistency criteria",
			want: "### Persistence Correctness and Consistency",
		},
		{name: "Should separate performance criteria", want: "### Persistence Performance"},
		{
			name: "Should support exact or bounded statement counts",
			want: "exact or bounded executed SQL statement counts",
		},
		{name: "Should support one-snapshot reads", want: "reads share one snapshot transaction"},
		{name: "Should support shared row and total predicates", want: "row and total queries use the same predicates"},
		{name: "Should support generated query-plan checks", want: "generated query-plan checks"},
		{name: "Should support statement-observer evidence", want: "statement-observer evidence"},
		{
			name: "Should reject criteria without observable assertions",
			want: "Reject a persistence criterion that lacks an observable assertion",
		},
		{
			name: "Should reject tests of implementation constants without database execution",
			want: "implementation constants without executing the database behavior",
		},
	}
	for _, testCase := range requiredPhrases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(normalizedText, testCase.want) {
				t.Fatalf("expected %s to include %q", templatePath, testCase.want)
			}
		})
	}

	criterionPattern := regexp.MustCompile(`(?m)^- Property: \[[^\n]+\]; Target: \[[^\n]+\]; Measurement: \[[^\n]+\]$`)
	if matches := criterionPattern.FindAllString(text, -1); len(matches) != 2 {
		t.Fatalf(
			"expected %s to define two property/target/measurement criterion shapes, got %d",
			templatePath,
			len(matches),
		)
	}
}

func TestCreateTasksTemplateClassifiesAndValidatesFilePaths(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	templatePath := filepath.Join(root, "skills", "cy-create-tasks", "references", "task-template.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read %s: %v", templatePath, err)
	}

	text := string(content)
	required := []string{
		"`existing`",
		"`proposed`",
		"`generated`",
		"`possible`",
		"Every path MUST use exactly one classification",
		"missing or renamed `existing` path as a blocking error",
		"Treat `proposed` and `possible` paths as advisory",
		"derive `Dependent Files` from repository analysis",
		"outcomes, not advisory filenames",
	}
	for _, snippet := range required {
		if !strings.Contains(text, snippet) {
			t.Errorf("expected %s to include %q", templatePath, snippet)
		}
	}
}

func TestTaskDocsOmitLegacyTaskFrontmatterKeys(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	legacyKeyPattern := regexp.MustCompile(`(?m)^[ \t]*(domain|scope):`)

	paths := []string{filepath.Join(root, "README.md")}
	err := filepath.WalkDir(filepath.Join(root, "skills"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk skills directory: %v", err)
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator))), func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if match := legacyKeyPattern.FindString(string(content)); match != "" {
				t.Fatalf("expected %s to omit legacy task frontmatter keys, found %q", path, match)
			}
		})
	}
}

func TestEmbeddedSkillsFSMatchesOnDisk(t *testing.T) {
	t.Parallel()

	t.Run("Should match embedded skills filesystem with the filtered on-disk skills tree", func(t *testing.T) {
		t.Parallel()

		root := repoRoot(t)
		source := filepath.Join(root, "skills")
		sourceTree := snapshotTree(t, source)

		// Filter out non-skill files (embed.go, autoresearch artifacts, etc.)
		wantTree := make(map[string]string, len(sourceTree))
		for p, content := range sourceTree {
			if strings.HasSuffix(p, ".go") {
				continue
			}
			if strings.Contains(p, "autoresearch-") {
				continue
			}
			base := filepath.Base(p)
			if base == ".DS_Store" || strings.HasPrefix(base, ".") {
				continue
			}
			wantTree[p] = content
		}

		embeddedTree := make(map[string]string)
		err := fs.WalkDir(skills.FS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, readErr := fs.ReadFile(skills.FS, p)
			if readErr != nil {
				return readErr
			}
			embeddedTree[p] = string(data)
			return nil
		})
		if err != nil {
			t.Fatalf("walk embedded FS: %v", err)
		}

		if len(embeddedTree) != len(wantTree) {
			t.Fatalf("expected embedded FS to contain %d files, got %d", len(wantTree), len(embeddedTree))
		}
		for p, wantContent := range wantTree {
			gotContent, ok := embeddedTree[p]
			if !ok {
				t.Fatalf("expected embedded FS to contain %s", p)
			}
			if gotContent != wantContent {
				t.Fatalf("expected embedded content for %s to match on-disk source", p)
			}
		}
	})
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

func TestSharedReferenceFilesAreIdentical(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)

	groups := [][]string{
		{
			"skills/cy-create-prd/references/adr-template.md",
			"skills/cy-create-techspec/references/adr-template.md",
			"extensions/cy-idea-factory/skills/cy-idea-factory/references/adr-template.md",
		},
	}

	for _, paths := range groups {
		reference, err := os.ReadFile(filepath.Join(root, paths[0]))
		if err != nil {
			t.Fatalf("read %s: %v", paths[0], err)
		}

		for _, p := range paths[1:] {
			t.Run("Should keep "+p+" identical to "+paths[0], func(t *testing.T) {
				t.Parallel()

				content, err := os.ReadFile(filepath.Join(root, p))
				if err != nil {
					t.Fatalf("read %s: %v", p, err)
				}
				if !bytes.Equal(content, reference) {
					t.Fatalf("expected %s to be identical to %s", p, paths[0])
				}
			})
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
