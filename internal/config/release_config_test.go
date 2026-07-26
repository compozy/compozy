package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	})

	t.Run("Should publish stable archives and curl installer asset", func(t *testing.T) {
		t.Parallel()

		archives := sliceAt(t, cfg, "archives")
		if len(archives) != 1 {
			t.Fatalf("archives len = %d, want 1", len(archives))
		}
		archive := asMap(t, archives[0], "archives[0]")
		if got, want := stringAt(t, archive, "id"), "agh-archive"; got != want {
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
		if got, want := stringAt(t, github, "name"), "agh"; got != want {
			t.Fatalf("release.github.name = %q, want %q", got, want)
		}

		extraFiles := sliceAt(t, release, "extra_files")
		assertReleaseExtraFile(t, extraFiles, "./packages/site/public/install.sh", "install.sh")
	})

	t.Run("Should configure Homebrew formula and Linux package targets", func(t *testing.T) {
		if _, ok := cfg["homebrew_casks"]; ok {
			t.Fatal("homebrew_casks configured, want Homebrew formula via brews")
		}
		brews := sliceAt(t, cfg, "brews")
		if len(brews) != 1 {
			t.Fatalf("brews len = %d, want 1", len(brews))
		}
		formula := asMap(t, brews[0], "brews[0]")
		if got, want := stringAt(t, formula, "name"), "agh"; got != want {
			t.Fatalf("brews[0].name = %q, want %q", got, want)
		}
		if !stringSliceContains(sliceAt(t, formula, "ids"), "agh-archive") {
			t.Fatalf("brews[0].ids = %#v, want agh-archive", formula["ids"])
		}
		if got, want := stringAt(t, formula, "directory"), "Formula"; got != want {
			t.Fatalf("brews[0].directory = %q, want %q", got, want)
		}
		repository := mapAt(t, formula, "repository")
		if got, want := stringAt(t, repository, "owner"), "compozy"; got != want {
			t.Fatalf("brews[0].repository.owner = %q, want %q", got, want)
		}
		if got, want := stringAt(t, repository, "name"), "homebrew-compozy"; got != want {
			t.Fatalf("brews[0].repository.name = %q, want %q", got, want)
		}
		if _, ok := formula["binaries"]; ok {
			t.Fatal("brews[0].binaries configured, want formula archive install semantics")
		}

		nfpms := sliceAt(t, cfg, "nfpms")
		if len(nfpms) != 1 {
			t.Fatalf("nfpms len = %d, want 1", len(nfpms))
		}
		nfpm := asMap(t, nfpms[0], "nfpms[0]")
		if got, want := stringAt(t, nfpm, "id"), "agh-linux-packages"; got != want {
			t.Fatalf("nfpms[0].id = %q, want %q", got, want)
		}
		if !stringSliceContains(sliceAt(t, nfpm, "ids"), "agh") {
			t.Fatalf("nfpms[0].ids = %#v, want agh build id", nfpm["ids"])
		}
		formats := sliceAt(t, nfpm, "formats")
		for _, want := range []string{"deb", "rpm"} {
			if !stringSliceContains(formats, want) {
				t.Fatalf("nfpms[0].formats = %#v, want %s", formats, want)
			}
		}
	})

	t.Run("Should configure public NPM package target", func(t *testing.T) {
		npms := sliceAt(t, cfg, "npms")
		if len(npms) != 1 {
			t.Fatalf("npms len = %d, want 1", len(npms))
		}
		npm := asMap(t, npms[0], "npms[0]")
		assertEqualString(t, "npms[0].name", stringAt(t, npm, "name"), "@compozy/agh")
		if !stringSliceContains(sliceAt(t, npm, "ids"), "agh-archive") {
			t.Fatalf("npms[0].ids = %#v, want agh-archive", npm["ids"])
		}
		assertEqualString(t, "npms[0].access", stringAt(t, npm, "access"), "public")
		assertEqualString(t, "npms[0].format", stringAt(t, npm, "format"), "tar.gz")
		assertEqualString(
			t,
			"npms[0].repository",
			stringAt(t, npm, "repository"),
			"git+https://github.com/compozy/agh.git",
		)
		assertEqualString(t, "npms[0].homepage", stringAt(t, npm, "homepage"), "https://agh.network")
	})
}

func TestGoReleaserArchivesStayAlignedWithPublicInstaller(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	goreleaser := readYAMLMap(t, root, ".goreleaser.yml")
	installScript := readTextFile(t, root, filepath.Join("packages", "site", "public", "install.sh"))

	projectName := stringAt(t, goreleaser, "project_name")
	assertEqualString(t, "goreleaser project_name", projectName, "agh")
	build := firstMapAt(t, goreleaser, "builds")
	buildID := stringAt(t, build, "id")
	assertEqualString(t, "build id", buildID, "agh")
	assertEqualString(t, "build binary", stringAt(t, build, "binary"), "agh")
	assertEqualString(t, "build main", stringAt(t, build, "main"), "./cmd/agh")
	ldflags := strings.Join(stringsFromSlice(t, sliceAt(t, build, "ldflags"), "builds[0].ldflags"), "\n")
	assertContainsText(
		t,
		"GoReleaser ldflags",
		ldflags,
		"github.com/compozy/agh/internal/version.Version",
	)
	assertNotContainsText(t, "GoReleaser ldflags", ldflags, "github.com/pedronauck/agh")

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
	if !strings.Contains(installScript, `ARCHIVE_NAME="agh_${OS}_${ARCH}.tar.gz"`) {
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
	if !strings.Contains(installScript, `TARGET="${INSTALL_DIR}/agh"`) {
		t.Fatalf("install.sh must install the same binary name GoReleaser builds")
	}
}

func TestReleaseTemplatesStayAlignedWithPublicInstallMethods(t *testing.T) {
	t.Parallel()

	root := findRepoRootForReleaseConfigTest(t)
	header := readTextFile(t, root, ".goreleaser.release-header.md.tmpl")
	footer := readTextFile(t, root, ".goreleaser.release-footer.md.tmpl")

	t.Run("Should keep release header aligned with public install methods", func(t *testing.T) {
		t.Parallel()

		for _, snippet := range []string{
			"brew install compozy/compozy/agh",
			"npm install -g @compozy/agh",
			"go install github.com/compozy/agh@{{ .Tag }}",
			"curl -fsSL https://agh.network/install.sh | sh",
			"Verified Binary Installer",
		} {
			assertContainsText(t, "GoReleaser release header", header, snippet)
		}
		assertNotContainsText(t, "GoReleaser release header", header, "github.com/pedronauck/agh")
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
