package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestUnixSocketClientMarketplaceMethods(t *testing.T) {
	t.Parallel()

	t.Run("Should map all marketplace namespace requests", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet {
					if got, want := req.URL.Query().Get("scope"), "workspace"; got != want {
						t.Fatalf("marketplace scope = %q, want %q", got, want)
					}
					if got, want := req.URL.Query().Get("workspace_id"), "ws-alpha"; got != want {
						t.Fatalf("marketplace workspace_id = %q, want %q", got, want)
					}
				}
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/marketplace/search":
					if got := req.URL.Query().Get("q"); got != "review" {
						t.Fatalf("search q = %q, want review", got)
					}
					if got := req.URL.Query().Get("limit"); got != "7" {
						t.Fatalf("search limit = %q, want 7", got)
					}
					return newHTTPResponse(http.StatusOK, `{"kinds":[]}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/marketplace/skill":
					if got := req.URL.Query().Get("q"); got != "review" {
						t.Fatalf("browse q = %q, want review", got)
					}
					if got := req.URL.Query().Get("limit"); got != "5" {
						t.Fatalf("browse limit = %q, want 5", got)
					}
					if got := req.URL.Query().Get("cursor"); got != "next-page" {
						t.Fatalf("browse cursor = %q, want next-page", got)
					}
					return newHTTPResponse(http.StatusOK, `{"kind":"skill","items":[]}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/marketplace/skill/skill-entry":
					if got := req.URL.Query().Get("installed_name"); got != "local-skill" {
						t.Fatalf("detail installed_name = %q, want local-skill", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"entry":{"kind":"skill","entry_id":"skill-entry","name":"Review","description":"Review helper","installed":false,"source":"curated"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/marketplace/refresh":
					if got := req.URL.Query().Get("kind"); got != "skill" {
						t.Fatalf("refresh kind = %q, want skill", got)
					}
					return newHTTPResponse(http.StatusOK, `{"kinds":[]}`), nil
				default:
					t.Fatalf("unexpected marketplace request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})},
		}

		ctx := context.Background()
		readScope := MarketplaceReadScope{Scope: "workspace", WorkspaceID: "ws-alpha"}
		search, err := client.SearchMarketplace(ctx, " review ", 7, readScope)
		if err != nil {
			t.Fatalf("SearchMarketplace() error = %v", err)
		}
		if len(search.Kinds) != 0 {
			t.Fatalf("SearchMarketplace().Kinds = %#v, want empty", search.Kinds)
		}

		browse, err := client.BrowseMarketplace(ctx, "skill", " review ", 5, " next-page ", readScope)
		if err != nil {
			t.Fatalf("BrowseMarketplace() error = %v", err)
		}
		if browse.Kind != "skill" {
			t.Fatalf("BrowseMarketplace().Kind = %q, want skill", browse.Kind)
		}

		detail, err := client.MarketplaceInfo(ctx, "skill", "skill-entry", " local-skill ", readScope)
		if err != nil {
			t.Fatalf("MarketplaceInfo() error = %v", err)
		}
		if detail.Entry.EntryID != "skill-entry" {
			t.Fatalf("MarketplaceInfo().Entry.EntryID = %q, want skill-entry", detail.Entry.EntryID)
		}

		refresh, err := client.RefreshMarketplace(ctx, " skill ")
		if err != nil {
			t.Fatalf("RefreshMarketplace() error = %v", err)
		}
		if len(refresh.Kinds) != 0 {
			t.Fatalf("RefreshMarketplace().Kinds = %#v, want empty", refresh.Kinds)
		}
	})

	t.Run("Should reject blank marketplace path segments before transport", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{}
		ctx := context.Background()
		globalScope := MarketplaceReadScope{Scope: "global"}
		if _, err := client.BrowseMarketplace(ctx, "   ", "", 20, "", globalScope); err == nil ||
			!strings.Contains(err.Error(), "marketplace kind is required") {
			t.Fatalf("BrowseMarketplace(blank kind) error = %v", err)
		}
		if _, err := client.MarketplaceInfo(ctx, "   ", "entry", "", globalScope); err == nil ||
			!strings.Contains(err.Error(), "marketplace kind is required") {
			t.Fatalf("MarketplaceInfo(blank kind) error = %v", err)
		}
		if _, err := client.MarketplaceInfo(ctx, "skill", "   ", "", globalScope); err == nil ||
			!strings.Contains(err.Error(), "marketplace entry ID is required") {
			t.Fatalf("MarketplaceInfo(blank entry ID) error = %v", err)
		}
		if _, err := client.SearchMarketplace(
			ctx,
			"",
			20,
			MarketplaceReadScope{Scope: "workspace"},
		); err == nil || !strings.Contains(err.Error(), "--scope workspace requires --workspace") {
			t.Fatalf("SearchMarketplace(workspace without ID) error = %v", err)
		}
		if _, err := client.MarketplaceInfo(
			ctx,
			"skill",
			"entry",
			"",
			MarketplaceReadScope{Scope: "global", WorkspaceID: "ws-alpha"},
		); err == nil || !strings.Contains(err.Error(), "--workspace requires --scope workspace") {
			t.Fatalf("MarketplaceInfo(global with workspace) error = %v", err)
		}
	})

	t.Run("Should reject installed identity for bundle detail before transport", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected marketplace request: %s %s", req.Method, req.URL.String())
				return nil, nil
			})},
		}

		_, err := client.MarketplaceInfo(
			t.Context(),
			"bundle",
			"review-kit",
			"activation-review-kit",
			MarketplaceReadScope{Scope: "global"},
		)
		if err == nil || !strings.Contains(err.Error(), "--installed-name is only supported") {
			t.Fatalf("MarketplaceInfo(bundle installed identity) error = %v, want local validation", err)
		}
	})
}
