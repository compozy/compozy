package contracts

import (
	"encoding/json"
	"sort"
	"strings"
)

type candidate struct {
	raw    json.RawMessage
	offset int
}

// ExtractCandidate returns the newest valid top-level JSON object in text.
func ExtractCandidate(text string) (json.RawMessage, bool) {
	candidates := ExtractCandidates(text)
	if len(candidates) == 0 {
		return nil, false
	}
	return cloneRaw(candidates[0]), true
}

// ExtractCandidates returns distinct object candidates newest-first.
func ExtractCandidates(text string) []json.RawMessage {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	found := make([]candidate, 0, 6)
	if raw, ok := validObject(trimmed); ok {
		found = append(found, candidate{raw: raw, offset: strings.Index(text, trimmed)})
	}
	found = append(found, fencedObjects(text)...)
	found = append(found, balancedObjects(text)...)
	sort.SliceStable(found, func(i, j int) bool { return found[i].offset > found[j].offset })
	seen := make(map[string]struct{}, len(found))
	result := make([]json.RawMessage, 0, len(found))
	for _, item := range found {
		key := string(item.raw)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cloneRaw(item.raw))
	}
	return result
}

func fencedObjects(text string) []candidate {
	var result []candidate
	for cursor := 0; cursor < len(text); {
		open := strings.Index(text[cursor:], "```")
		if open < 0 {
			break
		}
		bodyStart := cursor + open + 3
		closeOffset := strings.Index(text[bodyStart:], "```")
		if closeOffset < 0 {
			break
		}
		body := text[bodyStart : bodyStart+closeOffset]
		bodyOffset := bodyStart
		if newline := strings.IndexByte(body, '\n'); newline >= 0 {
			body = body[newline+1:]
			bodyOffset += newline + 1
		}
		trimmed := strings.TrimSpace(body)
		if raw, ok := validObject(trimmed); ok {
			result = append(result, candidate{raw: raw, offset: bodyOffset + strings.Index(body, trimmed)})
		}
		cursor = bodyStart + closeOffset + 3
	}
	return result
}

func balancedObjects(text string) []candidate {
	var result []candidate
	for cursor := 0; cursor < len(text); {
		offset := strings.IndexByte(text[cursor:], '{')
		if offset < 0 {
			break
		}
		start := cursor + offset
		raw, end, ok := balancedObjectAt(text, start)
		if ok {
			result = append(result, candidate{raw: raw, offset: start})
			cursor = end
			continue
		}
		cursor = start + 1
	}
	return result
}

func balancedObjectAt(text string, start int) (json.RawMessage, int, bool) {
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(text); index++ {
		char := text[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				raw, ok := validObject(strings.TrimSpace(text[start : index+1]))
				return raw, index + 1, ok
			}
		}
	}
	return nil, len(text), false
}

func validObject(value string) (json.RawMessage, bool) {
	if value == "" || value[0] != '{' || !json.Valid([]byte(value)) {
		return nil, false
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(value), &object); err != nil || object == nil {
		return nil, false
	}
	return json.RawMessage(value), true
}
