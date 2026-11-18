package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactListing struct {
	Trust            *compactPlatformTrust              `json:"trust,omitzero"`
	Category         string                             `json:"category"`
	Featured         bool                               `json:"featured"`
	SourceRef        string                             `json:"source_ref,omitempty"`
	Layout           string                             `json:"layout,omitempty"`
	Installable      bool                               `json:"installable"`
	InstallBlocker   string                             `json:"install_blocker,omitempty"`
	DigestSHA256     string                             `json:"digest_sha256"`
	NameConflict     *contract.MarketplaceOriginPayload `json:"name_conflict,omitzero"`
	EntryID          string                             `json:"entry_id"`
	Name             string                             `json:"name"`
	Description      string                             `json:"description"`
	Version          string                             `json:"version,omitempty"`
	Author           string                             `json:"author,omitempty"`
	InstallSlug      string                             `json:"install_slug"`
	Source           string                             `json:"source"`
	Tier             string                             `json:"tier,omitempty"`
	Format           string                             `json:"format,omitempty"`
	Installed        bool                               `json:"installed"`
	InstalledName    string                             `json:"installed_name,omitempty"`
	InstalledVersion string                             `json:"installed_version,omitempty"`
	UpdateAvailable  bool                               `json:"update_available"`
}

func projectCompactListing(p contract.MarketplaceListingPayload) compactListing {
	return compactListing{
		Trust:            compactTrust(p.Trust),
		Category:         p.Category,
		Featured:         p.Featured,
		SourceRef:        p.SourceRef,
		Layout:           p.Layout,
		Installable:      p.Installable,
		InstallBlocker:   p.InstallBlocker,
		DigestSHA256:     p.DigestSHA256,
		NameConflict:     p.NameConflict,
		EntryID:          p.EntryID,
		Name:             p.Name,
		Description:      p.Description,
		Version:          p.Version,
		Author:           p.Author,
		InstallSlug:      p.InstallSlug,
		Source:           p.Source,
		Tier:             p.Tier,
		Format:           p.Format,
		Installed:        p.Installed,
		InstalledName:    p.InstalledName,
		InstalledVersion: p.InstalledVersion,
		UpdateAvailable:  p.UpdateAvailable,
	}
}

type compactSource struct {
	Name         string                      `json:"name"`
	Kind         string                      `json:"kind"`
	Source       string                      `json:"source"`
	Enabled      bool                        `json:"enabled"`
	State        string                      `json:"state"`
	Plugins      int                         `json:"plugins"`
	Installable  int                         `json:"installable"`
	LastReadAt   *time.Time                  `json:"last_read_at,omitempty"`
	ErrorClass   string                      `json:"error_class,omitempty"`
	Error        string                      `json:"error,omitempty"`
	DocumentPath string                      `json:"document_path,omitempty"`
	Owner        string                      `json:"owner,omitempty"`
	Stability    string                      `json:"stability"`
	Diagnostics  []compactPlatformDiagnostic `json:"diagnostics"`
}

func projectCompactSource(p contract.MarketplaceSourcePayload) compactSource {
	return compactSource{
		Name:         p.Name,
		Kind:         p.Kind,
		Source:       p.Source,
		Enabled:      p.Enabled,
		State:        p.State,
		Plugins:      p.Plugins,
		Installable:  p.Installable,
		LastReadAt:   p.LastReadAt,
		ErrorClass:   p.ErrorClass,
		Error:        p.Error,
		DocumentPath: p.DocumentPath,
		Owner:        p.Owner,
		Stability:    p.Stability,
		Diagnostics:  compactPlatformDiagnostics(p.Diagnostics),
	}
}

func compactMarketplace(p contract.MarketplaceListResponse) any {
	var items []compactListing
	if p.Items != nil {
		items = make([]compactListing, len(p.Items))
		for i := range p.Items {
			items[i] = projectCompactListing(p.Items[i])
		}
	}
	return struct {
		Categories []contract.MarketplaceCategoryCount `json:"categories"`
		Refreshing bool                                `json:"refreshing,omitzero"`
		Total      int                                 `json:"total"`
		NextCursor string                              `json:"next_cursor,omitempty"`
		Revision   string                              `json:"revision"`
		Stale      bool                                `json:"stale"`
		ErrorClass string                              `json:"error_class,omitempty"`
		Error      string                              `json:"error,omitempty"`
		Sources    []contract.MarketplaceSourceSummary `json:"sources"`
		Items      []compactListing                    `json:"items"`
	}{p.Categories, p.Refreshing, p.Total, p.NextCursor, p.Revision, p.Stale, p.ErrorClass, p.Error, p.Sources, items}
}
func compactMarketplaceSources(p contract.MarketplaceSourcesResponse) any {
	var sources []compactSource
	if p.Sources != nil {
		sources = make([]compactSource, len(p.Sources))
		for i := range p.Sources {
			sources[i] = projectCompactSource(p.Sources[i])
		}
	}
	return map[string]any{"sources": sources}
}
