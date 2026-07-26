package loop

import (
	"slices"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

func catalogSpecFromDefinition(def dsl.Definition) CatalogResourceSpec {
	keywords := append([]string(nil), def.Meta.Catalog.Keywords...)
	return normalizeCatalogSpec(CatalogResourceSpec{
		UseWhen:  def.Meta.Catalog.UseWhen,
		Keywords: keywords,
		Category: def.Meta.Catalog.Category,
	})
}

func normalizeCatalogSpec(spec CatalogResourceSpec) CatalogResourceSpec {
	spec.UseWhen = strings.TrimSpace(spec.UseWhen)
	spec.Category = strings.TrimSpace(spec.Category)
	keywords := make([]string, 0, len(spec.Keywords))
	seen := map[string]struct{}{}
	for _, keyword := range spec.Keywords {
		normalized := strings.TrimSpace(keyword)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		keywords = append(keywords, normalized)
	}
	slices.Sort(keywords)
	spec.Keywords = keywords
	return spec
}

func startSpecsFromDefinition(def dsl.Definition) []StartResourceSpec {
	specs := make([]StartResourceSpec, 0, len(def.Start))
	for _, binding := range def.Start {
		specs = append(specs, StartResourceSpec{
			Kind:             string(binding.Kind),
			InputKeys:        sortedMapKeys(binding.Inputs),
			InputMappingKeys: sortedMapKeys(binding.InputMapping),
		})
	}
	return normalizeStartSpecs(specs)
}

func normalizeStartSpecs(specs []StartResourceSpec) []StartResourceSpec {
	normalized := make([]StartResourceSpec, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		spec.Kind = strings.TrimSpace(spec.Kind)
		spec.InputKeys = normalizeStringSlice(spec.InputKeys)
		spec.InputMappingKeys = normalizeStringSlice(spec.InputMappingKeys)
		if spec.Kind == "" {
			continue
		}
		key := strings.Join(
			[]string{spec.Kind, strings.Join(spec.InputKeys, ","), strings.Join(spec.InputMappingKeys, ",")},
			"\x00",
		)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, spec)
	}
	slices.SortFunc(normalized, func(a, b StartResourceSpec) int {
		if cmp := strings.Compare(a.Kind, b.Kind); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(strings.Join(a.InputKeys, ","), strings.Join(b.InputKeys, ",")); cmp != 0 {
			return cmp
		}
		return strings.Compare(strings.Join(a.InputMappingKeys, ","), strings.Join(b.InputMappingKeys, ","))
	})
	return normalized
}

func cloneStartSpecs(specs []StartResourceSpec) []StartResourceSpec {
	cloned := make([]StartResourceSpec, len(specs))
	for idx := range specs {
		cloned[idx] = specs[idx]
		cloned[idx].InputKeys = append([]string(nil), specs[idx].InputKeys...)
		cloned[idx].InputMappingKeys = append([]string(nil), specs[idx].InputMappingKeys...)
	}
	return cloned
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	slices.Sort(out)
	return out
}

func sortedMapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return normalizeStringSlice(keys)
}
