package worktree

import (
	"errors"
	"testing"
)

// Canonical suite: strict Git porcelain parsing and unknown status semantics.
func TestGitPorcelain(t *testing.T) {
	t.Parallel()

	t.Run("Should parse linked worktrees and identify the main checkout", func(t *testing.T) {
		t.Parallel()
		fixture := []byte("worktree /repo\x00HEAD aaa\x00branch refs/heads/main\x00\x00" +
			"worktree /repo/a\x00HEAD bbb\x00branch refs/heads/feature/a\x00\x00" +
			"worktree /repo/b\x00HEAD ccc\x00detached\x00prunable gitdir file missing\x00")
		got, err := ParseWorktreeList(fixture)
		if err != nil {
			t.Fatalf("ParseWorktreeList() error = %v", err)
		}
		if len(got) != 3 || !got[0].Main || got[1].Branch != "feature/a" || !got[2].Detached || !got[2].Prunable {
			t.Fatalf("ParseWorktreeList() = %#v, want main plus two linked entries", got)
		}
	})

	t.Run("Should skip bare entries and flush the final record", func(t *testing.T) {
		t.Parallel()
		fixture := []byte("worktree /repo\x00HEAD aaa\x00branch refs/heads/main\x00\x00" +
			"worktree /bare\x00bare\x00\x00worktree /repo/a\x00HEAD bbb\x00branch refs/heads/a")
		got, err := ParseWorktreeList(fixture)
		if err != nil || len(got) != 2 || got[1].Path != "/repo/a" {
			t.Fatalf("ParseWorktreeList() = (%#v, %v), want two selectable records", got, err)
		}
	})

	t.Run("Should preserve lock and prune diagnostics", func(t *testing.T) {
		t.Parallel()
		got, err := ParseWorktreeList([]byte(
			"worktree /repo\x00HEAD aaa\x00branch refs/heads/main\x00\x00" +
				"worktree /repo/held\x00HEAD bbb\x00locked operator action\x00prunable stale admin\x00",
		))
		if err != nil || len(got) != 2 || !got[1].Locked || got[1].LockReason != "operator action" ||
			!got[1].Prunable || got[1].PrunableReason != "stale admin" {
			t.Fatalf("ParseWorktreeList(diagnostics) = (%#v, %v), want lock/prune reasons", got, err)
		}
	})

	t.Run("Should preserve upstream ahead and behind counts", func(t *testing.T) {
		t.Parallel()
		fixture := []byte(
			"# branch.oid abc\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -1\n1 M. N... file\x00",
		)
		got, err := ParseStatusV2(fixture)
		if err != nil || got.Branch != "main" || got.Ahead == nil || *got.Ahead != 2 || got.Behind == nil ||
			*got.Behind != 1 {
			t.Fatalf("ParseStatusV2() = (%#v, %v), want main +2 -1", got, err)
		}
	})

	t.Run("Should keep ahead and behind unknown without an upstream", func(t *testing.T) {
		t.Parallel()
		got, err := ParseStatusV2([]byte("# branch.oid abc\n# branch.head main\n"))
		if err != nil || got.HasUpstream || got.Ahead != nil || got.Behind != nil {
			t.Fatalf("ParseStatusV2() = (%#v, %v), want nil remote counts", got, err)
		}
	})

	t.Run("Should count dirty entries for a detached checkout", func(t *testing.T) {
		t.Parallel()
		got, err := ParseStatusV2([]byte("# branch.oid abc\n# branch.head (detached)\n? new.txt\x00"))
		if err != nil || !got.Detached || got.Branch != "" || got.DirtyFiles != 1 {
			t.Fatalf("ParseStatusV2() = (%#v, %v), want detached dirty checkout", got, err)
		}
	})

	t.Run("Should preserve exact untracked names while excluding ignored paths", func(t *testing.T) {
		t.Parallel()
		got, err := ParseUntrackedStatusV2([]byte(
			"? z dir/new file.txt\x00! ignored.env\x00? a\nnewline.txt\x001 M. N... tracked.txt\x00",
		))
		if err != nil || len(got) != 2 || got[0] != "a\nnewline.txt" || got[1] != "z dir/new file.txt" {
			t.Fatalf("ParseUntrackedStatusV2() = (%#v, %v), want sorted exact untracked names", got, err)
		}
	})

	t.Run("Should preserve whitespace-only untracked names", func(t *testing.T) {
		t.Parallel()
		got, err := ParseUntrackedStatusV2([]byte("?  \x00? \t\x00"))
		if err != nil || len(got) != 2 || got[0] != "\t" || got[1] != " " {
			t.Fatalf("ParseUntrackedStatusV2(whitespace) = (%#v, %v)", got, err)
		}
	})

	t.Run("Should consume rename pairs and preserve newline filenames", func(t *testing.T) {
		t.Parallel()
		fixture := []byte("# branch.oid abc\n# branch.head main\n" +
			"2 R. N... 100644 100644 100644 abc def R100 new\nname.txt\x00old\nname.txt\x00" +
			"? another\nname.txt\x00")
		got, err := ParseStatusV2(fixture)
		if err != nil || got.DirtyFiles != 2 {
			t.Fatalf("ParseStatusV2(rename) = (%#v, %v), want two dirty paths", got, err)
		}
	})

	t.Run("Should merge numstat values and deduplicate files", func(t *testing.T) {
		t.Parallel()
		staged, err := ParseNumstat([]byte("5\t1\ta.txt\x001\t0\tb.txt\x00"))
		if err != nil {
			t.Fatalf("ParseNumstat(staged) error = %v", err)
		}
		unstaged, err := ParseNumstat([]byte("4\t1\ta.txt\x00-\t-\timage.png\x00"))
		if err != nil {
			t.Fatalf("ParseNumstat(unstaged) error = %v", err)
		}
		got := MergeNumstat(staged, unstaged)
		if len(got.Files) != 3 || got.Insertions != 10 || got.Deletions != 2 {
			t.Fatalf("MergeNumstat() = %#v, want 3 files +10 -2", got)
		}
	})

	t.Run("Should return a typed error for garbled input", func(t *testing.T) {
		t.Parallel()
		_, err := ParseStatusV2([]byte("garbled\x00"))
		if _, ok := errors.AsType[*ParseError](err); !ok {
			t.Fatalf("ParseStatusV2() error = %v, want ParseError", err)
		}
	})

	t.Run("Should reject malformed porcelain records at their owning parser", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			format string
			parse  func() error
		}{
			{name: "orphan worktree field", format: "worktree-list", parse: func() error {
				_, err := ParseWorktreeList([]byte("HEAD abc\x00"))
				return err
			}},
			{name: "unknown worktree field", format: "worktree-list", parse: func() error {
				_, err := ParseWorktreeList([]byte("worktree /repo\x00unknown\x00"))
				return err
			}},
			{name: "empty worktree path", format: "worktree-list", parse: func() error {
				_, err := ParseWorktreeList([]byte("worktree \x00"))
				return err
			}},
			{name: "missing status identity", format: "status-v2", parse: func() error {
				_, err := ParseStatusV2([]byte("# branch.oid abc\x00"))
				return err
			}},
			{name: "invalid ahead behind", format: "status-v2", parse: func() error {
				_, err := ParseStatusV2([]byte("# branch.oid abc\x00# branch.head main\x00# branch.ab +x -1\x00"))
				return err
			}},
			{name: "upstream without counts", format: "status-v2", parse: func() error {
				_, err := ParseStatusV2(
					[]byte("# branch.oid abc\x00# branch.head main\x00# branch.upstream origin/main\x00"),
				)
				return err
			}},
			{name: "rename without old path", format: "status-v2", parse: func() error {
				_, err := ParseStatusV2([]byte("# branch.oid abc\x00# branch.head main\x002 R. incomplete\x00"))
				return err
			}},
			{name: "short numstat", format: "numstat", parse: func() error {
				_, err := ParseNumstat([]byte("1\tfile\x00"))
				return err
			}},
			{name: "negative numstat", format: "numstat", parse: func() error {
				_, err := ParseNumstat([]byte("-1\t0\tfile\x00"))
				return err
			}},
			{name: "incomplete renamed numstat", format: "numstat", parse: func() error {
				_, err := ParseNumstat([]byte("1\t0\t\x00old\x00"))
				return err
			}},
		}
		for _, testCase := range cases {
			t.Run("Should reject "+testCase.name, func(t *testing.T) {
				t.Parallel()
				var parseErr *ParseError
				if err := testCase.parse(); !errors.As(err, &parseErr) || parseErr.Format != testCase.format {
					t.Fatalf("parse error = %v, want %s ParseError", err, testCase.format)
				}
			})
		}
	})
}
