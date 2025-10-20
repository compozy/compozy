package uc

// Pagination constants for memory operations
const (
	// DefaultPageLimit is the default number of items per page for general operations
	DefaultPageLimit = 50
	// MaxPageLimit is the maximum allowed items per page for general operations
	MaxPageLimit = 1000

	// DefaultStatsPageLimit is the default number of items per page for stats operations
	DefaultStatsPageLimit = 100
	// MaxStatsPageLimit is the maximum allowed items per page for stats operations
	MaxStatsPageLimit = 10000
)

// PaginationLimits defines the limits for pagination
type PaginationLimits struct {
	DefaultLimit int
	MaxLimit     int
}

var (
	// DefaultPaginationLimits for general operations
	DefaultPaginationLimits = PaginationLimits{
		DefaultLimit: DefaultPageLimit,
		MaxLimit:     MaxPageLimit,
	}

	// StatsPaginationLimits for stats operations that may need more data
	StatsPaginationLimits = PaginationLimits{
		DefaultLimit: DefaultStatsPageLimit,
		MaxLimit:     MaxStatsPageLimit,
	}
)

// NormalizePagination applies defaults and limits to pagination parameters
func NormalizePagination(offset, limit int, limits PaginationLimits) (normalizedOffset, normalizedLimit int) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = limits.DefaultLimit
	}
	if limit > limits.MaxLimit {
		limit = limits.MaxLimit
	}
	return offset, limit
}
