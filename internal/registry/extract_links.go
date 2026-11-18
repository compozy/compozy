package registry

import (
	"fmt"
	"path"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func rejectArchiveLinkParent(name string, links map[string]string) error {
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		if _, exists := links[parent]; exists {
			return fmt.Errorf("%w: %q", ErrPathTraversesSymlink, name)
		}
	}
	return nil
}

func collectArchiveSymlink(
	root *fileutil.Directory,
	name, target string,
	links map[string]string,
	budget *archiveTreeBudget,
) error {
	if err := rejectArchiveLinkParent(name, links); err != nil {
		return err
	}
	if _, exists := budget.nodes[name]; exists {
		return fmt.Errorf("%w: %q", ErrArchiveDuplicateEntry, name)
	}
	if target == "" || strings.HasPrefix(target, "/") || hasWindowsVolumeName(target) ||
		strings.ContainsAny(target, "\\\x00") {
		return fmt.Errorf("%w: %q", ErrPathTraversesSymlink, name)
	}
	if _, err := cleanArchiveEntryPath(path.Join(path.Dir(name), target)); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrPathTraversesSymlink, name, err)
	}
	if err := budget.reserve(name); err != nil {
		return err
	}
	parent, _, err := openExtractionParent(root, name, true, 0o755)
	if err != nil {
		return err
	}
	if err := closeOwnedExtractionDirectory(root, parent, nil); err != nil {
		return err
	}
	links[name] = target
	return nil
}
