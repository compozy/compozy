package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

const observabilityGlobalKey = "global"

type observabilitySource struct {
	id      string
	path    string
	store   *Store
	catalog *catalog
	filters []catalogFilter
	owned   bool
}

func (s *Store) observabilitySources(ctx context.Context, workspaces []string) ([]observabilitySource, error) {
	sources := make([]observabilitySource, 0, len(workspaces)+1)
	seenPaths := make(map[string]struct{})
	globalSource := -1
	seenGlobalWorkspaceIDs := make(map[string]struct{})
	if s.catalog != nil {
		path := filepath.Clean(s.catalog.path)
		globalSource = len(sources)
		sources = append(sources, observabilitySource{
			id:      observabilityGlobalKey,
			path:    path,
			store:   s,
			catalog: s.catalog,
			filters: []catalogFilter{{scope: memcontract.ScopeGlobal}},
		})
		seenPaths[path] = struct{}{}
	}
	for _, workspace := range workspaces {
		if globalSource >= 0 {
			filter, ok, err := workspaceObservabilityFilter(ctx, workspace)
			if err != nil {
				return nil, errors.Join(err, closeObservabilitySources(ctx, sources))
			}
			if ok {
				workspaceID := strings.TrimSpace(filter.workspaceID)
				if _, exists := seenGlobalWorkspaceIDs[workspaceID]; !exists {
					seenGlobalWorkspaceIDs[workspaceID] = struct{}{}
					sources[globalSource].filters = append(sources[globalSource].filters, filter)
				}
			}
		}
		source, ok, err := s.workspaceObservabilitySource(ctx, workspace)
		if err != nil {
			return nil, errors.Join(err, closeObservabilitySources(ctx, sources))
		}
		if !ok {
			continue
		}
		if _, exists := seenPaths[source.path]; exists {
			if source.owned {
				if err := source.store.CloseCatalog(ctx); err != nil {
					return nil, errors.Join(err, closeObservabilitySources(ctx, sources))
				}
			}
			continue
		}
		seenPaths[source.path] = struct{}{}
		sources = append(sources, source)
	}
	return sources, nil
}

func workspaceObservabilityFilter(ctx context.Context, workspace string) (catalogFilter, bool, error) {
	workspaceRoot := canonicalWorkspaceRoot(workspace)
	if workspaceRoot == "" {
		return catalogFilter{}, false, nil
	}
	identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		return catalogFilter{}, false, fmt.Errorf(
			"memory: resolve workspace identity for %q: %w",
			workspaceRoot,
			err,
		)
	}
	return catalogFilter{
		scope:         memcontract.ScopeWorkspace,
		workspaceRoot: workspaceRoot,
		workspaceID:   identity.WorkspaceID,
	}, true, nil
}

func (s *Store) healthSources(ctx context.Context, workspaces []string) ([]observabilitySource, error) {
	sources, err := s.observabilitySources(ctx, workspaces)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, nil
	}

	workspaceFilters := make([]catalogFilter, 0, len(workspaces))
	seenWorkspaceIDs := make(map[string]struct{})
	for _, workspace := range workspaces {
		workspaceRoot := canonicalWorkspaceRoot(workspace)
		if workspaceRoot == "" {
			continue
		}
		identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			return nil, errors.Join(
				fmt.Errorf("memory: resolve workspace identity for %q: %w", workspaceRoot, err),
				closeObservabilitySources(ctx, sources),
			)
		}
		if _, exists := seenWorkspaceIDs[identity.WorkspaceID]; exists {
			continue
		}
		seenWorkspaceIDs[identity.WorkspaceID] = struct{}{}
		workspaceFilters = append(workspaceFilters, catalogFilter{
			scope:         memcontract.ScopeWorkspace,
			workspaceRoot: workspaceRoot,
			workspaceID:   identity.WorkspaceID,
		})
	}

	for idx := range sources {
		if sources[idx].id == observabilityGlobalKey {
			sources[idx].filters = append(sources[idx].filters, workspaceFilters...)
			continue
		}
		workspaceID := strings.TrimPrefix(sources[idx].id, "workspace-")
		sources[idx].filters = filterCatalogFiltersByWorkspaceID(sources[idx].filters, workspaceID)
	}
	return sources, nil
}

func (s *Store) workspaceObservabilitySource(
	ctx context.Context,
	workspace string,
) (observabilitySource, bool, error) {
	workspaceRoot := canonicalWorkspaceRoot(workspace)
	if workspaceRoot == "" {
		return observabilitySource{}, false, nil
	}
	identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		return observabilitySource{}, false, fmt.Errorf(
			"memory: resolve workspace identity for %q: %w",
			workspaceRoot,
			err,
		)
	}
	dbPath := filepath.Clean(filepath.Join(filepath.Dir(identity.Path), storepkg.GlobalDatabaseName))
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return observabilitySource{}, false, nil
		}
		return observabilitySource{}, false, fmt.Errorf(
			"memory: stat workspace observability database %q: %w",
			dbPath,
			err,
		)
	}
	store := s.ForWorkspace(workspaceRoot)
	store.catalog = newCatalog(dbPath, func() time.Time {
		return time.Now().UTC()
	})
	if err := store.OpenCatalog(ctx); err != nil {
		return observabilitySource{}, false, fmt.Errorf(
			"memory: open workspace observability database %q: %w",
			dbPath,
			err,
		)
	}
	return observabilitySource{
		id:      "workspace-" + identity.WorkspaceID,
		path:    dbPath,
		store:   store,
		catalog: store.catalog,
		filters: []catalogFilter{{
			scope:         memcontract.ScopeWorkspace,
			workspaceRoot: workspaceRoot,
			workspaceID:   identity.WorkspaceID,
		}},
		owned: true,
	}, true, nil
}

func closeObservabilitySources(ctx context.Context, sources []observabilitySource) error {
	var closeErrors []error
	for _, source := range sources {
		if !source.owned || source.store == nil {
			continue
		}
		if err := source.store.CloseCatalog(ctx); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("memory: close %s observability source: %w", source.id, err))
		}
	}
	return errors.Join(closeErrors...)
}
