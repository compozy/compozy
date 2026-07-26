package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/agh/internal/fileutil"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const catalogDirtyMarkerFilename = ".agh-catalog-dirty"

func (s *Store) prepareCatalogSourceMutation(scope memcontract.Scope) (bool, error) {
	if s.catalog == nil {
		return false, nil
	}
	dirty, err := s.catalogSourceDirty(scope)
	if err != nil {
		return false, err
	}
	path, err := s.catalogDirtyMarkerPath(scope)
	if err != nil {
		return false, err
	}
	if err := fileutil.AtomicWrite(path, []byte("dirty\n")); err != nil {
		return false, fmt.Errorf("memory: mark catalog source dirty %q: %w", path, err)
	}
	return dirty, nil
}

func (s *Store) clearCatalogSourceDirty(scope memcontract.Scope) error {
	if s.catalog == nil {
		return nil
	}
	path, err := s.catalogDirtyMarkerPath(scope)
	if err != nil {
		return err
	}
	if err := fileutil.AtomicRemoveFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("memory: clear catalog dirty marker %q: %w", path, err)
	}
	return nil
}

func (s *Store) catalogSourceDirty(scope memcontract.Scope) (bool, error) {
	if s.catalog == nil {
		return false, nil
	}
	path, err := s.catalogDirtyMarkerPath(scope)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("memory: stat catalog dirty marker %q: %w", path, err)
	}
	return true, nil
}

func (s *Store) catalogDirtyMarkerPath(scope memcontract.Scope) (string, error) {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, catalogDirtyMarkerFilename), nil
}

func (s *Store) catalogSourceStore(scope memcontract.Scope, workspaceRoot string) *Store {
	if scope.Normalize() == memcontract.ScopeWorkspace {
		return s.ForWorkspace(workspaceRoot)
	}
	return s
}
