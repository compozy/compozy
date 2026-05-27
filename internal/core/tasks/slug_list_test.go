package tasks

import (
	"slices"
	"strings"
	"testing"
)

func TestParseCommaSeparatedSlugs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		input            string
		want             []string
		wantErrSubstring string
	}{
		{
			name:  "Should preserve slug order",
			input: "alpha,beta,gamma",
			want:  []string{"alpha", "beta", "gamma"},
		},
		{
			name:  "Should trim slug entries",
			input: "alpha, beta ,gamma",
			want:  []string{"alpha", "beta", "gamma"},
		},
		{
			name:             "Should reject empty entries",
			input:            "alpha,,beta",
			wantErrSubstring: "position 2 cannot be empty",
		},
		{
			name:             "Should reject duplicate slugs",
			input:            "alpha, beta ,alpha",
			wantErrSubstring: `duplicate task slug "alpha"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseCommaSeparatedSlugs(tc.input)
			if tc.wantErrSubstring != "" {
				if err == nil {
					t.Fatal("expected parse error")
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstring) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse slugs: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("unexpected slugs\nwant: %#v\ngot:  %#v", tc.want, got)
			}
		})
	}
}
