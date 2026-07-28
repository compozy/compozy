package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const definitionFingerprintPrefix = "sha256:"

type fingerprintDefinition struct {
	Target           Target   `json:"target"`
	Transport        string   `json:"transport"`
	RemoteURL        string   `json:"remote_url"`
	Type             string   `json:"type"`
	IssuerURL        string   `json:"issuer_url"`
	MetadataURL      string   `json:"metadata_url"`
	AuthorizationURL string   `json:"authorization_url"`
	TokenURL         string   `json:"token_url"`
	RevocationURL    string   `json:"revocation_url"`
	ClientID         string   `json:"client_id"`
	ClientSecretRef  string   `json:"client_secret_ref"`
	Scopes           []string `json:"scopes"`
}

// ServerDefinitionFingerprint binds OAuth state to one exact remote and auth definition.
// Resolved secret plaintext is intentionally excluded; its stable configured ref is included.
func ServerDefinitionFingerprint(cfg ServerConfig) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	scopes := trimStrings(cfg.Scopes)
	slices.Sort(scopes)
	definition := fingerprintDefinition{
		Target:           cfg.Target.Normalize(),
		Transport:        strings.TrimSpace(cfg.Transport),
		RemoteURL:        strings.TrimSpace(cfg.RemoteURL),
		Type:             strings.TrimSpace(cfg.Type),
		IssuerURL:        strings.TrimSpace(cfg.IssuerURL),
		MetadataURL:      strings.TrimSpace(cfg.MetadataURL),
		AuthorizationURL: strings.TrimSpace(cfg.AuthorizationURL),
		TokenURL:         strings.TrimSpace(cfg.TokenURL),
		RevocationURL:    strings.TrimSpace(cfg.RevocationURL),
		ClientID:         strings.TrimSpace(cfg.ClientID),
		ClientSecretRef:  strings.TrimSpace(cfg.ClientSecretRef),
		Scopes:           scopes,
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

func tokenMatchesServerDefinition(token TokenRecord, cfg ServerConfig) bool {
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return false
	}
	return strings.TrimSpace(token.DefinitionFingerprint) == fingerprint
}
