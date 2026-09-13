package agentplugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// LayoutStandard identifies the portable root manifest grammar.
const LayoutStandard = "standard"

var manifestLocations = []struct{ path, layout string }{
	{"plugin.json", LayoutStandard},
	{".claude-plugin/plugin.json", "claude-plugin"},
	{".codex-plugin/plugin.json", "codex-plugin"},
	{".cursor-plugin/plugin.json", "cursor-plugin"},
}

// NotManifestError records the candidate paths checked during lookup.
type NotManifestError struct {
	Root    string
	Checked []string
}

var _ error = (*NotManifestError)(nil)

func (e *NotManifestError) Error() string {
	return fmt.Sprintf("no plugin manifest in %q; checked %s", e.Root, strings.Join(e.Checked, ", "))
}

// LocateManifest selects the first authored manifest without following links.
func LocateManifest(root string) (string, string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", "", errors.New("plugin root is required")
	}
	checked := make([]string, 0, len(manifestLocations))
	for _, candidate := range manifestLocations {
		path := filepath.Join(root, filepath.FromSlash(candidate.path))
		checked = append(checked, path)
		file, err := fileutil.OpenRegularFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", fmt.Errorf("inspect plugin manifest %q: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return "", "", fmt.Errorf("close plugin manifest %q: %w", path, err)
		}
		return path, candidate.layout, nil
	}
	return "", "", &NotManifestError{Root: root, Checked: checked}
}
