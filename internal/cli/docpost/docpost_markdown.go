package docpost

import (
	"context"

	"fmt"

	"strings"
)

// fenceIndentedBlocks converts tab/4-space indented blocks to fenced code
// blocks. MDX does not treat indentation as code, so indented shell snippets
// with `<` or `{` cause parse errors. This function skips lines already
// inside fenced code blocks.
func fenceIndentedBlocks(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	inFence := false
	inIndent := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track existing fenced code blocks.
		if strings.HasPrefix(trimmed, "```") {
			if inIndent {
				inIndent = false
				result = append(result, "```")
			}
			inFence = !inFence
			result = append(result, line)
			continue
		}

		if inFence {
			result = append(result, line)
			continue
		}

		isIndented := strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ")
		isEmpty := trimmed == ""

		switch {
		case !inIndent && isIndented:
			inIndent = true
			result = append(result, "```", stripIndent(line))
		case inIndent && isIndented:
			result = append(result, stripIndent(line))
		case inIndent && isEmpty:
			result = append(result, line)
		default:
			if inIndent {
				inIndent = false
				result = append(result, "```")
			}
			result = append(result, line)
		}
	}

	if inIndent {
		result = append(result, "```")
	}

	return strings.Join(result, "\n")
}

// stripIndent removes one level of indentation (tab or 4 spaces).
func stripIndent(line string) string {
	if strings.HasPrefix(line, "\t") {
		return line[1:]
	}
	if strings.HasPrefix(line, "    ") {
		return line[4:]
	}
	return line
}

// escapeJSX escapes bare angle brackets in non-code regions so MDX does not
// parse them as JSX tags. Lines inside fenced code blocks (```) and inline
// code spans are left untouched.
func escapeJSX(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	inFence := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			result = append(result, line)
			continue
		}

		if inFence {
			result = append(result, line)
			continue
		}

		result = append(result, escapeLineJSX(line))
	}

	return strings.Join(result, "\n")
}

// escapeLineJSX escapes `<` and `{` in a single line, skipping inline code.
func escapeLineJSX(line string) string {
	if !strings.ContainsAny(line, "<{") {
		return line
	}

	var b strings.Builder
	inCode := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '`' {
			inCode = !inCode
			b.WriteByte(ch)
			continue
		}
		if !inCode {
			if ch == '<' {
				b.WriteString("\\<")
				continue
			}
			if ch == '{' {
				b.WriteString("\\{")
				continue
			}
		}
		b.WriteByte(ch)
	}

	return b.String()
}

// rewriteLinks strips the `.md` extension from Cobra-generated cross-command
// links. The stripped form is then remapped to an absolute URL by remapLinks
// during Process.
func rewriteLinks(raw string) string {
	return transformMarkdownOutsideCode(raw, func(text string) string {
		return crossLinkRe.ReplaceAllString(text, "[$1]($2)")
	})
}

func ensureContext(ctx context.Context, action string) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("docpost: %s: %w", action, err)
	}
	return nil
}

func baseNameToCommand(baseName string) string {
	return strings.ReplaceAll(baseName, "_", " ")
}

func transformMarkdownOutsideCode(raw string, transform func(string) string) string {
	lines := strings.Split(raw, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = transformInlineText(line, transform)
	}
	return strings.Join(lines, "\n")
}

func transformInlineText(line string, transform func(string) string) string {
	if !strings.Contains(line, "`") {
		return transform(line)
	}

	var b strings.Builder
	inCode := false
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] != '`' {
			continue
		}
		if inCode {
			b.WriteString(line[start : i+1])
		} else {
			b.WriteString(transform(line[start:i]))
			b.WriteByte('`')
		}
		inCode = !inCode
		start = i + 1
	}
	if inCode {
		b.WriteString(line[start:])
	} else {
		b.WriteString(transform(line[start:]))
	}
	return b.String()
}
