package archmap

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Suite: Architecture Depth Map grammar
// Invariant: Parse accepts every canonical map and rejects each grammar violation with its typed class.
// Boundary IN: Serialized Architecture Depth Map fixtures and the archmap parser.
// Boundary OUT: Runtime skill emission, report generation, and filesystem mutation.

func TestParseValidFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixture string
		check   func(*testing.T, *Map)
	}{
		{
			name:    "UT-001 parses a two-area map",
			fixture: "valid.md",
			check: func(t *testing.T, got *Map) {
				t.Helper()
				want := &Map{Areas: []Area{
					{
						Name:    "apps/web",
						Audited: "2026-07-13",
						Report:  ".compozy/arch-reviews/apps-web.md",
						Entries: []Entry{
							{
								Kind:   "deep",
								Target: "apps/web/navigation",
								Note:   "Route new navigation behavior through this module.",
							},
							{
								Kind:   "seam",
								Target: "apps/web/router",
								Note:   "Do not widen the router integration seam.",
							},
							{
								Kind:   "avoid",
								Target: "merge route handlers",
								Note:   "Framework ownership keeps these boundaries load-bearing.",
								Date:   "2026-07-12",
							},
						},
					},
					{
						Name:    "internal/core",
						Audited: "2026-07-13",
						Report:  "-",
						Entries: []Entry{},
					},
				}}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Parse() mismatch\ngot:  %#v\nwant: %#v", got, want)
				}
			},
		},
		{
			name:    "UT-002 parses the canonical empty state",
			fixture: "empty.md",
			check: func(t *testing.T, got *Map) {
				t.Helper()
				if got.Areas != nil {
					t.Fatalf("Parse() Areas = %#v, want nil", got.Areas)
				}
			},
		},
		{
			name:    "UT-008 parses an area with zero entries",
			fixture: "zero-entry.md",
			check: func(t *testing.T, got *Map) {
				t.Helper()
				if len(got.Areas) != 1 {
					t.Fatalf("Parse() area count = %d, want 1", len(got.Areas))
				}
				if got.Areas[0].Entries == nil || len(got.Areas[0].Entries) != 0 {
					t.Fatalf("Parse() Entries = %#v, want non-nil empty slice", got.Areas[0].Entries)
				}
			},
		},
		{
			name:    "UT-009 skips superseded avoid provenance",
			fixture: "superseded.md",
			check: func(t *testing.T, got *Map) {
				t.Helper()
				if len(got.Areas) != 1 {
					t.Fatalf("Parse() area count = %d, want 1", len(got.Areas))
				}
				want := []Entry{
					{Kind: "deep", Target: "internal/core/router", Note: "Route dispatch through this module."},
				}
				if !reflect.DeepEqual(got.Areas[0].Entries, want) {
					t.Fatalf("Parse() Entries = %#v, want %#v", got.Areas[0].Entries, want)
				}
			},
		},
		{
			name:    "UT-011 does not cap large sections",
			fixture: "large-section.md",
			check: func(t *testing.T, got *Map) {
				t.Helper()
				const wantEntries = 205
				if len(got.Areas) != 1 {
					t.Fatalf("Parse() area count = %d, want 1", len(got.Areas))
				}
				if len(got.Areas[0].Entries) != wantEntries {
					t.Fatalf("Parse() entry count = %d, want %d", len(got.Areas[0].Entries), wantEntries)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(readFixture(t, test.fixture))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			test.check(t, got)
		})
	}
}

func TestParseInvalidFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fixture     string
		wantKind    ErrorKind
		wantMessage string
	}{
		{
			name:        "UT-003 rejects an unknown kind",
			fixture:     "unknown-kind.md",
			wantKind:    ErrorUnknownKind,
			wantMessage: "note",
		},
		{
			name:        "UT-004 rejects deep wrong arity",
			fixture:     "bad-arity-deep.md",
			wantKind:    ErrorArity,
			wantMessage: "expected 3",
		},
		{
			name:        "UT-004 rejects avoid wrong arity",
			fixture:     "bad-arity-avoid.md",
			wantKind:    ErrorArity,
			wantMessage: "expected 4",
		},
		{
			name:        "UT-005 rejects a literal pipe in a field",
			fixture:     "literal-pipe.md",
			wantKind:    ErrorReservedDelimiter,
			wantMessage: "reserved delimiter",
		},
		{name: "UT-006 rejects unsorted areas", fixture: "unsorted.md", wantKind: ErrorAreaOrder, wantMessage: "alpha"},
		{
			name:        "UT-007 rejects a malformed audit date",
			fixture:     "bad-header-date.md",
			wantKind:    ErrorDate,
			wantMessage: "2026-13-40",
		},
		{
			name:        "UT-007 rejects a malformed avoid date",
			fixture:     "bad-avoid-date.md",
			wantKind:    ErrorDate,
			wantMessage: "07-13",
		},
		{
			name:        "UT-010 rejects entries outside group order",
			fixture:     "out-of-order.md",
			wantKind:    ErrorGroupOrder,
			wantMessage: "deep, seam, avoid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(readFixture(t, test.fixture))
			if err == nil {
				t.Fatalf("Parse() = %#v, want error", got)
			}

			var parseErr *ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("Parse() error type = %T, want *ParseError", err)
			}
			if parseErr.Kind != test.wantKind {
				t.Fatalf("ParseError.Kind = %q, want %q (error: %v)", parseErr.Kind, test.wantKind, err)
			}
			if !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("Parse() error = %q, want substring %q", err, test.wantMessage)
			}
		})
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}
