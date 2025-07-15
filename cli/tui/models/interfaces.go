package models

import "context"

// Sortable represents any item that can be sorted
type Sortable interface {
	GetSortKey(field string) any
}

// Listable represents any item that can be displayed in a list
type Listable interface {
	GetDisplayFields() []string
	GetDisplayValue(field string) string
}

// Searchable represents any item that can be searched
type Searchable interface {
	MatchesSearch(term string) bool
}

// DataClient represents a generic client for data operations
type DataClient[T any] interface {
	List(ctx context.Context) ([]T, error)
}

// ListableItem combines all the interfaces needed for list display
type ListableItem interface {
	Sortable
	Listable
	Searchable
}
