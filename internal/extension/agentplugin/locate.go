package agentplugin

import (
	"errors"
	"fmt"
	"io"
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

var _ error = &NotManifestError{}

func (e *NotManifestError) Error() string {
	return fmt.Sprintf("no plugin manifest in %q; checked %s", e.Root, strings.Join(e.Checked, ", "))
}

// ManifestDocument retains one securely acquired manifest for classification and loading.
type ManifestDocument struct {
	Path    string
	Layout  string
	root    string
	content []byte
}

func ReadManifest(root string) (*ManifestDocument, error) {
	file, path, layout, err := openManifest(root)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(file)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, fmt.Errorf("read plugin manifest %q: %w", path, err)
	}
	return &ManifestDocument{Path: path, Layout: layout, root: root, content: content}, nil
}

// LocateManifest selects a manifest when only its path and layout are needed.
func LocateManifest(root string) (string, string, error) {
	file, path, layout, err := openManifest(root)
	if err != nil {
		return "", "", err
	}
	if err := file.Close(); err != nil {
		return "", "", fmt.Errorf("close plugin manifest %q: %w", path, err)
	}
	return path, layout, nil
}

func openManifest(root string) (*os.File, string, string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, "", "", errors.New("plugin root is required")
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
			return nil, "", "", fmt.Errorf("inspect plugin manifest %q: %w", path, err)
		}
		return file, path, candidate.layout, nil
	}
	return nil, "", "", &NotManifestError{Root: root, Checked: checked}
}
