package opendesign

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/tools"
)

// Suite: embedded extension product contract. Real parsers and the Loop compiler
// must load the shipped catalog; this is not a snapshot of prose or layout.
func TestEmbeddedResources(t *testing.T) {
	t.Parallel()
	t.Run("Should load the profile resources and hydrate the executable review loop", func(t *testing.T) {
		t.Parallel()
		directory := t.TempDir()
		if err := os.CopyFS(directory, FS()); err != nil {
			t.Fatal(err)
		}
		manifest, err := extensionpkg.LoadManifest(directory)
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Profiles) != 1 || manifest.Profiles[0].Name != Name ||
			manifest.Profiles[0].Defaults.Agent != "open-design-designer" {
			t.Fatalf("profile defaults = %#v", manifest.Profiles)
		}
		// Explicit entrypoints expose only the three supported skills.
		if len(manifest.Resources.Skills) != 3 {
			t.Fatalf("skill declarations = %#v", manifest.Resources.Skills)
		}
		for _, resource := range manifest.Resources.Skills {
			if resource.Profile != Name || filepath.Base(resource.Path) != "SKILL.md" {
				t.Fatalf("skill placement = %#v", resource)
			}
			skill, err := skills.ParseSkillFile(filepath.Join(directory, resource.Path))
			if err != nil || skill == nil || skill.Meta.Name == "" {
				t.Fatalf("skill %s = %#v, %v", resource.Path, skill, err)
			}
		}
		for _, name := range []string{"open-design-designer", "open-design-critic"} {
			data, err := fs.ReadFile(FS(), "agents/"+name+"/AGENT.md")
			if err != nil {
				t.Fatal(err)
			}
			agent, err := config.ParseAgentDef(data)
			if err != nil || agent.Name != name {
				t.Fatalf("agent %s = %#v, %v", name, agent, err)
			}
		}
		descriptors, err := extensionpkg.ResolveManifestToolDescriptors(manifest)
		if err != nil || len(descriptors) != 1 {
			t.Fatalf("tool descriptors = %#v, %v", descriptors, err)
		}
		descriptor := descriptors[0].Tool
		if descriptor.ID.String() != LintToolID || !descriptor.ReadOnly || descriptor.Risk != tools.RiskRead {
			t.Fatalf("lint descriptor = %#v", descriptor)
		}
		described, err := Describe()
		if err != nil || len(described.Tools) != 1 {
			t.Fatalf("describe = %#v, %v", described, err)
		}
		for _, pair := range [][2]json.RawMessage{
			{descriptor.InputSchema, described.Tools[0].InputSchema},
			{descriptor.OutputSchema, described.Tools[0].OutputSchema},
		} {
			left, err := tools.SchemaDigest(pair[0])
			if err != nil {
				t.Fatal(err)
			}
			right, err := tools.SchemaDigest(pair[1])
			if err != nil || left != right {
				t.Fatalf("manifest/provider schema drift: %s != %s, %v", left, right, err)
			}
		}
		source := lintSchemas{snapshot: loop.ToolSchemaSnapshot{
			ToolID: LintToolID, InputSchema: descriptor.InputSchema, OutputSchema: descriptor.OutputSchema,
			InputSchemaDigest: descriptor.InputSchemaDigest, OutputSchemaDigest: descriptor.OutputSchemaDigest,
		}}
		data, err := fs.ReadFile(FS(), "loops/open-design-review/loop.yaml")
		if err != nil {
			t.Fatal(err)
		}
		_, definition, err := loop.ParseResource(data, loop.ResourceParseOptions{
			Source: loop.SourceMarketplace, Dir: "loops/open-design-review",
			FilePath: "loops/open-design-review/loop.yaml",
			Linter:   loop.NewLinter(loop.WithToolSchemaSource(source)),
		})
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := loop.NewCompiler(loop.WithCompilerToolSchemaSource(source)).Compile(definition)
		if err != nil {
			t.Fatal(err)
		}
		// The daemon persists and rehydrates a snapshot before admitting a run.
		// Exercise that boundary: compilation alone cannot prove runtime references.
		effective, err := loop.ResolveEffectiveConfig(resolved, loop.DefaultLoopDefaults(), nil, loop.LoopConfig{
			IterationCap: new(3), ReattemptStrategy: new(loop.ReattemptFullBody),
		})
		if err != nil {
			t.Fatal(err)
		}
		snapshot, digest, err := loop.BuildExecutedDefinitionSnapshot(resolved, effective)
		if err != nil {
			t.Fatal(err)
		}
		hydrated, err := loop.LoadExecutedDefinitionSnapshot(snapshot, digest)
		if err != nil {
			t.Fatal(err)
		}
		testReviewLoopCompletion(t, hydrated)
	})
}

type lintSchemas struct{ snapshot loop.ToolSchemaSnapshot }

var _ loop.ToolSchemaSource = lintSchemas{}

func (s lintSchemas) Snapshot(id string) (loop.ToolSchemaSnapshot, bool) {
	return s.snapshot, id == LintToolID
}

// Invariant: the shipped review Loop completes only for an explicit, evidenced
// approval of the unchanged candidate. Owner: embedded Loop product contract;
// use its real schema validator, template renderer, and compiled CEL predicate.
func testReviewLoopCompletion(t *testing.T, resolved *loop.ResolvedDefinition) {
	t.Helper()
	condition := resolved.Conditions["contract.stop_when"]
	if condition == nil || resolved.Definition.Contract.StopWhen.OnEvalError != dsl.EvalErrorFail {
		t.Fatal("review completion must have a fail-closed condition")
	}
	var criticSchema dsl.Schema
	for _, node := range resolved.Definition.Graph.Nodes {
		if node.ID == "critique" {
			var params dsl.RunAgentParams
			if err := node.Params.Decode(&params); err != nil {
				t.Fatal(err)
			}
			if node.Kind != "run-agent" || node.Session == nil || !node.Session.Isolated {
				t.Fatal("critic must own an isolated tool-capable agent session")
			}
			criticSchema = params.OutputSchema
		}
	}
	if len(criticSchema) == 0 {
		t.Fatal("critic output schema is missing")
	}
	cases := []struct {
		name  string
		patch func(map[string]any)
		want  bool
	}{
		{name: "Should accept an evidenced approval", want: true},
		{name: "Should reject a requested revision", patch: func(v map[string]any) {
			reviewNodeOutput(v, "critique")["verdict"] = "revise"
		}},
		{name: "Should reject approval with remaining blockers", patch: func(v map[string]any) {
			reviewNodeOutput(v, "critique")["blocking_issues"] = []any{map[string]any{"id": "missing_state"}}
		}},
		{name: "Should reject stale review digests", patch: func(v map[string]any) {
			reviewArtifact(v, "critique")["sha256"] = strings.Repeat("b", 64)
		}},
		{name: "Should reject files changed during review", patch: func(v map[string]any) {
			reviewArtifact(v, "verify")["sha256"] = strings.Repeat("b", 64)
		}},
		{name: "Should reject unreviewed files", patch: func(v map[string]any) {
			reviewNodeOutput(v, "critique")["artifacts"] = []any{}
		}},
		{name: "Should reject a different requested entry path", patch: func(v map[string]any) {
			reviewNodeOutput(v, "design")["primary_html_path"] = "docs/design/other.html"
		}},
		{name: "Should accept a matching source-backed lint exception", want: true, patch: func(v map[string]any) {
			addReviewLintFinding(v)
			reviewArtifact(v, "critique")["exceptions"] = []any{map[string]any{
				"id": "color_rule", "severity": "P1", "source": "DESIGN.md palette", "reason": "Approved brand color",
			}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			variables := reviewLoopFixture(t)
			if tc.patch != nil {
				tc.patch(variables)
			}
			if tc.want {
				raw, err := json.Marshal(reviewNodeOutput(variables, "critique"))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := loop.ValidateActionStructured(
					criticSchema,
					loop.ActionPromptResult{Structured: raw},
				); err != nil {
					t.Fatalf("valid review schema: %v", err)
				}
			}
			result, err := condition.Evaluate(variables)
			if err != nil || result.Value != tc.want {
				t.Fatalf("review completion = %v, %v; want %v", result.Value, err, tc.want)
			}
		})
	}
	for _, name := range []string{"design", "lint", "critique", "verify"} {
		t.Run("Should reject failed "+name+" execution", func(t *testing.T) {
			t.Parallel()
			variables := reviewLoopFixture(t)
			variables["nodes"].(map[string]any)[name].(map[string]any)["status"] = "failed"
			result, err := condition.Evaluate(variables)
			if err != nil || result.Value {
				t.Fatalf("failed action completion = %v, %v", result.Value, err)
			}
		})
	}
	t.Run("Should evaluate the full supported artifact batch within the native cost limit", func(t *testing.T) {
		t.Parallel()
		variables := reviewLoopFixture(t)
		variables["inputs"].(map[string]any)["artifact_path"] = "docs/design/board-0.html"
		reviewNodeOutput(variables, "design")["primary_html_path"] = "docs/design/board-0.html"
		paths := make([]any, 0, 32)
		for index := range 32 {
			paths = append(paths, fmt.Sprintf("docs/design/board-%d.html", index))
		}
		reviewNodeOutput(variables, "design")["artifact_paths"] = paths
		for _, node := range []string{"lint", "critique", "verify"} {
			artifacts := make([]any, 0, 32)
			for index := range 32 {
				artifact := map[string]any{
					"path": fmt.Sprintf("docs/design/board-%d.html", index), "sha256": strings.Repeat("a", 64),
				}
				findings := make([]any, 0, 16)
				for finding := range 16 {
					findings = append(
						findings,
						map[string]any{
							"id":       fmt.Sprintf("rule-%d", finding),
							"severity": "P1",
							"message":  "Color differs",
							"fix":      "Use approved color",
						},
					)
				}
				if node == "critique" {
					exceptions := make([]any, 0, len(findings))
					for _, finding := range findings {
						exceptions = append(exceptions, map[string]any{
							"id": finding.(map[string]any)["id"], "severity": "P1",
							"source": "DESIGN.md", "reason": "Approved design authority",
						})
					}
					artifact["evidence"] = "Read the full source and checked the brief"
					artifact["exceptions"] = exceptions
				} else {
					artifact["findings"] = findings
				}
				artifacts = append(artifacts, artifact)
			}
			reviewNodeOutput(variables, node)["artifacts"] = artifacts
		}
		result, err := condition.Evaluate(variables)
		if err != nil || !result.Value || result.CostWarning {
			t.Fatalf("full batch completion = %#v, %v", result, err)
		}
	})
	t.Run("Should reject critic output without actual evidence", func(t *testing.T) {
		t.Parallel()
		variables := reviewLoopFixture(t)
		raw, err := json.Marshal(reviewNodeOutput(variables, "critique"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loop.ValidateActionStructured(criticSchema, loop.ActionPromptResult{Structured: raw}); err != nil {
			t.Fatalf("valid review schema: %v", err)
		}
		reviewArtifact(variables, "critique")["evidence"] = " \t\n"
		raw, err = json.Marshal(reviewNodeOutput(variables, "critique"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loop.ValidateActionStructured(criticSchema, loop.ActionPromptResult{Structured: raw}); err == nil {
			t.Fatal("critic output without evidence was accepted")
		}
		if _, err := loop.ValidateActionStructured(criticSchema, loop.ActionPromptResult{}); err == nil {
			t.Fatal("missing critic output was accepted")
		}
	})
	t.Run("Should expose malformed completion input as an evaluation error", func(t *testing.T) {
		t.Parallel()
		variables := reviewLoopFixture(t)
		delete(reviewNodeOutput(variables, "critique"), "verdict")
		if _, err := condition.Evaluate(variables); err == nil {
			t.Fatal("malformed completion input did not produce an error")
		}
	})
	t.Run("Should carry the prior typed critique and file evidence to the designer", func(t *testing.T) {
		t.Parallel()
		variables := reviewLoopFixture(t)
		variables["generation"] = 2
		variables["previous"] = map[string]any{"generation": 1, "nodes": variables["nodes"]}
		prior := reviewNodeOutput(variables, "critique")
		prior["blocking_issues"] = []any{map[string]any{"id": "missing_state", "correction": "Add the failure state"}}
		template := resolved.Templates["nodes.design.params.prompt"]
		if template == nil {
			t.Fatal("compiled designer prompt missing")
		}
		rendered, err := refs.RenderTemplateString("designer", template.Raw, variables)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(prior)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(rendered, string(raw)) {
			t.Fatal("designer did not receive the prior critic's blockers and file evidence")
		}
	})
}

func reviewLoopFixture(t *testing.T) map[string]any {
	t.Helper()
	const raw = `{
	  "inputs": {"artifact_path": "docs/design/index.html", "brief": "Review selection"},
	  "nodes": {
	    "design": {"status": "succeeded", "output": {
	      "primary_html_path": "docs/design/index.html", "artifact_paths": ["docs/design/index.html"]}},
	    "lint": {"status": "succeeded", "output": {"artifacts": [{
	      "path": "docs/design/index.html", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	      "findings": []}]}},
	    "verify": {"status": "succeeded", "output": {"artifacts": [{
	      "path": "docs/design/index.html", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	      "findings": []}]}},
	    "critique": {"status": "succeeded", "output": {
	      "verdict": "approved", "blocking_issues": [], "limitations": [], "artifacts": [{
	        "path": "docs/design/index.html", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	        "evidence": "Read selection handlers against the brief", "exceptions": []}]}}
	  }
	}`
	var variables map[string]any
	if err := json.Unmarshal([]byte(raw), &variables); err != nil {
		t.Fatal(err)
	}
	return variables
}

func reviewNodeOutput(variables map[string]any, node string) map[string]any {
	return variables["nodes"].(map[string]any)[node].(map[string]any)["output"].(map[string]any)
}

func reviewArtifact(variables map[string]any, node string) map[string]any {
	return reviewNodeOutput(variables, node)["artifacts"].([]any)[0].(map[string]any)
}

func addReviewLintFinding(variables map[string]any) {
	for _, node := range []string{"lint", "verify"} {
		reviewArtifact(variables, node)["findings"] = []any{
			map[string]any{
				"id":       "color_rule",
				"severity": "P1",
				"message":  "Color differs",
				"fix":      "Use approved color",
			},
		}
	}
}
