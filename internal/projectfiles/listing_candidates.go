package projectfiles

import (
	"os"
	"path"
	"slices"
)

// Resolve only symlink kinds before ordering; full metadata is bounded by the response limit.
func orderedListingChildren(root Root, confined *os.Root, dir string, children []os.DirEntry) []os.DirEntry {
	directories := map[string]bool{}
	for _, child := range children {
		if child.IsDir() {
			directories[child.Name()] = true
			continue
		}
		if child.Type()&os.ModeSymlink == 0 {
			continue
		}
		rel, _, err := resolvePath(root, path.Join(dir, child.Name()))
		if err != nil {
			continue
		}
		info, err := confined.Stat(rel)
		if err == nil && info.IsDir() {
			directories[child.Name()] = true
		}
	}
	slices.SortFunc(children, func(a, b os.DirEntry) int {
		if directories[a.Name()] != directories[b.Name()] {
			if directories[a.Name()] {
				return -1
			}
			return 1
		}
		return naturalCompare(a.Name(), b.Name())
	})
	return children
}
