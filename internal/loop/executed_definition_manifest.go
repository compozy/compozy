package loop

import (
	"fmt"
	"slices"
)

func resolvedTemplateSources(resolved *ResolvedDefinition) map[string]string {
	sources := map[string]string{}
	if resolved == nil {
		return sources
	}
	for key, template := range resolved.Templates {
		if template != nil {
			sources[key] = template.Raw
		}
	}
	return sources
}

func resolvedConditionSources(resolved *ResolvedDefinition) map[string]string {
	sources := map[string]string{}
	if resolved == nil {
		return sources
	}
	for key, condition := range resolved.Conditions {
		if condition != nil {
			sources[key] = condition.Raw
		}
	}
	return sources
}

func validateExecutedManifestRoundTrip(
	label string,
	snapshot map[string]string,
	hydrated map[string]string,
) error {
	keySet := make(map[string]struct{}, len(snapshot)+len(hydrated))
	for key := range snapshot {
		keySet[key] = struct{}{}
	}
	for key := range hydrated {
		keySet[key] = struct{}{}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		snapshotSource, inSnapshot := snapshot[key]
		hydratedSource, inHydrated := hydrated[key]
		switch {
		case !inSnapshot:
			return fmt.Errorf(
				"%w: executed definition %s manifest key %q added during hydration with source %q",
				ErrValidation,
				label,
				key,
				hydratedSource,
			)
		case !inHydrated:
			return fmt.Errorf(
				"%w: executed definition %s manifest key %q missing after hydration; snapshot source %q",
				ErrValidation,
				label,
				key,
				snapshotSource,
			)
		case snapshotSource != hydratedSource:
			return fmt.Errorf(
				"%w: executed definition %s manifest key %q source changed during hydration from %q to %q",
				ErrValidation,
				label,
				key,
				snapshotSource,
				hydratedSource,
			)
		}
	}
	return nil
}
