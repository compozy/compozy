package marketplace

import (
	"errors"
	"fmt"
	"strings"
)

const (
	CompozyCatalogSource = "compozy-catalog"
	CompozyCatalogRef    = "catalog:compozy"
)

// Origin is acquisition identity; source display names do not participate in equality.
type Origin struct {
	SourceRef string `json:"source_ref"`
	EntryID   string `json:"entry_id"`
}

var (
	ErrSourceNotFound     = errors.New("marketplace_source_not_found")
	ErrInstallSlugInvalid = errors.New("marketplace_install_slug_invalid")
)

func (o Origin) InstallSlug(sourceName string) string {
	if o.SourceRef == CompozyCatalogRef {
		sourceName = "compozy"
	}
	return sourceName + "/" + o.EntryID
}

// ParseInstallSlug uses the source index; authored curated acquisition refs have a separate resolver.
func ParseInstallSlug(slug string, sources []SourceState) (Origin, error) {
	name, entryID, ok := strings.Cut(strings.TrimSpace(slug), "/")
	if !ok || name == "" || entryID == "." || entryID == ".." || !entryIDPattern.MatchString(entryID) {
		return Origin{}, ErrInstallSlugInvalid
	}
	for _, source := range sources {
		slugName := source.Source
		if source.Source == CompozyCatalogSource {
			slugName = "compozy"
		}
		if strings.EqualFold(name, slugName) && source.SourceRef != "" {
			return Origin{SourceRef: source.SourceRef, EntryID: entryID}, nil
		}
	}
	return Origin{}, fmt.Errorf("%w: %s", ErrSourceNotFound, name)
}
