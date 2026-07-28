package extensionpkg_test

import (
	"strings"
	"testing"
)

func TestNonEmptyLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "ShouldTrimAndDropBlankLines",
			input: "\n first \n\nsecond\n  \n third  \n",
			want:  []string{"first", "second", "third"},
		},
		{
			name:  "ShouldReturnEmptySliceWhenEveryLineIsBlank",
			input: "\n \n\t\n",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nonEmptyLines(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len(nonEmptyLines()) = %d, want %d", len(got), len(tt.want))
			}
			for index := range tt.want {
				if got[index] != tt.want[index] {
					t.Fatalf("nonEmptyLines()[%d] = %q, want %q", index, got[index], tt.want[index])
				}
			}
		})
	}
}

func TestContainsFragmentsInOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		fragments []string
		want      bool
	}{
		{
			name:      "ShouldMatchOrderedFragments",
			text:      "alpha beta gamma",
			fragments: []string{"alpha", "beta", "gamma"},
			want:      true,
		},
		{
			name:      "ShouldRejectOutOfOrderFragments",
			text:      "alpha gamma beta",
			fragments: []string{"alpha", "beta", "gamma"},
			want:      false,
		},
		{
			name:      "ShouldIgnoreEmptyFragments",
			text:      "alpha beta",
			fragments: []string{"alpha", "", "beta"},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsFragmentsInOrder(tt.text, tt.fragments...); got != tt.want {
				t.Fatalf("containsFragmentsInOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeJSONLines(t *testing.T) {
	t.Parallel()

	type sample struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name      string
		payload   []byte
		wantNames []string
		wantError string
	}{
		{
			name:      "ShouldDecodeMultipleJSONLines",
			payload:   []byte("{\"name\":\"alpha\"}\n\n{\"name\":\"beta\"}\n"),
			wantNames: []string{"alpha", "beta"},
		},
		{
			name:      "ShouldDecodeEmptyPayloadIntoEmptySlice",
			payload:   []byte("\n\t\n"),
			wantNames: []string{},
		},
		{
			name:      "ShouldReportInvalidJSONLineContent",
			payload:   []byte("{not-json}\n"),
			wantError: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := decodeJSONLines[sample](tt.payload)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("decodeJSONLines() error = nil, want containing %q", tt.wantError)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("decodeJSONLines() error = %q, want containing %q", err.Error(), tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeJSONLines() error = %v", err)
			}
			if len(items) != len(tt.wantNames) {
				t.Fatalf("len(decodeJSONLines()) = %d, want %d", len(items), len(tt.wantNames))
			}
			for index, wantName := range tt.wantNames {
				if items[index].Name != wantName {
					t.Fatalf("decodeJSONLines()[%d].Name = %q, want %q", index, items[index].Name, wantName)
				}
			}
		})
	}
}
