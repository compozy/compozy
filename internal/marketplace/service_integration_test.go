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

	"github.com/compozy/agh/internal/testutil"
)

func TestCatalogServiceHTTPProjectionIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should project every curated kind through HTTP and SQLite", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			var body string
			switch request.URL.Path {
			case "/mcp.json":
				body = validMCPDocumentJSON()
			case "/extensions.json":
				body = validExtensionDocumentJSON()
			case "/skills.json":
				body = validSkillDocumentJSON()
			default:
				http.NotFound(writer, request)
				return
			}
			if _, err := writer.Write([]byte(body)); err != nil {
				t.Errorf("write feed response: %v", err)
			}
		}))
		t.Cleanup(server.Close)

		client := &http.Client{Timeout: time.Second}
		sources := make([]Source, 0, len(AllKinds()))
		for _, kind := range AllKinds() {
			source, err := NewHTTPSource(kind, server.URL, client)
			if err != nil {
				t.Fatalf("NewHTTPSource(%q) error = %v", kind, err)
			}
			sources = append(sources, source)
		}
		service, err := NewService(openMarketplaceTestStore(t), sources, time.Hour, time.Minute)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		ctx := testutil.Context(t)
		if _, err := service.Refresh(ctx); err != nil {
			t.Fatalf("Refresh(all kinds) error = %v", err)
		}
		for kind, wantEntryID := range map[Kind]string{
			KindMCP:       "filesystem",
			KindExtension: "bridge-github",
			KindSkill:     "agh",
		} {
			result, err := service.Browse(ctx, kind, "", 0, 10)
			if err != nil {
				t.Fatalf("Browse(%q) error = %v", kind, err)
			}
			if got, want := len(result.Entries), 1; got != want || result.Entries[0].EntryID != wantEntryID {
				t.Fatalf("Browse(%q) = %#v, want only %q", kind, result.Entries, wantEntryID)
			}
		}
	})

	t.Run("Should prune pulled entries and preserve the last good projection across source failures", func(t *testing.T) {
		t.Parallel()

		feed := &mutableCatalogFeed{body: validSkillFeed("stable", "Stable skill", "pulled", "Pulled skill")}
		server := httptest.NewServer(feed)
		t.Cleanup(server.Close)

		store := openMarketplaceTestStore(t)
		source, err := NewHTTPSource(KindSkill, server.URL, &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatalf("NewHTTPSource() error = %v", err)
		}
		service, err := NewService(store, []Source{source}, time.Hour, time.Minute)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		ctx := testutil.Context(t)

		if _, err := service.Refresh(ctx, KindSkill); err != nil {
			t.Fatalf("Refresh(valid) error = %v", err)
		}
		assertProjectedSkillIDs(t, ctx, store, "pulled", "stable")

		feed.setBody(validSkillFeed("stable", "Stable skill"))
		if _, err := service.Refresh(ctx, KindSkill); err != nil {
			t.Fatalf("Refresh(kill switch) error = %v", err)
		}
		assertProjectedSkillIDs(t, ctx, store, "stable")

		feed.setBody(`{"manifest_version":1,"generated_at":"2026-07-13T12:00:00Z","entries":[{"entry_id":"broken"}]}`)
		if _, err := service.Refresh(ctx, KindSkill); err == nil || !strings.Contains(err.Error(), "name is required") {
			t.Fatalf("Refresh(malformed) error = %v, want required-name validation failure", err)
		}
		assertProjectedSkillIDs(t, ctx, store, "stable")

		server.Close()
		if _, err := service.Refresh(ctx, KindSkill); err == nil || !strings.Contains(err.Error(), "connection refused") {
			t.Fatalf("Refresh(unavailable) error = %v, want connection-refused transport failure", err)
		}
		result, err := service.Browse(ctx, KindSkill, "", 0, 10)
		if err != nil {
			t.Fatalf("Browse(stale fallback) error = %v", err)
		}
		if !result.State.Stale || result.State.ErrorClass != errorClassNetwork {
			t.Fatalf("Browse(stale fallback) state = %#v, want stale network state", result.State)
		}
		assertProjectedSkillIDs(t, ctx, store, "stable")
		states, err := service.Status(ctx)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if got, want := len(states), 1; got != want || !states[0].Stale || states[0].ErrorClass != errorClassNetwork {
			t.Fatalf("Status() = %#v, want one stale network state", states)
		}
	})
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

func validSkillFeed(fields ...string) string {
	entries := make([]string, 0, len(fields)/2)
	for index := 0; index < len(fields); index += 2 {
		entryID := fields[index]
		name := fields[index+1]
		entries = append(entries, `{"entry_id":"`+entryID+`","name":"`+name+`",`+
			`"description":"Integration fixture","version":"1.0.0","install_slug":"compozy/`+entryID+`"}`)
	}
	return `{"manifest_version":1,"generated_at":"2026-07-13T12:00:00Z","entries":[` +
		strings.Join(entries, ",") + `]}`
}

func assertProjectedSkillIDs(t *testing.T, ctx context.Context, store Store, wantEntryIDs ...string) {
	t.Helper()
	page, err := store.ListKind(ctx, KindSkill, "", 0, 10)
	if err != nil {
		t.Fatalf("ListKind() error = %v", err)
	}
	if got, want := len(page.Entries), len(wantEntryIDs); got != want {
		t.Fatalf("ListKind() count = %d, want %d: %#v", got, want, page)
	}
	for index, wantEntryID := range wantEntryIDs {
		if page.Entries[index].EntryID != wantEntryID {
			t.Fatalf("ListKind()[%d].EntryID = %q, want %q", index, page.Entries[index].EntryID, wantEntryID)
		}
	}
}
