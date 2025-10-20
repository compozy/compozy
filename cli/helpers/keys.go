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
			lastUsedI := keys[i].GetLastUsed()
			lastUsedJ := keys[j].GetLastUsed()
			if lastUsedI == "" {
				return false
			}
			if lastUsedJ == "" {
				return true
			}
			timeI, errI := time.Parse(time.RFC3339, lastUsedI)
			timeJ, errJ := time.Parse(time.RFC3339, lastUsedJ)
			if errI != nil || errJ != nil {
				return lastUsedI > lastUsedJ
			}
			return timeI.After(timeJ)
		})
	case "created", "":
		sort.Slice(keys, func(i, j int) bool {
			timeI, errI := time.Parse(time.RFC3339, keys[i].GetCreatedAt())
			timeJ, errJ := time.Parse(time.RFC3339, keys[j].GetCreatedAt())
			if errI != nil || errJ != nil {
				return keys[i].GetCreatedAt() > keys[j].GetCreatedAt()
			}
			return timeI.After(timeJ)
		})
	}
}
