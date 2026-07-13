package decisionlog

// Suite: decisionlog format validator.
// Invariant: a decision log conforms to the AD-NNN.md + DECISIONS.md contract —
//   required frontmatter, status enum, evidence-required-for-proven, six-field
//   index grammar, active-proven-only membership, bidirectional supersession,
//   no broken body references, and a valid empty state.
// Boundary IN: parsing/validation of hand-authored log fixtures (pure functions + fs.FS).
// Boundary OUT: the capture skill's LLM behavior (E2E evals) and install/gitignore (IT-*).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// UT-001: a fixture AD-001.md with all required frontmatter and a Reconciliation
// section parses into a valid record with no errors; a candidate with empty
// evidence is also valid (UT-002 happy half).
func TestParseDecisionRecordValid(t *testing.T) {
	t.Parallel()
	t.Run("full proven record parses every frontmatter field", func(t *testing.T) {
		t.Parallel()
		meta, err := parseDecisionRecord(readFixture(t, "record-valid.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertField(t, "id", meta.ID, "AD-001")
		assertField(t, "title", meta.Title, "Event-sourcing for orders")
		assertField(t, "status", meta.Status, statusProven)
		assertField(t, "source_slug", meta.SourceSlug, "feat-orders")
		assertField(t, "source_adr", meta.SourceADR, "adrs/adr-002.md")
		assertField(t, "promoted_at", meta.PromotedAt, "2026-07-11")
		assertField(t, "evidence", meta.Evidence,
			"cy-final-verify report p99<200ms; diff abc123; issue_003 resolved")
		if !equalStringSlice(meta.Tags, []string{"orders", "async"}) {
			t.Fatalf("tags = %v, want [orders async]", meta.Tags)
		}
		if len(meta.Supersedes) != 0 {
			t.Fatalf("supersedes = %v, want empty", meta.Supersedes)
		}
		if meta.SupersededBy != "" {
			t.Fatalf("superseded_by = %q, want empty", meta.SupersededBy)
		}
	})
	t.Run("candidate with empty evidence is valid", func(t *testing.T) {
		t.Parallel()
		meta, err := parseDecisionRecord(readFixture(t, "record-candidate-no-evidence.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertField(t, "status", meta.Status, statusCandidate)
		if meta.Evidence != "" {
			t.Fatalf("evidence = %q, want empty", meta.Evidence)
		}
	})
}

// UT-002 (error half) + UT-003: proven-without-evidence, out-of-enum status, a
// missing required field, and a missing Reconciliation section are all rejected.
func TestParseDecisionRecordErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fixture string
		wantErr error
	}{
		{"proven with empty evidence is rejected", "record-proven-no-evidence.md", errEvidenceReq},
		{"status outside the enum is rejected", "record-bad-status.md", errInvalidStatus},
		{"missing required field is rejected", "record-missing-field.md", errMissingField},
		{"missing reconciliation section is rejected", "record-no-reconciliation.md", errMissingSection},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseDecisionRecord(readFixture(t, tc.fixture))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
	t.Run("invalid status error names the offending value", func(t *testing.T) {
		t.Parallel()
		_, err := parseDecisionRecord(readFixture(t, "record-bad-status.md"))
		if !strings.Contains(err.Error(), "accepted") {
			t.Fatalf("err = %v, want it to name %q", err, "accepted")
		}
	})
}

// UT-004: a canonical six-field line parses into its fields; lines that violate
// the grammar (wrong field count, malformed id, unbracketed tags) are rejected.
func TestParseIndexLine(t *testing.T) {
	t.Parallel()
	t.Run("six-field line parses all fields", func(t *testing.T) {
		t.Parallel()
		const line = "AD-001 | Event-sourcing | proven | [orders, async] | terse rationale | feat-orders"
		got, err := parseIndexLine(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertField(t, "id", got.ID, "AD-001")
		assertField(t, "title", got.Title, "Event-sourcing")
		assertField(t, "status", got.Status, statusProven)
		assertField(t, "rationale", got.Rationale, "terse rationale")
		assertField(t, "source_slug", got.SourceSlug, "feat-orders")
		if !equalStringSlice(got.Tags, []string{"orders", "async"}) {
			t.Fatalf("tags = %v, want [orders async]", got.Tags)
		}
	})
	grammar := []struct {
		name string
		line string
	}{
		{"fewer than six fields", "AD-001 | Event-sourcing | proven | [orders] | rationale"},
		{"more than six fields", "AD-001 | title | proven | [x] | rationale | slug | extra"},
		{"malformed id", "AD1 | Event-sourcing | proven | [orders] | rationale | feat-orders"},
		{"unbracketed tags", "AD-001 | Event-sourcing | proven | orders | rationale | feat-orders"},
	}
	for _, tc := range grammar {
		tc := tc
		t.Run(tc.name+" is a grammar error", func(t *testing.T) {
			t.Parallel()
			if _, err := parseIndexLine(tc.line); !errors.Is(err, errIndexGrammar) {
				t.Fatalf("err = %v, want errIndexGrammar", err)
			}
		})
	}
}

// UT-005 + UT-007: a proven-only index parses every line; an empty-state index
// is valid with zero records; a candidate or superseded row is rejected.
func TestValidateIndex(t *testing.T) {
	t.Parallel()
	t.Run("proven-only index parses every line", func(t *testing.T) {
		t.Parallel()
		lines, err := validateIndex(readFixture(t, "index-valid.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 2 {
			t.Fatalf("lines = %d, want 2", len(lines))
		}
	})
	t.Run("empty-state index is valid with zero records", func(t *testing.T) {
		t.Parallel()
		lines, err := validateIndex(readFixture(t, "index-empty.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 0 {
			t.Fatalf("lines = %d, want 0", len(lines))
		}
	})
	membership := []struct {
		name    string
		fixture string
	}{
		{"candidate row", "index-candidate-row.md"},
		{"superseded row", "index-superseded-row.md"},
	}
	for _, tc := range membership {
		tc := tc
		t.Run(tc.name+" is rejected", func(t *testing.T) {
			t.Parallel()
			if _, err := validateIndex(readFixture(t, tc.fixture)); !errors.Is(err, errIndexNotProven) {
				t.Fatalf("err = %v, want errIndexNotProven", err)
			}
		})
	}
	t.Run("duplicate id row is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := validateIndex(readFixture(t, "index-duplicate-row.md"))
		if !errors.Is(err, errDuplicateID) {
			t.Fatalf("err = %v, want errDuplicateID", err)
		}
		if !strings.Contains(err.Error(), "AD-001") {
			t.Fatalf("err = %v, want it to name AD-001", err)
		}
	})
	t.Run("descending ids are rejected as unsorted", func(t *testing.T) {
		t.Parallel()
		_, err := validateIndex(readFixture(t, "index-unsorted.md"))
		if !errors.Is(err, errIndexUnsorted) {
			t.Fatalf("err = %v, want errIndexUnsorted", err)
		}
		if !strings.Contains(err.Error(), "AD-001") || !strings.Contains(err.Error(), "AD-004") {
			t.Fatalf("err = %v, want it to name AD-001 and AD-004", err)
		}
	})
	// Ordering is numeric on the id suffix, not lexical: a plain string compare
	// would misorder AD-1000 before AD-999. These two cases pin that behavior.
	t.Run("ascending multi-width ids pass numeric ordering", func(t *testing.T) {
		t.Parallel()
		const idx = "# Project Decisions (active, proven)\n\n" +
			"AD-999 | Nine ninety-nine | proven | [x] | r | s\n" +
			"AD-1000 | One thousand | proven | [x] | r | s\n"
		if _, err := validateIndex(idx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("descending multi-width ids are rejected", func(t *testing.T) {
		t.Parallel()
		const idx = "# Project Decisions (active, proven)\n\n" +
			"AD-1000 | One thousand | proven | [x] | r | s\n" +
			"AD-999 | Nine ninety-nine | proven | [x] | r | s\n"
		if _, err := validateIndex(idx); !errors.Is(err, errIndexUnsorted) {
			t.Fatalf("err = %v, want errIndexUnsorted", err)
		}
	})
}

// UT-006: bidirectional supersession is valid; a one-sided link is a broken-link
// error; a chain A->B->C is valid and leaves only C active.
func TestValidateSupersession(t *testing.T) {
	t.Parallel()
	t.Run("bidirectional pair is valid", func(t *testing.T) {
		t.Parallel()
		pair := []DecisionRecordMeta{
			{ID: "AD-001", Status: statusSuperseded, SupersededBy: "AD-002"},
			{ID: "AD-002", Status: statusProven, Supersedes: []string{"AD-001"}},
		}
		if err := validateSupersession(pair); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("one-sided link is a broken-link error", func(t *testing.T) {
		t.Parallel()
		oneSided := []DecisionRecordMeta{
			{ID: "AD-001", Status: statusSuperseded, SupersededBy: "AD-002"},
			{ID: "AD-002", Status: statusProven},
		}
		if err := validateSupersession(oneSided); !errors.Is(err, errBrokenLink) {
			t.Fatalf("err = %v, want errBrokenLink", err)
		}
	})
	t.Run("chain A->B->C is valid and leaves only C active", func(t *testing.T) {
		t.Parallel()
		chain := []DecisionRecordMeta{
			{ID: "AD-001", Status: statusSuperseded, SupersededBy: "AD-002"},
			{ID: "AD-002", Status: statusSuperseded, SupersededBy: "AD-003", Supersedes: []string{"AD-001"}},
			{ID: "AD-003", Status: statusProven, Supersedes: []string{"AD-002"}},
		}
		if err := validateSupersession(chain); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if active := activeHeads(chain); !equalStringSlice(active, []string{"AD-003"}) {
			t.Fatalf("active heads = %v, want [AD-003]", active)
		}
	})
	t.Run("superseded_by on a non-superseded record is a status/link mismatch", func(t *testing.T) {
		t.Parallel()
		linkedButActive := []DecisionRecordMeta{
			{ID: "AD-001", Status: statusProven, SupersededBy: "AD-002"},
			{ID: "AD-002", Status: statusProven, Supersedes: []string{"AD-001"}},
		}
		if err := validateSupersession(linkedButActive); !errors.Is(err, errStatusLinkMismatch) {
			t.Fatalf("err = %v, want errStatusLinkMismatch", err)
		}
	})
	t.Run("superseded status without a successor is a dangling head", func(t *testing.T) {
		t.Parallel()
		danglingHead := []DecisionRecordMeta{
			{ID: "AD-001", Status: statusSuperseded},
		}
		if err := validateSupersession(danglingHead); !errors.Is(err, errStatusLinkMismatch) {
			t.Fatalf("err = %v, want errStatusLinkMismatch", err)
		}
	})
}

// UT-008: a valid golden log passes; an index line whose body file is missing is
// a broken-reference error that names the dangling id.
func TestValidateLog(t *testing.T) {
	t.Parallel()
	t.Run("valid golden log passes", func(t *testing.T) {
		t.Parallel()
		if err := validateLog(os.DirFS(filepath.Join("testdata", "valid-log"))); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("index line with no body names the dangling id", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, indexFileName),
			"# Project Decisions (active, proven)\n\n"+
				"AD-001 | Event-sourcing for orders | proven | [orders, async] | audit + replay | feat-orders\n"+
				"AD-009 | Missing body | proven | [orders] | dangling reference | feat-orders\n")
		writeFile(t, filepath.Join(dir, decisionsDirName, "AD-001"+recordFileExt), validRecordBody("AD-001"))
		err := validateLog(os.DirFS(dir))
		if !errors.Is(err, errBrokenRef) {
			t.Fatalf("err = %v, want errBrokenRef", err)
		}
		if !strings.Contains(err.Error(), "AD-009") {
			t.Fatalf("err = %v, want it to name AD-009", err)
		}
	})
	t.Run("record body whose id does not match its filename is rejected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, indexFileName), "# Project Decisions (active, proven)\n")
		// File named AD-002.md but frontmatter claims AD-007 (a copy-paste slip);
		// neither id is in the index, so only the load-time identity check catches it.
		writeFile(t, filepath.Join(dir, decisionsDirName, "AD-002"+recordFileExt), validRecordBody("AD-007"))
		err := validateLog(os.DirFS(dir))
		if !errors.Is(err, errIDFilenameMismatch) {
			t.Fatalf("err = %v, want errIDFilenameMismatch", err)
		}
		if !strings.Contains(err.Error(), "AD-007") {
			t.Fatalf("err = %v, want it to name AD-007", err)
		}
	})
	t.Run("index line whose title/source_slug drift from the body is rejected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// validRecordBody("AD-001") declares title "Event-sourcing for orders" and
		// source_slug "feat-orders"; the index row denormalizes a contradicting
		// title and slug, so the line loads a record that disagrees with its body.
		writeFile(t, filepath.Join(dir, indexFileName),
			"# Project Decisions (active, proven)\n\n"+
				"AD-001 | Wrong Title | proven | [orders, async] | audit + replay | wrong-slug\n")
		writeFile(t, filepath.Join(dir, decisionsDirName, "AD-001"+recordFileExt), validRecordBody("AD-001"))
		err := validateLog(os.DirFS(dir))
		if !errors.Is(err, errIndexBodyMismatch) {
			t.Fatalf("err = %v, want errIndexBodyMismatch", err)
		}
		if !strings.Contains(err.Error(), "AD-001") {
			t.Fatalf("err = %v, want it to name AD-001", err)
		}
	})
	t.Run("index line whose tags drift from the body is rejected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// validRecordBody("AD-001") declares tags [orders, async]; the index keeps the
		// title and slug but denormalizes a stale tag list. Because the index is the
		// scoped-loading surface, a wrong tag must fail validation, not pass silently.
		writeFile(t, filepath.Join(dir, indexFileName),
			"# Project Decisions (active, proven)\n\n"+
				"AD-001 | Event-sourcing for orders | proven | [orders, sync] | audit + replay | feat-orders\n")
		writeFile(t, filepath.Join(dir, decisionsDirName, "AD-001"+recordFileExt), validRecordBody("AD-001"))
		err := validateLog(os.DirFS(dir))
		if !errors.Is(err, errIndexBodyMismatch) {
			t.Fatalf("err = %v, want errIndexBodyMismatch", err)
		}
		if !strings.Contains(err.Error(), "AD-001") {
			t.Fatalf("err = %v, want it to name AD-001", err)
		}
	})
	t.Run("active-proven body absent from the index is rejected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Empty-state index, but a proven, active body exists on disk — the reverse
		// membership half (body -> index) must flag the silently dropped decision.
		writeFile(t, filepath.Join(dir, indexFileName),
			"# Project Decisions (active, proven)\n\n# No active, proven decisions captured yet.\n")
		writeFile(t, filepath.Join(dir, decisionsDirName, "AD-001"+recordFileExt), validRecordBody("AD-001"))
		err := validateLog(os.DirFS(dir))
		if !errors.Is(err, errMissingIndexLine) {
			t.Fatalf("err = %v, want errMissingIndexLine", err)
		}
		if !strings.Contains(err.Error(), "AD-001") {
			t.Fatalf("err = %v, want it to name AD-001", err)
		}
	})
}

// --- test helpers ---

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertField(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", field, got, want)
	}
}

func equalStringSlice(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// activeHeads returns the ids of records with no superseded_by pointer — the
// active head(s) a valid chain must resolve to (test-local; not a production API).
func activeHeads(records []DecisionRecordMeta) []string {
	ids := make([]string, 0, len(records))
	for i := range records {
		if records[i].SupersededBy == "" {
			ids = append(ids, records[i].ID)
		}
	}
	return ids
}

func validRecordBody(id string) string {
	return "---\n" +
		"id: " + id + "\n" +
		"title: Event-sourcing for orders\n" +
		"status: proven\n" +
		"tags: [orders, async]\n" +
		"source_slug: feat-orders\n" +
		"source_adr: adrs/adr-002.md\n" +
		"promoted_at: 2026-07-11\n" +
		"supersedes: []\n" +
		"superseded_by: null\n" +
		"evidence: \"diff abc123; verify passed\"\n" +
		"---\n\n" +
		"## Context\n\nc\n\n## Decision\n\nd\n\n" +
		"## Alternatives\n\na\n\n## Consequences\n\nq\n\n" +
		"## Reconciliation\n\nImplemented as designed.\n"
}
