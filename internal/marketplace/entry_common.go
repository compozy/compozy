package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var entryIDPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)

const (
	protocolFile  = "file"
	protocolHTTP  = "http"
	protocolHTTPS = "https"
)

type entryCommon struct {
	EntryID     string `json:"entry_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func (c entryCommon) validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "entry_id", value: c.EntryID},
		{name: "name", value: c.Name},
		{name: "description", value: c.Description},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("marketplace catalog entry %s is required", field.name)
		}
	}
	entryID := strings.TrimSpace(c.EntryID)
	if entryID == "." || entryID == ".." || !entryIDPattern.MatchString(entryID) {
		return errors.New("marketplace catalog entry_id must be one URL-safe path segment")
	}

	if _, err := parseOptionalTimestamp(c.PublishedAt, "published_at"); err != nil {
		return err
	}
	if _, err := parseOptionalTimestamp(c.UpdatedAt, "updated_at"); err != nil {
		return err
	}
	return nil
}

func commonEntry(common entryCommon, payload any) (Entry, error) {
	publishedAt, err := parseOptionalTimestamp(common.PublishedAt, "published_at")
	if err != nil {
		return Entry{}, err
	}
	updatedAt, err := parseOptionalTimestamp(common.UpdatedAt, "updated_at")
	if err != nil {
		return Entry{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Entry{}, fmt.Errorf("marketplace catalog encode entry %q: %w", common.EntryID, err)
	}
	return Entry{
		EntryID:     strings.TrimSpace(common.EntryID),
		Name:        strings.TrimSpace(common.Name),
		Description: strings.TrimSpace(common.Description),
		Version:     strings.TrimSpace(common.Version),
		PublishedAt: publishedAt,
		UpdatedAt:   updatedAt,
		Payload:     raw,
	}, nil
}

func parseOptionalTimestamp(raw string, field string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog %s must be RFC3339: %w", field, err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
