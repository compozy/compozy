package memory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
)

func (s *Store) writeRaw(
	ctx context.Context,
	scope memcontract.Scope,
	filename string,
	content []byte,
	emitEvent bool,
) (err error) {
	if ctx == nil {
		return errors.New("memory: write context is required")
	}
	normalizedScope := scope.Normalize()
	var header memcontract.Header
	if _, err := parseFrontmatter(content, &header); err != nil {
		return fmt.Errorf("memory: parse frontmatter %q: %w", filename, fmt.Errorf("%w: %v", ErrValidation, err))
	}
	completedHeader, err := s.completeHeaderForScope(normalizedScope, header)
	if err != nil {
		return err
	}
	header = completedHeader
	if err := header.Validate(); err != nil {
		return wrapValidationError("validate frontmatter", filename, err)
	}

	path, err := s.pathFor(normalizedScope, filename)
	if err != nil {
		return err
	}
	if normalizedScope == memcontract.ScopeWorkspace && s.catalog != nil {
		if _, err := s.workspaceIDForRoot(ctx, s.workspaceRoot); err != nil {
			return err
		}
	}
	unlock := s.lockMutations()
	defer unlock()

	directory, err := fileutil.OpenOrCreateDirectory(filepath.Dir(path), dirPerm)
	if err != nil {
		return fmt.Errorf("memory: ensure directory %q: %w", filepath.Dir(path), err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("memory: close mutation directory %q: %w", filepath.Dir(path), closeErr))
		}
	}()
	catalogWasDirty, err := s.prepareCatalogSourceMutation(normalizedScope)
	if err != nil {
		return err
	}
	if err := directory.AtomicWriteFile(filepath.Base(path), content, filePerm, true); err != nil {
		return fmt.Errorf("memory: write %q: %w", path, err)
	}
	_, info, err := directory.ReadRegularFile(filepath.Base(path))
	if err != nil {
		s.recordCommittedMutation()
		return fmt.Errorf("memory: stat written file %q: %w", path, err)
	}
	header.Filename = filepath.Base(path)
	header.FilePath = path
	header.ModTime = info.ModTime()
	syncErr := s.syncScopeAfterWriteErr(ctx, normalizedScope, header, content, catalogWasDirty)
	if err := s.finishCatalogSourceMutation(
		ctx,
		normalizedScope,
		"write",
		header.Filename,
		syncErr,
		emitEvent,
	); err != nil {
		s.recordCommittedMutation()
		return err
	}
	s.recordCommittedMutation()
	if emitEvent {
		s.logMutationEvent(ctx, "write", normalizedScope, filepath.Base(path))
	}
	return nil
}

func (s *Store) finishCatalogSourceMutation(
	ctx context.Context,
	scope memcontract.Scope,
	action string,
	filename string,
	syncErr error,
	emitEvent bool,
) error {
	if syncErr == nil {
		syncErr = s.clearCatalogSourceDirty(scope)
	}
	if syncErr == nil {
		return nil
	}
	syncErr = s.invalidateCatalogIdentityAfterSyncFailure(
		ctx,
		scope,
		action,
		filename,
		syncErr,
	)
	if !emitEvent {
		return syncErr
	}
	s.warn(
		"memory: sync derived state failed after mutation",
		"action", action,
		"scope", scope.Normalize(),
		"filename", strings.TrimSpace(filename),
		"error", syncErr,
	)
	return nil
}

func (s *Store) deleteRaw(
	ctx context.Context,
	scope memcontract.Scope,
	filename string,
	emitEvent bool,
) (err error) {
	if ctx == nil {
		return errors.New("memory: delete context is required")
	}
	normalizedScope := scope.Normalize()
	path, err := s.pathFor(normalizedScope, filename)
	if err != nil {
		return err
	}
	if normalizedScope == memcontract.ScopeWorkspace && s.catalog != nil {
		if _, err := s.workspaceIDForRoot(ctx, s.workspaceRoot); err != nil {
			return err
		}
	}
	unlock := s.lockMutations()
	defer unlock()
	directory, err := fileutil.OpenDirectoryForMutation(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("memory: open delete directory %q: %w", filepath.Dir(path), err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("memory: close delete directory %q: %w", filepath.Dir(path), closeErr))
		}
	}()

	if filepath.Base(path) == indexFilename {
		if err := directory.RemoveRegularFile(filepath.Base(path)); err != nil {
			return fmt.Errorf("memory: delete %q: %w", path, err)
		}
		s.recordCommittedMutation()
		return nil
	}
	catalogWasDirty, err := s.prepareCatalogSourceMutation(normalizedScope)
	if err != nil {
		return err
	}
	if err := directory.RemoveRegularFile(filepath.Base(path)); err != nil {
		return fmt.Errorf("memory: delete %q: %w", path, err)
	}
	syncErr := s.syncScopeAfterDeleteErr(
		ctx,
		normalizedScope,
		filepath.Base(path),
		catalogWasDirty,
	)
	if err := s.finishCatalogSourceMutation(
		ctx,
		normalizedScope,
		"delete",
		filepath.Base(path),
		syncErr,
		emitEvent,
	); err != nil {
		s.recordCommittedMutation()
		return err
	}
	s.recordCommittedMutation()
	if emitEvent {
		s.logMutationEvent(ctx, "delete", normalizedScope, filepath.Base(path))
	}
	return nil
}
