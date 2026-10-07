//go:build mage

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGolangCILintCacheDir(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve explicit isolation", func(t *testing.T) {
		t.Parallel()

		want := filepath.Join(t.TempDir(), "isolated-lint-cache")
		got, err := golangCILintCacheDir("  "+want+"  ", "ignored-project-root")
		if err != nil {
			t.Fatalf("golangCILintCacheDir() error = %v", err)
		}
		if got != want {
			t.Fatalf("golangCILintCacheDir() = %q, want %q", got, want)
		}
	})

	t.Run("Should isolate the implicit cache by project root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		first, err := golangCILintCacheDir("", root)
		if err != nil {
			t.Fatalf("golangCILintCacheDir() first error = %v", err)
		}
		repeated, err := golangCILintCacheDir("", root)
		if err != nil {
			t.Fatalf("golangCILintCacheDir() repeated error = %v", err)
		}
		other, err := golangCILintCacheDir("", t.TempDir())
		if err != nil {
			t.Fatalf("golangCILintCacheDir() other error = %v", err)
		}
		if first != repeated {
			t.Fatalf("golangCILintCacheDir() = %q, want stable %q", repeated, first)
		}
		if first == other {
			t.Fatalf("golangCILintCacheDir() reused %q across project roots", first)
		}
	})
}

func TestGolangciLintScopes(t *testing.T) {
	t.Parallel()

	t.Run("Should lint the whole module when no scopes are set", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			raw  string
		}{
			{name: "Should accept an empty value", raw: ""},
			{name: "Should accept spaces", raw: "   "},
			{name: "Should accept mixed whitespace", raw: "\t\n"},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				if got := golangciLintScopes(testCase.raw); !reflect.DeepEqual(got, []string{"./..."}) {
					t.Fatalf("golangciLintScopes(%q) = %v, want [./...]", testCase.raw, got)
				}
			})
		}
	})

	t.Run("Should narrow the lint run to the explicit scopes", func(t *testing.T) {
		t.Parallel()

		got := golangciLintScopes("  ./internal/api/...   ./cmd/... ")
		want := []string{"./internal/api/...", "./cmd/..."}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("golangciLintScopes() = %v, want %v", got, want)
		}
	})
}

func TestGolangciEnabledLinters(t *testing.T) {
	t.Parallel()

	writeConfig := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "golangci.yml")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture config: %v", err)
		}
		return path
	}

	t.Run("Should mirror the enable list with entries trimmed", func(t *testing.T) {
		t.Parallel()

		path := writeConfig(
			t,
			"linters:\n  default: none\n  enable:\n    - errcheck\n    - '  staticcheck '\n    - unused\n",
		)
		got, err := golangciEnabledLinters(path)
		if err != nil {
			t.Fatalf("golangciEnabledLinters() error = %v", err)
		}
		want := []string{"errcheck", "staticcheck", "unused"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("golangciEnabledLinters() = %v, want %v", got, want)
		}
	})

	t.Run("Should fail loud when the enable list is empty", func(t *testing.T) {
		t.Parallel()

		path := writeConfig(t, "linters:\n  default: none\n")
		if _, err := golangciEnabledLinters(path); err == nil {
			t.Fatal("golangciEnabledLinters() expected an error for an empty enable list")
		}
	})

	t.Run("Should fail loud on unparseable config", func(t *testing.T) {
		t.Parallel()

		path := writeConfig(t, "linters: [broken")
		if _, err := golangciEnabledLinters(path); err == nil {
			t.Fatal("golangciEnabledLinters() expected an error for invalid yaml")
		}
	})

	t.Run("Should fail loud when the config file is missing", func(t *testing.T) {
		t.Parallel()

		if _, err := golangciEnabledLinters(filepath.Join(t.TempDir(), "absent.yml")); err == nil {
			t.Fatal("golangciEnabledLinters() expected an error for a missing file")
		}
	})
}

func TestFormattersMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		explicit string
		ci       string
		want     bool
	}{
		{name: "Should keep formatters in run when explicitly requested", explicit: "run", ci: "", want: true},
		{name: "Should split even in CI when explicitly requested", explicit: "split", ci: "true", want: false},
		{name: "Should default to run inside CI", explicit: "", ci: "true", want: true},
		{name: "Should default to split locally", explicit: "", ci: "", want: false},
		{name: "Should fall back to the CI default on invalid values", explicit: "bogus", ci: "true", want: true},
		{name: "Should fall back to the local default on invalid values", explicit: "bogus", ci: "", want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := formattersMode(testCase.explicit, testCase.ci); got != testCase.want {
				t.Fatalf("formattersMode(%q, %q) = %v, want %v", testCase.explicit, testCase.ci, got, testCase.want)
			}
		})
	}
}

func TestFilterFmtTargets(t *testing.T) {
	t.Parallel()

	t.Run("Should keep existing Go files across modules, sorted and deduped", func(t *testing.T) {
		t.Parallel()

		existing := map[string]bool{
			"internal/loop/action.go": true,
			"cmd/compozy/main.go":     true,
			"magefiles/deps_lint.go":  true,
			"sdk/go/client.go":        true,
		}
		paths := []string{
			"internal/loop/action.go",
			"  cmd/compozy/main.go  ",
			"internal/loop/action.go",
			"internal/loop/deleted.go",
			"sdk/go/client.go",
			"web/src/app.tsx",
			"README.md",
			"",
			"magefiles/deps_lint.go",
		}
		got := filterFmtTargets(paths, func(path string) bool { return existing[path] })
		want := []string{"cmd/compozy/main.go", "internal/loop/action.go", "magefiles/deps_lint.go", "sdk/go/client.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("filterFmtTargets() = %v, want %v", got, want)
		}
	})

	t.Run("Should return an empty set when nothing qualifies", func(t *testing.T) {
		t.Parallel()

		got := filterFmtTargets([]string{"docs/readme.md"}, func(string) bool { return true })
		if len(got) != 0 {
			t.Fatalf("filterFmtTargets() = %v, want empty", got)
		}
	})
}

func TestGolangciLintConcurrencyFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		effectiveCPU int
		want         int
	}{
		{name: "Should floor at one worker", effectiveCPU: 0, want: 1},
		{name: "Should keep single-core machines at full width", effectiveCPU: 1, want: 1},
		{name: "Should keep small machines at full width", effectiveCPU: 4, want: 4},
		{name: "Should keep eight-core machines at full width", effectiveCPU: 8, want: 8},
		{name: "Should hold the cap just above the threshold", effectiveCPU: 10, want: 8},
		{name: "Should halve a sixteen-core machine", effectiveCPU: 16, want: 8},
		{name: "Should cap a thirty-two-core machine", effectiveCPU: 32, want: 8},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := golangciLintConcurrencyFor(testCase.effectiveCPU); got != testCase.want {
				t.Fatalf(
					"golangciLintConcurrencyFor(%d) = %d, want %d",
					testCase.effectiveCPU,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestGolangciLintTargets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, raw string
		want      []goLintTarget
	}{
		{name: "Should cover all supported modules by default", want: []goLintTarget{
			{dir: ".", scopes: []string{"./..."}},
			{dir: "sdk/go", scopes: []string{"./..."}},
			{dir: "sdk/examples/notes-commands", scopes: []string{"./..."}},
			{dir: "sdk/examples/clarify-tool", scopes: []string{"./..."}},
			{dir: "internal/extension/testdata/command-fixture-go", scopes: []string{"./..."}},
			{dir: "internal/extension/testdata/palette-fixture-go", scopes: []string{"./..."}},
			{dir: "magefiles", scopes: []string{"./..."}, tags: "mage"},
		}},
		{name: "Should enter example modules without linting their empty parent", raw: "./sdk/examples/...", want: []goLintTarget{
			{dir: "sdk/examples/notes-commands", scopes: []string{"./..."}},
			{dir: "sdk/examples/clarify-tool", scopes: []string{"./..."}},
		}},
		{name: "Should keep a scoped root check narrow", raw: "./internal/config/...", want: []goLintTarget{{dir: ".", scopes: []string{"./internal/config/..."}}}},
		{name: "Should resolve nested scopes relative to their module", raw: "./sdk/go/... ./magefiles/...", want: []goLintTarget{
			{dir: "sdk/go", scopes: []string{"./..."}}, {dir: "magefiles", scopes: []string{"./..."}, tags: "mage"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := golangciLintTargets(tc.raw); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("targets = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestGoLintFixtureWorkspace(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve the local SDK without mutating fixture modules", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		module := "internal/extension/testdata/command-fixture-go"
		original := "module fixture\n\ngo 1.26.4\n\nrequire github.com/compozy/compozy/sdk/go v99.0.0+incompatible\n"
		writeTestFile(t, root, module+"/go.mod", original)
		writeTestFile(t, root, "sdk/go/go.mod", "module github.com/compozy/compozy/sdk/go\n\ngo 1.27.1\n")
		env, cleanup, err := goLintModuleEnv(root, module, map[string]string{"GOLANGCI_LINT_CACHE": "preserved"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		command := exec.CommandContext(
			t.Context(),
			"go",
			"list",
			"-m",
			"-f",
			"{{.Dir}}",
			"github.com/compozy/compozy/sdk/go",
		)
		command.Dir = filepath.Join(root, module)
		command.Env = mergeEnvOverrides(os.Environ(), env)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("resolve fixture SDK: %v\n%s", err, output)
		}
		if got, want := strings.TrimSpace(string(output)), filepath.Join(root, "sdk", "go"); got != want {
			t.Fatalf("SDK = %q, want %q", got, want)
		}
		if got := readTestFile(t, root, module+"/go.mod"); got != original {
			t.Fatalf("fixture module changed: %s", got)
		}
		if env["GOLANGCI_LINT_CACHE"] != "preserved" {
			t.Fatal("lint cache override was lost")
		}
		cleanup()
		if _, err := os.Stat(env["GOWORK"]); !os.IsNotExist(err) {
			t.Fatalf("workspace retained after cleanup: %v", err)
		}
	})
}

func TestGoWorkspaceVersion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, first, second, want string }{
		{"Should select the newer SDK requirement", "1.26.4", "1.27.1", "1.27.1"},
		{"Should select the newer fixture requirement", "1.27.1", "1.26.4", "1.27.1"},
		{"Should compare patch versions numerically", "1.26.9", "1.26.10", "1.26.10"},
		{"Should compare minor versions numerically", "1.9", "1.26.4", "1.26.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeTestFile(t, root, "fixture/go.mod", "module fixture\n\ngo "+tc.first+"\n")
			writeTestFile(t, root, "sdk/go.mod", "module sdk\n\ngo "+tc.second+"\n")
			got, err := goWorkspaceVersion([]string{filepath.Join(root, "fixture"), filepath.Join(root, "sdk")})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("workspace Go version = %q, want %q", got, tc.want)
			}
		})
	}
}
