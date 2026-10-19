package mention

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const sessionPrefix = "session:"

// Parse returns the candidate mention tokens of text in text order, at most
// MaxCandidateTokens of them.
//
// Grammar (Business Rules 1–3 and 7):
//   - A token starts with `@` at offset 0 or right after a Unicode whitespace
//     rune, outside inline code spans and fenced code blocks. Emails such as
//     `me@example.com` and `(@x)` therefore never start a token.
//   - Quoted form `@"path with spaces"` ends at the closing `"` on the same
//     line; the text after the closing quote is plain text. An unterminated
//     quote or an empty quoted path (`@""`) is not a token. The quoted form is
//     always a path token, even when it reads `@"session:x"`.
//   - Unquoted form ends at the next whitespace (or the start of a code span)
//     and has trailing `.` `,` `;` `:` `!` `?` `)` `]` `}` `'` `"` trimmed.
//     A bare `@`, `@` followed by whitespace, or a body made only of trimmed
//     punctuation is not a token. A body starting with another `@` (`@@x`) is
//     not a token.
//   - `@session:<id>` fills SessionID; `@session:` with an empty id (after
//     trimming) is not a token. Every other token fills Path.
//
// Start and End are UTF-16 code-unit offsets into text, matching the offset
// model the web composer and internal/command use.
func Parse(text string) []Token {
	tokens, _ := ParseCounted(text)
	return tokens
}

// ParseCounted is Parse plus the number of candidate tokens past MaxCandidateTokens that were left
// as plain text without being returned.
func ParseCounted(text string) ([]Token, int) {
	tokens := parseAll(text)
	if len(tokens) <= MaxCandidateTokens {
		return tokens, 0
	}
	return tokens[:MaxCandidateTokens:MaxCandidateTokens], len(tokens) - MaxCandidateTokens
}

func parseAll(text string) []Token {
	if !strings.Contains(text, "@") {
		return nil
	}
	code := codeRegions(text)
	var tokens []Token
	cursor := utf16Cursor{text: text}
	regionIndex := 0
	for offset := 0; offset < len(text); {
		for regionIndex < len(code) && code[regionIndex].end <= offset {
			regionIndex++
		}
		if regionIndex < len(code) && code[regionIndex].start <= offset {
			offset = code[regionIndex].end
			continue
		}
		next := strings.IndexByte(text[offset:], '@')
		if next < 0 {
			break
		}
		at := offset + next
		if regionIndex < len(code) && code[regionIndex].start <= at {
			offset = code[regionIndex].start
			continue
		}
		limit := len(text)
		if regionIndex < len(code) {
			limit = code[regionIndex].start
		}
		if !hasTokenStartBoundary(text, at) {
			offset = at + 1
			continue
		}
		token, end, ok := scanToken(text[:limit], at)
		if ok {
			token.Start = cursor.offset(at)
			token.End = cursor.offset(at + len(token.Raw))
			tokens = append(tokens, token)
		}
		offset = max(end, at+1)
	}
	return tokens
}

// scanToken reads the token whose `@` sits at byte offset at. It returns the
// byte offset where scanning resumes and whether a token was produced.
func scanToken(text string, at int) (Token, int, bool) {
	bodyStart := at + 1
	if bodyStart < len(text) && text[bodyStart] == '"' {
		return scanQuotedToken(text, at)
	}
	end := bodyStart
	for end < len(text) {
		character, size := utf8.DecodeRuneInString(text[end:])
		if unicode.IsSpace(character) {
			break
		}
		end += size
	}
	rawEnd := end
	for end > bodyStart {
		character, size := utf8.DecodeLastRuneInString(text[bodyStart:end])
		if !isTrailingPunctuation(character) {
			break
		}
		end -= size
	}
	body := text[bodyStart:end]
	untrimmed := text[bodyStart:rawEnd]
	if body == "" || body[0] == '@' {
		return Token{}, rawEnd, false
	}
	if strings.HasPrefix(untrimmed, sessionPrefix) {
		if len(body) <= len(sessionPrefix) {
			return Token{}, rawEnd, false
		}
		return Token{Raw: text[at:end], SessionID: body[len(sessionPrefix):]}, rawEnd, true
	}
	return Token{Raw: text[at:end], Path: body}, rawEnd, true
}

func scanQuotedToken(text string, at int) (Token, int, bool) {
	pathStart := at + 2
	for end := pathStart; end < len(text); end++ {
		switch text[end] {
		case '"':
			if end == pathStart {
				return Token{}, end + 1, false
			}
			return Token{Raw: text[at : end+1], Path: text[pathStart:end]}, end + 1, true
		case '\n', '\r':
			return Token{}, end, false
		}
	}
	return Token{}, len(text), false
}

func hasTokenStartBoundary(text string, offset int) bool {
	if offset == 0 {
		return true
	}
	previous, _ := utf8.DecodeLastRuneInString(text[:offset])
	return unicode.IsSpace(previous)
}

func isTrailingPunctuation(character rune) bool {
	switch character {
	case '.', ',', ';', ':', '!', '?', ')', ']', '}', '\'', '"':
		return true
	default:
		return false
	}
}

// utf16Cursor converts increasing byte offsets to UTF-16 offsets in linear
// total time.
type utf16Cursor struct {
	text   string
	byteAt int
	unitAt int
}

func (c *utf16Cursor) offset(byteOffset int) int {
	for _, character := range c.text[c.byteAt:byteOffset] {
		c.unitAt += utf16Len(character)
	}
	c.byteAt = byteOffset
	return c.unitAt
}

func utf16Len(character rune) int {
	if character >= 0x10000 && character <= unicode.MaxRune {
		return 2
	}
	return 1
}
