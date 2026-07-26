// Package vault owns encrypted daemon-managed secret material.
package vault

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	typesSessionsKey = "sessions"
)

var (
	// ErrSecretNotFound reports that a secret reference has no stored value.
	ErrSecretNotFound = errors.New("vault: secret not found")
	// ErrUnsupportedSecretRef reports that a launch binding uses an unsupported reference scheme.
	ErrUnsupportedSecretRef = errors.New("vault: unsupported secret ref")
	// ErrMissingSecret reports that a required env or secret reference resolved to no value.
	ErrMissingSecret = errors.New("vault: secret value missing")
)

// EnvNamePattern is the daemon-wide grammar for environment variable names.
var EnvNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var vaultRefPattern = regexp.MustCompile(
	`^vault:(providers|bridges|automation|mcp|hooks|extensions|sandbox|sessions)/` +
		`[a-z0-9][a-z0-9_.-]*(?:/[A-Za-z0-9][A-Za-z0-9_.-]*)*$`,
)

var vaultRefPrefixPattern = regexp.MustCompile(
	`^vault:(providers|bridges|automation|mcp|hooks|extensions|sandbox|sessions)/` +
		`(?:[A-Za-z0-9][A-Za-z0-9_.-]*(?:/|$))*$`,
)

var supportedNamespaces = map[string]struct{}{
	"automation":     {},
	"bridges":        {},
	"extensions":     {},
	"hooks":          {},
	"mcp":            {},
	"providers":      {},
	"sandbox":        {},
	typesSessionsKey: {},
}

var secretLikeEnvNeedles = []string{
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"PASSWD",
	"API_KEY",
	"APIKEY",
	"PRIVATE_KEY",
	"PRIVATEKEY",
	"AUTHORIZATION",
	"BEARER",
	"CREDENTIAL",
}

// Record is one encrypted vault row. EncryptedValue must never contain plaintext.
type Record struct {
	Ref            string
	Kind           string
	EncryptedValue string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Metadata is a redacted vault row safe for operator-facing status surfaces.
type Metadata struct {
	Ref       string
	Kind      string
	Present   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store persists encrypted vault records.
type Store interface {
	PutVaultSecret(ctx context.Context, record Record) error
	GetVaultSecret(ctx context.Context, ref string) (Record, error)
	ListVaultSecrets(ctx context.Context, prefix string) ([]Record, error)
	DeleteVaultSecret(ctx context.Context, ref string) error
}

// NormalizeRef returns the trimmed secret ref used by stores and resolvers.
func NormalizeRef(ref string) string {
	return strings.TrimSpace(ref)
}

// IsSecretRef reports whether a ref points at AGH-managed encrypted storage.
func IsSecretRef(ref string) bool {
	return strings.HasPrefix(NormalizeRef(ref), "vault:")
}

// IsEnvRef reports whether a ref points at an operator-managed environment variable.
func IsEnvRef(ref string) bool {
	return strings.HasPrefix(NormalizeRef(ref), "env:")
}

// EnvNameFromRef returns the validated environment variable name in an env: ref.
func EnvNameFromRef(ref string) (string, error) {
	normalized := NormalizeRef(ref)
	envName, ok := strings.CutPrefix(normalized, "env:")
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
	envName = strings.TrimSpace(envName)
	if !EnvNamePattern.MatchString(envName) {
		return "", fmt.Errorf("%w: invalid env ref %q", ErrUnsupportedSecretRef, normalized)
	}
	return envName, nil
}

// ValidateSecretRef reports whether ref belongs to one of AGH's durable vault namespaces.
func ValidateSecretRef(ref string) error {
	normalized := NormalizeRef(ref)
	if !vaultRefPattern.MatchString(normalized) {
		return fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
	return nil
}

// ValidateSecretRefPrefix reports whether prefix can safely filter AGH vault refs.
func ValidateSecretRefPrefix(prefix string) error {
	normalized := NormalizeRef(prefix)
	if normalized == "" {
		return nil
	}
	if !vaultRefPrefixPattern.MatchString(normalized) {
		return fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
	return nil
}

// RefMatchesPrefix reports whether ref is exactly prefix or is nested below it.
func RefMatchesPrefix(ref string, prefix string) bool {
	normalizedRef := NormalizeRef(ref)
	normalizedPrefix := NormalizeRef(prefix)
	if normalizedPrefix == "" {
		return true
	}
	if normalizedRef == normalizedPrefix {
		return true
	}
	if !strings.HasSuffix(normalizedPrefix, "/") {
		normalizedPrefix += "/"
	}
	return strings.HasPrefix(normalizedRef, normalizedPrefix)
}

// ValidateNamespace reports whether namespace is one of AGH's durable vault namespaces.
func ValidateNamespace(namespace string) error {
	normalized := strings.Trim(strings.TrimSpace(namespace), "/")
	if normalized == "" {
		return fmt.Errorf("%w: namespace is required", ErrUnsupportedSecretRef)
	}
	if _, ok := supportedNamespaces[normalized]; !ok {
		return fmt.Errorf("%w: vault namespace %q", ErrUnsupportedSecretRef, normalized)
	}
	return nil
}

// SecretRefNamespace returns the first path segment for a validated vault ref.
func SecretRefNamespace(ref string) (string, error) {
	normalized := NormalizeRef(ref)
	if err := ValidateSecretRef(normalized); err != nil {
		return "", err
	}
	withoutScheme := strings.TrimPrefix(normalized, "vault:")
	namespace, _, ok := strings.Cut(withoutScheme, "/")
	if !ok || namespace == "" {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
	return namespace, nil
}

// SecretRefPrefixNamespace returns the first path segment for a validated vault ref prefix.
func SecretRefPrefixNamespace(prefix string) (string, error) {
	normalized := NormalizeRef(prefix)
	if err := ValidateSecretRefPrefix(normalized); err != nil {
		return "", err
	}
	if normalized == "" {
		return "", fmt.Errorf("%w: namespace is required", ErrUnsupportedSecretRef)
	}
	withoutScheme := strings.TrimPrefix(normalized, "vault:")
	namespace, _, ok := strings.Cut(withoutScheme, "/")
	if !ok || namespace == "" {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
	return namespace, nil
}

// ValidateSecretRefNamespace reports whether ref is a vault ref in the requested namespace.
func ValidateSecretRefNamespace(ref string, namespace string) error {
	normalizedNamespace := strings.Trim(strings.TrimSpace(namespace), "/")
	if normalizedNamespace == "" {
		return fmt.Errorf("%w: namespace is required", ErrUnsupportedSecretRef)
	}
	refNamespace, err := SecretRefNamespace(ref)
	if err != nil {
		return err
	}
	if refNamespace != normalizedNamespace {
		return fmt.Errorf(
			"%w: %s must use vault:%s/<path>",
			ErrUnsupportedSecretRef,
			NormalizeRef(ref),
			normalizedNamespace,
		)
	}
	return nil
}

// ValidateRef reports whether ref uses AGH's supported env: or vault: grammar.
func ValidateRef(ref string) error {
	normalized := NormalizeRef(ref)
	switch {
	case IsEnvRef(normalized):
		_, err := EnvNameFromRef(normalized)
		return err
	case IsSecretRef(normalized):
		return ValidateSecretRef(normalized)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
}

// ValidateRefNamespace reports whether ref uses env: or the requested vault namespace.
func ValidateRefNamespace(ref string, namespace string) error {
	normalized := NormalizeRef(ref)
	switch {
	case IsEnvRef(normalized):
		_, err := EnvNameFromRef(normalized)
		return err
	case IsSecretRef(normalized):
		return ValidateSecretRefNamespace(normalized, namespace)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
}

// SecretLikeEnvName reports whether an environment variable name conventionally
// carries durable credential material and should be declared through secret_env.
func SecretLikeEnvName(name string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(name))
	if nonCredentialEnvName(normalized) {
		return false
	}
	for _, needle := range secretLikeEnvNeedles {
		if strings.Contains(normalized, needle) {
			return true
		}
	}
	return false
}

func nonCredentialEnvName(name string) bool {
	switch {
	case strings.HasSuffix(name, "_URL"):
		return true
	case strings.HasSuffix(name, "_URI"):
		return true
	case strings.HasSuffix(name, "_PATH"):
		return true
	case strings.HasSuffix(name, "_FILE"):
		return true
	case strings.HasSuffix(name, "_DIR"):
		return true
	default:
		return false
	}
}

// ValidateNonSecretEnvMap rejects literal env maps that appear to carry secrets.
func ValidateNonSecretEnvMap(path string, env map[string]string) error {
	for key := range env {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return fmt.Errorf("%s.env contains an empty environment variable name", path)
		}
		if !EnvNamePattern.MatchString(trimmedKey) {
			return fmt.Errorf("%s.env.%s must be an environment variable name", path, trimmedKey)
		}
		if SecretLikeEnvName(trimmedKey) {
			return fmt.Errorf("%s.env.%s must move secret-like values to secret_env", path, trimmedKey)
		}
	}
	return nil
}

// ValidateSecretEnvMap validates env-name to secret-ref bindings for one vault namespace.
func ValidateSecretEnvMap(path string, namespace string, secretEnv map[string]string) error {
	for key, ref := range secretEnv {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return fmt.Errorf("%s.secret_env contains an empty environment variable name", path)
		}
		if !EnvNamePattern.MatchString(trimmedKey) {
			return fmt.Errorf("%s.secret_env.%s must be an environment variable name", path, trimmedKey)
		}
		if err := ValidateRefNamespace(ref, namespace); err != nil {
			return fmt.Errorf("%s.secret_env.%s is invalid: %w", path, trimmedKey, err)
		}
	}
	return nil
}
