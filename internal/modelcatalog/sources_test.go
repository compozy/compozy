package modelcatalog

import (
	"errors"
	"slices"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
)

func TestProviderConfigSources(t *testing.T) {
	t.Parallel()

	t.Run("Should expose manual default outside curated list", func(t *testing.T) {
		t.Parallel()

		source := NewConfigSource(map[string]aghconfig.ProviderConfig{
			"codex": {
				Models: aghconfig.ProviderModelsConfig{
					Default: "manual-model",
					Curated: []aghconfig.ProviderModelConfig{
						{ID: "curated-model", DisplayName: "Curated Model"},
					},
				},
			},
		})
		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "codex", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := rowModelIDs(rows), []string{"manual-model", "curated-model"}; !slices.Equal(got, want) {
			t.Fatalf("row ids = %#v, want %#v", got, want)
		}
		if rows[0].DisplayName != "" {
			t.Fatalf("default DisplayName = %q, want empty metadata for manual default", rows[0].DisplayName)
		}
		if rows[0].ExplicitlyCurated || !rows[1].ExplicitlyCurated {
			t.Fatalf(
				"explicit curation = default %v curated %v, want false/true",
				rows[0].ExplicitlyCurated,
				rows[1].ExplicitlyCurated,
			)
		}
	})

	t.Run("Should preserve exact configured model ids without aliases", func(t *testing.T) {
		t.Parallel()

		source := NewConfigSource(map[string]aghconfig.ProviderConfig{
			"claude": {
				Models: aghconfig.ProviderModelsConfig{Default: "sonnet"},
			},
		})
		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "claude", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := rowModelIDs(rows), []string{"sonnet"}; !slices.Equal(got, want) {
			t.Fatalf("row ids = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve explicit curated ids before applying aliases", func(t *testing.T) {
		t.Parallel()

		source := NewConfigSource(map[string]aghconfig.ProviderConfig{
			"codex": {
				Models: aghconfig.ProviderModelsConfig{
					Default: "gpt-5",
					Curated: []aghconfig.ProviderModelConfig{
						{ID: "gpt-5", DisplayName: "GPT-5"},
					},
				},
			},
		})
		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "codex", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if got, want := rowModelIDs(rows), []string{"gpt-5"}; !slices.Equal(got, want) {
			t.Fatalf("row ids = %#v, want %#v", got, want)
		}
		if !rows[0].ExplicitlyCurated {
			t.Fatal("default + curated row ExplicitlyCurated = false, want true")
		}
	})

	t.Run("Should convert curated config metadata into rows", func(t *testing.T) {
		t.Parallel()

		supportsTools := true
		contextWindow := int64(128000)
		defaultEffort := ReasoningEffortHigh
		source := NewConfigSource(map[string]aghconfig.ProviderConfig{
			"codex": {
				Models: aghconfig.ProviderModelsConfig{
					Default: "gpt-5.4",
					Curated: []aghconfig.ProviderModelConfig{
						{
							ID:                     "gpt-5.4",
							DisplayName:            "GPT-5.4",
							ContextWindow:          &contextWindow,
							SupportsTools:          &supportsTools,
							ReasoningEfforts:       []string{"low", "high"},
							DefaultReasoningEffort: string(defaultEffort),
						},
					},
				},
			},
		})
		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "codex", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1: %#v", len(rows), rows)
		}
		row := rows[0]
		if row.DisplayName != "GPT-5.4" {
			t.Fatalf("DisplayName = %q, want GPT-5.4", row.DisplayName)
		}
		if row.ContextWindow == nil || *row.ContextWindow != contextWindow {
			t.Fatalf("ContextWindow = %v, want %d", row.ContextWindow, contextWindow)
		}
		if row.SupportsTools == nil || !*row.SupportsTools {
			t.Fatalf("SupportsTools = %v, want true", row.SupportsTools)
		}
		if !slices.Equal(row.ReasoningEfforts, []ReasoningEffort{ReasoningEffortLow, ReasoningEffortHigh}) {
			t.Fatalf("ReasoningEfforts = %#v, want low/high", row.ReasoningEfforts)
		}
		if row.DefaultReasoningEffort == nil || *row.DefaultReasoningEffort != defaultEffort {
			t.Fatalf("DefaultReasoningEffort = %v, want high", row.DefaultReasoningEffort)
		}
	})

	t.Run("Should reject invalid configured reasoning profiles", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name          string
			efforts       []string
			defaultEffort string
			wantField     string
			wantEffort    string
		}{
			{
				name:       "a non-canonical selectable effort",
				efforts:    []string{"ultra"},
				wantField:  "reasoning_efforts[0]",
				wantEffort: "ultra",
			},
			{
				name:          "a default effort outside the selectable profile",
				efforts:       []string{"low"},
				defaultEffort: "max",
				wantField:     "default_reasoning_effort",
				wantEffort:    "max",
			},
		}
		for _, test := range tests {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()

				source := NewConfigSource(map[string]aghconfig.ProviderConfig{
					"codex": {
						Models: aghconfig.ProviderModelsConfig{
							Curated: []aghconfig.ProviderModelConfig{
								{
									ID:                     "configured-model",
									ReasoningEfforts:       test.efforts,
									DefaultReasoningEffort: test.defaultEffort,
								},
							},
						},
					},
				})
				_, err := source.ListModels(
					testutil.Context(t),
					ListOptions{ProviderID: "codex", Now: testTime(0)},
				)
				var invalid *InvalidReasoningEffortError
				if !errors.As(err, &invalid) {
					t.Fatalf("ListModels() error = %T %v, want *InvalidReasoningEffortError", err, err)
				}
				if invalid.ProviderID != "codex" || invalid.ModelID != "configured-model" ||
					invalid.Field != test.wantField || invalid.Effort != test.wantEffort {
					t.Fatalf(
						"InvalidReasoningEffortError = %#v, want field %q effort %q",
						invalid,
						test.wantField,
						test.wantEffort,
					)
				}
			})
		}
	})

	t.Run("Should snapshot provider configs at construction", func(t *testing.T) {
		t.Parallel()

		contextWindow := int64(128000)
		supportsTools := true
		deprecated := false
		hidden := false
		featured := true
		providers := map[string]aghconfig.ProviderConfig{
			"codex": {
				Models: aghconfig.ProviderModelsConfig{
					Curated: []aghconfig.ProviderModelConfig{
						{
							ID:               "gpt-5.4",
							DisplayName:      "GPT-5.4",
							ContextWindow:    &contextWindow,
							SupportsTools:    &supportsTools,
							ReasoningEfforts: []string{"low", "high"},
							Deprecated:       &deprecated,
							Hidden:           &hidden,
							Featured:         &featured,
						},
					},
				},
			},
		}

		source := NewConfigSource(providers)

		cfg := providers["codex"]
		cfg.Models.Curated[0].DisplayName = "Mutated"
		cfg.Models.Curated[0].ReasoningEfforts[0] = "xhigh"
		providers["codex"] = cfg
		contextWindow = 1
		supportsTools = false
		deprecated = true
		hidden = true
		featured = false

		rows, err := source.ListModels(testutil.Context(t), ListOptions{ProviderID: "codex", Now: testTime(0)})
		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1: %#v", len(rows), rows)
		}
		row := rows[0]
		if row.DisplayName != "GPT-5.4" {
			t.Fatalf("DisplayName = %q, want GPT-5.4 snapshot", row.DisplayName)
		}
		if row.ContextWindow == nil || *row.ContextWindow != 128000 {
			t.Fatalf("ContextWindow = %v, want 128000 snapshot", row.ContextWindow)
		}
		if row.SupportsTools == nil || !*row.SupportsTools {
			t.Fatalf("SupportsTools = %v, want true snapshot", row.SupportsTools)
		}
		if !slices.Equal(row.ReasoningEfforts, []ReasoningEffort{ReasoningEffortLow, ReasoningEffortHigh}) {
			t.Fatalf("ReasoningEfforts = %#v, want low/high snapshot", row.ReasoningEfforts)
		}
		if row.Deprecated == nil || *row.Deprecated {
			t.Fatalf("Deprecated = %v, want snapshotted false", row.Deprecated)
		}
		if row.Hidden == nil || *row.Hidden {
			t.Fatalf("Hidden = %v, want snapshotted false", row.Hidden)
		}
		if row.Featured == nil || !*row.Featured {
			t.Fatalf("Featured = %v, want snapshotted true", row.Featured)
		}
	})

	t.Run("Should expose builtin provider model defaults", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name       string
			providerID string
			modelID    string
		}{
			{name: "Should expose Codex defaults", providerID: "codex"},
			{
				name:       "Should expose the canonical Cursor descriptor",
				providerID: "cursor",
				modelID:    "grok-4.5[effort=high,fast=true]",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				source := NewBuiltinSource()
				rows, err := source.ListModels(
					testutil.Context(t),
					ListOptions{ProviderID: tc.providerID, Now: testTime(0)},
				)
				if err != nil {
					t.Fatalf("ListModels() error = %v", err)
				}
				if len(rows) == 0 {
					t.Fatalf("len(rows) = 0, want builtin %s models", tc.providerID)
				}
				if tc.modelID != "" && rows[0].ModelID != tc.modelID {
					t.Fatalf("ModelID = %q, want %q", rows[0].ModelID, tc.modelID)
				}
				if rows[0].SourceID != SourceIDBuiltin || rows[0].SourceKind != SourceKindBuiltin ||
					rows[0].Priority != PriorityBuiltin {
					t.Fatalf("row source = %#v, want builtin source metadata", rows[0])
				}
				if !rows[0].ExplicitlyCurated {
					t.Fatal("builtin curated row ExplicitlyCurated = false, want true")
				}
			})
		}
	})
}

func rowModelIDs(rows []ModelRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ModelID)
	}
	return ids
}
