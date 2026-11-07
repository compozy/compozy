package settings

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
)

func newProviderEnabledTestService(t *testing.T, config string) (Service, compozyconfig.HomePaths) {
	t.Helper()
	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, config)
	db, err := openSettingsTestGlobalDB(ctx, t, homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	service := testService(t, homePaths, Dependencies{ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil)})
	if _, err := service.ActiveConfig(ctx); err != nil {
		t.Fatalf("ActiveConfig() error = %v", err)
	}
	return service, homePaths
}

// enabledBaseConfig is the shared fixture without its leading blank line, which config loads normalize.
func enabledBaseConfig() string {
	return strings.TrimLeft(baseSettingsConfig(), "\n")
}

// pendingDefaultConfig moves defaults.provider to claude next to a restart-required port change, so
// the default switch stays pending.
func pendingDefaultConfig() string {
	pending := strings.Replace(enabledBaseConfig(), `provider = "codex"`, `provider = "claude"`, 1)
	return strings.Replace(pending, "port = 9001", "port = 9002", 1)
}

func activeProviderDisabled(t *testing.T, service Service, provider string) bool {
	t.Helper()
	active, err := service.ActiveConfig(context.Background())
	if err != nil {
		t.Fatalf("ActiveConfig() error = %v", err)
	}
	return active.IsProviderDisabled(provider)
}

func TestSetProviderEnabled(t *testing.T) {
	t.Parallel()

	const customizedGemini = `
[providers.gemini]
display_name = "Gemini (work)"
env_policy = "isolated"

[provider_usage.providers.gemini]
enabled = true
`

	t.Run("Should toggle only the enabled key and restore the file byte for byte", func(t *testing.T) {
		t.Parallel()
		original := enabledBaseConfig() + customizedGemini
		service, homePaths := newProviderEnabledTestService(t, original)
		ctx := WithMutationSource(t.Context(), "http")

		item, apply, err := service.SetProviderEnabled(ctx, "gemini", false)
		if err != nil {
			t.Fatalf("SetProviderEnabled(gemini, false) error = %v", err)
		}
		if item.Enabled || !apply.Applied || apply.RestartRequired || apply.Record.Lifecycle != lifecycle.Live {
			t.Fatalf("disable = enabled %v applied %v restart %v lifecycle %q, want live disabled",
				item.Enabled, apply.Applied, apply.RestartRequired, apply.Record.Lifecycle)
		}
		disabled := readFile(t, homePaths.ConfigFile)
		if !strings.Contains(disabled, `display_name = "Gemini (work)"`) ||
			!strings.Contains(disabled, "enabled = false") {
			t.Fatalf("config after disable = %q, want the authored keys plus enabled = false", disabled)
		}
		if !activeProviderDisabled(t, service, "gemini") {
			t.Fatal("active config keeps gemini enabled, want the live disable")
		}

		if _, _, err := service.SetProviderEnabled(ctx, "gemini", true); err != nil {
			t.Fatalf("SetProviderEnabled(gemini, true) error = %v", err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != original {
			t.Fatalf("config after re-enable = %q, want the original bytes %q", got, original)
		}
		_, again, err := service.SetProviderEnabled(ctx, "gemini", true)
		if err != nil || !again.Skipped {
			t.Fatalf("repeated re-enable = skipped %v, error %v, want a skipped no-op", again.Skipped, err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != original {
			t.Fatalf("config after repeated re-enable = %q, want unchanged", got)
		}
	})

	t.Run("Should remove a provider block that only held enabled", func(t *testing.T) {
		t.Parallel()
		original := enabledBaseConfig()
		service, homePaths := newProviderEnabledTestService(t, original)
		ctx := t.Context()

		if _, _, err := service.SetProviderEnabled(ctx, "claude", false); err != nil {
			t.Fatalf("SetProviderEnabled(claude, false) error = %v", err)
		}
		if !strings.Contains(readFile(t, homePaths.ConfigFile), "[providers.claude]") {
			t.Fatal("config after disable has no [providers.claude] block")
		}
		if _, _, err := service.SetProviderEnabled(ctx, "claude", true); err != nil {
			t.Fatalf("SetProviderEnabled(claude, true) error = %v", err)
		}
		if got := readFile(t, homePaths.ConfigFile); strings.Contains(got, "providers.claude") {
			t.Fatalf("config after re-enable = %q, want the emptied block removed", got)
		}
	})

	t.Run("Should reject disabling the default provider without writing", func(t *testing.T) {
		t.Parallel()
		original := enabledBaseConfig()
		service, homePaths := newProviderEnabledTestService(t, original)

		_, _, err := service.SetProviderEnabled(t.Context(), "codex", false)
		if !errors.Is(err, compozyconfig.ErrProviderDefaultDisableRejected) || !errors.Is(err, ErrUnprocessable) {
			t.Fatalf("SetProviderEnabled(codex, false) error = %v, want unprocessable default rejection", err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != original {
			t.Fatalf("config after rejected disable = %q, want unchanged", got)
		}
	})

	t.Run("Should reject disabling the active default while its change is pending", func(t *testing.T) {
		t.Parallel()
		service, homePaths := newProviderEnabledTestService(t, enabledBaseConfig())
		pending := pendingDefaultConfig()
		writeFile(t, homePaths.ConfigFile, pending)

		_, _, err := service.SetProviderEnabled(t.Context(), "codex", false)
		defaultErr, ok := errors.AsType[*compozyconfig.ProviderDefaultDisableError](err)
		if !ok || !defaultErr.PendingDefault {
			t.Fatalf("SetProviderEnabled(codex, false) error = %v, want the pending default rejection", err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != pending {
			t.Fatalf("config after rejected disable = %q, want unchanged", got)
		}
	})

	t.Run("Should reject an unknown provider", func(t *testing.T) {
		t.Parallel()
		service, _ := newProviderEnabledTestService(t, enabledBaseConfig())
		if _, _, err := service.SetProviderEnabled(t.Context(), "nope", false); !errors.Is(err, ErrNotFound) {
			t.Fatalf("SetProviderEnabled(nope) error = %v, want not found", err)
		}
	})

	t.Run("Should keep a pending command edit pending while enabled applies live", func(t *testing.T) {
		t.Parallel()
		service, homePaths := newProviderEnabledTestService(t, enabledBaseConfig()+customizedGemini)
		withCommand := strings.Replace(customizedGemini, `env_policy = "isolated"`,
			"env_policy = \"isolated\"\ncommand = \"gemini-alt --acp\"", 1)
		writeFile(t, homePaths.ConfigFile, enabledBaseConfig()+withCommand)
		before, err := service.ActiveConfig(t.Context())
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}

		if _, apply, err := service.SetProviderEnabled(t.Context(), "gemini", false); err != nil || !apply.Applied {
			t.Fatalf("SetProviderEnabled(gemini, false) = applied %v, error %v, want live", apply.Applied, err)
		}
		after, err := service.ActiveConfig(t.Context())
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}
		if !after.IsProviderDisabled("gemini") ||
			after.Providers["gemini"].Command != before.Providers["gemini"].Command {
			t.Fatalf("active gemini = %#v, want disabled with the previous command", after.Providers["gemini"])
		}
		if pending, err := service.HasPendingConfigRestart(t.Context()); err != nil || !pending {
			t.Fatalf("HasPendingConfigRestart() = %v, %v, want the command edit still pending", pending, err)
		}
	})

	t.Run("Should preserve the authored enabled key across a full provider edit", func(t *testing.T) {
		t.Parallel()
		service, homePaths := newProviderEnabledTestService(t, enabledBaseConfig()+customizedGemini)
		ctx := t.Context()
		if _, _, err := service.SetProviderEnabled(ctx, "gemini", false); err != nil {
			t.Fatalf("SetProviderEnabled(gemini, false) error = %v", err)
		}
		if _, err := service.ApplyCollectionItem(ctx, CollectionItemPutRequest{
			Collection: CollectionProviders,
			Name:       "gemini",
			Provider:   &ProviderSettings{DisplayName: "Gemini (renamed)"},
		}); err != nil {
			t.Fatalf("ApplyCollectionItem(gemini) error = %v", err)
		}
		got := readFile(t, homePaths.ConfigFile)
		if !strings.Contains(got, "Gemini (renamed)") || !strings.Contains(got, "enabled = false") {
			t.Fatalf("config after full edit = %q, want the rename with enabled = false kept", got)
		}
	})
}

func TestReloadProviderEnabledSplit(t *testing.T) {
	t.Parallel()

	t.Run("Should apply enabled live and keep other restart-required paths pending", func(t *testing.T) {
		t.Parallel()
		service, homePaths := newProviderEnabledTestService(t, enabledBaseConfig())
		before, err := service.ActiveConfig(t.Context())
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, enabledBaseConfig()+`
[providers.gemini]
command = "gemini-alt --acp"
enabled = false
`)

		result, err := service.Reload(t.Context())
		if err != nil {
			t.Fatalf("Reload() error = %v", err)
		}
		if !result.Applied || !result.RestartRequired || result.NextAction != lifecycle.NextActionRestartDaemon {
			t.Fatalf("Reload() = applied %v restart %v next %q, want live enabled plus pending restart",
				result.Applied, result.RestartRequired, result.NextAction)
		}
		after, err := service.ActiveConfig(t.Context())
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}
		if !after.IsProviderDisabled("gemini") ||
			after.Providers["gemini"].Command != before.Providers["gemini"].Command {
			t.Fatalf("active gemini = %#v, want disabled without the pending command", after.Providers["gemini"])
		}
		again, err := service.Reload(t.Context())
		if err != nil || again.Applied || !again.RestartRequired {
			t.Fatalf("second Reload() = applied %v restart %v, error %v, want blocked restart",
				again.Applied, again.RestartRequired, err)
		}
	})

	t.Run("Should leave an invalid enabled projection pending with a warning", func(t *testing.T) {
		t.Parallel()
		service, homePaths := newProviderEnabledTestService(t, enabledBaseConfig())
		writeFile(t, homePaths.ConfigFile, pendingDefaultConfig()+"\n[providers.codex]\nenabled = false\n")

		result, err := service.Reload(t.Context())
		if err != nil {
			t.Fatalf("Reload() error = %v", err)
		}
		if result.Applied || !result.RestartRequired || len(result.Warnings) != 1 ||
			!strings.Contains(result.Warnings[0], "not applied live") {
			t.Fatalf("Reload() = applied %v restart %v warnings %v, want pending with one warning",
				result.Applied, result.RestartRequired, result.Warnings)
		}
		if activeProviderDisabled(t, service, "codex") {
			t.Fatal("active config disabled codex, want active state untouched")
		}
	})
}

func TestDiffProviderSettingsEnabledPath(t *testing.T) {
	t.Parallel()

	t.Run("Should emit the enabled path alone or next to a whole-provider change", func(t *testing.T) {
		t.Parallel()
		disabled := map[string]compozyconfig.ProviderConfig{"gemini": {Enabled: new(false)}}
		if got := diffProviderSettings(nil, disabled); !slices.Equal(got, []string{"providers.gemini.enabled"}) {
			t.Fatalf("enabled-only diff = %v, want only providers.gemini.enabled", got)
		}
		combined := map[string]compozyconfig.ProviderConfig{
			"gemini": {Enabled: new(false), Command: "gemini-alt --acp"},
		}
		want := []string{"providers.gemini", "providers.gemini.enabled"}
		if got := diffProviderSettings(nil, combined); !slices.Equal(got, want) {
			t.Fatalf("combined diff = %v, want %v", got, want)
		}
	})
}
