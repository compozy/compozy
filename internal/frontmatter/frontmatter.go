package frontmatter

import (
	"bytes"
	"errors"
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
