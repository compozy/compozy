package modelcatalog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ExecutionScope identifies the owner boundary for persisted catalog observations.
type ExecutionScope string

const (
	// ExecutionScopeGlobal owns provider-independent catalog data.
	ExecutionScopeGlobal ExecutionScope = "global"
	// ExecutionScopeProfile owns observations derived from one provider profile.
	ExecutionScopeProfile ExecutionScope = "profile"
	// ExecutionScopeWorkspace owns observations derived from one workspace runtime.
	ExecutionScopeWorkspace ExecutionScope = "workspace"
)

// CatalogExecutionContext isolates catalog observations produced by one runtime environment.
// CommandFingerprint is required for non-global contexts before persistence.
type CatalogExecutionContext struct {
	Scope              ExecutionScope
	ProfileID          string
	WorkspaceID        string
	CommandFingerprint string
}

// GlobalCatalogExecutionContext returns the sole context for provider-independent sources.
func GlobalCatalogExecutionContext() CatalogExecutionContext {
	return CatalogExecutionContext{Scope: ExecutionScopeGlobal}
}

// NormalizeCatalogExecutionScope validates an execution scope before source-specific fingerprinting.
func NormalizeCatalogExecutionScope(value CatalogExecutionContext) (CatalogExecutionContext, error) {
	normalized := CatalogExecutionContext{
		Scope:              ExecutionScope(strings.TrimSpace(string(value.Scope))),
		ProfileID:          strings.TrimSpace(value.ProfileID),
		WorkspaceID:        strings.TrimSpace(value.WorkspaceID),
		CommandFingerprint: strings.TrimSpace(value.CommandFingerprint),
	}
	switch normalized.Scope {
	case ExecutionScopeGlobal:
		if normalized.ProfileID != "" || normalized.WorkspaceID != "" || normalized.CommandFingerprint != "" {
			return CatalogExecutionContext{}, errors.New(
				"model catalog: global execution context cannot include profile, workspace, or command fingerprint",
			)
		}
	case ExecutionScopeProfile:
		if normalized.ProfileID == "" {
			return CatalogExecutionContext{}, errors.New("model catalog: profile execution context requires profile_id")
		}
		if normalized.WorkspaceID != "" {
			return CatalogExecutionContext{}, errors.New(
				"model catalog: profile execution context cannot include workspace_id",
			)
		}
	case ExecutionScopeWorkspace:
		if normalized.ProfileID == "" || normalized.WorkspaceID == "" {
			return CatalogExecutionContext{}, errors.New(
				"model catalog: workspace execution context requires profile_id and workspace_id",
			)
		}
	default:
		return CatalogExecutionContext{}, fmt.Errorf(
			"model catalog: unsupported execution scope %q",
			value.Scope,
		)
	}
	return normalized, nil
}

// NormalizePersistedExecutionContext validates a source context that is ready for persistence.
func NormalizePersistedExecutionContext(value CatalogExecutionContext) (CatalogExecutionContext, error) {
	normalized, err := NormalizeCatalogExecutionScope(value)
	if err != nil {
		return CatalogExecutionContext{}, err
	}
	if normalized.Scope != ExecutionScopeGlobal && normalized.CommandFingerprint == "" {
		return CatalogExecutionContext{}, errors.New(
			"model catalog: scoped execution context requires command_fingerprint",
		)
	}
	return normalized, nil
}

// WithCommandFingerprint binds a source fingerprint to a profile or workspace scope.
func (c CatalogExecutionContext) WithCommandFingerprint(fingerprint string) (CatalogExecutionContext, error) {
	normalized, err := NormalizeCatalogExecutionScope(c)
	if err != nil {
		return CatalogExecutionContext{}, err
	}
	if normalized.Scope == ExecutionScopeGlobal {
		return normalized, nil
	}
	normalized.CommandFingerprint = strings.TrimSpace(fingerprint)
	return NormalizePersistedExecutionContext(normalized)
}

// ID returns a stable, non-sensitive database key for the normalized context.
func (c CatalogExecutionContext) ID() (string, error) {
	normalized, err := NormalizePersistedExecutionContext(c)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{
		string(normalized.Scope),
		normalized.ProfileID,
		normalized.WorkspaceID,
		normalized.CommandFingerprint,
	}, "\x00")))
	return hex.EncodeToString(digest[:]), nil
}

func executionContextScopeKey(value CatalogExecutionContext) string {
	return strings.Join([]string{
		string(value.Scope),
		strings.TrimSpace(value.ProfileID),
		strings.TrimSpace(value.WorkspaceID),
	}, "\x00")
}

// CatalogExecutionFingerprint hashes non-secret execution inputs without persisting their contents.
func CatalogExecutionFingerprint(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}
