package skills

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/fileutil"
)

func ensurePathWithinRoot(root string, candidate string) error {
	resolvedRoot, resolvedCandidate, err := fileutil.ResolveExistingPathWithinRoot(root, candidate)
	if err == nil {
		return nil
	}
	if errors.Is(err, fileutil.ErrPathOutsideRoot) {
		return fmt.Errorf(
			"skills: path %q escapes skill root %q: %w",
			resolvedCandidate,
			resolvedRoot,
			err,
		)
	}
	return fmt.Errorf("skills: validate path %q within root %q: %w", candidate, root, err)
}
