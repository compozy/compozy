package memory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
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
	doc, err := buildCatalogDocument(scope, workspaceID, header, content)
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
	ready, err := s.catalog.identityReady(ctx, newCatalogIdentity(
		scope,
		workspaceID,
		s.catalogAgentName(scope),
		s.catalogAgentTier(scope),
	))
	if err != nil {
		return false, err
	}
	return !ready, nil
}

func (s *Store) indexMissingWithExistingDocuments(scope memcontract.Scope, mutatedFilename string) (bool, error) {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return false, err
	}
	indexPath := filepath.Join(dir, indexFilename)
	if _, err := os.Stat(indexPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("memory: stat index %q: %w", indexPath, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("memory: inspect memory directory %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || shouldSkipFile(entry.Name()) || entry.Name() == strings.TrimSpace(mutatedFilename) {
			continue
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
