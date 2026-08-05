package registry

import "slices"

func cloneRegistryListings(listings []Listing) []Listing {
	return slices.Clone(listings)
}

func cloneRegistryDetail(detail *Detail) *Detail {
	if detail == nil {
		return nil
	}
	cloned := *detail
	cloned.MCPServers = slices.Clone(detail.MCPServers)
	cloned.Tags = slices.Clone(detail.Tags)
	cloned.Versions = slices.Clone(detail.Versions)
	return &cloned
}

func cloneRegistryDownloadResult(result *DownloadResult) *DownloadResult {
	if result == nil {
		return nil
	}
	cloned := *result
	return &cloned
}
