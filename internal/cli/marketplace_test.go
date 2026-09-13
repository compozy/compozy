package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestMarketplaceCommands(t *testing.T) {
	t.Parallel()
	t.Run("Should expose experimental help and fail refresh only when every source failed", func(t *testing.T) {
		t.Parallel()
		for _, args := range [][]string{{"marketplace", "sources", "--help"}, {"marketplace", "sources", "add", "--help"}} {
			stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, &stubClient{}), args...)
			if err != nil || !strings.Contains(stdout, "Stability: experimental") {
				t.Fatalf("source help = %q, %v", stdout, err)
			}
		}
		for _, success := range []bool{false, true} {
			rows := []contract.MarketplaceRefreshSourcePayload{{Source: "team", Outcome: "failed"}}
			if success {
				rows = append(
					rows,
					contract.MarketplaceRefreshSourcePayload{Source: "compozy-catalog", Outcome: "succeeded"},
				)
			}
			deps := newWorkspaceTestDeps(
				t,
				&stubClient{refreshMarketplaceFn: func(context.Context) (MarketplaceRefreshRecord, error) {
					return MarketplaceRefreshRecord{Sources: rows}, nil
				}},
			)
			stdout, _, err := executeRootCommand(t, deps, "marketplace", "refresh", "-o", "json")
			if (err == nil) != success {
				t.Fatalf("refresh success=%v: %v", success, err)
			}
			var response contract.MarketplaceRefreshResponse
			if decodeErr := json.Unmarshal(
				[]byte(stdout),
				&response,
			); decodeErr != nil ||
				len(response.Sources) != len(rows) {
				t.Fatalf("refresh lost outcomes: %s, %v", stdout, decodeErr)
			}
		}
	})

	t.Run("Should render catalog search as the shared JSON contract", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceListRecord{Total: 1, Revision: "revision-a",
			Items: []contract.MarketplaceListingPayload{{
				EntryID: "skill-entry", Name: "Review", Version: "1.2.0",
				Installed: true, Source: "curated",
			}},
		}
		deps := newDefaultProfileWorkspaceTestDeps(t, &stubClient{
			searchMarketplaceFn: func(
				_ context.Context,
				query string,
				limit int,
				_ string,
				scope MarketplaceReadScope,
			) (MarketplaceListRecord, error) {
				if query != "review" {
					t.Fatalf("query = %q, want review", query)
				}
				if limit != 7 {
					t.Fatalf("limit = %d, want 7", limit)
				}
				if scope.Scope != contract.SettingsLayeredScopeWorkspace || scope.WorkspaceID != "ws-alpha" {
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
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &fields); err != nil {
			t.Fatalf("json.Unmarshal(marketplace search fields) error = %v", err)
		}
		if _, found := fields["resolution_source"]; found {
			t.Fatalf("marketplace search JSON contains resolution_source, want shared daemon payload: %s", stdout)
		}
		var got MarketplaceListRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(marketplace search) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marketplace search = %#v, want %#v", got, want)
		}
	})

	t.Run("Should isolate installed state by the active profile", func(t *testing.T) {
		t.Parallel()

		client := &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{searchMarketplaceFn: func(
				_ context.Context,
				_ string,
				_ int,
				_ string,
				scope MarketplaceReadScope,
			) (MarketplaceListRecord, error) {
				if scope.Scope != contract.SettingsLayeredScopeProfile || scope.Profile != "marketing" ||
					scope.WorkspaceID != "" {
					t.Fatalf("profile marketplace scope = %#v", scope)
				}
				return MarketplaceListRecord{}, nil
			}}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active"},
			}},
		}
		_, _, err := executeRootCommand(
			t, newTestDeps(t, client),
			"--profile", "marketing", "marketplace", "search", "--scope", "profile", "-o", "json",
		)
		if err != nil {
			t.Fatalf("profile marketplace search error = %v", err)
		}
	})

	t.Run("Should reject a non-positive search limit before transport", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newWorkspaceTestDeps(t, &stubClient{searchMarketplaceFn: func(
			context.Context,
			string,
			int,
			string,
			MarketplaceReadScope,
		) (MarketplaceListRecord, error) {
			called = true
			return MarketplaceListRecord{}, nil
		}})
		_, _, err := executeRootCommand(t, deps, "marketplace", "search", "--limit", "0")
		if err == nil || !strings.Contains(err.Error(), "marketplace limit must be positive") {
			t.Fatalf("marketplace search --limit=0 error = %v, want positive-limit validation", err)
		}
		if called {
			t.Fatal("marketplace transport called after local limit validation failure")
		}
	})

	t.Run("Should render a catalog page without changing the daemon payload", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceListRecord{
			Items: []MarketplaceListingRecord{{
				EntryID: "extension-entry", Name: "Bridge", Source: "curated",
			}},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			searchMarketplaceFn: func(
				_ context.Context,
				query string,
				limit int,
				cursor string,
				scope MarketplaceReadScope,
			) (MarketplaceListRecord, error) {
				if query != "" {
					t.Fatalf("query = %q, want empty", query)
				}
				if limit != marketplaceDefaultLimit {
					t.Fatalf("limit = %d, want %d", limit, marketplaceDefaultLimit)
				}
				if cursor != "page-two" {
					t.Fatalf("cursor = %q, want page-two", cursor)
				}
				if scope.Scope != contract.SettingsLayeredScopeUser || scope.WorkspaceID != "" {
					t.Fatalf("read scope = %#v, want user", scope)
				}
				return want, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t, deps, "marketplace", "search", "--cursor", "page-two", "-o", "json",
		)
		if err != nil {
			t.Fatalf("marketplace kind search command error = %v", err)
		}
		var got MarketplaceListRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(marketplace kind search) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marketplace kind search = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve catalog continuation metadata in every non-JSON format", func(t *testing.T) {
		t.Parallel()

		total := 7
		response := MarketplaceListRecord{
			Total: total, NextCursor: "skill-page-two", Stale: true,
			ErrorClass: "network", Error: "serving cached catalog",
			Items: []MarketplaceListingRecord{{
				EntryID: "skill-entry", Name: "Reviewer", Source: "clawhub",
			}},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{searchMarketplaceFn: func(
			context.Context,
			string,
			int,
			string,
			MarketplaceReadScope,
		) (MarketplaceListRecord, error) {
			return response, nil
		}})

		for _, format := range []string{"human", "toon", "jsonl"} {
			t.Run("Should preserve continuation metadata in "+format+" format", func(t *testing.T) {
				t.Parallel()

				stdout, _, err := executeRootCommand(
					t, deps, "marketplace", "search", "-o", format,
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
					var page marketplaceCatalogPageRecord
					if err := json.Unmarshal([]byte(lines[1]), &page); err != nil {
						t.Fatalf("json.Unmarshal(marketplace page) error = %v", err)
					}
					if page.Type != listPageRecordType || page.NextCursor != response.NextCursor ||
						page.Total != total {
						t.Fatalf("marketplace JSONL page = %#v, want continuation metadata", page)
					}
				}
			})
		}
	})

	t.Run("Should resolve detail by source and stable entry id", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceEntryRecord{Entry: MarketplaceListingRecord{
			EntryID: "github-mcp", Name: "GitHub", Source: "curated",
		}}
		deps := newDefaultProfileWorkspaceTestDeps(t, &stubClient{
			marketplaceInfoFn: func(
				_ context.Context,
				entryID string,
				source string,
				installedName string,
				scope MarketplaceReadScope,
			) (MarketplaceEntryRecord, error) {
				if entryID != "github-mcp" || source != "compozy-catalog" {
					t.Fatalf("entryID = %q, want github-mcp", entryID)
				}
				if installedName != "custom-github" {
					t.Fatalf("installedName = %q, want custom-github", installedName)
				}
				if scope.Scope != contract.SettingsLayeredScopeWorkspace || scope.WorkspaceID != "ws-alpha" {
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
			"github-mcp",
			"--source",
			"compozy-catalog",
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
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &fields); err != nil {
			t.Fatalf("json.Unmarshal(marketplace info fields) error = %v", err)
		}
		if _, found := fields["resolution_source"]; found {
			t.Fatalf("marketplace info JSON contains resolution_source, want shared daemon payload: %s", stdout)
		}
		var got MarketplaceEntryRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(marketplace info) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marketplace info = %#v, want %#v", got, want)
		}
	})

	t.Run("Should refresh the catalog", func(t *testing.T) {
		t.Parallel()

		want := MarketplaceRefreshRecord{Sources: []contract.MarketplaceRefreshSourcePayload{{
			Source: "compozy-catalog", Outcome: "updated", EntryCount: 3,
		}}}
		deps := newWorkspaceTestDeps(t, &stubClient{
			refreshMarketplaceFn: func(_ context.Context) (MarketplaceRefreshRecord, error) {
				return want, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t, deps, "marketplace", "refresh", "-o", "json",
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

		deps := newWorkspaceTestDeps(t, &stubClient{})
		cases := []struct {
			name    string
			args    []string
			wantErr string
		}{
			{
				name: "Should reject a workspace ID for user scope",
				args: []string{
					"marketplace", "info", "github-mcp", "--scope", "user", "--workspace", "ws-alpha",
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

	t.Run("Should infer workspace read scope from cwd", func(t *testing.T) {
		t.Parallel()

		deps := newDefaultProfileWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				if ref != "/workspace/project/nested" {
					t.Fatalf("GetWorkspace() ref = %q, want nested cwd", ref)
				}
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws-project", RootDir: "/workspace/project"},
				}, nil
			},
			searchMarketplaceFn: func(
				_ context.Context,
				_ string,
				_ int,
				_ string,
				scope MarketplaceReadScope,
			) (MarketplaceListRecord, error) {
				if scope.Scope != contract.SettingsLayeredScopeWorkspace ||
					scope.WorkspaceID != "ws-project" {
					t.Fatalf("marketplace scope = %#v, want workspace ws-project", scope)
				}
				return MarketplaceListRecord{}, nil
			},
		})
		deps.getwd = func() (string, error) { return "/workspace/project/nested", nil }

		if _, _, err := executeRootCommand(
			t,
			deps,
			"marketplace",
			"search",
			"--scope",
			"workspace",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("marketplace search with inferred workspace error = %v", err)
		}
	})

	for _, args := range [][]string{
		{"marketplace", "search", "--kind", "extension"},
		{"marketplace", "refresh", "--kind", "extension"},
		{"marketplace", "info", "extension", "review"},
	} {
		t.Run("Should reject retired "+strings.Join(args, " ")+" before client access", func(t *testing.T) {
			t.Parallel()
			deps := commandDeps{newClient: func(ClientTarget) (DaemonClient, error) {
				t.Fatal("retired command opened a client")
				return nil, nil
			}}
			_, _, err := executeRootCommand(t, deps, args...)
			want := "unknown flag"
			if args[1] == "info" {
				want = "accepts 1 arg(s)"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("retired command error = %v, want %q", err, want)
			}
		})
	}
}

// Invariant: source failures retain actionable wire fields and invalid input exits 2.
// Owner: CLI API decoding and error output; canonical Marketplace suite.
func TestMarketplaceSourceErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"Should render a suggested name", http.StatusConflict, `{"error":"Name exists","code":"marketplace_source_exists","suggested_name":"team-2"}`},
		{"Should render retained instances", http.StatusConflict, `{"error":"Name retained","code":"marketplace_source_name_retained","retained_by":["tool"]}`},
		{"Should render checked document paths", http.StatusUnprocessableEntity, `{"error":"Not a marketplace","code":"marketplace_not_a_marketplace","checked":["marketplace.json",".claude-plugin/marketplace.json"]}`},
		{"Should reject an invalid source name", http.StatusUnprocessableEntity, `{"error":"Invalid name","code":"marketplace_source_name_invalid"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, err := parseMarketplaceSourceAPIError(tc.status, http.StatusText(tc.status), []byte(tc.body))
			if !matched || err == nil {
				t.Fatal("source error not recognized")
			}
			var output bytes.Buffer
			if code := writeExecutionError(
				&output,
				[]string{"marketplace", "sources", "add", "fixture", "-o", "json"},
				err,
			); code != 2 {
				t.Fatalf("exit = %d, output = %s", code, &output)
			}
			var got, want contract.MarketplaceSourceErrorPayload
			if err := json.Unmarshal(output.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("error metadata lost: %+v", got)
			}
		})
	}
}

// Invariant: config source addressing preserves the full registered name, including dots.
// Owner: CLI config path decoder; canonical Marketplace CLI suite.
func TestMarketplaceSourceConfigPath(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"team", "team.plugins"} {
		t.Run("Should preserve "+name, func(t *testing.T) {
			t.Parallel()
			path, kind, redacted, err := configMutationPath("marketplace.plugin_sources." + name + ".enabled")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(path, []string{"marketplace", "plugin_sources", name, "enabled"}) ||
				kind != configSetBool ||
				redacted {
				t.Fatalf("source config path = %#v, %v, %v", path, kind, redacted)
			}
		})
	}
}
