package workspace

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

func scanWorkspaceProfileDeclarations(
	workspaceRoot string,
	snapshots map[string]filesnap.Snapshot,
) ([]ProfileDeclaration, error) {
	profilesRoot := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName)
	if err := addDependencySnapshot(profilesRoot, snapshots); err != nil {
		return nil, fmt.Errorf("workspace: snapshot profile declarations %q: %w", profilesRoot, err)
	}
	entries, err := os.ReadDir(profilesRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("workspace: read profile declarations %q: %w", profilesRoot, err)
	}

	declarations := make([]ProfileDeclaration, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || compozyconfig.ValidateResourceProfileName(entry.Name()) != nil {
			continue
		}
		path := filepath.Join(profilesRoot, entry.Name())
		if err := addDependencySnapshot(path, snapshots); err != nil {
			return nil, fmt.Errorf("workspace: snapshot profile declaration %q: %w", path, err)
		}
		contents, readErr := os.ReadDir(path)
		if readErr != nil {
			return nil, fmt.Errorf("workspace: read profile declaration %q: %w", path, readErr)
		}
		if len(contents) == 0 {
			continue
		}
		declarations = append(declarations, ProfileDeclaration{Name: entry.Name(), Path: path})
	}
	slices.SortFunc(declarations, func(a, b ProfileDeclaration) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return declarations, nil
}
