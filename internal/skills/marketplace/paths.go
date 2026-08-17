package marketplace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

// PathInsideRoot resolves a target path and validates it remains under root.
func PathInsideRoot(root string, target string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", registrypkg.ErrPathRootRequired
	}
	resolvedRoot, err := fileutil.CanonicalPathWithExistingPrefix(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	resolvedTarget, err := fileutil.CanonicalPathWithExistingPrefix(target)
	if err != nil {
		return "", fmt.Errorf("resolve target %q: %w", target, err)
	}

	relative, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return "", fmt.Errorf("resolve target %q within %q: %w", resolvedTarget, resolvedRoot, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", registrypkg.ErrPathOutsideRoot
	}
	return resolvedTarget, nil
}

// ResolveMarketplaceInstallTarget validates the final install destination.
func ResolveMarketplaceInstallTarget(
	skillsDir string,
	parsedName string,
	targetDirOverride string,
) (string, error) {
	if trimmedOverride := strings.TrimSpace(targetDirOverride); trimmedOverride != "" {
		return PathInsideRoot(skillsDir, trimmedOverride)
	}
	name, err := NormalizeSkillName(parsedName)
	if err != nil {
		return "", err
	}
	return PathInsideRoot(skillsDir, filepath.Join(skillsDir, name))
}
