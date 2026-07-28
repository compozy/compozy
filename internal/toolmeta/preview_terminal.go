package toolmeta

import "strings"

func terminalPreview(input map[string]any) string {
	command := scalarString(input["command"])
	if command == "" {
		command = scalarString(input["cmd"])
	}
	parts := splitShellOutsideQuotes(command, true)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		pipeline := splitShellOutsideQuotes(part, false)
		if len(pipeline) == 0 {
			continue
		}
		if commandName := terminalCommandName(pipeline[0]); commandName != "" {
			result = append(result, commandName)
		}
	}
	return strings.Join(result, " · ")
}

func terminalCommandName(value string) string {
	for _, word := range terminalShellWords(oneLine(value)) {
		if isShellAssignment(word) {
			continue
		}
		if separator := strings.LastIndexAny(word, `/\\`); separator >= 0 {
			word = word[separator+1:]
		}
		return strings.TrimSpace(word)
	}
	return ""
}

func terminalShellWords(value string) []string {
	words := make([]string, 0, 2)
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() == 0 {
			return
		}
		words = append(words, current.String())
		current.Reset()
	}
	for _, char := range value {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
				continue
			}
			current.WriteRune(char)
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' || char == '\r' || char == '\n' {
			flush()
			continue
		}
		current.WriteRune(char)
	}
	if escaped {
		current.WriteRune('\\')
	}
	flush()
	return words
}

func isShellAssignment(value string) bool {
	key, _, found := strings.Cut(value, "=")
	if !found || key == "" {
		return false
	}
	for index, char := range key {
		if char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}

func splitShellOutsideQuotes(value string, compounds bool) []string {
	parts := make([]string, 0, 2)
	start := 0
	var quote rune
	escaped := false
	runes := []rune(value)
	for idx := 0; idx < len(runes); idx++ {
		char := runes[idx]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}

		width := 0
		if compounds {
			switch {
			case char == ';':
				width = 1
			case idx+1 < len(runes) && char == '&' && runes[idx+1] == '&':
				width = 2
			case idx+1 < len(runes) && char == '|' && runes[idx+1] == '|':
				width = 2
			}
		} else if char == '|' {
			width = 1
		}
		if width == 0 {
			continue
		}
		parts = append(parts, string(runes[start:idx]))
		idx += width - 1
		start = idx + 1
	}
	parts = append(parts, string(runes[start:]))
	return parts
}
