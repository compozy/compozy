package projectfiles

import "github.com/compozy/compozy/internal/fileutil"

func CanonicalRoot(root Root) (Root, error) {
	canonical, err := fileutil.CanonicalExistingDirectoryExact(root.AbsPath)
	if err != nil {
		return Root{}, ErrRootUnavailable
	}
	root.AbsPath = canonical
	return root, nil
}
