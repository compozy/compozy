package modelcatalog

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func applyCurationMetadata(model *Model, rows []ModelRow) {
	for _, row := range rows {
		model.ExplicitlyCurated = model.ExplicitlyCurated || row.ExplicitlyCurated
	}
	model.Deprecated = firstCurationFlag(rows, func(row ModelRow) *bool { return row.Deprecated })
	model.Hidden = firstCurationFlag(rows, func(row ModelRow) *bool { return row.Hidden })
	model.Featured = firstCurationFlag(rows, func(row ModelRow) *bool { return row.Featured })
	model.Curated = model.ExplicitlyCurated
}

func firstCurationFlag(rows []ModelRow, field func(ModelRow) *bool) bool {
	for _, row := range rows {
		if value := field(row); value != nil {
			return *value
		}
	}
	return false
}

func applyCatalogView(models []Model, view CatalogView) ([]Model, error) {
	normalized, err := normalizeCatalogView(view)
	if err != nil {
		return nil, err
	}
	providerHasExplicitCuratedSet := make(map[string]bool)
	for index := range models {
		model := &models[index]
		providerHasExplicitCuratedSet[model.ProviderID] =
			providerHasExplicitCuratedSet[model.ProviderID] || model.ExplicitlyCurated
	}
	for index := range models {
		model := &models[index]
		if model.Hidden || model.Deprecated {
			model.Curated = false
			continue
		}
		if providerHasExplicitCuratedSet[model.ProviderID] {
			model.Curated = model.ExplicitlyCurated || model.Featured
			continue
		}
		model.Curated = true
	}
	sortCatalogModels(models)
	if normalized == CatalogViewAll {
		return models, nil
	}
	filtered := make([]Model, 0, len(models))
	for _, model := range models {
		if model.Curated {
			filtered = append(filtered, model)
		}
	}
	return filtered, nil
}

func normalizeCatalogView(view CatalogView) (CatalogView, error) {
	switch CatalogView(strings.TrimSpace(string(view))) {
	case "", CatalogViewCurated:
		return CatalogViewCurated, nil
	case CatalogViewAll:
		return CatalogViewAll, nil
	default:
		return "", &InvalidViewError{View: string(view)}
	}
}

func sortCatalogModels(models []Model) {
	sort.SliceStable(models, func(i, j int) bool {
		left := models[i]
		right := models[j]
		if left.ProviderID != right.ProviderID {
			return left.ProviderID < right.ProviderID
		}
		if left.Curated != right.Curated {
			return left.Curated
		}
		if left.Deprecated != right.Deprecated {
			return !left.Deprecated
		}
		if left.Featured != right.Featured {
			return left.Featured
		}
		leftDate := releaseDateValue(left.ReleaseDate)
		rightDate := releaseDateValue(right.ReleaseDate)
		if leftDate != rightDate {
			return leftDate > rightDate
		}
		return left.ModelID < right.ModelID
	})
}

func releaseDateValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// NormalizeReleaseDate validates and normalizes YYYY-MM or YYYY-MM-DD metadata.
func NormalizeReleaseDate(value string) (*string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	layout := "2006-01-02"
	if len(trimmed) == len("2006-01") {
		layout = "2006-01"
	}
	if _, err := time.Parse(layout, trimmed); err != nil {
		return nil, fmt.Errorf("model catalog release_date %q must be YYYY-MM or YYYY-MM-DD: %w", trimmed, err)
	}
	return &trimmed, nil
}
