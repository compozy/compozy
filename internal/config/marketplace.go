package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	DefaultMarketplaceCatalogBaseURL = "https://raw.githubusercontent.com/compozy/compozy/main/catalog"
	DefaultMarketplaceCatalogTTL     = "1h"
	DefaultMarketplaceCatalogTimeout = "10s"
	// MarketplaceCatalogBaseURLEnv selects a runtime-only catalog source without persisting it.
	MarketplaceCatalogBaseURLEnv = "COMPOZY_MARKETPLACE_CATALOG_BASE_URL"
)

// MarketplaceRuntimeConfig contains shared curated-marketplace runtime settings.
type MarketplaceRuntimeConfig struct {
	Catalog MarketplaceCatalogConfig `toml:"catalog"`
}

// MarketplaceCatalogConfig controls curated feed fetching and projection freshness.
type MarketplaceCatalogConfig struct {
	BaseURL string `toml:"base_url"`
	TTL     string `toml:"ttl"`
	Timeout string `toml:"timeout"`
}

type marketplaceRuntimeOverlay struct {
	Catalog marketplaceCatalogOverlay `toml:"catalog"`
}

type marketplaceCatalogOverlay struct {
	BaseURL *string `toml:"base_url"`
	TTL     *string `toml:"ttl"`
	Timeout *string `toml:"timeout"`
}

// DefaultMarketplaceRuntimeConfig returns the curated catalog defaults.
func DefaultMarketplaceRuntimeConfig() MarketplaceRuntimeConfig {
	return MarketplaceRuntimeConfig{
		Catalog: MarketplaceCatalogConfig{
			BaseURL: DefaultMarketplaceCatalogBaseURL,
			TTL:     DefaultMarketplaceCatalogTTL,
			Timeout: DefaultMarketplaceCatalogTimeout,
		},
	}
}

// Validate reports whether the curated marketplace config is usable.
func (c MarketplaceRuntimeConfig) Validate() error {
	return c.Catalog.Validate("marketplace.catalog")
}

// Validate reports whether the curated catalog fetch settings are usable.
func (c MarketplaceCatalogConfig) Validate(path string) error {
	if err := validateMarketplaceCatalogBaseURL(path+".base_url", c.EffectiveBaseURL()); err != nil {
		return err
	}
	if err := validatePositiveDuration(path+".ttl", c.EffectiveTTL()); err != nil {
		return err
	}
	return validatePositiveDuration(path+".timeout", c.EffectiveTimeout())
}

// ApplyMarketplaceCatalogEnvironment overlays the runtime catalog source without persisting it.
func ApplyMarketplaceCatalogEnvironment(cfg *Config, getenv func(string) string) {
	if cfg == nil || getenv == nil {
		return
	}
	if value := strings.TrimSpace(getenv(MarketplaceCatalogBaseURLEnv)); value != "" {
		cfg.Marketplace.Catalog.BaseURL = value
	}
}

func validateMarketplaceCatalogBaseURL(path string, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must be an absolute HTTP(S) or file URL", path)
	}
	switch parsed.Scheme {
	case marketplaceSchemeHTTP, urlSchemeHTTPS:
		if parsed.Host != "" {
			return nil
		}
	case "file":
		if parsed.Host == "" && filepath.IsAbs(filepath.FromSlash(parsed.Path)) {
			return nil
		}
	}
	return fmt.Errorf("%s must be an absolute HTTP(S) or file URL", path)
}

// EffectiveBaseURL returns the configured feed base URL or the built-in default.
func (c MarketplaceCatalogConfig) EffectiveBaseURL() string {
	if value := strings.TrimSpace(c.BaseURL); value != "" {
		return value
	}
	return DefaultMarketplaceCatalogBaseURL
}

// EffectiveTTL returns the configured projection TTL or the built-in default.
func (c MarketplaceCatalogConfig) EffectiveTTL() string {
	if value := strings.TrimSpace(c.TTL); value != "" {
		return value
	}
	return DefaultMarketplaceCatalogTTL
}

// EffectiveTimeout returns the configured request timeout or the built-in default.
func (c MarketplaceCatalogConfig) EffectiveTimeout() string {
	if value := strings.TrimSpace(c.Timeout); value != "" {
		return value
	}
	return DefaultMarketplaceCatalogTimeout
}

func (o marketplaceRuntimeOverlay) Apply(dst *MarketplaceRuntimeConfig) {
	o.Catalog.Apply(&dst.Catalog)
}

func (o marketplaceCatalogOverlay) Apply(dst *MarketplaceCatalogConfig) {
	if o.BaseURL != nil {
		dst.BaseURL = *o.BaseURL
	}
	if o.TTL != nil {
		dst.TTL = *o.TTL
	}
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
}
