package cli

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCandidateRenderAndPrepare(t *testing.T) {
	t.Parallel()

	t.Run("Should render TAB-separated description and sanitize control characters", func(t *testing.T) {
		t.Parallel()

		withDesc := completionCandidate{value: "sess-1", description: "Fix\tlogin\nbug"}
		if got := withDesc.render(); got != "sess-1\tFix login bug" {
			t.Fatalf("render() = %q, want %q", got, "sess-1\tFix login bug")
		}

		noDesc := completionCandidate{value: "sess-2", description: "  \t\n  "}
		if got := noDesc.render(); got != "sess-2" {
			t.Fatalf("render() without description = %q, want %q", got, "sess-2")
		}

		blankVal := completionCandidate{value: "  ", description: "ignored"}
		if got := blankVal.render(); got != "" {
			t.Fatalf("render() blank value = %q, want empty", got)
		}
	})

	t.Run("Should filter by prefix, dedupe, and stable-sort candidates", func(t *testing.T) {
		t.Parallel()

		input := []completionCandidate{
			{value: "sess-b", description: ""},
			{value: "other-1", description: "ignored"},
			{value: "sess-a", description: "First"},
			{value: "sess-b", description: "Second"},
			{value: "", description: "empty"},
			{value: "sess-a", description: "Duplicate"},
		}

		got := renderCompletionCandidates(input, "sess-")
		want := []string{
			"sess-a\tFirst",
			"sess-b\tSecond",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("renderCompletionCandidates() = %#v, want %#v", got, want)
		}
	})
}

func TestCompletionAdapterDirectives(t *testing.T) {
	t.Parallel()

	t.Run("Should return NoFileComp directive and formatted completions", func(t *testing.T) {
		t.Parallel()

		fn := completionAdapter(cobra.ShellCompDirectiveNoFileComp, func(_ *cobra.Command, _ []string, _ string) []completionCandidate {
			return []completionCandidate{
				{value: "ws_2", description: "Beta"},
				{value: "ws_1", description: "Alpha"},
			}
		})
		got, directive := fn(&cobra.Command{Use: "test"}, nil, "ws_")
		want := []string{"ws_1\tAlpha", "ws_2\tBeta"}
		if !slices.Equal(got, want) {
			t.Fatalf("completions = %#v, want %#v", got, want)
		}
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
		}
	})

	t.Run("Should preserve Default directive when configured with nil fetch", func(t *testing.T) {
		t.Parallel()

		fn := completionAdapter(cobra.ShellCompDirectiveDefault, nil)
		got, directive := fn(&cobra.Command{Use: "test"}, nil, "")
		if len(got) != 0 {
			t.Fatalf("completions = %#v, want empty", got)
		}
		if directive != cobra.ShellCompDirectiveDefault {
			t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveDefault)
		}
	})
}
