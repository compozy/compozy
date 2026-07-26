package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestMergeRows(t *testing.T) {
	t.Parallel()

	t.Run("Should let higher priority source win conflicting fields", func(t *testing.T) {
		t.Parallel()

		contextWindowConfig := int64(100)
		contextWindowCatalog := int64(200)
		models := mergeTestRows([]ModelRow{
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.DisplayName = "Catalog GPT"
					row.ContextWindow = &contextWindowCatalog
				},
			),
			testRow("config", SourceKindConfig, PriorityConfig, "codex", "gpt-5.4", testTime(0), func(row *ModelRow) {
				row.DisplayName = "Config GPT"
				row.ContextWindow = &contextWindowConfig
			}),
		})

		model := requireSingleModel(t, models)
		if model.DisplayName != "Config GPT" {
			t.Fatalf("DisplayName = %q, want Config GPT", model.DisplayName)
		}
		if model.ContextWindow == nil || *model.ContextWindow != contextWindowConfig {
			t.Fatalf("ContextWindow = %v, want %d", model.ContextWindow, contextWindowConfig)
		}
	})

	t.Run("Should let provider live priority win over extension priority", func(t *testing.T) {
		t.Parallel()

		liveAvailable := true
		extensionAvailable := false
		models := mergeTestRows([]ModelRow{
			testRow(
				"extension:alpha",
				SourceKindExtension,
				PriorityExtension,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.DisplayName = "Extension GPT"
					row.Available = &extensionAvailable
				},
			),
			testRow(
				"provider_live:codex",
				SourceKindProviderLive,
				PriorityProviderLive,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.DisplayName = "Live GPT"
					row.Available = &liveAvailable
				},
			),
		})

		model := requireSingleModel(t, models)
		if model.DisplayName != "Live GPT" {
			t.Fatalf("DisplayName = %q, want Live GPT", model.DisplayName)
		}
		if model.Available == nil || !*model.Available {
			t.Fatalf("Available = %v, want true", model.Available)
		}
		if model.AvailabilityState != AvailabilityStateAvailableLive {
			t.Fatalf("AvailabilityState = %q, want available_live", model.AvailabilityState)
		}
	})

	t.Run("Should resolve equal priority and freshness by ascending source id", func(t *testing.T) {
		t.Parallel()

		models := mergeTestRows([]ModelRow{
			testRow(
				"extension:b",
				SourceKindExtension,
				PriorityExtension,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.DisplayName = "B Source"
				},
			),
			testRow(
				"extension:a",
				SourceKindExtension,
				PriorityExtension,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.DisplayName = "A Source"
				},
			),
		})

		model := requireSingleModel(t, models)
		if model.DisplayName != "A Source" {
			t.Fatalf("DisplayName = %q, want A Source", model.DisplayName)
		}
		if got, want := model.Sources[0].SourceID, "extension:a"; got != want {
			t.Fatalf("Sources[0].SourceID = %q, want %q", got, want)
		}
	})

	t.Run("Should let lower priority source fill missing metadata", func(t *testing.T) {
		t.Parallel()

		contextWindow := int64(256000)
		costInput := 1.25
		models := mergeTestRows([]ModelRow{
			testRow("config", SourceKindConfig, PriorityConfig, "codex", "gpt-5.4", testTime(0), nil),
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.DisplayName = "Catalog GPT"
					row.ContextWindow = &contextWindow
					row.CostInputPerMillion = &costInput
				},
			),
		})

		model := requireSingleModel(t, models)
		if model.DisplayName != "Catalog GPT" {
			t.Fatalf("DisplayName = %q, want Catalog GPT", model.DisplayName)
		}
		if model.ContextWindow == nil || *model.ContextWindow != contextWindow {
			t.Fatalf("ContextWindow = %v, want %d", model.ContextWindow, contextWindow)
		}
		if model.CostInputPerMillion == nil || *model.CostInputPerMillion != costInput {
			t.Fatalf("CostInputPerMillion = %v, want %f", model.CostInputPerMillion, costInput)
		}
		if model.CostInputSource != SourceKindModelsDev {
			t.Fatalf("CostInputSource = %q, want %q", model.CostInputSource, SourceKindModelsDev)
		}
	})

	t.Run("Should merge five prices with independent field provenance", func(t *testing.T) {
		t.Parallel()

		configInput := 1.0
		configCacheWrite := 3.0
		catalogOutput := 2.0
		catalogCacheRead := 0.5
		catalogReasoning := 4.0
		model := requireSingleModel(t, mergeTestRows([]ModelRow{
			testRow("config", SourceKindConfig, PriorityConfig, "codex", "gpt-5.4", testTime(0), func(row *ModelRow) {
				row.CostInputPerMillion = &configInput
				row.CostCacheWritePerMillion = &configCacheWrite
			}),
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.CostOutputPerMillion = &catalogOutput
					row.CostCacheReadPerMillion = &catalogCacheRead
					row.CostReasoningPerMillion = &catalogReasoning
				},
			),
		}))

		if model.CostInputPerMillion != &configInput || model.CostInputSource != SourceKindConfig ||
			model.CostCacheWritePerMillion != &configCacheWrite || model.CostCacheWriteSource != SourceKindConfig ||
			model.CostOutputPerMillion != &catalogOutput || model.CostOutputSource != SourceKindModelsDev ||
			model.CostCacheReadPerMillion != &catalogCacheRead || model.CostCacheReadSource != SourceKindModelsDev ||
			model.CostReasoningPerMillion != &catalogReasoning || model.CostReasoningSource != SourceKindModelsDev {
			t.Fatalf("merged five-rate provenance = %#v, want independent config/catalog sources", model)
		}
	})

	t.Run("Should keep stale metadata flag when fresh availability wins", func(t *testing.T) {
		t.Parallel()

		available := true
		contextWindow := int64(256000)
		models := mergeTestRows([]ModelRow{
			testRow(
				"provider_live:codex",
				SourceKindProviderLive,
				PriorityProviderLive,
				"codex",
				"gpt-5.4",
				testTime(2),
				func(row *ModelRow) {
					row.Available = &available
				},
			),
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"codex",
				"gpt-5.4",
				testTime(1),
				func(row *ModelRow) {
					row.ContextWindow = &contextWindow
					row.Stale = true
				},
			),
		})

		model := requireSingleModel(t, models)
		if model.ContextWindow == nil || *model.ContextWindow != contextWindow {
			t.Fatalf("ContextWindow = %v, want %d", model.ContextWindow, contextWindow)
		}
		if model.AvailabilityState != AvailabilityStateAvailableLive {
			t.Fatalf("AvailabilityState = %q, want available_live", model.AvailabilityState)
		}
		if !model.Stale {
			t.Fatal("Model.Stale = false, want true because stale metadata contributed to projection")
		}
	})

	t.Run("Should project merged availability states", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name      string
			available bool
			stale     bool
			state     AvailabilityState
		}{
			{name: "Should project stale available live truth", available: true, stale: true, state: AvailabilityStateAvailableStale},
			{name: "Should project fresh available live truth", available: true, stale: false, state: AvailabilityStateAvailableLive},
			{name: "Should project stale unavailable live truth", available: false, stale: true, state: AvailabilityStateUnavailableStale},
			{name: "Should project fresh unavailable live truth", available: false, stale: false, state: AvailabilityStateUnavailableLive},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				models := mergeTestRows([]ModelRow{
					testRow(
						"provider_live:codex",
						SourceKindProviderLive,
						PriorityProviderLive,
						"codex",
						"gpt-5.4",
						testTime(0),
						func(row *ModelRow) {
							row.Available = &tc.available
							row.Stale = tc.stale
						},
					),
				})
				model := requireSingleModel(t, models)
				if model.Available == nil || *model.Available != tc.available {
					t.Fatalf("Available = %v, want %t", model.Available, tc.available)
				}
				if model.AvailabilityState != tc.state {
					t.Fatalf("AvailabilityState = %q, want %q", model.AvailabilityState, tc.state)
				}
			})
		}
	})

	t.Run("Should keep catalog only models at unknown availability", func(t *testing.T) {
		t.Parallel()

		models := mergeTestRows([]ModelRow{
			testRow("models_dev", SourceKindModelsDev, PriorityModelsDev, "codex", "gpt-5.4", testTime(0), nil),
		})
		model := requireSingleModel(t, models)
		if model.Available != nil {
			t.Fatalf("Available = %v, want nil", model.Available)
		}
		if model.AvailabilityState != AvailabilityStateUnknown {
			t.Fatalf("AvailabilityState = %q, want unknown", model.AvailabilityState)
		}
	})

	t.Run("Should sort merged projection and source refs deterministically", func(t *testing.T) {
		t.Parallel()

		models := mergeTestRows([]ModelRow{
			testRow("extension:b", SourceKindExtension, PriorityExtension, "claude", "claude-4", testTime(1), nil),
			testRow(
				"provider_live:codex",
				SourceKindProviderLive,
				PriorityProviderLive,
				"codex",
				"gpt-5.4",
				testTime(0),
				nil,
			),
			testRow("extension:a", SourceKindExtension, PriorityExtension, "codex", "gpt-5.4", testTime(2), nil),
		})
		if got, want := modelKeys(models), []string{"claude/claude-4", "codex/gpt-5.4"}; !slices.Equal(got, want) {
			t.Fatalf("model keys = %#v, want %#v", got, want)
		}
		if got, want := sourceIDs(
			models[1].Sources,
		), []string{
			"provider_live:codex",
			"extension:a",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("source ids = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve support without fabricating selectable efforts", func(t *testing.T) {
		t.Parallel()

		supportsReasoning := true
		models := mergeTestRows([]ModelRow{
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"research",
				"reasoner-1",
				testTime(0),
				func(row *ModelRow) {
					row.SupportsReasoning = &supportsReasoning
				},
			),
		})

		model := requireSingleModel(t, models)
		if model.SupportsReasoning == nil || !*model.SupportsReasoning {
			t.Fatalf("SupportsReasoning = %v, want true", model.SupportsReasoning)
		}
		if len(model.ReasoningEfforts) != 0 {
			t.Fatalf("ReasoningEfforts = %#v, want no fabricated levels", model.ReasoningEfforts)
		}
		if model.DefaultReasoningEffort != nil {
			t.Fatalf("DefaultReasoningEffort = %v, want nil", model.DefaultReasoningEffort)
		}
	})

	t.Run("Should not infer reasoning from model name families", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name     string
			provider string
			model    string
		}{
			{name: "Should keep GPT leaf model unknown", provider: "codex", model: "gpt-5.6-sol"},
			{name: "Should keep namespaced GPT model unknown", provider: "openrouter", model: "openai/gpt-5.6-sol"},
			{name: "Should keep Claude model unknown", provider: "claude", model: "claude-sonnet-5"},
			{name: "Should keep namespaced Claude model unknown", provider: "openrouter", model: "anthropic/claude-opus-4-8"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				models := mergeTestRows([]ModelRow{
					testRow(
						"models_dev",
						SourceKindModelsDev,
						PriorityModelsDev,
						tc.provider,
						tc.model,
						testTime(0),
						nil,
					),
				})

				model := requireSingleModel(t, models)
				if model.SupportsReasoning != nil {
					t.Fatalf("SupportsReasoning = %v, want nil", model.SupportsReasoning)
				}
				if len(model.ReasoningEfforts) != 0 {
					t.Fatalf("ReasoningEfforts = %#v, want empty", model.ReasoningEfforts)
				}
			})
		}
	})

	t.Run("Should keep explicit reasoning support false disabled", func(t *testing.T) {
		t.Parallel()

		supportsReasoning := false
		defaultEffort := ReasoningEffortHigh
		models := mergeTestRows([]ModelRow{
			testRow(
				"config",
				SourceKindConfig,
				PriorityConfig,
				"codex",
				"gpt-5.5",
				testTime(0),
				func(row *ModelRow) {
					row.SupportsReasoning = &supportsReasoning
				},
			),
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"codex",
				"gpt-5.5",
				testTime(0),
				func(row *ModelRow) {
					row.ReasoningEfforts = []ReasoningEffort{ReasoningEffortLow, ReasoningEffortHigh}
					row.DefaultReasoningEffort = &defaultEffort
				},
			),
		})

		model := requireSingleModel(t, models)
		if model.SupportsReasoning == nil || *model.SupportsReasoning {
			t.Fatalf("SupportsReasoning = %v, want false", model.SupportsReasoning)
		}
		if len(model.ReasoningEfforts) != 0 {
			t.Fatalf("ReasoningEfforts = %#v, want empty when support is false", model.ReasoningEfforts)
		}
		if model.DefaultReasoningEffort != nil {
			t.Fatalf("DefaultReasoningEffort = %v, want nil when support is false", *model.DefaultReasoningEffort)
		}
	})

	t.Run("Should let higher priority reasoning efforts imply support over lower priority false", func(t *testing.T) {
		t.Parallel()

		supportsReasoning := false
		defaultEffort := ReasoningEffortHigh
		models := mergeTestRows([]ModelRow{
			testRow(
				"config",
				SourceKindConfig,
				PriorityConfig,
				"vendor",
				"reasoner-priority",
				testTime(0),
				func(row *ModelRow) {
					row.ReasoningEfforts = []ReasoningEffort{ReasoningEffortLow, ReasoningEffortHigh}
					row.DefaultReasoningEffort = &defaultEffort
				},
			),
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"vendor",
				"reasoner-priority",
				testTime(0),
				func(row *ModelRow) {
					row.SupportsReasoning = &supportsReasoning
				},
			),
		})

		model := requireSingleModel(t, models)
		requireReasoningProfile(
			t,
			model,
			true,
			[]ReasoningEffort{ReasoningEffortLow, ReasoningEffortHigh},
			ReasoningEffortHigh,
		)
	})

	t.Run("Should preserve explicit reasoning efforts and default", func(t *testing.T) {
		t.Parallel()

		supportsReasoning := true
		defaultEffort := ReasoningEffortHigh
		models := mergeTestRows([]ModelRow{
			testRow(
				"config",
				SourceKindConfig,
				PriorityConfig,
				"vendor",
				"reasoner-explicit",
				testTime(0),
				func(row *ModelRow) {
					row.SupportsReasoning = &supportsReasoning
					row.ReasoningEfforts = []ReasoningEffort{ReasoningEffortLow, ReasoningEffortHigh}
					row.DefaultReasoningEffort = &defaultEffort
				},
			),
		})

		model := requireSingleModel(t, models)
		requireReasoningProfile(
			t,
			model,
			true,
			[]ReasoningEffort{ReasoningEffortLow, ReasoningEffortHigh},
			ReasoningEffortHigh,
		)
	})

	t.Run("Should overlay a config default without freezing the lower reasoning profile", func(t *testing.T) {
		t.Parallel()

		supportsReasoning := true
		builtinDefault := ReasoningEffortHigh
		configDefault := ReasoningEffortMax
		models := mergeTestRows([]ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"codex",
				"gpt-5.6-sol",
				testTime(1),
				func(row *ModelRow) {
					row.DefaultReasoningEffort = &configDefault
				},
			),
			testRow(
				SourceIDBuiltin,
				SourceKindBuiltin,
				PriorityBuiltin,
				"codex",
				"gpt-5.6-sol",
				testTime(0),
				func(row *ModelRow) {
					row.SupportsReasoning = &supportsReasoning
					row.ReasoningEfforts = []ReasoningEffort{
						ReasoningEffortNone,
						ReasoningEffortHigh,
						ReasoningEffortMax,
					}
					row.DefaultReasoningEffort = &builtinDefault
				},
			),
		})

		model := requireSingleModel(t, models)
		requireReasoningProfile(
			t,
			model,
			true,
			[]ReasoningEffort{ReasoningEffortNone, ReasoningEffortHigh, ReasoningEffortMax},
			ReasoningEffortMax,
		)
	})

	t.Run("Should leave unknown models without support signal disabled", func(t *testing.T) {
		t.Parallel()

		models := mergeTestRows([]ModelRow{
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"vendor",
				"custom-chat-model",
				testTime(0),
				nil,
			),
		})

		model := requireSingleModel(t, models)
		if model.SupportsReasoning != nil {
			t.Fatalf("SupportsReasoning = %v, want nil", model.SupportsReasoning)
		}
		if len(model.ReasoningEfforts) != 0 {
			t.Fatalf("ReasoningEfforts = %#v, want empty", model.ReasoningEfforts)
		}
		if model.DefaultReasoningEffort != nil {
			t.Fatalf("DefaultReasoningEffort = %v, want nil", *model.DefaultReasoningEffort)
		}
	})

	t.Run("Should let an observed empty live profile override builtin efforts", func(t *testing.T) {
		t.Parallel()

		supportsReasoning := true
		defaultEffort := ReasoningEffortMedium
		models := mergeTestRows([]ModelRow{
			testRow(
				"provider_live:claude",
				SourceKindProviderLive,
				PriorityProviderLive,
				"claude",
				"claude-haiku-4-5-20251001",
				testTime(1),
				func(row *ModelRow) { row.SupportsReasoning = &supportsReasoning },
			),
			testRow(
				SourceIDBuiltin,
				SourceKindBuiltin,
				PriorityBuiltin,
				"claude",
				"claude-haiku-4-5-20251001",
				testTime(0),
				func(row *ModelRow) {
					row.SupportsReasoning = &supportsReasoning
					row.ReasoningEfforts = []ReasoningEffort{ReasoningEffortLow, ReasoningEffortMedium}
					row.DefaultReasoningEffort = &defaultEffort
				},
			),
		})

		model := requireSingleModel(t, models)
		if model.SupportsReasoning == nil || !*model.SupportsReasoning {
			t.Fatalf("SupportsReasoning = %v, want true", model.SupportsReasoning)
		}
		if len(model.ReasoningEfforts) != 0 || model.DefaultReasoningEffort != nil {
			t.Fatalf(
				"reasoning profile = %#v/%v, want observed empty profile",
				model.ReasoningEfforts,
				model.DefaultReasoningEffort,
			)
		}
	})

	t.Run("Should suppress selectable efforts when provider strategy is none", func(t *testing.T) {
		t.Parallel()

		supportsReasoning := true
		defaultEffort := ReasoningEffortMax
		models := MergeRows([]ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"custom",
				"reasoner",
				testTime(0),
				func(row *ModelRow) {
					row.SupportsReasoning = &supportsReasoning
					row.ReasoningEfforts = []ReasoningEffort{ReasoningEffortNone, ReasoningEffortMax}
					row.DefaultReasoningEffort = &defaultEffort
				},
			),
		}, MergeOptions{ReasoningApply: map[string]bool{"custom": false}})

		model := requireSingleModel(t, models)
		if model.SupportsReasoning == nil || !*model.SupportsReasoning {
			t.Fatalf("SupportsReasoning = %v, want true", model.SupportsReasoning)
		}
		if len(model.ReasoningEfforts) != 0 || model.DefaultReasoningEffort != nil {
			t.Fatalf(
				"reasoning profile = %#v/%v, want no selectable efforts",
				model.ReasoningEfforts,
				model.DefaultReasoningEffort,
			)
		}
	})
}

func TestCatalogViews(t *testing.T) {
	t.Parallel()

	t.Run("Should suppress noncanonical Cursor aliases from the public catalog", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		store.rows[sourceProviderKey(SourceIDBuiltin, "cursor")] = []ModelRow{
			testRow(
				SourceIDBuiltin,
				SourceKindBuiltin,
				PriorityBuiltin,
				"cursor",
				"grok-4.5[effort=high,fast=true]",
				testTime(0),
				func(row *ModelRow) { row.ExplicitlyCurated = true },
			),
		}
		store.rows[sourceProviderKey(SourceIDConfig, "cursor")] = []ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"cursor",
				"cursor-grok-4.5-high",
				testTime(0),
				func(row *ModelRow) { row.ExplicitlyCurated = true },
			),
		}
		service := newTestService(t, store, nil)
		models, err := service.ListModels(testutil.Context(t), ListOptions{
			ProviderID:   "cursor",
			View:         CatalogViewAll,
			IncludeAll:   true,
			IncludeStale: true,
			Now:          testTime(1),
		})
		if err != nil {
			t.Fatalf("ListModels(cursor) error = %v", err)
		}
		want := []string{"cursor/grok-4.5[effort=high,fast=true]"}
		if got := modelKeys(models); !slices.Equal(got, want) {
			t.Fatalf("Cursor catalog models = %#v, want %#v", got, want)
		}
	})

	t.Run("Should curate candidates and preserve hidden deprecated rows in all view", func(t *testing.T) {
		t.Parallel()

		available := true
		rows := []ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"curated",
				"configured",
				testTime(0),
				func(row *ModelRow) { row.ExplicitlyCurated = true },
			),
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"curated",
				"hidden",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Hidden = new(true)
				},
			),
			testRow(
				SourceIDModelsDev,
				SourceKindModelsDev,
				PriorityModelsDev,
				"curated",
				"deprecated",
				testTime(0),
				func(row *ModelRow) { row.Deprecated = new(true) },
			),
			testRow(
				SourceIDModelsDev,
				SourceKindModelsDev,
				PriorityModelsDev,
				"curated",
				"featured",
				testTime(0),
				func(row *ModelRow) { row.Featured = new(true) },
			),
			testRow(
				"provider_live:curated",
				SourceKindProviderLive,
				PriorityProviderLive,
				"curated",
				"live",
				testTime(0),
				func(row *ModelRow) { row.Available = &available },
			),
			testRow(SourceIDModelsDev, SourceKindModelsDev, PriorityModelsDev, "curated", "other", testTime(0), nil),
			testRow(SourceIDModelsDev, SourceKindModelsDev, PriorityModelsDev, "fallback", "visible", testTime(0), nil),
			testRow(
				SourceIDModelsDev,
				SourceKindModelsDev,
				PriorityModelsDev,
				"fallback",
				"hidden",
				testTime(0),
				func(row *ModelRow) { row.Hidden = new(true) },
			),
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"all-hidden",
				"explicit",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Hidden = new(true)
				},
			),
			testRow(
				"provider_live:all-hidden",
				SourceKindProviderLive,
				PriorityProviderLive,
				"all-hidden",
				"live",
				testTime(0),
				func(row *ModelRow) { row.Available = &available },
			),
		}
		merged := MergeRows(rows, MergeOptions{})
		all, err := applyCatalogView(append([]Model(nil), merged...), CatalogViewAll)
		if err != nil {
			t.Fatalf("applyCatalogView(all) error = %v", err)
		}
		if got, want := len(all), len(rows); got != want {
			t.Fatalf("len(all) = %d, want %d", got, want)
		}
		wantMembership := map[string]bool{
			"all-hidden/explicit": false,
			"all-hidden/live":     false,
			"curated/configured":  true,
			"curated/deprecated":  false,
			"curated/featured":    true,
			"curated/hidden":      false,
			"curated/live":        false,
			"curated/other":       false,
			"fallback/hidden":     false,
			"fallback/visible":    true,
		}
		for _, model := range all {
			key := model.ProviderID + "/" + model.ModelID
			if model.Curated != wantMembership[key] {
				t.Fatalf("all view model %q Curated = %v, want %v", key, model.Curated, wantMembership[key])
			}
		}
		curated, err := applyCatalogView(append([]Model(nil), merged...), CatalogViewCurated)
		if err != nil {
			t.Fatalf("applyCatalogView(curated) error = %v", err)
		}
		if got, want := modelKeys(curated), []string{
			"curated/featured",
			"curated/configured",
			"fallback/visible",
		}; !slices.Equal(got, want) {
			t.Fatalf("curated models = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject an unknown view", func(t *testing.T) {
		t.Parallel()

		_, err := applyCatalogView(nil, CatalogView("recent"))
		var invalid *InvalidViewError
		if !errors.As(err, &invalid) {
			t.Fatalf("applyCatalogView() error = %v, want InvalidViewError", err)
		}
	})

	t.Run("Should rank curated models before newer or excluded featured models", func(t *testing.T) {
		t.Parallel()

		rows := []ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"ranked",
				"curated-featured",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Featured = new(true)
					row.ReleaseDate = new("2024-01-01")
				},
			),
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"ranked",
				"curated-old",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.ReleaseDate = new("2025-01-01")
				},
			),
			testRow(
				SourceIDModelsDev,
				SourceKindModelsDev,
				PriorityModelsDev,
				"ranked",
				"hidden-featured",
				testTime(0),
				func(row *ModelRow) {
					row.Featured = new(true)
					row.Hidden = new(true)
					row.ReleaseDate = new("2027-01-01")
				},
			),
			testRow(
				SourceIDModelsDev,
				SourceKindModelsDev,
				PriorityModelsDev,
				"ranked",
				"non-curated-new",
				testTime(0),
				func(row *ModelRow) { row.ReleaseDate = new("2028-01-01") },
			),
			testRow(
				SourceIDModelsDev,
				SourceKindModelsDev,
				PriorityModelsDev,
				"ranked",
				"deprecated-featured",
				testTime(0),
				func(row *ModelRow) {
					row.Deprecated = new(true)
					row.Featured = new(true)
					row.ReleaseDate = new("2029-01-01")
				},
			),
		}
		all, err := applyCatalogView(MergeRows(rows, MergeOptions{}), CatalogViewAll)
		if err != nil {
			t.Fatalf("applyCatalogView(all) error = %v", err)
		}
		if got, want := modelKeys(all), []string{
			"ranked/curated-featured",
			"ranked/curated-old",
			"ranked/hidden-featured",
			"ranked/non-curated-new",
			"ranked/deprecated-featured",
		}; !slices.Equal(got, want) {
			t.Fatalf("ranked all-view models = %#v, want %#v", got, want)
		}
		if all[2].Curated || !all[2].Hidden || !all[2].Featured {
			t.Fatalf("excluded featured model = %#v, want non-curated hidden featured", all[2])
		}
	})

	t.Run("Should let explicit config override lower-priority curation flags", func(t *testing.T) {
		t.Parallel()

		explicitFalse := false
		models := MergeRows([]ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"codex",
				"gpt-5.6-sol",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Deprecated = &explicitFalse
					row.Hidden = &explicitFalse
					row.Featured = &explicitFalse
				},
			),
			testRow(
				SourceIDBuiltin,
				SourceKindBuiltin,
				PriorityBuiltin,
				"codex",
				"gpt-5.6-sol",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Deprecated = new(true)
					row.Hidden = new(true)
					row.Featured = new(true)
				},
			),
		}, MergeOptions{})

		model := requireSingleModel(t, models)
		if model.Deprecated || model.Hidden || model.Featured {
			t.Fatalf(
				"curation flags = deprecated:%v hidden:%v featured:%v, want explicit false overrides",
				model.Deprecated,
				model.Hidden,
				model.Featured,
			)
		}
	})

	t.Run("Should let a higher non-config source explicitly clear lower curation flags", func(t *testing.T) {
		t.Parallel()

		explicitFalse := false
		models := MergeRows([]ModelRow{
			testRow(
				"extension:current",
				SourceKindExtension,
				PriorityExtension,
				"custom",
				"model",
				testTime(1),
				func(row *ModelRow) {
					row.Deprecated = &explicitFalse
					row.Hidden = &explicitFalse
					row.Featured = &explicitFalse
				},
			),
			testRow(
				SourceIDBuiltin,
				SourceKindBuiltin,
				PriorityBuiltin,
				"custom",
				"model",
				testTime(0),
				func(row *ModelRow) {
					row.Deprecated = new(true)
					row.Hidden = new(true)
					row.Featured = new(true)
				},
			),
		}, MergeOptions{})

		model := requireSingleModel(t, models)
		if model.Deprecated || model.Hidden || model.Featured {
			t.Fatalf("curation flags = %#v, want higher extension false values", model)
		}
	})

	t.Run("Should preserve omitted flags while applying selective config curation", func(t *testing.T) {
		t.Parallel()

		defaultEffort := ReasoningEffortMax
		model := requireSingleModel(t, MergeRows([]ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"codex",
				"gpt-5.6-sol",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Hidden = new(true)
					row.DefaultReasoningEffort = &defaultEffort
				},
			),
			testRow(
				SourceIDModelsDev,
				SourceKindModelsDev,
				PriorityModelsDev,
				"codex",
				"gpt-5.6-sol",
				testTime(0),
				func(row *ModelRow) { row.Deprecated = new(true) },
			),
			testRow(
				SourceIDBuiltin,
				SourceKindBuiltin,
				PriorityBuiltin,
				"codex",
				"gpt-5.6-sol",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Featured = new(true)
				},
			),
		}, MergeOptions{}))
		if !model.Deprecated || !model.Hidden || !model.Featured {
			t.Fatalf("merged curation metadata = %#v, want upstream deprecated/featured plus config hidden", model)
		}
	})

	t.Run("Should preserve lower curation metadata for a config default outside curated", func(t *testing.T) {
		t.Parallel()

		model := requireSingleModel(t, MergeRows([]ModelRow{
			testRow(SourceIDConfig, SourceKindConfig, PriorityConfig, "codex", "gpt-5.6-sol", testTime(0), nil),
			testRow(
				SourceIDBuiltin,
				SourceKindBuiltin,
				PriorityBuiltin,
				"codex",
				"gpt-5.6-sol",
				testTime(0),
				func(row *ModelRow) {
					row.ExplicitlyCurated = true
					row.Deprecated = new(true)
					row.Hidden = new(true)
					row.Featured = new(true)
				},
			),
		}, MergeOptions{}))
		if !model.ExplicitlyCurated || !model.Deprecated || !model.Hidden || !model.Featured {
			t.Fatalf("merged curation metadata = %#v, want builtin metadata preserved", model)
		}
	})

	t.Run("Should preserve explicit membership while filtering by a live source", func(t *testing.T) {
		t.Parallel()

		available := true
		store := newMemoryStore()
		store.rows[sourceProviderKey(SourceIDConfig, "codex")] = []ModelRow{
			testRow(
				SourceIDConfig,
				SourceKindConfig,
				PriorityConfig,
				"codex",
				"alpha",
				testTime(0),
				func(row *ModelRow) { row.ExplicitlyCurated = true },
			),
		}
		liveSourceID := "provider_live:codex"
		store.rows[sourceProviderKey(liveSourceID, "codex")] = []ModelRow{
			testRow(
				liveSourceID,
				SourceKindProviderLive,
				PriorityProviderLive,
				"codex",
				"alpha",
				testTime(0),
				func(row *ModelRow) { row.Available = &available },
			),
			testRow(
				liveSourceID,
				SourceKindProviderLive,
				PriorityProviderLive,
				"codex",
				"beta",
				testTime(0),
				func(row *ModelRow) { row.Available = &available },
			),
		}
		service := newTestService(t, store, nil)
		models, err := service.ListModels(testutil.Context(t), ListOptions{
			ProviderID: "codex",
			SourceID:   liveSourceID,
			View:       CatalogViewCurated,
			Now:        testTime(1),
		})
		if err != nil {
			t.Fatalf("ListModels(curated live source) error = %v", err)
		}
		if got, want := modelKeys(models), []string{"codex/alpha"}; !slices.Equal(got, want) {
			t.Fatalf("curated live-source models = %#v, want %#v", got, want)
		}
	})
}

func TestCatalogServiceRefresh(t *testing.T) {
	t.Parallel()

	t.Run("Should respect stale filters when listing merged models", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		store.rows[sourceProviderKey("models_dev", "codex")] = []ModelRow{
			testRow(
				"models_dev",
				SourceKindModelsDev,
				PriorityModelsDev,
				"codex",
				"gpt-5.4",
				testTime(0),
				func(row *ModelRow) {
					row.Stale = true
				},
			),
		}
		service := newTestService(t, store, nil)

		models, err := service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Now: testTime(1)},
		)
		if err != nil {
			t.Fatalf("ListModels(exclude stale) error = %v", err)
		}
		if len(models) != 0 {
			t.Fatalf("ListModels(exclude stale) = %#v, want empty projection", models)
		}

		models, err = service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", IncludeStale: true, Now: testTime(1)},
		)
		if err != nil {
			t.Fatalf("ListModels(include stale) error = %v", err)
		}
		model := requireSingleModel(t, models)
		if !model.Stale {
			t.Fatalf("Model.Stale = %t, want true", model.Stale)
		}
	})

	t.Run("Should return partial success and record failed source status", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		service := newTestService(t, store, []Source{
			&fakeSource{
				id:        "config",
				kind:      SourceKindConfig,
				priority:  PriorityConfig,
				providers: []string{"codex"},
				rows: []ModelRow{
					testRow("config", SourceKindConfig, PriorityConfig, "codex", "gpt-5.4", testTime(0), nil),
				},
			},
			&fakeSource{
				id:        "models_dev",
				kind:      SourceKindModelsDev,
				priority:  PriorityModelsDev,
				providers: []string{"codex"},
				err:       fmt.Errorf("upstream failed with api_key=super-secret"),
			},
		})

		models, err := service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(10)},
		)
		if err != nil {
			t.Fatalf("ListModels(refresh) error = %v", err)
		}
		if got, want := modelKeys(models), []string{"codex/gpt-5.4"}; !slices.Equal(got, want) {
			t.Fatalf("model keys = %#v, want %#v", got, want)
		}
		statuses, err := service.ListSourceStatus(testutil.Context(t), "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		failed := requireStatus(t, statuses, "models_dev")
		if failed.RefreshState != RefreshStateFailed {
			t.Fatalf("RefreshState = %q, want failed", failed.RefreshState)
		}
		if strings.Contains(failed.LastError, "super-secret") || !strings.Contains(failed.LastError, "[REDACTED]") {
			t.Fatalf("LastError = %q, want redacted secret", failed.LastError)
		}
	})

	t.Run("Should fail all source failure when no stale rows exist", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		service := newTestService(t, store, []Source{
			&fakeSource{
				id:        "models_dev",
				kind:      SourceKindModelsDev,
				priority:  PriorityModelsDev,
				providers: []string{"codex"},
				err:       errors.New("models.dev down api_key=super-secret"),
			},
		})

		_, err := service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(0)},
		)
		if !errors.Is(err, ErrAllSourcesFailed) {
			t.Fatalf("ListModels() error = %v, want ErrAllSourcesFailed", err)
		}
		if !strings.Contains(err.Error(), "models.dev down") {
			t.Fatalf("ListModels() error = %v, want redacted source failure detail", err)
		}
		if strings.Contains(err.Error(), "super-secret") || !strings.Contains(err.Error(), "[REDACTED]") {
			t.Fatalf("ListModels() error = %v, want secret redaction", err)
		}
	})

	t.Run("Should refresh only sources that own the requested provider", func(t *testing.T) {
		t.Parallel()

		claudeSource := &fakeSource{
			id:        "provider_live:claude",
			kind:      SourceKindProviderLive,
			priority:  PriorityProviderLive,
			providers: []string{"claude"},
			rows: []ModelRow{
				testRow(
					"provider_live:claude",
					SourceKindProviderLive,
					PriorityProviderLive,
					"claude",
					"claude-sonnet-5",
					testTime(0),
					nil,
				),
			},
		}
		codexSource := &fakeSource{
			id:        "provider_live:codex",
			kind:      SourceKindProviderLive,
			priority:  PriorityProviderLive,
			providers: []string{"codex"},
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{claudeSource, codexSource})

		statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{
			ProviderID: "claude",
			Force:      true,
			Now:        testTime(1),
		})
		if err != nil {
			t.Fatalf("Refresh(claude) error = %v", err)
		}
		if got, want := claudeSource.calls, 1; got != want {
			t.Fatalf("claude source calls = %d, want %d", got, want)
		}
		if got := codexSource.calls; got != 0 {
			t.Fatalf("codex source calls = %d, want 0", got)
		}
		if got, want := len(statuses), 1; got != want {
			t.Fatalf("len(statuses) = %d, want %d: %#v", got, want, statuses)
		}
		if got, want := statuses[0].SourceID, "provider_live:claude"; got != want {
			t.Fatalf("status source = %q, want %q", got, want)
		}
		storedStatuses, err := service.ListSourceStatus(testutil.Context(t), "claude")
		if err != nil {
			t.Fatalf("ListSourceStatus(claude) error = %v", err)
		}
		if got, want := len(storedStatuses), 1; got != want {
			t.Fatalf("len(stored statuses) = %d, want %d: %#v", got, want, storedStatuses)
		}
	})

	t.Run("Should clear stale stored ownership for a provider no longer declared by a source", func(t *testing.T) {
		t.Parallel()

		claudeSource := &fakeSource{
			id:        "provider_live:claude",
			kind:      SourceKindProviderLive,
			priority:  PriorityProviderLive,
			providers: []string{"claude"},
			rows: []ModelRow{
				testRow(
					"provider_live:claude",
					SourceKindProviderLive,
					PriorityProviderLive,
					"claude",
					"claude-sonnet-5",
					testTime(0),
					nil,
				),
			},
		}
		codexSource := &fakeSource{
			id:        "provider_live:codex",
			kind:      SourceKindProviderLive,
			priority:  PriorityProviderLive,
			providers: []string{"codex"},
		}
		store := newMemoryStore()
		store.statuses[sourceProviderKey(codexSource.ID(), "claude")] = SourceStatus{
			SourceID:     codexSource.ID(),
			SourceKind:   codexSource.Kind(),
			ProviderID:   "claude",
			RefreshState: RefreshStateSucceeded,
			RowCount:     1,
		}
		store.rows[sourceProviderKey(codexSource.ID(), "claude")] = []ModelRow{
			testRow(
				codexSource.ID(),
				codexSource.Kind(),
				codexSource.Priority(),
				"claude",
				"stale-claude-model",
				testTime(0),
				nil,
			),
		}
		service := newTestService(t, store, []Source{claudeSource, codexSource})

		statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{
			ProviderID: "claude",
			Force:      true,
			Now:        testTime(1),
		})
		if err != nil {
			t.Fatalf("Refresh(claude) error = %v", err)
		}
		if got, want := claudeSource.calls, 1; got != want {
			t.Fatalf("claude source calls = %d, want %d", got, want)
		}
		if got, want := codexSource.calls, 1; got != want {
			t.Fatalf("codex source calls = %d, want %d to clear stored ownership", got, want)
		}
		if got, want := len(statuses), 2; got != want {
			t.Fatalf("len(statuses) = %d, want %d: %#v", got, want, statuses)
		}
		staleRows, err := store.ListRows(testutil.Context(t), ListOptions{
			ProviderID:   "claude",
			SourceID:     codexSource.ID(),
			IncludeAll:   true,
			IncludeStale: true,
			Now:          testTime(1),
		})
		if err != nil {
			t.Fatalf("ListRows(codex/claude) error = %v", err)
		}
		if len(staleRows) != 0 {
			t.Fatalf("ListRows(codex/claude) = %#v, want cleared", staleRows)
		}
		storedStatuses, err := service.ListSourceStatus(testutil.Context(t), "claude")
		if err != nil {
			t.Fatalf("ListSourceStatus(claude) error = %v", err)
		}
		if got, want := len(storedStatuses), 1; got != want {
			t.Fatalf("len(stored statuses) = %d, want %d: %#v", got, want, storedStatuses)
		}
		if got, want := storedStatuses[0].SourceID, "provider_live:claude"; got != want {
			t.Fatalf("stored status source = %q, want %q", got, want)
		}
	})

	t.Run("Should return stale rows when refresh fails after prior success", func(t *testing.T) {
		t.Parallel()

		available := true
		source := &fakeSource{
			id:        "provider_live:codex",
			kind:      SourceKindProviderLive,
			priority:  PriorityProviderLive,
			providers: []string{"codex"},
			rows: []ModelRow{
				testRow(
					"provider_live:codex",
					SourceKindProviderLive,
					PriorityProviderLive,
					"codex",
					"gpt-5.4",
					testTime(0),
					func(row *ModelRow) {
						row.Available = &available
					},
				),
			},
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		if _, err := service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(1)},
		); err != nil {
			t.Fatalf("ListModels(first refresh) error = %v", err)
		}

		source.rows = nil
		source.err = errors.New("live source unavailable sk-secret-token")
		models, err := service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, Now: testTime(2)},
		)
		if err != nil {
			t.Fatalf("ListModels(exclude stale refresh) error = %v", err)
		}
		if len(models) != 0 {
			t.Fatalf("ListModels(exclude stale refresh) = %#v, want empty projection", models)
		}

		models, err = service.ListModels(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", Refresh: true, IncludeStale: true, Now: testTime(2)},
		)
		if err != nil {
			t.Fatalf("ListModels(include stale refresh) error = %v", err)
		}
		model := requireSingleModel(t, models)
		if model.AvailabilityState != AvailabilityStateAvailableStale {
			t.Fatalf("AvailabilityState = %q, want available_stale", model.AvailabilityState)
		}
		if !model.Stale {
			t.Fatal("Model.Stale = false, want true")
		}
		if strings.Contains(model.LastError, "sk-secret-token") || !strings.Contains(model.LastError, "[REDACTED]") {
			t.Fatalf("LastError = %q, want redacted stale error", model.LastError)
		}
		statuses, err := service.ListSourceStatus(testutil.Context(t), "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		status := requireStatus(t, statuses, "provider_live:codex")
		if !status.LastSuccess.Equal(testTime(1)) {
			t.Fatalf("LastSuccess = %s, want first refresh time %s", status.LastSuccess, testTime(1))
		}
	})

	t.Run("Should reject invalid extension source id before persistence", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		_, err := NewService(store, []Source{
			&fakeSource{id: "extension:BadSlug", kind: SourceKindExtension, priority: PriorityExtension},
		}, MergeOptions{})
		if err == nil {
			t.Fatal("NewService(invalid extension source) error = nil, want validation error")
		}
		if store.replaceCount != 0 {
			t.Fatalf("replaceCount = %d, want 0", store.replaceCount)
		}
	})

	t.Run("Should skip fresh provider-scoped source statuses during global refresh", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{
			id:        "provider_live:shared",
			kind:      SourceKindProviderLive,
			priority:  PriorityProviderLive,
			providers: []string{"claude", "codex"},
			ttl:       time.Hour,
			rows: []ModelRow{
				testRow(
					"provider_live:shared",
					SourceKindProviderLive,
					PriorityProviderLive,
					"claude",
					"claude-sonnet-4-6",
					testTime(40),
					nil,
				),
				testRow(
					"provider_live:shared",
					SourceKindProviderLive,
					PriorityProviderLive,
					"codex",
					"gpt-5.4",
					testTime(40),
					nil,
				),
			},
		}
		store := newMemoryStore()
		store.statuses[sourceProviderKey(source.ID(), "claude")] = SourceStatus{
			SourceID:     source.ID(),
			SourceKind:   source.Kind(),
			ProviderID:   "claude",
			RefreshState: RefreshStateSucceeded,
			NextRefresh:  testTime(41),
		}
		store.statuses[sourceProviderKey(source.ID(), "codex")] = SourceStatus{
			SourceID:     source.ID(),
			SourceKind:   source.Kind(),
			ProviderID:   "codex",
			RefreshState: RefreshStateSucceeded,
			NextRefresh:  testTime(41),
		}
		service := newTestService(t, store, []Source{source})

		statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{Now: testTime(40)})
		if err != nil {
			t.Fatalf("Refresh(global fresh) error = %v", err)
		}
		if got, want := source.calls, 0; got != want {
			t.Fatalf("source calls = %d, want %d when statuses are still fresh", got, want)
		}
		if got, want := len(statuses), 2; got != want {
			t.Fatalf("len(statuses) = %d, want %d: %#v", got, want, statuses)
		}
	})

	t.Run("Should clear providers removed from an authoritative source during global refresh", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{
			id:        "config",
			kind:      SourceKindConfig,
			priority:  PriorityConfig,
			providers: []string{"alpha", "beta"},
			rows: []ModelRow{
				testRow("config", SourceKindConfig, PriorityConfig, "alpha", "alpha-model", testTime(42), nil),
				testRow("config", SourceKindConfig, PriorityConfig, "beta", "beta-model", testTime(42), nil),
			},
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		if _, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(42)}); err != nil {
			t.Fatalf("Refresh(initial global) error = %v", err)
		}
		if got, want := source.calls, 1; got != want {
			t.Fatalf("source calls after initial global refresh = %d, want %d", got, want)
		}

		source.providers = []string{"beta"}
		source.rows = []ModelRow{
			testRow("config", SourceKindConfig, PriorityConfig, "beta", "beta-model", testTime(43), nil),
		}
		if _, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(43)}); err != nil {
			t.Fatalf("Refresh(after provider removal) error = %v", err)
		}
		if got, want := source.calls, 2; got != want {
			t.Fatalf("source calls after two global refreshes = %d, want %d", got, want)
		}

		alphaRows, err := store.ListRows(testutil.Context(t), ListOptions{
			ProviderID:   "alpha",
			SourceID:     "config",
			IncludeAll:   true,
			IncludeStale: true,
			Now:          testTime(43),
		})
		if err != nil {
			t.Fatalf("ListRows(alpha) error = %v", err)
		}
		if len(alphaRows) != 0 {
			t.Fatalf("ListRows(alpha) = %#v, want removed provider rows cleared", alphaRows)
		}
		betaRows, err := store.ListRows(testutil.Context(t), ListOptions{
			ProviderID:   "beta",
			SourceID:     "config",
			IncludeAll:   true,
			IncludeStale: true,
			Now:          testTime(43),
		})
		if err != nil {
			t.Fatalf("ListRows(beta) error = %v", err)
		}
		if got, want := len(betaRows), 1; got != want {
			t.Fatalf("len(ListRows(beta)) = %d, want %d", got, want)
		}
	})

	t.Run("Should clear a removed provider during provider-scoped refresh", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{
			id:        "config",
			kind:      SourceKindConfig,
			priority:  PriorityConfig,
			providers: []string{"alpha", "beta"},
			rows: []ModelRow{
				testRow("config", SourceKindConfig, PriorityConfig, "alpha", "alpha-model", testTime(44), nil),
				testRow("config", SourceKindConfig, PriorityConfig, "beta", "beta-model", testTime(44), nil),
			},
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		if _, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(44)}); err != nil {
			t.Fatalf("Refresh(initial global) error = %v", err)
		}

		source.providers = []string{"beta"}
		source.rows = []ModelRow{
			testRow("config", SourceKindConfig, PriorityConfig, "beta", "beta-model", testTime(45), nil),
		}
		if _, err := service.Refresh(testutil.Context(t), RefreshOptions{
			ProviderID: "alpha",
			Force:      true,
			Now:        testTime(45),
		}); err != nil {
			t.Fatalf("Refresh(alpha after removal) error = %v", err)
		}

		alphaRows, err := store.ListRows(testutil.Context(t), ListOptions{
			ProviderID:   "alpha",
			SourceID:     "config",
			IncludeAll:   true,
			IncludeStale: true,
			Now:          testTime(45),
		})
		if err != nil {
			t.Fatalf("ListRows(alpha) error = %v", err)
		}
		if len(alphaRows) != 0 {
			t.Fatalf("ListRows(alpha) = %#v, want removed provider rows cleared by scoped refresh", alphaRows)
		}
		betaRows, err := store.ListRows(testutil.Context(t), ListOptions{
			ProviderID:   "beta",
			SourceID:     "config",
			IncludeAll:   true,
			IncludeStale: true,
			Now:          testTime(45),
		})
		if err != nil {
			t.Fatalf("ListRows(beta) error = %v", err)
		}
		if got, want := len(betaRows), 1; got != want {
			t.Fatalf("len(ListRows(beta)) = %d, want %d", got, want)
		}
	})

	t.Run("Should retain all source statuses when ListSourceStatus provider filter is empty", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{
			id:        "provider_live:codex",
			kind:      SourceKindProviderLive,
			priority:  PriorityProviderLive,
			providers: []string{"codex"},
			rows: []ModelRow{
				testRow(
					"provider_live:codex",
					SourceKindProviderLive,
					PriorityProviderLive,
					"codex",
					"gpt-5.4",
					testTime(46),
					nil,
				),
			},
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		if _, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(46)}); err != nil {
			t.Fatalf("Refresh(initial) error = %v", err)
		}

		statuses, err := service.ListSourceStatus(testutil.Context(t), "   ")
		if err != nil {
			t.Fatalf("ListSourceStatus(blank) error = %v", err)
		}
		if got, want := len(statuses), 1; got != want {
			t.Fatalf("len(ListSourceStatus(blank)) = %d, want %d: %#v", got, want, statuses)
		}
		if got, want := statuses[0].ProviderID, "codex"; got != want {
			t.Fatalf("ProviderID = %q, want %q", got, want)
		}
	})

	t.Run("Should retain discovered providers for global source failures without ProviderIDs", func(t *testing.T) {
		t.Parallel()

		source := &sourceWithoutProviderIDs{
			id:       "models_dev",
			kind:     SourceKindModelsDev,
			priority: PriorityModelsDev,
			rows: []ModelRow{
				testRow("models_dev", SourceKindModelsDev, PriorityModelsDev, "codex", "gpt-5.4", testTime(50), nil),
			},
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		if _, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(50)}); err != nil {
			t.Fatalf("Refresh(initial global) error = %v", err)
		}

		source.rows = nil
		source.err = errors.New("models.dev unavailable")
		statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(51)})
		if err == nil {
			t.Fatal("Refresh(failed global) error = nil, want stale failure")
		}
		status := requireStatus(t, statuses, "models_dev")
		if status.ProviderID != "codex" {
			t.Fatalf("ProviderID = %q, want codex", status.ProviderID)
		}
		if status.RefreshState != RefreshStateFailed || !status.Stale {
			t.Fatalf("status = %#v, want failed stale status", status)
		}
		rows, err := store.ListRows(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", SourceID: "models_dev", IncludeStale: true, Now: testTime(51)},
		)
		if err != nil {
			t.Fatalf("ListRows(stale source) error = %v", err)
		}
		if got, want := len(rows), 1; got != want {
			t.Fatalf("len(rows) = %d, want %d: %#v", got, want, rows)
		}
		if !rows[0].Stale {
			t.Fatalf("rows[0].Stale = %t, want true", rows[0].Stale)
		}
	})

	t.Run("Should clear discovered providers for global source disable without ProviderIDs", func(t *testing.T) {
		t.Parallel()

		source := &sourceWithoutProviderIDs{
			id:       "models_dev",
			kind:     SourceKindModelsDev,
			priority: PriorityModelsDev,
			rows: []ModelRow{
				testRow("models_dev", SourceKindModelsDev, PriorityModelsDev, "codex", "gpt-5.4", testTime(52), nil),
			},
		}
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		if _, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(52)}); err != nil {
			t.Fatalf("Refresh(initial global) error = %v", err)
		}

		source.rows = nil
		source.err = ErrSourceDisabled
		statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{Force: true, Now: testTime(53)})
		if err != nil {
			t.Fatalf("Refresh(disabled global) error = %v", err)
		}
		status := requireStatus(t, statuses, "models_dev")
		if status.ProviderID != "codex" {
			t.Fatalf("ProviderID = %q, want codex", status.ProviderID)
		}
		if status.RefreshState != RefreshStateDisabled {
			t.Fatalf("RefreshState = %q, want disabled", status.RefreshState)
		}
		rows, err := store.ListRows(
			testutil.Context(t),
			ListOptions{ProviderID: "codex", SourceID: "models_dev", IncludeAll: true, Now: testTime(53)},
		)
		if err != nil {
			t.Fatalf("ListRows(disabled source) error = %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("ListRows(disabled source) = %#v, want cleared rows", rows)
		}
	})

	t.Run("Should return stale fallback error from direct refresh", func(t *testing.T) {
		t.Parallel()

		sourceErr := &StaleFallbackError{SourceID: "models_dev", Err: errors.New("upstream unavailable")}
		source := &fakeSource{
			id:        "models_dev",
			kind:      SourceKindModelsDev,
			priority:  PriorityModelsDev,
			providers: []string{"codex"},
			rows: []ModelRow{
				testRow("models_dev", SourceKindModelsDev, PriorityModelsDev, "codex", "gpt-5.4", testTime(54), nil),
			},
			err: sourceErr,
		}
		service := newTestService(t, newMemoryStore(), []Source{source})
		statuses, err := service.Refresh(
			testutil.Context(t),
			RefreshOptions{ProviderID: "codex", SourceID: source.ID(), Force: true, Now: testTime(54)},
		)
		var fallback *StaleFallbackError
		if !errors.As(err, &fallback) {
			t.Fatalf("Refresh(stale fallback) error = %v, want StaleFallbackError", err)
		}
		if fallback.SourceID != source.ID() {
			t.Fatalf("StaleFallbackError.SourceID = %q, want %q", fallback.SourceID, source.ID())
		}
		status := requireStatus(t, statuses, source.ID())
		if status.RefreshState != RefreshStateFailed || !status.Stale {
			t.Fatalf("status = %#v, want failed stale status", status)
		}
	})
}

func TestCatalogServiceRefreshConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("Should coalesce concurrent refreshes for the same provider scope", func(t *testing.T) {
		t.Parallel()

		source := newBlockingRefreshSource(map[string][]ModelRow{
			"codex": {
				testRow(
					"provider_live:codex",
					SourceKindProviderLive,
					PriorityProviderLive,
					"codex",
					"gpt-5.4",
					testTime(30),
					nil,
				),
			},
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		ctx := testutil.Context(t)

		results := make(chan refreshTestResult, 2)
		for range 2 {
			go func() {
				statuses, err := service.Refresh(ctx, RefreshOptions{
					ProviderID: "codex",
					SourceID:   source.ID(),
					Force:      true,
					Now:        testTime(30),
				})
				results <- refreshTestResult{statuses: statuses, err: err}
			}()
		}
		source.waitForCalls(t, 1)
		source.requireCallCountStable(t, 1, 25*time.Millisecond)
		source.release()

		for range 2 {
			result := <-results
			if result.err != nil {
				t.Fatalf("Refresh() error = %v", result.err)
			}
			if got, want := len(result.statuses), 1; got != want {
				t.Fatalf("len(statuses) = %d, want %d: %#v", got, want, result.statuses)
			}
		}
	})

	t.Run("Should let concurrent refreshes across providers replace rows deterministically", func(t *testing.T) {
		t.Parallel()

		source := newBlockingRefreshSource(map[string][]ModelRow{
			"claude": {
				testRow(
					"provider_live:shared",
					SourceKindProviderLive,
					PriorityProviderLive,
					"claude",
					"claude-sonnet-4-6",
					testTime(31),
					nil,
				),
			},
			"codex": {
				testRow(
					"provider_live:shared",
					SourceKindProviderLive,
					PriorityProviderLive,
					"codex",
					"gpt-5.4",
					testTime(31),
					nil,
				),
			},
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})
		ctx := testutil.Context(t)

		results := make(chan refreshTestResult, 2)
		for _, providerID := range []string{"codex", "claude"} {
			go func(providerID string) {
				statuses, err := service.Refresh(ctx, RefreshOptions{
					ProviderID: providerID,
					SourceID:   source.ID(),
					Force:      true,
					Now:        testTime(31),
				})
				results <- refreshTestResult{statuses: statuses, err: err}
			}(providerID)
		}
		source.waitForCalls(t, 2)
		source.release()

		for range 2 {
			result := <-results
			if result.err != nil {
				t.Fatalf("Refresh() error = %v", result.err)
			}
			if got, want := len(result.statuses), 1; got != want {
				t.Fatalf("len(statuses) = %d, want %d: %#v", got, want, result.statuses)
			}
		}
		models, err := service.ListModels(
			ctx,
			ListOptions{IncludeStale: true, Now: testTime(32)},
		)
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := modelKeys(models), []string{"claude/claude-sonnet-4-6", "codex/gpt-5.4"}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("model keys = %#v, want %#v", got, want)
		}
		if got, want := source.callCount(), 2; got != want {
			t.Fatalf("source calls = %d, want %d cross-provider calls", got, want)
		}
	})

	t.Run("Should cancel waiters blocked on an in-flight refresh", func(t *testing.T) {
		t.Parallel()

		source := newBlockingRefreshSource(map[string][]ModelRow{
			"codex": {
				testRow(
					"provider_live:shared",
					SourceKindProviderLive,
					PriorityProviderLive,
					"codex",
					"gpt-5.4",
					testTime(33),
					nil,
				),
			},
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		results := make(chan refreshTestResult, 2)
		go func() {
			statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{
				ProviderID: "codex",
				SourceID:   source.ID(),
				Force:      true,
				Now:        testTime(33),
			})
			results <- refreshTestResult{statuses: statuses, err: err}
		}()

		source.waitForCalls(t, 1)

		waiterCtx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		go func() {
			statuses, err := service.Refresh(waiterCtx, RefreshOptions{
				ProviderID: "codex",
				SourceID:   source.ID(),
				Force:      true,
				Now:        testTime(33),
			})
			results <- refreshTestResult{statuses: statuses, err: err}
		}()

		source.requireCallCountStable(t, 1, 25*time.Millisecond)
		cancel()

		waiterResult := <-results
		if !errors.Is(waiterResult.err, context.Canceled) {
			t.Fatalf("waiter Refresh() error = %v, want context.Canceled", waiterResult.err)
		}

		source.release()

		ownerResult := <-results
		if ownerResult.err != nil {
			t.Fatalf("owner Refresh() error = %v", ownerResult.err)
		}
		if got, want := len(ownerResult.statuses), 1; got != want {
			t.Fatalf("len(statuses) = %d, want %d: %#v", got, want, ownerResult.statuses)
		}
	})

	t.Run("Should coalesce provider-scoped refreshes inside a concurrent global refresh", func(t *testing.T) {
		t.Parallel()

		source := newBlockingRefreshSource(map[string][]ModelRow{
			"codex": {
				testRow(
					"provider_live:shared",
					SourceKindProviderLive,
					PriorityProviderLive,
					"codex",
					"gpt-5.4",
					testTime(34),
					nil,
				),
			},
			"openrouter": {
				testRow(
					"provider_live:shared",
					SourceKindProviderLive,
					PriorityProviderLive,
					"openrouter",
					"gpt-5.4",
					testTime(34),
					nil,
				),
			},
		})
		store := newMemoryStore()
		service := newTestService(t, store, []Source{source})

		results := make(chan refreshTestResult, 2)
		go func() {
			statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{
				Force: true,
				Now:   testTime(34),
			})
			results <- refreshTestResult{statuses: statuses, err: err}
		}()

		source.waitForCalls(t, 1)
		go func() {
			statuses, err := service.Refresh(testutil.Context(t), RefreshOptions{
				ProviderID: "codex",
				SourceID:   source.ID(),
				Force:      true,
				Now:        testTime(34),
			})
			results <- refreshTestResult{statuses: statuses, err: err}
		}()

		source.requireCallCountStable(t, 1, 25*time.Millisecond)
		source.release()

		for range 2 {
			result := <-results
			if result.err != nil {
				t.Fatalf("Refresh() error = %v", result.err)
			}
		}
		if got, want := source.callCount(), 1; got != want {
			t.Fatalf("source calls = %d, want %d shared global snapshot", got, want)
		}
	})
}

type fakeSource struct {
	id        string
	kind      SourceKind
	priority  int
	providers []string
	rows      []ModelRow
	err       error
	ttl       time.Duration
	calls     int
}

type sourceWithoutProviderIDs struct {
	id       string
	kind     SourceKind
	priority int
	rows     []ModelRow
	err      error
	calls    int
}

func (s *sourceWithoutProviderIDs) ID() string {
	return s.id
}

func (s *sourceWithoutProviderIDs) Kind() SourceKind {
	return s.kind
}

func (s *sourceWithoutProviderIDs) Priority() int {
	return s.priority
}

func (s *sourceWithoutProviderIDs) ListModels(_ context.Context, opts ListOptions) ([]ModelRow, error) {
	s.calls++
	rows := make([]ModelRow, 0, len(s.rows))
	for _, row := range s.rows {
		if opts.ProviderID == "" || row.ProviderID == opts.ProviderID {
			rows = append(rows, row)
		}
	}
	return rows, s.err
}

func (s *fakeSource) ID() string {
	return s.id
}

func (s *fakeSource) Kind() SourceKind {
	return s.kind
}

func (s *fakeSource) Priority() int {
	return s.priority
}

func (s *fakeSource) ProviderIDs() []string {
	return append([]string(nil), s.providers...)
}

func (s *fakeSource) TTL() time.Duration {
	return s.ttl
}

func (s *fakeSource) ListModels(_ context.Context, opts ListOptions) ([]ModelRow, error) {
	s.calls++
	rows := make([]ModelRow, 0, len(s.rows))
	for _, row := range s.rows {
		if opts.ProviderID == "" || row.ProviderID == opts.ProviderID {
			rows = append(rows, row)
		}
	}
	return rows, s.err
}

type refreshTestResult struct {
	statuses []SourceStatus
	err      error
}

type blockingRefreshSource struct {
	mu             sync.Mutex
	rowsByProvider map[string][]ModelRow
	calls          int
	callsCh        chan int
	releaseCh      chan struct{}
	releaseOnce    sync.Once
}

func newBlockingRefreshSource(rowsByProvider map[string][]ModelRow) *blockingRefreshSource {
	return &blockingRefreshSource{
		rowsByProvider: rowsByProvider,
		callsCh:        make(chan int, 16),
		releaseCh:      make(chan struct{}),
	}
}

func (s *blockingRefreshSource) ID() string {
	return "provider_live:shared"
}

func (s *blockingRefreshSource) Kind() SourceKind {
	return SourceKindProviderLive
}

func (s *blockingRefreshSource) Priority() int {
	return PriorityProviderLive
}

func (s *blockingRefreshSource) ProviderIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	providers := make([]string, 0, len(s.rowsByProvider))
	for providerID := range s.rowsByProvider {
		providers = append(providers, providerID)
	}
	slices.Sort(providers)
	return providers
}

func (s *blockingRefreshSource) TTL() time.Duration {
	return 0
}

func (s *blockingRefreshSource) ListModels(ctx context.Context, opts ListOptions) ([]ModelRow, error) {
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
	rows := make([]ModelRow, 0)
	if opts.ProviderID == "" {
		providers := make([]string, 0, len(s.rowsByProvider))
		for providerID := range s.rowsByProvider {
			providers = append(providers, providerID)
		}
		slices.Sort(providers)
		for _, providerID := range providers {
			rows = append(rows, cloneModelRows(s.rowsByProvider[providerID])...)
		}
	} else {
		rows = cloneModelRows(s.rowsByProvider[opts.ProviderID])
	}
	s.mu.Unlock()
	return rows, nil
}

func (s *blockingRefreshSource) waitForCalls(t *testing.T, want int) {
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

func (s *blockingRefreshSource) requireCallCountStable(t *testing.T, want int, duration time.Duration) {
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

func (s *blockingRefreshSource) release() {
	s.releaseOnce.Do(func() {
		close(s.releaseCh)
	})
}

func (s *blockingRefreshSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type memoryStore struct {
	mu           sync.Mutex
	rows         map[string][]ModelRow
	statuses     map[string]SourceStatus
	replaceCount int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		rows:     make(map[string][]ModelRow),
		statuses: make(map[string]SourceStatus),
	}
}

func (s *memoryStore) ReplaceSourceRows(
	_ context.Context,
	sourceID string,
	providerID string,
	rows []ModelRow,
	status SourceStatus,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replaceCount++
	key := sourceProviderKey(sourceID, providerID)
	s.rows[key] = cloneModelRows(rows)
	s.statuses[key] = status
	return nil
}

func (s *memoryStore) ListRows(_ context.Context, opts ListOptions) ([]ModelRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]ModelRow, 0)
	for _, group := range s.rows {
		for _, row := range group {
			if opts.ProviderID != "" && row.ProviderID != opts.ProviderID {
				continue
			}
			if opts.SourceID != "" && row.SourceID != opts.SourceID {
				continue
			}
			if row.Stale && !opts.IncludeAll && !opts.IncludeStale {
				continue
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (s *memoryStore) ListSourceStatus(_ context.Context, providerID string) ([]SourceStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	statuses := make([]SourceStatus, 0, len(s.statuses))
	for _, status := range s.statuses {
		if providerID == "" || status.ProviderID == providerID {
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}

func sourceProviderKey(sourceID string, providerID string) string {
	return sourceID + "\x00" + providerID
}

func newTestService(t *testing.T, store Store, sources []Source) *CatalogService {
	t.Helper()

	service, err := NewService(store, sources, MergeOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func mergeTestRows(rows []ModelRow) []Model {
	reasoningApply := make(map[string]bool)
	for _, row := range rows {
		reasoningApply[row.ProviderID] = true
	}
	return MergeRows(rows, MergeOptions{ReasoningApply: reasoningApply})
}

func testRow(
	sourceID string,
	kind SourceKind,
	priority int,
	providerID string,
	modelID string,
	refreshedAt time.Time,
	mutate func(*ModelRow),
) ModelRow {
	row := ModelRow{
		SourceID:    sourceID,
		SourceKind:  kind,
		Priority:    priority,
		ProviderID:  providerID,
		ModelID:     modelID,
		RefreshedAt: refreshedAt,
	}
	if mutate != nil {
		mutate(&row)
	}
	return row
}

func testTime(offset int) time.Time {
	return time.Date(2026, 5, 7, 12, offset, 0, 0, time.UTC)
}

func requireSingleModel(t *testing.T, models []Model) Model {
	t.Helper()

	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1: %#v", len(models), models)
	}
	return models[0]
}

func requireReasoningProfile(
	t *testing.T,
	model Model,
	expectedSupport bool,
	expectedEfforts []ReasoningEffort,
	expectedDefault ReasoningEffort,
) {
	t.Helper()

	if model.SupportsReasoning == nil || *model.SupportsReasoning != expectedSupport {
		t.Fatalf("SupportsReasoning = %v, want %t", model.SupportsReasoning, expectedSupport)
	}
	if !slices.Equal(model.ReasoningEfforts, expectedEfforts) {
		t.Fatalf("ReasoningEfforts = %#v, want %#v", model.ReasoningEfforts, expectedEfforts)
	}
	if model.DefaultReasoningEffort == nil || *model.DefaultReasoningEffort != expectedDefault {
		t.Fatalf("DefaultReasoningEffort = %v, want %q", model.DefaultReasoningEffort, expectedDefault)
	}
}

func requireStatus(t *testing.T, statuses []SourceStatus, sourceID string) SourceStatus {
	t.Helper()

	for _, status := range statuses {
		if status.SourceID == sourceID {
			return status
		}
	}
	t.Fatalf("statuses = %#v, want source %q", statuses, sourceID)
	return SourceStatus{}
}

func modelKeys(models []Model) []string {
	keys := make([]string, 0, len(models))
	for _, model := range models {
		keys = append(keys, model.ProviderID+"/"+model.ModelID)
	}
	return keys
}

func sourceIDs(sources []SourceRef) []string {
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		ids = append(ids, source.SourceID)
	}
	return ids
}
