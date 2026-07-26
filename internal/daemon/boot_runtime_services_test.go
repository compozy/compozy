package daemon

import (
	"reflect"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
)

func TestDreamGateConfigFromConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve configured dream gate and scoring values", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DreamConfig{
			Gates: aghconfig.MemoryDreamGatesConfig{
				MinUnpromoted:  3,
				MinRecallCount: 4,
				MinScore:       0.65,
			},
			Scoring: aghconfig.MemoryDreamScoringConfig{
				RecencyHalfLifeDays: 21,
				Weights: aghconfig.MemoryDreamScoringWeightsConfig{
					Frequency: 0.1,
					Relevance: 0.2,
					Recency:   0.3,
					Freshness: 0.4,
				},
			},
		}

		got := dreamGateConfigFromConfig(cfg)
		want := memory.DreamGateConfig{
			MinCandidates:   3,
			MinRecallCount:  4,
			MinScore:        0.65,
			HalfLife:        21 * 24 * time.Hour,
			FrequencyWeight: 0.1,
			RelevanceWeight: 0.2,
			RecencyWeight:   0.3,
			FreshnessWeight: 0.4,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("dreamGateConfigFromConfig() = %#v, want %#v", got, want)
		}
	})
}
