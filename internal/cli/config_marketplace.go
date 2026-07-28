package cli

const (
	marketplaceCatalogBaseURLPath = "marketplace.catalog.base_url"
	marketplaceCatalogTTLPath     = "marketplace.catalog.ttl"
	marketplaceCatalogTimeoutPath = "marketplace.catalog.timeout"
)

func marketplaceConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		marketplaceCatalogBaseURLPath: configSetString,
		marketplaceCatalogTTLPath:     configSetDuration,
		marketplaceCatalogTimeoutPath: configSetDuration,
	}
}
