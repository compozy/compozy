//go:build integration

package modelcatalog_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	_ "modernc.org/sqlite"
)

// TestCatalogServiceGlobalDBIntegration verifies source replacement, account isolation, and persisted capability recovery.
func TestCatalogServiceGlobalDBIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should discover account scoped Claude overlays through ACP subprocesses and SQLite", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		driver := acpmock.RequireDriver(t)
		fixture := acpmock.Fixture{Version: acpmock.FixtureVersion}
		for _, name := range []string{"haiku", "sonnet"} {
			fixture.Agents = append(fixture.Agents, acpmock.AgentFixture{
				Name:     name,
				Provider: "claude",
				Model:    name,
				ConfigOptions: []acpmock.SessionConfigOptionFixture{
					{
						ID:       "model",
						Name:     "Model",
						Category: "model",
						Current:  name,
						Values: []acpmock.SessionConfigOptionValueFixture{
							{Value: name, Label: map[string]string{"haiku": "Haiku 4.5", "sonnet": "Sonnet 5"}[name]},
						},
					},
				},
				Turns: []acpmock.TurnFixture{
					{
						Match: acpmock.TurnMatch{UserText: "noop"},
						Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Chunks: []string{"noop"}}},
					},
				},
			})
			if name == "sonnet" {
				fixture.Agents[len(fixture.Agents)-1].ConfigOptions = append(
					fixture.Agents[len(fixture.Agents)-1].ConfigOptions,
					acpmock.SessionConfigOptionFixture{
						ID:       "deliberation",
						Name:     "Effort",
						Category: "thought_level",
						Current:  "future-effort",
						Values:   []acpmock.SessionConfigOptionValueFixture{{Value: "low"}, {Value: "future-effort"}},
					},
				)
			}
		}
		data, err := json.Marshal(fixture)
		if err != nil {
			t.Fatal(err)
		}
		fixturePath := filepath.Join(t.TempDir(), "overlay-fixture.json")
		if err := os.WriteFile(fixturePath, data, 0o600); err != nil {
			t.Fatal(err)
		}
		store, _ := openCatalogGlobalDB(t)
		var sources []modelcatalog.Source
		for _, name := range []string{"haiku", "sonnet"} {
			providerID := "claude-" + name
			provider := compozyconfig.ProviderConfig{
				RuntimeProvider: "claude", Command: acpmock.BuildCommand(driver, fixturePath, name, ""),
				Harness: compozyconfig.ProviderHarnessACP, AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
			}
			source, err := modelcatalog.NewLiveProviderSource(
				providerID,
				provider,
				&modelcatalog.LiveProviderSourcesConfig{
					BaseEnv: os.Environ(), WorkingDir: t.TempDir(),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			sources = append(sources, source)
		}
		service, err := modelcatalog.NewService(
			store,
			sources,
			modelcatalog.MergeOptions{ReasoningApply: map[string]bool{"claude-haiku": true, "claude-sonnet": true}},
		)
		if err != nil {
			t.Fatal(err)
		}
		execution := modelcatalog.CatalogExecutionContext{
			Scope:     modelcatalog.ExecutionScopeProfile,
			ProfileID: "overlay-test",
		}
		for _, test := range []struct{ provider, model, transport string }{
			{"claude-haiku", "claude-haiku-4-5-20251001", "haiku"},
			{"claude-sonnet", "claude-sonnet-5", "sonnet"},
		} {
			models, err := service.ListModels(
				ctx,
				modelcatalog.ListOptions{
					ProviderID:       test.provider,
					ExecutionContext: execution,
					Now:              integrationTime(0),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(models) != 1 || models[0].ProviderID != test.provider || models[0].ModelID != test.model ||
				models[0].AvailabilityState != modelcatalog.AvailabilityStateAvailableLive {
				t.Fatalf("account catalog = %#v", models)
			}
			if len(models[0].TransportBindings) != 1 ||
				models[0].TransportBindings[0].TransportModelID != test.transport {
				t.Fatalf("transport bindings = %#v", models[0].TransportBindings)
			}
			// A fresh service must recover the same model-specific profile from SQLite.
			persisted, err := modelcatalog.NewService(
				store,
				sources,
				modelcatalog.MergeOptions{ReasoningApply: map[string]bool{test.provider: true}},
			)
			if err != nil {
				t.Fatal(err)
			}
			reloaded, err := persisted.ListModels(
				ctx,
				modelcatalog.ListOptions{
					ProviderID:       test.provider,
					ExecutionContext: execution,
					Now:              integrationTime(0),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(reloaded) != 1 || !reflect.DeepEqual(reloaded[0].ConfigOptions, models[0].ConfigOptions) {
				t.Fatalf("reloaded model options = %#v, want %#v", reloaded, models[0].ConfigOptions)
			}
			var wantEfforts []modelcatalog.ReasoningEffort
			if test.transport == "sonnet" {
				wantEfforts = []modelcatalog.ReasoningEffort{"low", "future-effort"}
			}
			if !slices.Equal(reloaded[0].ReasoningEfforts, wantEfforts) {
				t.Fatalf(
					"reloaded effort for %s = %v, want %v",
					test.provider,
					reloaded[0].ReasoningEfforts,
					wantEfforts,
				)
			}
		}
	})

	t.Run("Should transition OpenCode versions and isolate invocation partitions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		fixtureDir := writeOpenCodeNativeFixture(t)
		workDir := t.TempDir()
		homeDir := t.TempDir()
		writeOpenCodeFixtureFile(t, fixtureDir, "version", "opencode v1.18.33\n")
		writeOpenCodeFixtureFile(t, fixtureDir, "v1.txt",
			"openrouter/acme/coding-default\n"+
				`{"name":"Coding default","capabilities":{"reasoning":true,"toolcall":true},`+
				`"limit":{"context":262144},"variants":{"high":{},"low":{}}}`+"\n")
		provider := compozyconfig.BuiltinProviders()["opencode"]
		liveSource, err := modelcatalog.NewLiveProviderSource(
			"opencode",
			provider,
			&modelcatalog.LiveProviderSourcesConfig{
				BaseEnv: []string{
					"PATH=" + filepath.Join(fixtureDir, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
					"HOME=" + homeDir,
					"OPENCODE_FIXTURE_DIR=" + fixtureDir,
					"OPENCODE_FIXTURE_LOG=" + filepath.Join(fixtureDir, "calls.log"),
				},
				WorkingDir: workDir,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		configSource := modelcatalog.NewConfigSource(map[string]compozyconfig.ProviderConfig{
			"opencode": {Models: compozyconfig.ProviderModelsConfig{
				Curated: []compozyconfig.ProviderModelConfig{{ID: "openrouter/acme/coding-default"}},
			}},
		})
		store, dbPath := openCatalogGlobalDB(t)
		service, err := modelcatalog.NewService(
			store,
			[]modelcatalog.Source{configSource, liveSource},
			modelcatalog.MergeOptions{ReasoningApply: map[string]bool{"opencode": true}},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		execution := modelcatalog.CatalogExecutionContext{
			Scope:     modelcatalog.ExecutionScopeProfile,
			ProfileID: "it-opencode-transition",
		}
		if err := service.SetDefaultExecutionContext(execution); err != nil {
			t.Fatalf("SetDefaultExecutionContext() error = %v", err)
		}
		refresh := func(force bool, now time.Time) []modelcatalog.SourceStatus {
			t.Helper()
			statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
				ProviderID:       "opencode",
				ExecutionContext: execution,
				Force:            force,
				Now:              now,
			})
			if err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
			return statuses
		}
		listAll := func(now time.Time) []modelcatalog.Model {
			t.Helper()
			models, err := service.ListModels(ctx, modelcatalog.ListOptions{
				ProviderID:       "opencode",
				ExecutionContext: execution,
				View:             modelcatalog.CatalogViewAll,
				IncludeStale:     true,
				Now:              now,
			})
			if err != nil {
				t.Fatalf("ListModels() error = %v", err)
			}
			return models
		}

		// Step 1: v1 discovery persists a curated row with lossless metadata.
		refresh(true, integrationTime(40))
		calls := openCodeFixtureCalls(t, fixtureDir)
		if len(calls) != 2 || calls[0] != "--version" || calls[1] != "models --verbose" {
			t.Fatalf("v1 discovery calls = %#v, want version then models --verbose", calls)
		}
		models := listAll(integrationTime(40))
		if got := integrationModelIDs(models); !slices.Equal(got, []string{"openrouter/acme/coding-default"}) {
			t.Fatalf("v1 model ids = %#v", got)
		}
		if !models[0].Curated || models[0].ContextWindow == nil || *models[0].ContextWindow != 262144 ||
			models[0].SupportsTools == nil || !*models[0].SupportsTools ||
			!slices.Equal(models[0].ReasoningEfforts, []modelcatalog.ReasoningEffort{"low", "high"}) {
			t.Fatalf("v1 metadata lost: %#v", models[0])
		}

		// Step 2: in-place v2 upgrade returns a complete new generation.
		writeOpenCodeFixtureFile(t, fixtureDir, "version", "opencode v2.0.32\n")
		writeOpenCodeFixtureFile(t, fixtureDir, "v2.json", `{"location":{},"data":[
{"providerID":"openrouter","id":"acme/coding-default","name":"Coding default","enabled":true,
"capabilities":{"tools":true},"limit":{"context":262144,"input":200000,"output":32768},
"variants":[{"id":"high"},{"id":"low"}]},
{"providerID":"openai","id":"qa-next","name":"QA Next","enabled":true,"limit":{"context":128000},"variants":[]}
]}`)
		refresh(true, integrationTime(41))
		calls = openCodeFixtureCalls(t, fixtureDir)
		if len(calls) != 4 || calls[2] != "--version" ||
			!strings.HasPrefix(calls[3], "api --standalone GET /api/model?") {
			t.Fatalf("v2 discovery calls = %#v, want version then standalone model.list", calls)
		}
		rawQuery, found := strings.CutPrefix(strings.SplitN(calls[3], " ", 4)[3], "/api/model?")
		if !found {
			t.Fatalf("v2 call = %q, want encoded model.list path", calls[3])
		}
		values, err := url.ParseQuery(rawQuery)
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		if got := values.Get("location[directory]"); got != workDir {
			t.Fatalf("v2 location = %q, want %q", got, workDir)
		}
		models = listAll(integrationTime(41))
		if got := integrationModelIDs(models); !slices.Equal(
			got,
			[]string{"openrouter/acme/coding-default", "openai/qa-next"},
		) {
			t.Fatalf("v2 model ids = %#v", got)
		}
		if !models[0].Curated || models[1].Curated {
			t.Fatalf("curation changed across version transition: %#v", models)
		}
		if models[0].SupportsTools == nil || !*models[0].SupportsTools ||
			models[0].MaxInputTokens == nil || *models[0].MaxInputTokens != 200000 {
			t.Fatalf("v2 metadata lost: %#v", models[0])
		}
		step2LiveRows := liveRowRefreshedAt(t, dbPath)

		// A warm cached read performs no further detection.
		if _, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
			Now:              integrationTime(42),
		}); err != nil {
			t.Fatalf("ListModels(cached) error = %v", err)
		}
		if got := len(openCodeFixtureCalls(t, fixtureDir)); got != 4 {
			t.Fatalf("cached read executed %d discovery calls, want 4", got)
		}

		// Step 3: malformed v2 after a valid row fails without partial rows.
		writeOpenCodeFixtureFile(t, fixtureDir, "v2.json", `{"data":[
{"providerID":"openrouter","id":"acme/coding-default","name":"Coding default","enabled":true},
{"providerID":"openai","id":"qa-next","enabled":"yes"}]}`)
		_, refreshErr := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
			Force:            true,
			Now:              integrationTime(43),
		})
		if refreshErr == nil || !strings.Contains(refreshErr.Error(), "invalid OpenCode v2 model payload") {
			t.Fatalf("Refresh() error = %v, want v2 payload failure", refreshErr)
		}
		models = listAll(integrationTime(43))
		if got := integrationModelIDs(models); !slices.Equal(
			got,
			[]string{"openrouter/acme/coding-default", "openai/qa-next"},
		) {
			t.Fatalf("stale model ids = %#v, want last complete generation", got)
		}
		for _, model := range models {
			if !model.Stale || model.AvailabilityState != modelcatalog.AvailabilityStateAvailableStale {
				t.Fatalf("stale model = %#v, want stale availability", model)
			}
		}
		liveOnly, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:       "opencode",
			SourceID:         "provider_live:opencode",
			ExecutionContext: execution,
			View:             modelcatalog.CatalogViewAll,
			IncludeStale:     true,
			Now:              integrationTime(43),
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got := integrationModelIDs(liveOnly); !slices.Equal(
			got,
			[]string{"openrouter/acme/coding-default", "openai/qa-next"},
		) {
			t.Fatalf("live-only model ids = %#v, want last complete generation", got)
		}
		for _, model := range liveOnly {
			if !model.Stale {
				t.Fatalf("live-only model not stale: %#v", model)
			}
		}
		if after := liveRowRefreshedAt(t, dbPath); !reflect.DeepEqual(step2LiveRows, after) {
			t.Fatalf("stale live rows changed across failure: %#v vs %#v", step2LiveRows, after)
		}
		statuses, err := service.ListSourceStatus(ctx, modelcatalog.StatusOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
		})
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		failed := false
		for _, status := range statuses {
			if status.SourceID != "provider_live:opencode" {
				continue
			}
			failed = status.RefreshState == modelcatalog.RefreshStateFailed &&
				status.LastSuccess.Equal(integrationTime(41))
		}
		if !failed {
			t.Fatalf("source statuses = %#v, want failed with preserved last success", statuses)
		}

		// Step 4: close/reopen keeps stale rows and curation without new detection.
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 45*time.Second)
		if err := store.Close(closeCtx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		closeCancel()
		reopened, err := globaldb.OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			if closeErr := reopened.Close(closeCtx); closeErr != nil {
				t.Errorf("GlobalDB.Close() error = %v", closeErr)
			}
		})
		reopenedService, err := modelcatalog.NewService(
			reopened,
			[]modelcatalog.Source{configSource, liveSource},
			modelcatalog.MergeOptions{ReasoningApply: map[string]bool{"opencode": true}},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if err := reopenedService.SetDefaultExecutionContext(execution); err != nil {
			t.Fatalf("SetDefaultExecutionContext() error = %v", err)
		}
		reopenedModels, err := reopenedService.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
			View:             modelcatalog.CatalogViewAll,
			IncludeStale:     true,
			Now:              integrationTime(44),
		})
		if err != nil {
			t.Fatalf("ListModels(reopened) error = %v", err)
		}
		if got := integrationModelIDs(reopenedModels); !slices.Equal(
			got,
			[]string{"openrouter/acme/coding-default", "openai/qa-next"},
		) {
			t.Fatalf("reopened model ids = %#v, want retained stale generation", got)
		}
		if !reopenedModels[0].Curated {
			t.Fatalf("reopened curation lost: %#v", reopenedModels[0])
		}
		if got := len(openCodeFixtureCalls(t, fixtureDir)); got != 6 {
			t.Fatalf("reopen executed %d discovery calls, want 6", got)
		}

		// Step 5: valid v2 recovers, then v1 dispatch returns without version cache.
		writeOpenCodeFixtureFile(t, fixtureDir, "v2.json", `{"data":[
{"providerID":"openrouter","id":"acme/coding-default","name":"Coding default","enabled":true},
{"providerID":"openai","id":"qa-next","name":"QA Next","enabled":true}]}`)
		serviceRefresh := func(svc *modelcatalog.CatalogService, now time.Time) {
			t.Helper()
			if _, err := svc.Refresh(ctx, modelcatalog.RefreshOptions{
				ProviderID:       "opencode",
				ExecutionContext: execution,
				Force:            true,
				Now:              now,
			}); err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
		}
		serviceRefresh(reopenedService, integrationTime(45))
		fresh, err := reopenedService.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
			View:             modelcatalog.CatalogViewAll,
			IncludeStale:     true,
			Now:              integrationTime(45),
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		for _, model := range fresh {
			if model.Stale {
				t.Fatalf("recovered model stale: %#v", model)
			}
		}
		writeOpenCodeFixtureFile(t, fixtureDir, "version", "opencode v1.18.33\n")
		serviceRefresh(reopenedService, integrationTime(46))
		calls = openCodeFixtureCalls(t, fixtureDir)
		if len(calls) != 10 || calls[8] != "--version" || calls[9] != "models --verbose" {
			t.Fatalf("v1 return calls = %#v, want re-detected version then models --verbose", calls)
		}
		returned, err := reopenedService.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
			View:             modelcatalog.CatalogViewAll,
			IncludeStale:     true,
			Now:              integrationTime(46),
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got := integrationModelIDs(returned); !slices.Equal(got, []string{"openrouter/acme/coding-default"}) {
			t.Fatalf("returned model ids = %#v", got)
		}

		// Step 6: a different selector partitions rows; stale transfer is denied.
		oldFingerprint, err := liveSource.CatalogExecutionFingerprint()
		if err != nil {
			t.Fatalf("CatalogExecutionFingerprint() error = %v", err)
		}
		if got := len(openCodeFixtureCalls(t, fixtureDir)); got != 10 {
			t.Fatalf("fingerprinting executed %d discovery calls, want 10", got)
		}
		nextFixture := writeOpenCodeNativeFixture(t)
		writeOpenCodeFixtureFile(t, nextFixture, "version", "opencode v2.0.32\n")
		writeOpenCodeFixtureFile(t, nextFixture, "v2.json", `{"data":[
{"providerID":"openrouter","id":"acme/coding-default","name":"Coding default","enabled":true},
{"providerID":"openai","id":"qa-next","enabled":"yes"}]}`)
		nextProvider := compozyconfig.ProviderConfig{
			Command: filepath.Join(nextFixture, "bin", "opencode") + " acp",
		}
		nextSource, err := modelcatalog.NewLiveProviderSource(
			"opencode",
			nextProvider,
			&modelcatalog.LiveProviderSourcesConfig{
				BaseEnv: []string{
					"PATH=" + filepath.Join(nextFixture, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
					"HOME=" + homeDir,
					"OPENCODE_FIXTURE_DIR=" + nextFixture,
					"OPENCODE_FIXTURE_LOG=" + filepath.Join(nextFixture, "calls.log"),
				},
				WorkingDir: workDir,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		nextFingerprint, err := nextSource.CatalogExecutionFingerprint()
		if err != nil {
			t.Fatalf("CatalogExecutionFingerprint() error = %v", err)
		}
		if nextFingerprint == oldFingerprint {
			t.Fatal("different selectors share an execution fingerprint")
		}
		if got := len(openCodeFixtureCalls(t, nextFixture)); got != 0 {
			t.Fatalf("fingerprinting executed %d discovery calls, want none", got)
		}
		nextService, err := modelcatalog.NewService(
			reopened,
			[]modelcatalog.Source{configSource, nextSource},
			modelcatalog.MergeOptions{ReasoningApply: map[string]bool{"opencode": true}},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if err := nextService.SetDefaultExecutionContext(execution); err != nil {
			t.Fatalf("SetDefaultExecutionContext() error = %v", err)
		}
		oldContext, err := execution.WithCommandFingerprint(oldFingerprint)
		if err != nil {
			t.Fatalf("WithCommandFingerprint() error = %v", err)
		}
		statuses, refreshErr = nextService.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID:               "opencode",
			ExecutionContext:         execution,
			PreviousExecutionContext: oldContext,
			Force:                    true,
			Now:                      integrationTime(47),
		})
		if refreshErr != nil {
			t.Fatalf("Refresh() error = %v, want masked partial success", refreshErr)
		}
		failedTransfer := false
		for _, status := range statuses {
			if status.SourceID != "provider_live:opencode" {
				continue
			}
			failedTransfer = status.RefreshState == modelcatalog.RefreshStateFailed &&
				strings.Contains(status.LastError, "invalid OpenCode v2 model payload")
		}
		if !failedTransfer {
			t.Fatalf("source statuses = %#v, want failed live source without stale transfer", statuses)
		}
		isolated, err := nextService.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
			View:             modelcatalog.CatalogViewAll,
			IncludeStale:     true,
			Now:              integrationTime(47),
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		for _, model := range isolated {
			for _, ref := range model.Sources {
				if ref.SourceID == "provider_live:opencode" {
					t.Fatalf("new invocation reads transferred live rows: %#v", isolated)
				}
			}
		}
		oldService, err := modelcatalog.NewService(
			reopened,
			[]modelcatalog.Source{configSource, liveSource},
			modelcatalog.MergeOptions{ReasoningApply: map[string]bool{"opencode": true}},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if err := oldService.SetDefaultExecutionContext(execution); err != nil {
			t.Fatalf("SetDefaultExecutionContext() error = %v", err)
		}
		retained, err := oldService.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:       "opencode",
			ExecutionContext: execution,
			View:             modelcatalog.CatalogViewAll,
			IncludeStale:     true,
			Now:              integrationTime(47),
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got := integrationModelIDs(retained); !slices.Equal(got, []string{"openrouter/acme/coding-default"}) {
			t.Fatalf("retained model ids = %#v, want stored old-selector rows", got)
		}
		retainedLiveRows := false
		for _, model := range retained {
			for _, ref := range model.Sources {
				if ref.SourceID == "provider_live:opencode" {
					retainedLiveRows = true
				}
			}
		}
		if !retainedLiveRows {
			t.Fatalf("old-selector live rows were pruned instead of retained: %#v", retained)
		}
	})

	t.Run("Should refresh and list rows by provider with global DB store", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, _ := openCatalogGlobalDB(t)
		source := modelcatalog.NewConfigSource(map[string]compozyconfig.ProviderConfig{
			"codex": {
				Models: compozyconfig.ProviderModelsConfig{
					Default: "manual-model",
					Curated: []compozyconfig.ProviderModelConfig{
						{ID: "gpt-5.4", DisplayName: "GPT-5.4"},
					},
				},
			},
			"claude": {
				Models: compozyconfig.ProviderModelsConfig{
					Default: "claude-sonnet-4-6",
				},
			},
		})
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if _, err := service.Refresh(
			ctx,
			modelcatalog.RefreshOptions{Force: true, Now: integrationTime(0)},
		); err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{ProviderID: "codex", Now: integrationTime(1)})
		if err != nil {
			t.Fatalf("ListModels(codex) error = %v", err)
		}
		if got, want := len(models), 1; got != want {
			t.Fatalf("len(curated models) = %d, want %d: %#v", got, want, models)
		}
		if models[0].ProviderID != "codex" || models[0].ModelID != "gpt-5.4" ||
			!models[0].ExplicitlyCurated || !models[0].Curated {
			t.Fatalf("curated models = %#v, want explicit codex/gpt-5.4", models)
		}
		allModels, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID: "codex",
			View:       modelcatalog.CatalogViewAll,
			Now:        integrationTime(1),
		})
		if err != nil {
			t.Fatalf("ListModels(codex all) error = %v", err)
		}
		if got, want := integrationModelIDs(allModels), []string{"gpt-5.4", "manual-model"}; !slices.Equal(got, want) {
			t.Fatalf("all model ids = %#v, want %#v", got, want)
		}
		if allModels[1].ExplicitlyCurated || allModels[1].Curated {
			t.Fatalf("manual default curation = %#v, want default-only outside curated view", allModels[1])
		}
		claudeModels, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID: "claude",
			Now:        integrationTime(1),
		})
		if err != nil {
			t.Fatalf("ListModels(claude) error = %v", err)
		}
		if len(claudeModels) != 1 || claudeModels[0].ModelID != "claude-sonnet-4-6" ||
			!claudeModels[0].Curated || claudeModels[0].ExplicitlyCurated {
			t.Fatalf("claude fallback models = %#v, want visible default-only fallback", claudeModels)
		}
	})

	t.Run("Should not persist raw models dev upstream payload", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, path := openCatalogGlobalDB(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprint(w, `{
				"openai": {
					"models": {
						"gpt-5.4": {
							"name": "GPT-5.4",
							"unused_raw_marker": "raw_secret_marker_should_not_persist"
						}
					}
				}
			}`)
			if err != nil {
				t.Errorf("Fprint(response) error = %v", err)
			}
		}))
		t.Cleanup(server.Close)
		enabled := true
		source, err := modelcatalog.NewModelsDevSource(nil, compozyconfig.ModelsDevSourceConfig{
			Enabled:  &enabled,
			Endpoint: server.URL,
			TTL:      "1h",
			Timeout:  "1s",
		})
		if err != nil {
			t.Fatalf("NewModelsDevSource() error = %v", err)
		}
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if _, err := service.Refresh(
			ctx,
			modelcatalog.RefreshOptions{ProviderID: "codex", Force: true, Now: integrationTime(0)},
		); err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}

		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := db.Close(); closeErr != nil {
				t.Errorf("db.Close() error = %v", closeErr)
			}
		})
		var matches int
		if err := db.QueryRowContext(
			ctx,
			`SELECT
				(SELECT COUNT(*) FROM model_catalog_rows
				 WHERE source_id LIKE ? OR provider_id LIKE ? OR model_id LIKE ? OR display_name LIKE ? OR last_error LIKE ?)
				+
				(SELECT COUNT(*) FROM model_catalog_sources
				 WHERE source_id LIKE ? OR provider_id LIKE ? OR last_error LIKE ?)`,
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
			"%raw_secret_marker%",
		).Scan(&matches); err != nil {
			t.Fatalf("QueryRowContext(raw marker) error = %v", err)
		}
		if matches != 0 {
			t.Fatalf("raw marker persisted in %d catalog fields, want 0", matches)
		}
	})

	t.Run("Should coalesce same provider refreshes without SQLite busy failures", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, _ := openCatalogGlobalDB(t)
		source := newIntegrationBlockingSource(map[string][]modelcatalog.ModelRow{
			"codex": {
				integrationRow("codex", "gpt-5.4", integrationTime(20)),
			},
		})
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if err := service.SetDefaultExecutionContext(modelcatalog.CatalogExecutionContext{
			Scope: modelcatalog.ExecutionScopeProfile, ProfileID: storepkg.DefaultProfileID,
		}); err != nil {
			t.Fatalf("SetDefaultExecutionContext(): %v", err)
		}

		results := make(chan error, 2)
		for range 2 {
			go func() {
				_, refreshErr := service.Refresh(ctx, modelcatalog.RefreshOptions{
					ProviderID: "codex",
					SourceID:   source.ID(),
					Force:      true,
					Now:        integrationTime(20),
				})
				results <- refreshErr
			}()
		}
		source.waitForCalls(t, 1)
		source.requireCallCountStable(t, 1, 25*time.Millisecond)
		source.release()

		for range 2 {
			if err := <-results; err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{ProviderID: "codex", Now: integrationTime(21)})
		if err != nil {
			t.Fatalf("ListModels(codex) error = %v", err)
		}
		if got, want := integrationModelKeys(models), []string{"codex/gpt-5.4"}; !slices.Equal(got, want) {
			t.Fatalf("model keys = %#v, want %#v", got, want)
		}
	})

	t.Run("Should persist concurrent cross provider refreshes without SQLite busy failures", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, _ := openCatalogGlobalDB(t)
		source := newIntegrationBlockingSource(map[string][]modelcatalog.ModelRow{
			"claude": {
				integrationRow("claude", "claude-sonnet-4-6", integrationTime(30)),
			},
			"codex": {
				integrationRow("codex", "gpt-5.4", integrationTime(30)),
			},
		})
		service, err := modelcatalog.NewService(store, []modelcatalog.Source{source}, modelcatalog.MergeOptions{})
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if err := service.SetDefaultExecutionContext(modelcatalog.CatalogExecutionContext{
			Scope: modelcatalog.ExecutionScopeProfile, ProfileID: storepkg.DefaultProfileID,
		}); err != nil {
			t.Fatalf("SetDefaultExecutionContext(): %v", err)
		}

		results := make(chan error, 2)
		for _, providerID := range []string{"codex", "claude"} {
			go func(providerID string) {
				_, refreshErr := service.Refresh(ctx, modelcatalog.RefreshOptions{
					ProviderID: providerID,
					SourceID:   source.ID(),
					Force:      true,
					Now:        integrationTime(30),
				})
				results <- refreshErr
			}(providerID)
		}
		source.waitForCalls(t, 2)
		source.release()

		for range 2 {
			if err := <-results; err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{Now: integrationTime(31)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := integrationModelKeys(
			models,
		), []string{
			"claude/claude-sonnet-4-6",
			"codex/gpt-5.4",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("model keys = %#v, want %#v", got, want)
		}
	})
}

func openCatalogGlobalDB(t *testing.T) (*globaldb.GlobalDB, string) {
	t.Helper()

	ctx := testutil.Context(t)
	path := filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)
	store, err := globaldb.OpenGlobalDB(ctx, path)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if closeErr := store.Close(closeCtx); closeErr != nil {
			t.Errorf("GlobalDB.Close() error = %v", closeErr)
		}
	})
	return store, path
}

func integrationTime(offset int) time.Time {
	return time.Date(2026, 5, 7, 13, offset, 0, 0, time.UTC)
}

// writeOpenCodeNativeFixture creates a native-executable placeholder whose
// version and catalog output change between requests. Only the external CLI is
// replaced; production resolution, decoders, and SQLite stay authoritative.
func writeOpenCodeNativeFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	script := `#!/bin/sh
printf '%s
' "$*" >> "${OPENCODE_FIXTURE_LOG:?missing log}"
data="${OPENCODE_FIXTURE_DIR:?missing fixture dir}"
case "$1" in
  --version) cat "$data/version";;
  models) cat "$data/v1.txt";;
  api) cat "$data/v2.json";;
  *) echo "unexpected discovery command: $*" >&2; exit 1;;
esac
`
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "opencode"), []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(opencode fixture) error = %v", err)
	}
	return dir
}

func writeOpenCodeFixtureFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}

func openCodeFixtureCalls(t *testing.T, dir string) []string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, "calls.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("ReadFile(calls.log) error = %v", err)
	}
	var calls []string
	for line := range strings.Lines(strings.TrimSpace(string(raw))) {
		calls = append(calls, strings.TrimSpace(line))
	}
	return calls
}

// liveRowRefreshedAt snapshots persisted live-row timestamps without merging
// config-source rows into the projection.
func liveRowRefreshedAt(t *testing.T, dbPath string) map[string]string {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("db.Close() error = %v", closeErr)
		}
	}()
	rows, err := db.QueryContext(
		context.Background(),
		`SELECT model_id, refreshed_at FROM model_catalog_rows
		 WHERE source_id = 'provider_live:opencode' AND provider_id = 'opencode'
		 ORDER BY model_id`,
	)
	if err != nil {
		t.Fatalf("QueryContext() error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("rows.Close() error = %v", closeErr)
		}
	}()
	snapshot := make(map[string]string)
	for rows.Next() {
		var modelID, refreshedAt string
		if err := rows.Scan(&modelID, &refreshedAt); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		snapshot[modelID] = refreshedAt
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	return snapshot
}

type integrationBlockingSource struct {
	mu             sync.Mutex
	rowsByProvider map[string][]modelcatalog.ModelRow
	calls          int
	callsCh        chan int
	releaseCh      chan struct{}
	releaseOnce    sync.Once
}

func newIntegrationBlockingSource(rowsByProvider map[string][]modelcatalog.ModelRow) *integrationBlockingSource {
	return &integrationBlockingSource{
		rowsByProvider: rowsByProvider,
		callsCh:        make(chan int, 16),
		releaseCh:      make(chan struct{}),
	}
}

func (*integrationBlockingSource) CatalogExecutionFingerprint() (string, error) {
	return modelcatalog.CatalogExecutionFingerprint("integration-blocking-source"), nil
}

func (s *integrationBlockingSource) ID() string {
	return "provider_live:integration"
}

func (s *integrationBlockingSource) Kind() modelcatalog.SourceKind {
	return modelcatalog.SourceKindProviderLive
}

func (s *integrationBlockingSource) Priority() int {
	return modelcatalog.PriorityProviderLive
}

func (s *integrationBlockingSource) ProviderIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	providers := make([]string, 0, len(s.rowsByProvider))
	for providerID := range s.rowsByProvider {
		providers = append(providers, providerID)
	}
	slices.Sort(providers)
	return providers
}

func (s *integrationBlockingSource) ListModels(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.ModelRow, error) {
	s.mu.Lock()
	s.calls++
	calls := s.calls
	s.mu.Unlock()
	select {
	case s.callsCh <- calls:
	default:
	}

	select {
	case <-s.releaseCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	s.mu.Lock()
	rows := cloneIntegrationRows(s.rowsByProvider[opts.ProviderID])
	s.mu.Unlock()
	return rows, nil
}

func (s *integrationBlockingSource) waitForCalls(t *testing.T, want int) {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		if s.callCount() >= want {
			return
		}
		select {
		case <-s.callsCh:
		case <-deadline:
			t.Fatalf("source calls = %d, want at least %d", s.callCount(), want)
		}
	}
}

func (s *integrationBlockingSource) requireCallCountStable(
	t *testing.T,
	want int,
	duration time.Duration,
) {
	t.Helper()

	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case <-s.callsCh:
			if got := s.callCount(); got > want {
				t.Fatalf("source calls = %d while first refresh was blocked, want at most %d", got, want)
			}
		case <-timer.C:
			return
		}
	}
}

func (s *integrationBlockingSource) release() {
	s.releaseOnce.Do(func() {
		close(s.releaseCh)
	})
}

func (s *integrationBlockingSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func integrationRow(providerID string, modelID string, refreshedAt time.Time) modelcatalog.ModelRow {
	return modelcatalog.ModelRow{
		SourceID:    "provider_live:integration",
		SourceKind:  modelcatalog.SourceKindProviderLive,
		Priority:    modelcatalog.PriorityProviderLive,
		ProviderID:  providerID,
		ModelID:     modelID,
		RefreshedAt: refreshedAt,
	}
}

func cloneIntegrationRows(rows []modelcatalog.ModelRow) []modelcatalog.ModelRow {
	return append([]modelcatalog.ModelRow(nil), rows...)
}

func integrationModelKeys(models []modelcatalog.Model) []string {
	keys := make([]string, 0, len(models))
	for _, model := range models {
		keys = append(keys, model.ProviderID+"/"+model.ModelID)
	}
	return keys
}

func integrationModelIDs(models []modelcatalog.Model) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ModelID)
	}
	return ids
}
