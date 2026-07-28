// Suite: outbound message chunking
// Invariant: chunking is lossless, limit-safe, and preserves fenced-code semantics.
// Boundary IN: ChunkMessage and UTF16Len pure behavior.
// Boundary OUT: provider delivery loops and platform formatting.
package bridgesdk

import (
	"fmt"
	"strings"
	"testing"
)

func TestChunkMessage(t *testing.T) {
	t.Parallel()

	t.Run("Should split on natural boundaries within the injected limit", func(t *testing.T) {
		t.Parallel()

		const limit = 28
		text := "alpha beta gamma\ndelta epsilon zeta eta theta iota"
		chunks := ChunkMessage(text, limit, nil)
		if len(chunks) < 2 {
			t.Fatalf("len(ChunkMessage()) = %d, want multiple chunks", len(chunks))
		}

		var reconstructed strings.Builder
		for index, chunk := range chunks {
			if got := runeLen(chunk); got > limit {
				t.Fatalf("chunk %d length = %d, want <= %d: %q", index, got, limit, chunk)
			}
			indicator := fmt.Sprintf("\n\n(%d/%d)", index+1, len(chunks))
			if !strings.HasSuffix(chunk, indicator) {
				t.Fatalf("chunk %d = %q, want suffix %q", index, chunk, indicator)
			}
			body := strings.TrimSuffix(chunk, indicator)
			if index < len(chunks)-1 && body != "" && !isChunkBoundary(body[len(body)-1]) {
				t.Fatalf("chunk %d body does not end on a natural boundary: %q", index, body)
			}
			reconstructed.WriteString(body)
		}
		if got := reconstructed.String(); got != text {
			t.Fatalf("reconstructed text = %q, want %q", got, text)
		}
	})

	t.Run("Should repair a fenced code block across chunk boundaries", func(t *testing.T) {
		t.Parallel()

		text := "Before\n```go\n" + strings.Repeat("fmt.Println(\"hello\")\n", 5) + "```\nAfter"
		chunks := ChunkMessage(text, 64, nil)
		if len(chunks) < 2 {
			t.Fatalf("len(ChunkMessage()) = %d, want multiple chunks", len(chunks))
		}
		if !strings.Contains(chunks[0], "\n```\n\n(1/") {
			t.Fatalf("first chunk does not close the spanning fence: %q", chunks[0])
		}
		if !strings.HasPrefix(chunks[1], "```go\n") {
			t.Fatalf("second chunk = %q, want reopened go fence", chunks[1])
		}
		for index, chunk := range chunks {
			if got := runeLen(chunk); got > 64 {
				t.Fatalf("chunk %d length = %d, want <= 64: %q", index, got, chunk)
			}
		}
	})

	t.Run("Should measure Telegram text in UTF-16 code units", func(t *testing.T) {
		t.Parallel()

		text := strings.Repeat("🙂", 2048) + "x"
		if got := runeLen(text); got >= 4096 {
			t.Fatalf("rune length = %d, fixture must fit the rune-count limit", got)
		}
		if got := UTF16Len(text); got <= 4096 {
			t.Fatalf("UTF16Len() = %d, fixture must exceed Telegram limit", got)
		}

		chunks := ChunkMessage(text, 4096, UTF16Len)
		if len(chunks) < 2 {
			t.Fatalf("len(ChunkMessage()) = %d, want UTF-16 split", len(chunks))
		}
		for index, chunk := range chunks {
			if got := UTF16Len(chunk); got > 4096 {
				t.Fatalf("UTF16Len(chunk %d) = %d, want <= 4096", index, got)
			}
		}
		if got := reconstructPlainChunks(t, chunks); got != text {
			t.Fatalf("reconstructed UTF-16 text length = %d, want %d", UTF16Len(got), UTF16Len(text))
		}
	})

	t.Run("Should return under-limit text unchanged without an indicator", func(t *testing.T) {
		t.Parallel()

		chunks := ChunkMessage("hello", 64, nil)
		if len(chunks) != 1 || chunks[0] != "hello" {
			t.Fatalf("ChunkMessage() = %#v, want [hello]", chunks)
		}
	})

	t.Run("Should return exact-limit text unchanged in each supported unit", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			text  string
			limit int
			lenFn func(string) int
		}{
			{
				name:  "Should keep an exact rune-count message",
				text:  "🙂a",
				limit: 2,
				lenFn: runeLen,
			},
			{
				name:  "Should keep an exact UTF-16 message",
				text:  "🙂",
				limit: 2,
				lenFn: UTF16Len,
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				chunks := ChunkMessage(testCase.text, testCase.limit, testCase.lenFn)
				if len(chunks) != 1 || chunks[0] != testCase.text {
					t.Fatalf("ChunkMessage() = %#v, want unchanged %q", chunks, testCase.text)
				}
			})
		}
	})

	t.Run("Should return text unchanged for a non-positive limit", func(t *testing.T) {
		t.Parallel()

		text := "unchanged"
		cases := []struct {
			name  string
			limit int
		}{
			{name: "Should keep text for a zero limit", limit: 0},
			{name: "Should keep text for a negative limit", limit: -1},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				chunks := ChunkMessage(text, testCase.limit, nil)
				if len(chunks) != 1 || chunks[0] != text {
					t.Fatalf(
						"ChunkMessage(limit=%d) = %#v, want unchanged %q",
						testCase.limit,
						chunks,
						text,
					)
				}
			})
		}
	})

	t.Run("Should preserve leading trailing and boundary whitespace exactly", func(t *testing.T) {
		t.Parallel()

		const limit = 18
		text := "  alpha beta\n\n\tgamma delta  "
		chunks := ChunkMessage(text, limit, nil)
		assertChunkLimits(t, chunks, limit, runeLen)
		if got := reconstructPlainChunks(t, chunks); got != text {
			t.Fatalf("reconstructed text = %q, want %q", got, text)
		}
	})

	t.Run("Should keep mid-line triple backticks inside the spanning code block", func(t *testing.T) {
		t.Parallel()

		const limit = 34
		text := "```go\n" + strings.Repeat("a", 24) + " ```literal " +
			strings.Repeat("b", 42) + "\n```"
		chunks := ChunkMessage(text, limit, nil)
		if len(chunks) < 3 {
			t.Fatalf("len(ChunkMessage()) = %d, want at least 3 chunks", len(chunks))
		}
		assertChunkLimits(t, chunks, limit, runeLen)
		assertSpanningFenceChunks(t, chunks, "go", false)
		if got := reconstructSpanningFence(t, chunks, "go", false); got != text {
			t.Fatalf("reconstructed fenced text = %q, want %q", got, text)
		}
	})

	t.Run("Should close an unclosed final fence after the last chunk", func(t *testing.T) {
		t.Parallel()

		const limit = 32
		text := "```go\n" + strings.Repeat("fmt.Println(1)\n", 5)
		chunks := ChunkMessage(text, limit, nil)
		if len(chunks) < 2 {
			t.Fatalf("len(ChunkMessage()) = %d, want multiple chunks", len(chunks))
		}
		assertChunkLimits(t, chunks, limit, runeLen)
		assertSpanningFenceChunks(t, chunks, "go", true)
		if got := reconstructSpanningFence(t, chunks, "go", true); got != text {
			t.Fatalf("reconstructed fenced text = %q, want %q", got, text)
		}
	})

	t.Run("Should widen real indicators across ten and one hundred chunks", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			textLen int
			minimum int
		}{
			{name: "Should use two-digit totals", textLen: 120, minimum: 10},
			{name: "Should use three-digit totals", textLen: 700, minimum: 100},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				const limit = 20
				text := strings.Repeat("x", testCase.textLen)
				chunks := ChunkMessage(text, limit, nil)
				if len(chunks) < testCase.minimum {
					t.Fatalf("len(ChunkMessage()) = %d, want >= %d", len(chunks), testCase.minimum)
				}
				assertChunkLimits(t, chunks, limit, runeLen)
				if got := reconstructPlainChunks(t, chunks); got != text {
					t.Fatalf("reconstructed length = %d, want %d", runeLen(got), testCase.textLen)
				}
			})
		}
	})

	t.Run("Should fall back to lossless undecorated chunks when indicators cannot fit", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			text  string
			limit int
			lenFn func(string) int
		}{
			{
				name:  "Should split one-rune ASCII chunks",
				text:  "abcd",
				limit: 1,
				lenFn: runeLen,
			},
			{
				name:  "Should split one-emoji UTF-16 chunks",
				text:  "🙂🙂🙂",
				limit: 2,
				lenFn: UTF16Len,
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				chunks := ChunkMessage(testCase.text, testCase.limit, testCase.lenFn)
				if len(chunks) < 2 {
					t.Fatalf("len(ChunkMessage()) = %d, want multiple chunks", len(chunks))
				}
				assertChunkLimits(t, chunks, testCase.limit, testCase.lenFn)
				if got := strings.Join(chunks, ""); got != testCase.text {
					t.Fatalf("joined chunks = %q, want %q", got, testCase.text)
				}
				for index, chunk := range chunks {
					if strings.Contains(chunk, fmt.Sprintf("(%d/", index+1)) {
						t.Fatalf("chunk %d = %q, want undecorated fallback", index, chunk)
					}
				}
			})
		}
	})
}

func TestDeliveryChunkCursor(t *testing.T) {
	t.Parallel()

	t.Run("Should retain the acknowledged prefix only for the same delivery attempt", func(t *testing.T) {
		t.Parallel()

		cursor := BeginDeliveryChunks(
			DeliveryChunkCursor{},
			7,
			DeliveryChunkModeUpdate,
			3,
			"current content",
			"remote-anchor",
			"remote-replaced",
		)
		cursor = cursor.Advance("remote-anchor").Advance("remote-continuation")
		resumed := BeginDeliveryChunks(
			cursor,
			7,
			DeliveryChunkModeUpdate,
			3,
			"current content",
			"ignored-anchor",
			"ignored-replaced",
		)
		if got, want := resumed.NextChunk(), 2; got != want {
			t.Fatalf("NextChunk() = %d, want %d", got, want)
		}
		if got, want := resumed.LastRemoteMessageID(), "remote-continuation"; got != want {
			t.Fatalf("LastRemoteMessageID() = %q, want %q", got, want)
		}
		if got, want := resumed.ReplaceRemoteMessageID(), "remote-replaced"; got != want {
			t.Fatalf("ReplaceRemoteMessageID() = %q, want %q", got, want)
		}

		fresh := BeginDeliveryChunks(
			resumed,
			8,
			DeliveryChunkModeCreate,
			2,
			"different content",
			"",
			"",
		)
		if got := fresh.NextChunk(); got != 0 {
			t.Fatalf("fresh NextChunk() = %d, want 0", got)
		}
		if got := fresh.LastRemoteMessageID(); got != "" {
			t.Fatalf("fresh LastRemoteMessageID() = %q, want empty", got)
		}
	})
}

func TestUTF16Len(t *testing.T) {
	t.Parallel()

	t.Run("Should count BMP and astral runes as UTF-16 code units", func(t *testing.T) {
		t.Parallel()

		if got, want := UTF16Len("a🙂𝄞"), 5; got != want {
			t.Fatalf("UTF16Len() = %d, want %d", got, want)
		}
	})
}

func runeLen(value string) int {
	return len([]rune(value))
}

func isChunkBoundary(value byte) bool {
	return value == ' ' || value == '\n' || value == '\t'
}

func assertChunkLimits(t *testing.T, chunks []string, limit int, lenFn func(string) int) {
	t.Helper()

	for index, chunk := range chunks {
		if got := lenFn(chunk); got > limit {
			t.Fatalf("chunk %d length = %d, want <= %d: %q", index, got, limit, chunk)
		}
	}
}

func reconstructPlainChunks(t *testing.T, chunks []string) string {
	t.Helper()

	var reconstructed strings.Builder
	for index, chunk := range chunks {
		indicator := fmt.Sprintf("\n\n(%d/%d)", index+1, len(chunks))
		if !strings.HasSuffix(chunk, indicator) {
			t.Fatalf("chunk %d = %q, want suffix %q", index, chunk, indicator)
		}
		reconstructed.WriteString(strings.TrimSuffix(chunk, indicator))
	}
	return reconstructed.String()
}

func assertSpanningFenceChunks(t *testing.T, chunks []string, language string, finalSyntheticClose bool) {
	t.Helper()

	prefix := "```" + language + "\n"
	for index, chunk := range chunks {
		body := stripChunkIndicator(t, chunk, index, len(chunks))
		if index > 0 && !strings.HasPrefix(body, prefix) {
			t.Fatalf("chunk %d = %q, want reopened %q fence", index, body, language)
		}
		if index < len(chunks)-1 && !strings.HasSuffix(body, chunkFenceClose) {
			t.Fatalf("chunk %d = %q, want synthetic closing fence", index, body)
		}
	}

	last := stripChunkIndicator(t, chunks[len(chunks)-1], len(chunks)-1, len(chunks))
	if finalSyntheticClose && !strings.HasSuffix(last, chunkFenceClose) {
		t.Fatalf("last chunk = %q, want synthetic closing fence", last)
	}
}

func reconstructSpanningFence(
	t *testing.T,
	chunks []string,
	language string,
	finalSyntheticClose bool,
) string {
	t.Helper()

	prefix := "```" + language + "\n"
	var reconstructed strings.Builder
	for index, chunk := range chunks {
		body := stripChunkIndicator(t, chunk, index, len(chunks))
		if index > 0 {
			body = strings.TrimPrefix(body, prefix)
		}
		if index < len(chunks)-1 || finalSyntheticClose {
			body = strings.TrimSuffix(body, chunkFenceClose)
		}
		reconstructed.WriteString(body)
	}
	return reconstructed.String()
}

func stripChunkIndicator(t *testing.T, chunk string, index int, total int) string {
	t.Helper()

	indicator := fmt.Sprintf("\n\n(%d/%d)", index+1, total)
	if !strings.HasSuffix(chunk, indicator) {
		t.Fatalf("chunk %d = %q, want suffix %q", index, chunk, indicator)
	}
	return strings.TrimSuffix(chunk, indicator)
}
