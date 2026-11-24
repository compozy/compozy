package main

import (
	"cmp"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	envEnvironment = "COMPOZY_CLOUD_ENV"
	envAddress     = "COMPOZY_CLOUD_LISTEN_ADDR"
	envPooledDSN   = "COMPOZY_CLOUD_DATABASE_URL"
	envDirectDSN   = "COMPOZY_CLOUD_DATABASE_DIRECT_URL"
	envSigningKID  = "COMPOZY_CLOUD_SIGNING_KID"
	envSigningSeed = "COMPOZY_CLOUD_SIGNING_SEED"
	envKeyOverlap  = "COMPOZY_CLOUD_JWKS_OVERLAP"

	production  = "production"
	staging     = "staging"
	development = "development"
)

type serveConfig struct {
	Address string
	Cloud   cloud.Config
}

func cloudEnvironment(getenv func(string) string) (string, error) {
	environment := cmp.Or(getenv(envEnvironment), production)
	switch environment {
	case production, staging, development:
		return environment, nil
	default:
		return "", fmt.Errorf("%s must be production, staging or development", envEnvironment)
	}
}

func databaseURL(getenv func(string) string, name, environment string) (string, error) {
	dsn := strings.TrimSpace(getenv(name))
	if dsn == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgresql" && parsed.Scheme != "postgres") || parsed.Hostname() == "" {
		return "", fmt.Errorf("%s must be a Postgres URL", name)
	}
	if environment != development && parsed.Query().Get("sslmode") != "verify-full" {
		return "", fmt.Errorf("%s must use sslmode=verify-full", name)
	}
	return dsn, nil
}

func loadServeConfig(getenv func(string) string) (serveConfig, error) {
	environment, err := cloudEnvironment(getenv)
	if err != nil {
		return serveConfig{}, err
	}
	pooled, err := databaseURL(getenv, envPooledDSN, environment)
	if err != nil {
		return serveConfig{}, err
	}
	direct, err := databaseURL(getenv, envDirectDSN, environment)
	if err != nil {
		return serveConfig{}, err
	}
	keys, err := loadSigningKeys(getenv)
	if err != nil {
		return serveConfig{}, err
	}
	issuer := remoteproto.CloudIssuer
	if environment == staging {
		issuer = "https://staging.cloud.compozy.com"
	}
	auth, err := loadCloudAuth(getenv, environment, issuer)
	if err != nil {
		return serveConfig{}, err
	}
	tunnels, err := loadHostedTunnels(getenv, environment)
	if err != nil {
		return serveConfig{}, err
	}
	return serveConfig{
		Address: cmp.Or(getenv(envAddress), ":8080"),
		Cloud: cloud.Config{
			Postgres:      store.Config{PooledDSN: pooled, DirectDSN: direct},
			Keys:          keys,
			Issuer:        issuer,
			Auth:          auth,
			MonthlyPrice:  getenv("STRIPE_PRICE_PRO_MONTHLY"),
			AnnualPrice:   getenv("STRIPE_PRICE_PRO_ANNUAL"),
			HostedTunnels: tunnels,
		},
	}, nil
}

func loadSigningKeys(getenv func(string) string) (*api.SigningKeys, error) {
	seed, err := base64.RawURLEncoding.Strict().DecodeString(getenv(envSigningSeed))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s must be a base64url Ed25519 seed", envSigningSeed)
	}
	kid := strings.TrimSpace(getenv(envSigningKID))
	if kid == "" {
		return nil, fmt.Errorf("%s is required", envSigningKID)
	}
	signer, err := remoteproto.NewSigner(kid, ed25519.NewKeyFromSeed(seed))
	if err != nil {
		return nil, err
	}
	var overlap []remoteproto.JWK
	if value := getenv(envKeyOverlap); value != "" {
		var keys remoteproto.JWKS
		if err := json.Unmarshal([]byte(value), &keys); err != nil {
			return nil, errors.New("invalid cloud overlap JWKS")
		}
		overlap = keys.Keys
	}
	return api.NewSigningKeys(signer, overlap)
}
