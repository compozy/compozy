package daemon

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
)

func (r *modelCatalogRuntime) resolveCatalogExecutionContext(
	requested modelcatalog.CatalogExecutionContext,
) (modelcatalog.CatalogExecutionContext, error) {
	if requested.Scope == "" {
		requested.ProfileID = strings.TrimSpace(requested.ProfileID)
		if requested.ProfileID == "" {
			requested.ProfileID = store.DefaultProfileID
		}
		requested.WorkspaceID = strings.TrimSpace(requested.WorkspaceID)
		if requested.WorkspaceID == "" {
			requested.Scope = modelcatalog.ExecutionScopeProfile
		} else {
			requested.Scope = modelcatalog.ExecutionScopeWorkspace
		}
	}
	if strings.TrimSpace(requested.CommandFingerprint) != "" {
		return modelcatalog.CatalogExecutionContext{}, errors.New(
			"daemon: model catalog request execution context cannot set command fingerprint",
		)
	}
	normalized, err := modelcatalog.NormalizeCatalogExecutionScope(requested)
	if err != nil {
		return modelcatalog.CatalogExecutionContext{}, fmt.Errorf(
			"daemon: invalid model catalog execution context: %w",
			err,
		)
	}
	r.rememberCatalogExecutionContext(normalized)
	return normalized, nil
}

func (r *modelCatalogRuntime) rememberCatalogExecutionContext(
	executionContext modelcatalog.CatalogExecutionContext,
) {
	if r == nil {
		return
	}
	key := catalogExecutionScopeKey(executionContext)
	r.executionContextMu.Lock()
	defer r.executionContextMu.Unlock()
	if r.executionContexts == nil {
		r.executionContexts = make(map[string]modelcatalog.CatalogExecutionContext)
	}
	r.executionContexts[key] = executionContext
}

func (r *modelCatalogRuntime) catalogExecutionContexts() []modelcatalog.CatalogExecutionContext {
	if r == nil {
		return nil
	}
	r.executionContextMu.RLock()
	defer r.executionContextMu.RUnlock()
	keys := make([]string, 0, len(r.executionContexts))
	for key := range r.executionContexts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	contexts := make([]modelcatalog.CatalogExecutionContext, 0, len(keys))
	for _, key := range keys {
		contexts = append(contexts, r.executionContexts[key])
	}
	return contexts
}

func catalogExecutionScopeKey(executionContext modelcatalog.CatalogExecutionContext) string {
	return strings.Join([]string{
		string(executionContext.Scope),
		executionContext.ProfileID,
		executionContext.WorkspaceID,
	}, "\x00")
}
