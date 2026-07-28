package config

import (
	"maps"

	"strings"
	"unicode"

	"github.com/joho/godotenv"
)

func parseDotEnvDocument(raw string) dotEnvParseResult {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	finalNewline := len(lines) > 0 && lines[len(lines)-1] == ""
	if finalNewline {
		lines = lines[:len(lines)-1]
	}

	result := dotEnvParseResult{
		values:       map[string]string{},
		lines:        make([]string, 0, len(lines)),
		finalNewline: finalNewline,
	}

	for idx, line := range lines {
		parsedLines, values, diagnostics, needsRepair, unsupported := parseDotEnvLine(idx+1, line)
		result.lines = append(result.lines, parsedLines...)
		maps.Copy(result.values, values)
		result.diagnostics = append(result.diagnostics, diagnostics...)
		result.needsRepair = result.needsRepair || needsRepair
		result.unsupported = result.unsupported || unsupported
	}

	return result
}

func parseDotEnvLine(
	lineNumber int,
	line string,
) ([]string, map[string]string, []DotEnvDiagnostic, bool, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return []string{line}, nil, nil, false, false
	}

	lineForSplit := trimmed
	if after, ok := strings.CutPrefix(lineForSplit, "export "); ok {
		lineForSplit = strings.TrimSpace(after)
	}
	segments := splitDotEnvAssignments(lineForSplit)
	if len(segments) == 0 {
		return []string{line}, nil, []DotEnvDiagnostic{{
			Line:    lineNumber,
			Code:    dotEnvDiagnosticUnsupportedLine,
			Message: ".env line is not a KEY=VALUE assignment",
		}}, false, true
	}

	parsedLines := make([]string, 0, len(segments))
	values := make(map[string]string, len(segments))
	diagnostics := make([]DotEnvDiagnostic, 0)
	needsRepair := len(segments) > 1
	unsupported := false
	if len(segments) > 1 {
		diagnostics = append(diagnostics, DotEnvDiagnostic{
			Line:    lineNumber,
			Code:    dotEnvDiagnosticMultiKeyLine,
			Message: ".env line contains multiple assignments and can be split safely",
		})
	}

	for _, segment := range segments {
		parsedLine, key, value, segmentDiagnostics, segmentNeedsRepair, segmentUnsupported := parseDotEnvAssignment(
			lineNumber,
			segment,
			len(segments) > 1,
		)
		diagnostics = append(diagnostics, segmentDiagnostics...)
		needsRepair = needsRepair || segmentNeedsRepair
		unsupported = unsupported || segmentUnsupported
		if segmentUnsupported {
			continue
		}
		parsedLines = append(parsedLines, parsedLine)
		values[key] = value
	}

	if len(parsedLines) == 0 {
		parsedLines = append(parsedLines, line)
	}
	if !needsRepair && !unsupported {
		parsedLines = []string{line}
	}
	return parsedLines, values, diagnostics, needsRepair, unsupported
}

func parseDotEnvAssignment(
	lineNumber int,
	line string,
	forceRewrite bool,
) (string, string, string, []DotEnvDiagnostic, bool, bool) {
	candidate := strings.TrimSpace(line)
	if after, ok := strings.CutPrefix(candidate, "export "); ok {
		candidate = strings.TrimSpace(after)
	}

	key, ok := dotEnvAssignmentKey(candidate)
	if !ok {
		return line, "", "", []DotEnvDiagnostic{{
			Line:    lineNumber,
			Code:    dotEnvDiagnosticInvalidKey,
			Message: ".env assignment key must match [A-Za-z_][A-Za-z0-9_]*",
		}}, false, true
	}

	parsed, err := godotenv.Unmarshal(candidate)
	if err != nil {
		return line, "", "", []DotEnvDiagnostic{{
			Line:    lineNumber,
			Key:     key,
			Code:    dotEnvDiagnosticUnsupportedFragment,
			Message: "assignment value uses unsupported .env syntax",
		}}, false, true
	}
	value, ok := parsed[key]
	if !ok {
		return line, "", "", []DotEnvDiagnostic{{
			Line:    lineNumber,
			Key:     key,
			Code:    dotEnvDiagnosticUnsupportedFragment,
			Message: "assignment could not be parsed as a single .env key",
		}}, false, true
	}

	sanitized := sanitizeDotEnvValue(key, value)
	needsRepair := forceRewrite || sanitized != value
	diagnostics := []DotEnvDiagnostic(nil)
	if sanitized != value {
		diagnostics = append(diagnostics, DotEnvDiagnostic{
			Line:    lineNumber,
			Key:     key,
			Code:    dotEnvDiagnosticSanitizedSecret,
			Message: "secret-like .env value contains non-ASCII characters that will be stripped",
		})
	}
	if needsRepair {
		return formatDotEnvAssignment(key, sanitized), key, sanitized, diagnostics, true, false
	}
	return line, key, sanitized, diagnostics, false, false
}

func splitDotEnvAssignments(line string) []string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil
	}
	startKey, ok := dotEnvAssignmentKey(trimmed)
	if !ok || startKey == "" {
		return nil
	}

	segments := make([]string, 0, 1)
	start := 0
	inSingle := false
	inDouble := false
	escaped := false
	for idx, r := range trimmed {
		if escaped {
			escaped = false
			continue
		}
		if inDouble && r == '\\' {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		default:
			if inSingle || inDouble || !unicode.IsSpace(r) {
				continue
			}
			next := nextNonSpaceIndex(trimmed, idx)
			if next < 0 {
				continue
			}
			if strings.HasPrefix(trimmed[next:], "export ") {
				exportValueStart := next + len("export ")
				next = nextNonSpaceIndex(trimmed, exportValueStart-1)
				if next < 0 {
					continue
				}
			}
			if _, ok := dotEnvAssignmentKey(trimmed[next:]); ok {
				segments = append(segments, strings.TrimSpace(trimmed[start:idx]))
				start = next
			}
		}
	}
	segments = append(segments, strings.TrimSpace(trimmed[start:]))
	return segments
}
