package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

func TestSQLiteStoreReplaceKind(t *testing.T) {
	t.Parallel()

	t.Run("Should replace one kind atomically and prune absent entries", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		first := testDocument(now,
			testEntry(KindMCP, "alpha", "Alpha server", "First server"),
			testEntry(KindMCP, "beta", "Beta server", "Second server"),
		)
		if err := store.ReplaceKind(ctx, KindMCP, first); err != nil {
			t.Fatalf("ReplaceKind(first) error = %v", err)
		}

		second := testDocument(now.Add(time.Hour),
			testEntry(KindMCP, "beta", "Beta server", "Updated description"),
		)
		if err := store.ReplaceKind(ctx, KindMCP, second); err != nil {
			t.Fatalf("ReplaceKind(second) error = %v", err)
		}

		page, err := store.ListKind(ctx, KindMCP, "", 0, 10)
		if err != nil {
			t.Fatalf("ListKind() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want {
			t.Fatalf("ListKind() entries = %d, want %d", got, want)
		}
		if got, want := page.Entries[0].EntryID, "beta"; got != want {
			t.Fatalf("ListKind() entry id = %q, want %q", got, want)
		}
		if got, want := page.Entries[0].Description, "Updated description"; got != want {
			t.Fatalf("ListKind() description = %q, want %q", got, want)
		}
		state, err := store.KindState(ctx, KindMCP)
		if err != nil {
			t.Fatalf("KindState() error = %v", err)
		}
		if state.Stale || state.EntryCount != 1 || !state.FetchedAt.Equal(second.FetchedAt) {
			t.Fatalf("KindState() = %#v, want fresh state for one entry", state)
		}
	})

	t.Run("Should leave the previous projection untouched when replacement validation fails", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		if err := store.ReplaceKind(ctx, KindSkill, testDocument(
			now,
			testEntry(KindSkill, "stable", "Stable skill", "Preserved row"),
		)); err != nil {
			t.Fatalf("ReplaceKind(valid) error = %v", err)
		}

		invalid := testDocument(now.Add(time.Hour), testEntry(KindSkill, "", "Invalid", "Missing id"))
		err := store.ReplaceKind(ctx, KindSkill, invalid)
		if err == nil || !strings.Contains(err.Error(), "identity fields are required") {
			t.Fatalf("ReplaceKind(invalid) error = %v, want identity-fields validation", err)
		}
		page, err := store.ListKind(ctx, KindSkill, "", 0, 10)
		if err != nil {
			t.Fatalf("ListKind() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want || page.Entries[0].EntryID != "stable" {
			t.Fatalf("ListKind() = %#v, want preserved stable entry", page)
		}
	})
}

func TestSQLiteStoreQueriesAndStaleState(t *testing.T) {
	t.Parallel()

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
		if err := store.ReplaceKind(ctx, KindExtension, document); err != nil {
			t.Fatalf("ReplaceKind() error = %v", err)
		}

		page, err := store.ListKind(ctx, KindExtension, "telemetry", 0, 1)
		if err != nil {
			t.Fatalf("ListKind(query) error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want {
			t.Fatalf("ListKind(query) entries = %d, want %d", got, want)
		}
		if got, want := page.Total, 2; got != want {
			t.Fatalf("ListKind(query) total = %d, want %d", got, want)
		}
		if got, want := page.Entries[0].EntryID, "beta"; got != want {
			t.Fatalf("ListKind(query) first id = %q, want deterministic %q", got, want)
		}
		nextPage, err := store.ListKind(ctx, KindExtension, "telemetry", 1, 1)
		if err != nil {
			t.Fatalf("ListKind(next page) error = %v", err)
		}
		if got, want := nextPage.Total, 2; got != want || len(nextPage.Entries) != 1 ||
			nextPage.Entries[0].EntryID != "alpha" {
			t.Fatalf("ListKind(next page) = %#v, want second stable match and exact total %d", nextPage, want)
		}
		byName, err := store.ListKind(ctx, KindExtension, "cost guard", 0, 10)
		if err != nil {
			t.Fatalf("ListKind(name query) error = %v", err)
		}
		if got, want := len(byName.Entries), 1; got != want || byName.Entries[0].EntryID != "gamma" {
			t.Fatalf("ListKind(name query) = %#v, want gamma", byName)
		}
		byDescription, err := store.ListKind(ctx, KindExtension, "exports traces", 0, 10)
		if err != nil {
			t.Fatalf("ListKind(description query) error = %v", err)
		}
		if got, want := len(byDescription.Entries), 1; got != want || byDescription.Entries[0].EntryID != "alpha" {
			t.Fatalf("ListKind(description query) = %#v, want alpha", byDescription)
		}
		caseInsensitive, err := store.ListKind(ctx, KindExtension, "TELEMETRY", 0, 10)
		if err != nil {
			t.Fatalf("ListKind(case-insensitive query) error = %v", err)
		}
		if got, want := len(caseInsensitive.Entries), 2; got != want {
			t.Fatalf("ListKind(case-insensitive query) = %#v, want alpha and beta", caseInsensitive)
		}
		unicodeFolded, err := store.ListKind(ctx, KindExtension, "résumé", 0, 10)
		if err != nil {
			t.Fatalf("ListKind(Unicode-folded query) error = %v", err)
		}
		if got, want := len(unicodeFolded.Entries), 1; got != want || unicodeFolded.Entries[0].EntryID != "resume" {
			t.Fatalf("ListKind(Unicode-folded query) = %#v, want resume", unicodeFolded)
		}
		fullFolded, err := store.ListKind(ctx, KindExtension, "STRASSE", 0, 10)
		if err != nil {
			t.Fatalf("ListKind(full-fold query) error = %v", err)
		}
		if got, want := len(fullFolded.Entries), 1; got != want || fullFolded.Entries[0].EntryID != "strasse" {
			t.Fatalf("ListKind(full-fold query) = %#v, want strasse", fullFolded)
		}
		canonicalEquivalent, err := store.ListKind(ctx, KindExtension, "Café", 0, 10)
		if err != nil {
			t.Fatalf("ListKind(canonical-equivalence query) error = %v", err)
		}
		if got, want := len(canonicalEquivalent.Entries), 1; got != want ||
			canonicalEquivalent.Entries[0].EntryID != "cafe" {
			t.Fatalf("ListKind(canonical-equivalence query) = %#v, want cafe", canonicalEquivalent)
		}
		empty, err := store.ListKind(ctx, KindExtension, "not-present", 0, 10)
		if err != nil {
			t.Fatalf("ListKind(zero-result query) error = %v", err)
		}
		if len(empty.Entries) != 0 {
			t.Fatalf("ListKind(zero-result query) = %#v, want empty", empty)
		}

		entry, err := store.GetEntry(ctx, KindExtension, "beta")
		if err != nil {
			t.Fatalf("GetEntry() error = %v", err)
		}
		if got, want := entry.Name, "Audit log"; got != want {
			t.Fatalf("GetEntry().Name = %q, want %q", got, want)
		}
		_, err = store.GetEntry(ctx, KindExtension, "missing")
		if !errors.Is(err, ErrEntryNotFound) {
			t.Fatalf("GetEntry(missing) error = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("Should mark state stale without deleting prior entries", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		if err := store.ReplaceKind(ctx, KindMCP, testDocument(
			now,
			testEntry(KindMCP, "server", "MCP server", "Available while offline"),
		)); err != nil {
			t.Fatalf("ReplaceKind() error = %v", err)
		}
		if err := store.MarkKindStale(ctx, KindMCP, errorClassNetwork, "token=[REDACTED]"); err != nil {
			t.Fatalf("MarkKindStale() error = %v", err)
		}

		state, err := store.KindState(ctx, KindMCP)
		if err != nil {
			t.Fatalf("KindState() error = %v", err)
		}
		if !state.Stale || state.ErrorClass != errorClassNetwork || state.LastError != "token=[REDACTED]" {
			t.Fatalf("KindState() = %#v, want stale network state", state)
		}
		page, err := store.ListKind(ctx, KindMCP, "", 0, 10)
		if err != nil {
			t.Fatalf("ListKind() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want {
			t.Fatalf("ListKind() entries = %d, want %d preserved", got, want)
		}
	})
}

func TestSQLiteStoreResolvesExactExtensionInstall(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve an extension by exact install slug and version", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		entry := testEntry(KindExtension, "telemetry", "Telemetry", "Exports traces")
		entry.Version = "2.1.0"
		if err := store.ReplaceKind(ctx, KindExtension, testDocument(now, entry)); err != nil {
			t.Fatalf("ReplaceKind() error = %v", err)
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

func TestSQLiteStoreListsCuratedSkillsByInstallSlugs(t *testing.T) {
	t.Parallel()

	t.Run("Should batch exact skill slugs without crossing marketplace kinds", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
		alpha := testEntry(KindSkill, "alpha", "Alpha", "Alpha skill")
		beta := testEntry(KindSkill, "beta", "Beta", "Beta skill")
		if err := store.ReplaceKind(ctx, KindSkill, testDocument(now, beta, alpha)); err != nil {
			t.Fatalf("ReplaceKind(skills) error = %v", err)
		}
		extension := testEntry(KindExtension, "extension-alpha", "Extension Alpha", "Same slug")
		extension.InstallSlug = alpha.InstallSlug
		if err := store.ReplaceKind(ctx, KindExtension, testDocument(now, extension)); err != nil {
			t.Fatalf("ReplaceKind(extension) error = %v", err)
		}

		entries, err := store.ListSkillsByInstallSlugs(ctx, []string{
			" compozy/beta ",
			"compozy/alpha",
			"compozy/beta",
			"",
		})
		if err != nil {
			t.Fatalf("ListSkillsByInstallSlugs() error = %v", err)
		}
		if len(entries) != 2 || entries[0].EntryID != "alpha" || entries[1].EntryID != "beta" {
			t.Fatalf("ListSkillsByInstallSlugs() = %#v, want alpha then beta", entries)
		}
	})

	t.Run("Should reject a list with no non-blank skill slug", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		_, err := store.ListSkillsByInstallSlugs(testutil.Context(t), []string{"", "  "})
		if err == nil || !strings.Contains(err.Error(), "skill install slugs are required") {
			t.Fatalf("ListSkillsByInstallSlugs(blank) error = %v, want slug validation", err)
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
		kind     Kind
		document *Document
		wantErr  string
	}{
		{
			name: "Should reject an unsupported kind", kind: Kind("bundle"),
			document: testDocument(now), wantErr: "unsupported kind",
		},
		{name: "Should reject a missing document", kind: KindSkill, wantErr: "document is required"},
		{
			name:     "Should reject a future manifest",
			kind:     KindSkill,
			document: &Document{ManifestVersion: 2, GeneratedAt: now, FetchedAt: now},
			wantErr:  "client too old",
		},
		{
			name:     "Should reject missing timestamps",
			kind:     KindSkill,
			document: &Document{ManifestVersion: ManifestVersion},
			wantErr:  "generated_at and fetched_at are required",
		},
		{
			name:     "Should reject an entry from another kind",
			kind:     KindSkill,
			document: testDocument(now, testEntry(KindMCP, "wrong", "Wrong", "Wrong kind")),
			wantErr:  "kind = \"mcp\"",
		},
		{
			name: "Should reject duplicate entry ids",
			kind: KindSkill,
			document: testDocument(
				now,
				testEntry(KindSkill, "dupe", "One", "First"),
				testEntry(KindSkill, "dupe", "Two", "Second"),
			),
			wantErr: "is duplicated",
		},
		{
			name: "Should reject duplicate extension install slugs",
			kind: KindExtension,
			document: func() *Document {
				first := testEntry(KindExtension, "first", "First", "First extension")
				second := testEntry(KindExtension, "second", "Second", "Second extension")
				second.InstallSlug = first.InstallSlug
				return testDocument(now, first, second)
			}(),
			wantErr: "install_slug \"compozy/first\" is duplicated",
		},
		{
			name: "Should reject duplicate skill install slugs",
			kind: KindSkill,
			document: func() *Document {
				first := testEntry(KindSkill, "first", "First", "First skill")
				second := testEntry(KindSkill, "second", "Second", "Second skill")
				second.InstallSlug = first.InstallSlug
				return testDocument(now, first, second)
			}(),
			wantErr: "install_slug \"compozy/first\" is duplicated",
		},
		{
			name: "Should reject invalid payload JSON",
			kind: KindSkill,
			document: testDocument(now, Entry{
				Kind:        KindSkill,
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
			err := store.ReplaceKind(ctx, tc.kind, tc.document)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ReplaceKind() error = %v, want %q", err, tc.wantErr)
			}
		})
	}

	t.Run("Should reject a blank entry id", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		_, err := store.GetEntry(testutil.Context(t), KindSkill, "   ")
		if err == nil || !strings.Contains(err.Error(), "entry id is required") {
			t.Fatalf("GetEntry(blank) error = %v, want entry-id validation", err)
		}
	})

	t.Run("Should reject an unsupported stale kind", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		err := store.MarkKindStale(testutil.Context(t), Kind("bundle"), "validation", "bad kind")
		if err == nil || !strings.Contains(err.Error(), "unsupported kind") {
			t.Fatalf("MarkKindStale(unsupported kind) error = %v, want kind validation", err)
		}
	})

	t.Run("Should reject a nil list context", func(t *testing.T) {
		t.Parallel()

		store := openMarketplaceTestStore(t)
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		_, err := store.ListKind(nil, KindSkill, "", 0, 10)
		if err == nil || !strings.Contains(err.Error(), "store context is required") {
			t.Fatalf("ListKind(nil context) error = %v, want context validation", err)
		}
	})

	t.Run("Should reject a nil store receiver", func(t *testing.T) {
		t.Parallel()

		var nilStore *SQLiteStore
		_, err := nilStore.KindState(testutil.Context(t), KindSkill)
		if err == nil || !strings.Contains(err.Error(), "SQLite store is required") {
			t.Fatalf("KindState(nil store) error = %v, want store validation", err)
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
