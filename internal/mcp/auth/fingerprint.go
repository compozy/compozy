package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const definitionFingerprintPrefix = "sha256:"

type fingerprintDefinition struct {
	Target          Target `json:"target"`
	Transport       string `json:"transport"`
	RemoteURL       string `json:"remote_url"`
	CatalogEntry    string `json:"catalog_entry"`
	Type            string `json:"type"`
	IssuerURL       string `json:"issuer_url"`
	ClientID        string `json:"client_id"`
	ClientSecretRef string `json:"client_secret_ref"`
	Registration    string `json:"registration"`
	ResourceURL     string `json:"resource_url"`
	PRMURL          string `json:"prm_url"`
}

// ServerDefinitionFingerprint binds OAuth state to one exact remote and auth definition.
// Resolved secret plaintext is intentionally excluded; its stable configured ref is included.
func ServerDefinitionFingerprint(cfg ServerConfig) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	clientID := strings.TrimSpace(cfg.ClientID)
	clientSecretRef := strings.TrimSpace(cfg.ClientSecretRef)
	if cfg.registrationStrategy() == RegistrationAuto {
		clientID = ""
		clientSecretRef = ""
	}
	definition := fingerprintDefinition{
		Target:          cfg.Target.Normalize(),
		Transport:       strings.TrimSpace(cfg.Transport),
		RemoteURL:       strings.TrimSpace(cfg.RemoteURL),
		CatalogEntry:    strings.TrimSpace(cfg.CatalogEntry),
		Type:            strings.TrimSpace(cfg.Type),
		IssuerURL:       strings.TrimSpace(cfg.IssuerURL),
		ClientID:        clientID,
		ClientSecretRef: clientSecretRef,
		Registration:    string(cfg.registrationStrategy()),
		ResourceURL:     strings.TrimSpace(cfg.ResourceURL),
		PRMURL:          strings.TrimSpace(cfg.PRMURL),
	}
	payload, err := json.Marshal(definition)
	if err != nil {
		return "", fmt.Errorf("mcp auth: encode server definition fingerprint: %w", err)
	}
	digest := sha256.Sum256(payload)
	return definitionFingerprintPrefix + hex.EncodeToString(digest[:]), nil
}

// ValidateDefinitionFingerprint rejects unbound or malformed durable token records.
func ValidateDefinitionFingerprint(value string) error {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, definitionFingerprintPrefix) {
		return errors.New("mcp auth: server definition fingerprint must use sha256")
	}
	raw := strings.TrimPrefix(trimmed, definitionFingerprintPrefix)
	if len(raw) != sha256.Size*2 {
		return errors.New("mcp auth: server definition fingerprint has invalid length")
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return fmt.Errorf("mcp auth: server definition fingerprint is invalid: %w", err)
	}
	return nil
}

func tokenMatchesFingerprint(token TokenRecord, cfg ServerConfig) bool {
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return false
	}
	return strings.TrimSpace(token.DefinitionFingerprint) == fingerprint
}

func (s *Service) tokenMatchesServerDefinition(
	ctx context.Context,
	token TokenRecord,
	cfg ServerConfig,
) (bool, error) {
	if !tokenMatchesFingerprint(token, cfg) {
		return false, nil
	}
	if !scopesCover(token.Scopes, cfg.Scopes) {
		return false, nil
	}
	if cfg.registrationStrategy() == RegistrationPreRegistered {
		return issuersEqual(token.Issuer, cfg.IssuerURL) &&
			strings.TrimSpace(token.ClientID) == strings.TrimSpace(cfg.ClientID), nil
	}
	if s.registrations == nil {
		return false, errors.New(
			"mcp auth: registration store is required to validate token binding",
		)
	}
	registration, err := s.registrations.GetMCPAuthRegistration(ctx, cfg.Target)
	if err != nil {
		if errors.Is(err, ErrRegistrationNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("mcp auth: load registration for token binding: %w", err)
	}
	return registration.DefinitionFingerprint == token.DefinitionFingerprint &&
		issuersEqual(registration.Issuer, token.Issuer) &&
		strings.TrimSpace(registration.ClientID) == strings.TrimSpace(token.ClientID) &&
		strings.TrimSpace(
			registration.ResourceURL,
		) == firstNonEmpty(
			cfg.ResourceURL,
			cfg.RemoteURL,
		), nil
}

func scopesCover(granted []string, required []string) bool {
	grantedSet := make(map[string]struct{}, len(granted))
	for _, scope := range trimStrings(granted) {
		grantedSet[scope] = struct{}{}
	}
	for _, scope := range trimStrings(required) {
		if _, ok := grantedSet[scope]; !ok {
			return false
		}
	}
	return true
}
