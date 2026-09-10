package daemon

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	"github.com/gin-gonic/gin"
)

func TestDreamGateConfigFromConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve configured dream gate and scoring values", func(t *testing.T) {
		t.Parallel()

		cfg := compozyconfig.DreamConfig{
			Gates: compozyconfig.MemoryDreamGatesConfig{
				MinUnpromoted:  3,
				MinRecallCount: 4,
				MinScore:       0.65,
			},
			Scoring: compozyconfig.MemoryDreamScoringConfig{
				RecencyHalfLifeDays: 21,
				Weights: compozyconfig.MemoryDreamScoringWeightsConfig{
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

func TestDaemonMemorySessionLedgerAvailability(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		state      *bootState
		wantStatus int
	}{
		{name: "Should report unsupported without boot state", wantStatus: http.StatusNotImplemented},
		{name: "Should report unsupported when memory is disabled", state: &bootState{}, wantStatus: http.StatusNotImplemented},
		{name: "Should report unsupported without a ledger root", state: &bootState{
			cfg: compozyconfig.Config{Memory: compozyconfig.MemoryConfig{Enabled: true}},
		}, wantStatus: http.StatusNotImplemented},
		{name: "Should keep configured session memory available", state: &bootState{
			cfg: compozyconfig.Config{Memory: compozyconfig.MemoryConfig{
				Enabled: true,
				Session: compozyconfig.MemorySessionConfig{LedgerRoot: t.TempDir()},
			}},
		}, wantStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
				MemorySessionLedger: newDaemonMemorySessionLedgerService(tc.state, time.Now),
			})
			router := gin.New()
			router.Use(gin.RecoveryWithWriter(io.Discard))
			router.POST("/memory/sessions/repair", handlers.RepairMemorySessions)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "/memory/sessions/repair", nil,
			))
			if response.Code != tc.wantStatus {
				t.Fatalf("repair status = %d, want %d; body: %s", response.Code, tc.wantStatus, response.Body)
			}
			if tc.wantStatus == http.StatusOK {
				return
			}
			var payload struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode unsupported response: %v", err)
			}
			if payload.Code != "memory.unsupported" {
				t.Fatalf("repair error code = %q, want memory.unsupported", payload.Code)
			}
		})
	}
}
