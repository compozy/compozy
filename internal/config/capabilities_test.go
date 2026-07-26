package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadAgentCapabilitiesFromSingleFileTOMLNormalizesEntries(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	writeFile(t, filepath.Join(agentDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "build-site"
summary = " Build the landing page. "
outcome = " A finished landing page. "
version = " 1.0.0 "
context_needed = [" repo ", "", " brand brief "]
execution_outline = [" Inspect ", " Build "]
constraints = [" No mocks "]
examples = [" marketing page "]
requirements = [" workspace-write ", "review-guidelines"]

[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
artifacts_expected = [" Annotated doc "]
`)

	catalog, err := LoadAgentCapabilities(agentDir)
	if err != nil {
		t.Fatalf("LoadAgentCapabilities() error = %v", err)
	}
	if catalog == nil {
		t.Fatal("LoadAgentCapabilities() = nil, want catalog")
	}
	if got, want := len(catalog.Capabilities), 2; got != want {
		t.Fatalf("len(Capabilities) = %d, want %d", got, want)
	}

	first := catalog.Capabilities[0]
	if got, want := first.Summary, "Build the landing page."; got != want {
		t.Fatalf("Capabilities[0].Summary = %q, want %q", got, want)
	}
	if got, want := first.Version, "1.0.0"; got != want {
		t.Fatalf("Capabilities[0].Version = %q, want %q", got, want)
	}
	if got, want := strings.Join(first.ContextNeeded, ","), "repo,brand brief"; got != want {
		t.Fatalf("Capabilities[0].ContextNeeded = %#v, want normalized list", first.ContextNeeded)
	}
	if got, want := strings.Join(first.ExecutionOutline, ","), "Inspect,Build"; got != want {
		t.Fatalf("Capabilities[0].ExecutionOutline = %#v, want normalized list", first.ExecutionOutline)
	}
	if got, want := strings.Join(first.Constraints, ","), "No mocks"; got != want {
		t.Fatalf("Capabilities[0].Constraints = %#v, want normalized list", first.Constraints)
	}
	if got, want := strings.Join(first.Examples, ","), "marketing page"; got != want {
		t.Fatalf("Capabilities[0].Examples = %#v, want normalized list", first.Examples)
	}
	if got, want := strings.Join(first.Requirements, ","), "review-guidelines,workspace-write"; got != want {
		t.Fatalf("Capabilities[0].Requirements = %#v, want normalized sorted requirements", first.Requirements)
	}
	if !strings.HasPrefix(first.Digest, "sha256:") {
		t.Fatalf("Capabilities[0].Digest = %q, want sha256 digest", first.Digest)
	}

	second := catalog.Capabilities[1]
	if got, want := strings.Join(second.ArtifactsExpected, ","), "Annotated doc"; got != want {
		t.Fatalf("Capabilities[1].ArtifactsExpected = %#v, want normalized list", second.ArtifactsExpected)
	}
	if !strings.HasPrefix(second.Digest, "sha256:") {
		t.Fatalf("Capabilities[1].Digest = %q, want sha256 digest", second.Digest)
	}
}

func TestLoadAgentCapabilitiesFromSingleFileJSONStrictness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		payload  string
		wantErr  string
		validate func(t *testing.T, catalog *CapabilityCatalog)
	}{
		{
			name: "ShouldAcceptValidSchema",
			payload: `{
  "capabilities": [
    {
      "id": "review-copy",
      "summary": "Review conversion copy.",
      "outcome": "A prioritized copy review.",
      "context_needed": ["brief", "analytics"],
      "execution_outline": ["inspect", "rewrite"]
    }
  ]
}`,
			validate: func(t *testing.T, catalog *CapabilityCatalog) {
				t.Helper()

				if catalog == nil {
					t.Fatal("catalog = nil, want parsed catalog")
				}
				if got, want := len(catalog.Capabilities), 1; got != want {
					t.Fatalf("len(Capabilities) = %d, want %d", got, want)
				}
				if got, want := strings.Join(
					catalog.Capabilities[0].ExecutionOutline,
					",",
				), "inspect,rewrite"; got != want {
					t.Fatalf("ExecutionOutline = %#v, want preserved outline", catalog.Capabilities[0].ExecutionOutline)
				}
			},
		},
		{
			name: "ShouldRejectUnknownFields",
			payload: `{
  "capabilities": [
    {
      "id": "review-copy",
      "summary": "Review conversion copy.",
      "outcome": "A prioritized copy review.",
      "unknown": true
    }
  ]
}`,
			wantErr: `unknown field "unknown"`,
		},
		{
			name: "ShouldRejectAuthoredDigestField",
			payload: `{
  "capabilities": [
    {
      "id": "review-copy",
      "summary": "Review conversion copy.",
      "outcome": "A prioritized copy review.",
      "digest": "sha256:authored"
    }
  ]
}`,
			wantErr: `unknown field "digest"`,
		},
		{
			name:    "ShouldRejectTrailingJSON",
			payload: `{"capabilities":[{"id":"review-copy","summary":"Review conversion copy.","outcome":"A prioritized copy review."}]}{"extra":true}`,
			wantErr: "unexpected trailing JSON value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agentDir := t.TempDir()
			writeFile(t, filepath.Join(agentDir, capabilityCatalogJSONName), tt.payload)

			catalog, err := LoadAgentCapabilities(agentDir)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("LoadAgentCapabilities() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadAgentCapabilities() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadAgentCapabilities() error = %v", err)
			}
			tt.validate(t, catalog)
		})
	}
}

func TestLoadAgentCapabilitiesDirectoryModeLoadsSelectedRegularFilesOnly(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	capabilitiesDir := filepath.Join(agentDir, capabilityCatalogDirName)

	writeFile(t, filepath.Join(capabilitiesDir, "build-site.toml"), `
id = "build-site"
summary = "Build the landing page."
outcome = "A finished landing page."
`)
	writeFile(t, filepath.Join(capabilitiesDir, "review-copy.toml"), `
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
`)
	writeFile(t, filepath.Join(capabilitiesDir, ".hidden.toml"), `
id = "hidden"
summary = "Should be ignored."
outcome = "Hidden."
`)
	writeFile(t, filepath.Join(capabilitiesDir, "notes.txt"), "ignored")
	writeFile(t, filepath.Join(capabilitiesDir, "nested", "ignored.toml"), `
id = "ignored"
summary = "Ignored because nested."
outcome = "Nested."
`)

	catalog, err := LoadAgentCapabilities(agentDir)
	if err != nil {
		t.Fatalf("LoadAgentCapabilities() error = %v", err)
	}
	if catalog == nil {
		t.Fatal("LoadAgentCapabilities() = nil, want catalog")
	}
	if got, want := len(catalog.Capabilities), 2; got != want {
		t.Fatalf("len(Capabilities) = %d, want %d", got, want)
	}
	if got, want := catalog.Capabilities[0].ID, "build-site"; got != want {
		t.Fatalf("Capabilities[0].ID = %q, want %q", got, want)
	}
	if got, want := catalog.Capabilities[1].ID, "review-copy"; got != want {
		t.Fatalf("Capabilities[1].ID = %q, want %q", got, want)
	}
}

func TestLoadAgentCapabilitiesRejectsMixedFileAndDirectoryModes(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	writeFile(t, filepath.Join(agentDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "build-site"
summary = "Build the landing page."
outcome = "A finished landing page."
`)
	writeFile(t, filepath.Join(agentDir, capabilityCatalogDirName, "review-copy.toml"), `
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
`)

	_, err := LoadAgentCapabilities(agentDir)
	if err == nil {
		t.Fatal("LoadAgentCapabilities() error = nil, want mixed-mode failure")
	}
	if !strings.Contains(err.Error(), "mixed capability catalog layouts") {
		t.Fatalf("LoadAgentCapabilities() error = %q, want mixed layout context", err.Error())
	}
	if !strings.Contains(err.Error(), filepath.Join(agentDir, capabilityCatalogTOMLName)) {
		t.Fatalf("LoadAgentCapabilities() error = %q, want file path context", err.Error())
	}
	if !strings.Contains(err.Error(), filepath.Join(agentDir, capabilityCatalogDirName)) {
		t.Fatalf("LoadAgentCapabilities() error = %q, want directory path context", err.Error())
	}
}

func TestLoadAgentCapabilitiesRejectsMultipleSingleFiles(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	writeFile(t, filepath.Join(agentDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "build-site"
summary = "Build the landing page."
outcome = "A finished landing page."
`)
	writeFile(t, filepath.Join(agentDir, capabilityCatalogJSONName), `{"capabilities":[]}`)

	_, err := LoadAgentCapabilities(agentDir)
	if err == nil {
		t.Fatal("LoadAgentCapabilities() error = nil, want multiple single-file failure")
	}
	if !strings.Contains(err.Error(), "multiple capability catalog files") {
		t.Fatalf("LoadAgentCapabilities() error = %q, want multiple file context", err.Error())
	}
}

func TestLoadAgentCapabilitiesRejectsReservedPathTypeMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupPath  func(t *testing.T, agentDir string) string
		wantErr    string
		wantTarget string
	}{
		{
			name: "ShouldRejectDirectoryAtTOMLCatalogPath",
			setupPath: func(t *testing.T, agentDir string) string {
				t.Helper()

				path := filepath.Join(agentDir, capabilityCatalogTOMLName)
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("os.Mkdir(%q) error = %v", path, err)
				}
				return path
			},
			wantErr:    "must be a file",
			wantTarget: capabilityCatalogTOMLName,
		},
		{
			name: "ShouldRejectDirectoryAtJSONCatalogPath",
			setupPath: func(t *testing.T, agentDir string) string {
				t.Helper()

				path := filepath.Join(agentDir, capabilityCatalogJSONName)
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("os.Mkdir(%q) error = %v", path, err)
				}
				return path
			},
			wantErr:    "must be a file",
			wantTarget: capabilityCatalogJSONName,
		},
		{
			name: "ShouldRejectFileAtCatalogDirectoryPath",
			setupPath: func(t *testing.T, agentDir string) string {
				t.Helper()

				path := filepath.Join(agentDir, capabilityCatalogDirName)
				writeFile(t, path, "not-a-directory")
				return path
			},
			wantErr:    "must be a directory",
			wantTarget: capabilityCatalogDirName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agentDir := t.TempDir()
			targetPath := tt.setupPath(t, agentDir)

			catalog, err := LoadAgentCapabilities(agentDir)
			if err == nil {
				t.Fatal("LoadAgentCapabilities() error = nil, want wrong-type path failure")
			}
			if catalog != nil {
				t.Fatalf("LoadAgentCapabilities() catalog = %#v, want nil on error", catalog)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadAgentCapabilities() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
			if !strings.Contains(err.Error(), targetPath) {
				t.Fatalf("LoadAgentCapabilities() error = %q, want path %q", err.Error(), targetPath)
			}
			if !strings.Contains(err.Error(), tt.wantTarget) {
				t.Fatalf("LoadAgentCapabilities() error = %q, want reserved target %q", err.Error(), tt.wantTarget)
			}
		})
	}
}

func TestLoadAgentCapabilitiesRejectsMixedDirectoryFormats(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	writeFile(t, filepath.Join(agentDir, capabilityCatalogDirName, "build-site.toml"), `
id = "build-site"
summary = "Build the landing page."
outcome = "A finished landing page."
`)
	writeFile(t, filepath.Join(agentDir, capabilityCatalogDirName, "review-copy.json"), `{
  "id": "review-copy",
  "summary": "Review conversion copy.",
  "outcome": "A prioritized copy review."
}`)

	_, err := LoadAgentCapabilities(agentDir)
	if err == nil {
		t.Fatal("LoadAgentCapabilities() error = nil, want mixed-format failure")
	}
	if !strings.Contains(err.Error(), "mixed capability file formats") {
		t.Fatalf("LoadAgentCapabilities() error = %q, want mixed format context", err.Error())
	}
}

func TestLoadAgentCapabilitiesRejectsDuplicateNormalizedIDsAcrossDirectoryEntries(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	writeFile(t, filepath.Join(agentDir, capabilityCatalogDirName, "build-site.toml"), `
id = "build-site"
summary = "Build the landing page."
outcome = "A finished landing page."
`)
	writeFile(t, filepath.Join(agentDir, capabilityCatalogDirName, " build-site.toml"), `
id = " build-site "
summary = "Build the landing page again."
outcome = "A second finished landing page."
`)

	_, err := LoadAgentCapabilities(agentDir)
	if err == nil {
		t.Fatal("LoadAgentCapabilities() error = nil, want duplicate ID failure")
	}
	if !strings.Contains(err.Error(), `duplicate capability id "build-site" after normalization`) {
		t.Fatalf("LoadAgentCapabilities() error = %q, want duplicate normalized ID context", err.Error())
	}
}

func TestLoadAgentCapabilitiesRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		payload  string
		wantErr  string
		filename string
	}{
		{
			name: "ShouldRejectMissingID",
			payload: `
[[capabilities]]
summary = "Review conversion copy."
outcome = "A prioritized copy review."
`,
			filename: capabilityCatalogTOMLName,
			wantErr:  ".id is required",
		},
		{
			name: "ShouldRejectMissingSummary",
			payload: `
[[capabilities]]
id = "review-copy"
outcome = "A prioritized copy review."
`,
			filename: capabilityCatalogTOMLName,
			wantErr:  ".summary is required",
		},
		{
			name:     "ShouldRejectMissingOutcome",
			payload:  `{"capabilities":[{"id":"review-copy","summary":"Review conversion copy."}]}`,
			filename: capabilityCatalogJSONName,
			wantErr:  ".outcome is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agentDir := t.TempDir()
			writeFile(t, filepath.Join(agentDir, tt.filename), tt.payload)

			_, err := LoadAgentCapabilities(agentDir)
			if err == nil {
				t.Fatal("LoadAgentCapabilities() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadAgentCapabilities() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadAgentCapabilitiesEquivalentTOMLAndJSONProduceSameRuntimeShapeAndDigest(t *testing.T) {
	t.Parallel()

	tomlAgentDir := t.TempDir()
	jsonAgentDir := t.TempDir()

	writeFile(t, filepath.Join(tomlAgentDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "review-copy"
summary = " Review conversion copy. "
outcome = " A prioritized copy review. "
version = " 1.2.0 "
context_needed = [" brief ", " analytics "]
execution_outline = [" inspect ", " rewrite "]
constraints = [" keep evidence "]
examples = [" homepage review "]
requirements = [" workspace-write ", "review-guidelines"]
`)
	writeFile(t, filepath.Join(jsonAgentDir, capabilityCatalogJSONName), `{
  "capabilities": [
    {
      "id": "review-copy",
      "summary": "Review conversion copy.",
      "outcome": "A prioritized copy review.",
      "version": "1.2.0",
      "context_needed": ["brief", "analytics"],
      "execution_outline": ["inspect", "rewrite"],
      "constraints": ["keep evidence"],
      "examples": ["homepage review"],
      "requirements": ["review-guidelines", "workspace-write"]
    }
  ]
}`)

	tomlCatalog, err := LoadAgentCapabilities(tomlAgentDir)
	if err != nil {
		t.Fatalf("LoadAgentCapabilities(TOML) error = %v", err)
	}
	jsonCatalog, err := LoadAgentCapabilities(jsonAgentDir)
	if err != nil {
		t.Fatalf("LoadAgentCapabilities(JSON) error = %v", err)
	}
	if !reflect.DeepEqual(tomlCatalog, jsonCatalog) {
		t.Fatalf("normalized catalogs differ:\nTOML: %#v\nJSON: %#v", tomlCatalog, jsonCatalog)
	}
	if got := tomlCatalog.Capabilities[0].Digest; got == "" {
		t.Fatal("TOML digest = empty, want computed digest")
	}
	if got, want := tomlCatalog.Capabilities[0].Digest, jsonCatalog.Capabilities[0].Digest; got != want {
		t.Fatalf("digest mismatch: TOML=%q JSON=%q", got, want)
	}
}

func TestLoadAgentCapabilitiesDigestChangesWhenMeaningfulFieldsChange(t *testing.T) {
	t.Parallel()

	loadDigest := func(t *testing.T, payload string) string {
		t.Helper()

		agentDir := t.TempDir()
		writeFile(t, filepath.Join(agentDir, capabilityCatalogTOMLName), payload)

		catalog, err := LoadAgentCapabilities(agentDir)
		if err != nil {
			t.Fatalf("LoadAgentCapabilities() error = %v", err)
		}
		if catalog == nil || len(catalog.Capabilities) != 1 {
			t.Fatalf("catalog = %#v, want single capability", catalog)
		}
		return catalog.Capabilities[0].Digest
	}

	baseDigest := loadDigest(t, `
[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
execution_outline = ["inspect", "rewrite"]
requirements = ["workspace-write", "review-guidelines"]
`)
	changedExecutionDigest := loadDigest(t, `
[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
execution_outline = ["inspect", "rewrite", "publish"]
requirements = ["workspace-write", "review-guidelines"]
`)
	changedRequirementsDigest := loadDigest(t, `
[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
execution_outline = ["inspect", "rewrite"]
requirements = ["workspace-write", "design-brief"]
`)

	if baseDigest == changedExecutionDigest {
		t.Fatalf("execution outline digest = %q, want different digest", changedExecutionDigest)
	}
	if baseDigest == changedRequirementsDigest {
		t.Fatalf("requirements digest = %q, want different digest", changedRequirementsDigest)
	}
}

func TestLoadAgentCapabilitiesRejectsInvalidVersionAndRequirements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name: "ShouldRejectBlankVersionWhenProvided",
			payload: `
[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
version = "   "
`,
			wantErr: ".version must not be blank when provided",
		},
		{
			name: "ShouldRejectBlankRequirementEntry",
			payload: `
[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
requirements = ["workspace-write", "   "]
`,
			wantErr: ".requirements[1] is required",
		},
		{
			name: "ShouldRejectDuplicateRequirementsAfterNormalization",
			payload: `
[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
requirements = ["workspace-write", " workspace-write "]
`,
			wantErr: `.requirements duplicate value "workspace-write" after normalization`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agentDir := t.TempDir()
			writeFile(t, filepath.Join(agentDir, capabilityCatalogTOMLName), tt.payload)

			_, err := LoadAgentCapabilities(agentDir)
			if err == nil {
				t.Fatal("LoadAgentCapabilities() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadAgentCapabilities() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadAgentCapabilitiesRejectsDirectoryBasenameMismatch(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	writeFile(t, filepath.Join(agentDir, capabilityCatalogDirName, "build-site.toml"), `
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
`)

	_, err := LoadAgentCapabilities(agentDir)
	if err == nil {
		t.Fatal("LoadAgentCapabilities() error = nil, want basename mismatch failure")
	}
	if !strings.Contains(err.Error(), `basename "build-site" must match id "review-copy"`) {
		t.Fatalf("LoadAgentCapabilities() error = %q, want basename mismatch context", err.Error())
	}
}

func TestLoadAgentCapabilitiesMissingCatalogIsOptional(t *testing.T) {
	t.Parallel()

	catalog, err := LoadAgentCapabilities(t.TempDir())
	if err != nil {
		t.Fatalf("LoadAgentCapabilities() error = %v, want nil", err)
	}
	if catalog != nil {
		t.Fatalf("LoadAgentCapabilities() = %#v, want nil for missing catalog", catalog)
	}
}
