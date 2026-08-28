package modelcatalog

import (
	"errors"
	"fmt"
	"maps"
	"strings"
)

func (s *CatalogService) requestExecutionContext(
	requested CatalogExecutionContext,
) CatalogExecutionContext {
	if requested.Scope != "" || s == nil {
		return requested
	}
	s.executionContextMu.RLock()
	defer s.executionContextMu.RUnlock()
	return s.defaultExecutionContext
}

// SetDefaultExecutionContext defines the profile or workspace scope used when
// an internal caller omits an explicit catalog execution context.
func (s *CatalogService) SetDefaultExecutionContext(value CatalogExecutionContext) error {
	if s == nil {
		return errors.New("model catalog: service is required")
	}
	normalized, err := NormalizeCatalogExecutionScope(value)
	if err != nil {
		return err
	}
	if normalized.CommandFingerprint != "" {
		return errors.New("model catalog: default execution context cannot include command_fingerprint")
	}
	s.executionContextMu.Lock()
	s.defaultExecutionContext = normalized
	s.executionContextMu.Unlock()
	return nil
}

type sourceExecutionFingerprinter interface {
	CatalogExecutionFingerprint() (string, error)
}

func resolveSourceExecutionContexts(
	sources []Source,
	requested CatalogExecutionContext,
) (map[string]CatalogExecutionContext, error) {
	if executionContextIsEmpty(requested) && sourcesUseGlobalExecutionContext(sources) {
		requested = GlobalCatalogExecutionContext()
	}
	requested, err := NormalizeCatalogExecutionScope(requested)
	if err != nil {
		return nil, err
	}
	contexts := make(map[string]CatalogExecutionContext, len(sources))
	for _, source := range sources {
		resolved, resolveErr := resolveSourceExecutionContext(source, requested)
		if resolveErr != nil {
			return nil, resolveErr
		}
		contexts[source.ID()] = resolved
	}
	return contexts, nil
}

func executionContextIsEmpty(value CatalogExecutionContext) bool {
	return strings.TrimSpace(string(value.Scope)) == "" &&
		strings.TrimSpace(value.ProfileID) == "" &&
		strings.TrimSpace(value.WorkspaceID) == "" &&
		strings.TrimSpace(value.CommandFingerprint) == ""
}

func sourcesUseGlobalExecutionContext(sources []Source) bool {
	for _, source := range sources {
		if !sourceUsesGlobalExecutionContext(source) {
			return false
		}
	}
	return true
}

func sourceUsesGlobalExecutionContext(source Source) bool {
	if source == nil {
		return false
	}
	switch source.Kind() {
	case SourceKindBuiltin, SourceKindConfig, SourceKindModelsDev:
		return true
	default:
		return false
	}
}

func resolveReadSourceExecutionContexts(
	sources []Source,
	requested CatalogExecutionContext,
) (map[string]CatalogExecutionContext, error) {
	contexts := map[string]CatalogExecutionContext{
		SourceIDBuiltin:   GlobalCatalogExecutionContext(),
		SourceIDConfig:    GlobalCatalogExecutionContext(),
		SourceIDModelsDev: GlobalCatalogExecutionContext(),
	}
	resolved, err := resolveSourceExecutionContexts(sources, requested)
	if err != nil {
		return nil, err
	}
	maps.Copy(contexts, resolved)
	return contexts, nil
}

func resolveSourceExecutionContext(
	source Source,
	requested CatalogExecutionContext,
) (CatalogExecutionContext, error) {
	if source == nil {
		return CatalogExecutionContext{}, errors.New("model catalog: execution context source is required")
	}
	if sourceUsesGlobalExecutionContext(source) {
		return GlobalCatalogExecutionContext(), nil
	}
	switch source.Kind() {
	case SourceKindProviderLive, SourceKindExtension, SourceKindACPSession:
		if requested.Scope == ExecutionScopeGlobal {
			return CatalogExecutionContext{}, fmt.Errorf(
				"model catalog: source %q requires a profile or workspace execution scope",
				source.ID(),
			)
		}
		fingerprinter, ok := source.(sourceExecutionFingerprinter)
		if !ok {
			return CatalogExecutionContext{}, fmt.Errorf(
				"model catalog: scoped source %q does not provide an execution fingerprint",
				source.ID(),
			)
		}
		fingerprint, err := fingerprinter.CatalogExecutionFingerprint()
		if err != nil {
			return CatalogExecutionContext{}, fmt.Errorf(
				"model catalog: resolve execution fingerprint for %q: %w",
				source.ID(),
				err,
			)
		}
		return requested.WithCommandFingerprint(fingerprint)
	default:
		return CatalogExecutionContext{}, fmt.Errorf(
			"model catalog: source %q has unsupported kind %q",
			source.ID(),
			source.Kind(),
		)
	}
}

func sourceExecutionContext(
	contexts map[string]CatalogExecutionContext,
	sourceID string,
) (CatalogExecutionContext, error) {
	contextValue, ok := contexts[strings.TrimSpace(sourceID)]
	if !ok {
		return CatalogExecutionContext{}, fmt.Errorf(
			"model catalog: execution context for source %q is unavailable",
			sourceID,
		)
	}
	return NormalizePersistedExecutionContext(contextValue)
}

func cloneSourceExecutionContexts(
	source map[string]CatalogExecutionContext,
) map[string]CatalogExecutionContext {
	if len(source) == 0 {
		return nil
	}
	return maps.Clone(source)
}

func singleSourceExecutionContext(
	sourceID string,
	contextValue CatalogExecutionContext,
) map[string]CatalogExecutionContext {
	return map[string]CatalogExecutionContext{strings.TrimSpace(sourceID): contextValue}
}
