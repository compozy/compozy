package memory

import (
	"fmt"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (s *Store) parseControlledWrite(
	scope memcontract.Scope,
	filename string,
	content []byte,
) (string, memcontract.Header, error) {
	return s.parseControlledWriteDocument(scope, filename, content, true)
}

func (s *Store) parseControlledWriteDocument(
	scope memcontract.Scope,
	filename string,
	content []byte,
	validateBudget bool,
) (string, memcontract.Header, error) {
	var header memcontract.Header
	body, err := parseFrontmatter(content, &header)
	if err != nil {
		return "", memcontract.Header{}, fmt.Errorf(
			"memory: parse frontmatter %q: %w",
			filename,
			fmt.Errorf("%w: %v", ErrValidation, err),
		)
	}
	completedHeader, err := s.completeHeaderForScope(scope, header)
	if err != nil {
		return "", memcontract.Header{}, err
	}
	completedHeader.Filename = filename
	if err := completedHeader.Validate(); err != nil {
		return "", memcontract.Header{}, wrapValidationError("validate frontmatter", filename, err)
	}
	normalizedBody := strings.TrimSpace(body)
	if validateBudget {
		if err := s.validateControlledBody(normalizedBody); err != nil {
			return "", memcontract.Header{}, err
		}
	}
	return normalizedBody, completedHeader, nil
}

func (s *Store) validateControlledBody(body string) error {
	byteCount := int64(len(body))
	if s != nil && s.maxFileBytes > 0 && byteCount > s.maxFileBytes {
		return fmt.Errorf(
			"%w: memory body uses %d bytes; maximum is %d bytes",
			ErrValidation,
			byteCount,
			s.maxFileBytes,
		)
	}
	lineCount := controlledBodyLineCount(body)
	if s != nil && s.maxFileLines > 0 && lineCount > s.maxFileLines {
		return fmt.Errorf(
			"%w: memory body uses %d lines; maximum is %d lines",
			ErrValidation,
			lineCount,
			s.maxFileLines,
		)
	}
	return nil
}

func controlledBodyLineCount(body string) int {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "\n") + 1
}
