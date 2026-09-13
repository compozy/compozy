package marketplace

import (
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// ResolveMarketplaceInstallTarget validates the final install destination.
func ResolveMarketplaceInstallTarget(
	skillsDir string,
	parsedName string,
	targetDirOverride string,
) (string, error) {
	if trimmedOverride := strings.TrimSpace(targetDirOverride); trimmedOverride != "" {
		return fileutil.ResolvePathWithinRoot(skillsDir, trimmedOverride)
	}
	name, err := NormalizeSkillName(parsedName)
	if err != nil {
		return "", err
	}
	return fileutil.ResolvePathWithinRoot(skillsDir, filepath.Join(skillsDir, name))
}
