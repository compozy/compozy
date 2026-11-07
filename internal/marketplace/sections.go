package marketplace

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

type BrowseFilter struct {
	Query     string
	Category  CategoryID
	Installed []Origin
}
type CategoryCount struct {
	Category Category
	Count    int
}
type Section struct {
	Category Category
	Total    int
	Entries  []Entry
}
type SectionsResult struct {
	Sections   []Section
	Revision   string
	Sources    []SourceState
	Refreshing bool
	Stale      bool
	ErrorClass string
	LastError  string
}

// FilterBrowse applies collapse and facets before slicing the complete enabled-source snapshot.
func FilterBrowse(page BrowseResult, filter BrowseFilter, offset, limit int) (BrowseResult, error) {
	if filter.Category != "" && filter.Category != CategoryFeatured {
		if _, err := ParseCategoryID(string(filter.Category)); err != nil {
			return BrowseResult{}, err
		}
	}
	rows, err := visibleCategoryCandidates(page.Entries, filter.Installed)
	if err != nil {
		return BrowseResult{}, err
	}
	needle := foldMarketplaceText(NormalizeQuery(filter.Query))
	counts := make(map[CategoryID]int)
	matches := make([]categoryCandidate, 0, len(rows))
	for _, row := range rows {
		if needle != "" && !strings.Contains(foldMarketplaceText(row.entry.Name), needle) &&
			!strings.Contains(foldMarketplaceText(row.entry.Description), needle) {
			continue
		}
		counts[row.details.Category.ID]++
		if row.details.Featured {
			counts[CategoryFeatured]++
		}
		if filter.Category == "" || row.details.Category.ID == filter.Category ||
			(filter.Category == CategoryFeatured && row.details.Featured) {
			matches = append(matches, row)
		}
	}
	// Preserve the historical source order on unfiltered Browse; category views sort across sources.
	if filter.Category != "" {
		sortCategoryCandidates(matches)
	}
	page.Categories = []CategoryCount{
		{Category: Category{CategoryFeatured, "Featured"}, Count: counts[CategoryFeatured]},
	}
	for _, category := range Categories() {
		page.Categories = append(page.Categories, CategoryCount{category, counts[category.ID]})
	}
	page.Total = len(matches)
	page.Entries = []Entry{}
	pageLimit := normalizeListLimit(limit)
	if limit == maxCatalogEntriesPerSource {
		pageLimit = len(matches)
	}
	for _, row := range matches[min(offset, len(matches)):min(offset+pageLimit, len(matches))] {
		page.Entries = append(page.Entries, row.entry)
	}
	return page, nil
}

func (s *CatalogService) Sections(ctx context.Context, query string, perSection int) (SectionsResult, error) {
	if perSection == 0 {
		perSection = 6
	}
	if perSection < 1 || perSection > 12 {
		return SectionsResult{}, fmt.Errorf("per_section must be between 1 and 12")
	}
	page, err := s.BrowseFiltered(ctx, BrowseFilter{Query: query}, 0, maxCatalogEntriesPerSource)
	if err != nil {
		return SectionsResult{}, err
	}
	return BuildSections(page, perSection)
}

func BuildSections(page BrowseResult, perSection int) (SectionsResult, error) {
	result := SectionsResult{
		Sections:   []Section{},
		Revision:   page.Revision,
		Sources:    page.Sources,
		Refreshing: page.Refreshing,
		Stale:      page.Stale,
		ErrorClass: page.ErrorClass,
		LastError:  page.LastError,
	}
	for _, count := range page.Categories {
		if count.Count == 0 {
			continue
		}
		section, err := FilterBrowse(page, BrowseFilter{Category: count.Category.ID}, 0, perSection)
		if err != nil {
			return SectionsResult{}, err
		}
		result.Sections = append(result.Sections, Section{count.Category, section.Total, section.Entries})
	}
	return result, nil
}

type categoryCandidate struct {
	entry   Entry
	details EntryDetails
}

func visibleCategoryCandidates(entries []Entry, origins []Origin) ([]categoryCandidate, error) {
	rows := make([]categoryCandidate, 0, len(entries))
	byKey := make(map[string]int, len(entries))
	installed := make(map[Origin]bool, len(origins))
	for _, origin := range origins {
		installed[origin] = true
	}
	for _, entry := range entries {
		details, err := ProjectEntry(entry)
		if err != nil {
			return nil, err
		}
		byKey[entry.SourceName+"/"+entry.EntryID] = len(rows)
		rows = append(rows, categoryCandidate{entry, details})
	}
	hidden := make(map[int]bool)
	for index, row := range rows {
		for _, key := range row.details.Supersedes {
			target, found := byKey[key]
			if !found || target == index {
				continue
			}
			other := rows[target]
			if installed[Origin{SourceRef: other.details.SourceRef, EntryID: other.entry.EntryID}] {
				if !installed[Origin{SourceRef: row.details.SourceRef, EntryID: row.entry.EntryID}] {
					hidden[index] = true
				}
			} else {
				hidden[target] = true
			}
		}
	}

	visible := make([]categoryCandidate, 0, len(rows))
	for index, row := range rows {
		if !hidden[index] {
			visible = append(visible, row)
		}
	}
	return visible, nil
}

func sortCategoryCandidates(matches []categoryCandidate) {
	slices.SortStableFunc(matches, func(a, b categoryCandidate) int {
		if a.details.Featured != b.details.Featured {
			if a.details.Featured {
				return -1
			}
			return 1
		}
		return cmp.Or(
			cmp.Compare(foldMarketplaceText(a.entry.Name), foldMarketplaceText(b.entry.Name)),
			cmp.Compare(a.entry.EntryID, b.entry.EntryID),
			cmp.Compare(a.entry.SourceName, b.entry.SourceName),
		)
	})
}
