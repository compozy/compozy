package marketplace

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var entryIDPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)

const (
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

func (c entryCommon) validate(kind Kind) error {
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
			return fmt.Errorf("marketplace catalog %q entry %s is required", kind, field.name)
		}
	}
	entryID := strings.TrimSpace(c.EntryID)
	if entryID == "." || entryID == ".." || !entryIDPattern.MatchString(entryID) {
		return fmt.Errorf("marketplace catalog %q entry_id must be one URL-safe path segment", kind)
	}
	if kind == KindSkill && strings.HasPrefix(entryID, RemoteSkillEntryPrefix) {
		return fmt.Errorf(
			"marketplace catalog %q entry_id must not use reserved prefix %q",
			kind,
			RemoteSkillEntryPrefix,
		)
	}
	if _, err := parseOptionalTimestamp(c.PublishedAt, "published_at"); err != nil {
		return err
	}
	if _, err := parseOptionalTimestamp(c.UpdatedAt, "updated_at"); err != nil {
		return err
	}
	return nil
}

func commonEntry(kind Kind, common entryCommon, payload any) (Entry, error) {
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
		return Entry{}, fmt.Errorf("marketplace catalog %q encode entry %q: %w", kind, common.EntryID, err)
	}
	return Entry{
		Kind:        kind,
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
