package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
)

func TestSQLiteStoreReplaceSource(t *testing.T) {
	t.Parallel()

	t.Run("Should replace one source atomically and prune absent entries", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		first := testDocument(now,
			testEntry(KindExtension, "alpha", "Alpha server", "First server"),
			testEntry(KindExtension, "beta", "Beta server", "Second server"),
		)
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, first); err != nil {
			t.Fatalf("ReplaceSource(first) error = %v", err)
		}

		second := testDocument(now.Add(time.Hour),
			testEntry(KindExtension, "beta", "Beta server", "Updated description"),
		)
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, second); err != nil {
			t.Fatalf("ReplaceSource(second) error = %v", err)
		}

		page, err := store.BrowseSource(ctx, CompozyCatalogSource, "", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want {
			t.Fatalf("BrowseSource() entries = %d, want %d", got, want)
		}
		if got, want := page.Entries[0].EntryID, "beta"; got != want {
			t.Fatalf("BrowseSource() entry id = %q, want %q", got, want)
		}
		if got, want := page.Entries[0].Description, "Updated description"; got != want {
			t.Fatalf("BrowseSource() description = %q, want %q", got, want)
		}
		state, err := store.SourceState(ctx, CompozyCatalogSource)
		if err != nil {
			t.Fatalf("SourceState() error = %v", err)
		}
		if state.Stale || state.EntryCount != 1 || !state.FetchedAt.Equal(second.FetchedAt) {
			t.Fatalf("SourceState() = %#v, want fresh state for one entry", state)
		}
	})

	t.Run("Should leave the previous projection untouched when replacement validation fails", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, testDocument(
			now,
			testEntry(KindExtension, "stable", "Stable skill", "Preserved row"),
		)); err != nil {
			t.Fatalf("ReplaceSource(valid) error = %v", err)
		}

		invalid := testDocument(now.Add(time.Hour), testEntry(KindExtension, "", "Invalid", "Missing id"))
		err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, invalid)
		if err == nil || !strings.Contains(err.Error(), "identity fields are required") {
			t.Fatalf("ReplaceSource(invalid) error = %v, want identity-fields validation", err)
		}
		page, err := store.BrowseSource(ctx, CompozyCatalogSource, "", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want || page.Entries[0].EntryID != "stable" {
			t.Fatalf("BrowseSource() = %#v, want preserved stable entry", page)
		}
	})
}

func TestMarketplaceSourceStateRejectsUnrepresentableStoredCounts(t *testing.T) {
	t.Parallel()

	t.Run("Should reject negative durable fields", func(t *testing.T) {
		t.Parallel()

		for _, row := range []storepkg.MarketplaceCatalogState{
			{ManifestVersion: -1},
			{ManifestVersion: 1, EntryCount: -1},
		} {
			if _, err := marketplaceSourceStateFromRow(row); err == nil {
				t.Fatalf("marketplaceSourceStateFromRow(%#v) error = nil, want range failure", row)
			}
		}
	})

	t.Run("Should reject values wider than a 32 bit int", func(t *testing.T) {
		t.Parallel()

		value := int64(^uint32(0)>>1) + 1
		if _, err := storedCatalogIntForSize(value, "entry_count", 32); err == nil {
			t.Fatalf("storedCatalogIntForSize(%d, 32) error = nil, want overflow", value)
		}
		if got, err := storedCatalogIntForSize(42, "entry_count", 32); err != nil || got != 42 {
			t.Fatalf("storedCatalogIntForSize(42, 32) = %d, %v, want 42, nil", got, err)
		}
	})
}

func TestSQLiteStoreQueriesAndStaleState(t *testing.T) {
	t.Parallel()

	// Invariant: equal entry IDs and freshness records remain isolated by source.
	// Owner: catalog persistence; canonical suite: TestSQLiteStoreQueriesAndStaleState.
	t.Run("Should scope equal entry IDs and stale state to their source", func(t *testing.T) {
		t.Parallel()
		catalog := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		for _, source := range []string{CompozyCatalogSource, "partner"} {
			document := testDocument(at, testEntry(KindExtension, "shared", source, "Source isolation"))
			if err := catalog.ReplaceSource(ctx, source, 0, document); err != nil {
				t.Fatal(err)
			}
		}
		if err := catalog.MarkSourceStale(ctx, "partner", 0, errorClassNetwork, "unavailable"); err != nil {
			t.Fatal(err)
		}
		for _, source := range []string{CompozyCatalogSource, "partner"} {
			entry, err := catalog.GetEntry(ctx, source, "shared")
			if err != nil {
				t.Fatal(err)
			}
			if entry.SourceName != source || entry.Name != source {
				t.Fatalf("GetEntry(%q) = %#v", source, entry)
			}
			state, err := catalog.SourceState(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			if state.Source != source || state.Stale != (source == "partner") || state.EntryCount != 1 {
				t.Fatalf("SourceState(%q) = %#v", source, state)
			}
		}
		if _, err := catalog.GetEntry(ctx, "missing", "shared"); !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("GetEntry(missing) = %v", err)
		}
		if _, err := catalog.SourceState(ctx, "missing"); !errors.Is(err, ErrSourceStateMissing) {
			t.Fatalf("SourceState(missing) = %v", err)
		}
	})

	t.Run("Should filter name and description, honor limit, and return detail", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		document := testDocument(now,
			testEntry(KindExtension, "alpha", "Telemetry bridge", "Exports traces"),
			testEntry(KindExtension, "beta", "Audit log", "Records telemetry events"),
			testEntry(KindExtension, "gamma", "Cost guard", "Enforces budgets"),
			testEntry(KindExtension, "resume", "RÉSUMÉ helper", "Indexes accented metadata"),
			testEntry(KindExtension, "strasse", "Straße tools", "Indexes German names"),
			testEntry(KindExtension, "cafe", "Cafe\u0301 index", "Decomposed canonical text"),
		)
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, document); err != nil {
			t.Fatalf("ReplaceSource() error = %v", err)
		}

		page, err := store.BrowseSource(ctx, CompozyCatalogSource, "telemetry", 0, 1)
		if err != nil {
			t.Fatalf("BrowseSource(query) error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want {
			t.Fatalf("BrowseSource(query) entries = %d, want %d", got, want)
		}
		if got, want := page.Total, 2; got != want {
			t.Fatalf("BrowseSource(query) total = %d, want %d", got, want)
		}
		if got, want := page.Entries[0].EntryID, "beta"; got != want {
			t.Fatalf("BrowseSource(query) first id = %q, want deterministic %q", got, want)
		}
		nextPage, err := store.BrowseSource(ctx, CompozyCatalogSource, "telemetry", 1, 1)
		if err != nil {
			t.Fatalf("BrowseSource(next page) error = %v", err)
		}
		if got, want := nextPage.Total, 2; got != want || len(nextPage.Entries) != 1 ||
			nextPage.Entries[0].EntryID != "alpha" {
			t.Fatalf("BrowseSource(next page) = %#v, want second stable match and exact total %d", nextPage, want)
		}
		byName, err := store.BrowseSource(ctx, CompozyCatalogSource, "cost guard", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource(name query) error = %v", err)
		}
		if got, want := len(byName.Entries), 1; got != want || byName.Entries[0].EntryID != "gamma" {
			t.Fatalf("BrowseSource(name query) = %#v, want gamma", byName)
		}
		byDescription, err := store.BrowseSource(ctx, CompozyCatalogSource, "exports traces", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource(description query) error = %v", err)
		}
		if got, want := len(byDescription.Entries), 1; got != want || byDescription.Entries[0].EntryID != "alpha" {
			t.Fatalf("BrowseSource(description query) = %#v, want alpha", byDescription)
		}
		caseInsensitive, err := store.BrowseSource(ctx, CompozyCatalogSource, "TELEMETRY", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource(case-insensitive query) error = %v", err)
		}
		if got, want := len(caseInsensitive.Entries), 2; got != want {
			t.Fatalf("BrowseSource(case-insensitive query) = %#v, want alpha and beta", caseInsensitive)
		}
		unicodeFolded, err := store.BrowseSource(ctx, CompozyCatalogSource, "résumé", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource(Unicode-folded query) error = %v", err)
		}
		if got, want := len(unicodeFolded.Entries), 1; got != want || unicodeFolded.Entries[0].EntryID != "resume" {
			t.Fatalf("BrowseSource(Unicode-folded query) = %#v, want resume", unicodeFolded)
		}
		fullFolded, err := store.BrowseSource(ctx, CompozyCatalogSource, "STRASSE", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource(full-fold query) error = %v", err)
		}
		if got, want := len(fullFolded.Entries), 1; got != want || fullFolded.Entries[0].EntryID != "strasse" {
			t.Fatalf("BrowseSource(full-fold query) = %#v, want strasse", fullFolded)
		}
		canonicalEquivalent, err := store.BrowseSource(ctx, CompozyCatalogSource, "Café", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource(canonical-equivalence query) error = %v", err)
		}
		if got, want := len(canonicalEquivalent.Entries), 1; got != want ||
			canonicalEquivalent.Entries[0].EntryID != "cafe" {
			t.Fatalf("BrowseSource(canonical-equivalence query) = %#v, want cafe", canonicalEquivalent)
		}
		empty, err := store.BrowseSource(ctx, CompozyCatalogSource, "not-present", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource(zero-result query) error = %v", err)
		}
		if len(empty.Entries) != 0 {
			t.Fatalf("BrowseSource(zero-result query) = %#v, want empty", empty)
		}

		entry, err := store.GetEntry(ctx, CompozyCatalogSource, "beta")
		if err != nil {
			t.Fatalf("GetEntry() error = %v", err)
		}
		if got, want := entry.Name, "Audit log"; got != want {
			t.Fatalf("GetEntry().Name = %q, want %q", got, want)
		}
		_, err = store.GetEntry(ctx, CompozyCatalogSource, "missing")
		if !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("GetEntry(missing) error = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("Should mark state stale without deleting prior entries", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, testDocument(
			now,
			testEntry(KindExtension, "server", "MCP server", "Available while offline"),
		)); err != nil {
			t.Fatalf("ReplaceSource() error = %v", err)
		}
		if err := store.MarkSourceStale(
			ctx,
			CompozyCatalogSource,
			0,
			errorClassNetwork,
			"token=[REDACTED]",
		); err != nil {
			t.Fatalf("MarkSourceStale() error = %v", err)
		}

		state, err := store.SourceState(ctx, CompozyCatalogSource)
		if err != nil {
			t.Fatalf("SourceState() error = %v", err)
		}
		if !state.Stale || state.ErrorClass != errorClassNetwork || state.LastError != "token=[REDACTED]" {
			t.Fatalf("SourceState() = %#v, want stale network state", state)
		}
		page, err := store.BrowseSource(ctx, CompozyCatalogSource, "", 0, 10)
		if err != nil {
			t.Fatalf("BrowseSource() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want {
			t.Fatalf("BrowseSource() entries = %d, want %d preserved", got, want)
		}
	})
}

func TestSQLiteStoreResolvesExactExtensionInstall(t *testing.T) {
	t.Parallel()

	// Invariant: canonical catalog slugs select immutable entry identity; retained slugs stay usable.
	t.Run("Should resolve canonical and retained slugs without crossing entry or version identity", func(t *testing.T) {
		t.Parallel()
		store := openMarketplaceTestStore(t)
		entry := testEntry(KindExtension, "herdr-bridge", "herdr bridge", "Bridge integration")
		entry.InstallSlug = "AlexandreAkao/herdr-bridge-compozy"
		collision := testEntry(KindExtension, "other", "Other", "Different package")
		collision.InstallSlug, collision.Version = "compozy/herdr-bridge", "2.0.0"
		if err := store.ReplaceSource(t.Context(), CompozyCatalogSource, 0,
			testDocument(time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), entry, collision)); err != nil {
			t.Fatal(err)
		}
		for _, slug := range []string{"compozy/herdr-bridge", entry.InstallSlug} {
			resolved, err := store.GetExtensionByInstallSlug(t.Context(), slug, "1.0.0")
			if err != nil {
				t.Fatalf("resolve(%s): %v", slug, err)
			}
			if resolved.EntryID != entry.EntryID || resolved.InstallSlug != slug ||
				resolved.DigestSHA256 != entry.DigestSHA256 {
				t.Fatalf("resolve(%s) = %#v, want exact entry and request slug", slug, resolved)
			}
		}
		if _, err := store.GetExtensionByInstallSlug(
			t.Context(),
			"compozy/herdr-bridge",
			"2.0.0",
		); !errors.Is(
			err,
			ErrEntryNotFound,
		) {
			t.Fatalf("wrong canonical version: %v, want not found instead of another entry", err)
		}
		persisted, err := store.GetEntry(t.Context(), CompozyCatalogSource, entry.EntryID)
		if err != nil || persisted.InstallSlug != entry.InstallSlug {
			t.Fatalf("persisted entry = %#v, %v, want unchanged feed acquisition identity", persisted, err)
		}
	})
	t.Run("Should resolve an extension by exact install slug and version", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		entry := testEntry(KindExtension, "telemetry", "Telemetry", "Exports traces")
		entry.Version = "2.1.0"
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, testDocument(now, entry)); err != nil {
			t.Fatalf("ReplaceSource() error = %v", err)
		}

		resolved, err := store.GetExtensionByInstallSlug(ctx, " compozy/telemetry ", " 2.1.0 ")
		if err != nil {
			t.Fatalf("GetExtensionByInstallSlug() error = %v", err)
		}
		if resolved.EntryID != "telemetry" || resolved.DigestSHA256 != entry.DigestSHA256 {
			t.Fatalf("GetExtensionByInstallSlug() = %#v, want exact telemetry row", resolved)
		}
		if _, err := store.GetExtensionByInstallSlug(
			ctx,
			entry.InstallSlug,
			"1.0.0",
		); !errors.Is(
			err,
			ErrEntryNotFound,
		) {
			t.Fatalf("GetExtensionByInstallSlug(wrong version) error = %v, want ErrEntryNotFound", err)
		}
	})
}

func TestSQLiteStoreRejectsInvalidInputsBeforeMutation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a missing database", func(t *testing.T) {
		t.Parallel()

		_, err := NewSQLiteStore(nil)
		if err == nil || !strings.Contains(err.Error(), "SQLite repository is required") {
			t.Fatalf("NewSQLiteStore(nil) error = %v, want database validation", err)
		}
	})
	store := openMarketplaceTestStore(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		document *Document
		wantErr  string
	}{
		{name: "Should reject a missing document", wantErr: "document is required"},
		{
			name:     "Should reject a future manifest",
			document: &Document{ManifestVersion: ManifestVersion + 1, GeneratedAt: now, FetchedAt: now},
			wantErr:  "client too old",
		},
		{
			name:     "Should reject missing timestamps",
			document: &Document{ManifestVersion: ManifestVersion},
			wantErr:  "generated_at and fetched_at are required",
		},

		{
			name: "Should reject duplicate entry ids",
			document: testDocument(
				now,
				testEntry(KindExtension, "dupe", "One", "First"),
				testEntry(KindExtension, "dupe", "Two", "Second"),
			),
			wantErr: "is duplicated",
		},
		{
			name: "Should reject duplicate extension install slugs",
			document: func() *Document {
				first := testEntry(KindExtension, "first", "First", "First extension")
				second := testEntry(KindExtension, "second", "Second", "Second extension")
				second.InstallSlug = first.InstallSlug
				return testDocument(now, first, second)
			}(),
			wantErr: "install_slug \"compozy/first\" is duplicated",
		},

		{
			name: "Should reject invalid payload JSON",
			document: testDocument(now, Entry{
				Kind:        KindExtension,
				EntryID:     "bad-json",
				Name:        "Bad JSON",
				Description: "Invalid payload",
				InstallSlug: "compozy/bad-json",
				Payload:     []byte("{"),
			}),
			wantErr: "payload_json is invalid",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, tc.document)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ReplaceSource() error = %v, want %q", err, tc.wantErr)
			}
		})
	}

	t.Run("Should reject a blank entry id", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		_, err := store.GetEntry(testutil.Context(t), CompozyCatalogSource, "   ")
		if err == nil || !strings.Contains(err.Error(), "entry id is required") {
			t.Fatalf("GetEntry(blank) error = %v, want entry-id validation", err)
		}
	})

	t.Run("Should reject a nil list context", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err := store.BrowseSource(nil, CompozyCatalogSource, "", 0, 10)
		if err == nil || !strings.Contains(err.Error(), "store context is required") {
			t.Fatalf("BrowseSource(nil context) error = %v, want context validation", err)
		}
	})

	t.Run("Should reject a nil store receiver", func(t *testing.T) {
		t.Parallel()

		var nilStore *SQLiteStore
		_, err := nilStore.SourceState(testutil.Context(t), CompozyCatalogSource)
		if err == nil || !strings.Contains(err.Error(), "SQLite store is required") {
			t.Fatalf("SourceState(nil store) error = %v, want store validation", err)
		}
	})
}

func openMarketplaceTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	ctx := testutil.Context(t)
	databasePath := filepath.Join(t.TempDir(), "marketplace-test.db")
	if err := marketplaceTestStoreSeed.Clone(databasePath); err != nil {
		t.Fatalf("marketplace store seed Clone() error = %v", err)
	}
	db, err := globaldb.OpenGlobalDB(ctx, databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := db.Close(closeCtx); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	return store
}

func testDocument(fetchedAt time.Time, entries ...Entry) *Document {
	return &Document{
		ManifestVersion: ManifestVersion,
		GeneratedAt:     fetchedAt.Add(-time.Minute),
		FetchedAt:       fetchedAt,
		Entries:         entries,
	}
}

func testEntry(kind Kind, entryID string, name string, description string) Entry {
	return Entry{
		Kind:         kind,
		EntryID:      entryID,
		Name:         name,
		Description:  description,
		Version:      "1.0.0",
		InstallSlug:  "compozy/" + entryID,
		DigestSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Tier:         "official",
		Payload:      json.RawMessage(`{"entry_id":"` + entryID + `"}`),
	}
}

// Invariant: content-identical refreshes retain their cursor revision across freshness and order changes.
// Owner: catalog source projection; canonical suite: store_test.go (UT-068).
func TestSQLiteStoreSourceContentRevision(t *testing.T) {
	t.Parallel()
	// Invariant: source projection retains v3 inputs through SQLite; owner: catalog persistence.
	t.Run("Should preserve v3 typed inputs and icon through the source projection", func(t *testing.T) {
		t.Parallel()
		store := openMarketplaceTestStore(t)
		document, err := DecodeDocument(KindExtension, v3ExtensionJSON(t, "https://images.example.test/icon.png"))
		if err != nil {
			t.Fatal(err)
		}
		document.FetchedAt = time.Now().UTC()
		if err := store.ReplaceSource(t.Context(), CompozyCatalogSource, 0, document); err != nil {
			t.Fatal(err)
		}
		result, err := store.BrowseSource(t.Context(), CompozyCatalogSource, "", 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Entries) != 1 || len(result.Entries[0].Inputs) != 2 ||
			result.Entries[0].Icon != document.Entries[0].Icon {
			t.Fatalf("projection = %#v", result)
		}
	})

	t.Run("Should keep the revision for identical content and change it for edited entries", func(t *testing.T) {
		t.Parallel()
		store := openMarketplaceTestStore(t)
		ctx := t.Context()
		at := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
		first := testDocument(
			at,
			testEntry(KindExtension, "b", "Beta", "Original"),
			testEntry(KindExtension, "a", "Alpha", "Original"),
		)
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, first); err != nil {
			t.Fatal(err)
		}
		before, err := store.SourceState(ctx, CompozyCatalogSource)
		if err != nil {
			t.Fatal(err)
		}
		refreshed := testDocument(at.Add(time.Hour), first.Entries[1], first.Entries[0])
		for i := range refreshed.Entries {
			refreshed.Entries[i].FetchedAt = refreshed.FetchedAt
		}
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, refreshed); err != nil {
			t.Fatal(err)
		}
		after, err := store.SourceState(ctx, CompozyCatalogSource)
		if err != nil {
			t.Fatal(err)
		}
		if before.Revision == "" || before.Revision != after.Revision || !after.FetchedAt.Equal(refreshed.FetchedAt) {
			t.Fatalf("before=%#v after=%#v", before, after)
		}
		refreshed.Entries[0].Description = "Changed"
		if err := store.ReplaceSource(ctx, CompozyCatalogSource, 0, refreshed); err != nil {
			t.Fatal(err)
		}
		changed, err := store.SourceState(ctx, CompozyCatalogSource)
		if err != nil {
			t.Fatal(err)
		}
		if changed.Revision == after.Revision {
			t.Fatal("edited content kept the old revision")
		}
	})
}
