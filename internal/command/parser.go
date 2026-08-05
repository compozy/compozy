package command

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	// ErrUnavailable reports a recognized slash command that is unavailable for the session.
	ErrUnavailable = errors.New("command: unavailable")
)

// ParseSkillInvocations resolves skill markers anywhere at a whitespace boundary.
func ParseSkillInvocations(text string, catalog Catalog) ([]Invocation, error) {
	available := make(map[string]SkillRef)
	for _, descriptor := range catalog.Commands {
		if descriptor.Skill == nil {
			continue
		}
		available[descriptor.CanonicalToken] = *descriptor.Skill
	}

	invocations := make([]Invocation, 0)
	seen := make(map[string]struct{})
	for offset := 0; offset < len(text); {
		character, size := utf8.DecodeRuneInString(text[offset:])
		if character != '/' || !hasTokenStartBoundary(text, offset) {
			offset += size
			continue
		}
		end := slashTokenEnd(text, offset)
		token := text[offset:end]
		if isPathLikeToken(text[offset:end]) {
			offset = end
			continue
		}
		if ref, ok := available[token]; ok {
			if _, duplicate := seen[ref.CommandID]; !duplicate {
				invocations = append(invocations, Invocation{
					Ref: ref, Token: token, Start: utf16Offset(text, offset), End: utf16Offset(text, end),
				})
				seen[ref.CommandID] = struct{}{}
			}
			offset = end
			continue
		}
		if _, unavailable := catalog.unavailable[token]; unavailable {
			return nil, fmt.Errorf("%w: %s", ErrUnavailable, token)
		}
		offset = end
	}
	return invocations, nil
}

func utf16Offset(text string, byteOffset int) int {
	return len(utf16.Encode([]rune(text[:byteOffset])))
}

// ReconcileInvocations narrows admitted refs to markers that survive a hook rewrite.
func ReconcileInvocations(text string, admitted []Invocation) []Invocation {
	allowed := make(map[string]SkillRef, len(admitted))
	for _, invocation := range admitted {
		allowed[invocation.Token] = invocation.Ref
	}
	present := make(map[string]struct{}, len(admitted))
	for offset := 0; offset < len(text); {
		character, size := utf8.DecodeRuneInString(text[offset:])
		if character != '/' || !hasTokenStartBoundary(text, offset) {
			offset += size
			continue
		}
		end := slashTokenEnd(text, offset)
		token := text[offset:end]
		ref, ok := allowed[token]
		if ok {
			present[ref.CommandID] = struct{}{}
		}
		offset = end
	}
	reconciled := make([]Invocation, 0, len(admitted))
	for _, invocation := range admitted {
		if _, ok := present[invocation.Ref.CommandID]; ok {
			reconciled = append(reconciled, invocation)
		}
	}
	return reconciled
}

func hasTokenStartBoundary(text string, offset int) bool {
	if offset == 0 {
		return true
	}
	previous, _ := utf8.DecodeLastRuneInString(text[:offset])
	return unicode.IsSpace(previous)
}

func slashTokenEnd(text string, offset int) int {
	end := offset + 1
	for end < len(text) {
		character, size := utf8.DecodeRuneInString(text[end:])
		if unicode.IsSpace(character) || isTerminalPunctuation(character) {
			break
		}
		end += size
	}
	for end > offset+1 {
		character, size := utf8.DecodeLastRuneInString(text[offset:end])
		if character != '.' && character != ':' && character != '…' {
			break
		}
		end -= size
	}
	return end
}

func isPathLikeToken(token string) bool {
	return strings.Count(token, "/") > 1
}

func isTerminalPunctuation(character rune) bool {
	switch character {
	case ',', ';', '!', '?', ')', ']', '}', '"', '\'':
		return true
	default:
		return false
	}
}
