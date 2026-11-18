package marketplace

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCategoryResolver(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, explicit, raw string
		want                CategoryID
	}{
		{"Should map database", "", "database", "data"},
		{"Should map uppercase separators", "", "DATA_ANALYTICS", "data"},
		{"Should map source words", "", "Data and Analytics", "data"},
		{"Should fall back for empty category", "", "", "more"},
		{"Should fall back for location", "", "location", "more"},
		{"Should prefer explicit category", "design", "development", "design"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolveCategory(tc.explicit, tc.raw); got != tc.want {
				t.Fatalf("category=%q want %q", got, tc.want)
			}
		})
	}
	t.Run("Should validate request ids strictly", func(t *testing.T) {
		t.Parallel()
		for _, c := range Categories() {
			id, err := ParseCategoryID(string(c.ID))
			if err != nil || id != c.ID {
				t.Fatalf("parse=%q,%v", id, err)
			}
		}
		if _, err := ParseCategoryID("apps"); !errors.Is(err, ErrUnknownCategory) {
			t.Fatal(err)
		}
	})
}
func TestCategoryProjection(t *testing.T) {
	t.Parallel()
	if curatedSource("") {
		t.Fatal("empty source grants curation")
	}
	for _, tc := range []struct {
		name, source, payload string
		want                  CategoryID
		featured              bool
	}{
		{"Should resolve old rows", "compozy-catalog", `{"category":"payments"}`, "finance", false},
		{"Should honor curated metadata", "compozy-plugins", `{"source_ref":"github:compozy/plugins","category":"database","x-compozy":{"category":"productivity","featured":true}}`, "productivity", true},
		{"Should ignore a custom source impersonating a curated name", "compozy-plugins", `{"source_ref":"github:attacker/plugins","category":"database","x-compozy":{"category":"productivity","featured":true,"supersedes":["official/x"]}}`, "data", false},
		{"Should ignore third party metadata", "third-party", `{"category":"database","compozy_category":"design","featured":true,"x-compozy":{"category":"productivity","featured":true}}`, "data", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			details, err := ProjectEntry(Entry{SourceName: tc.source, Payload: json.RawMessage(tc.payload)})
			if err != nil || details.Category.ID != tc.want || details.Featured != tc.featured ||
				(!tc.featured && len(details.Supersedes) != 0) {
				t.Fatalf("projection=%+v err=%v", details, err)
			}
		})
	}
}
