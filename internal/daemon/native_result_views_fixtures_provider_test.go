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
				`{"models":[{"provider_id":"claude","model_id":"model","available":false,"availability_state":"unavailable","stale":true,"curated":true,"hidden":false,"featured":false,"deprecated":false,"last_error":"source failed","config_options":[{"id":"effort"}],"sources":[{"source_id":"config","source_kind":"config","priority":1,"stale":true}],"configurations":[{"fast":true}]}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__provider_models_list",
				Name:     "failure",
				MustKeep: []string{"models.0.model_id", "models.0.last_error"},
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
				`{"model":{"provider_id":"claude","model_id":"model","available":false,"availability_state":"unavailable","stale":true,"curated":true,"hidden":false,"featured":false,"deprecated":false,"last_error":"source failed","config_options":[{"id":"effort"}],"sources":[{"source_id":"config","source_kind":"config","priority":1,"stale":true}],"configurations":[{"fast":true}]},"apply":{"applied":false,"lifecycle":"live","next_action":"retry","partial_failures":[{"error":"apply failed"}]},"default_speed":"fast"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__provider_models_curate",
				Name:     "failure",
				MustKeep: []string{"model.model_id", "model.last_error", "apply.partial_failures", "apply.next_action"},
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
