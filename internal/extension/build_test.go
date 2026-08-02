package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/testutil"
)

func TestBuildBundle(t *testing.T) {
	t.Parallel()

	t.Run("Should Detect Package Go And Override Toolchains", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name         string
			prepare      func(*testing.T, string)
			request      func(string) BuildRequest
			wantBuild    []string
			wantDescribe []string
		}{
			{
				name:         "Should Select Bun Package Scripts",
				prepare:      writePackageBuildFixture,
				request:      func(dir string) BuildRequest { return BuildRequest{SourceDir: dir} },
				wantBuild:    []string{"bun", "run", "build"},
				wantDescribe: []string{"bun", "run", "start", "--", "__describe"},
			},
			{
				name:         "Should Select Go Build",
				prepare:      writeGoBuildFixture,
				request:      func(dir string) BuildRequest { return BuildRequest{SourceDir: dir} },
				wantBuild:    []string{"go", "build", "-o", "dist/bin", "."},
				wantDescribe: []string{"./dist/bin", "__describe"},
			},
			{
				name:    "Should Prefer Build Command Override",
				prepare: writeGoBuildFixture,
				request: func(dir string) BuildRequest {
					return BuildRequest{SourceDir: dir, BuildCmd: []string{"make", "extension"}}
				},
				wantBuild:    []string{"make", "extension"},
				wantDescribe: []string{"./dist/bin", "__describe"},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				dir := t.TempDir()
				testCase.prepare(t, dir)
				runner := newBuildTestRunner(validDescribePayload())
				if _, err := buildBundle(testutil.Context(t), testCase.request(dir), runner); err != nil {
					t.Fatalf("buildBundle() error = %v", err)
				}
				calls := runner.Commands()
				if len(calls) != 2 {
					t.Fatalf("len(commands) = %d, want 2: %#v", len(calls), calls)
				}
				if !reflect.DeepEqual(calls[0], testCase.wantBuild) {
					t.Fatalf("build command = %#v, want %#v", calls[0], testCase.wantBuild)
				}
				if !reflect.DeepEqual(calls[1], testCase.wantDescribe) {
					t.Fatalf("describe command = %#v, want %#v", calls[1], testCase.wantDescribe)
				}
			})
		}
	})

	t.Run("Should Produce Byte Identical Immutable Generations", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeGoBuildFixture(t, dir)
		runner := newBuildTestRunner(validDescribePayload())
		first, err := buildBundle(testutil.Context(t), BuildRequest{SourceDir: dir}, runner)
		if err != nil {
			t.Fatalf("buildBundle(first) error = %v", err)
		}
		firstBytes, err := os.ReadFile(first.ManifestPath)
		if err != nil {
			t.Fatalf("os.ReadFile(first manifest) error = %v", err)
		}
		second, err := buildBundle(testutil.Context(t), BuildRequest{SourceDir: dir}, runner)
		if err != nil {
			t.Fatalf("buildBundle(second) error = %v", err)
		}
		secondBytes, err := os.ReadFile(second.ManifestPath)
		if err != nil {
			t.Fatalf("os.ReadFile(second manifest) error = %v", err)
		}
		if first.GenerationHash != second.GenerationHash || first.GenerationDir != second.GenerationDir {
			t.Fatalf("generation mismatch: first=%#v second=%#v", first, second)
		}
		if !slices.Equal(firstBytes, secondBytes) {
			t.Fatal("generated extension.toml bytes differ across identical builds")
		}
	})

	t.Run("Should copy declared resources and emit them deterministically", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeGoBuildFixture(t, dir)
		skillDir := filepath.Join(dir, "skills", "writer")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(skillDir) error = %v", err)
		}
		writeFile(t, filepath.Join(skillDir, "SKILL.md"), "# Writer\n")
		writeStaticAgentFixture(t, dir, "writer", true)
		writeAutomationResource(t, dir, "automation/main.toml", `
[[jobs]]
name = "daily"
agent = "writer"
prompt = "Write a digest."
[jobs.schedule]
mode = "cron"
expr = "0 * * * *"
`)
		if err := os.MkdirAll(filepath.Join(dir, "layouts"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(layouts) error = %v", err)
		}
		writeLayoutJSONFile(t, filepath.Join(dir, "layouts", "two-up.json"), testLayoutResource("two-up"))
		payload := validDescribePayload()
		payload.Resources.Agents = []string{"agents"}
		payload.Resources.Skills = []string{"skills"}
		payload.Resources.Automation = []string{"automation"}
		payload.Resources.Layouts = []string{"layouts/two-up.json"}

		first, err := buildBundle(testutil.Context(t), BuildRequest{SourceDir: dir}, newBuildTestRunner(payload))
		if err != nil {
			t.Fatalf("buildBundle(first) error = %v", err)
		}
		copied, err := os.ReadFile(filepath.Join(first.GenerationDir, "skills", "writer", "SKILL.md"))
		if err != nil {
			t.Fatalf("os.ReadFile(copied skill) error = %v", err)
		}
		if string(copied) != "# Writer\n" || !reflect.DeepEqual(first.Manifest.Resources.Skills, []string{"skills"}) {
			t.Fatalf("copied = %q resources = %#v", copied, first.Manifest.Resources.Skills)
		}
		for _, relativePath := range []string{
			"agents/writer/AGENT.md",
			"agents/writer/SOUL.md",
			"agents/writer/HEARTBEAT.md",
			"automation/main.toml",
			"layouts/two-up.json",
		} {
			if _, err := os.Stat(filepath.Join(first.GenerationDir, relativePath)); err != nil {
				t.Fatalf("os.Stat(copied %q) error = %v", relativePath, err)
			}
		}
		firstManifest, err := os.ReadFile(first.ManifestPath)
		if err != nil {
			t.Fatalf("os.ReadFile(first manifest) error = %v", err)
		}
		for _, declaration := range []string{
			"agents = [\"agents\"]",
			"skills = [\"skills\"]",
			"automation = [\"automation\"]",
			"layouts = [\"layouts/two-up.json\"]",
		} {
			if !strings.Contains(string(firstManifest), declaration) {
				t.Fatalf("manifest = %s, want declaration %q", firstManifest, declaration)
			}
		}
		second, err := buildBundle(testutil.Context(t), BuildRequest{SourceDir: dir}, newBuildTestRunner(payload))
		if err != nil {
			t.Fatalf("buildBundle(second) error = %v", err)
		}
		if first.GenerationHash != second.GenerationHash {
			t.Fatalf("generation hash = %q then %q, want stable", first.GenerationHash, second.GenerationHash)
		}
	})

	t.Run("Should reject invalid declared automation before publishing a generation", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeGoBuildFixture(t, dir)
		if err := os.MkdirAll(filepath.Join(dir, "automation"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(automation) error = %v", err)
		}
		writeFile(t, filepath.Join(dir, "automation", "jobs.toml"), `
[[jobs]]
name = "daily"
agent = "writer"
prompt = "Write a digest."
[jobs.schedule]
mode = "cron"
expr = "not-a-cron"
`)
		payload := validDescribePayload()
		payload.Resources.Automation = []string{"automation"}
		_, err := buildBundle(testutil.Context(t), BuildRequest{SourceDir: dir}, newBuildTestRunner(payload))
		if err == nil || !strings.Contains(err.Error(), "automation/jobs.toml") ||
			!strings.Contains(err.Error(), "expr is invalid") {
			t.Fatalf("buildBundle() error = %v, want positioned automation validation", err)
		}
		generations, globErr := filepath.Glob(filepath.Join(dir, "dist", "gen-*"))
		if globErr != nil {
			t.Fatalf("filepath.Glob(generations) error = %v", globErr)
		}
		if len(generations) != 0 {
			t.Fatalf("published generations = %#v, want none", generations)
		}
	})

	t.Run("Should reject declared resource overlaps and symlinks", func(t *testing.T) {
		t.Parallel()

		t.Run("Should reject overlapping declarations", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeGoBuildFixture(t, dir)
			if err := os.MkdirAll(filepath.Join(dir, "resources", "nested"), 0o755); err != nil {
				t.Fatalf("os.MkdirAll(resources) error = %v", err)
			}
			payload := validDescribePayload()
			payload.Resources.Skills = []string{"resources"}
			payload.Resources.Loops = []string{"resources/nested"}
			_, err := buildBundle(testutil.Context(t), BuildRequest{SourceDir: dir}, newBuildTestRunner(payload))
			if !errors.Is(err, ErrManifestInvalid) || !strings.Contains(err.Error(), "overlap") {
				t.Fatalf("buildBundle() error = %v, want overlap rejection", err)
			}
		})

		t.Run("Should reject symlinks inside declared resources", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeGoBuildFixture(t, dir)
			resourceDir := filepath.Join(dir, "skills")
			if err := os.MkdirAll(resourceDir, 0o755); err != nil {
				t.Fatalf("os.MkdirAll(skills) error = %v", err)
			}
			outside := filepath.Join(t.TempDir(), "outside.md")
			writeFile(t, outside, "outside")
			if err := os.Symlink(outside, filepath.Join(resourceDir, "escape.md")); err != nil {
				t.Fatalf("os.Symlink() error = %v", err)
			}
			payload := validDescribePayload()
			payload.Resources.Skills = []string{"skills"}
			_, err := buildBundle(testutil.Context(t), BuildRequest{SourceDir: dir}, newBuildTestRunner(payload))
			if !errors.Is(err, ErrManifestInvalid) || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("buildBundle() error = %v, want symlink rejection", err)
			}
		})
	})

	t.Run("Should Time Out Describe Without Partial Manifest", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeGoBuildFixture(t, dir)
		runner := newBuildTestRunner(validDescribePayload())
		runner.blockDescribe = true
		_, err := buildBundle(testutil.Context(t), BuildRequest{
			SourceDir: dir,
			Timeout:   10 * time.Millisecond,
		}, runner)
		if err == nil || !strings.Contains(err.Error(), "describe timed out") {
			t.Fatalf("buildBundle() error = %v, want describe timeout", err)
		}
		matches, globErr := filepath.Glob(filepath.Join(dir, "dist", "**", "extension.toml"))
		if globErr != nil {
			t.Fatalf("filepath.Glob() error = %v", globErr)
		}
		if len(matches) != 0 {
			t.Fatalf("partial manifests = %#v, want none", matches)
		}
	})

	t.Run("Should Reject Unknown Provide Before Manifest Write", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeGoBuildFixture(t, dir)
		payload := validDescribePayload()
		payload.Provides = []string{"prompt.provider"}
		_, err := buildBundle(
			testutil.Context(t),
			BuildRequest{SourceDir: dir},
			newBuildTestRunner(payload),
		)
		if err == nil || !strings.Contains(err.Error(), "prompt.provider") {
			t.Fatalf("buildBundle() error = %v, want unknown provide", err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, "dist", "extension.toml")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("root manifest stat error = %v, want not exist", statErr)
		}
	})

	t.Run("Should Stamp SDK Compatibility And Ignore Authored Manifest", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeGoBuildFixture(t, dir)
		writeFile(t, filepath.Join(dir, "extension.toml"), `[extension]
name = "stale"
version = "9.9.9"
min_compozy_version = "9.9.9"
`)
		payload := validDescribePayload()
		payload.SDK.MinCompozyVersion = "0.3.0-beta.1"
		result, err := buildBundle(
			testutil.Context(t),
			BuildRequest{SourceDir: dir},
			newBuildTestRunner(payload),
		)
		if err != nil {
			t.Fatalf("buildBundle() error = %v", err)
		}
		if result.Manifest.MinCompozyVersion != payload.SDK.MinCompozyVersion {
			t.Fatalf(
				"MinCompozyVersion = %q, want %q",
				result.Manifest.MinCompozyVersion,
				payload.SDK.MinCompozyVersion,
			)
		}
	})

	t.Run("Should Reject Output Containing Source Without Removing Files", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		sourceDir := filepath.Join(root, "source")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(source) error = %v", err)
		}
		writeGoBuildFixture(t, sourceDir)
		sentinel := filepath.Join(root, "keep.txt")
		writeFile(t, sentinel, "keep")

		_, err := buildBundle(testutil.Context(t), BuildRequest{
			SourceDir: sourceDir,
			OutputDir: root,
		}, newBuildTestRunner(validDescribePayload()))
		if err == nil || !strings.Contains(err.Error(), "must not contain the source") {
			t.Fatalf("buildBundle() error = %v, want overlapping output rejection", err)
		}
		data, readErr := os.ReadFile(sentinel)
		if readErr != nil {
			t.Fatalf("os.ReadFile(sentinel) error = %v", readErr)
		}
		if string(data) != "keep" {
			t.Fatalf("sentinel content = %q, want keep", data)
		}
	})
}

func TestManifestFromDescribeResources(t *testing.T) {
	t.Parallel()

	t.Run("Should map every declared resource kind in sorted order", func(t *testing.T) {
		t.Parallel()

		payload := validDescribePayload()
		payload.Resources = extensioncontract.DescribeResources{
			Skills:     []string{"skills/z", "skills/a"},
			Loops:      []string{"loops"},
			Agents:     []string{"agents"},
			Automation: []string{"automation/z.toml", "automation/a.toml"},
			Layouts:    []string{"layouts"},
		}
		manifest, err := manifestFromDescribe(payload)
		if err != nil {
			t.Fatalf("manifestFromDescribe() error = %v", err)
		}
		want := ResourcesConfig{
			Skills:     []string{"skills/a", "skills/z"},
			Loops:      []string{"loops"},
			Agents:     []string{"agents"},
			Automation: []string{"automation/a.toml", "automation/z.toml"},
			Layouts:    []string{"layouts"},
		}
		if !reflect.DeepEqual(manifest.Resources, want) {
			t.Fatalf("manifest.Resources = %#v, want %#v", manifest.Resources, want)
		}
	})
}

func TestResourceOnlyManifestHardCut(t *testing.T) {
	t.Parallel()

	t.Run("Should validate automation and layout declarations", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "automation"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(automation) error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "layouts"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(layouts) error = %v", err)
		}
		writeFile(t, filepath.Join(dir, manifestTOMLFileName), `
[extension]
name = "resource-only"
version = "0.1.0"
min_compozy_version = "0.3.0-beta.1"

[resources]
automation = ["automation"]
layouts = ["layouts"]

[subprocess]
command = "./bin"
`)
		manifest, issues, err := ValidateBundle(dir)
		if err != nil {
			t.Fatalf("ValidateBundle() error = %v", err)
		}
		if manifest == nil || len(issues) != 0 {
			t.Fatalf("ValidateBundle() manifest = %#v issues = %#v, want valid", manifest, issues)
		}
	})

	t.Run("Should reject the removed bundles resource field", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, manifestTOMLFileName), `
[extension]
name = "legacy-bundle"
version = "0.1.0"
min_compozy_version = "0.3.0-beta.1"

[resources]
bundles = ["bundles"]

[subprocess]
command = "./bin"
`)
		manifest, issues, err := ValidateBundle(dir)
		if err != nil {
			t.Fatalf("ValidateBundle() error = %v", err)
		}
		if manifest != nil || len(issues) != 1 || !strings.Contains(issues[0].Message, "bundles") {
			t.Fatalf("ValidateBundle() manifest = %#v issues = %#v, want bundles unknown-field issue", manifest, issues)
		}
	})
}

func TestValidateBundle(t *testing.T) {
	t.Parallel()

	t.Run("Should Return Positioned TOML Syntax Issue", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, manifestTOMLFileName), strings.Join([]string{
			"[extension]",
			`name = "broken"`,
			`version = "0.1.0"`,
			`min_compozy_version = "0.3.0-beta.1"`,
			"[permissions]",
			"requires = []",
			"  @",
		}, "\n"))
		manifest, issues, err := ValidateBundle(dir)
		if err != nil {
			t.Fatalf("ValidateBundle() error = %v", err)
		}
		if manifest != nil {
			t.Fatalf("manifest = %#v, want nil", manifest)
		}
		if len(issues) != 1 {
			t.Fatalf("issues = %#v, want one", issues)
		}
		if issues[0].Line != 7 || issues[0].Column != 3 {
			t.Fatalf("issue position = %d:%d, want 7:3", issues[0].Line, issues[0].Column)
		}
	})

	t.Run("Should Return Derived Consent Areas", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, manifestJSONFileName), `{
  "extension": {
    "name": "consent-fixture",
    "version": "0.1.0",
    "min_compozy_version": "0.3.0-beta.1"
  },
  "permissions": {"requires": ["sessions/list", "memory/store"]},
  "capabilities": {"provides": []},
  "subprocess": {"command": "./bin"}
}`)
		report, err := ValidateBundleReport(dir)
		if err != nil {
			t.Fatalf("ValidateBundleReport() error = %v", err)
		}
		want := []ConsentArea{{Area: "memory", Access: "write"}, {Area: "sessions", Access: "read"}}
		if !reflect.DeepEqual(report.ConsentAreas, want) {
			t.Fatalf("ConsentAreas = %#v, want %#v", report.ConsentAreas, want)
		}
		if len(report.Issues) != 0 || report.Manifest == nil {
			t.Fatalf("report = %#v, want valid manifest without issues", report)
		}
	})
}

func TestScaffoldExtension(t *testing.T) {
	t.Parallel()

	expectedTemplates := []ScaffoldTemplate{
		ScaffoldTemplateHookTS,
		ScaffoldTemplateLoopWatchSourceGo,
		ScaffoldTemplateMemoryBackendTS,
		ScaffoldTemplateToolProviderGo,
		ScaffoldTemplateToolProviderTS,
	}
	if templates := ScaffoldTemplates(); !slices.Equal(templates, expectedTemplates) {
		t.Fatalf("ScaffoldTemplates() = %v, want %v", templates, expectedTemplates)
	}

	for _, template := range expectedTemplates {
		t.Run("Should Render "+string(template)+" Without Dual Declaration", func(t *testing.T) {
			t.Parallel()

			target := filepath.Join(t.TempDir(), "hello-extension")
			result, err := ScaffoldExtension(ScaffoldRequest{
				Name:      "hello-extension",
				Template:  template,
				Directory: target,
			})
			if err != nil {
				t.Fatalf("ScaffoldExtension() error = %v", err)
			}
			if len(result.Files) == 0 || !slices.IsSorted(result.Files) {
				t.Fatalf("Files = %v, want non-empty sorted paths", result.Files)
			}
			for _, relative := range result.Files {
				if relative == manifestTOMLFileName || relative == manifestJSONFileName {
					t.Fatalf("template %q contains hand-authored manifest %q", template, relative)
				}
				data, err := os.ReadFile(filepath.Join(result.Directory, relative))
				if err != nil {
					t.Fatalf("read scaffold file %q: %v", relative, err)
				}
				content := string(data)
				for _, forbidden := range []string{
					"__EXTENSION_NAME__",
					"__COMPOZY_GO_SDK_VERSION__",
					"min_compozy_version",
					`"private": true`,
				} {
					if strings.Contains(content, forbidden) {
						t.Fatalf("template %q file %q contains forbidden %q", template, relative, forbidden)
					}
				}
				if relative == "go.mod" {
					wantRequirement := "github.com/compozy/compozy/sdk/go " + scaffoldGoSDKVersion
					if !strings.Contains(content, wantRequirement) {
						t.Fatalf("template %q go.mod = %q, want requirement %q", template, content, wantRequirement)
					}
				}
			}
		})
	}

	t.Run("Should Preserve A Non Empty Target", func(t *testing.T) {
		t.Parallel()

		target := filepath.Join(t.TempDir(), "occupied")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("create occupied target: %v", err)
		}
		sentinel := filepath.Join(target, "keep.txt")
		writeFile(t, sentinel, "keep")
		_, err := ScaffoldExtension(ScaffoldRequest{
			Name:      "occupied",
			Template:  ScaffoldTemplateToolProviderGo,
			Directory: target,
		})
		if err == nil || !strings.Contains(err.Error(), "not empty") {
			t.Fatalf("ScaffoldExtension() error = %v, want non-empty target rejection", err)
		}
		data, err := os.ReadFile(sentinel)
		if err != nil {
			t.Fatalf("read preserved target: %v", err)
		}
		if string(data) != "keep" {
			t.Fatalf("preserved target content = %q, want keep", data)
		}
	})
}

type buildTestRunner struct {
	mu            sync.Mutex
	commands      [][]string
	payload       extensioncontract.DescribePayload
	blockDescribe bool
}

func newBuildTestRunner(payload extensioncontract.DescribePayload) *buildTestRunner {
	return &buildTestRunner{payload: payload}
}

func (r *buildTestRunner) LookPath(file string) (string, error) {
	if file == "bun" {
		return "/test/bin/bun", nil
	}
	return "", os.ErrNotExist
}

func (r *buildTestRunner) Run(
	ctx context.Context,
	_ string,
	argv []string,
) (buildCommandOutput, error) {
	r.mu.Lock()
	r.commands = append(r.commands, slices.Clone(argv))
	r.mu.Unlock()
	if !slices.Contains(argv, "__describe") {
		return buildCommandOutput{}, nil
	}
	if r.blockDescribe {
		<-ctx.Done()
		return buildCommandOutput{}, ctx.Err()
	}
	data, err := json.Marshal(r.payload)
	if err != nil {
		return buildCommandOutput{}, err
	}
	return buildCommandOutput{Stdout: data}, nil
}

func (r *buildTestRunner) Commands() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	commands := make([][]string, 0, len(r.commands))
	for _, command := range r.commands {
		commands = append(commands, slices.Clone(command))
	}
	return commands
}

func validDescribePayload() extensioncontract.DescribePayload {
	return extensioncontract.DescribePayload{
		Name:        "fixture",
		Version:     "0.1.0",
		Description: "Build fixture",
		Provides:    []string{},
		Permissions: []string{},
		Subprocess:  extensioncontract.DescribeSubprocess{Command: "./bin"},
		SDK: extensioncontract.DescribeSDKInfo{
			Name:              "test-sdk",
			Version:           "0.1.0",
			ProtocolVersion:   "1",
			MinCompozyVersion: "0.3.0-beta.1",
		},
	}
}

func writeGoBuildFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/build-fixture\n\ngo 1.26.4\n")
}

func writePackageBuildFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "scripts": {"build": "tsc", "start": "node dist/index.js"}
}`)
}
