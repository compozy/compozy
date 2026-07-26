//go:build mage

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/e2elane"
)

func TestShouldEnsureWebBundle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		plan e2elane.Plan
		want bool
	}{
		{
			name: "Should require the bundle for runtime Go suites",
			plan: e2elane.Plan{
				GoSuites: []e2elane.GoSuite{{Packages: []string{"./internal/daemon"}}},
			},
			want: true,
		},
		{
			name: "Should require the bundle for daemon-served browser suites",
			plan: e2elane.Plan{
				ScriptSuites:                []e2elane.ScriptSuite{{Dir: "web", Script: "test:e2e:daemon-served"}},
				RequiresDaemonServedBrowser: true,
			},
			want: true,
		},
		{
			name: "Should skip the bundle for non-browser script suites alone",
			plan: e2elane.Plan{
				ScriptSuites: []e2elane.ScriptSuite{{Dir: "scripts", Script: "echo"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldEnsureWebBundle(tt.plan); got != tt.want {
				t.Fatalf("shouldEnsureWebBundle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureWebAssetsAlwaysBuildsCurrentSources(t *testing.T) {
	t.Parallel()

	t.Run("Should check codegen and rebuild even when a prior bundle exists", func(t *testing.T) {
		t.Parallel()

		calls := make([]string, 0, 2)
		err := ensureWebAssetsWith(
			func() error {
				calls = append(calls, "codegen-check")
				return nil
			},
			func() error {
				calls = append(calls, "web-build")
				return nil
			},
		)
		if err != nil {
			t.Fatalf("ensureWebAssetsWith() error = %v", err)
		}
		if got, want := strings.Join(calls, ","), "codegen-check,web-build"; got != want {
			t.Fatalf("ensureWebAssetsWith() calls = %q, want %q", got, want)
		}
	})
}

func TestFilesImporting(t *testing.T) {
	t.Parallel()

	t.Run("Should match only the exact decoded Go import path", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(t, root, "exact.go", "package fixture\nimport _ \"github.com/compozy/agh/internal/session\"\n")
		writeTestFile(
			t,
			root,
			"prefix.go",
			"package fixture\nimport _ \"github.com/compozy/agh/internal/sessions/ledger\"\n",
		)
		files, err := filesImporting(root, "github.com/compozy/agh/internal/session")
		if err != nil {
			t.Fatalf("filesImporting() error = %v", err)
		}
		want := []string{filepath.Join(root, "exact.go")}
		if len(files) != len(want) || files[0] != want[0] {
			t.Fatalf("filesImporting() = %#v, want %#v", files, want)
		}
	})

	t.Run("Should fail closed when the import tree cannot be scanned", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing")
		if _, err := filesImporting(missing, "github.com/compozy/agh/internal/session"); err == nil {
			t.Fatal("filesImporting(missing) error = nil, want scan failure")
		}
	})

	t.Run("Should fail closed when a Go import declaration is malformed", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(t, root, "broken.go", "package fixture\nimport (\n")
		if _, err := filesImporting(root, "github.com/compozy/agh/internal/session"); err == nil {
			t.Fatal("filesImporting(malformed) error = nil, want parse failure")
		}
	})

	t.Run("Should match an imported package and its subpackages by prefix", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(
			t,
			root,
			"root.go",
			"package fixture\nimport _ \"github.com/compozy/agh/internal/extension\"\n",
		)
		writeTestFile(
			t,
			root,
			"child.go",
			"package fixture\nimport _ \"github.com/compozy/agh/internal/extension/contract\"\n",
		)
		writeTestFile(
			t,
			root,
			"neighbor.go",
			"package fixture\nimport _ \"github.com/compozy/agh/internal/extensionprotocol\"\n",
		)

		files, err := filesImportingPrefix(root, "github.com/compozy/agh/internal/extension")
		if err != nil {
			t.Fatalf("filesImportingPrefix() error = %v", err)
		}
		want := []string{filepath.Join(root, "child.go"), filepath.Join(root, "root.go")}
		if !slices.Equal(files, want) {
			t.Fatalf("filesImportingPrefix() = %#v, want %#v", files, want)
		}
	})

	t.Run("Should reject exact and provider-sibling platform implementation imports", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(
			t,
			root,
			"root.go",
			"package fixture\nimport _ \"github.com/compozy/agh/extensions/bridges\"\n",
		)
		writeTestFile(
			t,
			root,
			"provider.go",
			"package fixture\nimport _ \"github.com/compozy/agh/extensions/bridges/slack\"\n",
		)
		writeTestFile(
			t,
			root,
			"neighbor.go",
			"package fixture\nimport _ \"github.com/compozy/agh/extensions/bridgekit\"\n",
		)

		files, err := filesImportingPrefix(root, "github.com/compozy/agh/extensions/bridges")
		if err != nil {
			t.Fatalf("filesImportingPrefix() error = %v", err)
		}
		want := []string{filepath.Join(root, "provider.go"), filepath.Join(root, "root.go")}
		if !slices.Equal(files, want) {
			t.Fatalf("filesImportingPrefix() = %#v, want exact and provider-sibling imports %#v", files, want)
		}
	})
}

func TestProductionSourceLineLimit(t *testing.T) {
	t.Parallel()

	t.Run("Should classify handwritten product sources and explicit exclusions", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			path string
			want bool
		}{
			{name: "Should include Go runtime source", path: "internal/store/sqlite.go", want: true},
			{name: "Should include Web source", path: "web/src/systems/session/view.tsx", want: true},
			{name: "Should include Storybook source", path: "web/src/system.stories.tsx", want: true},
			{name: "Should exclude Go tests", path: "internal/store/sqlite_test.go", want: false},
			{name: "Should exclude Web tests", path: "web/src/__tests__/view.test.tsx", want: false},
			{
				name: "Should exclude generated contracts",
				path: "sdk/typescript/src/generated/contracts.ts",
				want: false,
			},
			{name: "Should exclude sqlc output", path: "internal/store/sqlcgen/query.sql.go", want: false},
			{name: "Should exclude fixtures", path: "web/src/system/fixtures/data.ts", want: false},
			{name: "Should exclude reference slides", path: "packages/slides/slides/demo/index.tsx", want: false},
			{name: "Should exclude declarative tokens", path: "packages/ui/src/tokens.css", want: false},
			{name: "Should exclude workflow governance", path: ".agents/skills/tool/scripts/check.py", want: false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if got := isProductionSourcePath(tt.path); got != tt.want {
					t.Fatalf("isProductionSourcePath(%q) = %v, want %v", tt.path, got, tt.want)
				}
			})
		}
	})

	t.Run("Should report only handwritten production files above the hard cap", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		oversized := strings.Repeat("package fixture\n", maxProductionSourceLines+1)
		writeTestFile(t, root, "internal/runtime/oversized.go", oversized)
		writeTestFile(
			t,
			root,
			"internal/runtime/within_limit.go",
			strings.Repeat("package fixture\n", maxProductionSourceLines),
		)
		writeTestFile(
			t,
			root,
			"internal/runtime/generated.go",
			"// Code generated by fixture. DO NOT EDIT.\n"+oversized,
		)
		writeTestFile(t, root, "internal/runtime/oversized_test.go", oversized)

		violations, err := inspectTrackedProductionSources(root, []string{
			"internal/runtime/oversized.go",
			"internal/runtime/within_limit.go",
			"internal/runtime/generated.go",
			"internal/runtime/oversized_test.go",
		})
		if err != nil {
			t.Fatalf("inspectTrackedProductionSources() error = %v", err)
		}
		want := []productionSourceSizeViolation{{
			path:  "internal/runtime/oversized.go",
			lines: maxProductionSourceLines + 1,
		}}
		if !slices.Equal(violations, want) {
			t.Fatalf("inspectTrackedProductionSources() = %#v, want %#v", violations, want)
		}
	})
}

func TestDependencyClosureBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should reject forbidden exact and transitive package families", func(t *testing.T) {
		t.Parallel()

		rules := []dependencyClosureRule{{
			root:            "./fixture",
			allowedPrefixes: []string{aghModulePath + "internal/bridges/contract"},
			forbiddenPrefixes: []string{
				aghModulePath + "internal/bridges",
				aghModulePath + "internal/store",
				aghModulePath + "internal/extension",
			},
		}}
		violations, err := inspectDependencyClosures(rules, func(root string) ([]string, error) {
			if root != "./fixture" {
				t.Fatalf("dependency root = %q, want ./fixture", root)
			}
			return []string{
				"fmt",
				aghModulePath + "internal/bridges/contract",
				aghModulePath + "internal/extensionprotocol",
				aghModulePath + "internal/store/globaldb",
				aghModulePath + "internal/extension/contract",
				aghModulePath + "internal/bridges",
				aghModulePath + "internal/bridges/experimental",
			}, nil
		})
		if err != nil {
			t.Fatalf("inspectDependencyClosures() error = %v", err)
		}
		want := []dependencyClosureViolation{
			{root: "./fixture", dependency: aghModulePath + "internal/bridges"},
			{root: "./fixture", dependency: aghModulePath + "internal/bridges/experimental"},
			{root: "./fixture", dependency: aghModulePath + "internal/extension/contract"},
			{root: "./fixture", dependency: aghModulePath + "internal/store/globaldb"},
		}
		if !slices.Equal(violations, want) {
			t.Fatalf("inspectDependencyClosures() = %#v, want %#v", violations, want)
		}
	})

	t.Run("Should fail closed when dependency listing fails", func(t *testing.T) {
		t.Parallel()

		_, err := inspectDependencyClosures(
			[]dependencyClosureRule{{root: "./fixture"}},
			func(string) ([]string, error) { return nil, errors.New("go list failed") },
		)
		if err == nil || !strings.Contains(err.Error(), "go list failed") {
			t.Fatalf("inspectDependencyClosures() error = %v, want go list failure", err)
		}
	})
}

func TestBuildGitMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should use source-build fallbacks when Git metadata is unavailable", func(t *testing.T) {
		t.Parallel()

		version, commit := buildGitMetadata(func(...string) (string, error) {
			return "", errors.New("git unavailable")
		})
		if version != "dev" || commit != "unknown" {
			t.Fatalf("buildGitMetadata() = (%q, %q), want (dev, unknown)", version, commit)
		}
	})

	t.Run("Should preserve resolved Git metadata", func(t *testing.T) {
		t.Parallel()

		responses := []string{"v1.2.3", "abc1234"}
		version, commit := buildGitMetadata(func(...string) (string, error) {
			value := responses[0]
			responses = responses[1:]
			return value, nil
		})
		if version != "v1.2.3" || commit != "abc1234" {
			t.Fatalf("buildGitMetadata() = (%q, %q), want (v1.2.3, abc1234)", version, commit)
		}
	})
}

func TestWorktreeScriptRejectsMissingFlagValues(t *testing.T) {
	t.Parallel()

	for _, flag := range []string{"--branch", "--base", "--dir"} {
		flag := flag
		t.Run("Should report a missing value for "+flag, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command("bash", "scripts/worktree.sh", "new", "test-worktree", flag)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("worktree.sh new %s error = nil, want usage failure", flag)
			}
			want := "worktree: " + flag + " requires a value"
			if !strings.Contains(string(output), want) {
				t.Fatalf("worktree.sh new %s output = %q, want %q", flag, output, want)
			}
			if strings.Contains(string(output), "unbound variable") {
				t.Fatalf("worktree.sh new %s leaked raw shell failure: %q", flag, output)
			}
		})
	}
}

func TestDirectoryDigest(t *testing.T) {
	t.Parallel()

	t.Run("Should be stable for the same relative paths and bytes", func(t *testing.T) {
		t.Parallel()

		first := t.TempDir()
		writeTestFile(t, first, "b.txt", "two")
		writeTestFile(t, first, "nested/a.txt", "one")
		firstDigest, err := directoryDigest(first)
		if err != nil {
			t.Fatalf("directoryDigest(first) error = %v", err)
		}

		second := t.TempDir()
		writeTestFile(t, second, "nested/a.txt", "one")
		writeTestFile(t, second, "b.txt", "two")
		secondDigest, err := directoryDigest(second)
		if err != nil {
			t.Fatalf("directoryDigest(second) error = %v", err)
		}

		if firstDigest != secondDigest {
			t.Fatalf("directoryDigest() = %s and %s, want stable digest", firstDigest, secondDigest)
		}
	})

	t.Run("Should change when file contents change", func(t *testing.T) {
		t.Parallel()

		first := t.TempDir()
		writeTestFile(t, first, "index.html", "one")
		firstDigest, err := directoryDigest(first)
		if err != nil {
			t.Fatalf("directoryDigest(first) error = %v", err)
		}

		second := t.TempDir()
		writeTestFile(t, second, "index.html", "two")
		secondDigest, err := directoryDigest(second)
		if err != nil {
			t.Fatalf("directoryDigest(second) error = %v", err)
		}
		if firstDigest == secondDigest {
			t.Fatalf("directoryDigest() = %s for different contents", firstDigest)
		}
	})
}

func TestWebAssetsMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should write deterministic metadata without timestamps", func(t *testing.T) {
		t.Parallel()

		repoDir := t.TempDir()
		metadata := webAssetsMetadata{
			BuildDigest:      "digest-123",
			SourceRepository: webAssetsSourceRepository,
			SourceCommit:     "abcdef123456",
		}
		if err := writeWebAssetsMetadata(repoDir, metadata); err != nil {
			t.Fatalf("writeWebAssetsMetadata(first) error = %v", err)
		}
		first := readTestFile(t, repoDir, webAssetsMetadataFile)
		if err := writeWebAssetsMetadata(repoDir, metadata); err != nil {
			t.Fatalf("writeWebAssetsMetadata(second) error = %v", err)
		}
		second := readTestFile(t, repoDir, webAssetsMetadataFile)
		if first != second {
			t.Fatalf("metadata changed between identical writes")
		}
		for _, forbidden := range []string{"GeneratedAt", "generated_at", "timestamp", "time.Now"} {
			if strings.Contains(first, forbidden) {
				t.Fatalf("metadata contains %q: %s", forbidden, first)
			}
		}

		parsed, err := readWebAssetsMetadata(repoDir)
		if err != nil {
			t.Fatalf("readWebAssetsMetadata() error = %v", err)
		}
		if parsed != metadata {
			t.Fatalf("readWebAssetsMetadata() = %#v, want %#v", parsed, metadata)
		}
	})
}

func TestWebAssetsNextTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "Should start at v0.0.1 when no semver tags exist",
			tags: []string{"latest", "assets"},
			want: "v0.0.1",
		},
		{
			name: "Should increment the highest semver patch",
			tags: []string{"v0.0.2", "v0.0.10", "v1.2.3", "not-a-tag"},
			want: "v1.2.4",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := nextWebAssetsTag(tt.tags)
			if err != nil {
				t.Fatalf("nextWebAssetsTag() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("nextWebAssetsTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWebAssetsReleaseSyncHelpers(t *testing.T) {
	t.Run("Should prefer the dedicated web assets token over the release token", func(t *testing.T) {
		t.Setenv(webAssetsTokenEnvVar, "assets-token")
		t.Setenv(releaseTokenEnvVar, "release-token")

		if got := webAssetsPublishToken(); got != "assets-token" {
			t.Fatalf("webAssetsPublishToken() = %q, want dedicated token", got)
		}
	})

	t.Run("Should fall back to the release token", func(t *testing.T) {
		t.Setenv(webAssetsTokenEnvVar, "")
		t.Setenv(releaseTokenEnvVar, "release-token")

		if got := webAssetsPublishToken(); got != "release-token" {
			t.Fatalf("webAssetsPublishToken() = %q, want release token", got)
		}
	})

	t.Run("Should keep Git credentials executable without writing the token to disk", func(t *testing.T) {
		t.Parallel()

		credentials, err := newWebAssetsGitCredentials("secret-token")
		if err != nil {
			t.Fatalf("newWebAssetsGitCredentials() error = %v", err)
		}
		t.Cleanup(credentials.cleanup)

		if got := credentials.env["AGH_WEB_ASSETS_GIT_TOKEN"]; got != "secret-token" {
			t.Fatalf("AGH_WEB_ASSETS_GIT_TOKEN = %q, want secret token", got)
		}
		if got := credentials.env["GIT_TERMINAL_PROMPT"]; got != "0" {
			t.Fatalf("GIT_TERMINAL_PROMPT = %q, want 0", got)
		}
		askpassPath := credentials.env["GIT_ASKPASS"]
		if askpassPath == "" {
			t.Fatal("GIT_ASKPASS is empty")
		}
		info, err := os.Stat(askpassPath)
		if err != nil {
			t.Fatalf("stat askpass helper: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Fatalf("askpass helper mode = %v, want 0700", got)
		}
		askpassSource := readTestFile(t, filepath.Dir(askpassPath), filepath.Base(askpassPath))
		if strings.Contains(askpassSource, "secret-token") {
			t.Fatal("askpass helper wrote the token to disk")
		}

		assertAskpassOutput(t, credentials.env, askpassPath, "Username for https://github.com", "x-access-token")
		assertAskpassOutput(t, credentials.env, askpassPath, "Password for https://github.com", "secret-token")
	})

	t.Run("Should force public module resolution through the Go proxy", func(t *testing.T) {
		t.Parallel()

		env := webAssetsPublicModuleEnv("/tmp/agh-web-assets-test")
		want := map[string]string{
			"GO111MODULE": "on",
			"GOFLAGS":     "-mod=mod",
			"GONOPROXY":   "",
			"GONOSUMDB":   "",
			"GOPRIVATE":   "",
			"GOPROXY":     "https://proxy.golang.org,direct",
			"GOSUMDB":     "sum.golang.org",
			"GOMODCACHE":  filepath.Join("/tmp/agh-web-assets-test", "mod"),
			"GOPATH":      filepath.Join("/tmp/agh-web-assets-test", "gopath"),
		}
		for key, value := range want {
			if env[key] != value {
				t.Fatalf("webAssetsPublicModuleEnv() %s = %q, want %q", key, env[key], value)
			}
		}
	})

	t.Run("Should parse generated assets metadata from a tag", func(t *testing.T) {
		t.Parallel()

		source := strings.Join([]string{
			"package webassets",
			"const (",
			"\tBuildDigest = \"digest-123\"",
			"\tSourceRepository = \"github.com/compozy/agh\"",
			"\tSourceCommit = \"abcdef123456\"",
			")",
		}, "\n")
		got := parseWebAssetsMetadataSource(source)
		want := webAssetsMetadata{
			BuildDigest:      "digest-123",
			SourceRepository: webAssetsSourceRepository,
			SourceCommit:     "abcdef123456",
		}
		if got != want {
			t.Fatalf("parseWebAssetsMetadataSource() = %#v, want %#v", got, want)
		}
	})
}

func TestWebAssetsPrepare(t *testing.T) {
	t.Parallel()

	t.Run("Should be a no-op for the same BuildDigest and SourceCommit", func(t *testing.T) {
		t.Parallel()

		srcDist := t.TempDir()
		writeTestFile(t, srcDist, "index.html", "<!doctype html><div id=\"app\"></div>")
		writeTestFile(t, srcDist, "assets/app.js", "console.log('app');")
		buildDigest, err := directoryDigest(srcDist)
		if err != nil {
			t.Fatalf("directoryDigest(srcDist) error = %v", err)
		}
		metadata := webAssetsMetadata{
			BuildDigest:      buildDigest,
			SourceRepository: webAssetsSourceRepository,
			SourceCommit:     "source-commit-1",
		}

		repoDir := t.TempDir()
		if err := prepareWebAssetsRepo(srcDist, repoDir, metadata); err != nil {
			t.Fatalf("prepareWebAssetsRepo(first) error = %v", err)
		}
		firstDigest, err := directoryDigest(repoDir)
		if err != nil {
			t.Fatalf("directoryDigest(repo first) error = %v", err)
		}

		if err := prepareWebAssetsRepo(srcDist, repoDir, metadata); err != nil {
			t.Fatalf("prepareWebAssetsRepo(second) error = %v", err)
		}
		secondDigest, err := directoryDigest(repoDir)
		if err != nil {
			t.Fatalf("directoryDigest(repo second) error = %v", err)
		}
		if firstDigest != secondDigest {
			t.Fatalf("repo digest changed for identical metadata: %s != %s", firstDigest, secondDigest)
		}
	})
}

func TestWebAssetsDeterminismCheck(t *testing.T) {
	t.Parallel()

	t.Run("Should require two clean builds with matching digests", func(t *testing.T) {
		t.Parallel()

		cleanCount := 0
		buildCount := 0
		err := webAssetsDeterminismCheck(
			func() error {
				buildCount++
				return nil
			},
			func() error {
				cleanCount++
				return nil
			},
			func() (string, error) {
				return "same-digest", nil
			},
		)
		if err != nil {
			t.Fatalf("webAssetsDeterminismCheck() error = %v", err)
		}
		if cleanCount != 2 || buildCount != 2 {
			t.Fatalf("clean/build counts = %d/%d, want 2/2", cleanCount, buildCount)
		}
	})

	t.Run("Should fail when clean builds produce different digests", func(t *testing.T) {
		t.Parallel()

		digests := []string{"first", "second"}
		err := webAssetsDeterminismCheck(
			func() error { return nil },
			func() error { return nil },
			func() (string, error) {
				next := digests[0]
				digests = digests[1:]
				return next, nil
			},
		)
		if err == nil {
			t.Fatal("webAssetsDeterminismCheck() error = nil, want mismatch error")
		}
		if !strings.Contains(err.Error(), "not deterministic") {
			t.Fatalf("webAssetsDeterminismCheck() error = %v, want determinism message", err)
		}
	})
}

func TestInstallerCheck(t *testing.T) {
	t.Parallel()

	t.Run("Should validate the installer script in dry-run mode", func(t *testing.T) {
		t.Parallel()

		if err := InstallerCheck(); err != nil {
			t.Fatalf("InstallerCheck() error = %v", err)
		}
	})
}

func writeTestFile(t *testing.T, root string, rel string, body string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", rel, err)
	}
}

func readTestFile(t *testing.T, root string, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", rel, err)
	}
	return string(data)
}

func assertAskpassOutput(t *testing.T, env map[string]string, askpassPath string, prompt string, want string) {
	t.Helper()

	cmd := exec.Command(askpassPath, prompt)
	cmd.Env = mergeCommandEnv(env)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("askpass %q error = %v\n%s", prompt, err, output)
	}
	if got := strings.TrimSpace(string(output)); got != want {
		t.Fatalf("askpass %q output = %q, want %q", prompt, got, want)
	}
}
