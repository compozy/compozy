//go:build integration

package marketplace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

func TestCatalogServiceHTTPProjectionIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should project the extension catalog through HTTP and SQLite", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			if request.URL.Path != "/v3/extensions.json" {
				http.NotFound(writer, request)
				return
			}
			body := validExtensionDocumentJSON()
			if _, err := writer.Write([]byte(body)); err != nil {
				t.Errorf("write feed response: %v", err)
			}
		}))
		t.Cleanup(server.Close)

		client := &http.Client{Timeout: time.Second}
		source, err := NewHTTPSource(KindExtension, server.URL, client)
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewService(openMarketplaceTestStore(t), source, time.Hour, time.Minute)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		ctx := testutil.Context(t)
		if _, err := service.Refresh(ctx); err != nil {
			t.Fatalf("Refresh(source) error = %v", err)
		}
		result, err := service.Browse(ctx, "", 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Entries) != 1 || result.Entries[0].EntryID != "bridge-github" {
			t.Fatalf("Browse() = %#v, want the published extension", result)
		}
	})

	t.Run(
		"Should prune pulled entries and preserve the last good projection across source failures",
		func(t *testing.T) {
			t.Parallel()

			feed := &mutableCatalogFeed{
				body: validExtensionFeed("stable", "Stable extension", "pulled", "Pulled extension"),
			}
			server := httptest.NewServer(feed)
			t.Cleanup(server.Close)

			store := openMarketplaceTestStore(t)
			source, err := NewHTTPSource(KindExtension, server.URL, &http.Client{Timeout: time.Second})
			if err != nil {
				t.Fatalf("NewHTTPSource() error = %v", err)
			}
			service, err := NewService(store, source, time.Hour, time.Minute)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			ctx := testutil.Context(t)

			if _, err := service.Refresh(ctx); err != nil {
				t.Fatalf("Refresh(valid) error = %v", err)
			}
			assertProjectedExtensionIDs(t, ctx, store, "pulled", "stable")

			feed.setBody(validExtensionFeed("stable", "Stable extension"))
			if _, err := service.Refresh(ctx); err != nil {
				t.Fatalf("Refresh(kill switch) error = %v", err)
			}
			assertProjectedExtensionIDs(t, ctx, store, "stable")

			feed.setBody(
				`{"manifest_version": 3,"generated_at":"2026-07-13T12:00:00Z","entries":[{"entry_id":"broken"}]}`,
			)
			if _, err := service.Refresh(
				ctx,
			); err == nil ||
				!strings.Contains(err.Error(), "name is required") {
				t.Fatalf("Refresh(malformed) error = %v, want required-name validation failure", err)
			}
			assertProjectedExtensionIDs(t, ctx, store, "stable")

			server.Close()
			if _, err := service.Refresh(
				ctx,
			); err == nil ||
				!strings.Contains(err.Error(), "connection refused") {
				t.Fatalf("Refresh(unavailable) error = %v, want connection-refused transport failure", err)
			}
			result, err := service.Browse(ctx, "", 0, 10)
			if err != nil {
				t.Fatalf("Browse(stale fallback) error = %v", err)
			}
			if !result.State.Stale || result.State.ErrorClass != errorClassNetwork {
				t.Fatalf("Browse(stale fallback) state = %#v, want stale network state", result.State)
			}
			assertProjectedExtensionIDs(t, ctx, store, "stable")
			states, err := service.Status(ctx)
			if err != nil {
				t.Fatalf("Status() error = %v", err)
			}
			if got, want := len(
				states,
			), 1; got != want || !states[0].Stale ||
				states[0].ErrorClass != errorClassNetwork {
				t.Fatalf("Status() = %#v, want one stale network state", states)
			}
		},
	)
}

type mutableCatalogFeed struct {
	mu   sync.RWMutex
	body string
}

func (f *mutableCatalogFeed) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	writer.Header().Set("Content-Type", "application/json")
	if _, err := writer.Write([]byte(f.body)); err != nil {
		return
	}
}

func (f *mutableCatalogFeed) setBody(body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.body = body
}

func validExtensionFeed(fields ...string) string {
	entries := make([]string, 0, len(fields)/2)
	for index := 0; index < len(fields); index += 2 {
		entryID := fields[index]
		name := fields[index+1]
		entries = append(entries, `{"entry_id":"`+entryID+`","name":"`+name+`",`+
			`"description":"Integration fixture","version":"1.0.0","install_slug":"compozy/`+entryID+`",`+
			`"tier":"official","artifact_url":"https://example.test/package.tgz","digest_sha256":"`+strings.Repeat("a", 64)+`"}`)
	}
	return `{"manifest_version": 3,"generated_at":"2026-07-13T12:00:00Z","entries":[` +
		strings.Join(entries, ",") + `]}`
}

func assertProjectedExtensionIDs(t *testing.T, ctx context.Context, store Store, wantEntryIDs ...string) {
	t.Helper()
	page, err := store.BrowseSource(ctx, CompozyCatalogSource, "", 0, 10)
	if err != nil {
		t.Fatalf("BrowseSource() error = %v", err)
	}
	if got, want := len(page.Entries), len(wantEntryIDs); got != want {
		t.Fatalf("BrowseSource() count = %d, want %d: %#v", got, want, page)
	}
	for index, wantEntryID := range wantEntryIDs {
		if page.Entries[index].EntryID != wantEntryID {
			t.Fatalf("BrowseSource()[%d].EntryID = %q, want %q", index, page.Entries[index].EntryID, wantEntryID)
		}
	}
}
