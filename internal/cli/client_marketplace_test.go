package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestUnixSocketClientMarketplaceMethods(t *testing.T) {
	t.Parallel()

	t.Run("Should map canonical catalog pagination detail and refresh requests", func(t *testing.T) {
		t.Parallel()
		client := &daemonClient{
			target: LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				if query.Has("kind") {
					t.Fatal("canonical request carried an obsolete kind")
				}
				if req.Method == http.MethodGet &&
					(query.Get("scope") != "workspace" || query.Get("workspace_id") != "ws-alpha") {
					t.Fatalf("read scope = %v", query)
				}
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/marketplace":
					if query.Get("q") != "review" || query.Get("limit") != "7" || query.Get("cursor") != "page-two" {
						t.Fatalf("catalog query = %v", query)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"total":4,"revision":"revision-a","stale":true,"error_class":"network","next_cursor":"page-three","sources":[],"items":[]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/marketplace/entries/review":
					if query.Get("installed_name") != "local-review" || query.Get("source") != "compozy-catalog" {
						t.Fatalf("detail query = %v", query)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"entry":{"entry_id":"review","name":"Review","description":"Review helper","installed":true,"source":"compozy-catalog"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/marketplace/refresh":
					if len(query) != 0 {
						t.Fatalf("refresh query = %v", query)
					}
					return newHTTPResponse(http.StatusOK, `{"sources":[]}`), nil
				default:
					t.Fatalf("unexpected marketplace request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})},
		}
		ctx := t.Context()
		scope := MarketplaceReadScope{Scope: "workspace", WorkspaceID: "ws-alpha"}
		page, err := client.SearchMarketplace(ctx, " review ", 7, " page-two ", scope)
		if err != nil || page.Total != 4 || page.Revision != "revision-a" || page.NextCursor != "page-three" ||
			!page.Stale ||
			page.ErrorClass != "network" {
			t.Fatalf("SearchMarketplace() = %#v, %v", page, err)
		}
		detail, err := client.MarketplaceInfo(ctx, "review", " compozy-catalog ", " local-review ", scope)
		if err != nil || detail.Entry.EntryID != "review" || !detail.Entry.Installed {
			t.Fatalf("MarketplaceInfo() = %#v, %v", detail, err)
		}
		refresh, err := client.RefreshMarketplace(ctx)
		if err != nil || len(refresh.Sources) != 0 {
			t.Fatalf("RefreshMarketplace() = %#v, %v", refresh, err)
		}
	})

	t.Run("Should reject invalid identity and scope before transport", func(t *testing.T) {
		t.Parallel()
		client := &daemonClient{}
		ctx := t.Context()
		if _, err := client.MarketplaceInfo(
			ctx,
			"   ",
			"",
			"",
			MarketplaceReadScope{Scope: "user"},
		); err == nil ||
			!strings.Contains(err.Error(), "marketplace entry ID is required") {
			t.Fatalf("MarketplaceInfo(blank entry) = %v", err)
		}
		for _, tc := range []struct {
			scope   MarketplaceReadScope
			message string
		}{
			{MarketplaceReadScope{Scope: "workspace"}, "--scope workspace requires --workspace"},
			{MarketplaceReadScope{Scope: "user", WorkspaceID: "ws-alpha"}, "--workspace requires --scope workspace"},
			{MarketplaceReadScope{Scope: "user", Profile: "marketing"}, "--profile requires --scope profile"},
			{MarketplaceReadScope{Scope: "profile"}, "--scope profile requires an active non-default profile"},
		} {
			if _, err := client.SearchMarketplace(
				ctx,
				"",
				20,
				"",
				tc.scope,
			); err == nil ||
				!strings.Contains(err.Error(), tc.message) {
				t.Fatalf("SearchMarketplace(%#v) = %v, want %q", tc.scope, err, tc.message)
			}
		}
	})

	t.Run("Should carry profile identity in installed-state reads", func(t *testing.T) {
		t.Parallel()
		client := &daemonClient{
			target: LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/api/marketplace" || req.URL.Query().Get("scope") != "profile" ||
					req.URL.Query().Get("profile") != "marketing" {
					t.Fatalf("profile catalog request = %s", req.URL)
				}
				return newHTTPResponse(
					http.StatusOK,
					`{"total":0,"revision":"revision-a","stale":false,"items":[],"sources":[]}`,
				), nil
			})},
		}
		_, err := client.SearchMarketplace(context.Background(), "", 20, "", MarketplaceReadScope{
			Scope: contract.SettingsLayeredScopeProfile, Profile: "marketing",
		})
		if err != nil {
			t.Fatalf("SearchMarketplace(profile) = %v", err)
		}
	})
}
