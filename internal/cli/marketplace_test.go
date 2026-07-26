package cli

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

func TestMarketplaceCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should render grouped search as the shared JSON contract", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceSearchRecord{Kinds: []contract.MarketplaceKindResult{{
			Kind: "skill",
			Items: []contract.MarketplaceListingPayload{{
				Kind: "skill", EntryID: "skill-entry", Name: "Review", Version: "1.2.0",
				Installed: true, Source: "curated",
			}},
		}}}
		deps := newTestDeps(t, &stubClient{
			searchMarketplaceFn: func(
				_ context.Context,
				query string,
				limit int,
				scope MarketplaceReadScope,
			) (MarketplaceSearchRecord, error) {
				if query != "review" {
					t.Fatalf("query = %q, want review", query)
				}
				if limit != 7 {
					t.Fatalf("limit = %d, want 7", limit)
				}
				if scope.Scope != contract.SettingsWorkspaceScopeWorkspace || scope.WorkspaceID != "ws-alpha" {
					t.Fatalf("read scope = %#v, want workspace ws-alpha", scope)
				}
				return want, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"marketplace",
			"search",
			"review",
			"--limit",
			"7",
			"--scope",
			"workspace",
			"--workspace",
			"ws-alpha",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("marketplace search command error = %v", err)
		}
		var got MarketplaceSearchRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(marketplace search) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marketplace search = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject a non-positive search limit before transport", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newTestDeps(t, &stubClient{searchMarketplaceFn: func(
			context.Context,
			string,
			int,
			MarketplaceReadScope,
		) (MarketplaceSearchRecord, error) {
			called = true
			return MarketplaceSearchRecord{}, nil
		}})
		_, _, err := executeRootCommand(t, deps, "marketplace", "search", "--limit", "0")
		if err == nil || !strings.Contains(err.Error(), "marketplace limit must be positive") {
			t.Fatalf("marketplace search --limit=0 error = %v, want positive-limit validation", err)
		}
		if called {
			t.Fatal("marketplace transport called after local limit validation failure")
		}
	})

	t.Run("Should render one kind without changing the daemon payload", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceKindRecord{
			Kind: "extension",
			Items: []MarketplaceListingRecord{{
				Kind: "extension", EntryID: "extension-entry", Name: "Bridge", Source: "curated",
			}},
		}
		deps := newTestDeps(t, &stubClient{
			browseMarketplaceFn: func(
				_ context.Context,
				kind string,
				query string,
				limit int,
				cursor string,
				scope MarketplaceReadScope,
			) (MarketplaceKindRecord, error) {
				if kind != "extension" {
					t.Fatalf("kind = %q, want extension", kind)
				}
				if query != "" {
					t.Fatalf("query = %q, want empty", query)
				}
				if limit != marketplaceDefaultLimit {
					t.Fatalf("limit = %d, want %d", limit, marketplaceDefaultLimit)
				}
				if cursor != "page-two" {
					t.Fatalf("cursor = %q, want page-two", cursor)
				}
				if scope.Scope != contract.SettingsWorkspaceScopeGlobal || scope.WorkspaceID != "" {
					t.Fatalf("read scope = %#v, want global", scope)
				}
				return want, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t, deps, "marketplace", "search", "--kind", "extension", "--cursor", "page-two", "-o", "json",
		)
		if err != nil {
			t.Fatalf("marketplace kind search command error = %v", err)
		}
		var got MarketplaceKindRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(marketplace kind search) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marketplace kind search = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve one-kind continuation metadata in every non-JSON format", func(t *testing.T) {
		t.Parallel()

		total := 7
		response := MarketplaceKindRecord{
			Kind: "skill", Total: &total, NextCursor: "skill-page-two", Stale: true,
			ErrorClass: "network", Error: "serving cached catalog",
			Items: []MarketplaceListingRecord{{
				Kind: "skill", EntryID: "skill-entry", Name: "Reviewer", Source: "clawhub",
			}},
		}
		deps := newTestDeps(t, &stubClient{browseMarketplaceFn: func(
			context.Context,
			string,
			string,
			int,
			string,
			MarketplaceReadScope,
		) (MarketplaceKindRecord, error) {
			return response, nil
		}})

		for _, format := range []string{"human", "toon", "jsonl"} {
			t.Run("Should preserve continuation metadata in "+format+" format", func(t *testing.T) {
				t.Parallel()

				stdout, _, err := executeRootCommand(
					t, deps, "marketplace", "search", "--kind", "skill", "-o", format,
				)
				if err != nil {
					t.Fatalf("marketplace kind search -o %s error = %v", format, err)
				}
				if !strings.Contains(stdout, "skill-page-two") {
					t.Fatalf("marketplace %s output dropped next cursor: %s", format, stdout)
				}
				if !strings.Contains(stdout, "serving cached catalog") {
					t.Fatalf("marketplace %s output dropped page diagnostic: %s", format, stdout)
				}
				if format == "jsonl" {
					lines := strings.Split(strings.TrimSpace(stdout), "\n")
					if len(lines) != 2 {
						t.Fatalf("marketplace JSONL lines = %d, want item plus page; output=%q", len(lines), stdout)
					}
					var page marketplaceKindPageRecord
					if err := json.Unmarshal([]byte(lines[1]), &page); err != nil {
						t.Fatalf("json.Unmarshal(marketplace page) error = %v", err)
					}
					if page.Type != listPageRecordType || page.NextCursor != response.NextCursor ||
						page.Total == nil || *page.Total != total {
						t.Fatalf("marketplace JSONL page = %#v, want continuation metadata", page)
					}
				}
			})
		}
	})

	t.Run("Should resolve detail by kind and stable entry id", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceEntryRecord{Entry: MarketplaceListingRecord{
			Kind: "mcp", EntryID: "github-mcp", Name: "GitHub", Source: "curated",
		}}
		deps := newTestDeps(t, &stubClient{
			marketplaceInfoFn: func(
				_ context.Context,
				kind string,
				entryID string,
				installedName string,
				scope MarketplaceReadScope,
			) (MarketplaceEntryRecord, error) {
				if kind != "mcp" {
					t.Fatalf("kind = %q, want mcp", kind)
				}
				if entryID != "github-mcp" {
					t.Fatalf("entryID = %q, want github-mcp", entryID)
				}
				if installedName != "custom-github" {
					t.Fatalf("installedName = %q, want custom-github", installedName)
				}
				if scope.Scope != contract.SettingsWorkspaceScopeWorkspace || scope.WorkspaceID != "ws-alpha" {
					t.Fatalf("read scope = %#v, want workspace ws-alpha", scope)
				}
				return want, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"marketplace",
			"info",
			"mcp",
			"github-mcp",
			"--installed-name",
			"custom-github",
			"--scope",
			"workspace",
			"--workspace",
			"ws-alpha",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("marketplace info command error = %v", err)
		}
		var got MarketplaceEntryRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(marketplace info) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marketplace info = %#v, want %#v", got, want)
		}
	})

	t.Run("Should refresh the selected feed-backed kind", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceRefreshRecord{Kinds: []contract.MarketplaceRefreshKindPayload{{
			Kind: "skill", Outcome: "updated", EntryCount: 3,
		}}}
		deps := newTestDeps(t, &stubClient{
			refreshMarketplaceFn: func(_ context.Context, kind string) (MarketplaceRefreshRecord, error) {
				if kind != "skill" {
					t.Fatalf("kind = %q, want skill", kind)
				}
				return want, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t, deps, "marketplace", "refresh", "--kind", "skill", "-o", "json",
		)
		if err != nil {
			t.Fatalf("marketplace refresh command error = %v", err)
		}
		var got MarketplaceRefreshRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(marketplace refresh) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marketplace refresh = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject incomplete installed-state scope flags before transport", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		cases := []struct {
			name    string
			args    []string
			wantErr string
		}{
			{
				name:    "Should require a workspace ID for workspace scope",
				args:    []string{"marketplace", "search", "--scope", "workspace"},
				wantErr: "--scope workspace requires --workspace",
			},
			{
				name: "Should reject a workspace ID for global scope",
				args: []string{
					"marketplace", "info", "mcp", "github-mcp", "--scope", "global", "--workspace", "ws-alpha",
				},
				wantErr: "--workspace requires --scope workspace",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				_, _, err := executeRootCommand(t, deps, tc.args...)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("executeRootCommand() error = %v, want containing %q", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("Should reject a continuation cursor without one marketplace kind", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newTestDeps(t, &stubClient{searchMarketplaceFn: func(
			context.Context,
			string,
			int,
			MarketplaceReadScope,
		) (MarketplaceSearchRecord, error) {
			called = true
			return MarketplaceSearchRecord{}, nil
		}})
		_, _, err := executeRootCommand(t, deps, "marketplace", "search", "--cursor", "page-two")
		if err == nil || !strings.Contains(err.Error(), "--cursor requires --kind") {
			t.Fatalf("marketplace search --cursor error = %v, want kind validation", err)
		}
		if called {
			t.Fatal("marketplace transport called after local cursor validation failure")
		}
	})
}
