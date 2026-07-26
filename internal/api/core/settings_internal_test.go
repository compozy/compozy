package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	automationmodel "github.com/compozy/agh/internal/automation/model"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
	settingspkg "github.com/compozy/agh/internal/settings"
)

func TestSettingsHelperFunctionsAndNilErrorWrappers(t *testing.T) {
	t.Parallel()

	if got := NewSettingsValidationError(nil); got != nil {
		t.Fatalf("NewSettingsValidationError(nil) = %v, want nil", got)
	}
	if got := NewSettingsNotFoundError(nil); got != nil {
		t.Fatalf("NewSettingsNotFoundError(nil) = %v, want nil", got)
	}
	if got := NewSettingsConflictError(nil); got != nil {
		t.Fatalf("NewSettingsConflictError(nil) = %v, want nil", got)
	}
	if got := NewSettingsForbiddenError(nil); got != nil {
		t.Fatalf("NewSettingsForbiddenError(nil) = %v, want nil", got)
	}

	if duration, err := parseOptionalDuration("", "path"); err != nil || duration != 0 {
		t.Fatalf("parseOptionalDuration(empty) = %v, %v", duration, err)
	}
	if duration, err := parseOptionalDuration("1m", "path"); err != nil || duration != time.Minute {
		t.Fatalf("parseOptionalDuration(valid) = %v, %v", duration, err)
	}
	if _, err := parseOptionalDuration("bad", "path"); err == nil {
		t.Fatal("parseOptionalDuration(invalid) error = nil, want non-nil")
	}

	if got := settingsLogTailPollInterval(0); got != defaultPollInterval {
		t.Fatalf("settingsLogTailPollInterval(0) = %v, want %v", got, defaultPollInterval)
	}
	if got := settingsLogTailPollInterval(15 * time.Millisecond); got != 15*time.Millisecond {
		t.Fatalf("settingsLogTailPollInterval(custom) = %v, want 15ms", got)
	}

	if _, ok := findSettingsProvider([]settingspkg.ProviderItem{{Name: "openai"}}, "openai"); !ok {
		t.Fatal("findSettingsProvider() = false, want true")
	}
	if _, ok := findSettingsSandbox([]settingspkg.SandboxItem{{Name: "local"}}, "local"); !ok {
		t.Fatal("findSettingsSandbox() = false, want true")
	}

	if providerSettingsPayloadEmpty(contract.SettingsProviderSettingsPayload{
		Models: &contract.SettingsProviderModelsPayload{},
	}) {
		t.Fatal("providerSettingsPayloadEmpty(empty models payload) = true, want false")
	}

	fireLimit := automationFireLimitFromPayload(automationmodel.FireLimitConfig{Max: 5, Window: "1m"})
	if fireLimit.Max != 5 || fireLimit.Window != "1m" {
		t.Fatalf("automationFireLimitFromPayload() = %#v", fireLimit)
	}
}

func TestSettingsLogTailFileHelpers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "agh.log")
	if err := os.WriteFile(path, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	file, info, err := openSettingsLogTailFile(path)
	if err != nil {
		t.Fatalf("openSettingsLogTailFile() error = %v", err)
	}
	defer file.Close()

	rotated, err := settingsLogTailRotated(path, info, file)
	if err != nil {
		t.Fatalf("settingsLogTailRotated(no rotation) error = %v", err)
	}
	if rotated {
		t.Fatal("settingsLogTailRotated(no rotation) = true, want false")
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	rotated, err = settingsLogTailRotated(path, info, file)
	if err != nil {
		t.Fatalf("settingsLogTailRotated(missing path) error = %v", err)
	}
	if !rotated {
		t.Fatal("settingsLogTailRotated(missing path) = false, want true")
	}
}

func TestSettingsConversionErrorBranches(t *testing.T) {
	t.Parallel()

	if _, err := SettingsSectionResponseFromEnvelope(settingspkg.SectionEnvelope{}); err == nil {
		t.Fatal("SettingsSectionResponseFromEnvelope(unknown) error = nil, want non-nil")
	}
	if _, err := SettingsCollectionResponseFromEnvelope(settingspkg.CollectionEnvelope{}); err == nil {
		t.Fatal("SettingsCollectionResponseFromEnvelope(unknown) error = nil, want non-nil")
	}
	if got := StatusForSettingsError(errors.New("settings exploded")); got != 500 {
		t.Fatalf("StatusForSettingsError(default) = %d, want 500", got)
	}
}

func TestSettingsCollectionResponseFromEnvelopeUsesEmptyArrays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		envelope   settingspkg.CollectionEnvelope
		assertList func(t *testing.T, payload any)
	}{
		{
			name:     "providers",
			envelope: settingspkg.CollectionEnvelope{Collection: settingspkg.CollectionProviders},
			assertList: func(t *testing.T, payload any) {
				t.Helper()
				response, ok := payload.(contract.SettingsProvidersResponse)
				if !ok {
					t.Fatalf("payload type = %T, want SettingsProvidersResponse", payload)
				}
				if response.Providers == nil || len(response.Providers) != 0 {
					t.Fatalf("Providers = %#v, want empty non-nil array", response.Providers)
				}
			},
		},
		{
			name:     "mcp servers",
			envelope: settingspkg.CollectionEnvelope{Collection: settingspkg.CollectionMCPServers},
			assertList: func(t *testing.T, payload any) {
				t.Helper()
				response, ok := payload.(contract.SettingsMCPServersResponse)
				if !ok {
					t.Fatalf("payload type = %T, want SettingsMCPServersResponse", payload)
				}
				if response.MCPServers == nil || len(response.MCPServers) != 0 {
					t.Fatalf("MCPServers = %#v, want empty non-nil array", response.MCPServers)
				}
			},
		},
		{
			name:     "sandboxes",
			envelope: settingspkg.CollectionEnvelope{Collection: settingspkg.CollectionSandboxes},
			assertList: func(t *testing.T, payload any) {
				t.Helper()
				response, ok := payload.(contract.SettingsSandboxesResponse)
				if !ok {
					t.Fatalf("payload type = %T, want SettingsSandboxesResponse", payload)
				}
				if response.Sandboxes == nil || len(response.Sandboxes) != 0 {
					t.Fatalf("Sandboxes = %#v, want empty non-nil array", response.Sandboxes)
				}
			},
		},
		{
			name:     "hooks",
			envelope: settingspkg.CollectionEnvelope{Collection: settingspkg.CollectionHooks},
			assertList: func(t *testing.T, payload any) {
				t.Helper()
				response, ok := payload.(contract.SettingsHooksResponse)
				if !ok {
					t.Fatalf("payload type = %T, want SettingsHooksResponse", payload)
				}
				if response.Hooks == nil || len(response.Hooks) != 0 {
					t.Fatalf("Hooks = %#v, want empty non-nil array", response.Hooks)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run("Should return empty array for "+tc.name, func(t *testing.T) {
			t.Parallel()

			payload, err := SettingsCollectionResponseFromEnvelope(tc.envelope)
			if err != nil {
				t.Fatalf("SettingsCollectionResponseFromEnvelope() error = %v", err)
			}
			tc.assertList(t, payload)
		})
	}
}

func TestSettingsSectionResponseFromEnvelopeRequiresConcreteSectionPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		envelope settingspkg.SectionEnvelope
		want     string
	}{
		{
			name:     "general",
			envelope: settingspkg.SectionEnvelope{Section: settingspkg.SectionGeneral},
			want:     "settings general section is required",
		},
		{
			name:     "memory",
			envelope: settingspkg.SectionEnvelope{Section: settingspkg.SectionMemory},
			want:     "settings memory section is required",
		},
		{
			name:     "skills",
			envelope: settingspkg.SectionEnvelope{Section: settingspkg.SectionSkills},
			want:     "settings skills section is required",
		},
		{
			name:     "automation",
			envelope: settingspkg.SectionEnvelope{Section: settingspkg.SectionAutomation},
			want:     "settings automation section is required",
		},
		{
			name:     "network",
			envelope: settingspkg.SectionEnvelope{Section: settingspkg.SectionNetwork},
			want:     "settings network section is required",
		},
		{
			name:     "observability",
			envelope: settingspkg.SectionEnvelope{Section: settingspkg.SectionObservability},
			want:     "settings observability section is required",
		},
		{
			name:     "hooks extensions",
			envelope: settingspkg.SectionEnvelope{Section: settingspkg.SectionHooksExtensions},
			want:     "settings hooks-extensions section is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := SettingsSectionResponseFromEnvelope(tc.envelope)
			if err == nil {
				t.Fatal("SettingsSectionResponseFromEnvelope() error = nil, want non-nil")
			}
			if err.Error() != tc.want {
				t.Fatalf("SettingsSectionResponseFromEnvelope() error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestSettingsPayloadHelpersRejectInvalidInputs(t *testing.T) {
	t.Parallel()

	if _, err := generalSettingsFromPayload(contract.SettingsGeneralConfigPayload{
		Defaults:       contract.SettingsDefaultsPayload{Agent: "coder"},
		Limits:         contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
		Permissions:    contract.SettingsPermissionsPayload{Mode: contract.SettingsPermissionModeApproveReads},
		SessionTimeout: "bad",
		HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
		Daemon:         contract.SettingsDaemonPayload{Socket: "/tmp/agh.sock"},
	}); err == nil {
		t.Fatal("generalSettingsFromPayload(invalid timeout) error = nil, want non-nil")
	}

	t.Run("Should reject an invalid daemon memory report interval", func(t *testing.T) {
		t.Parallel()

		validGeneral := contract.SettingsGeneralConfigPayload{
			Defaults:       contract.SettingsDefaultsPayload{Agent: "coder"},
			Limits:         contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
			Permissions:    contract.SettingsPermissionsPayload{Mode: contract.SettingsPermissionModeApproveReads},
			SessionTimeout: "30m",
			HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
			Daemon: contract.SettingsDaemonPayload{
				Socket:               "/tmp/agh.sock",
				MemoryReportInterval: "invalid",
			},
		}
		if _, err := generalSettingsFromPayload(validGeneral); err == nil {
			t.Fatal("generalSettingsFromPayload(invalid memory report interval) error = nil, want non-nil")
		}
	})

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "memory-home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	memoryConfig := aghconfig.DefaultWithHome(homePaths).Memory
	memoryPayload := settingsMemoryConfigPayload(&memoryConfig)
	memoryPayload.Dream.CheckInterval = "bad"
	if _, err := memoryConfigFromPayload(&memoryPayload); err == nil {
		t.Fatal("memoryConfigFromPayload(invalid interval) error = nil, want non-nil")
	}

	if _, err := skillsConfigFromPayload(contract.SettingsSkillsConfigPayload{
		Enabled:      true,
		PollInterval: "bad",
		Marketplace:  contract.SettingsMarketplacePayload{Registry: "clawhub"},
	}); err == nil {
		t.Fatal("skillsConfigFromPayload(invalid interval) error = nil, want non-nil")
	}

	if _, err := extensionRateLimitConfigFromPayload(
		contract.SettingsExtensionRateLimitPayload{Requests: 1, Window: "bad"},
		"extensions.resources.snapshot_rate_limit",
	); err == nil {
		t.Fatal("extensionRateLimitConfigFromPayload(invalid window) error = nil, want non-nil")
	}

	if _, err := sandboxProfileFromPayload(contract.SettingsSandboxProfilePayload{
		Backend: "invalid",
	}); err == nil {
		t.Fatal("sandboxProfileFromPayload(invalid backend) error = nil, want non-nil")
	}

	if _, err := hookDeclarationFromPayload(contract.SettingsHookDeclarationPayload{
		Name:         "capture",
		Event:        hookspkg.HookToolPreCall,
		Mode:         hookspkg.HookModeAsync,
		ExecutorKind: hookspkg.HookExecutorSubprocess,
		Command:      "/bin/capture",
		Timeout:      "bad",
		Matcher: hookspkg.HookMatcher{
			ToolID: "agh__read",
		},
	}); err == nil {
		t.Fatal("hookDeclarationFromPayload(invalid timeout) error = nil, want non-nil")
	}

	if _, err := parseSettingsTarget("invalid"); err == nil {
		t.Fatal("parseSettingsTarget(invalid) error = nil, want non-nil")
	}
	if _, err := requiredSettingsPathValue("", "name"); err == nil {
		t.Fatal("requiredSettingsPathValue(empty) error = nil, want non-nil")
	}
	if _, _, err := parseSettingsScope("invalid", ""); err == nil {
		t.Fatal("parseSettingsScope(invalid) error = nil, want non-nil")
	}

	if _, err := extensionsConfigFromPayload(contract.SettingsExtensionsConfigPayload{
		Marketplace: contract.SettingsExtensionMarketplacePayload{
			Registry:        "github",
			AllowUnverified: true,
		},
		Resources: contract.SettingsExtensionResourcesPayload{
			AllowedKinds: []string{string(resources.ResourceKind("tool"))},
			MaxScope:     resources.ResourceScopeKindWorkspace,
			SnapshotRateLimit: contract.SettingsExtensionRateLimitPayload{
				Requests: 1,
				Window:   "1m",
			},
			OperatorWriteRateLimit: contract.SettingsExtensionRateLimitPayload{
				Requests: 1,
				Window:   "1m",
			},
		},
	}); err != nil {
		t.Fatalf("extensionsConfigFromPayload(valid) error = %v", err)
	}

	if _, err := automationSettingsFromPayload(contract.SettingsAutomationConfigPayload{
		Enabled:           true,
		Timezone:          "UTC",
		MaxConcurrentJobs: 1,
		DefaultFireLimit:  automationmodel.FireLimitConfig{Max: 1, Window: "1m"},
	}); err != nil {
		t.Fatalf("automationSettingsFromPayload(valid) error = %v", err)
	}
	if _, err := networkConfigFromPayload(contract.SettingsNetworkConfigPayload{
		Enabled:      true,
		MaxReplayAge: 10,
		Live: contract.SettingsNetworkLiveConfigPayload{
			Defaults: contract.SettingsNetworkLiveDefaultsPayload{
				MaxWakes:         8,
				MaxWakeWallTime:  "5m",
				MaxTotalWallTime: "30m",
				MaxInputTokens:   200_000,
				MaxOutputTokens:  50_000,
				MaxWakeDepth:     3,
				CoalesceWindow:   "500ms",
			},
			Limits: contract.SettingsNetworkLiveLimitsPayload{
				MaxWakes:          64,
				MaxWakeWallTime:   "15m",
				MaxTotalWallTime:  "2h",
				MaxInputTokens:    1_000_000,
				MaxOutputTokens:   200_000,
				MaxWakeDepth:      5,
				MinCoalesceWindow: "100ms",
				MaxCoalesceWindow: "5s",
			},
		},
	}); err != nil {
		t.Fatalf("networkConfigFromPayload(valid) error = %v", err)
	}
	if _, err := observabilityConfigFromPayload(contract.SettingsObservabilityConfigPayload{
		Enabled:        true,
		RetentionDays:  7,
		MaxGlobalBytes: 1024,
		Transcripts: contract.SettingsObservabilityTranscriptPayload{
			Enabled:            true,
			SegmentBytes:       2048,
			MaxBytesPerSession: 4096,
		},
	}); err != nil {
		t.Fatalf("observabilityConfigFromPayload(valid) error = %v", err)
	}
}

func TestExtensionsConfigFromPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		allowedKinds []string
	}{
		{name: "Should preserve omitted allowed kinds as nil"},
		{name: "Should normalize explicit empty allowed kinds as nil", allowedKinds: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, err := extensionsConfigFromPayload(contract.SettingsExtensionsConfigPayload{
				Resources: contract.SettingsExtensionResourcesPayload{
					AllowedKinds: tt.allowedKinds,
				},
			})
			if err != nil {
				t.Fatalf("extensionsConfigFromPayload() error = %v", err)
			}
			if config.Resources.AllowedKinds != nil {
				t.Fatalf("AllowedKinds = %#v, want nil", config.Resources.AllowedKinds)
			}
		})
	}
}

func TestGeneralSettingsPayloadRoundTripPreservesRedactionGate(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should preserve enabled=%t", enabled), func(t *testing.T) {
			t.Parallel()

			payload := contract.SettingsGeneralConfigPayload{
				Defaults:       contract.SettingsDefaultsPayload{Agent: "coder"},
				Limits:         contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
				Permissions:    contract.SettingsPermissionsPayload{Mode: contract.SettingsPermissionModeApproveReads},
				SessionTimeout: "30m",
				HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
				Daemon:         contract.SettingsDaemonPayload{Socket: "/tmp/agh.sock"},
				Redact:         contract.SettingsRedactPayload{Enabled: enabled},
			}
			settings, err := generalSettingsFromPayload(payload)
			if err != nil {
				t.Fatalf("generalSettingsFromPayload() error = %v", err)
			}
			if settings.Redact.Enabled != enabled {
				t.Fatalf("GeneralSettings.Redact.Enabled = %t, want %t", settings.Redact.Enabled, enabled)
			}
			if got := settingsGeneralConfigPayload(settings).Redact.Enabled; got != enabled {
				t.Fatalf("settingsGeneralConfigPayload().Redact.Enabled = %t, want %t", got, enabled)
			}
		})
	}
}

func TestMemorySettingsPayloadRoundTripIncludesV2Config(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "memory-home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	want := aghconfig.DefaultWithHome(homePaths).Memory
	want.GlobalDir = "/tmp/roundtrip-memory"
	want.Controller.Mode = "rules"
	want.Controller.Policy.AllowOrigins = []string{"cli", "tool"}
	want.Recall.IncludeAlreadySurfaced = true
	want.Extractor.Queue.CoalesceMax = 12
	want.Dream.MinHours = 18
	want.Session.UnboundPartition = "_orphans"
	want.Provider.Name = "local"
	want.Workspace.AutoCreate = false

	payload := settingsMemoryConfigPayload(&want)
	got, err := memoryConfigFromPayload(&payload)
	if err != nil {
		t.Fatalf("memoryConfigFromPayload() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("memoryConfigFromPayload() = %#v, want %#v", got, want)
	}
}
