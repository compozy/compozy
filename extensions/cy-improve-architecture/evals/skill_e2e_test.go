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
			protected := snapshotProtectedFiles(t, workspace)
			installShippedSkills(t, workspace)

			output := executeAudit(t, binary, workspace, test.target)
			assertArtifacts(t, workspace, test.slug, test.wantArea, test.wantEmpty, output)
			assertProtectedFilesUnchanged(t, workspace, protected)
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

func snapshotProtectedFiles(t *testing.T, workspace string) map[string][]byte {
	t.Helper()

	protected := map[string][]byte{}
	for _, name := range []string{".gitignore", "CLAUDE.md", "AGENTS.md"} {
		path := filepath.Join(workspace, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read protected fixture file %s: %v", name, err)
		}
		protected[name] = data
	}
	return protected
}

func installShippedSkills(t *testing.T, workspace string) {
	t.Helper()

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
		AgentNames: []string{"codex"},
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
	command := exec.CommandContext(
		ctx,
		binary,
		"exec",
		"--extensions",
		"--ide", evaluationIDE(),
		"--model", evaluationModel(),
		"--timeout", "10m",
		"--access-mode", "full",
		prompt,
	)
	command.Dir = workspace
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
	if model := os.Getenv("COMPOZY_E2E_MODEL"); model != "" {
		return model
	}
	return "gpt-5.6-sol"
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
		if !bytes.Contains(markdown, []byte("Healthy target — no credible deepening candidate found.")) {
			t.Fatalf("healthy fixture did not produce the zero-candidate outcome\nreport:\n%s", markdown)
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

func assertProtectedFilesUnchanged(t *testing.T, workspace string, before map[string][]byte) {
	t.Helper()

	for name, want := range before {
		got := readFile(t, filepath.Join(workspace, name))
		if !bytes.Equal(got, want) {
			t.Fatalf("protected file %s changed\ngot:\n%s\nwant:\n%s", name, got, want)
		}
	}
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
