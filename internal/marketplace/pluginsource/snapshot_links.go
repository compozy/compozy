package pluginsource

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/compozy/compozy/internal/registry"
)

func restoreSnapshotLinks(path string, links map[string]string) (err error) {
	if len(links) == 0 {
		return nil
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	names := slices.Sorted(maps.Keys(links))
	for _, name := range names {
		if err := root.Symlink(links[name], name); err != nil {
			return fmt.Errorf("%w: %q: %w", registry.ErrPathTraversesSymlink, name, err)
		}
	}
	for _, name := range names {
		if _, err := root.Stat(name); err != nil {
			return fmt.Errorf("%w: %q: %w", registry.ErrPathTraversesSymlink, name, err)
		}
	}
	return nil
}
