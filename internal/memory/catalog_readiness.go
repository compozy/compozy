package memory

import (
	"context"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (s *Store) ensureCatalogReady(
	ctx context.Context,
	scope memcontract.Scope,
	workspaceRoot string,
	workspaceID string,
) error {
	if s.catalog == nil {
		return nil
	}

	filters := []catalogFilter{{scope: memcontract.ScopeGlobal}}
	switch scope.Normalize() {
	case memcontract.ScopeGlobal:
		filters = filters[:1]
	case memcontract.ScopeWorkspace:
		filters = []catalogFilter{{
			scope:         memcontract.ScopeWorkspace,
			workspaceRoot: workspaceRoot,
			workspaceID:   workspaceID,
		}}
	case memcontract.ScopeAgent:
		filters = []catalogFilter{{
			scope:         memcontract.ScopeAgent,
			workspaceRoot: workspaceRoot,
			workspaceID:   workspaceID,
		}}
	default:
		if strings.TrimSpace(workspaceID) != "" {
			filters = append(filters, catalogFilter{
				scope:         memcontract.ScopeWorkspace,
				workspaceRoot: workspaceRoot,
				workspaceID:   workspaceID,
			})
		}
	}

	for _, filter := range filters {
		if err := s.ensureCatalogFilterReady(ctx, filter); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureCatalogFilterReady(ctx context.Context, filter catalogFilter) error {
	if s.catalog == nil {
		return nil
	}

	identity := newCatalogIdentity(
		filter.scope,
		filter.workspaceID,
		s.catalogAgentName(filter.scope),
		s.catalogAgentTier(filter.scope),
	)
	ready, err := s.catalogFilterReady(ctx, filter, identity)
	if err != nil {
		return err
	}
	if ready {
		return nil
	}

	unlock := s.lockMutations()
	defer unlock()

	ready, err = s.catalogFilterReady(ctx, filter, identity)
	if err != nil {
		return err
	}
	if ready {
		return nil
	}

	_, err = s.reindexScopesLocked(ctx, filter.scope, filter.workspaceRoot, filter.workspaceID)
	return err
}

func (s *Store) catalogFilterReady(
	ctx context.Context,
	filter catalogFilter,
	identity catalogIdentity,
) (bool, error) {
	target := s.catalogSourceStore(filter.scope, filter.workspaceRoot)
	dirty, err := target.catalogSourceDirty(filter.scope)
	if err != nil {
		return false, err
	}
	if dirty {
		return false, nil
	}
	return s.catalog.identityReady(ctx, identity)
}

func (s *Store) reindexScopes(
	ctx context.Context,
	scope memcontract.Scope,
	workspaceRoot string,
	workspaceID string,
) (int, error) {
	if s.catalog == nil {
		return 0, nil
	}

	unlock := s.lockMutations()
	defer unlock()
	return s.reindexScopesLocked(ctx, scope, workspaceRoot, workspaceID)
}

func (s *Store) reindexScopesLocked(
	ctx context.Context,
	scope memcontract.Scope,
	workspaceRoot string,
	workspaceID string,
) (int, error) {
	if s.catalog == nil {
		return 0, nil
	}

	total := 0
	seenWorkspaceRoot := strings.TrimSpace(workspaceRoot)
	seenWorkspaceID := strings.TrimSpace(workspaceID)

	reindexScope := func(scope memcontract.Scope, workspaceRoot string, workspaceID string) error {
		headers, err := s.headersForCatalogScope(scope, workspaceRoot)
		if err != nil {
			return err
		}
		docs, err := s.documentsForHeaders(scope, workspaceRoot, workspaceID, headers)
		if err != nil {
			return err
		}
		if err := s.catalog.replaceScope(
			ctx,
			scope,
			workspaceID,
			s.catalogAgentName(scope),
			s.catalogAgentTier(scope),
			docs,
		); err != nil {
			return err
		}
		if err := s.catalogSourceStore(scope, workspaceRoot).clearCatalogSourceDirty(scope); err != nil {
			return err
		}
		total += len(docs)
		return nil
	}

	switch scope.Normalize() {
	case memcontract.ScopeGlobal:
		if err := reindexScope(memcontract.ScopeGlobal, "", ""); err != nil {
			return 0, err
		}
	case memcontract.ScopeWorkspace:
		if err := reindexScope(memcontract.ScopeWorkspace, seenWorkspaceRoot, seenWorkspaceID); err != nil {
			return 0, err
		}
	case memcontract.ScopeAgent:
		if err := reindexScope(memcontract.ScopeAgent, seenWorkspaceRoot, seenWorkspaceID); err != nil {
			return 0, err
		}
	default:
		if err := reindexScope(memcontract.ScopeGlobal, "", ""); err != nil {
			return 0, err
		}
		if seenWorkspaceRoot != "" {
			if err := reindexScope(memcontract.ScopeWorkspace, seenWorkspaceRoot, seenWorkspaceID); err != nil {
				return 0, err
			}
		}
	}

	if err := s.catalog.setLastReindex(ctx, time.Now().UTC()); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) headersForCatalogScope(
	scope memcontract.Scope,
	workspaceRoot string,
) ([]memcontract.Header, error) {
	target := s.catalogSourceStore(scope, workspaceRoot)
	return target.scan(scope, 0)
}

func (s *Store) documentsForHeaders(
	scope memcontract.Scope,
	workspaceRoot string,
	workspaceID string,
	headers []memcontract.Header,
) ([]catalogDocument, error) {
	target := s.catalogSourceStore(scope, workspaceRoot)

	docs := make([]catalogDocument, 0, len(headers))
	for _, header := range headers {
		rawContent, err := target.Read(scope, header.Filename)
		if err != nil {
			return nil, err
		}
		doc, err := buildCatalogDocument(scope, workspaceID, header, rawContent)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
