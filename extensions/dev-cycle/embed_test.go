package devcycle

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
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

func TestDevCycleRuntimeToolDescriptorsShouldPinImportTasksSchemaDigests(t *testing.T) {
	t.Run("Should pin import tasks schema digests", func(t *testing.T) {
		t.Parallel()

		const (
			wantInputDigest  = "ff6206bbb7edbf85229a394c4752286046cdbc069b1a89b3067f139f1f68a832"
			wantOutputDigest = "8ebb2e834741f5f5e908adcf0c3b2e251eee9ee37dcf24ba88f461e7eb5ccbc3"
		)
		descriptors, err := runtimeToolDescriptors()
		if err != nil {
			t.Fatalf("runtimeToolDescriptors() error = %v", err)
		}
		var found bool
		for _, descriptor := range descriptors {
			if descriptor.Handler != toolImportTasks {
				continue
			}
			found = true
			if got, want := descriptor.ID.String(), devCycleToolID(t, toolImportTasks); got != want {
				t.Fatalf("import_tasks id = %q, want %q", got, want)
			}
			if !descriptor.ReadOnly {
				t.Fatalf("import_tasks read_only = false, want true")
			}
			if got, want := descriptor.Risk, toolspkg.RiskRead; got != want {
				t.Fatalf("import_tasks risk = %q, want %q", got, want)
			}
			if got := descriptor.InputSchemaDigest; got != wantInputDigest {
				t.Fatalf("import_tasks input digest = %q, want %q", got, wantInputDigest)
			}
			if got := descriptor.OutputSchemaDigest; got != wantOutputDigest {
				t.Fatalf("import_tasks output digest = %q, want %q", got, wantOutputDigest)
			}
		}
		if !found {
			t.Fatal("runtimeToolDescriptors() missing import_tasks descriptor")
		}
	})
}

func TestDevCycleManagedInstallShouldPublishImportTasksManifestTool(t *testing.T) {
	t.Run("Should publish import tasks through the bundled manifest install", func(t *testing.T) {
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
		tool, ok := manifest.Resources.Tools[toolImportTasks]
		if !ok {
			t.Fatalf("manifest tools = %#v, want %q", manifest.Resources.Tools, toolImportTasks)
		}
		if tool.Handler != toolImportTasks ||
			tool.Backend.Kind != "extension_host" ||
			tool.Backend.Handler != toolImportTasks {
			t.Fatalf(
				"import_tasks backend = %#v, handler %q; want extension_host import_tasks",
				tool.Backend,
				tool.Handler,
			)
		}
		if !tool.ReadOnly || !tool.ConcurrencySafe || tool.Risk != string(toolspkg.RiskRead) {
			t.Fatalf("import_tasks tool policy = %#v, want read-only concurrency-safe read risk", tool)
		}
		if len(tool.InputSchema) == 0 || len(tool.OutputSchema) == 0 {
			t.Fatalf(
				"import_tasks schemas are empty: input=%d output=%d",
				len(tool.InputSchema),
				len(tool.OutputSchema),
			)
		}
	})
}

func TestEmbeddedLoopsShouldKeepDevCycleRuntimeContracts(t *testing.T) {
	t.Run("Should keep reviews-watch trigger start and clean-tick stop_when", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/reviews-watch/loop.yaml")
		if got, want := def.Contract.StopWhen,
			"nodes.fetch_issues.status == 'succeeded' && size(nodes.fetch_issues.output.issues) == 0"; got != want {
			t.Fatalf("reviews-watch stop_when = %q, want %q", got, want)
		}
		if got, want := def.Contract.IterationCap, 0; got != want {
			t.Fatalf("reviews-watch iteration_cap = %d, want %d", got, want)
		}
		if !hasStartKind(def, dsl.StartTrigger) {
			t.Fatalf("reviews-watch start = %#v, want trigger start binding", def.Start)
		}
		newReview := requireDevCycleNode(t, def, "new_review")
		reviewSchema := requireSchemaObject(t, newReview.Produces, "review")
		properties := requireSchemaObject(t, reviewSchema, "properties")
		for _, field := range []string{"head_sha", "review_id", "submitted_at"} {
			if _, ok := properties[field]; !ok {
				t.Fatalf("new_review produces.review.properties missing %q: %#v", field, properties)
			}
		}
	})

	t.Run("Should keep reviews-watch fixer prompt and inputs wired", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/reviews-watch/loop.yaml")
		for _, input := range []string{"include_nitpicks", "auto_commit", "auto_push"} {
			if _, ok := def.Inputs[input]; !ok {
				t.Fatalf("reviews-watch input %q missing", input)
			}
		}
		if got, want := def.Inputs["include_nitpicks"].Default, false; got != want {
			t.Fatalf("include_nitpicks default = %#v, want %#v", got, want)
		}
		if got, want := def.Inputs["auto_commit"].Default, false; got != want {
			t.Fatalf("auto_commit default = %#v, want %#v", got, want)
		}
		fetchIssues := requireDevCycleNode(t, def, "fetch_issues")
		if got, want := fetchIssues.Params["include_nitpicks"], "{{ .inputs.include_nitpicks }}"; got != want {
			t.Fatalf("fetch_issues include_nitpicks param = %#v, want %q", got, want)
		}
		fixBatch := requireDevCycleNode(t, def, "fix_batch")
		prompt := requireStringParam(t, fixBatch, "prompt")
		for _, required := range []string{
			"systematic-debugging",
			"no-workarounds",
			"cy-fix-reviews",
			"cy-final-verify",
			"Remediate this batch of {{ len .item }}",
			"do NOT ask for confirmation",
			"Read every issue in this batch completely before editing code",
			"real verification",
			"fails as a whole",
			"auto_commit: {{ .inputs.auto_commit }}",
			"auto_push: {{ .inputs.auto_push }}",
			"Because auto_push is true",
			"Because auto_commit and auto_push are false",
			"Follow the execution mode above exactly",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("fix_batch prompt missing %q", required)
			}
		}
		pushPrompt := renderReviewsWatchFixerPromptForTest(t, prompt, false, true)
		for _, required := range []string{
			"auto_commit: false",
			"auto_push: true",
			"Because auto_push is true, create exactly one local fix commit after verification",
		} {
			if !strings.Contains(pushPrompt, required) {
				t.Fatalf("auto_push rendered prompt missing %q", required)
			}
		}
		if strings.Contains(pushPrompt, "leave verified changes uncommitted") {
			t.Fatalf("auto_push rendered prompt incorrectly leaves changes uncommitted")
		}
		manualPrompt := renderReviewsWatchFixerPromptForTest(t, prompt, false, false)
		for _, required := range []string{
			"auto_commit: false",
			"auto_push: false",
			"Because auto_commit and auto_push are false, leave verified changes uncommitted for",
		} {
			if !strings.Contains(manualPrompt, required) {
				t.Fatalf("manual rendered prompt missing %q", required)
			}
		}
		if strings.Contains(manualPrompt, "Because auto_push is true") {
			t.Fatalf("manual rendered prompt incorrectly instructs auto_push commit")
		}
		if strings.Contains(prompt, "notes") {
			t.Fatalf("fix_batch prompt contains retired notes contract")
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
			"id":     "task_03",
			"title":  "Ship loops",
			"path":   ".compozy/tasks/loops/task_03.md",
			"body":   "Implement the loop runtime.",
			"blocks": []any{"task_01", "task_02"},
		},
	})
	if err != nil {
		t.Fatalf("RenderTemplateString() error = %v", err)
	}
	return rendered
}

func renderReviewsWatchFixerPromptForTest(
	t *testing.T,
	prompt string,
	autoCommit bool,
	autoPush bool,
) string {
	t.Helper()

	rendered, err := refs.RenderTemplateString("reviews-watch.fix_batch.prompt", prompt, map[string]any{
		"inputs": map[string]any{
			"pr":          123,
			"auto_commit": autoCommit,
			"auto_push":   autoPush,
		},
		"item": []map[string]any{{
			"id":           "CR-1",
			"title":        "Fix loop event handling",
			"file":         "internal/daemon/loop.go",
			"line":         42,
			"severity":     "medium",
			"provider_ref": "THREAD-1",
			"body":         "Please fix this issue.",
		}},
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
			"Read every issue in the batch completely before editing code",
			"fails as a whole",
			"real verification commands",
			"`resolution` is accepted only as `fixed` or `documented`",
			"`id`, `triage`, `resolution`, and `summary`",
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

	t.Run("Should keep reviewer judge discipline", func(t *testing.T) {
		t.Parallel()

		data, err := fs.ReadFile(FS(), "agents/reviewer/AGENT.md")
		if err != nil {
			t.Fatalf("ReadFile(reviewer) error = %v", err)
		}
		prompt := string(data)
		for _, required := range []string{
			"files, tests, command output",
			"stable issue ids",
			"most severe first",
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
		devCycleToolID(t, toolFetchUnresolved): toolSnapshot(
			t,
			toolFetchUnresolved,
			fetchInputSchema,
			fetchOutputSchema,
		),
		devCycleToolID(t, toolResolveThreads): toolSnapshot(
			t,
			toolResolveThreads,
			resolveInputSchema,
			resolveOutputSchema,
		),
		devCycleToolID(t, toolGitPush): toolSnapshot(
			t,
			toolGitPush,
			pushInputSchema,
			pushOutputSchema,
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
