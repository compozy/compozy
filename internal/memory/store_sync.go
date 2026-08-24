package memory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
)

func (s *Store) syncScopeAfterWriteErr(
	ctx context.Context,
	scope memcontract.Scope,
	header memcontract.Header,
	content []byte,
	forceFullSync bool,
) error {
	if forceFullSync {
		return s.syncScope(ctx, scope)
	}
	if needsFullSync, err := s.needsFullSyncAfterMutation(ctx, scope, strings.TrimSpace(header.Filename)); err != nil {
		return err
	} else if needsFullSync {
		return s.syncScope(ctx, scope)
	}

	if err := s.syncIndexAfterWrite(scope, header); err != nil {
		return err
	}
	if s.catalog == nil {
		return nil
	}

	_, workspaceID, err := s.catalogWorkspaceForScope(ctx, scope)
	if err != nil {
		return err
	}
	doc, err := buildCatalogDocument(s.profileIDForScope(scope), scope, workspaceID, header, content)
	if err != nil {
		return err
	}
	return s.catalog.upsertDocument(ctx, doc)
}

func (s *Store) syncScopeAfterDeleteErr(
	ctx context.Context,
	scope memcontract.Scope,
	filename string,
	forceFullSync bool,
) error {
	if forceFullSync {
		return s.syncScope(ctx, scope)
	}
	needsFullSync, err := s.needsFullSyncAfterMutation(ctx, scope, strings.TrimSpace(filename))
	if err != nil {
		return err
	}
	if needsFullSync {
		return s.syncScope(ctx, scope)
	}

	if err := s.syncIndexAfterDelete(scope, filename); err != nil {
		return err
	}
	if s.catalog == nil {
		return nil
	}

	_, workspaceID, err := s.catalogWorkspaceForScope(ctx, scope)
	if err != nil {
		return err
	}
	return s.catalog.deleteDocument(
		ctx,
		s.profileIDForScope(scope),
		scope,
		workspaceID,
		s.catalogAgentName(scope),
		s.catalogAgentTier(scope),
		filename,
	)
}

func (s *Store) needsFullSyncAfterMutation(
	ctx context.Context,
	scope memcontract.Scope,
	filename string,
) (bool, error) {
	indexMissing, err := s.indexMissingWithExistingDocuments(scope, filename)
	if err != nil {
		return false, err
	}
	if indexMissing {
		return true, nil
	}
	if s.catalog == nil {
		return false, nil
	}

	_, workspaceID, err := s.catalogWorkspaceForScope(ctx, scope)
	if err != nil {
		return false, err
	}
	ready, err := s.catalog.identityReady(ctx, s.catalogIdentity(scope, workspaceID))
	if err != nil {
		return false, err
	}
	return !ready, nil
}

func (s *Store) indexMissingWithExistingDocuments(
	scope memcontract.Scope,
	mutatedFilename string,
) (_ bool, err error) {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return false, err
	}
	directory, err := fileutil.OpenDirectory(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("memory: inspect memory directory %q: %w", dir, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("memory: close inspected directory %q: %w", dir, closeErr))
		}
	}()

	index, err := directory.OpenRegularFile(indexFilename)
	if err == nil {
		if err := index.Close(); err != nil {
			return false, fmt.Errorf("memory: close index %q: %w", filepath.Join(dir, indexFilename), err)
		}
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("memory: open index %q: %w", filepath.Join(dir, indexFilename), err)
	}

	names, err := directory.ReadDir()
	if err != nil {
		return false, fmt.Errorf("memory: inspect memory directory %q: %w", dir, err)
	}
	for _, name := range names {
		if shouldSkipFile(name) || name == strings.TrimSpace(mutatedFilename) {
			continue
		}
		file, err := directory.OpenRegularFile(name)
		if errors.Is(err, fileutil.ErrDirectory) || errors.Is(err, fileutil.ErrNotRegular) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("memory: inspect entry %q: %w", filepath.Join(dir, name), err)
		}
		if err := file.Close(); err != nil {
			return false, fmt.Errorf("memory: close inspected entry %q: %w", filepath.Join(dir, name), err)
		}
		return true, nil
	}
	return false, nil
}

func (s *Store) syncIndexAfterWrite(scope memcontract.Scope, header memcontract.Header) error {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return err
	}
	indexPath := filepath.Join(dir, indexFilename)
	lines, err := readIndexLines(indexPath)
	if err != nil {
		return err
	}

	filename := strings.TrimSpace(header.Filename)
	updated := []string{renderIndexLine(header)}
	for _, line := range lines {
		target, ok := firstMarkdownLinkTarget(line)
		if ok && target == filename {
			continue
		}
		updated = append(updated, line)
	}
	return writeIndexLines(indexPath, updated)
}

func (s *Store) syncIndexAfterDelete(scope memcontract.Scope, filename string) error {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return err
	}
	indexPath := filepath.Join(dir, indexFilename)
	lines, err := readIndexLines(indexPath)
	if err != nil {
		return err
	}

	targetFilename := strings.TrimSpace(filename)
	updated := make([]string, 0, len(lines))
	for _, line := range lines {
		target, ok := firstMarkdownLinkTarget(line)
		if ok && target == targetFilename {
			continue
		}
		updated = append(updated, line)
	}
	return writeIndexLines(indexPath, updated)
}

func (s *Store) warn(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Warn(msg, args...)
		return
	}

	slog.Warn(msg, args...)
}
