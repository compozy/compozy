package marketplace

import (
	"fmt"
	"strings"
)

type skillEntry struct {
	entryCommon
	InstallSlug string   `json:"install_slug"`
	DisplayName string   `json:"display_name,omitempty"`
	Author      string   `json:"author,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func decodeSkillEntry(raw []byte) (Entry, error) {
	var value skillEntry
	if err := decodeStrict(raw, &value); err != nil {
		return Entry{}, err
	}
	if err := value.validate(KindSkill); err != nil {
		return Entry{}, err
	}
	if strings.TrimSpace(value.InstallSlug) == "" {
		return Entry{}, fmt.Errorf("marketplace catalog skill entry %q install_slug is required", value.EntryID)
	}
	entry, err := commonEntry(KindSkill, value.entryCommon, value)
	if err != nil {
		return Entry{}, err
	}
	entry.InstallSlug = strings.TrimSpace(value.InstallSlug)
	return entry, nil
}
