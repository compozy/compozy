package config

import (
	"errors"
	"fmt"
	"log/slog"

	"net/url"

	"strings"

	"github.com/compozy/agh/internal/extension/surfaces"
)

// Validate ensures the skills configuration is internally consistent.
func (c SkillsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("skills.poll_interval must be positive: %s", c.PollInterval)
	}
	if err := c.Marketplace.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate ensures the extension marketplace configuration is internally consistent.
func (c ExtensionsConfig) Validate() error {
	if err := c.Marketplace.Validate(); err != nil {
		return err
	}
	return c.Resources.Validate()
}

// Validate ensures the extension resource policy is internally consistent.
func (c ExtensionsResourcesConfig) Validate() error {
	if _, err := surfaces.NormalizeAllowedKinds(c.AllowedKinds); err != nil {
		return fmt.Errorf("extensions.resources.allowed_kinds: %w", err)
	}
	if c.MaxScope != "" {
		if err := c.MaxScope.Validate("extensions.resources.max_scope"); err != nil {
			return err
		}
	}
	if err := c.SnapshotRateLimit.Validate("extensions.resources.snapshot_rate_limit"); err != nil {
		return err
	}
	if err := c.OperatorWriteRateLimit.Validate("extensions.resources.operator_write_rate_limit"); err != nil {
		return err
	}
	return nil
}

// Validate ensures one configured resource rate-limit bucket is internally consistent.
func (c ExtensionsResourceRateLimitConfig) Validate(path string) error {
	if c.Requests == 0 && c.Window == 0 && c.Queue == 0 {
		return nil
	}
	if c.Requests <= 0 {
		return fmt.Errorf("%s.requests must be positive: %d", path, c.Requests)
	}
	if c.Window <= 0 {
		return fmt.Errorf("%s.window must be positive: %s", path, c.Window)
	}
	if c.Queue < 0 {
		return fmt.Errorf("%s.queue must be zero or positive: %d", path, c.Queue)
	}
	return nil
}

// Validate ensures the marketplace configuration is internally consistent when configured.
func (c MarketplaceConfig) Validate() error {
	registry := strings.TrimSpace(c.Registry)
	baseURL := strings.TrimSpace(c.BaseURL)
	if registry == "" && baseURL == "" {
		return nil
	}
	if registry == "" {
		return errors.New("skills.marketplace.registry is required")
	}
	if baseURL != "" {
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("skills.marketplace.base_url is invalid: %w", err)
		}
		if parsed.Scheme != marketplaceSchemeHTTP && parsed.Scheme != urlSchemeHTTPS {
			return fmt.Errorf("skills.marketplace.base_url must use http or https: %q", c.BaseURL)
		}
		if strings.TrimSpace(parsed.Host) == "" {
			return fmt.Errorf("skills.marketplace.base_url must include a host: %q", c.BaseURL)
		}
	}

	switch strings.ToLower(registry) {
	case skillsMarketplaceRegistryClawhub:
		return nil
	default:
		return fmt.Errorf("skills.marketplace.registry must be %q: %q", skillsMarketplaceRegistryClawhub, c.Registry)
	}
}

// Validate ensures the extension marketplace configuration is internally consistent when configured.
func (c ExtensionsMarketplaceConfig) Validate() error {
	const githubRegistry = "github"

	registry := strings.TrimSpace(c.Registry)
	baseURL := strings.TrimSpace(c.BaseURL)
	if registry == "" && baseURL == "" {
		return nil
	}
	if registry == "" {
		return errors.New("extensions.marketplace.registry is required")
	}
	if baseURL != "" {
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("extensions.marketplace.base_url is invalid: %w", err)
		}
		if parsed.Scheme != marketplaceSchemeHTTP && parsed.Scheme != urlSchemeHTTPS {
			return fmt.Errorf("extensions.marketplace.base_url must use http or https: %q", c.BaseURL)
		}
		if strings.TrimSpace(parsed.Host) == "" {
			return fmt.Errorf("extensions.marketplace.base_url must include a host: %q", c.BaseURL)
		}
		if parsed.Scheme == marketplaceSchemeHTTP {
			slog.Warn("config: extensions marketplace base_url uses insecure http scheme", "url", c.BaseURL)
		}
	}

	switch strings.ToLower(registry) {
	case githubRegistry:
		return nil
	default:
		return fmt.Errorf("extensions.marketplace.registry must be %q: %q", githubRegistry, c.Registry)
	}
}
