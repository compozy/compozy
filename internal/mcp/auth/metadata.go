package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	metadataS256Value = "S256"
)

const metadataWellKnownPath = "/.well-known/oauth-authorization-server"
const defaultMetadataClientTimeout = 10 * time.Second

func discoverMetadata(ctx context.Context, client *http.Client, cfg ServerConfig) (metadata Metadata, err error) {
	if ctx == nil {
		return Metadata{}, errors.New("mcp auth: metadata context is required")
	}
	if client == nil {
		client = &http.Client{Timeout: defaultMetadataClientTimeout}
	}

	if strings.TrimSpace(cfg.MetadataURL) == "" &&
		strings.TrimSpace(cfg.IssuerURL) == "" &&
		strings.TrimSpace(cfg.AuthorizationURL) != "" &&
		strings.TrimSpace(cfg.TokenURL) != "" {
		metadata := Metadata{
			AuthorizationEndpoint:         strings.TrimSpace(cfg.AuthorizationURL),
			TokenEndpoint:                 strings.TrimSpace(cfg.TokenURL),
			RevocationEndpoint:            strings.TrimSpace(cfg.RevocationURL),
			CodeChallengeMethodsSupported: []string{metadataS256Value},
		}
		if err := metadata.Validate(); err != nil {
			return Metadata{}, err
		}
		return metadata, nil
	}

	metadataURL, err := resolveMetadataURL(cfg)
	if err != nil {
		return Metadata{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, http.NoBody)
	if err != nil {
		return Metadata{}, fmt.Errorf("mcp auth: build metadata request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("mcp auth: fetch OAuth metadata: %w", err)
	}
	defer func() { err = errors.Join(err, drainAndCloseResponseBody(resp.Body)) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Metadata{}, fmt.Errorf("mcp auth: fetch OAuth metadata: HTTP %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("mcp auth: decode OAuth metadata: %w", err)
	}
	if err := metadata.Validate(); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func resolveMetadataURL(cfg ServerConfig) (string, error) {
	if raw := strings.TrimSpace(cfg.MetadataURL); raw != "" {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("mcp auth: invalid metadata_url %q", raw)
		}
		if err := validateAbsoluteHTTPURL("metadata URL", parsed.String()); err != nil {
			return "", err
		}
		return parsed.String(), nil
	}

	issuer := strings.TrimSpace(cfg.IssuerURL)
	if issuer == "" {
		return "", errors.New("mcp auth: issuer_url or metadata_url is required")
	}
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("mcp auth: invalid issuer_url %q", issuer)
	}
	if err := validateAbsoluteHTTPURL("issuer URL", parsed.String()); err != nil {
		return "", err
	}
	issuerPath := strings.Trim(parsed.Path, "/")
	parsed.Path = metadataWellKnownPath
	if issuerPath != "" {
		parsed.Path += "/" + issuerPath
	}
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

// Validate checks OAuth metadata required by authorization code with PKCE.
func (m Metadata) Validate() error {
	if strings.TrimSpace(m.AuthorizationEndpoint) == "" {
		return errors.New("mcp auth: authorization endpoint is required")
	}
	if strings.TrimSpace(m.TokenEndpoint) == "" {
		return errors.New("mcp auth: token endpoint is required")
	}
	if err := validateAbsoluteHTTPURL("authorization endpoint", m.AuthorizationEndpoint); err != nil {
		return err
	}
	if err := validateAbsoluteHTTPURL("token endpoint", m.TokenEndpoint); err != nil {
		return err
	}
	if strings.TrimSpace(m.RevocationEndpoint) != "" {
		if err := validateAbsoluteHTTPURL("revocation endpoint", m.RevocationEndpoint); err != nil {
			return err
		}
	}
	return nil
}

func validateAbsoluteHTTPURL(label string, raw string) error {
	if err := aghconfig.ValidateMCPOAuthURL(label, raw); err != nil {
		return fmt.Errorf("mcp auth: %w", err)
	}
	return nil
}
