package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

func TestGoReleaserConfigPreservesTrustArtifactsAndPackageTargets(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	data, err := os.ReadFile(filepath.Join(root, ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("os.ReadFile(.goreleaser.yml) error = %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal(.goreleaser.yml) error = %v", err)
	}

	t.Run("Should preserve checksum signing configuration", func(t *testing.T) {
		checksum := mapAt(t, cfg, "checksum")
		if got, want := stringAt(t, checksum, "name_template"), "checksums.txt"; got != want {
			t.Fatalf("checksum.name_template = %q, want %q", got, want)
		}
		if got, want := stringAt(t, checksum, "algorithm"), "sha256"; got != want {
			t.Fatalf("checksum.algorithm = %q, want %q", got, want)
		}

		signs := sliceAt(t, cfg, "signs")
		if len(signs) == 0 {
			t.Fatal("signs is empty, want checksum signing preserved")
		}
		firstSign := asMap(t, signs[0], "signs[0]")
		if got, want := stringAt(t, firstSign, "cmd"), "cosign"; got != want {
			t.Fatalf("signs[0].cmd = %q, want %q", got, want)
		}
		if got, want := stringAt(t, firstSign, "artifacts"), "checksum"; got != want {
			t.Fatalf("signs[0].artifacts = %q, want %q", got, want)
		}
		if !stringSliceContains(sliceAt(t, firstSign, "args"), "sign-blob") {
			t.Fatalf("signs[0].args = %#v, want sign-blob", firstSign["args"])
		}
		if got, want := stringAt(t, firstSign, "signature"), "${artifact}.sigstore.json"; got != want {
			t.Fatalf("signs[0].signature = %q, want %q", got, want)
		}
		if !stringSliceContains(sliceAt(t, firstSign, "args"), "--bundle=${signature}") {
			t.Fatalf("signs[0].args = %#v, want --bundle=${signature}", firstSign["args"])
		}
	})

	t.Run("Should preserve SBOM artifact coverage", func(t *testing.T) {
		t.Parallel()

		sboms := sliceAt(t, cfg, "sboms")
		assertUniqueSBOMIDs(t, sboms)
		assertSBOMArtifact(t, sboms, "archive", "archive")
		assertSBOMArtifact(t, sboms, "package", "package")
		assertSBOMArtifact(t, sboms, "source", "source")
	})

	t.Run("Should build embedded web bundle before release binaries", func(t *testing.T) {
		t.Parallel()

		before := mapAt(t, cfg, "before")
		hooks := sliceAt(t, before, "hooks")
		if !stringSliceContains(hooks, "go run github.com/magefile/mage@v1.17.2 webBuild") {
			t.Fatalf("before.hooks = %#v, want webBuild before GoReleaser builds embedded web assets", hooks)
		}
		if !stringSliceContains(hooks, "go run github.com/magefile/mage@v1.17.2 webAssetsCheck") {
			t.Fatalf("before.hooks = %#v, want webAssetsCheck before GoReleaser builds binaries", hooks)
		}
		if !stringSliceContains(hooks, "bash scripts/render-install-script.sh {{ .Tag }} .artifacts/install.sh") {
			t.Fatalf("before.hooks = %#v, want the installer rendered with the release tag", hooks)
		}
	})

	t.Run("Should validate update archives with the runtime policy before publishing", func(t *testing.T) {
		t.Parallel()

		hooks := sliceAt(t, cfg, "before_publish")
		if len(hooks) != 1 {
			t.Fatalf("before_publish len = %d, want 1 update archive policy hook", len(hooks))
		}
		hook := asMap(t, hooks[0], "before_publish[0]")
		if !stringSliceContains(sliceAt(t, hook, "ids"), "compozy-archive") {
			t.Fatalf("before_publish[0].ids = %#v, want compozy-archive", hook["ids"])
		}
		gotArtifacts := stringsFromSlice(t, sliceAt(t, hook, "artifacts"), "before_publish[0].artifacts")
		if wantArtifacts := []string{"archive"}; !slices.Equal(gotArtifacts, wantArtifacts) {
			t.Fatalf("before_publish[0].artifacts = %#v, want %#v", gotArtifacts, wantArtifacts)
		}
		command := stringAt(t, hook, "cmd")
		for _, want := range []string{"updateArchiveCheck", "{{ .ArtifactPath }}"} {
			if !strings.Contains(command, want) {
				t.Fatalf("before_publish[0].cmd = %q, want %q", command, want)
			}
		}
		output, ok := hook["output"].(bool)
		if !ok || !output {
			t.Fatalf("before_publish[0].output = %#v, want true", hook["output"])
		}
	})

	t.Run("Should publish stable archives and curl installer asset", func(t *testing.T) {
		t.Parallel()

		archives := sliceAt(t, cfg, "archives")
		if len(archives) != 1 {
			t.Fatalf("archives len = %d, want 1", len(archives))
		}
		archive := asMap(t, archives[0], "archives[0]")
		if got, want := stringAt(t, archive, "id"), "compozy-archive"; got != want {
			t.Fatalf("archives[0].id = %q, want %q", got, want)
		}
		nameTemplate := stringAt(t, archive, "name_template")
		for _, want := range []string{
			"{{ .ProjectName }}_{{ .Os }}_",
			`{{- if eq .Arch "amd64" }}x86_64`,
			`{{- else }}{{ .Arch }}{{ end }}`,
		} {
			if !strings.Contains(nameTemplate, want) {
				t.Fatalf("archives[0].name_template = %q, want to contain %q", nameTemplate, want)
			}
		}
		if strings.Contains(nameTemplate, "{{ .Version }}") {
			t.Fatalf("archives[0].name_template = %q, want stable name without version", nameTemplate)
		}

		release := mapAt(t, cfg, "release")
		github := mapAt(t, release, "github")
		if got, want := stringAt(t, github, "owner"), "compozy"; got != want {
			t.Fatalf("release.github.owner = %q, want %q", got, want)
		}
		if got, want := stringAt(t, github, "name"), "compozy"; got != want {
			t.Fatalf("release.github.name = %q, want %q", got, want)
		}

		extraFiles := sliceAt(t, release, "extra_files")
		assertReleaseExtraFile(t, extraFiles, "./.artifacts/install.sh", "install.sh")
	})

	t.Run("Should keep Homebrew publishing outside deprecated GoReleaser targets", func(t *testing.T) {
		t.Parallel()

		if _, ok := cfg["homebrew_casks"]; ok {
			t.Fatal("homebrew_casks configured, want the existing Formula/compozy.rb channel")
		}
		if _, ok := cfg["brews"]; ok {
			t.Fatal("brews configured, want no deprecated GoReleaser Homebrew publisher")
		}
	})

	t.Run("Should configure Linux package targets", func(t *testing.T) {
		t.Parallel()

		nfpms := sliceAt(t, cfg, "nfpms")
		if len(nfpms) != 1 {
			t.Fatalf("nfpms len = %d, want 1", len(nfpms))
		}
		nfpm := asMap(t, nfpms[0], "nfpms[0]")
		if got, want := stringAt(t, nfpm, "id"), "compozy-linux-packages"; got != want {
			t.Fatalf("nfpms[0].id = %q, want %q", got, want)
		}
		if !stringSliceContains(sliceAt(t, nfpm, "ids"), "compozy") {
			t.Fatalf("nfpms[0].ids = %#v, want compozy build id", nfpm["ids"])
		}
		formats := sliceAt(t, nfpm, "formats")
		for _, want := range []string{"deb", "rpm"} {
			if !stringSliceContains(formats, want) {
				t.Fatalf("nfpms[0].formats = %#v, want %s", formats, want)
			}
		}
	})

	t.Run("Should configure public NPM package target", func(t *testing.T) {
		t.Parallel()

		npms := sliceAt(t, cfg, "npms")
		if len(npms) != 1 {
			t.Fatalf("npms len = %d, want 1", len(npms))
		}
		npm := asMap(t, npms[0], "npms[0]")
		assertEqualString(t, "npms[0].name", stringAt(t, npm, "name"), "@compozy/cli")
		if !stringSliceContains(sliceAt(t, npm, "ids"), "compozy-archive") {
			t.Fatalf("npms[0].ids = %#v, want compozy-archive", npm["ids"])
		}
		assertEqualString(t, "npms[0].access", stringAt(t, npm, "access"), "public")
		assertEqualString(t, "npms[0].format", stringAt(t, npm, "format"), "tar.gz")
		assertEqualString(
			t,
			"npms[0].repository",
			stringAt(t, npm, "repository"),
			"git+https://github.com/compozy/compozy.git",
		)
		assertEqualString(t, "npms[0].homepage", stringAt(t, npm, "homepage"), "https://compozy.com")
		assertEqualString(t, "npms[0].tag", stringAt(t, npm, "tag"), "{{ .Env.NPM_TAG }}")
		disableTemplate := stringAt(t, npm, "disable")
		assertEqualString(
			t,
			"npms[0].disable",
			disableTemplate,
			`{{ eq (index .Env "GORELEASER_PUBLISH_NPM") "false" }}`,
		)

		cases := []struct {
			name string
			env  map[string]string
			want string
		}{
			{
				name: "Should keep package generation enabled when the publish control is absent",
				env:  map[string]string{},
				want: "false",
			},
			{
				name: "Should disable the NPM publisher when publication is deferred",
				env:  map[string]string{"GORELEASER_PUBLISH_NPM": "false"},
				want: "true",
			},
			{
				name: "Should keep the NPM publisher enabled when publication is allowed",
				env:  map[string]string{"GORELEASER_PUBLISH_NPM": "true"},
				want: "false",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				tmpl, err := template.New("npm-disable").Option("missingkey=error").Parse(disableTemplate)
				if err != nil {
					t.Fatalf("template.Parse(npms[0].disable) error = %v", err)
				}

				var rendered strings.Builder
				if err := tmpl.Execute(&rendered, map[string]any{"Env": tc.env}); err != nil {
					t.Fatalf("template.Execute(npms[0].disable) error = %v", err)
				}
				if got := rendered.String(); got != tc.want {
					t.Fatalf("rendered npms[0].disable = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("Should use supported publication policy without an AUR target", func(t *testing.T) {
		t.Parallel()

		release := mapAt(t, cfg, "release")
		assertEqualString(
			t,
			"release.prerelease",
			stringAt(t, release, "prerelease"),
			"auto",
		)
		assertEqualString(
			t,
			"release.make_latest",
			stringAt(t, release, "make_latest"),
			"{{ .Env.GITHUB_MAKE_LATEST }}",
		)
		assertEqualString(
			t,
			"release.name_template",
			stringAt(t, release, "name_template"),
			"CompozyOS {{ .Version }} — {{ .Env.RELEASE_CHANNEL }}",
		)
		for _, key := range []string{"aurs", "aursources"} {
			if _, ok := cfg[key]; ok {
				t.Fatalf("%s configured, want no active AUR publisher", key)
			}
		}
	})
}

func TestHomebrewFormulaRendererPreservesLegacyFormulaIdentity(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	checksumsPath := filepath.Join(t.TempDir(), "checksums.txt")
	checksums := strings.Join([]string{
		strings.Repeat("1", 64) + "  compozy_darwin_x86_64.tar.gz",
		strings.Repeat("2", 64) + "  compozy_darwin_arm64.tar.gz",
		strings.Repeat("3", 64) + "  compozy_linux_x86_64.tar.gz",
		strings.Repeat("4", 64) + "  compozy_linux_arm64.tar.gz",
	}, "\n") + "\n"
	if err := os.WriteFile(checksumsPath, []byte(checksums), 0o600); err != nil {
		t.Fatalf("os.WriteFile(checksums) error = %v", err)
	}
	formulaPath := filepath.Join(t.TempDir(), "Formula", "compozy.rb")
	cmd := exec.CommandContext(
		t.Context(),
		"bash",
		filepath.Join(root, "scripts", "render-homebrew-formula.sh"),
		"--release-repo", "compozy/compozy",
		"--release-version", "9.9.9",
		"--release-tag", "v9.9.9",
		"--checksums", checksumsPath,
		"--output", formulaPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render-homebrew-formula.sh error = %v, output = %s", err, output)
	}
	formula := readTextFile(t, "", formulaPath)
	for _, snippet := range []string{
		"class Compozy < Formula",
		`version "9.9.9"`,
		`license "MIT"`,
		"https://github.com/compozy/compozy/releases/download/v9.9.9/compozy_darwin_arm64.tar.gz",
		"https://github.com/compozy/compozy/releases/download/v9.9.9/compozy_linux_x86_64.tar.gz",
		`bin.install "compozy"`,
		`system "#{bin}/compozy", "version"`,
	} {
		assertContainsText(t, "Homebrew formula", formula, snippet)
	}
}

func TestGoReleaserArchivesStayAlignedWithPublicInstaller(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	goreleaser := readYAMLMap(t, root, ".goreleaser.yml")
	installScript := readTextFile(t, root, filepath.Join("scripts", "install.template.sh"))

	projectName := stringAt(t, goreleaser, "project_name")
	assertEqualString(t, "goreleaser project_name", projectName, "compozy")
	build := firstMapAt(t, goreleaser, "builds")
	buildID := stringAt(t, build, "id")
	assertEqualString(t, "build id", buildID, "compozy")
	assertEqualString(t, "build binary", stringAt(t, build, "binary"), "compozy")
	assertEqualString(t, "build main", stringAt(t, build, "main"), ".")
	ldflags := strings.Join(stringsFromSlice(t, sliceAt(t, build, "ldflags"), "builds[0].ldflags"), "\n")
	assertContainsText(
		t,
		"GoReleaser ldflags",
		ldflags,
		"github.com/compozy/compozy/internal/version.Version",
	)
	assertNotContainsText(t, "GoReleaser ldflags", ldflags, "github.com/pedronauck/compozy")

	archive := firstMapAt(t, goreleaser, "archives")
	if !stringSliceContains(sliceAt(t, archive, "ids"), buildID) {
		t.Fatalf("archives[0].ids = %#v, want build id %q", archive["ids"], buildID)
	}
	nameTemplate := stringAt(t, archive, "name_template")
	for _, fragment := range []string{
		"{{ .ProjectName }}_{{ .Os }}_",
		`{{- if eq .Arch "amd64" }}x86_64`,
		`{{- else if eq .Arch "386" }}i386`,
		`{{- else }}{{ .Arch }}{{ end }}`,
	} {
		if !strings.Contains(nameTemplate, fragment) {
			t.Fatalf("archives[0].name_template = %q, want fragment %q", nameTemplate, fragment)
		}
	}
	if !strings.Contains(installScript, `ARCHIVE_NAME="compozy_${OS}_${ARCH}.tar.gz"`) {
		t.Fatalf("install.sh archive naming must stay aligned with GoReleaser template")
	}

	goos := stringsFromSlice(t, sliceAt(t, build, "goos"), "builds[0].goos")
	goarch := stringsFromSlice(t, sliceAt(t, build, "goarch"), "builds[0].goarch")
	for _, platform := range []string{"linux", "darwin"} {
		if !stringListContains(goos, platform) {
			t.Fatalf("builds[0].goos = %#v, want installer platform %q", goos, platform)
		}
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if !stringListContains(goarch, arch) {
			t.Fatalf("builds[0].goarch = %#v, want installer architecture %q", goarch, arch)
		}
	}

	release := mapAt(t, goreleaser, "release")
	github := mapAt(t, release, "github")
	releaseRepo := shellAssignment(t, installScript, "RELEASE_REPO")
	goreleaserRepo := stringAt(t, github, "owner") + "/" + stringAt(t, github, "name")
	assertEqualString(t, "installer RELEASE_REPO", releaseRepo, goreleaserRepo)
	if !strings.Contains(installScript, `TARGET="${INSTALL_DIR}/compozy"`) {
		t.Fatalf("install.sh must install the same binary name GoReleaser builds")
	}
	t.Run("Should default to the rendered placeholder pin with COMPOZY_VERSION override", func(t *testing.T) {
		t.Parallel()

		installerVersion := shellAssignment(t, installScript, "VERSION")
		if got, want := installerVersion, "${COMPOZY_VERSION:-__COMPOZY_PINNED_VERSION__}"; got != want {
			t.Fatalf("installer template VERSION = %q, want %q", got, want)
		}
	})
	assertNotContainsText(t, "installer", installScript, "resolve_latest_release_tag")
	assertNotContainsText(t, "installer", installScript, "releases/latest")
}

func TestReleaseTemplatesStayAlignedWithPublicInstallMethods(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	header := readTextFile(t, root, ".goreleaser.release-header.md.tmpl")
	footer := readTextFile(t, root, ".goreleaser.release-footer.md.tmpl")

	t.Run("Should keep release header aligned with public install methods", func(t *testing.T) {
		t.Parallel()

		for _, snippet := range []string{
			"npm install -g @compozy/cli@beta",
			"go install github.com/compozy/compozy@{{ .Tag }}",
			"curl -fsSL https://compozy.com/install.sh | sh",
			"Verified Binary Installer",
		} {
			assertContainsText(t, "GoReleaser release header", header, snippet)
		}
		assertNotContainsText(t, "GoReleaser release header", header, "brew install")
		assertNotContainsText(t, "GoReleaser release header", header, "github.com/pedronauck/compozy")
	})

	t.Run("Should keep release footer honest about verification posture", func(t *testing.T) {
		t.Parallel()

		for _, snippet := range []string{
			"### Verification posture",
			"`checksums.txt.sigstore.json`",
			"Syft SBOMs for archives, packages, and source",
			"does not claim a manual post-release install smoke",
		} {
			assertContainsText(t, "GoReleaser release footer", footer, snippet)
		}
		assertNotContainsText(t, "GoReleaser release footer", footer, "All release artifacts are signed")
		assertNotContainsText(t, "GoReleaser release footer", footer, "production-ready")
	})
}

func TestReleaseWorkflowConsumesExplicitPlan(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	workflow := readTextFile(t, root, filepath.Join(".github", "workflows", "release.yml"))

	for _, snippet := range []string{
		"release_ref:",
		"release_version:",
		"release_channel:",
		"release_previous_tag:",
		"release_git_range:",
		"release_initial:",
		"github.com/compozy/releasepr@v0.0.25",
		"COMPOZY_WEB_ASSETS_TOKEN: ${{ secrets.COMPOZY_WEB_ASSETS_TOKEN }}",
		"go run \"${PR_RELEASE_MODULE}\" plan",
		"render-and-validate-release-body.sh",
		"scripts/resolve-auto-beta-version.sh",
		"validate-release-body.sh",
		"PR_RELEASE_LOG_LEVEL: error",
		"scripts/release-plan-contract.sh",
		"ref: ${{ needs.release-plan.outputs.release_ref }}",
		"if [[ \"$(git rev-parse HEAD)\" != \"${RELEASE_COMMIT}\" ]]",
		"ensure_annotated_tag \"${RELEASE_TAG}\"",
		"sdk_go_tag=\"sdk/go/${RELEASE_TAG}\"",
		"git push origin \"refs/tags/${RELEASE_TAG}\" \"refs/tags/${sdk_go_tag}\"",
		"scripts/publish-extension-sdk.sh \"${RELEASE_VERSION}\" \"${NPM_TAG}\"",
		"scripts/release-npm-readiness.sh",
		"GORELEASER_CURRENT_TAG: ${{ needs.release-plan.outputs.release_tag }}",
		"RELEASE_VERSION: ${{ needs.release-plan.outputs.release_version }}",
		"RELEASE_CHANNEL: ${{ needs.release-plan.outputs.release_channel }}",
		"GITHUB_PRERELEASE: ${{ needs.release-plan.outputs.github_prerelease }}",
		"GITHUB_MAKE_LATEST: ${{ needs.release-plan.outputs.github_make_latest }}",
		"NPM_TAG: ${{ needs.release-plan.outputs.npm_tag }}",
		"HOMEBREW_SKIP_UPLOAD: ${{ needs.release-plan.outputs.homebrew_skip_upload }}",
		"if: needs.release-plan.outputs.homebrew_skip_upload == 'false'",
		"scripts/render-homebrew-formula.sh",
		"repository: compozy/homebrew-compozy",
		"--output .release-homebrew/Formula/compozy.rb",
	} {
		assertContainsText(t, "release workflow", workflow, snippet)
	}
	for _, forbidden := range []string{
		"git cliff --bumped-version",
		"GITHUB_HEAD_REF",
		"RELEASE_PR_TITLE",
		"@compozy/compozy",
		"compozy/compozy",
		"schedule:",
		"aurs:",
		"aursources:",
		"secrets.COMPOZY_WEB_ASSETS_TOKEN || secrets.RELEASE_TOKEN",
		"cp RELEASE_BODY.md",
		"Publish site changelog receipt",
		"release:site-changelog",
		"sed -i -E",
		`git describe --tags --abbrev=0 "${RELEASE_TAG}^"`,
	} {
		assertNotContainsText(t, "release workflow", workflow, forbidden)
	}
	if got := strings.Count(workflow, `go run "${PR_RELEASE_MODULE}" plan`); got != 2 {
		t.Fatalf("release workflow plan invocation count = %d, want dry-run and production plans", got)
	}
	if got := strings.Count(workflow, "\n            validate-release-body.sh\n"); got != 1 {
		t.Fatalf("release body validator staging count = %d, want 1", got)
	}
	if got := strings.Count(workflow, `render-and-validate-release-body.sh`); got != 3 {
		t.Fatalf("release body render/validation reference count = %d, want staging plus two invocations", got)
	}

	t.Run("Should derive the next beta from the greatest numeric tag", func(t *testing.T) {
		t.Parallel()

		repo := t.TempDir()
		manifest := filepath.Join(repo, "package.json")
		if err := os.WriteFile(manifest, []byte("{\"version\":\"0.3.0\"}\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(package.json) error = %v", err)
		}
		runReleasePreflightFixtureCommand(t, repo, "git", "init", "--initial-branch=main")
		runReleasePreflightFixtureCommand(t, repo, "git", "config", "user.name", "Compozy Beta Resolver")
		runReleasePreflightFixtureCommand(t, repo, "git", "config", "user.email", "beta-resolver@compozy.com")
		runReleasePreflightFixtureCommand(t, repo, "git", "add", "package.json")
		runReleasePreflightFixtureCommand(
			t,
			repo,
			"git",
			"-c",
			"commit.gpgsign=false",
			"commit",
			"-m",
			"test: seed beta resolver",
		)
		for _, tag := range []string{"v0.3.0-beta.13", "v0.3.0-beta.14", "v0.3.0-beta.preview"} {
			runReleasePreflightFixtureCommand(t, repo, "git", "tag", tag)
		}

		cmd := exec.CommandContext(t.Context(), "bash", filepath.Join(root, "scripts", "resolve-auto-beta-version.sh"))
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("resolve-auto-beta-version.sh error = %v, output = %s", err, output)
		}
		if got, want := strings.TrimSpace(string(output)), "0.3.0-beta.15"; got != want {
			t.Fatalf("resolved beta version = %q, want %q", got, want)
		}
	})

	t.Run("Should reject empty ranges and heading-only release bodies", func(t *testing.T) {
		t.Parallel()

		repo := t.TempDir()
		runReleasePreflightFixtureCommand(t, repo, "git", "init", "--initial-branch=main")
		runReleasePreflightFixtureCommand(t, repo, "git", "config", "user.name", "Compozy Release Body")
		runReleasePreflightFixtureCommand(t, repo, "git", "config", "user.email", "release-body@compozy.com")
		seed := filepath.Join(repo, "release.txt")
		if err := os.WriteFile(seed, []byte("beta 14\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(seed) error = %v", err)
		}
		runReleasePreflightFixtureCommand(t, repo, "git", "add", "release.txt")
		runReleasePreflightFixtureCommand(
			t,
			repo,
			"git",
			"-c",
			"commit.gpgsign=false",
			"commit",
			"-m",
			"test: seed release body",
		)
		runReleasePreflightFixtureCommand(t, repo, "git", "tag", "v0.3.0-beta.14")
		if err := os.WriteFile(seed, []byte("beta 15\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(candidate) error = %v", err)
		}
		runReleasePreflightFixtureCommand(t, repo, "git", "add", "release.txt")
		runReleasePreflightFixtureCommand(
			t,
			repo,
			"git",
			"-c",
			"commit.gpgsign=false",
			"commit",
			"-m",
			"fix: validate release body",
		)

		validator := filepath.Join(root, "scripts", "validate-release-body.sh")
		body := filepath.Join(repo, "body.md")
		validBody := "## 0.3.0-beta.15 - 2026-08-13\n\n" +
			"### Bug Fixes\n\n" +
			"- Validate release body\n"
		if err := os.WriteFile(body, []byte(validBody), 0o600); err != nil {
			t.Fatalf("os.WriteFile(valid body) error = %v", err)
		}
		run := func(gitRange string) ([]byte, error) {
			cmd := exec.CommandContext(
				t.Context(),
				"bash",
				validator,
				"v0.3.0-beta.15",
				gitRange,
				"false",
				body,
			)
			cmd.Dir = repo
			return cmd.CombinedOutput()
		}
		if output, err := run("v0.3.0-beta.14..HEAD"); err != nil {
			t.Fatalf("valid release body error = %v, output = %s", err, output)
		}
		output, err := run("")
		if err == nil {
			t.Fatalf("empty release range unexpectedly passed: %s", output)
		}
		assertContainsText(t, "release body validator", string(output), "non-initial release must include a Git range")

		if err := os.WriteFile(body, []byte("## 0.3.0-beta.15 - 2026-08-13\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(heading-only body) error = %v", err)
		}
		output, err = run("v0.3.0-beta.14..HEAD")
		if err == nil {
			t.Fatalf("heading-only release body unexpectedly passed: %s", output)
		}
		assertContainsText(t, "release body validator", string(output), "release body contains no changelog entries")
	})
}

func TestDesktopReleaseWorkflowFailsClosedAndPublishesDraftLast(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	workflow := readTextFile(t, root, filepath.Join(".github", "workflows", "release.yml"))
	cargoManifest := readTextFile(t, root, filepath.Join("desktop", "src-tauri", "Cargo.toml"))
	goreleaser := readYAMLMap(t, root, ".goreleaser.yml")
	release := mapAt(t, goreleaser, "release")

	t.Run("Should publish package managers only after public installation succeeds", func(t *testing.T) {
		t.Parallel()

		if got, ok := release["draft"].(bool); !ok || !got {
			t.Fatalf("release.draft = %#v, want true", release["draft"])
		}
		publishFeed := strings.Index(workflow, "scripts/publish-desktop-release.sh")
		stageDraft := strings.Index(workflow, "name: Stage GitHub draft without publishing npm")
		publishDraft := strings.Index(workflow, "- name: Publish GitHub release before downstream package managers")
		publicInstall := strings.Index(workflow, "  cli-public-install-smoke:")
		publishNPM := strings.Index(workflow, "- name: Publish verified CLI package")
		verifyPublished := strings.Index(workflow, "- name: Verify published channel policy")
		if publishFeed == -1 || stageDraft == -1 || publishDraft == -1 || publicInstall == -1 ||
			publishNPM == -1 || verifyPublished == -1 || !strings.Contains(workflow, "-F draft=false") {
			t.Fatal("release workflow is missing staged, public-install, or package publication phases")
		}
		if stageDraft > publishFeed || publishFeed > publishDraft || publishDraft > publicInstall ||
			publicInstall > publishNPM {
			t.Fatal("release workflow must publish GitHub and prove a public install before npm publication")
		}
		assertContainsText(t, "GoReleaser npm custody", workflow, `GORELEASER_PUBLISH_NPM: "false"`)
		assertContainsText(t, "public CLI installation", workflow, "smoke-public-cli-package.sh")

		finalizer := workflow[publishDraft:verifyPublished]
		for _, required := range []string{
			`gh api --paginate --slurp "repos/${GITHUB_REPOSITORY}/releases?per_page=100"`,
			`select(.tag_name == $tag)`,
			`if [[ "${matching_count}" != "1" ]]; then`,
			`release_id="$(jq -r '.[0].id' <<<"$matching_releases")"`,
			`release_draft="$(jq -r '.[0].draft' <<<"$matching_releases")"`,
			`if [[ "${release_draft}" == "true" ]]; then`,
			`elif [[ "${release_draft}" == "false" ]]; then`,
			`"repos/${GITHUB_REPOSITORY}/releases/${release_id}"`,
		} {
			assertContainsText(t, "GitHub draft finalizer", finalizer, required)
		}
		assertNotContainsText(t, "GitHub draft finalizer", finalizer, `releases/tags/${RELEASE_TAG}`)
		if got := strings.Count(finalizer, "-F draft=false"); got != 1 {
			t.Fatalf("GitHub draft publication count = %d, want exactly one finalizer", got)
		}
	})

	t.Run("Should pin the shipping-platform matrix and Tauri v1 contract", func(t *testing.T) {
		t.Parallel()

		for _, snippet := range []string{
			"fail-fast: false",
			"matrix: &desktop-release-matrix",
			"matrix: *desktop-release-matrix",
			"runner: macos-15",
			"target: universal-apple-darwin",
			"runner: ubuntu-22.04",
			"target: x86_64-unknown-linux-gnu",
			"uses: tauri-apps/tauri-action@v1",
			"releaseDraft: true",
			"uploadUpdaterJson: false",
			"uploadWorkflowArtifacts: true",
			"libwebkit2gtk-4.1-dev",
			"xdg-utils",
		} {
			assertContainsText(t, "desktop release workflow", workflow, snippet)
		}
		// Windows is paused until Trusted Signing is restored.
		assertNotContainsText(t, "desktop release workflow", workflow, "runner: windows-latest")
		if got := strings.Count(workflow, "uses: tauri-apps/tauri-action@v1"); got != 2 {
			t.Fatalf("tauri-action step count = %d, want dry-run and production matrix steps", got)
		}
	})

	t.Run("Should bundle the shipping matrix without publishing on release PRs", func(t *testing.T) {
		t.Parallel()

		start := strings.Index(workflow, "  desktop-dry-run:")
		end := strings.Index(workflow, "  desktop-dry-run-smoke:")
		if start == -1 || end == -1 || start >= end {
			t.Fatal("release workflow is missing the desktop dry-run job before the nightly lane")
		}
		dryRun := workflow[start:end]
		for _, required := range []string{
			"name: Dry-Run Desktop ${{ matrix.name }}",
			"matrix: &desktop-release-matrix",
			"runner: macos-15",
			"target: universal-apple-darwin",
			"bundles: app,dmg",
			"runner: ubuntu-22.04",
			"target: x86_64-unknown-linux-gnu",
			"bundles: appimage,deb",
			"uses: tauri-apps/tauri-action@v1",
			"uploadUpdaterJson: false",
			"uploadWorkflowArtifacts: false",
			"COMPOZY_RELEASE_CHANNEL: beta",
			"scripts/normalize-desktop-artifacts.sh",
			"uses: actions/upload-artifact@v7",
		} {
			assertContainsText(t, "desktop release dry-run", dryRun, required)
		}
		for _, forbidden := range []string{
			"tagName:",
			"releaseName:",
			"releaseDraft:",
			"GITHUB_TOKEN:",
			"APPLE_CERTIFICATE:",
			"APPLE_ID:",
		} {
			assertNotContainsText(t, "desktop release dry-run", dryRun, forbidden)
		}

		smokeEnd := strings.Index(workflow, "  e2e-nightly:")
		if smokeEnd == -1 || end >= smokeEnd {
			t.Fatal("release workflow is missing the cross-job desktop dry-run smoke")
		}
		smoke := workflow[end:smokeEnd]
		for _, required := range []string{
			"needs: desktop-dry-run",
			"actions/download-artifact@v8",
			"scripts/smoke-desktop-release-artifact.sh",
			"desktop-dry-run-smoke-${{ matrix.id }}-diagnostics",
		} {
			assertContainsText(t, "desktop dry-run smoke", smoke, required)
		}
	})

	t.Run("Should assert signing material before bundling with current platform APIs", func(t *testing.T) {
		t.Parallel()

		preflight := strings.Index(workflow, "- name: Assert signing material before build")
		bundle := strings.Index(workflow, "- name: Build signed desktop bundle")
		if preflight == -1 || bundle == -1 || preflight > bundle {
			t.Fatal("desktop signing preflight must run before the Tauri build")
		}
		for _, snippet := range []string{
			"APPLE_ID: ${{ secrets.APPLE_ID }}",
			"APPLE_PASSWORD: ${{ secrets.APPLE_PASSWORD }}",
			"APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}",
			"xcrun notarytool submit",
			"xcrun stapler staple",
		} {
			assertContainsText(t, "desktop release workflow", workflow, snippet)
		}
		assertNotContainsText(t, "desktop release workflow", workflow, "APPLE_API_ISSUER")
		assertNotContainsText(t, "desktop release workflow", workflow, "APPLE_API_KEY")
	})

	t.Run("Should verify the signed runtime manifest with the desktop consumer before publication", func(t *testing.T) {
		t.Parallel()

		signManifest := strings.Index(workflow, "scripts/sign-runtime-manifest.sh")
		verifyManifest := strings.Index(workflow, "--bin compozy-runtime-manifest-check")
		publishFeed := strings.Index(workflow, "scripts/publish-desktop-release.sh")
		if signManifest == -1 || verifyManifest == -1 || publishFeed == -1 {
			t.Fatal(
				"desktop release workflow is missing candidate manifest signing, " +
					"consumer verification, or publication",
			)
		}
		if signManifest > verifyManifest || verifyManifest > publishFeed {
			t.Fatal("desktop release workflow must verify signed candidate manifest bytes before publication")
		}
		verification := workflow[signManifest:publishFeed]
		for _, required := range []string{
			"--features runtime-manifest-check",
			"TAURI_SIGNING_PUBLIC_KEY: ${{ secrets.TAURI_SIGNING_PUBLIC_KEY }}",
			".artifacts/feed/runtime.json.minisig",
			`"${RELEASE_VERSION}"`,
			"darwin-aarch64",
			"darwin-x86_64",
			"linux-x86_64",
		} {
			assertContainsText(t, "runtime manifest consumer verification", verification, required)
		}
		for _, required := range []string{
			"runtime-manifest-check = []",
			`required-features = ["runtime-manifest-check"]`,
		} {
			assertContainsText(t, "desktop Cargo manifest", cargoManifest, required)
		}
	})
}

func TestDesktopReleaseScriptsRefuseUnsafeOperatorInputs(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)

	t.Run("Should restore the AppImage executable bit after artifact transfer", func(t *testing.T) {
		t.Parallel()

		artifact := filepath.Join(t.TempDir(), "CompozyOS.AppImage")
		if err := os.WriteFile(artifact, []byte("fixture"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(AppImage) error = %v", err)
		}
		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "prepare-desktop-smoke-artifact.sh"),
			"linux",
			artifact,
		)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("prepare AppImage error = %v, output = %s", err, output)
		}
		info, err := os.Stat(artifact)
		if err != nil {
			t.Fatalf("os.Stat(AppImage) error = %v", err)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("AppImage mode = %o, want owner executable", info.Mode().Perm())
		}
	})

	t.Run("Should fail before build when signing material is absent", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "assert-desktop-signing-material.sh"),
			"linux",
		)
		cmd.Dir = root
		cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("signing preflight unexpectedly succeeded: %s", output)
		}
		assertContainsText(t, "signing preflight", string(output), "TAURI_SIGNING_PRIVATE_KEY is missing")
	})

	t.Run("Should refuse stable publication before touching credentials", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "publish-desktop-release.sh"),
			"desktop", "runtime", "feed", "stable", "0.4.0", "bucket", "https://example.r2.cloudflarestorage.com",
		)
		cmd.Dir = root
		cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("stable desktop publication unexpectedly succeeded: %s", output)
		}
		assertContainsText(t, "stable guard", string(output), "stable remains reserved")
	})

	t.Run("Should refuse update key generation in CI", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "generate-desktop-update-key.sh"),
			filepath.Join(t.TempDir(), "update.key"),
		)
		cmd.Dir = root
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"CI=true",
			"TAURI_SIGNING_PRIVATE_KEY_PASSWORD=not-used",
		}
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("CI key generation unexpectedly succeeded: %s", output)
		}
		assertContainsText(t, "key generation guard", string(output), "must never be generated in CI")
	})

	t.Run("Should create an update signing key with owner-only permissions", func(t *testing.T) {
		t.Parallel()

		binDir := t.TempDir()
		bunxStub := "#!/bin/sh\nfor argument do output=\"${argument}\"; done\nprintf private-key >\"${output}\"\n"
		if err := os.WriteFile(filepath.Join(binDir, "bunx"), []byte(bunxStub), 0o755); err != nil {
			t.Fatalf("os.WriteFile(bunx stub) error = %v", err)
		}
		outputPath := filepath.Join(t.TempDir(), "update.key")
		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "generate-desktop-update-key.sh"),
			outputPath,
		)
		cmd.Dir = root
		cmd.Env = []string{
			"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
			"TAURI_SIGNING_PRIVATE_KEY_PASSWORD=fixture",
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate update key error = %v; output=%s", err, output)
		}
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("os.Stat(update key) error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("update key mode = %04o, want 0600", got)
		}
	})

	t.Run("Should require exact runtime payload bytes from the signed manifest", func(t *testing.T) {
		t.Parallel()

		payloadDir := t.TempDir()
		payload := []byte("runtime-payload")
		payloadName := "compozy_darwin_arm64.tar.gz"
		payloadPath := filepath.Join(payloadDir, payloadName)
		if err := os.WriteFile(payloadPath, payload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(runtime payload) error = %v", err)
		}
		digest := sha256.Sum256(payload)
		manifest := map[string]any{
			"platforms": map[string]any{
				"darwin-aarch64": map[string]any{
					"url":    "https://releases.compozy.com/desktop/v/runtime/0.4.0-beta.2/" + payloadName,
					"size":   len(payload),
					"sha256": hex.EncodeToString(digest[:]),
				},
			},
		}
		manifestBytes, err := json.Marshal(manifest)
		if err != nil {
			t.Fatalf("json.Marshal(runtime manifest) error = %v", err)
		}
		manifestPath := filepath.Join(t.TempDir(), "runtime.json")
		if err := os.WriteFile(manifestPath, manifestBytes, 0o600); err != nil {
			t.Fatalf("os.WriteFile(runtime manifest) error = %v", err)
		}
		assertScript := filepath.Join(root, "scripts", "assert-runtime-manifest-payloads.sh")
		run := func() ([]byte, error) {
			cmd := exec.CommandContext(t.Context(), "bash", assertScript, manifestPath, payloadDir, "0.4.0-beta.2")
			cmd.Dir = root
			return cmd.CombinedOutput()
		}
		if output, err := run(); err != nil {
			t.Fatalf("valid runtime payload verification error = %v; output=%s", err, output)
		}
		if err := os.WriteFile(filepath.Join(payloadDir, "unexpected.txt"), []byte("extra"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(extra payload) error = %v", err)
		}
		output, err := run()
		if err == nil {
			t.Fatalf("runtime verification accepted an extra payload: %s", output)
		}
		assertContainsText(t, "runtime inventory", string(output), "unexpected runtime payload")
		if err := os.Remove(filepath.Join(payloadDir, "unexpected.txt")); err != nil {
			t.Fatalf("os.Remove(extra payload) error = %v", err)
		}
		if err := os.WriteFile(payloadPath, []byte("runtime-payloae"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(corrupt payload) error = %v", err)
		}
		output, err = run()
		if err == nil {
			t.Fatalf("runtime verification accepted a digest mismatch: %s", output)
		}
		assertContainsText(t, "runtime digest", string(output), "SHA-256 does not match")
		if err := os.Remove(payloadPath); err != nil {
			t.Fatalf("os.Remove(runtime payload) error = %v", err)
		}
		output, err = run()
		if err == nil {
			t.Fatalf("runtime verification accepted a missing payload: %s", output)
		}
		assertContainsText(t, "runtime inventory", string(output), "declared runtime payload is missing")
	})

	t.Run("Should fail artifact normalization when discovery fails after a match", func(t *testing.T) {
		t.Parallel()

		binDir := t.TempDir()
		findStub := "#!/bin/sh\nprintf '%s\\0' \"${EMITTED_MATCH}\"\nexit 7\n"
		if err := os.WriteFile(filepath.Join(binDir, "find"), []byte(findStub), 0o755); err != nil {
			t.Fatalf("os.WriteFile(find stub) error = %v", err)
		}
		bundleRoot := t.TempDir()
		match := filepath.Join(bundleRoot, "bundle", "macos", "CompozyOS.app.tar.gz")
		if err := os.MkdirAll(filepath.Dir(match), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(bundle) error = %v", err)
		}
		if err := os.WriteFile(match, []byte("fixture"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(bundle) error = %v", err)
		}
		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "normalize-desktop-artifacts.sh"),
			"macos", "0.4.0-beta.2", bundleRoot, t.TempDir(),
		)
		cmd.Dir = root
		cmd.Env = []string{
			"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
			"EMITTED_MATCH=" + match,
		}
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("normalization accepted a failed discovery: %s", output)
		}
		assertContainsText(t, "normalization discovery", string(output), "failed to enumerate")
	})
}

func TestReleasePreflightValidatesPublishWorkspace(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	preflight := filepath.Join(root, "scripts", "release-preflight.sh")
	tests := []struct {
		name            string
		contamination   string
		wantSuccess     bool
		wantOutputParts []string
	}{
		{
			name:        "Should accept a valid clean release workspace",
			wantSuccess: true,
			wantOutputParts: []string{
				"release preflight: PASS",
			},
		},
		{
			name:          "Should reject an untracked file in the release workspace",
			contamination: "untracked",
			wantOutputParts: []string{
				"Release worktree must be clean before publication",
				"?? release-workflow-tools/",
			},
		},
		{
			name:          "Should reject a tracked modification in the release workspace",
			contamination: "tracked",
			wantOutputParts: []string{
				"Release worktree must be clean before publication",
				" M RELEASE_BODY.md",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pathDir := newReleasePreflightFixture(t)
			switch tt.contamination {
			case "untracked":
				contamination := filepath.Join(repo, "release-workflow-tools", "tool.sh")
				if err := os.MkdirAll(filepath.Dir(contamination), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(contamination) error = %v", err)
				}
				if err := os.WriteFile(contamination, []byte("#!/bin/sh\n"), 0o600); err != nil {
					t.Fatalf("os.WriteFile(contamination) error = %v", err)
				}
			case "tracked":
				releaseBody := filepath.Join(repo, "RELEASE_BODY.md")
				if err := os.WriteFile(releaseBody, []byte("# Changed release body\n"), 0o600); err != nil {
					t.Fatalf("os.WriteFile(RELEASE_BODY.md) error = %v", err)
				}
			}

			cmd := exec.CommandContext(t.Context(), "bash", preflight)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			output, err := cmd.CombinedOutput()
			if tt.wantSuccess && err != nil {
				t.Fatalf("release-preflight.sh error = %v, output = %s", err, output)
			}
			if !tt.wantSuccess && err == nil {
				t.Fatalf("release-preflight.sh unexpectedly succeeded, output = %s", output)
			}
			for _, want := range tt.wantOutputParts {
				assertContainsText(t, "release preflight output", string(output), want)
			}
		})
	}
}

func TestReleasePublicationStateKeepsImmutableRegistryVersionsRecoverable(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	tests := []struct {
		name           string
		cliState       string
		extensionState string
		releases       string
		wantState      string
		wantStage      string
		wantDesktop    string
	}{
		{
			name:           "Should repair a missing GitHub release without rebuilding desktop",
			cliState:       "published",
			extensionState: "published",
			releases:       `[]`,
			wantState:      "missing_published_published",
			wantStage:      "true",
			wantDesktop:    "false",
		},
		{
			name:           "Should require desktop validation for a fresh release",
			cliState:       "missing",
			extensionState: "missing",
			releases:       `[]`,
			wantState:      "missing_missing_missing",
			wantStage:      "true",
			wantDesktop:    "true",
		},
		{
			name:           "Should resume an orphaned draft without presenting its old desktop as fixed",
			cliState:       "published",
			extensionState: "published",
			releases:       `[{"id":42,"tag_name":"v0.3.0-beta.15","target_commitish":"release-commit","draft":true,"assets":[{"id":1,"name":"checksums.txt","state":"uploaded","size":100},{"id":2,"name":"checksums.txt.sigstore.json","state":"uploaded","size":1},{"id":3,"name":"install.sh","state":"uploaded","size":1},{"id":4,"name":"compozy_darwin_arm64.tar.gz","state":"uploaded","size":1}]}]`,
			wantState:      "draft_published_published",
			wantStage:      "false",
			wantDesktop:    "false",
		},
		{
			name:           "Should resume npm publication from an already public GitHub release",
			cliState:       "missing",
			extensionState: "missing",
			releases:       `[{"id":42,"tag_name":"v0.3.0-beta.15","target_commitish":"release-commit","draft":false,"assets":[{"id":1,"name":"checksums.txt","state":"uploaded","size":100},{"id":2,"name":"checksums.txt.sigstore.json","state":"uploaded","size":1},{"id":3,"name":"install.sh","state":"uploaded","size":1},{"id":4,"name":"compozy_darwin_arm64.tar.gz","state":"uploaded","size":1}]}]`,
			wantState:      "published_missing_missing",
			wantStage:      "false",
			wantDesktop:    "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			binDir := t.TempDir()
			npmStub := `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "whoami" ]]; then exit 0; fi
package="$2"
state="${CLI_STATE}"
if [[ "${package}" == @compozy/extension-sdk@* ]]; then state="${EXTENSION_STATE}"; fi
if [[ "${state}" == "published" ]]; then printf '%s\n' "${RELEASE_VERSION}"; exit 0; fi
echo 'npm error E404 404 Not Found' >&2
exit 1
`
			ghStub := `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"releases/assets/"* ]]; then
  printf '%064d  compozy_darwin_arm64.tar.gz\n' 0
  exit 0
fi
printf '[%s]\n' "${GH_RELEASES}"
`
			for name, contents := range map[string]string{"npm": npmStub, "gh": ghStub} {
				if err := os.WriteFile(filepath.Join(binDir, name), []byte(contents), 0o755); err != nil {
					t.Fatalf("os.WriteFile(%s stub) error = %v", name, err)
				}
			}

			outputPath := filepath.Join(t.TempDir(), "github-output")
			cmd := exec.CommandContext(
				t.Context(),
				"bash",
				filepath.Join(root, "scripts", "release-publication-state.sh"),
			)
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"CLI_STATE="+tt.cliState,
				"EXTENSION_STATE="+tt.extensionState,
				"GH_RELEASES="+tt.releases,
				"GITHUB_OUTPUT="+outputPath,
				"GITHUB_REPOSITORY=compozy/compozy",
				"NPM_TOKEN=fixture",
				"RELEASE_COMMIT=release-commit",
				"RELEASE_TAG=v0.3.0-beta.15",
				"RELEASE_VERSION=0.3.0-beta.15",
				"RUNNER_TEMP="+t.TempDir(),
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("release-publication-state.sh error = %v, output = %s", err, output)
			}
			decision, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("os.ReadFile(GITHUB_OUTPUT) error = %v", err)
			}
			assertContainsText(t, "publication state", string(decision), "publication_state="+tt.wantState)
			assertContainsText(t, "stage decision", string(decision), "stage_release="+tt.wantStage)
			assertContainsText(t, "desktop decision", string(decision), "desktop_required="+tt.wantDesktop)
		})
	}
}

func TestReleaseWorkflowKeepsRepositoryCleanBeforeTagPublication(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	workflow := readTextFile(t, root, filepath.Join(".github", "workflows", "release.yml"))

	t.Run("Should share workflow tool staging outside the release checkout", func(t *testing.T) {
		t.Parallel()

		for _, snippet := range []string{
			"&load-release-workflow-tools",
			"*load-release-workflow-tools",
			"WORKFLOW_COMMIT: ${{ github.sha }}",
			"release-publication-state.sh",
			"release-preflight.sh",
			"Accept: application/vnd.github.raw+json",
			"repos/${GITHUB_REPOSITORY}/contents/scripts/${workflow_tool}?ref=${WORKFLOW_COMMIT}",
			`run: bash "${RUNNER_TEMP}/release-publication-state.sh"`,
		} {
			assertContainsText(t, "release workflow", workflow, snippet)
		}
		if got := strings.Count(workflow, "Accept: application/vnd.github.raw+json"); got != 2 {
			t.Fatalf("release workflow raw loader count = %d, want scripts and GoReleaser config", got)
		}
		assertNotContainsText(t, "release workflow", workflow, ".release-workflow-tools")
	})

	t.Run("Should run the same preflight before dry-run and tag publication", func(t *testing.T) {
		t.Parallel()

		preflightDefinition := strings.Index(workflow, "- &run-release-preflight")
		if preflightDefinition == -1 {
			t.Fatal("release workflow missing shared preflight definition")
		}
		dryRun := strings.Index(workflow, `go run "${{ env.PR_RELEASE_MODULE }}" dry-run --ci-output`)
		if dryRun == -1 {
			t.Fatal("release workflow missing release PR dry-run")
		}
		if preflightDefinition > dryRun {
			t.Fatal("release workflow must run the preflight before the release PR dry-run")
		}

		preflightAlias := strings.Index(workflow, "- *run-release-preflight")
		if preflightAlias == -1 {
			t.Fatal("release workflow missing production preflight alias")
		}
		tagPush := strings.Index(workflow, `git push origin "refs/tags/${RELEASE_TAG}"`)
		if tagPush == -1 {
			t.Fatal("release workflow missing release tag push")
		}
		if preflightAlias > tagPush {
			t.Fatal("release workflow must run the preflight before pushing the release tag")
		}
		if got := strings.Count(workflow, `run: bash "${RUNNER_TEMP}/release-preflight.sh"`); got != 1 {
			t.Fatalf("release workflow preflight definition count = %d, want 1 shared definition", got)
		}
		assertNotContainsText(t, "release workflow", workflow, "git status --porcelain --untracked-files=normal")
	})
}

func TestReleaseWorkflowVerifiesGoReleaserWithCompatibleCosign(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	workflow := readTextFile(t, root, filepath.Join(".github", "workflows", "release.yml"))
	setupAction := readTextFile(t, root, filepath.Join(".github", "actions", "setup-release", "action.yml"))

	t.Run("Should pin Cosign v3 in dry-run and production", func(t *testing.T) {
		t.Parallel()

		assertContainsText(t, "release workflow", workflow, `COSIGN_VERSION: "v3.1.3"`)
		if got := strings.Count(workflow, "cosign-version: ${{ env.COSIGN_VERSION }}"); got != 2 {
			t.Fatalf("release workflow Cosign input count = %d, want 2", got)
		}
		assertNotContainsText(t, "release workflow", workflow, "COSIGN_EXPERIMENTAL")
	})

	t.Run("Should install Cosign before GoReleaser verification", func(t *testing.T) {
		t.Parallel()

		cosignInputStart := strings.Index(setupAction, "  cosign-version:\n")
		cosignInputEnd := strings.Index(setupAction, "  setup-docker:\n")
		if cosignInputStart == -1 || cosignInputEnd == -1 || cosignInputStart >= cosignInputEnd {
			t.Fatal("setup release action missing Cosign input block")
		}
		cosignInput := setupAction[cosignInputStart:cosignInputEnd]
		assertContainsText(t, "setup release Cosign input", cosignInput, "required: true")
		assertNotContainsText(t, "setup release Cosign input", cosignInput, "default:")
		assertContainsText(t, "setup release action", setupAction, "uses: sigstore/cosign-installer@v4.1.2")

		cosignSetup := strings.Index(setupAction, "- name: Setup Cosign for artifact signing")
		goReleaserSetup := strings.Index(setupAction, "- name: Setup GoReleaser")
		if cosignSetup == -1 || goReleaserSetup == -1 {
			t.Fatal("setup release action missing Cosign or GoReleaser setup step")
		}
		if cosignSetup > goReleaserSetup {
			t.Fatal("setup release action must install Cosign before GoReleaser verification")
		}
		assertContainsText(t, "setup release action", setupAction, "uses: goreleaser/goreleaser-action@v7")
	})

	t.Run("Should expose the pinned verifier to the production release action", func(t *testing.T) {
		t.Parallel()

		productionSetup := strings.LastIndex(workflow, "- name: Setup Release Tools")
		productionRelease := strings.LastIndex(workflow, "- uses: goreleaser/goreleaser-action@v7")
		if productionSetup == -1 || productionRelease == -1 {
			t.Fatal("release workflow missing production setup or GoReleaser action")
		}
		if productionSetup > productionRelease {
			t.Fatal("release workflow must add Cosign to PATH before the production GoReleaser action")
		}
		productionPrerequisites := workflow[productionSetup:productionRelease]
		assertContainsText(
			t,
			"production release prerequisites",
			productionPrerequisites,
			"cosign-version: ${{ env.COSIGN_VERSION }}",
		)
		if got := strings.Count(workflow, "- uses: goreleaser/goreleaser-action@v7"); got != 2 {
			t.Fatalf("production GoReleaser action count = %d, want prepare and publish", got)
		}
	})
}

func TestExtensionSDKPublisherAcceptsStrictSemVer(t *testing.T) {
	t.Parallel()

	t.Run("Should accept prerelease and build metadata together", func(t *testing.T) {
		t.Parallel()

		root := findRepoRootForReleaseConfigTest(t)
		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "publish-extension-sdk.sh"),
			"1.2.3-rc.1+build.7",
			"INVALID",
		)
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("publish-extension-sdk.sh error = nil, want invalid npm tag rejection")
		}
		if !strings.Contains(string(output), "npm tag is invalid") {
			t.Fatalf("publish-extension-sdk.sh output = %s, want npm tag validation", output)
		}
	})
}

func TestNPMReleasePolicyWaitsForRegistryReadiness(t *testing.T) {
	t.Parallel()

	t.Run("Should wait until every package dist-tag converges", func(t *testing.T) {
		t.Parallel()

		root := findRepoRootForReleaseConfigTest(t)
		binDir := t.TempDir()
		counterPath := filepath.Join(binDir, "extension-sdk-queries")
		npmStub := `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" != "view" || "${3:-}" != "dist-tags" ]]; then
  printf 'unexpected npm invocation: %s\n' "$*" >&2
  exit 64
fi
if [[ "${2:-}" == "@compozy/extension-sdk" ]]; then
  query_count=0
  if [[ -f "${FAKE_NPM_SDK_COUNTER}" ]]; then
    query_count="$(<"${FAKE_NPM_SDK_COUNTER}")"
  fi
  query_count=$((query_count + 1))
  printf '%s\n' "${query_count}" >"${FAKE_NPM_SDK_COUNTER}"
  if ((query_count == 1)); then
    printf '{"latest":"0.3.0-beta.3","beta":"0.3.0-beta.5"}\n'
    exit 0
  fi
fi
printf '{"latest":"0.2.15","beta":"0.3.0-beta.6"}\n'
`
		if err := os.WriteFile(filepath.Join(binDir, "npm"), []byte(npmStub), 0o755); err != nil {
			t.Fatalf("os.WriteFile(npm stub) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(binDir, "sleep"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("os.WriteFile(sleep stub) error = %v", err)
		}

		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "release-npm-readiness.sh"),
			"0.3.0-beta.6",
			"beta",
			"@compozy/cli",
			"@compozy/extension-sdk",
		)
		cmd.Env = append(
			os.Environ(),
			"FAKE_NPM_SDK_COUNTER="+counterPath,
			"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("release-npm-readiness.sh error = %v, output = %s", err, output)
		}
		if !strings.Contains(string(output), "npm release policy converged after 2 attempts") {
			t.Fatalf("release-npm-readiness.sh output = %s, want convergence evidence", output)
		}
		counter, err := os.ReadFile(counterPath)
		if err != nil {
			t.Fatalf("os.ReadFile(extension SDK query counter) error = %v", err)
		}
		if got, want := string(counter), "2\n"; got != want {
			t.Fatalf("extension SDK query count = %q, want %q", got, want)
		}
	})

	t.Run("Should reject a beta release that moves the latest tag without retrying", func(t *testing.T) {
		t.Parallel()

		root := findRepoRootForReleaseConfigTest(t)
		binDir := t.TempDir()
		sleepMarker := filepath.Join(binDir, "sleep-called")
		npmStub := `#!/bin/sh
set -eu
printf '{"latest":"0.3.0-beta.6","beta":"0.3.0-beta.6"}\n'
`
		if err := os.WriteFile(filepath.Join(binDir, "npm"), []byte(npmStub), 0o755); err != nil {
			t.Fatalf("os.WriteFile(npm stub) error = %v", err)
		}
		sleepStub := "#!/bin/sh\nset -eu\nprintf 'called\\n' >\"${FAKE_SLEEP_MARKER}\"\n"
		if err := os.WriteFile(filepath.Join(binDir, "sleep"), []byte(sleepStub), 0o755); err != nil {
			t.Fatalf("os.WriteFile(sleep stub) error = %v", err)
		}

		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "release-npm-readiness.sh"),
			"0.3.0-beta.6",
			"beta",
			"@compozy/extension-sdk",
		)
		cmd.Env = append(
			os.Environ(),
			"FAKE_SLEEP_MARKER="+sleepMarker,
			"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		)
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("release-npm-readiness.sh error = nil, output = %s", output)
		}
		if !strings.Contains(string(output), "latest moved during a beta release") {
			t.Fatalf("release-npm-readiness.sh output = %s, want latest-tag policy failure", output)
		}
		sleepCalls, readErr := os.ReadFile(sleepMarker)
		if readErr == nil {
			t.Fatalf("release-npm-readiness.sh retried a terminal policy violation: %s", sleepCalls)
		}
		if !os.IsNotExist(readErr) {
			t.Fatalf("os.ReadFile(sleep marker) error = %v", readErr)
		}
	})

	t.Run("Should stop with the last observed tags when readiness times out", func(t *testing.T) {
		t.Parallel()

		root := findRepoRootForReleaseConfigTest(t)
		binDir := t.TempDir()
		npmStub := `#!/bin/sh
set -eu
printf '{"latest":"0.3.0-beta.3","beta":"0.3.0-beta.5"}\n'
`
		if err := os.WriteFile(filepath.Join(binDir, "npm"), []byte(npmStub), 0o755); err != nil {
			t.Fatalf("os.WriteFile(npm stub) error = %v", err)
		}

		cmd := exec.CommandContext(
			t.Context(),
			"bash",
			filepath.Join(root, "scripts", "release-npm-readiness.sh"),
			"0.3.0-beta.6",
			"beta",
			"@compozy/extension-sdk",
		)
		cmd.Env = append(
			os.Environ(),
			"NPM_RELEASE_READINESS_TIMEOUT_SECONDS=1",
			"NPM_RELEASE_READINESS_INTERVAL_SECONDS=1",
			"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		)
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("release-npm-readiness.sh error = nil, output = %s", output)
		}
		for _, want := range []string{
			"npm release policy did not converge within 1s",
			"@compozy/extension-sdk beta points to 0.3.0-beta.5, want 0.3.0-beta.6",
		} {
			if !strings.Contains(string(output), want) {
				t.Fatalf("release-npm-readiness.sh output = %s, want %q", output, want)
			}
		}
	})
}

func TestChangelogConfigPreservesCompozyOSHardCut(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	cliffConfig := readTextFile(t, root, "cliff.toml")
	firstMessageParser := strings.Index(cliffConfig, `{ message = "^feat"`)
	if firstMessageParser == -1 {
		t.Fatal(`cliff.toml missing the first conventional commit parser`)
	}

	legacyCommits := []struct {
		name string
		sha  string
	}{
		{
			name: "Should exclude the leaked test daemon fix from CompozyOS releases",
			sha:  "00dba6a1c32bc4a032b177961919578b62d62b1f",
		},
		{
			name: "Should exclude the legacy task archive fix from CompozyOS releases",
			sha:  "c202311c8430fc0d4a7442e2dc715cabfbdc68a1",
		},
		{
			name: "Should exclude the inherited runtime model fix from CompozyOS releases",
			sha:  "f2f2cf8c2f4b526f726878fcbb732de5ce955ee9",
		},
	}
	for _, legacyCommit := range legacyCommits {
		t.Run(legacyCommit.name, func(t *testing.T) {
			t.Parallel()

			parser := `{ sha = "` + legacyCommit.sha + `", skip = true }`
			parserIndex := strings.Index(cliffConfig, parser)
			if parserIndex == -1 {
				t.Fatalf("cliff.toml missing hard-cut parser %q", parser)
			}
			if parserIndex > firstMessageParser {
				t.Fatalf("hard-cut parser %q must precede conventional message parsers", parser)
			}
		})
	}

	t.Run("Should exclude internal commit types from every release surface", func(t *testing.T) {
		t.Parallel()

		for _, commitType := range []string{"docs", "build", "ci"} {
			parser := `{ message = "^` + commitType + `(?:\\([^)]*\\))?!?:", skip = true }`
			parserIndex := strings.Index(cliffConfig, parser)
			if parserIndex == -1 {
				t.Fatalf("cliff.toml missing internal commit parser %q", parser)
			}
			if parserIndex > firstMessageParser {
				t.Fatalf("internal commit parser %q must precede conventional message parsers", parser)
			}
		}
		assertContainsText(t, "cliff.toml", cliffConfig, "protect_breaking_commits = false")
	})

	t.Run("Should retain the CompozyOS migration commit", func(t *testing.T) {
		t.Parallel()

		migrationParser := `{ sha = "8eeb8a3813fcc268c7a0d241e5e3f8c7b8c6c1b6", skip = true }`
		assertNotContainsText(t, "cliff.toml", cliffConfig, migrationParser)
	})
}

func findRepoRootForReleaseConfigTest(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".goreleaser.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root with .goreleaser.yml not found")
		}
		dir = parent
	}
}

func newReleasePreflightFixture(t *testing.T) (string, string) {
	t.Helper()

	repo := t.TempDir()
	root := findRepoRootForReleaseConfigTest(t)
	installTemplate := "#!/bin/sh\n" +
		"VERSION=\"${COMPOZY_VERSION:-__COMPOZY_PINNED_VERSION__}\"\n" +
		"COSIGN_VERSION=\"v3.1.3\"\n" +
		"# checksums.txt.sigstore.json\n" +
		"# refs/heads/main\n"
	goreleaserConfig := "before:\n" +
		"  hooks:\n" +
		"    - bash scripts/render-install-script.sh {{ .Tag }} .artifacts/install.sh\n" +
		"release:\n" +
		"  extra_files:\n" +
		"    - glob: ./.artifacts/install.sh\n" +
		"signs:\n" +
		"  - args:\n" +
		"      - \"--bundle=${signature}\"\n" +
		"npms:\n" +
		"  - name: \"@compozy/cli\"\n"
	files := map[string]string{
		"RELEASE_BODY.md":                    "# Release body\n",
		"RELEASE_NOTES.md":                   "# Release notes\n",
		".goreleaser.release-header.md.tmpl": "# Header\n",
		".goreleaser.release-footer.md.tmpl": "# Footer\n",
		"scripts/install.template.sh":        installTemplate,
		// The preflight executes the real renderer against the fixture template.
		"scripts/render-install-script.sh": readTextFile(t, root, filepath.Join("scripts", "render-install-script.sh")),
		".goreleaser.yml":                  goreleaserConfig,
	}
	for path, contents := range files {
		target := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%s) error = %v", path, err)
		}
		if err := os.WriteFile(target, []byte(contents), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%s) error = %v", path, err)
		}
	}

	pathDir := filepath.Join(repo, "test-bin")
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(test-bin) error = %v", err)
	}
	goStub := "#!/bin/sh\nset -eu\n" +
		`[ "$*" = "run github.com/magefile/mage@v1.17.2 releaseInstallCheck" ]` + "\n"
	if err := os.WriteFile(filepath.Join(pathDir, "go"), []byte(goStub), 0o755); err != nil {
		t.Fatalf("os.WriteFile(go stub) error = %v", err)
	}

	runReleasePreflightFixtureCommand(t, repo, "git", "init", "--initial-branch=main")
	runReleasePreflightFixtureCommand(t, repo, "git", "config", "user.name", "Compozy Release Preflight")
	runReleasePreflightFixtureCommand(t, repo, "git", "config", "user.email", "release-preflight@compozy.com")
	runReleasePreflightFixtureCommand(t, repo, "git", "add", ".")
	runReleasePreflightFixtureCommand(
		t,
		repo,
		"git",
		"-c",
		"commit.gpgsign=false",
		"commit",
		"-m",
		"test: seed release preflight",
	)
	return repo, pathDir
}

func runReleasePreflightFixtureCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s error = %v, output = %s", name, strings.Join(args, " "), err, output)
	}
}

func readTextFile(t *testing.T, root string, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", rel, err)
	}
	return string(data)
}

func readYAMLMap(t *testing.T, root string, rel string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", rel, err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal(%s) error = %v", rel, err)
	}
	return cfg
}

func mapAt(t *testing.T, src map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := src[key]
	if !ok {
		t.Fatalf("%s missing", key)
	}
	return asMap(t, value, key)
}

func asMap(t *testing.T, value any, label string) map[string]any {
	t.Helper()

	item, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s type = %T, want map[string]any", label, value)
	}
	return item
}

func sliceAt(t *testing.T, src map[string]any, key string) []any {
	t.Helper()

	value, ok := src[key]
	if !ok {
		t.Fatalf("%s missing", key)
	}
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("%s type = %T, want []any", key, value)
	}
	return items
}

func stringAt(t *testing.T, src map[string]any, key string) string {
	t.Helper()

	value, ok := src[key]
	if !ok {
		t.Fatalf("%s missing", key)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("%s type = %T, want string", key, value)
	}
	return text
}

func stringSliceContains(values []any, want string) bool {
	for _, value := range values {
		if text, ok := value.(string); ok && text == want {
			return true
		}
	}
	return false
}

func stringsFromSlice(t *testing.T, values []any, label string) []string {
	t.Helper()

	items := make([]string, 0, len(values))
	for index, value := range values {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("%s[%d] type = %T, want string", label, index, value)
		}
		items = append(items, text)
	}
	return items
}

func stringListContains(values []string, want string) bool {
	return slices.Contains(values, want)
}

func firstMapAt(t *testing.T, src map[string]any, key string) map[string]any {
	t.Helper()

	items := sliceAt(t, src, key)
	if len(items) == 0 {
		t.Fatalf("%s is empty", key)
	}
	return asMap(t, items[0], key+"[0]")
}

func shellAssignment(t *testing.T, script string, key string) string {
	t.Helper()

	prefix := key + "=\""
	for line := range strings.SplitSeq(script, "\n") {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			trimmed, ok := strings.CutSuffix(value, "\"")
			if !ok {
				t.Fatalf("%s assignment = %q, want quoted shell string", key, line)
			}
			return trimmed
		}
	}
	t.Fatalf("install.sh missing %s assignment", key)
	return ""
}

func assertEqualString(t *testing.T, label string, got string, want string) {
	t.Helper()

	if got != want {
		t.Fatalf("%s = %q, want %q", label, got, want)
	}
}

func assertContainsText(t *testing.T, label string, text string, want string) {
	t.Helper()

	if !strings.Contains(text, want) {
		t.Fatalf("%s missing %q", label, want)
	}
}

func assertNotContainsText(t *testing.T, label string, text string, unwanted string) {
	t.Helper()

	if strings.Contains(text, unwanted) {
		t.Fatalf("%s contains %q", label, unwanted)
	}
}

func assertUniqueSBOMIDs(t *testing.T, sboms []any) {
	t.Helper()

	seen := make(map[string]struct{}, len(sboms))
	for index, entry := range sboms {
		sbom := asMap(t, entry, "sboms[]")
		id := stringAt(t, sbom, "id")
		if _, ok := seen[id]; ok {
			t.Fatalf("sboms[%d].id = %q, want unique SBOM IDs", index, id)
		}
		seen[id] = struct{}{}
	}
}

func assertSBOMArtifact(t *testing.T, sboms []any, id string, artifact string) {
	t.Helper()

	for _, entry := range sboms {
		sbom := asMap(t, entry, "sboms[]")
		if stringAt(t, sbom, "id") == id && stringAt(t, sbom, "artifacts") == artifact {
			return
		}
	}
	t.Fatalf("sboms = %#v, want id %q with artifacts %q", sboms, id, artifact)
}

func assertReleaseExtraFile(t *testing.T, extraFiles []any, glob string, nameTemplate string) {
	t.Helper()

	for _, entry := range extraFiles {
		extraFile := asMap(t, entry, "release.extra_files[]")
		if stringAt(t, extraFile, "glob") == glob &&
			stringAt(t, extraFile, "name_template") == nameTemplate {
			return
		}
	}
	t.Fatalf("release.extra_files = %#v, want glob %q with name_template %q", extraFiles, glob, nameTemplate)
}
