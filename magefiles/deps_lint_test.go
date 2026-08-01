//go:build mage

package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGolangCILintCacheDir(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve explicit isolation", func(t *testing.T) {
		t.Parallel()

		want := filepath.Join(t.TempDir(), "isolated-lint-cache")
		got, err := golangCILintCacheDir("  "+want+"  ", "ignored-project-root")
		if err != nil {
			t.Fatalf("golangCILintCacheDir() error = %v", err)
		}
		if got != want {
			t.Fatalf("golangCILintCacheDir() = %q, want %q", got, want)
		}
	})

	t.Run("Should isolate the implicit cache by project root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		first, err := golangCILintCacheDir("", root)
		if err != nil {
			t.Fatalf("golangCILintCacheDir() first error = %v", err)
		}
		repeated, err := golangCILintCacheDir("", root)
		if err != nil {
			t.Fatalf("golangCILintCacheDir() repeated error = %v", err)
		}
		other, err := golangCILintCacheDir("", t.TempDir())
		if err != nil {
			t.Fatalf("golangCILintCacheDir() other error = %v", err)
		}
		if first != repeated {
			t.Fatalf("golangCILintCacheDir() = %q, want stable %q", repeated, first)
		}
		if first == other {
			t.Fatalf("golangCILintCacheDir() reused %q across project roots", first)
		}
	})
}

func TestGolangciLintScopes(t *testing.T) {
	t.Parallel()

	t.Run("Should lint the whole module when no scopes are set", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			raw  string
		}{
			{name: "Should accept an empty value", raw: ""},
			{name: "Should accept spaces", raw: "   "},
			{name: "Should accept mixed whitespace", raw: "\t\n"},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				if got := golangciLintScopes(testCase.raw); !reflect.DeepEqual(got, []string{"./..."}) {
					t.Fatalf("golangciLintScopes(%q) = %v, want [./...]", testCase.raw, got)
				}
			})
		}
	})

	t.Run("Should narrow the lint run to the explicit scopes", func(t *testing.T) {
		t.Parallel()

		got := golangciLintScopes("  ./internal/api/...   ./cmd/... ")
		want := []string{"./internal/api/...", "./cmd/..."}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("golangciLintScopes() = %v, want %v", got, want)
		}
	})
}

func TestGolangciLintConcurrencyFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		effectiveCPU int
		want         int
	}{
		{name: "Should floor at one worker", effectiveCPU: 0, want: 1},
		{name: "Should keep single-core machines at full width", effectiveCPU: 1, want: 1},
		{name: "Should keep small machines at full width", effectiveCPU: 4, want: 4},
		{name: "Should keep eight-core machines at full width", effectiveCPU: 8, want: 8},
		{name: "Should hold the cap just above the threshold", effectiveCPU: 10, want: 8},
		{name: "Should halve a sixteen-core machine", effectiveCPU: 16, want: 8},
		{name: "Should cap a thirty-two-core machine", effectiveCPU: 32, want: 8},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := golangciLintConcurrencyFor(testCase.effectiveCPU); got != testCase.want {
				t.Fatalf(
					"golangciLintConcurrencyFor(%d) = %d, want %d",
					testCase.effectiveCPU,
					got,
					testCase.want,
				)
			}
		})
	}
}
