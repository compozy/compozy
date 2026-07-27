package frontmatter

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const delimiter = "---"

var (
	delimiterBytes = []byte(delimiter)
	crlfBytes      = []byte("\r\n")
	lfBytes        = []byte("\n")
	utf8BOMBytes   = []byte{0xEF, 0xBB, 0xBF}

	// ErrMissing reports content that does not start with a valid YAML frontmatter block.
	ErrMissing = errors.New("frontmatter: missing YAML frontmatter")
	// ErrUnterminated reports content whose opening delimiter has no matching closing delimiter.
	ErrUnterminated = errors.New("frontmatter: unterminated YAML frontmatter")
	// ErrBOM reports content that starts with a UTF-8 BOM before the YAML frontmatter delimiter.
	ErrBOM = errors.New("frontmatter: UTF-8 BOM before YAML frontmatter")
)

// Parts contains the parsed metadata bytes and normalized markdown body.
type Parts struct {
	Metadata []byte
	Body     string
}

// Split normalizes line endings and separates YAML frontmatter from the body.
func Split(content []byte) (Parts, error) {
	normalized := normalizeLineEndings(content)
	if bytes.HasPrefix(normalized, utf8BOMBytes) {
		return Parts{}, ErrBOM
	}
	if !bytes.HasPrefix(normalized, delimiterBytes) {
		return Parts{}, ErrMissing
	}

	openLineEnd := nextLineBoundary(normalized, 0)
	if !bytes.Equal(normalized[:openLineEnd], delimiterBytes) {
		return Parts{}, ErrMissing
	}

	offset := openLineEnd
	if offset < len(normalized) && normalized[offset] == '\n' {
		offset++
	}

	closeStart, closeEnd, ok := findClosingDelimiter(normalized, offset)
	if !ok {
		return Parts{}, ErrUnterminated
	}

	bodyStart := closeEnd
	if bodyStart < len(normalized) && normalized[bodyStart] == '\n' {
		bodyStart++
	}

	return Parts{
		Metadata: normalized[offset:closeStart],
		Body:     string(normalized[bodyStart:]),
	}, nil
}

// Decode splits frontmatter and delegates metadata decoding to the supplied callback.
func Decode(content []byte, decode func([]byte) error) (string, error) {
	if decode == nil {
		return "", errors.New("frontmatter: decode callback is required")
	}

	parts, err := Split(content)
	if err != nil {
		return "", err
	}
	if err := decode(parts.Metadata); err != nil {
		return "", err
	}

	return parts.Body, nil
}

// Format serializes metadata and a Markdown body using the canonical frontmatter byte shape.
func Format[T any](metadata T, body string) (string, error) {
	rawYAML, err := yaml.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("frontmatter: marshal metadata: %w", err)
	}
	return formatRaw(string(rawYAML), body), nil
}

// RewriteStringField updates one top-level scalar without reserializing the surrounding document.
func RewriteStringField(content string, key string, value string) (string, error) {
	if strings.ContainsAny(key, ":\r\n") || strings.TrimSpace(key) != key || key == "" {
		return "", fmt.Errorf("frontmatter: invalid field key %q", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", errors.New("frontmatter: string field value must be single-line")
	}

	normalized := normalizeLineEndings([]byte(content))
	parts, err := Split(normalized)
	if err != nil {
		return "", err
	}
	valueNode, err := frontmatterScalarField(parts.Metadata, key)
	if err != nil {
		return "", err
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("frontmatter: marshal field %q: %w", key, err)
	}
	replacement := bytes.TrimSuffix(encoded, lfBytes)
	metadataStart := nextLineBoundary(normalized, 0) + 1
	lineOffset, err := lineOffset(parts.Metadata, valueNode.Line)
	if err != nil {
		return "", err
	}
	tokenStart := metadataStart + lineOffset + valueNode.Column - 1
	lineEnd := nextLineBoundary(normalized, tokenStart)
	tokenLength, err := scalarTokenLength(normalized[tokenStart:lineEnd], valueNode.Style)
	if err != nil {
		return "", fmt.Errorf("frontmatter: field %q: %w", key, err)
	}

	rewritten := make([]byte, 0, len(normalized)-tokenLength+len(replacement))
	rewritten = append(rewritten, normalized[:tokenStart]...)
	rewritten = append(rewritten, replacement...)
	rewritten = append(rewritten, normalized[tokenStart+tokenLength:]...)
	return string(rewritten), nil
}

func frontmatterScalarField(metadata []byte, key string) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(metadata, &document); err != nil {
		return nil, fmt.Errorf("frontmatter: unmarshal metadata: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("frontmatter: metadata is not a mapping")
	}
	mapping := document.Content[0]
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		keyNode := mapping.Content[index]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Value != key {
			continue
		}
		valueNode := mapping.Content[index+1]
		if valueNode.Kind != yaml.ScalarNode || keyNode.Line != valueNode.Line || valueNode.Column <= 0 {
			return nil, fmt.Errorf("frontmatter: field %q is not a single-line scalar", key)
		}
		return valueNode, nil
	}
	return nil, fmt.Errorf("frontmatter: field %q not found", key)
}

func lineOffset(content []byte, line int) (int, error) {
	if line <= 0 {
		return 0, errors.New("frontmatter: scalar has invalid source position")
	}
	offset := 0
	for current := 1; current < line; current++ {
		boundary := nextLineBoundary(content, offset)
		if boundary == len(content) {
			return 0, errors.New("frontmatter: scalar source line is outside metadata")
		}
		offset = boundary + 1
	}
	return offset, nil
}

func scalarTokenLength(line []byte, style yaml.Style) (int, error) {
	switch style {
	case yaml.DoubleQuotedStyle:
		return quotedScalarLength(line, '"', true)
	case yaml.SingleQuotedStyle:
		return quotedScalarLength(line, '\'', false)
	case yaml.LiteralStyle, yaml.FoldedStyle:
		return 0, errors.New("multiline scalar cannot be rewritten as a string field")
	default:
		end := len(line)
		for index, character := range line {
			if character == '#' && index > 0 && (line[index-1] == ' ' || line[index-1] == '\t') {
				end = index
				break
			}
		}
		for end > 0 && (line[end-1] == ' ' || line[end-1] == '\t') {
			end--
		}
		if end == 0 {
			return 0, errors.New("scalar token is empty")
		}
		return end, nil
	}
}

func quotedScalarLength(line []byte, quote byte, escaped bool) (int, error) {
	if len(line) == 0 || line[0] != quote {
		return 0, errors.New("quoted scalar source does not start with its quote")
	}
	for index := 1; index < len(line); index++ {
		if escaped && line[index] == '\\' {
			index++
			continue
		}
		if line[index] != quote {
			continue
		}
		if !escaped && index+1 < len(line) && line[index+1] == quote {
			index++
			continue
		}
		return index + 1, nil
	}
	return 0, errors.New("quoted scalar source is unterminated")
}

func formatRaw(rawYAML string, body string) string {
	var output strings.Builder
	output.WriteString("---\n")
	output.WriteString(strings.TrimLeft(rawYAML, "\n"))
	output.WriteString("---\n")

	trimmedBody := strings.TrimLeft(body, "\n")
	if trimmedBody != "" {
		output.WriteString("\n")
		output.WriteString(trimmedBody)
		if !strings.HasSuffix(trimmedBody, "\n") {
			output.WriteString("\n")
		}
	}
	return output.String()
}

func normalizeLineEndings(content []byte) []byte {
	if bytes.IndexByte(content, '\r') < 0 {
		return bytes.Clone(content)
	}

	return bytes.ReplaceAll(content, crlfBytes, lfBytes)
}

func nextLineBoundary(content []byte, start int) int {
	if start >= len(content) {
		return len(content)
	}

	if idx := bytes.IndexByte(content[start:], '\n'); idx >= 0 {
		return start + idx
	}

	return len(content)
}

func findClosingDelimiter(content []byte, start int) (int, int, bool) {
	lineStart := start
	for lineStart <= len(content) {
		lineEnd := nextLineBoundary(content, lineStart)
		if bytes.Equal(content[lineStart:lineEnd], delimiterBytes) {
			return lineStart, lineEnd, true
		}
		if lineEnd == len(content) {
			break
		}
		lineStart = lineEnd + 1
	}

	return 0, 0, false
}
