package extensionpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// GatewayRequirement declares the gateway tiers an extension may expose.
type GatewayRequirement struct {
	Permissions []string `toml:"permissions" json:"permissions"`
}

func (r *GatewayRequirement) Normalize() *GatewayRequirement {
	if r == nil {
		return nil
	}
	permissions := make([]string, 0, len(r.Permissions))
	for _, permission := range r.Permissions {
		if value := strings.TrimSpace(permission); value != "" {
			permissions = append(permissions, value)
		}
	}
	slices.Sort(permissions)
	return &GatewayRequirement{Permissions: slices.Compact(permissions)}
}

func (r *GatewayRequirement) Validate(field string) error {
	if r == nil {
		return nil
	}
	for _, permission := range r.Normalize().Permissions {
		switch permission {
		case "gateway.private", "gateway.public":
		default:
			return &ManifestValidationError{
				Field:   field + ".permissions",
				Value:   permission,
				Message: "unsupported gateway permission",
			}
		}
	}
	return nil
}

// GatewayRequirementDigest binds operator consent to the exact declared gateway permissions.
func GatewayRequirementDigest(req *GatewayRequirement) (string, error) {
	normalized := req.Normalize()
	if normalized == nil || len(normalized.Permissions) == 0 {
		return "", nil
	}
	if err := normalized.Validate("gateway"); err != nil {
		return "", err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("extension: marshal gateway requirement: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
