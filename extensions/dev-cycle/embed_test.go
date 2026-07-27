package devcycle

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestEmbeddedLoopsShouldCompileWithDevCycleToolSchemas(t *testing.T) {
	t.Parallel()

	source := devCycleToolSchemaSource(t)
	compiler := loop.NewCompiler(loop.WithCompilerToolSchemaSource(source))
	linter := loop.NewLinter(loop.WithToolSchemaSource(source))
	loopFiles := embeddedLoopFiles(t)
	if len(loopFiles) == 0 {
		t.Fatal("embedded loop files = 0, want at least one dev-cycle loop")
	}

	for _, path := range loopFiles {
		t.Run("Should compile "+strings.TrimSuffix(filepath.Base(filepath.Dir(path)), ".yaml"), func(t *testing.T) {
			t.Parallel()

			data, err := fs.ReadFile(FS(), path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}
			spec, def, err := loop.ParseResource(data, loop.ResourceParseOptions{
				Source:   loop.SourceMarketplace,
				Dir:      filepath.ToSlash(filepath.Dir(path)),
				FilePath: filepath.ToSlash(path),
				Linter:   linter,
			})
			if err != nil {
				t.Fatalf("ParseResource(%q) error = %v", path, err)
			}
			if spec.Name == "" {
				t.Fatalf("ParseResource(%q) returned empty loop name", path)
			}
			if _, err := compiler.Compile(def); err != nil {
				t.Fatalf("Compile(%q) error = %v", path, err)
			}
		})
	}
}

func TestDevCycleRuntimeToolDescriptorsShouldPinSchemaDigests(t *testing.T) {
	t.Run("Should pin managed tool schema digests and policies", func(t *testing.T) {
		t.Parallel()

		want := map[string]struct {
			inputDigest  string
			outputDigest string
			readOnly     bool
			risk         toolspkg.RiskClass
		}{
			toolImportTasks: {
				inputDigest:  "ff6206bbb7edbf85229a394c4752286046cdbc069b1a89b3067f139f1f68a832",
				outputDigest: "084491ee6855dd4b58a53ac222f7eeebe59d8f53eba12e7ed4583c58fee3d1cf",
				readOnly:     true,
				risk:         toolspkg.RiskRead,
			},
			toolWriteArtifacts: {
				inputDigest:  "7834650b96b2c8eb7190656b380d1c85541c7ebdc47c6be5a4550f6491602716",
				outputDigest: "fc17d6fe3eabaab11c8cec9c79edda8a860f5012c3bb0f5ad17a5ec48f359219",
				readOnly:     false,
				risk:         toolspkg.RiskMutating,
			},
			toolFinalizeRound: {
				inputDigest:  "1edaf49be27be8c496cf20384c75219558bbd5d7e5a7c34c84c4e192c4c45972",
				outputDigest: "278a9a459aa25e19b2558ca21ff698f69ea686a7f733da48b7da4f8ad9a43f3a",
				readOnly:     false,
				risk:         toolspkg.RiskMutating,
			},
		}
		descriptors, err := runtimeToolDescriptors()
		if err != nil {
			t.Fatalf("runtimeToolDescriptors() error = %v", err)
		}
		found := make(map[string]bool, len(want))
		for _, descriptor := range descriptors {
			expected, ok := want[descriptor.Handler]
			if !ok {
				continue
			}
			found[descriptor.Handler] = true
			if got, wantID := descriptor.ID.String(), devCycleToolID(t, descriptor.Handler); got != wantID {
				t.Fatalf("%s id = %q, want %q", descriptor.Handler, got, wantID)
			}
			if descriptor.ReadOnly != expected.readOnly {
				t.Fatalf("%s read_only = %t, want %t", descriptor.Handler, descriptor.ReadOnly, expected.readOnly)
			}
			if got := descriptor.Risk; got != expected.risk {
				t.Fatalf("%s risk = %q, want %q", descriptor.Handler, got, expected.risk)
			}
			if got := descriptor.InputSchemaDigest; got != expected.inputDigest {
				t.Errorf("%s input digest = %q, want %q", descriptor.Handler, got, expected.inputDigest)
			}
			if got := descriptor.OutputSchemaDigest; got != expected.outputDigest {
				t.Errorf("%s output digest = %q, want %q", descriptor.Handler, got, expected.outputDigest)
			}
		}
		for handler := range want {
			if !found[handler] {
				t.Fatalf("runtimeToolDescriptors() missing %s descriptor", handler)
			}
		}
	})
}

func TestDevCycleManagedInstallShouldPublishManagedManifestTools(t *testing.T) {
	t.Run("Should publish managed tools through the bundled manifest install", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		globalDB, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := globalDB.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(globalDB) error = %v", err)
			}
		})
		registry := extensionpkg.NewRegistry(globalDB.DB())

		if err := EnsureManagedInstall(homePaths, registry); err != nil {
			t.Fatalf("EnsureManagedInstall() error = %v", err)
		}
		installed, err := registry.Get(Name)
		if err != nil {
			t.Fatalf("registry.Get(%q) error = %v", Name, err)
		}
		if installed.Source != extensionpkg.SourceBundled || !installed.Enabled {
			t.Fatalf("installed dev-cycle = %#v, want enabled bundled extension", installed)
		}

		manifest, err := extensionpkg.LoadManifest(filepath.Dir(installed.ManifestPath))
		if err != nil {
			t.Fatalf("LoadManifest(%q) error = %v", installed.ManifestPath, err)
		}
		want := map[string]struct {
			readOnly        bool
			concurrencySafe bool
			risk            toolspkg.RiskClass
		}{
			toolImportTasks:    {readOnly: true, concurrencySafe: true, risk: toolspkg.RiskRead},
			toolWriteArtifacts: {readOnly: false, concurrencySafe: true, risk: toolspkg.RiskMutating},
			toolFinalizeRound:  {readOnly: false, concurrencySafe: false, risk: toolspkg.RiskMutating},
		}
		for handler, expected := range want {
			tool, ok := manifest.Resources.Tools[handler]
			if !ok {
				t.Fatalf("manifest tools = %#v, want %q", manifest.Resources.Tools, handler)
			}
			if tool.Handler != handler ||
				tool.Backend.Kind != "extension_host" ||
				tool.Backend.Handler != handler {
				t.Fatalf(
					"%s backend = %#v, handler %q; want extension_host %s",
					handler,
					tool.Backend,
					tool.Handler,
					handler,
				)
			}
			if tool.ReadOnly != expected.readOnly ||
				tool.ConcurrencySafe != expected.concurrencySafe ||
				tool.Risk != string(expected.risk) {
				t.Fatalf(
					"%s tool policy = %#v, want read_only=%t concurrency_safe=%t risk=%q",
					handler,
					tool,
					expected.readOnly,
					expected.concurrencySafe,
					expected.risk,
				)
			}
			if len(tool.InputSchema) == 0 || len(tool.OutputSchema) == 0 {
				t.Fatalf(
					"%s schemas are empty: input=%d output=%d",
					handler,
					len(tool.InputSchema),
					len(tool.OutputSchema),
				)
			}
		}

		manifestDescriptors, err := extensionpkg.ResolveManifestToolDescriptors(manifest)
		if err != nil {
			t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
		}
		runtimeDescriptors, err := runtimeToolDescriptors()
		if err != nil {
			t.Fatalf("runtimeToolDescriptors() error = %v", err)
		}
		runtimeByID := make(map[toolspkg.ToolID]toolspkg.ExtensionToolRuntimeDescriptor, len(runtimeDescriptors))
		for _, descriptor := range runtimeDescriptors {
			runtimeByID[descriptor.ID] = descriptor
		}
		if got, wantCount := len(manifestDescriptors), len(runtimeByID); got != wantCount {
			t.Fatalf("manifest descriptor count = %d, runtime descriptor count = %d", got, wantCount)
		}
		state := extensionpkg.ExtensionToolRuntimeState{
			Enabled:              true,
			Active:               true,
			Healthy:              true,
			ProvidedCapabilities: []string{"tool.provider"},
		}
		for index := range manifestDescriptors {
			descriptor := &manifestDescriptors[index]
			runtimeDescriptor, ok := runtimeByID[descriptor.Tool.ID]
			if !ok {
				t.Fatalf("runtime descriptors omit %q", descriptor.Tool.ID)
			}
			availability := extensionpkg.ReconcileManifestToolRuntime(descriptor, &runtimeDescriptor, state)
			if !availability.Executable {
				t.Fatalf(
					"manifest/runtime descriptor %q is not executable: reasons=%v manifest=%#v runtime=%#v",
					descriptor.Tool.ID,
					availability.ReasonCodes,
					descriptor.RuntimeDescriptor,
					runtimeDescriptor,
				)
			}
		}
	})
}

func TestEmbeddedLoopsShouldKeepDevCycleRuntimeContracts(t *testing.T) {
	t.Run("Should keep the internal review source and clean round termination contract", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/review-and-fix/loop.yaml")
		if got, want := def.Contract.StopWhen,
			"nodes.review.status == 'succeeded' && size(nodes.review.output.issues) == 0"; got != want {
			t.Fatalf("review-and-fix stop_when = %q, want %q", got, want)
		}
		if got, want := def.Contract.IterationCap, 3; got != want {
			t.Fatalf("review-and-fix iteration_cap = %d, want %d", got, want)
		}
		if !hasStartKind(def, dsl.StartTrigger) || hasStartKind(def, dsl.StartSchedule) {
			t.Fatalf("review-and-fix start = %#v, want trigger and no schedule", def.Start)
		}
		review := requireDevCycleNode(t, def, "review")
		if got, want := review.Kind, string(dsl.ActionRunAgent); got != want {
			t.Fatalf("review kind = %q, want %q", got, want)
		}
		outputSchema := requireSchemaObject(t, review.Params, "output_schema")
		properties := requireSchemaObject(t, outputSchema, "properties")
		issues := requireSchemaObject(t, properties, "issues")
		item := requireSchemaObject(t, issues, "items")
		required, ok := item["required"].([]any)
		if !ok {
			t.Fatalf("review issue required = %#v, want []any", item["required"])
		}
		for _, field := range []string{reviewIssueFieldTitle, reviewIssueFieldBody, reviewIssueFieldSeverity} {
			if !schemaStringListContains(required, field) {
				t.Fatalf("review issue required = %#v, want %q", required, field)
			}
		}
		hasIssues := requireDevCycleNode(t, def, "has_issues")
		if got, want := hasIssues.Condition, "size(nodes.review.output.issues) > 0"; got != want {
			t.Fatalf("has_issues condition = %q, want %q", got, want)
		}
	})

	t.Run("Should keep the review write fix finalize graph and local execution modes", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/review-and-fix/loop.yaml")
		wantInputs := []string{"task_name", "reviewer", "fixer", "auto_commit"}
		if len(def.Inputs) != len(wantInputs) {
			t.Fatalf("review-and-fix inputs = %#v, want exactly %#v", def.Inputs, wantInputs)
		}
		for _, input := range wantInputs {
			if _, ok := def.Inputs[input]; !ok {
				t.Fatalf("review-and-fix input %q missing", input)
			}
		}
		if !def.Inputs["task_name"].Required {
			t.Fatal("task_name required = false, want true")
		}
		if got, want := def.Inputs["auto_commit"].Default, false; got != want {
			t.Fatalf("auto_commit default = %#v, want %#v", got, want)
		}
		wantNodes := []dsl.NodeID{
			"review", "has_issues", "write_artifacts", "fix_batches", "fix_batch", "collect_fixes", "finalize_round",
		}
		if len(def.Graph.Nodes) != len(wantNodes) {
			t.Fatalf("review-and-fix nodes = %#v, want exactly %#v", def.Graph.Nodes, wantNodes)
		}
		for index, wantNode := range wantNodes {
			if got := def.Graph.Nodes[index].ID; got != wantNode {
				t.Fatalf("review-and-fix node[%d] = %q, want %q", index, got, wantNode)
			}
		}
		writeArtifacts := requireDevCycleNode(t, def, "write_artifacts")
		wantWriteArtifactsKind := toolspkg.ToolID("ext__dev_cycle__write_review_artifacts").String()
		if got, want := writeArtifacts.Kind, wantWriteArtifactsKind; got != want {
			t.Fatalf("write_artifacts kind = %q, want %q", got, want)
		}
		if got, want := writeArtifacts.Params["issues"], "{{ .nodes.review.output.issues }}"; got != want {
			t.Fatalf("write_artifacts issues = %#v, want %q", got, want)
		}
		fixBatches := requireDevCycleNode(t, def, "fix_batches")
		if got, want := fixBatches.Collection, "{{ .nodes.write_artifacts.output.batches }}"; got != want {
			t.Fatalf("fix_batches collection = %q, want %q", got, want)
		}
		fixBatch := requireDevCycleNode(t, def, "fix_batch")
		prompt := requireStringParam(t, fixBatch, "prompt")
		for _, required := range []string{
			"systematic-debugging",
			"no-workarounds",
			"cy-fix-reviews",
			"cy-final-verify",
			"Remediate this complete batch of review artifact files",
			"begin immediately without asking for confirmation",
			"Read every listed issue file completely before editing code",
			"Never create, rename, timestamp",
			"{{ .item.file }}",
			"{{ range .item.issue_files -}}",
			"all-or-nothing",
			"{{ if .inputs.auto_commit -}}",
			"Create exactly one local commit for this batch",
			"Leave the verified changes uncommitted",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("fix_batch prompt missing %q", required)
			}
		}
		commitPrompt := renderReviewAndFixPromptForTest(t, prompt, true)
		for _, required := range []string{
			"Create exactly one local commit for this batch after verification. Do not push.",
		} {
			if !strings.Contains(commitPrompt, required) {
				t.Fatalf("auto_commit rendered prompt missing %q", required)
			}
		}
		if strings.Contains(commitPrompt, "Leave the verified changes uncommitted") {
			t.Fatal("auto_commit prompt incorrectly leaves changes uncommitted")
		}
		manualPrompt := renderReviewAndFixPromptForTest(t, prompt, false)
		if !strings.Contains(manualPrompt, "Leave the verified changes uncommitted for manual review") {
			t.Fatal("manual prompt missing leave-uncommitted instruction")
		}
		if strings.Contains(manualPrompt, "Create exactly one local commit") {
			t.Fatal("manual prompt incorrectly creates a commit")
		}
		resultsSchema := requireSchemaObject(t, requireSchemaObject(t, fixBatch.Params, "output_schema"), "properties")
		results := requireSchemaObject(t, resultsSchema, "results")
		item := requireSchemaObject(t, results, "items")
		required, ok := item["required"].([]any)
		if !ok {
			t.Fatalf("fix_batch result required schema = %#v, want []any", item["required"])
		}
		if !schemaStringListContains(required, "summary") {
			t.Fatalf("fix_batch result required = %#v, want summary", required)
		}
		if !schemaStringListContains(required, "resolution") {
			t.Fatalf("fix_batch result required = %#v, want resolution", required)
		}
		for _, field := range []string{"path", "triage"} {
			if !schemaStringListContains(required, field) {
				t.Fatalf("fix_batch result required = %#v, want %s", required, field)
			}
		}
		finalize := requireDevCycleNode(t, def, "finalize_round")
		if got, want := finalize.Kind, toolspkg.ToolID("ext__dev_cycle__finalize_review_round").String(); got != want {
			t.Fatalf("finalize_round kind = %q, want %q", got, want)
		}
		if _, ok := finalize.Produces["pr"]; ok {
			t.Fatalf("finalize_round produces deleted pr field: %#v", finalize.Produces)
		}
	})

	t.Run("Should keep software-delivery verification opt-in", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/software-delivery/loop.yaml")
		verifyCommand, ok := def.Inputs["verify_command"]
		if !ok {
			t.Fatal("software-delivery input verify_command missing")
		}
		if got, want := verifyCommand.Default, ""; got != want {
			t.Fatalf("verify_command default = %#v, want %q", got, want)
		}
		verify := requireDevCycleNode(t, def, "verify")
		if got, want := len(verify.Criteria), 1; got != want {
			t.Fatalf("verify criteria = %d, want %d", got, want)
		}
		if got, want := verify.Criteria[0].Type, dsl.CriterionCommand; got != want {
			t.Fatalf("verify criterion type = %q, want %q", got, want)
		}
		if got, want := verify.Criteria[0].Check, "{{ .inputs.verify_command }}"; got != want {
			t.Fatalf("verify criterion check = %q, want %q", got, want)
		}
		execute := requireDevCycleNode(t, def, "execute_task")
		prompt := requireStringParam(t, execute, "prompt")
		for _, required := range []string{
			".compozy/tasks/{{ .inputs.slug }}/memory",
			".compozy/tasks/{{ .inputs.slug }}/memory/MEMORY.md",
			".compozy/tasks/{{ .inputs.slug }}/memory/{{ .item.id }}.md",
			"cy-workflow-memory",
			"cy-execute-task",
			"cy-final-verify",
			"do NOT ask for confirmation",
			"_techspec.md",
			"record meaningful follow-up work",
			"promote only durable cross-task context",
			"Execute every explicit Validation, Test Plan, or Testing item",
			"Do not push",
			"Closing directive:",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("execute_task prompt missing %q", required)
			}
		}
	})

	t.Run("Should load software-delivery tasks through import_tasks action", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/software-delivery/loop.yaml")
		loadTasks := requireDevCycleNode(t, def, "load_tasks")
		if got, want := loadTasks.Class, dsl.NodeClassAction; got != want {
			t.Fatalf("load_tasks class = %q, want %q", got, want)
		}
		if got, want := loadTasks.Kind, devCycleToolID(t, toolImportTasks); got != want {
			t.Fatalf("load_tasks kind = %q, want %q", got, want)
		}
		if loadTasks.Pattern != "" || loadTasks.Parse != "" {
			t.Fatalf(
				"load_tasks legacy file-import fields = pattern %q parse %q, want empty",
				loadTasks.Pattern,
				loadTasks.Parse,
			)
		}
		pattern := requireStringParam(t, loadTasks, "pattern")
		if got, want := pattern, ".compozy/tasks/{{ .inputs.slug }}/task_*.md"; got != want {
			t.Fatalf("load_tasks params.pattern = %q, want %q", got, want)
		}
		if got, want := loadTasks.Produces["tasks"], "array"; got != want {
			t.Fatalf("load_tasks produces.tasks = %#v, want %q", got, want)
		}
		implement := requireDevCycleNode(t, def, "implement")
		if got, want := implement.Collection, "{{ .nodes.load_tasks.output.tasks }}"; got != want {
			t.Fatalf("implement collection = %q, want %q", got, want)
		}
		resolved, err := loop.NewCompiler(
			loop.WithCompilerToolSchemaSource(devCycleToolSchemaSource(t)),
		).Compile(def)
		if err != nil {
			t.Fatalf("Compile(software-delivery) error = %v", err)
		}
		if resolved.Templates["nodes.implement.collection"] == nil {
			t.Fatal("Compile(software-delivery) missing implement collection template")
		}
	})

	t.Run("Should render software-delivery implementer prompt for both commit modes", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/software-delivery/loop.yaml")
		execute := requireDevCycleNode(t, def, "execute_task")
		prompt := requireStringParam(t, execute, "prompt")

		autoCommit := renderSoftwareDeliveryImplementerPromptForTest(t, prompt, true)
		for _, required := range []string{
			"Begin work on Ship loops immediately",
			"Depends on: task_01, task_02",
			"Create exactly one commit for this task after clean verification",
		} {
			if !strings.Contains(autoCommit, required) {
				t.Fatalf("auto_commit rendered prompt missing %q", required)
			}
		}
		if strings.Contains(autoCommit, "Leave changes uncommitted") {
			t.Fatalf("auto_commit rendered prompt incorrectly leaves changes uncommitted")
		}
		manual := renderSoftwareDeliveryImplementerPromptForTest(t, prompt, false)
		if !strings.Contains(manual, "Leave changes uncommitted for manual review. Do not push.") {
			t.Fatalf("manual rendered prompt missing leave-uncommitted instruction")
		}
		if strings.Contains(manual, "Create exactly one commit") {
			t.Fatalf("manual rendered prompt incorrectly instructs an automatic commit")
		}
	})
}

func renderSoftwareDeliveryImplementerPromptForTest(
	t *testing.T,
	prompt string,
	autoCommit bool,
) string {
	t.Helper()

	rendered, err := refs.RenderTemplateString("software-delivery.execute_task.prompt", prompt, map[string]any{
		"inputs": map[string]any{
			"slug":        "loops",
			"auto_commit": autoCommit,
		},
		"item": map[string]any{
			"id":                  "task_03",
			reviewIssueFieldTitle: "Ship loops",
			"path":                ".compozy/tasks/loops/task_03.md",
			reviewIssueFieldBody:  "Implement the loop runtime.",
			"blocks":              []any{"task_01", "task_02"},
		},
	})
	if err != nil {
		t.Fatalf("RenderTemplateString() error = %v", err)
	}
	return rendered
}

func renderReviewAndFixPromptForTest(
	t *testing.T,
	prompt string,
	autoCommit bool,
) string {
	t.Helper()

	rendered, err := refs.RenderTemplateString("review-and-fix.fix_batch.prompt", prompt, map[string]any{
		"inputs": map[string]any{
			"auto_commit": autoCommit,
		},
		"item": map[string]any{
			"file":        "internal/daemon/loop.go",
			"issue_files": []string{".compozy/tasks/delivery/reviews-001/issue_001.md"},
		},
	})
	if err != nil {
		t.Fatalf("RenderTemplateString() error = %v", err)
	}
	return rendered
}

func TestEmbeddedAgentsShouldKeepPromptContracts(t *testing.T) {
	t.Run("Should require review fixer summary output", func(t *testing.T) {
		t.Parallel()

		data, err := fs.ReadFile(FS(), "agents/review_fixer/AGENT.md")
		if err != nil {
			t.Fatalf("ReadFile(review_fixer) error = %v", err)
		}
		prompt := string(data)
		for _, required := range []string{
			"systematic-debugging",
			"no-workarounds",
			"cy-fix-reviews",
			"Read every supplied issue file completely before editing code",
			"fails as a whole",
			"real verification commands",
			"Never create, rename, timestamp, or set an issue file to `resolved`",
			"`resolution` is accepted only as `fixed` or `documented`",
			"`path`, `triage`, `resolution`, and `summary`",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("review_fixer prompt missing %q", required)
			}
		}
		if strings.Contains(prompt, "notes") {
			t.Fatalf("review_fixer prompt contains retired notes field")
		}
	})

	t.Run("Should keep implementer role prompt discipline", func(t *testing.T) {
		t.Parallel()

		data, err := fs.ReadFile(FS(), "agents/code_implementer/AGENT.md")
		if err != nil {
			t.Fatalf("ReadFile(code_implementer) error = %v", err)
		}
		prompt := string(data)
		for _, required := range []string{
			"do not ask for confirmation",
			"cy-workflow-memory",
			"cy-execute-task",
			"cy-final-verify",
			"_techspec.md",
			"meaningful follow-up work",
			"durable cross-task context",
			"Never push",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("code_implementer prompt missing %q", required)
			}
		}
	})

	t.Run("Should keep reviewer source discipline", func(t *testing.T) {
		t.Parallel()

		data, err := fs.ReadFile(FS(), "agents/reviewer/AGENT.md")
		if err != nil {
			t.Fatalf("ReadFile(reviewer) error = %v", err)
		}
		prompt := string(data)
		for _, required := range []string{
			"files, tests, and command output",
			"source-agnostic `ReviewIssue[]`",
			"`title`, `body`, and `severity`",
			"most severe first",
			"empty issue array only when the round is clean",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("reviewer prompt missing %q", required)
			}
		}
	})

	t.Run("Should give implementer and reviewer graceful degradation clauses", func(t *testing.T) {
		t.Parallel()

		for _, path := range []string{
			"agents/code_implementer/AGENT.md",
			"agents/reviewer/AGENT.md",
		} {
			data, err := fs.ReadFile(FS(), path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}
			if !strings.Contains(string(data), "degrad") {
				t.Fatalf("%s missing graceful degradation guidance", path)
			}
		}
	})
}

func TestEmbeddedAgentsShouldParseWithRuntimeSchema(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	root, err := materializeEmbeddedExtension(homePaths)
	if err != nil {
		t.Fatalf("materializeEmbeddedExtension() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Fatalf("RemoveAll(%q) error = %v", root, err)
		}
	})
	agentFiles := embeddedAgentFiles(t, root)
	if len(agentFiles) == 0 {
		t.Fatal("embedded agent files = 0, want at least one dev-cycle agent")
	}
	for _, path := range agentFiles {
		t.Run("Should parse "+filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			t.Parallel()

			agent, err := aghconfig.LoadAgentDefFile(path)
			if err != nil {
				t.Fatalf("LoadAgentDefFile(%q) error = %v", path, err)
			}
			if strings.TrimSpace(agent.Name) == "" {
				t.Fatalf("LoadAgentDefFile(%q).Name is empty", path)
			}
		})
	}
}

type devCycleToolSchemas map[string]loop.ToolSchemaSnapshot

func (s devCycleToolSchemas) Snapshot(toolID string) (loop.ToolSchemaSnapshot, bool) {
	snapshot, ok := s[toolID]
	return snapshot, ok
}

func embeddedLoopFiles(t *testing.T) []string {
	t.Helper()
	files := []string{}
	if err := fs.WalkDir(FS(), "loops", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Base(path) != "loop.yaml" {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		t.Fatalf("WalkDir(loops) error = %v", err)
	}
	return files
}

func embeddedAgentFiles(t *testing.T, root string) []string {
	t.Helper()
	files := []string{}
	agentsRoot := filepath.Join(root, "agents")
	if err := filepath.WalkDir(agentsRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != aghconfig.AgentDefinitionFileName {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		t.Fatalf("WalkDir(%q) error = %v", agentsRoot, err)
	}
	return files
}

func parseEmbeddedLoopForTest(t *testing.T, path string) dsl.Definition {
	t.Helper()
	data, err := fs.ReadFile(FS(), path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	source := devCycleToolSchemaSource(t)
	linter := loop.NewLinter(loop.WithToolSchemaSource(source))
	_, def, err := loop.ParseResource(data, loop.ResourceParseOptions{
		Source:   loop.SourceMarketplace,
		Dir:      filepath.ToSlash(filepath.Dir(path)),
		FilePath: filepath.ToSlash(path),
		Linter:   linter,
	})
	if err != nil {
		t.Fatalf("ParseResource(%q) error = %v", path, err)
	}
	return def
}

func hasStartKind(def dsl.Definition, kind dsl.StartKind) bool {
	for _, binding := range def.Start {
		if binding.Kind == kind {
			return true
		}
	}
	return false
}

func requireDevCycleNode(t *testing.T, def dsl.Definition, id dsl.NodeID) dsl.Node {
	t.Helper()
	for _, node := range def.Graph.Nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("node %q missing", id)
	return dsl.Node{}
}

func requireSchemaObject(t *testing.T, schema map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := schema[key]
	if !ok {
		t.Fatalf("schema key %q missing from %#v", key, schema)
	}
	object, ok := raw.(map[string]any)
	if ok {
		return object
	}
	typed, ok := raw.(dsl.Schema)
	if ok {
		return map[string]any(typed)
	}
	params, ok := raw.(dsl.NodeParams)
	if ok {
		return map[string]any(params)
	}
	t.Fatalf("schema key %q type = %T, want map[string]any", key, raw)
	return nil
}

func requireStringParam(t *testing.T, node dsl.Node, key string) string {
	t.Helper()
	raw, ok := node.Params[key]
	if !ok {
		t.Fatalf("node %q param %q missing", node.ID, key)
	}
	value, ok := raw.(string)
	if !ok {
		t.Fatalf("node %q param %q type = %T, want string", node.ID, key, raw)
	}
	return value
}

func schemaStringListContains(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func devCycleToolSchemaSource(t *testing.T) devCycleToolSchemas {
	t.Helper()

	return devCycleToolSchemas{
		devCycleToolID(t, toolImportTasks): toolSnapshot(
			t,
			toolImportTasks,
			importTasksInputSchema,
			importTasksOutputSchema,
		),
		devCycleToolID(t, toolWriteArtifacts): toolSnapshot(
			t,
			toolWriteArtifacts,
			writeArtifactsInputSchema,
			writeArtifactsOutputSchema,
		),
		devCycleToolID(t, toolFinalizeRound): toolSnapshot(
			t,
			toolFinalizeRound,
			finalizeRoundInputSchema,
			finalizeRoundOutputSchema,
		),
	}
}

func devCycleToolID(t *testing.T, handler string) string {
	t.Helper()
	id, err := runtimeToolID(handler)
	if err != nil {
		t.Fatalf("runtimeToolID(%q) error = %v", handler, err)
	}
	return id.String()
}

func toolSnapshot(
	t *testing.T,
	handler string,
	inputSchema json.RawMessage,
	outputSchema json.RawMessage,
) loop.ToolSchemaSnapshot {
	t.Helper()
	id := devCycleToolID(t, handler)
	inputDigest := schemaDigest(t, handler+" input", inputSchema)
	outputDigest := schemaDigest(t, handler+" output", outputSchema)
	return loop.ToolSchemaSnapshot{
		ToolID:             id,
		InputSchema:        cloneRawMessage(inputSchema),
		OutputSchema:       cloneRawMessage(outputSchema),
		InputSchemaDigest:  inputDigest,
		OutputSchemaDigest: outputDigest,
	}
}

func schemaDigest(t *testing.T, name string, schema json.RawMessage) string {
	t.Helper()
	digest, err := toolspkg.SchemaDigest(schema)
	if err != nil {
		t.Fatalf("%s schema digest error = %v", name, err)
	}
	return digest
}
