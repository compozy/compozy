package service

import (
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// Pagination constants for memory service operations
const (
	// MinPageLimit is the minimum allowed items per page
	MinPageLimit = 1
	// MaxPageLimit is the maximum allowed items per page for service operations
	MaxPageLimit = 1000
)

// ValidatePaginationParams validates pagination parameters
func ValidatePaginationParams(offset, limit int) error {
	if offset < 0 {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"offset cannot be negative",
			nil,
		).WithContext("offset", offset)
	}
	if limit < MinPageLimit || limit > MaxPageLimit {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"limit must be between 1 and 1000",
			nil,
		).WithContext("limit", limit)
	}
	return nil
}
