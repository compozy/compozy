package devcycle

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// Suite: embedded dev-cycle product contracts.
// Invariant: shipped loops, agents, tools, and skills remain loadable and mutually consistent.
// Boundary IN: embedded bytes, managed installation, and runtime parsers.
// Boundary OUT: daemon publication and workspace projection, owned by internal/daemon integration tests.

var bundledSkillNames = []string{
	"cy-create-prd",
	"cy-create-tasks",
	"cy-create-techspec",
	"cy-execute-task",
	"cy-final-verify",
	"cy-fix-reviews",
	"cy-review-round",
	"cy-workflow-memory",
	"git-rebase",
}

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

func TestDevCycleManagedInstallShouldPreserveManagedManifestTools(t *testing.T) {
	t.Run("Should install managed tool declarations in an enabled bundled extension", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
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
		if installed.Provenance.InstalledFrom != extensionpkg.ExtensionInstalledFromBundled ||
			installed.Provenance.RegistryTier != extensionpkg.ExtensionRegistryTierOfficial ||
			!installed.Provenance.ChecksumVerified ||
			installed.Provenance.AllowUnverified {
			t.Fatalf(
				"installed provenance = %#v, want bundled official verified provenance",
				installed.Provenance,
			)
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

func TestDevCycleManagedInstallShouldReconcileBundledProvenance(t *testing.T) {
	t.Run("Should repair bundled trust metadata without changing enabled state or install time", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		globalDB, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := globalDB.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(globalDB) error = %v", err)
			}
		})
		registry := extensionpkg.NewRegistry(globalDB.DB())

		if err := EnsureManagedInstall(homePaths, registry); err != nil {
			t.Fatalf("EnsureManagedInstall(first) error = %v", err)
		}
		before, err := registry.Get(Name)
		if err != nil {
			t.Fatalf("registry.Get(%q before) error = %v", Name, err)
		}
		if err := registry.Disable(Name); err != nil {
			t.Fatalf("registry.Disable(%q) error = %v", Name, err)
		}
		stale := before.Provenance
		stale.InstalledFrom = extensionpkg.ExtensionInstalledFromLocalPath
		stale.RegistryTier = extensionpkg.ExtensionRegistryTierUnverified
		stale.ChecksumVerified = false
		stale.AllowUnverified = true
		staleJSON, err := json.Marshal(stale)
		if err != nil {
			t.Fatalf("json.Marshal(stale provenance) error = %v", err)
		}
		if _, err := registry.DB().ExecContext(
			testutil.Context(t),
			`UPDATE extensions SET provenance_json = ? WHERE name = ?`,
			string(staleJSON),
			Name,
		); err != nil {
			t.Fatalf("persist stale provenance error = %v", err)
		}

		if err := EnsureManagedInstall(homePaths, registry); err != nil {
			t.Fatalf("EnsureManagedInstall(reconcile) error = %v", err)
		}
		after, err := registry.Get(Name)
		if err != nil {
			t.Fatalf("registry.Get(%q after) error = %v", Name, err)
		}
		if after.Enabled {
			t.Fatal("reconciled extension enabled = true, want preserved disabled state")
		}
		if !after.InstalledAt.Equal(before.InstalledAt) {
			t.Fatalf("reconciled installed_at = %s, want %s", after.InstalledAt, before.InstalledAt)
		}
		if after.Provenance.InstalledFrom != extensionpkg.ExtensionInstalledFromBundled ||
			after.Provenance.RegistryTier != extensionpkg.ExtensionRegistryTierOfficial ||
			!after.Provenance.ChecksumVerified ||
			after.Provenance.AllowUnverified {
			t.Fatalf("reconciled provenance = %#v, want bundled official verified provenance", after.Provenance)
		}
		described := extensionpkg.DescribeExtension(
			&extensionpkg.Extension{Info: *after},
			false,
			time.Now(),
		)
		if described.Trust == nil || described.Trust.Decision != extensionpkg.ExtensionTrustDecisionVerified {
			t.Fatalf("described trust = %#v, want verified", described.Trust)
		}

		if err := EnsureManagedInstall(homePaths, registry); err != nil {
			t.Fatalf("EnsureManagedInstall(idempotent) error = %v", err)
		}
		repeated, err := registry.Get(Name)
		if err != nil {
			t.Fatalf("registry.Get(%q repeated) error = %v", Name, err)
		}
		if repeated.Enabled || !repeated.InstalledAt.Equal(after.InstalledAt) ||
			!repeated.Provenance.InstalledAt.Equal(after.Provenance.InstalledAt) {
			t.Fatalf("repeated reconciliation changed stable metadata: before=%#v after=%#v", after, repeated)
		}
	})
}

func TestEmbeddedSkillsShouldKeepBundleContract(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	root, err := extensionpkg.MaterializeBundledExtension(homePaths, Name, FS())
	if err != nil {
		t.Fatalf("MaterializeBundledExtension() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("RemoveAll(%q) error = %v", root, err)
		}
	})
	manifest, err := extensionpkg.LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", root, err)
	}

	t.Run("Should embed exactly the nine manifest-declared skills", func(t *testing.T) {
		t.Parallel()

		paths, err := fs.Glob(FS(), "skills/*/SKILL.md")
		if err != nil {
			t.Fatalf("Glob(skills/*/SKILL.md) error = %v", err)
		}
		if got, want := len(paths), len(bundledSkillNames); got != want {
			t.Fatalf("embedded skill count = %d, want %d; paths=%#v", got, want, paths)
		}
		for _, name := range bundledSkillNames {
			path := "skills/" + name + "/SKILL.md"
			if !slices.Contains(paths, path) {
				t.Fatalf("embedded skills = %#v, want %q", paths, path)
			}
		}
		if !slices.Equal(manifest.Resources.Skills, []string{"skills"}) {
			t.Fatalf("manifest.Resources.Skills = %#v, want [skills]", manifest.Resources.Skills)
		}
	})

	t.Run("Should parse every skill with bundled extension classification", func(t *testing.T) {
		t.Parallel()

		for _, skillName := range bundledSkillNames {
			t.Run("Should parse "+skillName, func(t *testing.T) {
				t.Parallel()

				path := filepath.Join(root, "skills", skillName, "SKILL.md")
				skill, err := skillspkg.ParseSkillFileWithSource(path, skillspkg.SourceBundled)
				if err != nil {
					t.Fatalf("ParseSkillFileWithSource(%q) error = %v", path, err)
				}
				if skill.Meta.Name != skillName || skill.Source != skillspkg.SourceBundled {
					t.Fatalf("parsed skill = %#v, want name %q and bundled source", skill, skillName)
				}
			})
		}
	})

	t.Run("Should exclude retired commands and skills", func(t *testing.T) {
		t.Parallel()

		for _, name := range bundledSkillNames {
			path := "skills/" + name + "/SKILL.md"
			data, err := fs.ReadFile(FS(), path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}
			body := strings.ToLower(string(data))
			for _, retired := range []string{
				"compozy tasks run",
				"compozy tasks validate",
				"compozy reviews fix",
				"compozy setup",
				"cy-capture-decisions",
			} {
				if strings.Contains(body, retired) {
					t.Fatalf("%s contains retired surface %q", path, retired)
				}
			}
		}
		if slices.Contains(bundledSkillNames, "compozy") {
			t.Fatal("bundled skill names contain retired duplicate compozy skill")
		}
	})
}

func TestDevCycleManagedInstallShouldReenrollChangedBundledSkills(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	globalDB, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close(globalDB) error = %v", err)
		}
	})
	registry := extensionpkg.NewRegistry(globalDB.DB())
	if err := EnsureManagedInstall(homePaths, registry); err != nil {
		t.Fatalf("EnsureManagedInstall(first) error = %v", err)
	}
	installed, err := registry.Get(Name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", Name, err)
	}
	root := filepath.Dir(installed.ManifestPath)
	oldSkillPath := filepath.Join(root, "skills", "cy-execute-task", "SKILL.md")
	if err := os.WriteFile(oldSkillPath, []byte("old bundled skill bytes\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", oldSkillPath, err)
	}
	oldManifestPath := filepath.Join(root, "extension.json")
	manifestBytes, err := os.ReadFile(oldManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", oldManifestPath, err)
	}
	manifestBytes = []byte(strings.Replace(string(manifestBytes), `"version": "0.3.1"`, `"version": "0.3.0"`, 1))
	if err := os.WriteFile(oldManifestPath, manifestBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", oldManifestPath, err)
	}
	oldManifest, err := extensionpkg.LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest(old) error = %v", err)
	}
	oldChecksum, err := extensionpkg.ComputeDirectoryChecksum(root)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(old) error = %v", err)
	}
	if err := registry.Install(
		oldManifest,
		root,
		oldChecksum,
		extensionpkg.WithInstallSource(extensionpkg.SourceBundled),
		extensionpkg.WithInstallReplaceExisting(),
	); err != nil {
		t.Fatalf("registry.Install(old bundle) error = %v", err)
	}

	if err := EnsureManagedInstall(homePaths, registry); err != nil {
		t.Fatalf("EnsureManagedInstall(updated) error = %v", err)
	}
	updated, err := registry.Get(Name)
	if err != nil {
		t.Fatalf("registry.Get(%q updated) error = %v", Name, err)
	}
	if updated.Version != "0.3.1" || strings.EqualFold(updated.Checksum, oldChecksum) {
		t.Fatalf(
			"updated identity = version %q checksum %q, want 0.3.1 and checksum != %q",
			updated.Version,
			updated.Checksum,
			oldChecksum,
		)
	}
	wantBody, err := fs.ReadFile(FS(), "skills/cy-execute-task/SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(embedded cy-execute-task) error = %v", err)
	}
	gotBody, err := os.ReadFile(
		filepath.Join(filepath.Dir(updated.ManifestPath), "skills", "cy-execute-task", "SKILL.md"),
	)
	if err != nil {
		t.Fatalf("ReadFile(updated cy-execute-task) error = %v", err)
	}
	if !slices.Equal(gotBody, wantBody) {
		t.Fatal("updated cy-execute-task bytes do not match embedded bundle")
	}
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

	t.Run("Should keep implement-tasks focused on task implementation", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/implement-tasks/loop.yaml")
		wantInputs := []string{"slug", "implementer", "auto_commit"}
		if len(def.Inputs) != len(wantInputs) {
			t.Fatalf("implement-tasks inputs = %#v, want exactly %#v", def.Inputs, wantInputs)
		}
		for _, input := range wantInputs {
			if _, ok := def.Inputs[input]; !ok {
				t.Fatalf("implement-tasks input %q missing", input)
			}
		}

		wantNodes := []dsl.NodeID{"slug_input", "load_tasks", "implement", "execute_task", "collect"}
		if len(def.Graph.Nodes) != len(wantNodes) {
			t.Fatalf("implement-tasks nodes = %#v, want exactly %#v", def.Graph.Nodes, wantNodes)
		}
		for index, wantNode := range wantNodes {
			if got := def.Graph.Nodes[index].ID; got != wantNode {
				t.Fatalf("implement-tasks node[%d] = %q, want %q", index, got, wantNode)
			}
		}
		wantEdges := [][2]dsl.NodeID{
			{"slug_input", "load_tasks"},
			{"load_tasks", "implement"},
			{"implement", "execute_task"},
			{"execute_task", "collect"},
		}
		if len(def.Graph.Edges) != len(wantEdges) {
			t.Fatalf("implement-tasks edges = %#v, want exactly %#v", def.Graph.Edges, wantEdges)
		}
		for index, wantEdge := range wantEdges {
			got := def.Graph.Edges[index]
			if got.From != wantEdge[0] || got.To != wantEdge[1] {
				t.Fatalf(
					"implement-tasks edge[%d] = %q -> %q, want %q -> %q",
					index,
					got.From,
					got.To,
					wantEdge[0],
					wantEdge[1],
				)
			}
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
		if strings.Contains(prompt, "verify_command") || strings.Contains(prompt, "`blocked`") {
			t.Fatalf("execute_task prompt retains removed final-gate language: %q", prompt)
		}
		outputSchema := requireSchemaObject(t, execute.Params, "output_schema")
		properties := requireSchemaObject(t, outputSchema, "properties")
		status := requireSchemaObject(t, properties, "status")
		statuses, ok := status["enum"].([]any)
		if !ok || len(statuses) != 1 || statuses[0] != "completed" {
			t.Fatalf("execute_task status enum = %#v, want [completed]", status["enum"])
		}
	})

	t.Run("Should load implement-tasks tasks through import_tasks action", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/implement-tasks/loop.yaml")
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
			t.Fatalf("Compile(implement-tasks) error = %v", err)
		}
		if resolved.Templates["nodes.implement.collection"] == nil {
			t.Fatal("Compile(implement-tasks) missing implement collection template")
		}
	})

	t.Run("Should render implement-tasks implementer prompt for both commit modes", func(t *testing.T) {
		t.Parallel()

		def := parseEmbeddedLoopForTest(t, "loops/implement-tasks/loop.yaml")
		execute := requireDevCycleNode(t, def, "execute_task")
		prompt := requireStringParam(t, execute, "prompt")

		autoCommit := renderImplementTasksPromptForTest(t, prompt, true)
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
		manual := renderImplementTasksPromptForTest(t, prompt, false)
		if !strings.Contains(manual, "Leave changes uncommitted for manual review. Do not push.") {
			t.Fatalf("manual rendered prompt missing leave-uncommitted instruction")
		}
		if strings.Contains(manual, "Create exactly one commit") {
			t.Fatalf("manual rendered prompt incorrectly instructs an automatic commit")
		}
	})
}

func renderImplementTasksPromptForTest(
	t *testing.T,
	prompt string,
	autoCommit bool,
) string {
	t.Helper()

	rendered, err := refs.RenderTemplateString("implement-tasks.execute_task.prompt", prompt, map[string]any{
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
			"cy-review-round",
			"cy-final-verify",
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

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	root, err := extensionpkg.MaterializeBundledExtension(homePaths, Name, FS())
	if err != nil {
		t.Fatalf("MaterializeBundledExtension() error = %v", err)
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

			agent, err := compozyconfig.LoadAgentDefFile(path)
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
		if entry.IsDir() || entry.Name() != compozyconfig.AgentDefinitionFileName {
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
