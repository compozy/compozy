package listcursor

import (
	"errors"
	"strings"
	"testing"
)

type cursorTestPosition struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

func TestCursorCodec(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip a query-bound typed position", func(t *testing.T) {
		t.Parallel()

		fingerprint, err := Fingerprint(struct {
			Query string `json:"q"`
		}{Query: "needle"})
		if err != nil {
			t.Fatalf("Fingerprint() error = %v", err)
		}
		encoded, err := Encode(1, "jobs", fingerprint, cursorTestPosition{Name: "alpha", ID: "job-1"})
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		decoded, err := Decode[cursorTestPosition](encoded, 1, "jobs", fingerprint, DefaultMaxEncodedSize)
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if got, want := decoded, (cursorTestPosition{Name: "alpha", ID: "job-1"}); got != want {
			t.Fatalf("Decode() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject malformed or mismatched cursors", func(t *testing.T) {
		t.Parallel()

		fingerprint, err := Fingerprint(map[string]string{"q": "needle"})
		if err != nil {
			t.Fatalf("Fingerprint() error = %v", err)
		}
		encoded, err := Encode(1, "jobs", fingerprint, cursorTestPosition{Name: "alpha", ID: "job-1"})
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		cases := []struct {
			name        string
			raw         string
			version     int
			kind        string
			fingerprint string
			maxSize     int
		}{
			{name: "malformed", raw: "not-base64", version: 1, kind: "jobs", fingerprint: fingerprint},
			{name: "version", raw: encoded, version: 2, kind: "jobs", fingerprint: fingerprint},
			{name: "kind", raw: encoded, version: 1, kind: "triggers", fingerprint: fingerprint},
			{name: "fingerprint", raw: encoded, version: 1, kind: "jobs", fingerprint: "different"},
			{
				name:        "size",
				raw:         strings.Repeat("x", 20),
				version:     1,
				kind:        "jobs",
				fingerprint: fingerprint,
				maxSize:     10,
			},
		}
		for _, testCase := range cases {
			t.Run("Should reject "+testCase.name, func(t *testing.T) {
				t.Parallel()

				_, err := Decode[cursorTestPosition](
					testCase.raw,
					testCase.version,
					testCase.kind,
					testCase.fingerprint,
					testCase.maxSize,
				)
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("Decode() error = %v, want ErrInvalid", err)
				}
			})
		}
	})
}
