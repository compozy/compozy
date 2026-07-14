// Suite: architecture audit skill evaluation
// Invariant: a real installed audit skill produces parity-checked reports and a parseable depth map for Go and TypeScript workspaces.
// Boundary IN: shipped skill pack, Compozy ACP execution, and disposable fixture workspaces.
// Boundary OUT: model availability and judgment quality beyond the audited artifacts.
package evals

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/extensions/cy-improve-architecture/archmap"
	"github.com/compozy/compozy/internal/setup"
)

const runSkillE2EEnv = "COMPOZY_RUN_SKILL_E2E"

var shippedSkillNames = []string{
	"cy-codebase-design",
	"cy-domain-modeling",
	"cy-improve-architecture",
}

var (
	markdownCandidateAnchorPattern = regexp.MustCompile(`<a\s+id="candidate-([^"]+)"></a>`)
	htmlCandidateArticlePattern    = regexp.MustCompile(`<article\b[^>]*\bid="candidate-([^"]+)"[^>]*>`)
	markdownTopPickPattern         = regexp.MustCompile(
		`(?s)## Top pick\b.*?- Full evidence: \[[^\]]+\]\(#candidate-([^)]+)\)`,
	)
	htmlTopPickPattern = regexp.MustCompile(`(?s)id="top-pick"[^>]*>.*?href="#candidate-([^"]+)"`)
)

func TestAuditSkillProducesInspectableArtifacts(t *testing.T) {
	if os.Getenv(runSkillE2EEnv) != "1" {
		t.Skipf("set %s=1 with COMPOZY_E2E_BINARY to run the ACP-backed skill evaluation", runSkillE2EEnv)
	}

	binary := requiredEvaluationBinary(t)
	for _, test := range []struct {
		name      string
		fixture   string
		target    string
		slug      string
		wantArea  string
		wantEmpty bool
	}{
		{
			name:     "TypeScript fixture produces a candidate report and depth map",
			fixture:  "typescript",
			target:   "apps/checkout",
			slug:     "apps-checkout",
			wantArea: "apps/checkout",
		},
		{
			name:      "Go functional-options fixture remains a healthy target",
			fixture:   "go",
			target:    "internal/client",
			slug:      "internal-client",
			wantArea:  "internal/client",
			wantEmpty: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := copyFixtureWorkspace(t, test.fixture)
			installShippedSkills(t, workspace)
			fixtureFiles := snapshotFixtureFiles(t, workspace)

			output := executeAudit(t, binary, workspace, test.target)
			assertArtifacts(t, workspace, test.slug, test.wantArea, test.wantEmpty, output)
			assertFixtureFilesUnchanged(t, workspace, fixtureFiles)
		})
	}
}

func TestCandidateParity(t *testing.T) {
	t.Parallel()

	const matchingMarkdown = `## Top pick

- Full evidence: [Order intake](#candidate-order-intake)

## Candidates

<a id="candidate-order-intake"></a>

### Order intake

<a id="candidate-payment-service"></a>

### Payment service
`
	const matchingHTML = `<section id="top-pick"><a href="#candidate-order-intake">Full evidence</a></section>
<section id="candidates">
  <article id="candidate-order-intake"></article>
  <article id="candidate-payment-service"></article>
</section>
`

	for _, test := range []struct {
		name        string
		markdown    string
		html        string
		wantErrPart string
	}{
		{
			name:     "accepts matching ordered candidate IDs and top pick",
			markdown: matchingMarkdown,
			html:     matchingHTML,
		},
		{
			name:     "rejects reordered HTML candidates",
			markdown: matchingMarkdown,
			html: `<section id="top-pick"><a href="#candidate-order-intake">Full evidence</a></section>
<section id="candidates">
  <article id="candidate-payment-service"></article>
  <article id="candidate-order-intake"></article>
</section>
`,
			wantErrPart: "candidate IDs differ",
		},
		{
			name:     "rejects a mismatched HTML top pick",
			markdown: matchingMarkdown,
			html: strings.Replace(
				matchingHTML,
				"#candidate-order-intake",
				"#candidate-payment-service",
				1,
			),
			wantErrPart: "top-pick candidate IDs differ",
		},
		{
			name:        "rejects Markdown without candidate anchors",
			markdown:    "## Top pick\n\n- Full evidence: [Order intake](#candidate-order-intake)\n",
			html:        matchingHTML,
			wantErrPart: "markdown report has no candidate anchors",
		},
		{
			name:        "rejects HTML without candidate articles",
			markdown:    matchingMarkdown,
			html:        `<section id="top-pick"><a href="#candidate-order-intake">Full evidence</a></section>`,
			wantErrPart: "HTML report has no candidate articles",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := candidateParityError([]byte(test.markdown), []byte(test.html))
			if test.wantErrPart == "" {
				if err != nil {
					t.Fatalf("candidate parity returned an unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("candidate parity error = %v, want it to contain %q", err, test.wantErrPart)
			}
		})
	}
}

func TestFixtureFileChangeError(t *testing.T) {
	t.Parallel()

	before := map[string][]byte{
		"apps/checkout/src/place-order.ts": []byte("export function placeOrder() {}\n"),
	}
	for _, test := range []struct {
		name        string
		after       map[string][]byte
		wantErrPart string
	}{
		{
			name:  "accepts an unchanged fixture",
			after: before,
		},
		{
			name: "allows installed skill files",
			after: map[string][]byte{
				"apps/checkout/src/place-order.ts":                []byte("export function placeOrder() {}\n"),
				".agents/skills/cy-improve-architecture/SKILL.md": []byte("---\nname: cy-improve-architecture\n"),
			},
		},
		{
			name: "allows generated audit artifacts",
			after: map[string][]byte{
				"apps/checkout/src/place-order.ts":       []byte("export function placeOrder() {}\n"),
				".compozy/arch-reviews/apps-checkout.md": []byte("# Architecture audit\n"),
			},
		},
		{
			name: "rejects a modified source file",
			after: map[string][]byte{
				"apps/checkout/src/place-order.ts": []byte("export function placeOrder() { return 1 }\n"),
			},
			wantErrPart: "changed",
		},
		{
			name:        "rejects a deleted source file",
			after:       map[string][]byte{},
			wantErrPart: "is missing",
		},
		{
			name: "rejects a new source file",
			after: map[string][]byte{
				"apps/checkout/src/place-order.ts": []byte("export function placeOrder() {}\n"),
				"apps/checkout/src/fraud-check.ts": []byte("export function checkFraud() {}\n"),
			},
			wantErrPart: "unexpected",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := fixtureFileChangeError(before, test.after)
			if test.wantErrPart == "" {
				if err != nil {
					t.Fatalf("fixture change check returned an unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("fixture change check error = %v, want it to contain %q", err, test.wantErrPart)
			}
		})
	}
}

func TestHealthyReport(t *testing.T) {
	t.Parallel()

	const healthyDepthMap = `# Architecture Depth Map (active)
# @import'd into agent memory. Route behavior INTO deep modules; do NOT widen seams;
# do NOT re-propose avoided deepenings. Detail: .compozy/arch-reviews/<area>.md

## internal/client | audited 2026-07-13 | report .compozy/arch-reviews/internal-client.md
# no deepening opportunities as of 2026-07-13
`
	const healthyMarkdown = `# Architecture audit: internal/client

## Top pick

Healthy target — no credible deepening candidate found.

## Candidates

No candidates.
`
	const healthyHTML = `<section id="top-pick"><div>Healthy target</div></section>
<section id="candidates"></section>
`

	for _, test := range []struct {
		name        string
		depthMap    string
		markdown    string
		html        string
		wantErrPart string
	}{
		{
			name:     "accepts a complete healthy artifact set",
			depthMap: healthyDepthMap,
			markdown: healthyMarkdown,
			html:     healthyHTML,
		},
		{
			name: "rejects deep guidance in a healthy map",
			depthMap: strings.Replace(
				healthyDepthMap,
				"# no deepening opportunities as of 2026-07-13",
				"deep | internal/client | Centralize client construction.\n# no deepening opportunities as of 2026-07-13",
				1,
			),
			markdown:    healthyMarkdown,
			html:        healthyHTML,
			wantErrPart: "depth-map entries",
		},
		{
			name: "rejects seam guidance in a healthy map",
			depthMap: strings.Replace(
				healthyDepthMap,
				"# no deepening opportunities as of 2026-07-13",
				"seam | internal/client | Keep the adapter boundary narrow.\n# no deepening opportunities as of 2026-07-13",
				1,
			),
			markdown:    healthyMarkdown,
			html:        healthyHTML,
			wantErrPart: "depth-map entries",
		},
		{
			name: "rejects a missing dated no-opportunities map comment",
			depthMap: strings.Replace(
				healthyDepthMap,
				"# no deepening opportunities as of 2026-07-13",
				"# no deepening opportunities as of 2026-07-12",
				1,
			),
			markdown:    healthyMarkdown,
			html:        healthyHTML,
			wantErrPart: "dated no-opportunities comment",
		},
		{
			name:        "rejects Markdown without the no-candidates outcome",
			depthMap:    healthyDepthMap,
			markdown:    strings.Replace(healthyMarkdown, "No candidates.", "Candidates are pending.", 1),
			html:        healthyHTML,
			wantErrPart: "lacks no-candidates outcome",
		},
		{
			name:        "rejects a Markdown candidate anchor",
			depthMap:    healthyDepthMap,
			markdown:    healthyMarkdown + "\n<a id=\"candidate-fabricated\"></a>\n",
			html:        healthyHTML,
			wantErrPart: "candidate anchor",
		},
		{
			name:        "rejects an HTML candidate article",
			depthMap:    healthyDepthMap,
			markdown:    healthyMarkdown,
			html:        healthyHTML + "<article id=\"candidate-fabricated\"></article>\n",
			wantErrPart: "candidate article",
		},
		{
			name:        "rejects an HTML report without the healthy outcome",
			depthMap:    healthyDepthMap,
			markdown:    healthyMarkdown,
			html:        strings.Replace(healthyHTML, "Healthy target", "No recommendation", 1),
			wantErrPart: "lacks healthy target outcome",
		},
		{
			name:     "rejects an HTML candidate-targeting top-pick CTA",
			depthMap: healthyDepthMap,
			markdown: healthyMarkdown,
			html: strings.Replace(
				healthyHTML,
				"<div>Healthy target</div>",
				"<a href=\"#candidate-fabricated\">Healthy target</a>",
				1,
			),
			wantErrPart: "candidate-targeting top-pick CTA",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := archmap.Parse([]byte(test.depthMap))
			if err != nil {
				t.Fatalf("parse depth map: %v", err)
			}
			area := findArea(parsed, "internal/client")
			if area == nil {
				t.Fatal("parsed depth map has no internal/client area")
			}

			err = healthyReportError(area, []byte(test.depthMap), []byte(test.markdown), []byte(test.html))
			if test.wantErrPart == "" {
				if err != nil {
					t.Fatalf("healthy report returned an unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("healthy report error = %v, want it to contain %q", err, test.wantErrPart)
			}
		})
	}
}

func TestInstallShippedSkillsUsesSelectedEvaluationRuntime(t *testing.T) {
	for _, test := range []struct {
		name              string
		ide               string
		installedSkillDir string
		otherSkillDir     string
	}{
		{
			name:              "defaults to Codex",
			installedSkillDir: filepath.Join(".agents", "skills"),
			otherSkillDir:     filepath.Join(".claude", "skills"),
		},
		{
			name:              "maps Claude to its project skill directory",
			ide:               "claude",
			installedSkillDir: filepath.Join(".claude", "skills"),
			otherSkillDir:     filepath.Join(".agents", "skills"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("COMPOZY_E2E_IDE", test.ide)

			workspace := t.TempDir()
			installShippedSkills(t, workspace)

			installedSkill := filepath.Join(
				workspace,
				test.installedSkillDir,
				"cy-improve-architecture",
				"SKILL.md",
			)
			if _, err := os.Stat(installedSkill); err != nil {
				t.Fatalf("stat installed evaluation skill %s: %v", installedSkill, err)
			}

			otherSkill := filepath.Join(
				workspace,
				test.otherSkillDir,
				"cy-improve-architecture",
				"SKILL.md",
			)
			if _, err := os.Stat(otherSkill); err == nil {
				t.Fatalf("evaluation skill unexpectedly installed for another runtime at %s", otherSkill)
			} else if !os.IsNotExist(err) {
				t.Fatalf("stat other-runtime evaluation skill %s: %v", otherSkill, err)
			}
		})
	}
}

func TestAuditCommandUsesSelectedRuntimeDefaultModel(t *testing.T) {
	for _, test := range []struct {
		name      string
		ide       string
		model     string
		wantIDE   string
		wantModel string
		hasModel  bool
	}{
		{
			name:    "Codex defers to its registered default model",
			wantIDE: "codex",
		},
		{
			name:    "Claude defers to its registered default model",
			ide:     "claude",
			wantIDE: "claude",
		},
		{
			name:      "explicit model override is forwarded unchanged",
			ide:       "claude",
			model:     "opus",
			wantIDE:   "claude",
			wantModel: "opus",
			hasModel:  true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("COMPOZY_E2E_IDE", test.ide)
			t.Setenv("COMPOZY_E2E_MODEL", test.model)

			command := auditCommand(context.Background(), "compozy", t.TempDir(), "internal/client")
			gotIDE, hasIDE := commandFlagValue(command.Args, "--ide")
			if !hasIDE || gotIDE != test.wantIDE {
				t.Fatalf("audit command IDE = %q, present = %t; want %q, present = true", gotIDE, hasIDE, test.wantIDE)
			}
			gotModel, hasModel := commandFlagValue(command.Args, "--model")
			if hasModel != test.hasModel || gotModel != test.wantModel {
				t.Fatalf(
					"audit command model = %q, present = %t; want %q, present = %t",
					gotModel,
					hasModel,
					test.wantModel,
					test.hasModel,
				)
			}
		})
	}
}

func requiredEvaluationBinary(t *testing.T) string {
	t.Helper()

	binary := os.Getenv("COMPOZY_E2E_BINARY")
	if binary == "" {
		t.Fatal("COMPOZY_E2E_BINARY must name a built compozy binary")
	}
	info, err := os.Stat(binary)
	if err != nil {
		t.Fatalf("stat COMPOZY_E2E_BINARY %q: %v", binary, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		t.Fatalf("COMPOZY_E2E_BINARY %q is not an executable file", binary)
	}
	return binary
}

func copyFixtureWorkspace(t *testing.T, fixture string) string {
	t.Helper()

	workspace := t.TempDir()
	fixtureRoot := filepath.Join(evaluationRoot(t), "testdata", fixture)
	if err := copyTree(fixtureRoot, workspace); err != nil {
		t.Fatalf("copy %s fixture: %v", fixture, err)
	}
	return workspace
}

func evaluationRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate evaluation source directory")
	}
	return filepath.Dir(file)
}

func extensionRoot(t *testing.T) string {
	t.Helper()
	return filepath.Dir(evaluationRoot(t))
}

func copyTree(source string, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}

func snapshotFixtureFiles(t *testing.T, workspace string) map[string][]byte {
	t.Helper()

	files := map[string][]byte{}
	err := filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("fixture path %s is not a regular file", path)
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot fixture workspace: %v", err)
	}
	return files
}

func installShippedSkills(t *testing.T, workspace string) {
	t.Helper()

	agentName, err := setup.AgentNameForIDE(evaluationIDE())
	if err != nil {
		t.Fatalf("map evaluation IDE %q to setup agent: %v", evaluationIDE(), err)
	}

	root := extensionRoot(t)
	packs := make([]setup.SkillPackSource, 0, len(shippedSkillNames))
	for _, name := range shippedSkillNames {
		packs = append(packs, setup.ExtensionSkillPackSources([]setup.Skill{{
			Name:            name,
			Origin:          setup.AssetOriginExtension,
			ExtensionName:   "cy-improve-architecture",
			ExtensionSource: "bundled",
			ManifestPath:    filepath.Join(root, "extension.toml"),
			ResolvedPath:    filepath.Join(root, "skills", name),
		}})...)
	}
	result, err := setup.InstallExtensionSkillPacks(setup.ExtensionInstallConfig{
		ResolverOptions: setup.ResolverOptions{
			CWD:     workspace,
			HomeDir: t.TempDir(),
		},
		Packs:      packs,
		AgentNames: []string{agentName},
		Mode:       setup.InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install shipped skill pack: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("install shipped skill pack failures: %#v", result.Failed)
	}
	if len(result.Successful) != len(shippedSkillNames) {
		t.Fatalf("installed skill count = %d, want %d", len(result.Successful), len(shippedSkillNames))
	}
}

func executeAudit(t *testing.T, binary string, workspace string, target string) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	t.Cleanup(cancel)
	prompt := fmt.Sprintf(
		"Use the installed cy-improve-architecture skill to audit %q. Execute the complete skill now, "+
			"not merely a plan. This is a disposable evaluation workspace: produce every required report and "+
			"depth-map artifact, preserve protected instructions and configuration, and report any unavailable "+
			"optional companion as the skill requires.",
		target,
	)
	command := auditCommand(ctx, binary, workspace, prompt)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("execute installed audit skill: %v\noutput:\n%s", err, output)
	}
	return output
}

func evaluationIDE() string {
	if ide := os.Getenv("COMPOZY_E2E_IDE"); ide != "" {
		return ide
	}
	return "codex"
}

func evaluationModel() string {
	return os.Getenv("COMPOZY_E2E_MODEL")
}

func auditCommand(ctx context.Context, binary string, workspace string, prompt string) *exec.Cmd {
	arguments := []string{
		"exec",
		"--extensions",
		"--ide", evaluationIDE(),
	}
	if model := evaluationModel(); model != "" {
		arguments = append(arguments, "--model", model)
	}
	arguments = append(arguments,
		"--timeout", "10m",
		"--access-mode", "full",
		prompt,
	)
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = workspace
	return command
}

func commandFlagValue(arguments []string, flag string) (string, bool) {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == flag {
			return arguments[index+1], true
		}
	}
	return "", false
}

func assertArtifacts(t *testing.T, workspace string, slug string, areaName string, wantEmpty bool, output []byte) {
	t.Helper()

	reports := filepath.Join(workspace, ".compozy", "arch-reviews")
	markdownPath := filepath.Join(reports, slug+".md")
	htmlPath := filepath.Join(reports, slug+".html")
	markdown := readFile(t, markdownPath)
	html := readFile(t, htmlPath)
	if !bytes.Contains(markdown, []byte("# Architecture audit:")) {
		t.Fatalf("markdown report missing audit heading\noutput:\n%s", output)
	}
	if !bytes.Contains(markdown, []byte("## Top pick")) || !bytes.Contains(markdown, []byte("## Candidates")) {
		t.Fatalf("markdown report lacks required stable sections\noutput:\n%s", output)
	}
	if !bytes.Contains(html, []byte("id=\"top-pick\"")) || !bytes.Contains(html, []byte("id=\"candidates\"")) {
		t.Fatalf("HTML report lacks required candidate sections\noutput:\n%s", output)
	}

	depthMap := readFile(t, filepath.Join(workspace, ".compozy", "ARCHITECTURE.md"))
	parsed, err := archmap.Parse(depthMap)
	if err != nil {
		t.Fatalf("parse produced ARCHITECTURE.md: %v\nmap:\n%s", err, depthMap)
	}
	area := findArea(parsed, areaName)
	if area == nil {
		t.Fatalf("produced map has no %q area: %#v", areaName, parsed.Areas)
	}
	wantReport := ".compozy/arch-reviews/" + slug + ".md"
	if area.Report != wantReport {
		t.Fatalf("area report = %q, want %q", area.Report, wantReport)
	}
	if wantEmpty {
		if err := healthyReportError(area, depthMap, markdown, html); err != nil {
			t.Fatalf(
				"healthy fixture violates the zero-candidate contract: %v\nmap:\n%s\nmarkdown:\n%s\nHTML:\n%s",
				err,
				depthMap,
				markdown,
				html,
			)
		}
		return
	}
	if len(area.Entries) == 0 {
		t.Fatalf("candidate fixture produced no depth-map guidance\nreport:\n%s", markdown)
	}
	if !bytes.Contains(markdown, []byte("```mermaid")) {
		t.Fatalf("candidate report lacks Mermaid evidence\nreport:\n%s", markdown)
	}
	if err := candidateParityError(markdown, html); err != nil {
		t.Fatalf("candidate reports violate parity: %v\nmarkdown:\n%s\nHTML:\n%s", err, markdown, html)
	}
}

func healthyReportError(area *archmap.Area, depthMap []byte, markdown []byte, html []byte) error {
	if len(area.Entries) != 0 {
		return fmt.Errorf("healthy report has %d depth-map entries, want 0", len(area.Entries))
	}
	noOpportunitiesComment := fmt.Sprintf("# no deepening opportunities as of %s", area.Audited)
	if !bytes.Contains(depthMapAreaSection(depthMap, area), []byte(noOpportunitiesComment)) {
		return fmt.Errorf(
			"depth map area %q lacks dated no-opportunities comment %q",
			area.Name,
			noOpportunitiesComment,
		)
	}
	if !bytes.Contains(markdown, []byte("Healthy target — no credible deepening candidate found.")) {
		return fmt.Errorf("markdown healthy report lacks healthy target outcome")
	}
	if !bytes.Contains(markdown, []byte("No candidates.")) {
		return fmt.Errorf("markdown healthy report lacks no-candidates outcome")
	}
	if bytes.Contains(markdown, []byte("candidate-")) {
		return fmt.Errorf("markdown healthy report has candidate anchor reference")
	}
	if !bytes.Contains(html, []byte("Healthy target")) {
		return fmt.Errorf("HTML healthy report lacks healthy target outcome")
	}
	if htmlCandidateArticlePattern.Match(html) {
		return fmt.Errorf("HTML healthy report has candidate article")
	}
	if htmlTopPickPattern.Match(html) {
		return fmt.Errorf("HTML healthy report has candidate-targeting top-pick CTA")
	}
	return nil
}

func depthMapAreaSection(depthMap []byte, area *archmap.Area) []byte {
	headerPrefix := []byte(fmt.Sprintf("## %s | audited %s |", area.Name, area.Audited))
	lines := bytes.Split(depthMap, []byte("\n"))
	section := make([][]byte, 0, len(lines))
	inArea := false
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("## ")) {
			if inArea {
				break
			}
			inArea = bytes.HasPrefix(line, headerPrefix)
		}
		if inArea {
			section = append(section, line)
		}
	}
	return bytes.Join(section, []byte("\n"))
}

func candidateParityError(markdown []byte, html []byte) error {
	markdownIDs := candidateIDs(markdownCandidateAnchorPattern, markdown)
	if len(markdownIDs) == 0 {
		return fmt.Errorf("markdown report has no candidate anchors")
	}
	htmlIDs := candidateIDs(htmlCandidateArticlePattern, html)
	if len(htmlIDs) == 0 {
		return fmt.Errorf("HTML report has no candidate articles")
	}
	if !sameCandidateIDs(markdownIDs, htmlIDs) {
		return fmt.Errorf("candidate IDs differ: markdown=%q HTML=%q", markdownIDs, htmlIDs)
	}

	markdownTopPickID, err := topPickCandidateID(markdownTopPickPattern, markdown, "markdown")
	if err != nil {
		return err
	}
	if !containsCandidateID(markdownIDs, markdownTopPickID) {
		return fmt.Errorf("markdown top-pick candidate ID %q has no candidate anchor", markdownTopPickID)
	}
	htmlTopPickID, err := topPickCandidateID(htmlTopPickPattern, html, "HTML")
	if err != nil {
		return err
	}
	if !containsCandidateID(htmlIDs, htmlTopPickID) {
		return fmt.Errorf("HTML top-pick candidate ID %q has no candidate article", htmlTopPickID)
	}
	if markdownTopPickID != htmlTopPickID {
		return fmt.Errorf("top-pick candidate IDs differ: markdown=%q HTML=%q", markdownTopPickID, htmlTopPickID)
	}
	return nil
}

func candidateIDs(pattern *regexp.Regexp, report []byte) []string {
	matches := pattern.FindAllSubmatch(report, -1)
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, string(match[1]))
	}
	return ids
}

func sameCandidateIDs(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func topPickCandidateID(pattern *regexp.Regexp, report []byte, reportKind string) (string, error) {
	matches := pattern.FindAllSubmatch(report, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("%s report has %d top-pick candidate links, want 1", reportKind, len(matches))
	}
	return string(matches[0][1]), nil
}

func containsCandidateID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func findArea(depthMap *archmap.Map, name string) *archmap.Area {
	for index := range depthMap.Areas {
		if depthMap.Areas[index].Name == name {
			return &depthMap.Areas[index]
		}
	}
	return nil
}

func assertFixtureFilesUnchanged(t *testing.T, workspace string, before map[string][]byte) {
	t.Helper()

	after := snapshotFixtureFiles(t, workspace)
	if err := fixtureFileChangeError(before, after); err != nil {
		t.Fatal(err)
	}
}

func fixtureFileChangeError(before map[string][]byte, after map[string][]byte) error {
	for name, want := range before {
		if isAllowedGeneratedFixtureFile(name) {
			continue
		}
		got, ok := after[name]
		if !ok {
			return fmt.Errorf("fixture file %s is missing", name)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("fixture file %s changed", name)
		}
	}
	for name := range after {
		if isAllowedGeneratedFixtureFile(name) {
			continue
		}
		if _, ok := before[name]; !ok {
			return fmt.Errorf("unexpected fixture file %s", name)
		}
	}
	return nil
}

func isAllowedGeneratedFixtureFile(name string) bool {
	return strings.HasPrefix(name, ".agents/skills/") ||
		name == ".compozy/ARCHITECTURE.md" ||
		strings.HasPrefix(name, ".compozy/arch-reviews/")
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestRequiredScenarioCatalogIsComplete(t *testing.T) {
	t.Parallel()

	catalog := string(readFile(t, filepath.Join(evaluationRoot(t), "SCENARIOS.md")))
	for number := 1; number <= 42; number++ {
		id := fmt.Sprintf("E2E-%03d", number)
		if !strings.Contains(catalog, id) {
			t.Fatalf("scenario catalog does not cover %s", id)
		}
	}
	if !scenarioIsDeferred(catalog) {
		t.Fatal("scenario catalog must identify E2E-028 as deferred")
	}
}

func scenarioIsDeferred(catalog string) bool {
	for _, line := range strings.Split(catalog, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			continue
		}
		if strings.TrimSpace(fields[1]) == "E2E-028" && strings.TrimSpace(fields[2]) == "deferred" {
			return true
		}
	}
	return false
}
