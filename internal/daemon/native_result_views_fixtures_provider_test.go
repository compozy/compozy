package daemon

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesProvider(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload contract.ProviderModelListResponse
		if err := json.Unmarshal(
			[]byte(
				`{"models":[{"provider_id":"claude","model_id":"model","available":false,"availability_state":"unavailable","stale":true,"curated":true,"hidden":false,"featured":false,"deprecated":false,"last_error":"config failed","config_options":[{"id":"effort"}],"sources":[{"source_id":"config","source_kind":"config","priority":1,"stale":true,"last_error":"config failed"},{"source_id":"live","source_kind":"provider_live","priority":2,"stale":true,"last_error":"authentication required"}],"configurations":[{"fast":true}]}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__provider_models_list",
				Name:   "failure",
				MustKeep: []string{
					"models.0.model_id",
					"models.0.last_error",
					"models.0.sources.1.source_id",
					"models.0.sources.1.stale",
					"models.0.sources.1.last_error",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.ProviderModelListResponse]{Compact: compactProviderModels}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload contract.ProviderModelListResponse
		if err := json.Unmarshal([]byte(`{"models":[]}`), &payload); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__provider_models_list",
				Name:     "empty",
				MustKeep: []string{"models"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.ProviderModelListResponse]{Compact: compactProviderModels}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload *contract.ProviderModelCurationResponse
		if err := json.Unmarshal(
			[]byte(
				`{"model":{"provider_id":"claude","model_id":"model","available":false,"availability_state":"unavailable","stale":true,"curated":true,"hidden":false,"featured":false,"deprecated":false,"last_error":"config failed","config_options":[{"id":"effort"}],"sources":[{"source_id":"config","source_kind":"config","priority":1,"stale":true,"last_error":"config failed"},{"source_id":"live","source_kind":"provider_live","priority":2,"stale":true,"last_error":"authentication required"}],"configurations":[{"fast":true}]},"apply":{"applied":false,"lifecycle":"live","next_action":"retry","partial_failures":[{"subsystem":"provider","diagnostic":{"id":"apply-a","code":"apply_failed","severity":"critical","category":"config","title":"Apply failed","message":"Provider refused config","data_freshness":"fresh"}}],"apply_record_id":"apply-a","active_generation":2,"active_config_hash":"hash"},"default_speed":"fast"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__provider_models_curate",
				Name:   "failure",
				MustKeep: []string{
					"model.model_id",
					"model.last_error",
					"model.sources.1.last_error",
					"apply.apply_record_id",
					"apply.active_generation",
					"apply.active_config_hash",
					"apply.partial_failures.0.diagnostic.message",
					"apply.next_action",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.ProviderModelCurationResponse]{Compact: compactProviderCuration}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	return rows
}
