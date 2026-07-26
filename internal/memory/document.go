package memory

import (
	"fmt"
	"path/filepath"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// ParseHeader decodes and validates memory frontmatter from a raw document.
func ParseHeader(content []byte) (memcontract.Header, error) {
	var header memcontract.Header

	if _, err := parseFrontmatter(content, &header); err != nil {
		return memcontract.Header{}, fmt.Errorf(
			"memory: parse frontmatter: %w",
			fmt.Errorf("%w: %v", ErrValidation, err),
		)
	}
	if err := header.Validate(); err != nil {
		return memcontract.Header{}, fmt.Errorf(
			"memory: validate frontmatter: %w",
			fmt.Errorf("%w: %v", ErrValidation, err),
		)
	}

	return header, nil
}

// ConsolidationLockPath returns the canonical lock path for a global memory directory.
func ConsolidationLockPath(globalDir string) string {
	return filepath.Join(cleanDirPath(globalDir), consolidationLockName)
}
