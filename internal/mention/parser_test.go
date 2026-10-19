package mention_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/mention"
)

// Suite: prompt mention grammar
// Invariant: authored text yields the same syntactic tokens and UTF-16 offsets on every surface,
// and a hook rewrite can only narrow admitted mentions.
// Boundary IN: pure token grammar, code-region exclusion, offset model, candidate cap, reconcile.
// Boundary OUT: filesystem/session resolution, persistence, ACP delivery, and UI rendering.

func path(raw, target string, start, end int) mention.Token {
	return mention.Token{Raw: raw, Path: target, Start: start, End: end}
}

func session(raw, id string, start, end int) mention.Token {
	return mention.Token{Raw: raw, SessionID: id, Start: start, End: end}
}

func TestParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		want []mention.Token
	}{
		{
			name: "Should parse a path token with UTF-16 offsets",
			text: "Review @internal/acp/client_prompt.go now",
			want: []mention.Token{path("@internal/acp/client_prompt.go", "internal/acp/client_prompt.go", 7, 37)},
		},
		{name: "Should ignore an email address", text: "mail me@example.com", want: nil},
		{name: "Should ignore a mid-word at sign", text: "a@b", want: nil},
		{
			name: "Should trim a trailing period",
			text: "see @README.md.",
			want: []mention.Token{path("@README.md", "README.md", 4, 14)},
		},
		{
			name: "Should trim every trailing punctuation rune",
			text: "@a.go,;:!?)]}'\" next",
			want: []mention.Token{path("@a.go", "a.go", 0, 5)},
		},
		{name: "Should require whitespace before the at sign", text: "(@x)", want: nil},
		{
			name: "Should keep the trailing slash of a folder token",
			text: "list @internal/acp/, please",
			want: []mention.Token{path("@internal/acp/", "internal/acp/", 5, 19)},
		},
		{
			name: "Should parse a quoted path with spaces",
			text: `open @"docs/My Notes.md" please`,
			want: []mention.Token{path(`@"docs/My Notes.md"`, "docs/My Notes.md", 5, 24)},
		},
		{
			name: "Should end a quoted token at the closing quote",
			text: `@"a b".`,
			want: []mention.Token{path(`@"a b"`, "a b", 0, 6)},
		},
		{name: "Should reject an unterminated quote", text: `open @"docs/x`, want: nil},
		{name: "Should reject a quote closed on a later line", text: "@\"docs/x\ny\" @", want: nil},
		{name: "Should reject an empty quoted path", text: `@"" x`, want: nil},
		{
			name: "Should treat a quoted session-like text as a path",
			text: `@"session:x"`,
			want: []mention.Token{path(`@"session:x"`, "session:x", 0, 12)},
		},
		{
			name: "Should parse a session token",
			text: "@session:sess-9f8e7d",
			want: []mention.Token{session("@session:sess-9f8e7d", "sess-9f8e7d", 0, 20)},
		},
		{
			name: "Should trim punctuation after a session id",
			text: "ask @session:sess-1.",
			want: []mention.Token{session("@session:sess-1", "sess-1", 4, 19)},
		},
		{name: "Should reject a session token with an empty id", text: "@session: x @session:.", want: nil},
		{name: "Should reject a bare at sign", text: "@", want: nil},
		{name: "Should reject an at sign followed by whitespace", text: "hi @ there", want: nil},
		{name: "Should reject a punctuation-only body", text: "wow @!? ok", want: nil},
		{name: "Should reject a double at sign", text: "@@x", want: nil},
		{
			name: "Should start after any Unicode whitespace",
			text: "a @x\t@y\n@z",
			want: []mention.Token{path("@x", "x", 2, 4), path("@y", "y", 5, 7), path("@z", "z", 8, 10)},
		},
		{
			name: "Should count an emoji as two UTF-16 units",
			text: "🔥 @a.go",
			want: []mention.Token{path("@a.go", "a.go", 3, 8)},
		},
		{
			name: "Should count CJK runes as one UTF-16 unit and keep them in paths",
			text: "看看 @文档/说明.md 和 @🔥.go",
			want: []mention.Token{
				path("@文档/说明.md", "文档/说明.md", 3, 12),
				path("@🔥.go", "🔥.go", 15, 21),
			},
		},
		{name: "Should ignore a token inside inline code", text: "use `@Component` here", want: nil},
		{name: "Should ignore a token inside a double-backtick span", text: "x ``a ` @b`` y", want: nil},
		{
			name: "Should treat an unmatched backtick as text",
			text: "a ` @b",
			want: []mention.Token{path("@b", "b", 4, 6)},
		},
		{
			name: "Should ignore tokens inside a fenced block and parse the next line",
			text: "```go\n@inside.go\n```\n@outside.go",
			want: []mention.Token{path("@outside.go", "outside.go", 21, 32)},
		},
		{
			name: "Should honor tilde fences and longer closing runs",
			text: "~~~~\n@a\n~~~\n@b\n~~~~~\n@c",
			want: []mention.Token{path("@c", "c", 21, 23)},
		},
		{name: "Should keep an unterminated fence open to the end", text: "```\n@a\n@b", want: nil},
		{
			name: "Should end an unquoted token where inline code starts and resume after it",
			text: "@a`x` @b",
			want: []mention.Token{path("@a", "a", 0, 2), path("@b", "b", 6, 8)},
		},
		{
			name: "Should keep repeated tokens as separate occurrences",
			text: "@a.go and @a.go",
			want: []mention.Token{path("@a.go", "a.go", 0, 5), path("@a.go", "a.go", 10, 15)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mention.Parse(tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tc.text, got, tc.want)
			}
		})
	}

	t.Run("Should return at most MaxCandidateTokens tokens in text order", func(t *testing.T) {
		t.Parallel()
		words := make([]string, 250)
		for index := range words {
			words[index] = fmt.Sprintf("@x%d", index)
		}
		got := mention.Parse(strings.Join(words, " "))
		if len(got) != mention.MaxCandidateTokens {
			t.Fatalf("Parse() returned %d tokens, want %d", len(got), mention.MaxCandidateTokens)
		}
		if got[0].Path != "x0" || got[len(got)-1].Path != "x199" {
			t.Fatalf("Parse() first/last = %q/%q, want x0/x199", got[0].Path, got[len(got)-1].Path)
		}
	})

	t.Run("Should count the candidate tokens past MaxCandidateTokens", func(t *testing.T) {
		t.Parallel()
		words := make([]string, 250)
		for index := range words {
			words[index] = fmt.Sprintf("@x%d", index)
		}
		got, skipped := mention.ParseCounted(strings.Join(words, " ") + " `@code` me@example.com")
		if len(got) != mention.MaxCandidateTokens || skipped != 50 {
			t.Fatalf("ParseCounted() = %d tokens, %d skipped; want %d and 50", len(got), skipped,
				mention.MaxCandidateTokens)
		}
		if tokens, skipped := mention.ParseCounted("read @a.go"); len(tokens) != 1 || skipped != 0 {
			t.Fatalf("ParseCounted(one token) = %#v, %d skipped", tokens, skipped)
		}
	})

	t.Run("Should count 100k candidates while returning only the capped tokens", func(t *testing.T) {
		t.Parallel()
		text := strings.Repeat("@x ", 100_000)
		got, skipped := mention.ParseCounted(text)
		if len(got) != mention.MaxCandidateTokens || skipped != 100_000-mention.MaxCandidateTokens {
			t.Fatalf("ParseCounted() = %d tokens, %d skipped", len(got), skipped)
		}
		last := got[len(got)-1]
		wantStart := 3 * (mention.MaxCandidateTokens - 1)
		if last.Start != wantStart || last.End != wantStart+2 {
			t.Fatalf("last token offsets = [%d,%d), want [%d,%d)", last.Start, last.End, wantStart, wantStart+2)
		}
	})
}

func BenchmarkParseCountedManyCandidates(b *testing.B) {
	text := strings.Repeat("@x ", 100_000)
	b.ReportAllocs()
	for b.Loop() {
		mention.ParseCounted(text)
	}
}

func TestReconcile(t *testing.T) {
	t.Parallel()

	file := func(token, target string, start, end int) mention.Mention {
		return mention.Mention{
			Kind: mention.KindFile, Token: token, Target: target, Label: target, Start: start, End: end,
		}
	}

	sessionMention := func(start, end int) mention.Mention {
		return mention.Mention{
			Kind: mention.KindSession, Token: "@session:sess-1", Target: "sess-1", Label: "Fix", Start: start, End: end,
		}
	}

	cases := []struct {
		name     string
		text     string
		admitted []mention.Mention
		want     []mention.Mention
	}{
		{
			name: "Should keep surviving mentions at their authored offsets and drop removed ones",
			text: "Prefix: check @b.go",
			admitted: []mention.Mention{
				file("@a.go", "a.go", 0, 5),
				file("@b.go", "b.go", 10, 15),
			},
			want: []mention.Mention{file("@b.go", "b.go", 10, 15)},
		},
		{
			name:     "Should never add a token that was not admitted",
			text:     "@a.go @new.go @session:sess-2",
			admitted: []mention.Mention{file("@a.go", "a.go", 0, 5)},
			want:     []mention.Mention{file("@a.go", "a.go", 0, 5)},
		},
		{
			name: "Should keep one admitted mention per surviving occurrence of a repeated token",
			text: "@a.go x @a.go",
			admitted: []mention.Mention{
				file("@a.go", "a.go", 0, 5),
				file("@a.go", "a.go", 6, 11),
				file("@a.go", "a.go", 12, 17),
			},
			want: []mention.Mention{file("@a.go", "a.go", 0, 5), file("@a.go", "a.go", 6, 11)},
		},
		{
			name: "Should drop a token a hook moved into a code span",
			text: "see `@a.go` and @b.go",
			admitted: []mention.Mention{
				file("@a.go", "a.go", 4, 9),
				file("@b.go", "b.go", 14, 19),
			},
			want: []mention.Mention{file("@b.go", "b.go", 14, 19)},
		},
		{
			name: "Should keep authored order and offsets when a hook reorders tokens",
			text: "@b.go then @session:sess-1",
			admitted: []mention.Mention{
				sessionMention(0, 15),
				file("@b.go", "b.go", 16, 21),
			},
			want: []mention.Mention{
				sessionMention(0, 15),
				file("@b.go", "b.go", 16, 21),
			},
		},
		{
			name:     "Should require the exact token and not a prefix",
			text:     "@a.gox",
			admitted: []mention.Mention{file("@a.go", "a.go", 0, 5)},
			want:     []mention.Mention{},
		},
		{name: "Should return nothing when nothing was admitted", text: "@a.go", admitted: nil, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mention.Reconcile(tc.text, tc.admitted)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Reconcile(%q) = %#v, want %#v", tc.text, got, tc.want)
			}
		})
	}
}
