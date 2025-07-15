package auth

import (
	"strings"
	"time"
)

// KeyInfo represents API key information
type KeyInfo struct {
	ID        string  `json:"id"`
	Prefix    string  `json:"prefix"`
	CreatedAt string  `json:"created_at"`
	LastUsed  *string `json:"last_used,omitempty"`
}

// GetPrefix returns the key prefix for sorting
func (k KeyInfo) GetPrefix() string {
	return k.Prefix
}

// GetCreatedAt returns the creation timestamp for sorting
func (k KeyInfo) GetCreatedAt() string {
	return k.CreatedAt
}

// GetLastUsed returns the last used timestamp for sorting
func (k KeyInfo) GetLastUsed() *string {
	return k.LastUsed
}

// GetSortKey returns the sort key for the specified field
func (k KeyInfo) GetSortKey(field string) any {
	switch field {
	case "name":
		return k.Prefix
	case "created":
		if t, err := time.Parse(time.RFC3339, k.CreatedAt); err == nil {
			return t
		}
		return k.CreatedAt
	case "last_used":
		if k.LastUsed == nil {
			return time.Time{}
		}
		if t, err := time.Parse(time.RFC3339, *k.LastUsed); err == nil {
			return t
		}
		return *k.LastUsed
	default:
		return k.Prefix
	}
}

// GetDisplayFields returns the fields that can be displayed
func (k KeyInfo) GetDisplayFields() []string {
	return []string{"Prefix", "Created", "Last Used", "Usage Count"}
}

// GetDisplayValue returns the display value for the specified field
func (k KeyInfo) GetDisplayValue(field string) string {
	switch field {
	case "Prefix":
		return k.Prefix
	case "Created":
		if t, err := time.Parse(time.RFC3339, k.CreatedAt); err == nil {
			return t.Format("2006-01-02 15:04")
		}
		return k.CreatedAt
	case "Last Used":
		if k.LastUsed == nil {
			return "Never"
		}
		if t, err := time.Parse(time.RFC3339, *k.LastUsed); err == nil {
			return t.Format("2006-01-02 15:04")
		}
		return *k.LastUsed
	case "Usage Count":
		return "N/A" // TODO: Add usage count when available from API
	default:
		return ""
	}
}

// MatchesSearch returns true if the key matches the search term
func (k KeyInfo) MatchesSearch(term string) bool {
	lowerTerm := strings.ToLower(term)
	return strings.Contains(strings.ToLower(k.Prefix), lowerTerm) ||
		strings.Contains(strings.ToLower(k.ID), lowerTerm)
}

// UserInfo represents user information
type UserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
