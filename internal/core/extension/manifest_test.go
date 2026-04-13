package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/version"
)

func TestLoadManifestLoadsSupportedFormats(t *testing.T) {
	t.Helper()

	withVersion(t, "1.5.0")

	testCases := []struct {
		name         string
		fileName     string
		content      string
		wantName     string
		wantCommand  string
		wantTimeout  string
		wantPeriod   string
		wantPriority int
	}{
		{
			name:         "toml",
			fileName:     ManifestFileNameTOML,
			content:      validTOMLManifest,
			wantName:     "toml-ext",
			wantCommand:  "bin/toml-ext",
			wantTimeout:  "12s",
			wantPeriod:   "45s",
			wantPriority: 200,
		},
		{
			name:         "json",
			fileName:     ManifestFileNameJSON,
			content:      validJSONManifest,
			wantName:     "json-ext",
			wantCommand:  "bin/json-ext",
			wantTimeout:  "15s",
			wantPeriod:   "1m0s",
			wantPriority: DefaultHookPriority,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, tc.fileName), tc.content)

			manifest, err := LoadManifest(context.Background(), dir)
			if err != nil {
				t.Fatalf("LoadManifest() error = %v", err)
			}
			if manifest.Extension.Name != tc.wantName {
				t.Fatalf("Extension.Name = %q, want %q", manifest.Extension.Name, tc.wantName)
			}
			if manifest.Subprocess == nil {
				t.Fatal("Subprocess = nil, want populated config")
			}
			if manifest.Subprocess.Command != tc.wantCommand {
				t.Fatalf("Subprocess.Command = %q, want %q", manifest.Subprocess.Command, tc.wantCommand)
			}
			if got := manifest.Subprocess.ShutdownTimeout.String(); got != tc.wantTimeout {
				t.Fatalf("Subprocess.ShutdownTimeout = %q, want %q", got, tc.wantTimeout)
			}
			if got := manifest.Subprocess.HealthCheckPeriod.String(); got != tc.wantPeriod {
				t.Fatalf("Subprocess.HealthCheckPeriod = %q, want %q", got, tc.wantPeriod)
			}
			if len(manifest.Hooks) != 1 {
				t.Fatalf("len(Hooks) = %d, want 1", len(manifest.Hooks))
			}
			if manifest.Hooks[0].Priority != tc.wantPriority {
				t.Fatalf("Hooks[0].Priority = %d, want %d", manifest.Hooks[0].Priority, tc.wantPriority)
			}
			if len(manifest.Resources.Skills) != 1 || manifest.Resources.Skills[0] != "skills/*" {
				t.Fatalf("Resources.Skills = %#v, want [skills/*]", manifest.Resources.Skills)
			}
			if len(manifest.Providers.Model) != 1 || manifest.Providers.Model[0].Name == "" {
				t.Fatalf("Providers.Model = %#v, want populated model entry", manifest.Providers.Model)
			}
		})
	}
}

func TestLoadManifestPrefersTOMLAndWarns(t *testing.T) {
	withVersion(t, "1.5.0")

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ManifestFileNameTOML), validTOMLManifest)
	writeTestFile(t, filepath.Join(dir, ManifestFileNameJSON), validJSONManifest)

	logBuf := captureDefaultLogger(t)

	manifest, err := LoadManifest(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if manifest.Extension.Name != "toml-ext" {
		t.Fatalf("Extension.Name = %q, want %q", manifest.Extension.Name, "toml-ext")
	}

	records := decodeLogRecords(t, logBuf)
	if len(records) != 1 {
		t.Fatalf("len(log records) = %d, want 1", len(records))
	}
	if got := records[0]["msg"]; got != "extension.toml takes precedence over extension.json" {
		t.Fatalf("log message = %v, want precedence warning", got)
	}
	if got := records[0]["ignored_manifest_path"]; got != filepath.Join(dir, ManifestFileNameJSON) {
		t.Fatalf("ignored_manifest_path = %v, want %q", got, filepath.Join(dir, ManifestFileNameJSON))
	}
}

func TestLoadManifestMissingManifestReturnsStructuredError(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadManifest(context.Background(), dir)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want not found error")
	}

	var notFoundErr *ManifestNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("LoadManifest() error = %T, want *ManifestNotFoundError", err)
	}
	if notFoundErr.Dir != dir {
		t.Fatalf("ManifestNotFoundError.Dir = %q, want %q", notFoundErr.Dir, dir)
	}
	if len(notFoundErr.CandidatePaths) != 2 {
		t.Fatalf("len(CandidatePaths) = %d, want 2", len(notFoundErr.CandidatePaths))
	}
}

func TestValidateManifestRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		name       string
		current    string
		mutate     func(*Manifest)
		wantSubstr string
	}{
		{
			name:    "unknown capability",
			current: "1.5.0",
			mutate: func(manifest *Manifest) {
				manifest.Security.Capabilities = append(manifest.Security.Capabilities, Capability("bogus.capability"))
			},
			wantSubstr: `security.capabilities="bogus.capability": unknown capability`,
		},
		{
			name:    "unknown hook event",
			current: "1.5.0",
			mutate: func(manifest *Manifest) {
				manifest.Hooks[0].Event = HookName("prompt.not_real")
			},
			wantSubstr: `hooks[0].event="prompt.not_real": unknown hook event`,
		},
		{
			name:    "priority out of range",
			current: "1.5.0",
			mutate: func(manifest *Manifest) {
				manifest.Hooks[0].Priority = MaxHookPriority + 1
			},
			wantSubstr: `hooks[0].priority="1001": must be within [0, 1000]`,
		},
		{
			name:    "min version newer than current",
			current: "1.0.0",
			mutate: func(manifest *Manifest) {
				manifest.Extension.MinCompozyVersion = "2.0.0"
			},
			wantSubstr: `extension.min_compozy_version="2.0.0": requires Compozy 2.0.0 or newer (current 1.0.0)`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			withVersion(t, tc.current)

			manifest := validManifest()
			tc.mutate(manifest)

			err := ValidateManifest(context.Background(), manifest)
			if err == nil {
				t.Fatal("ValidateManifest() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("ValidateManifest() error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestLoadManifestRealisticFixture(t *testing.T) {
	withVersion(t, "1.5.0")

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ManifestFileNameTOML), realisticTOMLManifest)

	manifest, err := LoadManifest(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if manifest.Extension.Name != "fixture-ext" {
		t.Fatalf("Extension.Name = %q, want %q", manifest.Extension.Name, "fixture-ext")
	}
	if manifest.Subprocess == nil {
		t.Fatal("Subprocess = nil, want populated config")
	}
	if got := manifest.Subprocess.ShutdownTimeout.String(); got != "10s" {
		t.Fatalf("Subprocess.ShutdownTimeout = %q, want %q", got, "10s")
	}
	if len(manifest.Security.Capabilities) != 3 {
		t.Fatalf("len(Security.Capabilities) = %d, want 3", len(manifest.Security.Capabilities))
	}
	if len(manifest.Hooks) != 2 {
		t.Fatalf("len(Hooks) = %d, want 2", len(manifest.Hooks))
	}
	if manifest.Hooks[1].Priority != DefaultHookPriority {
		t.Fatalf("Hooks[1].Priority = %d, want %d", manifest.Hooks[1].Priority, DefaultHookPriority)
	}
	if len(manifest.Resources.Skills) != 2 {
		t.Fatalf("len(Resources.Skills) = %d, want 2", len(manifest.Resources.Skills))
	}
	if len(manifest.Providers.Review) != 1 {
		t.Fatalf("len(Providers.Review) = %d, want 1", len(manifest.Providers.Review))
	}
	if manifest.Providers.Review[0].Metadata["tier"] != "gold" {
		t.Fatalf(
			"Providers.Review[0].Metadata[tier] = %q, want %q",
			manifest.Providers.Review[0].Metadata["tier"],
			"gold",
		)
	}
}

func TestLoadManifestSupportsTypedProviderDeclarations(t *testing.T) {
	withVersion(t, "1.5.0")

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ManifestFileNameJSON), `
{
  "extension": {
    "name": "typed-ext",
    "version": "1.2.3",
    "description": "Typed provider extension",
    "min_compozy_version": "1.0.0"
  },
  "subprocess": {
    "command": "bin/typed-ext"
  },
  "security": {
    "capabilities": ["providers.register"]
  },
  "providers": {
    "ide": [
      {
        "name": "typed-ide",
        "display_name": "Typed IDE",
        "command": "mock-acp",
        "fixed_args": ["serve"],
        "probe_args": ["--probe"],
        "default_model": "typed-model",
        "setup_agent_name": "codex",
        "supports_add_dirs": true,
        "uses_bootstrap_model": true,
        "docs_url": "https://example.com/docs",
        "install_hint": "Install the typed ACP runtime",
        "full_access_mode_id": "danger-full-access",
        "env": {
          "MOCK_ENV": "enabled"
        },
        "fallbacks": [
          {
            "command": "npx",
            "fixed_args": ["-y", "mock-acp"]
          }
        ],
        "bootstrap": {
          "model_flag": "--model",
          "reasoning_effort_flag": "--reasoning",
          "add_dir_flag": "--add-dir",
          "default_access_mode_args": ["--sandbox"],
          "full_access_mode_args": ["--danger"]
        }
      }
    ],
    "review": [
      {
        "name": "typed-review",
        "kind": "extension",
        "display_name": "Typed Review"
      }
    ],
    "model": [
      {
        "name": "typed-model",
        "target": "openai/gpt-5.4",
        "display_name": "Typed Model"
      }
    ]
  }
}
`)

	manifest, err := LoadManifest(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if len(manifest.Providers.IDE) != 1 {
		t.Fatalf("len(Providers.IDE) = %d, want 1", len(manifest.Providers.IDE))
	}
	ide := manifest.Providers.IDE[0]
	if ide.DisplayName != "Typed IDE" {
		t.Fatalf("Providers.IDE[0].DisplayName = %q, want %q", ide.DisplayName, "Typed IDE")
	}
	if got := ide.FixedArgs; !strings.EqualFold(strings.Join(got, ","), "serve") {
		t.Fatalf("Providers.IDE[0].FixedArgs = %#v, want [serve]", got)
	}
	if got := ide.ProbeArgs; !strings.EqualFold(strings.Join(got, ","), "--probe") {
		t.Fatalf("Providers.IDE[0].ProbeArgs = %#v, want [--probe]", got)
	}
	if ide.Bootstrap == nil {
		t.Fatal("Providers.IDE[0].Bootstrap = nil, want typed bootstrap config")
	}
	if ide.Bootstrap.ModelFlag != "--model" || ide.Bootstrap.AddDirFlag != "--add-dir" {
		t.Fatalf("unexpected bootstrap config: %#v", ide.Bootstrap)
	}
	if len(ide.Fallbacks) != 1 || ide.Fallbacks[0].Command != "npx" {
		t.Fatalf("unexpected fallback launchers: %#v", ide.Fallbacks)
	}

	if len(manifest.Providers.Review) != 1 {
		t.Fatalf("len(Providers.Review) = %d, want 1", len(manifest.Providers.Review))
	}
	if got := manifest.Providers.Review[0].Kind; got != ProviderKindExtension {
		t.Fatalf("Providers.Review[0].Kind = %q, want %q", got, ProviderKindExtension)
	}

	if len(manifest.Providers.Model) != 1 {
		t.Fatalf("len(Providers.Model) = %d, want 1", len(manifest.Providers.Model))
	}
	if got := manifest.Providers.Model[0].Target; got != "openai/gpt-5.4" {
		t.Fatalf("Providers.Model[0].Target = %q, want %q", got, "openai/gpt-5.4")
	}
}

func TestLoadManifestDecodeAndValidationErrors(t *testing.T) {
	withVersion(t, "1.5.0")

	testCases := []struct {
		name       string
		fileName   string
		content    string
		wantType   any
		wantSubstr string
	}{
		{
			name:       "json trailing content",
			fileName:   ManifestFileNameJSON,
			content:    strings.TrimSpace(validJSONManifest) + "\n{}",
			wantType:   (*ManifestDecodeError)(nil),
			wantSubstr: "unexpected trailing JSON content",
		},
		{
			name:     "toml missing security section",
			fileName: ManifestFileNameTOML,
			content: `
[extension]
name = "broken-ext"
version = "1.2.3"
description = "Broken extension"
min_compozy_version = "1.0.0"
`,
			wantType:   (*ManifestValidationError)(nil),
			wantSubstr: "security: section is required",
		},
		{
			name:     "toml invalid duration",
			fileName: ManifestFileNameTOML,
			content: `
[extension]
name = "broken-ext"
version = "1.2.3"
description = "Broken extension"
min_compozy_version = "1.0.0"

[subprocess]
command = "bin/broken-ext"
shutdown_timeout = "not-a-duration"

[security]
capabilities = ["prompt.mutate"]
`,
			wantType:   (*ManifestDecodeError)(nil),
			wantSubstr: `parse duration "not-a-duration"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, tc.fileName), tc.content)

			_, err := LoadManifest(context.Background(), dir)
			if err == nil {
				t.Fatal("LoadManifest() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("LoadManifest() error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
			switch want := tc.wantType.(type) {
			case *ManifestDecodeError:
				var decodeErr *ManifestDecodeError
				if !errors.As(err, &decodeErr) {
					t.Fatalf("LoadManifest() error = %T, want %T", err, want)
				}
				if decodeErr.Unwrap() == nil {
					t.Fatal("ManifestDecodeError.Unwrap() = nil, want wrapped error")
				}
			case *ManifestValidationError:
				var validationErr *ManifestValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("LoadManifest() error = %T, want %T", err, want)
				}
				if validationErr.Unwrap() == nil {
					t.Fatal("ManifestValidationError.Unwrap() = nil, want wrapped error")
				}
			default:
				t.Fatalf("unsupported wantType %T", tc.wantType)
			}
		})
	}
}

func TestValidateManifestRejectsRequiredRelationships(t *testing.T) {
	testCases := []struct {
		name       string
		mutate     func(*Manifest)
		wantSubstr string
	}{
		{
			name: "missing extension description",
			mutate: func(manifest *Manifest) {
				manifest.Extension.Description = ""
			},
			wantSubstr: "extension.description: value is required",
		},
		{
			name: "hooks require subprocess",
			mutate: func(manifest *Manifest) {
				manifest.Subprocess = nil
			},
			wantSubstr: "subprocess: section is required when hooks are declared",
		},
		{
			name: "hook requires matching capability",
			mutate: func(manifest *Manifest) {
				manifest.Security.Capabilities = []Capability{
					CapabilityProvidersRegister,
					CapabilitySkillsShip,
				}
			},
			wantSubstr: `hooks[0].event="prompt.post_build": requires capability "prompt.mutate"`,
		},
		{
			name: "resources require skills capability",
			mutate: func(manifest *Manifest) {
				manifest.Security.Capabilities = []Capability{
					CapabilityPromptMutate,
					CapabilityProvidersRegister,
				}
			},
			wantSubstr: `resources.skills[0]="skills/*": requires capability "skills.ship"`,
		},
		{
			name: "providers require register capability",
			mutate: func(manifest *Manifest) {
				manifest.Security.Capabilities = []Capability{
					CapabilityPromptMutate,
					CapabilitySkillsShip,
				}
			},
			wantSubstr: `providers.model[0]="fixture-model": requires capability "providers.register"`,
		},
		{
			name: "blank provider command",
			mutate: func(manifest *Manifest) {
				manifest.Providers.Model[0].Command = ""
			},
			wantSubstr: "providers.model[0].target: value is required",
		},
		{
			name: "extension review provider requires subprocess",
			mutate: func(manifest *Manifest) {
				manifest.Providers.Review = []ProviderEntry{{
					Name: "fixture-review",
					Kind: ProviderKindExtension,
				}}
				manifest.Hooks = nil
				manifest.Subprocess = nil
			},
			wantSubstr: `providers.review[0].kind="extension": requires a [subprocess] section`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			withVersion(t, "1.5.0")

			manifest := validManifest()
			tc.mutate(manifest)

			err := ValidateManifest(context.Background(), manifest)
			if err == nil {
				t.Fatal("ValidateManifest() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("ValidateManifest() error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestCapabilityForHookFamilies(t *testing.T) {
	testCases := []struct {
		name string
		hook HookName
		want Capability
	}{
		{name: "plan", hook: HookPlanPreDiscover, want: CapabilityPlanMutate},
		{name: "prompt", hook: HookPromptPreBuild, want: CapabilityPromptMutate},
		{name: "agent", hook: HookAgentPreSessionCreate, want: CapabilityAgentMutate},
		{name: "job", hook: HookJobPreExecute, want: CapabilityJobMutate},
		{name: "run", hook: HookRunPreStart, want: CapabilityRunMutate},
		{name: "review", hook: HookReviewPreFetch, want: CapabilityReviewMutate},
		{name: "artifact", hook: HookArtifactPreWrite, want: CapabilityArtifactsWrite},
		{name: "unknown", hook: HookName("unknown.hook"), want: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := capabilityForHook(tc.hook); got != tc.want {
				t.Fatalf("capabilityForHook(%q) = %q, want %q", tc.hook, got, tc.want)
			}
		})
	}
}

func TestDurationValueUnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name       string
		payload    string
		want       string
		wantErrSub string
	}{
		{name: "string duration", payload: `"3s"`, want: "3s"},
		{name: "numeric duration", payload: `1000000000`, want: "1s"},
		{name: "null duration", payload: `null`, want: "0s"},
		{name: "invalid duration", payload: `"later"`, wantErrSub: `parse duration "later"`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var value durationValue
			err := value.UnmarshalJSON([]byte(tc.payload))
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatal("UnmarshalJSON() error = nil, want failure")
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("UnmarshalJSON() error = %q, want substring %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalJSON() error = %v", err)
			}
			if got := value.String(); got != tc.want {
				t.Fatalf("Duration = %q, want %q", got, tc.want)
			}
		})
	}
}

func validManifest() *Manifest {
	return &Manifest{
		Extension: ExtensionInfo{
			Name:              "fixture-ext",
			Version:           "1.2.3",
			Description:       "Fixture extension",
			MinCompozyVersion: "1.0.0",
		},
		Subprocess: &SubprocessConfig{
			Command: "bin/fixture-ext",
		},
		Security: SecurityConfig{
			Capabilities: []Capability{
				CapabilityPromptMutate,
				CapabilityProvidersRegister,
				CapabilitySkillsShip,
			},
		},
		Hooks: []HookDeclaration{
			{
				Event:    HookPromptPostBuild,
				Priority: 200,
				Required: true,
			},
		},
		Resources: ResourcesConfig{
			Skills: []string{"skills/*"},
		},
		Providers: ProvidersConfig{
			Model: []ProviderEntry{
				{
					Name:    "fixture-model",
					Command: "bin/fixture-model",
				},
			},
		},
	}
}

func withVersion(t *testing.T, value string) {
	t.Helper()

	previous := version.Version
	version.Version = value
	t.Cleanup(func() {
		version.Version = previous
	})
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})
	return &buf
}

func decodeLogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	raw := strings.TrimSpace(buf.String())
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		record := make(map[string]any)
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

const validTOMLManifest = `
[extension]
name = "toml-ext"
version = "1.2.3"
description = "TOML extension"
min_compozy_version = "1.0.0"

[subprocess]
command = "bin/toml-ext"
args = ["serve"]
shutdown_timeout = "12s"
health_check_period = "45s"

[security]
capabilities = ["prompt.mutate", "providers.register", "skills.ship"]

[[hooks]]
event = "prompt.post_build"
priority = 200
required = true
timeout = "2s"

[resources]
skills = ["skills/*"]

[[providers.model]]
name = "toml-model"
command = "bin/toml-model"

[providers.model.metadata]
kind = "toml"
`

const validJSONManifest = `
{
  "extension": {
    "name": "json-ext",
    "version": "2.0.0",
    "description": "JSON extension",
    "min_compozy_version": "1.0.0"
  },
  "subprocess": {
    "command": "bin/json-ext",
    "args": ["serve"],
    "shutdown_timeout": "15s",
    "health_check_period": "60s"
  },
  "security": {
    "capabilities": ["prompt.mutate", "providers.register", "skills.ship"]
  },
  "hooks": [
    {
      "event": "prompt.post_build",
      "required": true,
      "timeout": "3s"
    }
  ],
  "resources": {
    "skills": ["skills/*"]
  },
  "providers": {
    "model": [
      {
        "name": "json-model",
        "command": "bin/json-model",
        "metadata": {
          "kind": "json"
        }
      }
    ]
  }
}
`

const realisticTOMLManifest = `
[extension]
name = "fixture-ext"
version = "1.2.3"
description = "Fixture extension"
min_compozy_version = "1.0.0"

[subprocess]
command = "bin/fixture-ext"
args = ["serve", "--log=json"]
env = { LOG_LEVEL = "debug" }
shutdown_timeout = "10s"
health_check_period = "30s"

[security]
capabilities = ["prompt.mutate", "providers.register", "skills.ship"]

[[hooks]]
event = "prompt.pre_build"
priority = 100
timeout = "1s"

[[hooks]]
event = "prompt.post_build"
required = true

[resources]
skills = ["skills/*", "skills/beta/*"]

[[providers.review]]
name = "fixture-review"
command = "bin/review-provider"

[providers.review.metadata]
tier = "gold"
region = "us"
`
