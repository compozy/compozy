package daemon

import (
	"encoding/json"
	"testing"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesResources(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload resourceListResult
		if err := json.Unmarshal(
			[]byte(
				`{"records":[{"kind":"agent","id":"agent-a","version":2,"scope":{"kind":"workspace","id":"ws-a"},"owner":{"kind":"user","id":"u-a"},"source":{"kind":"user","id":"u-a"},"spec":{"prompt":"large embedded definition"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__resources_list",
				Name:     "failure",
				MustKeep: []string{"records.0.id", "records.0.version", "records.0.scope"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[resourceListResult]{Compact: compactResources}).result(view, payload)
				},
			},
		)
	}
	{
		var payload resourceListResult
		if err := json.Unmarshal([]byte(`{"records":[]}`), &payload); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__resources_list",
				Name:     "empty",
				MustKeep: []string{"records"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[resourceListResult]{Compact: compactResources}).result(view, payload)
				},
			},
		)
	}
	{
		var payload resourceListResult
		if err := json.Unmarshal(
			[]byte(
				`{"records":[{"kind":"agent","id":"agent-a","version":2,"scope":{"kind":"workspace","id":"ws-a"},"owner":{"kind":"user","id":"u-a"},"source":{"kind":"user","id":"u-a"},"spec":{"prompt":"large embedded definition"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"}],"count":1}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__resources_snapshot",
				Name:     "failure",
				MustKeep: []string{"records.0.id", "records.0.version", "records.0.scope", "count"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[resourceListResult]{Compact: compactResources}).result(view, payload)
				},
			},
		)
	}
	{
		var payload resourceListResult
		if err := json.Unmarshal([]byte(`{"records":[],"count":1}`), &payload); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__resources_snapshot",
				Name:     "empty",
				MustKeep: []string{"count", "records"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[resourceListResult]{Compact: compactResources}).result(view, payload)
				},
			},
		)
	}
	return rows
}
