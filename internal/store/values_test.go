package store

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestFormatTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			name: "zero time returns empty string",
			in:   time.Time{},
			want: "",
		},
		{
			name: "non-zero UTC time formats canonically",
			in:   time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC),
			want: "2024-01-15T10:30:00.123456789Z",
		},
		{
			name: "non-UTC time is converted to UTC",
			in:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*3600)),
			want: "2024-01-15T10:00:00.000000000Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatTimestamp(tt.in)
			if got != tt.want {
				t.Errorf("FormatTimestamp() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{
			name: "valid canonical timestamp",
			in:   "2024-01-15T10:30:00.123456789Z",
			want: time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC),
		},
		{
			name: "trims surrounding whitespace",
			in:   "  2024-01-15T10:30:00.000000000Z  ",
			want: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:    "invalid format returns error",
			in:      "not-a-timestamp",
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			in:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTimestamp(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTimestamp(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseTimestamp(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNullableString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want any
	}{
		{name: "empty string returns nil", in: "", want: nil},
		{name: "whitespace-only string returns nil", in: "   \t\n", want: nil},
		{name: "non-empty string returns trimmed value", in: "  hello  ", want: "hello"},
		{name: "value without spaces returned unchanged", in: "value", want: "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NullableString(tt.in)
			if got != tt.want {
				t.Errorf("NullableString(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNullString(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name string
		in   sql.NullString
		want *string
	}{
		{
			name: "invalid NullString returns nil",
			in:   sql.NullString{Valid: false},
			want: nil,
		},
		{
			name: "valid but empty NullString returns nil",
			in:   sql.NullString{Valid: true, String: ""},
			want: nil,
		},
		{
			name: "valid whitespace-only NullString returns nil",
			in:   sql.NullString{Valid: true, String: "   "},
			want: nil,
		},
		{
			name: "valid non-empty NullString returns trimmed pointer",
			in:   sql.NullString{Valid: true, String: "  hello  "},
			want: strPtr("hello"),
		},
		{
			name: "valid string without padding returned as-is",
			in:   sql.NullString{Valid: true, String: "value"},
			want: strPtr("value"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NullString(tt.in)
			if tt.want == nil {
				if got != nil {
					t.Errorf("NullString() = %q, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("NullString() = nil, want %q", *tt.want)
			}
			if *got != *tt.want {
				t.Errorf("NullString() = %q, want %q", *got, *tt.want)
			}
		})
	}
}

func TestNewID(t *testing.T) {
	t.Parallel()

	t.Run("without prefix returns 16-char hex string", func(t *testing.T) {
		t.Parallel()
		id := NewID("")
		if id == "" {
			t.Fatal("NewID(\"\") returned empty string")
		}
		if len(id) != 16 {
			t.Errorf("NewID(\"\") length = %d, want 16", len(id))
		}
	})

	t.Run("with prefix returns prefix-hex format", func(t *testing.T) {
		t.Parallel()
		id := NewID("run")
		if !strings.HasPrefix(id, "run-") {
			t.Errorf("NewID(%q) = %q, want prefix %q", "run", id, "run-")
		}
	})

	t.Run("whitespace-only prefix treated as empty prefix", func(t *testing.T) {
		t.Parallel()
		id := NewID("   ")
		if strings.Contains(id, " ") {
			t.Errorf("NewID(whitespace) = %q, must not contain spaces", id)
		}
		if len(id) != 16 {
			t.Errorf("NewID(whitespace) length = %d, want 16", len(id))
		}
	})

	t.Run("successive calls return unique IDs", func(t *testing.T) {
		t.Parallel()
		id1 := NewID("test")
		id2 := NewID("test")
		if id1 == id2 {
			t.Errorf("NewID() returned duplicate IDs: %q", id1)
		}
	})
}
