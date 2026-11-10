package daemon

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesMarketplace(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload contract.MarketplaceListResponse
		if err := json.Unmarshal(
			[]byte(
				`{"categories":[{"id":"skills","label":"Skills","count":1}],"total":1,"next_cursor":"next","revision":"rev","stale":true,"refreshing":true,"error_class":"source_failed","error":"source failed","sources":[{"name":"source","kind":"git","state":"failed","count":1}],"items":[{"category":"skills","featured":false,"source_ref":"repo/path","installable":false,"install_blocker":"missing_digest","digest_sha256":"digest","name_conflict":{"source":"local","source_ref":"repo/path","entry_id":"other"},"entry_id":"sample","name":"sample","description":"Sample","install_slug":"sample","source":"repo","installed":false,"update_available":false,"icon":"data:image/svg+xml;xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","trust":{"verdict":"unknown"}}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__marketplace_search",
				Name:   "failure",
				MustKeep: []string{
					"next_cursor",
					"revision",
					"stale",
					"error",
					"items.0.digest_sha256",
					"items.0.install_blocker",
					"items.0.name_conflict",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.MarketplaceListResponse]{Compact: compactMarketplace}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload contract.MarketplaceListResponse
		if err := json.Unmarshal(
			[]byte(
				`{"categories":[],"total":1,"next_cursor":"next","revision":"rev","stale":true,"refreshing":true,"error_class":"source_failed","error":"source failed","sources":[],"items":[]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__marketplace_search",
				Name:     "empty",
				MustKeep: []string{"revision", "next_cursor", "stale", "error", "items"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.MarketplaceListResponse]{Compact: compactMarketplace}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload contract.MarketplaceSourcesResponse
		if err := json.Unmarshal(
			[]byte(
				`{"sources":[{"name":"source","kind":"git","source":"repo","enabled":true,"state":"failed","plugins":1,"installable":0,"stability":"stable","error_class":"invalid","error":"invalid source","diagnostics":[{"id":"d1","code":"config_invalid","severity":"critical","category":"config","title":"Source failed","message":"Source cannot be read","data_freshness":"stale","evidence":{"detail":"internal evidence"},"suggested_command":"compozy config show"}]}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__marketplace_sources",
				Name:   "failure",
				MustKeep: []string{
					"sources.0.error",
					"sources.0.state",
					"sources.0.diagnostics.0.severity",
					"sources.0.diagnostics.0.message",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.MarketplaceSourcesResponse]{Compact: compactMarketplaceSources}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload contract.MarketplaceSourcesResponse
		if err := json.Unmarshal([]byte(`{"sources":[]}`), &payload); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__marketplace_sources",
				Name:     "empty",
				MustKeep: []string{"sources"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.MarketplaceSourcesResponse]{Compact: compactMarketplaceSources}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	return rows
}
