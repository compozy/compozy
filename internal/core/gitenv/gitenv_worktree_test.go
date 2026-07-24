package gitenv

import (
	"reflect"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			// UT-025 (happy): primary block + one sibling block, in listed order.
			name:   "Should parse a primary and one sibling block in listed order",
			output: "worktree /a\nHEAD abc123\nbranch refs/heads/main\n\nworktree /a-wt\nHEAD def456\ndetached",
			want:   []string{"/a", "/a-wt"},
		},
		{
			// UT-026 (boundary): a primary-only block yields a one-element slice.
			name:   "Should return a one-element slice for a primary-only block",
			output: "worktree /a\nHEAD abc123\nbranch refs/heads/main",
			want:   []string{"/a"},
		},
		{
			// UT-027 (state): a prunable attribute line does not suppress the path.
			name:   "Should still return the path for a block carrying a prunable attribute",
			output: "worktree /a\nHEAD abc123\n\nworktree /a-wt\nHEAD def456\nprunable gitdir file points to non-existent location",
			want:   []string{"/a", "/a-wt"},
		},
		{
			// UT-028 (boundary): only the leading prefix is stripped; interior spaces survive.
			name:   "Should preserve a worktree path that contains a space",
			output: "worktree /a b/wt\nHEAD abc123",
			want:   []string{"/a b/wt"},
		},
		{
			// UT-029 (error): empty input -> empty, non-nil slice.
			name:   "Should return an empty slice for empty input",
			output: "",
			want:   []string{},
		},
		{
			// UT-029 (error): malformed input with no worktree lines -> empty, non-nil slice.
			name:   "Should return an empty slice for malformed output with no worktree lines",
			output: "HEAD abc123\nbranch refs/heads/main\nnot a worktree line\nbare",
			want:   []string{},
		},
		{
			// UT-030 (state): interleaved attribute lines are ignored; only worktree paths returned.
			name:   "Should ignore interleaved HEAD, branch, detached, and bare attribute lines",
			output: "worktree /a\nHEAD abc123\nbranch refs/heads/main\n\nworktree /a-wt\nHEAD def456\ndetached\n\nworktree /a-bare\nbare",
			want:   []string{"/a", "/a-wt", "/a-bare"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseWorktreeList(tt.output)
			if got == nil {
				t.Fatalf("ParseWorktreeList(%q) = nil, want a non-nil slice", tt.output)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseWorktreeList(%q) = %#v, want %#v", tt.output, got, tt.want)
			}
		})
	}
}
