package helpers

import (
	"sort"
	"time"
)

// KeySortable represents the interface that key objects must implement for sorting
type KeySortable interface {
	GetName() string
	GetPrefix() string
	GetCreatedAt() string
	GetLastUsed() string
}

// SortKeys sorts a slice of KeySortable objects based on the specified field using efficient sort.Slice
func SortKeys[T KeySortable](keys []T, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].GetPrefix() < keys[j].GetPrefix()
		})
	case "last_used":
		sort.Slice(keys, func(i, j int) bool {
			// Handle empty LastUsed values - empty values go to the end
			lastUsedI := keys[i].GetLastUsed()
			lastUsedJ := keys[j].GetLastUsed()
			if lastUsedI == "" {
				return false
			}
			if lastUsedJ == "" {
				return true
			}
			// Parse timestamps for proper chronological comparison
			timeI, errI := time.Parse(time.RFC3339, lastUsedI)
			timeJ, errJ := time.Parse(time.RFC3339, lastUsedJ)
			// Fallback to string comparison if parsing fails
			if errI != nil || errJ != nil {
				return lastUsedI > lastUsedJ
			}
			return timeI.After(timeJ)
		})
	case "created", "":
		sort.Slice(keys, func(i, j int) bool {
			// Parse timestamps for proper chronological comparison
			timeI, errI := time.Parse(time.RFC3339, keys[i].GetCreatedAt())
			timeJ, errJ := time.Parse(time.RFC3339, keys[j].GetCreatedAt())
			// Fallback to string comparison if parsing fails
			if errI != nil || errJ != nil {
				return keys[i].GetCreatedAt() > keys[j].GetCreatedAt()
			}
			return timeI.After(timeJ)
		})
	}
}
