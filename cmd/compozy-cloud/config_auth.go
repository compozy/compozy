// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/turnstile"
)

const (
	envGoogleID      = "COMPOZY_CLOUD_GOOGLE_CLIENT_ID"
	envGoogleSecret  = "COMPOZY_CLOUD_GOOGLE_CLIENT_SECRET" // #nosec G101 -- environment variable name.
	envGitHubID      = "COMPOZY_CLOUD_GITHUB_CLIENT_ID"
	envGitHubSecret  = "COMPOZY_CLOUD_GITHUB_CLIENT_SECRET" // #nosec G101 -- environment variable name.
	envTurnstileSite = "COMPOZY_CLOUD_TURNSTILE_SITE_KEY"
	envTurnstileKey  = "COMPOZY_CLOUD_TURNSTILE_SECRET_KEY"
	envPKCEKey       = "COMPOZY_CLOUD_PKCE_ENC_KEY"
)

func loadCloudAuth(getenv func(string) string, environment, issuer string) (*auth.Config, error) {
	names := []string{
		envGoogleID, envGoogleSecret, envGitHubID, envGitHubSecret, envTurnstileSite, envTurnstileKey, envPKCEKey,
	}
	settings := make(map[string]string, len(names))
	for _, name := range names {
		if value := strings.TrimSpace(getenv(name)); value != "" {
			settings[name] = value
		}
	}
	if environment == development && len(settings) == 0 {
		return nil, nil
	}
	for _, name := range names {
		if settings[name] == "" {
			return nil, fmt.Errorf("%s is required for Compozy Cloud sign-in", name)
		}
	}
	key, err := base64.RawURLEncoding.Strict().DecodeString(settings[envPKCEKey])
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be a base64url-encoded 32-byte key", envPKCEKey)
	}
	oauth, err := auth.NewOAuth(
		auth.OAuthClient{ID: settings[envGoogleID], Secret: settings[envGoogleSecret]},
		auth.OAuthClient{ID: settings[envGitHubID], Secret: settings[envGitHubSecret]},
	)
	if err != nil {
		return nil, err
	}
	origin, err := url.Parse(issuer)
	if err != nil {
		return nil, errors.New("invalid Compozy Cloud OAuth origin")
	}
	botCheck, err := turnstile.New(settings[envTurnstileKey], origin.Hostname())
	if err != nil {
		return nil, err
	}
	hostedDomain := "compozy.run"
	if environment == staging {
		hostedDomain = "staging." + hostedDomain
	}
	return &auth.Config{
		OAuth: oauth, EncryptionKey: key, Turnstile: botCheck, TurnstileSiteKey: settings[envTurnstileSite],
		TrustEdge: environment != development, HostedDomain: hostedDomain,
		MonthlyPriceID: getenv("STRIPE_PRICE_PRO_MONTHLY"), AnnualPriceID: getenv("STRIPE_PRICE_PRO_ANNUAL"),
	}, nil
}
