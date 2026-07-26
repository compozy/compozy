package clawhub

import (
	"strings"

	"github.com/compozy/agh/internal/registry"
)

type clawhubDetailEnvelope struct {
	Skill         *clawhubSkillDetail  `json:"skill"`
	LatestVersion clawhubDetailVersion `json:"latestVersion"`
	Owner         clawhubDetailOwner   `json:"owner"`
}

type clawhubDetailVersion struct {
	Version string `json:"version"`
	License string `json:"license"`
}

type clawhubDetailOwner struct {
	Handle string `json:"handle"`
}

type clawhubSkillDetail struct {
	Slug          string              `json:"slug"`
	Name          string              `json:"name"`
	DisplayName   string              `json:"displayName"`
	Summary       string              `json:"summary"`
	Description   string              `json:"description"`
	Author        string              `json:"author"`
	OwnerHandle   string              `json:"ownerHandle"`
	Version       string              `json:"version"`
	Downloads     int                 `json:"downloads"`
	Readme        string              `json:"readme"`
	MCPServers    []string            `json:"mcp_servers"`
	Tags          clawhubListingTags  `json:"tags"`
	Stats         clawhubListingStats `json:"stats"`
	License       string              `json:"license"`
	Repository    string              `json:"repository"`
	Versions      []string            `json:"versions"`
	LatestVersion struct {
		Version string `json:"version"`
	} `json:"latestVersion"`
	Owner struct {
		Handle string `json:"handle"`
	} `json:"owner"`
}

func (detail clawhubSkillDetail) registryDetail(
	fallbackSlug string,
	envelopeOwner string,
	envelopeVersion string,
	envelopeLicense string,
) registry.Detail {
	slug := firstNonEmpty(detail.Slug, fallbackSlug)
	return registry.Detail{
		Listing: registry.Listing{
			Slug: slug,
			Name: firstNonEmpty(
				detail.Name,
				listingNameFromSlug(slug),
				detail.DisplayName,
			),
			Description: firstNonEmpty(detail.Summary, detail.DisplayName),
			Author: firstNonEmpty(
				detail.Author,
				detail.OwnerHandle,
				detail.Owner.Handle,
				envelopeOwner,
				listingAuthorFromSlug(slug),
			),
			Version: firstNonEmpty(
				detail.Version,
				detail.Tags.Latest,
				detail.LatestVersion.Version,
				envelopeVersion,
			),
			Downloads: firstPositiveInt(
				detail.Downloads,
				detail.Stats.Downloads,
				detail.Stats.InstallsAllTime,
				detail.Stats.InstallsCurrent,
			),
			Source: sourceName,
			Type:   registry.PackageTypeSkill,
		},
		Readme:     firstNonBlankPreserve(detail.Readme, detail.Description),
		MCPServers: detail.MCPServers,
		License:    firstNonEmpty(detail.License, envelopeLicense),
		Repository: strings.TrimSpace(detail.Repository),
		Versions:   detail.Versions,
	}
}

type clawhubListing struct {
	registry.Listing
	DisplayName   string              `json:"displayName"`
	OwnerHandle   string              `json:"ownerHandle"`
	Summary       string              `json:"summary"`
	Tags          clawhubListingTags  `json:"tags"`
	Stats         clawhubListingStats `json:"stats"`
	LatestVersion struct {
		Version string `json:"version"`
	} `json:"latestVersion"`
	Owner struct {
		Handle string `json:"handle"`
	} `json:"owner"`
}

type clawhubListingTags struct {
	Latest string `json:"latest"`
}

type clawhubListingStats struct {
	Downloads       int `json:"downloads"`
	InstallsAllTime int `json:"installsAllTime"`
	InstallsCurrent int `json:"installsCurrent"`
}

func normalizeClawHubListings(listings []clawhubListing) []registry.Listing {
	if listings == nil {
		return []registry.Listing{}
	}

	normalized := make([]registry.Listing, 0, len(listings))
	for _, listing := range listings {
		normalized = append(normalized, listing.registryListing())
	}
	return normalized
}

func (listing clawhubListing) registryListing() registry.Listing {
	result := listing.Listing
	result.Slug = strings.TrimSpace(result.Slug)
	result.Name = firstNonEmpty(result.Name, listingNameFromSlug(result.Slug), listing.DisplayName)
	result.Description = firstNonEmpty(result.Description, listing.Summary, listing.DisplayName)
	result.Author = firstNonEmpty(
		result.Author,
		listing.OwnerHandle,
		listing.Owner.Handle,
		listingAuthorFromSlug(result.Slug),
	)
	result.Version = firstNonEmpty(result.Version, listing.Tags.Latest, listing.LatestVersion.Version)
	if result.Downloads <= 0 {
		result.Downloads = firstPositiveInt(
			listing.Stats.Downloads,
			listing.Stats.InstallsAllTime,
			listing.Stats.InstallsCurrent,
		)
	}
	return result
}

func listingNameFromSlug(slug string) string {
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" {
		return ""
	}
	_, name, found := strings.Cut(strings.TrimPrefix(trimmed, "@"), "/")
	if found {
		return strings.TrimSpace(name)
	}
	return trimmed
}

func listingAuthorFromSlug(slug string) string {
	trimmed := strings.TrimSpace(slug)
	if !strings.HasPrefix(trimmed, "@") {
		return ""
	}
	author, _, found := strings.Cut(strings.TrimPrefix(trimmed, "@"), "/")
	if !found {
		return ""
	}
	return strings.TrimSpace(author)
}
